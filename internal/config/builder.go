package config

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/efureev/parallel/internal/flow"
)

// cmdNameOnly — шаблон отображаемого имени без аргументов.
const cmdNameOnly = "%CMD_NAME%"

// dockerRunSubcommand — подкоманда docker по умолчанию. Строка совпадает
// с именем поля схемы `run` случайно: это разные вещи.
const dockerRunSubcommand = "run"

// dockerBinary — исполняемый файл, которым запускается docker-режим.
//
// Отдельно от одноимённого ключа схемы в loader.go: совпадение строк случайно,
// одна из них — имя поля YAML, другая — программа, которую надо найти в PATH.
const dockerBinary = "docker"

// dirResolver разрешает относительные рабочие каталоги команд от каталога
// конфигурации.
//
// Абсолютные пути и пустые значения остаются как есть. Если базы нет — например,
// конфигурация разобрана из памяти в тесте, — поведение прежнее: путь трактует
// операционная система относительно текущего каталога процесса.
func dirResolver(base string) func(string) string {
	return func(dir string) string {
		if dir == "" || base == "" || filepath.IsAbs(dir) {
			return dir
		}

		return filepath.Join(base, dir)
	}
}

// envPairs переводит карту переменных окружения в упорядоченный список "KEY=VALUE".
//
// Сортировка обязательна: обход Go-мапы рандомизирован, а порядок переменных
// попадает в окружение процесса и в тесты. Недетерминированность здесь стоила бы
// мигающих прогонов на ровном месте.
func envPairs(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}

	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, k+"="+env[k])
	}

	return pairs
}

// FlowBuilder собирает доменный Flow из загруженной конфигурации.
//
// Логгера у билдера нет намеренно: раньше отсутствие ключа commands он сообщал
// в лог и возвращал пустой Flow — валидный на вид zero value, о проблеме
// вызывающий не узнавал. Теперь это ошибка, а логирование —
// забота вызывающего слоя.
type FlowBuilder struct{}

func NewFlowBuilder() *FlowBuilder {
	return &FlowBuilder{}
}

// Build преобразует Data в доменную структуру Flow.
// Порядок цепочек и команд внутри них сохраняется ровно таким, каким он задан в конфигурации.
func (b *FlowBuilder) Build(data Data) (flow.Flow, error) {
	if len(data.Chains) == 0 {
		return flow.Flow{}, ErrMissingCommands
	}

	resolve := dirResolver(data.BaseDir)

	// Верхнеуровневые файлы читаются один раз на весь Flow: они общие, и
	// перечитывать их на каждую команду значило бы обращаться к диску впустую.
	baseEnv, err := loadEnvFiles(data.EnvFiles, resolve)
	if err != nil {
		return flow.Flow{}, err
	}

	result := &flow.Flow{}

	for idx, chainCfg := range data.Chains {
		chain := &flow.CommandChain{
			Name:     chainCfg.Name,
			ColorIdx: idx,
		}

		for _, namedCmd := range chainCfg.Commands {
			var cmd flow.Command

			env, lookup, err := commandEnv(namedCmd.Spec, baseEnv, resolve)
			if err != nil {
				return flow.Flow{}, fmt.Errorf("chain %q, command %q: %w", chainCfg.Name, namedCmd.Name, err)
			}

			if namedCmd.Spec.Docker != nil {
				cmd, err = b.createDockerCommand(namedCmd.Name, namedCmd.Spec, env, lookup, resolve)
			} else {
				cmd, err = b.createRegularCommand(namedCmd.Name, namedCmd.Spec, env, lookup)
			}

			if err != nil {
				return flow.Flow{}, fmt.Errorf("chain %q, command %q: %w", chainCfg.Name, namedCmd.Name, err)
			}

			if cmd.Dir, err = expand(cmd.Dir, lookup); err != nil {
				return flow.Flow{}, fmt.Errorf("chain %q, command %q, dir: %w", chainCfg.Name, namedCmd.Name, err)
			}

			cmd.Dir = resolve(cmd.Dir)

			chain.Add(cmd)
		}

		result.AddChain(chain)
	}

	return *result, nil
}

// loadEnvFiles читает и сливает переменные из перечисленных файлов.
//
// Пути разрешаются от файла конфигурации тем же способом, что и dir: файл
// с переменными лежит рядом с проектом, а не там, откуда его запускают.
// Отсутствующий файл — ошибка: молча пропущенный envFile даёт запуск без
// половины настроек, и понять это можно только по странному поведению команд.
func loadEnvFiles(paths []string, resolve func(string) string) (map[string]string, error) {
	merged := make(map[string]string, len(paths))

	for _, path := range paths {
		env, err := loadDotEnv(resolve(path))
		if err != nil {
			return nil, err
		}

		maps.Copy(merged, env)
	}

	return merged, nil
}

// commandEnv собирает окружение команды и набор значений для подстановки.
//
// Приоритет от слабого к сильному: окружение процесса → верхнеуровневые файлы →
// файлы команды → env. Возвращается два набора, и они намеренно разные:
// в окружение команды уходит всё, а источником подстановки служит всё, КРОМЕ
// самого env. Причина не в эстетике: env декодируется в Go-мапу, порядок
// записей теряется, и разрешать ссылки внутри неё пришлось бы в произвольном
// порядке.
func commandEnv(
	cmdRaw command, baseEnv map[string]string, resolve func(string) string,
) (env, lookup map[string]string, err error) {
	own, err := loadEnvFiles(cmdRaw.EnvFile, resolve)
	if err != nil {
		return nil, nil, err
	}

	lookup = make(map[string]string, len(baseEnv)+len(own))

	for _, pair := range os.Environ() {
		if key, value, found := strings.Cut(pair, "="); found {
			lookup[key] = value
		}
	}

	maps.Copy(lookup, baseEnv)
	maps.Copy(lookup, own)

	// Окружение команды строится из файлов, а переменные процесса добавит
	// раннер: копировать их в каждую команду незачем.
	env = make(map[string]string, len(baseEnv)+len(own)+len(cmdRaw.Env))
	maps.Copy(env, baseEnv)
	maps.Copy(env, own)

	for key, value := range cmdRaw.Env {
		expanded, expErr := expand(value, lookup)
		if expErr != nil {
			return nil, nil, fmt.Errorf("env %q: %w", key, expErr)
		}

		env[key] = expanded
	}

	return env, lookup, nil
}

// restartOf разбирает и проверяет политику перезапуска команды.
//
// Отрицательные значения отвергаются здесь же: «минус одна попытка» и
// «задержка в прошлое» смысла не имеют, а молча превратить их в ноль значило бы
// подменить заданное умолчанием.
func restartOf(cmdRaw command) (flow.RestartPolicy, int, time.Duration, error) {
	policy, err := flow.ParseRestartPolicy(cmdRaw.Restart)
	if err != nil {
		return "", 0, 0, err
	}

	if cmdRaw.RestartAttempts < 0 {
		return "", 0, 0, fmt.Errorf("%w: restartAttempts is %d", ErrNegativeValue, cmdRaw.RestartAttempts)
	}

	if cmdRaw.RestartDelay < 0 {
		return "", 0, 0, fmt.Errorf("%w: restartDelay is %s", ErrNegativeValue, cmdRaw.RestartDelay)
	}

	return policy, cmdRaw.RestartAttempts, cmdRaw.RestartDelay, nil
}

// resolveVolume разрешает хостовую часть тома относительно файла конфигурации.
//
// Трогаются только явно относительные пути — начинающиеся с ./ или ../. Всё
// прочее оставляется как есть: «data:/var/lib» это имя тома, а не каталог,
// и превратив его в путь, мы сломали бы именованные тома. Абсолютные пути,
// включая windows-овские с двоеточием после буквы диска, тоже не затрагиваются.
func resolveVolume(spec string, resolve func(string) string) string {
	host, rest, found := strings.Cut(spec, ":")
	if !found {
		// Анонимный том: задан только путь внутри контейнера.
		return spec
	}

	if !strings.HasPrefix(host, "./") && !strings.HasPrefix(host, "../") {
		return spec
	}

	return resolve(host) + ":" + rest
}

// dockerArgs собирает аргументы вызова docker.
//
// Порядок здесь — не вкус, а требование самого docker: всё после имени образа
// считается командой контейнера. Поэтому флаги идут строго до образа, а
// docker.args — строго после.
func dockerArgs(
	cmdName string, spec *dockerCommand, env, lookup map[string]string, resolve func(string) string,
) ([]string, error) {
	dockerCmd := spec.Cmd
	if dockerCmd == `` {
		dockerCmd = dockerRunSubcommand
	}

	args := []string{dockerCmd, `--name`, cmdName}

	if spec.RemoveAfterAll == nil {
		args = append(args, `--rm`)
	}

	if spec.Image.Pull != `` {
		args = append(args, `--pull`, spec.Image.Pull)
	}

	ports, err := expandAll(spec.Ports, lookup)
	if err != nil {
		return nil, err
	}

	for _, port := range ports {
		args = append(args, `-p`, port)
	}

	args, err = appendVolumes(args, spec.Volumes, lookup, resolve)
	if err != nil {
		return nil, err
	}

	network, err := expand(spec.Network, lookup)
	if err != nil {
		return nil, err
	}

	if network != `` {
		args = append(args, `--network`, network)
	}

	// Переменные уходят контейнеру флагом -e, а не окружению процесса docker.
	// Клиент docker собственное окружение контейнеру не передаёт, поэтому
	// заполнение Env здесь означало бы, что заданное пользователем рядом с
	// секцией docker не доходит никуда.
	//
	// Подстановка к ним уже применена в commandEnv: повторять её нельзя, иначе
	// значение, содержащее литеральную ${...}, раскрылось бы дважды.
	for _, pair := range envPairs(env) {
		args = append(args, `-e`, pair)
	}

	image, err := dockerImageRef(spec, lookup)
	if err != nil {
		return nil, err
	}

	args = append(args, image)

	// Команда контейнера — строго после образа.
	containerArgs, err := expandAll(spec.Args, lookup)
	if err != nil {
		return nil, err
	}

	return append(args, containerArgs...), nil
}

// appendVolumes дописывает тома, разрешая относительные хостовые пути.
func appendVolumes(
	args, volumes []string, lookup map[string]string, resolve func(string) string,
) ([]string, error) {
	expanded, err := expandAll(volumes, lookup)
	if err != nil {
		return nil, err
	}

	for _, volume := range expanded {
		args = append(args, `-v`, resolveVolume(volume, resolve))
	}

	return args, nil
}

// dockerImageRef собирает ссылку на образ вида name:tag.
func dockerImageRef(spec *dockerCommand, lookup map[string]string) (string, error) {
	name, err := expand(spec.Image.Name, lookup)
	if err != nil {
		return "", err
	}

	tag, err := expand(spec.Image.Tag, lookup)
	if err != nil {
		return "", err
	}

	if tag == `` {
		tag = `latest`
	}

	return name + `:` + tag, nil
}

func (b *FlowBuilder) createDockerCommand(
	cmdName string, cmdRaw command, env, lookup map[string]string, resolve func(string) string,
) (flow.Command, error) {
	args, err := dockerArgs(cmdName, cmdRaw.Docker, env, lookup, resolve)
	if err != nil {
		return flow.Command{}, err
	}

	policy, attempts, delay, err := restartOf(cmdRaw)
	if err != nil {
		return flow.Command{}, err
	}

	// Аргументы docker-команды собраны нами целиком и в префиксе каждой строки
	// вывода превращаются в шум: с томами и командой контейнера они длиннее
	// самого вывода. Поэтому по умолчанию показываем только имя — как и
	// у строковой формы run. Явный формат из конфигурации не трогаем.
	format := cmdRaw.Format.CmdName
	if format == "" {
		format = cmdNameOnly
	}

	return flow.Command{
		Name:    cmdName,
		Cmd:     dockerBinary,
		Args:    args,
		Dir:     cmdRaw.Dir,
		Pipe:    true,
		Disable: cmdRaw.Disable,
		// Env намеренно пуст: переменные уже ушли в аргументы флагами -e.
		Format:          flow.Format{CmdName: format},
		Timeout:         cmdRaw.Timeout,
		Restart:         policy,
		RestartAttempts: attempts,
		RestartDelay:    delay,
	}, nil
}

// createRegularCommand собирает обычную команду из формы cmd или run.
//
// Обе формы разом — почти наверняка недосмотр при правке конфигурации, и молча
// предпочесть одну значило бы выполнить не то, что написано.
func (b *FlowBuilder) createRegularCommand(
	cmdName string, cmdRaw command, env, lookup map[string]string,
) (flow.Command, error) {
	var (
		cmdStr string
		args   []string
	)

	format := cmdRaw.Format.CmdName

	switch {
	case len(cmdRaw.Cmd) > 0 && cmdRaw.Run != "":
		return flow.Command{}, ErrCmdAndRun

	case cmdRaw.Run != "":
		cmdStr, args = shellCommand(cmdRaw.Run)

		// У shell-формы аргумент ровно один — вся команда целиком, — и в
		// префиксе каждой строки вывода он превращается в шум вида
		// «api > serve -c echo one && echo two (0) > one». Поэтому по умолчанию
		// показываем только имя; явный формат из конфигурации остаётся за
		// пользователем.
		if format == "" {
			format = cmdNameOnly
		}

	case len(cmdRaw.Cmd) > 0:
		// Подстановка только в форме cmd: тело run: раскрывает оболочка, и
		// второе раскрытие нашими силами дало бы либо двойное, либо
		// расхождение с тем, что пользователь ждёт от $VAR.
		expanded, expErr := expandAll(cmdRaw.Cmd, lookup)
		if expErr != nil {
			return flow.Command{}, expErr
		}

		cmdStr = expanded[0]
		if len(expanded) > 1 {
			args = expanded[1:]
		}
	}

	policy, attempts, delay, err := restartOf(cmdRaw)
	if err != nil {
		return flow.Command{}, err
	}

	return flow.Command{
		Name:            cmdName,
		Cmd:             cmdStr,
		Args:            args,
		Dir:             cmdRaw.Dir,
		Pipe:            cmdRaw.Pipe,
		Disable:         cmdRaw.Disable,
		Env:             envPairs(env),
		Format:          flow.Format{CmdName: format},
		Timeout:         cmdRaw.Timeout,
		Restart:         policy,
		RestartAttempts: attempts,
		RestartDelay:    delay,
	}, nil
}
