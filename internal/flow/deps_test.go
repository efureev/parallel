package flow

import (
	"errors"
	"strings"
	"testing"
)

// depsFlow собирает Flow из описания «цепочка → предшественники».
func depsFlow(spec map[string][]string, order ...string) Flow {
	f := Flow{}

	for i, name := range order {
		chain := &CommandChain{Name: name, ColorIdx: i, Needs: spec[name]}
		chain.Add(Command{Name: "x", Cmd: "echo"})
		f.AddChain(chain)
	}

	return f
}

// TestValidateDeps_Cycle — сообщить только факт цикла недостаточно: в
// конфигурации из десятка цепочек искать его глазами это отдельная работа.
func TestValidateDeps_Cycle(t *testing.T) {
	f := depsFlow(map[string][]string{
		"a": {"b"},
		"b": {"c"},
		"c": {"a"},
	}, "a", "b", "c")

	err := ValidateDeps(f)
	if !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("ожидалась ErrDependencyCycle, получено %v", err)
	}

	for _, want := range []string{"a", "b", "c", "->"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("в сообщении нет %q: %v", want, err)
		}
	}
}

func TestValidateDeps_SelfDependency(t *testing.T) {
	f := depsFlow(map[string][]string{"a": {"a"}}, "a")

	if err := ValidateDeps(f); !errors.Is(err, ErrSelfDependency) {
		t.Fatalf("ожидалась ErrSelfDependency, получено %v", err)
	}
}

// TestValidateDeps_UnknownChain: ссылка на несуществующую цепочку означала бы
// вечное ожидание того, чего нет.
func TestValidateDeps_UnknownChain(t *testing.T) {
	f := depsFlow(map[string][]string{"api": {"nope"}}, "api")

	err := ValidateDeps(f)
	if !errors.Is(err, ErrUnknownDependency) {
		t.Fatalf("ожидалась ErrUnknownDependency, получено %v", err)
	}

	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("в сообщении нет имени: %v", err)
	}
}

func TestValidateDeps_Valid(t *testing.T) {
	f := depsFlow(map[string][]string{
		"api":    {"db"},
		"worker": {"db", "queue"},
	}, "db", "queue", "api", "worker")

	if err := ValidateDeps(f); err != nil {
		t.Fatalf("корректный граф отвергнут: %v", err)
	}
}

// TestWithDependencies — «запусти api» почти всегда означает «и то, без чего
// он не работает».
func TestWithDependencies(t *testing.T) {
	f := depsFlow(map[string][]string{
		"api":    {"db"},
		"worker": {"queue"},
		"queue":  {"db"},
	}, "db", "queue", "api", "worker")

	tests := []struct {
		name     string
		selected []string
		want     string
	}{
		{name: "прямая зависимость", selected: []string{"api"}, want: "db,api"},
		{name: "транзитивная", selected: []string{"worker"}, want: "db,queue,worker"},
		{name: "без зависимостей", selected: []string{"db"}, want: "db"},
		{name: "пустой отбор не трогаем", selected: nil, want: ""},
		{
			name:     "порядок из конфигурации, а не из аргументов",
			selected: []string{"worker", "api"},
			want:     "db,queue,api,worker",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := strings.Join(WithDependencies(f, tt.selected), ","); got != tt.want {
				t.Errorf("получено %q, ожидалось %q", got, tt.want)
			}
		})
	}
}

// TestOrder — уровни запуска для предпросмотра.
func TestOrder(t *testing.T) {
	f := depsFlow(map[string][]string{
		"api":    {"db"},
		"worker": {"api"},
	}, "db", "api", "worker", "ui")

	levels := Order(f)
	if len(levels) != 3 {
		t.Fatalf("уровней %d, ожидалось 3: %v", len(levels), levels)
	}

	// ui ни от кого не зависит и стартует сразу, вместе с db.
	if got := strings.Join(levels[0], ","); got != "db,ui" {
		t.Errorf("первый уровень = %q, ожидался %q", got, "db,ui")
	}

	if got := strings.Join(levels[2], ","); got != "worker" {
		t.Errorf("третий уровень = %q", got)
	}
}

// TestSelect_PullsInDependencies связывает отбор с графом: без этого
// `parallel api` запустил бы api без базы.
func TestSelect_PullsInDependencies(t *testing.T) {
	f := depsFlow(map[string][]string{"api": {"db"}}, "db", "api", "ui")

	got, err := Select(f, []string{"api"}, nil)
	if err != nil {
		t.Fatalf("select: %v", err)
	}

	if names := strings.Join(got.Names(), ","); names != "db,api" {
		t.Errorf("получено %q, ожидалось %q", names, "db,api")
	}
}
