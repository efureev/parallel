package flow

// MissingDir описывает команду, чей рабочий каталог не существует.
type MissingDir struct {
	Chain   string
	Command string
	Dir     string
}

// MissingDirs возвращает команды, у которых задан несуществующий рабочий каталог.
//
// Это намеренно НЕ часть Validate: отсутствие каталога не всегда ошибка.
// Каталог может создаваться предыдущей командой той же цепочки или соседней
// цепочкой, и падать на старте из-за этого нельзя — конфигурации, работавшие
// до v1, обязаны продолжать работать. Вызывающий слой решает сам: сейчас CLI
// печатает предупреждение, потому что в подавляющем большинстве случаев это
// опечатка в пути, а ошибка от ОС при запуске команды об этом не говорит.
func MissingDirs(f *Flow) []MissingDir {
	if f == nil {
		return nil
	}

	var result []MissingDir

	for _, chain := range f.Chains {
		for _, cmd := range chain.Commands() {
			if cmd.Dir == "" || PathExists(cmd.Dir) {
				continue
			}

			result = append(result, MissingDir{
				Chain:   chain.Name,
				Command: cmd.DisplayName(),
				Dir:     cmd.Dir,
			})
		}
	}

	return result
}
