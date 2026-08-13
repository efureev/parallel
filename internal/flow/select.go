package flow

import (
	"errors"
	"fmt"
	"strings"
)

// ErrUnknownChain — в отборе назван несуществующей цепочки.
var ErrUnknownChain = errors.New("unknown chain")

// Select оставляет во Flow только названные цепочки, затем убирает исключённые.
//
// Пустой include означает «все»: отбор задаётся не наличием списка, а его
// содержимым. Порядок и ColorIdx цепочек сохраняются исходными — цвет цепочки
// не должен зависеть от того, запущена она одна или вместе с соседями, иначе
// привычка «фронтенд всегда синий» ломается при каждом сужении отбора.
//
// Неизвестное имя — ошибка, а не пустой отбор: опечатка в имени цепочки иначе
// приводила бы к «успешному» запуску, в котором не выполнилось ничего.
func Select(f Flow, include, exclude []string) (Flow, error) {
	if len(include) == 0 && len(exclude) == 0 {
		return f, nil
	}

	known := make(map[string]bool, len(f.Chains))
	for _, chain := range f.Chains {
		known[chain.Name] = true
	}

	for _, name := range append(append([]string{}, include...), exclude...) {
		if !known[name] {
			return Flow{}, fmt.Errorf("%w %q, available: %s", ErrUnknownChain, name, strings.Join(names(f), ", "))
		}
	}

	// Предшественники подтягиваются сами: «запусти api» почти всегда означает
	// «и то, без чего он не работает». Исключение при этом остаётся явным —
	// если цепочку убрали через --except, её не вернёт и зависимость.
	wanted := toSet(WithDependencies(f, include))
	unwanted := toSet(exclude)

	result := Flow{}

	for _, chain := range f.Chains {
		if len(wanted) > 0 && !wanted[chain.Name] {
			continue
		}

		if unwanted[chain.Name] {
			continue
		}

		result.AddChain(chain)
	}

	if len(result.Chains) == 0 {
		return Flow{}, fmt.Errorf("%w: selection leaves no chains to run", ErrNoChains)
	}

	return result, nil
}

// names возвращает имена цепочек в порядке объявления — для сообщения об ошибке.
func names(f Flow) []string {
	out := make([]string, 0, len(f.Chains))
	for _, chain := range f.Chains {
		out = append(out, chain.Name)
	}

	return out
}

// Names — имена цепочек в порядке объявления.
func (f *Flow) Names() []string {
	return names(*f)
}

func toSet(items []string) map[string]bool {
	if len(items) == 0 {
		return nil
	}

	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}

	return set
}
