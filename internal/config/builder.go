package config

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/efureev/parallel/internal/flow"
)

// cmdNameOnly — шаблон отображаемого имени без аргументов.
const cmdNameOnly = "%CMD_NAME%"

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

	result := &flow.Flow{}

	for idx, chainCfg := range data.Chains {
		chain := &flow.CommandChain{
			Name:     chainCfg.Name,
			ColorIdx: idx,
		}

		for _, namedCmd := range chainCfg.Commands {
			var cmd flow.Command

			if namedCmd.Spec.Docker != nil {
				cmd = b.createDockerCommand(namedCmd.Name, namedCmd.Spec)
			} else {
				var err error
				if cmd, err = b.createRegularCommand(namedCmd.Name, namedCmd.Spec); err != nil {
					return flow.Flow{}, fmt.Errorf("chain %q, command %q: %w", chainCfg.Name, namedCmd.Name, err)
				}
			}

			cmd.Dir = resolve(cmd.Dir)

			chain.Add(cmd)
		}

		result.AddChain(chain)
	}

	return *result, nil
}

func (b *FlowBuilder) createDockerCommand(cmdName string, cmdRaw command) flow.Command {
	dockerCmd := cmdRaw.Docker.Cmd
	if dockerCmd == `` {
		dockerCmd = `run`
	}

	args := []string{dockerCmd, `--name`, cmdName}

	if cmdRaw.Docker.RemoveAfterAll == nil {
		args = append(args, `--rm`)
	}

	if cmdRaw.Docker.Image.Pull != `` {
		args = append(args, `--pull`, cmdRaw.Docker.Image.Pull)
	}

	for _, port := range cmdRaw.Docker.Ports {
		args = append(args, `-p`, port)
	}

	// Переменные уходят контейнеру флагом -e, а не окружению процесса docker.
	// Клиент docker собственное окружение контейнеру не передаёт, поэтому
	// заполнение Env здесь означало бы, что заданное пользователем рядом с
	// секцией docker не доходит никуда.
	//
	// Флаги обязаны стоять до имени образа: всё после него docker трактует как
	// команду контейнера.
	for _, pair := range envPairs(cmdRaw.Env) {
		args = append(args, `-e`, pair)
	}

	imageTag := cmdRaw.Docker.Image.Tag
	if imageTag == `` {
		imageTag = `latest`
	}

	imageName := cmdRaw.Docker.Image.Name + `:` + imageTag
	args = append(args, imageName)

	return flow.Command{
		Name:    cmdName,
		Cmd:     dockerBinary,
		Args:    args,
		Dir:     cmdRaw.Dir,
		Pipe:    true,
		Disable: cmdRaw.Disable,
		// Env намеренно пуст: переменные уже ушли в аргументы флагами -e.
		Format: flow.Format{CmdName: cmdRaw.Format.CmdName},
	}
}

// createRegularCommand собирает обычную команду из формы cmd или run.
//
// Обе формы разом — почти наверняка недосмотр при правке конфигурации, и молча
// предпочесть одну значило бы выполнить не то, что написано.
func (b *FlowBuilder) createRegularCommand(cmdName string, cmdRaw command) (flow.Command, error) {
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
		cmdStr = cmdRaw.Cmd[0]
		if len(cmdRaw.Cmd) > 1 {
			args = cmdRaw.Cmd[1:]
		}
	}

	return flow.Command{
		Name:    cmdName,
		Cmd:     cmdStr,
		Args:    args,
		Dir:     cmdRaw.Dir,
		Pipe:    cmdRaw.Pipe,
		Disable: cmdRaw.Disable,
		Env:     envPairs(cmdRaw.Env),
		Format:  flow.Format{CmdName: format},
	}, nil
}
