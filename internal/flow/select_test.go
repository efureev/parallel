package flow

import (
	"errors"
	"strings"
	"testing"
)

func sampleFlow() Flow {
	f := Flow{}

	for i, name := range []string{"api", "ui", "worker"} {
		chain := &CommandChain{Name: name, ColorIdx: i}
		chain.Add(Command{Name: "run", Cmd: "echo"})
		f.AddChain(chain)
	}

	return f
}

func TestSelect(t *testing.T) {
	tests := []struct {
		name    string
		include []string
		exclude []string
		want    []string
	}{
		{name: "без отбора — все", want: []string{"api", "ui", "worker"}},
		{name: "только названные", include: []string{"api", "ui"}, want: []string{"api", "ui"}},
		{name: "кроме названной", exclude: []string{"worker"}, want: []string{"api", "ui"}},
		{
			name:    "отбор и исключение вместе",
			include: []string{"api", "ui"},
			exclude: []string{"ui"},
			want:    []string{"api"},
		},
		{
			name:    "порядок задаётся конфигурацией, а не аргументами",
			include: []string{"worker", "api"},
			want:    []string{"api", "worker"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Select(sampleFlow(), tt.include, tt.exclude)
			if err != nil {
				t.Fatalf("select: %v", err)
			}

			if strings.Join(got.Names(), ",") != strings.Join(tt.want, ",") {
				t.Errorf("получено %v, ожидалось %v", got.Names(), tt.want)
			}
		})
	}
}

// TestSelect_KeepsColorIdx: цвет цепочки не должен зависеть от того, запущена
// она одна или со всеми, иначе привычка «фронтенд всегда синий» ломается при
// каждом сужении отбора.
func TestSelect_KeepsColorIdx(t *testing.T) {
	got, err := Select(sampleFlow(), []string{"worker"}, nil)
	if err != nil {
		t.Fatalf("select: %v", err)
	}

	if idx := got.Chains[0].ColorIdx; idx != 2 {
		t.Errorf("ColorIdx = %d, ожидался исходный 2", idx)
	}
}

// TestSelect_UnknownChain — опечатка в имени обязана падать: иначе запуск
// «успешно» не выполнит ничего.
func TestSelect_UnknownChain(t *testing.T) {
	for _, tt := range []struct {
		name             string
		include, exclude []string
	}{
		{name: "в отборе", include: []string{"appi"}},
		{name: "в исключении", exclude: []string{"workr"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Select(sampleFlow(), tt.include, tt.exclude)
			if !errors.Is(err, ErrUnknownChain) {
				t.Fatalf("ожидалась ErrUnknownChain, получено %v", err)
			}

			// Список доступных имён обязателен: иначе непонятно, как исправить.
			for _, want := range []string{"api", "ui", "worker"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("в сообщении нет %q: %v", want, err)
				}
			}
		})
	}
}

// TestSelect_EmptyResult: исключить всё — не то же самое, что не исключать
// ничего, и молча запускать всё в этом случае нельзя.
func TestSelect_EmptyResult(t *testing.T) {
	_, err := Select(sampleFlow(), nil, []string{"api", "ui", "worker"})
	if !errors.Is(err, ErrNoChains) {
		t.Fatalf("ожидалась ErrNoChains, получено %v", err)
	}
}
