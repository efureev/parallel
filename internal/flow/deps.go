package flow

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrUnknownDependency — цепочка ссылается на несуществующую.
	ErrUnknownDependency = errors.New("unknown dependency")
	// ErrDependencyCycle — зависимости образуют цикл.
	ErrDependencyCycle = errors.New("dependency cycle")
	// ErrSelfDependency — цепочка зависит сама от себя.
	ErrSelfDependency = errors.New("chain depends on itself")
)

// ValidateDeps проверяет граф зависимостей целиком.
//
// Обе проверки обязаны выполняться до запуска: неизвестное имя иначе означало
// бы вечное ожидание того, чего нет, а цикл — взаимную блокировку, которую
// снаружи не отличить от зависшей команды.
func ValidateDeps(f Flow) error {
	known := make(map[string]*CommandChain, len(f.Chains))
	for _, chain := range f.Chains {
		known[chain.Name] = chain
	}

	for _, chain := range f.Chains {
		for _, need := range chain.Needs {
			if need == chain.Name {
				return fmt.Errorf("%w: %q", ErrSelfDependency, chain.Name)
			}

			if _, ok := known[need]; !ok {
				return fmt.Errorf("%w: chain %q needs %q, available: %s",
					ErrUnknownDependency, chain.Name, need, strings.Join(names(f), ", "))
			}
		}
	}

	return findCycle(f, known)
}

// findCycle ищет цикл обходом в глубину и называет его участников.
//
// Сообщить только факт цикла недостаточно: в конфигурации из десятка цепочек
// искать его глазами — отдельная работа, которую инструмент может сделать сам.
func findCycle(f Flow, known map[string]*CommandChain) error {
	const (
		white = 0 // не посещали
		grey  = 1 // в текущем пути обхода
		black = 2 // полностью обработана
	)

	color := make(map[string]int, len(f.Chains))

	var path []string

	var visit func(name string) error

	visit = func(name string) error {
		color[name] = grey
		path = append(path, name)

		for _, need := range known[name].Needs {
			switch color[need] {
			case grey:
				// Нашли возврат в текущий путь: цикл — это его хвост.
				start := 0

				for i, n := range path {
					if n == need {
						start = i

						break
					}
				}

				return fmt.Errorf("%w: %s", ErrDependencyCycle,
					strings.Join(append(append([]string{}, path[start:]...), need), " -> "))
			case white:
				if err := visit(need); err != nil {
					return err
				}
			case black:
			}
		}

		path = path[:len(path)-1]
		color[name] = black

		return nil
	}

	for _, chain := range f.Chains {
		if color[chain.Name] == white {
			if err := visit(chain.Name); err != nil {
				return err
			}
		}
	}

	return nil
}

// WithDependencies дополняет отбор предшественниками названных цепочек.
//
// «Запусти api» почти всегда означает «и то, без чего он не работает»: без базы
// api всё равно не поднимется, и заставлять перечислять всю цепочку зависимостей
// руками — работа, которую инструмент видит сам.
//
// Порядок результата повторяет конфигурацию, а не обход графа: он виден
// пользователю в предпросмотре и в раскраске.
func WithDependencies(f Flow, selected []string) []string {
	if len(selected) == 0 {
		return selected
	}

	known := make(map[string]*CommandChain, len(f.Chains))
	for _, chain := range f.Chains {
		known[chain.Name] = chain
	}

	wanted := make(map[string]bool, len(selected))

	var add func(name string)

	add = func(name string) {
		if wanted[name] {
			return
		}

		wanted[name] = true

		chain, ok := known[name]
		if !ok {
			return
		}

		for _, need := range chain.Needs {
			add(need)
		}
	}

	for _, name := range selected {
		add(name)
	}

	out := make([]string, 0, len(wanted))

	for _, chain := range f.Chains {
		if wanted[chain.Name] {
			out = append(out, chain.Name)
		}
	}

	return out
}

// Order раскладывает цепочки по уровням запуска: цепочки одного уровня
// стартуют одновременно, следующий ждёт предыдущего.
//
// Нужен предпросмотру: «что запустится» без порядка отвечает на половину
// вопроса, когда зависимости заданы.
func Order(f Flow) [][]string {
	depth := make(map[string]int, len(f.Chains))
	known := make(map[string]*CommandChain, len(f.Chains))

	for _, chain := range f.Chains {
		known[chain.Name] = chain
	}

	var levelOf func(name string) int

	levelOf = func(name string) int {
		if d, ok := depth[name]; ok {
			return d
		}

		// Значение ставится до обхода предков: при цикле это не даст уйти
		// в бесконечную рекурсию. Сам цикл ловит ValidateDeps.
		depth[name] = 0

		chain, ok := known[name]
		if !ok {
			return 0
		}

		best := 0

		for _, need := range chain.Needs {
			if d := levelOf(need) + 1; d > best {
				best = d
			}
		}

		depth[name] = best

		return best
	}

	maxLevel := 0

	for _, chain := range f.Chains {
		if d := levelOf(chain.Name); d > maxLevel {
			maxLevel = d
		}
	}

	levels := make([][]string, maxLevel+1)
	for _, chain := range f.Chains {
		levels[depth[chain.Name]] = append(levels[depth[chain.Name]], chain.Name)
	}

	return levels
}
