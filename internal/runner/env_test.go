//go:build !windows

package runner

import (
	"os/exec"
	"testing"

	"github.com/efureev/parallel/internal/flow"
)

// TestSetEnv_NilWhenNoOwnVars: без собственных переменных Env не трогается вовсе.
// exec.Cmd с нулевым Env наследует окружение родителя сам, а безусловный
// os.Environ() копировал бы весь блок на каждый запуск команды.
func TestSetEnv_NilWhenNoOwnVars(t *testing.T) {
	cmd := execCommandForTest()

	setEnv(cmd, flow.Command{Cmd: "echo"})

	if cmd.Env != nil {
		t.Fatalf("expected Env to stay nil, got %d entries", len(cmd.Env))
	}
}

func TestSetEnv_AppendsOwnVars(t *testing.T) {
	cmd := execCommandForTest()

	setEnv(cmd, flow.Command{Cmd: "echo", Env: []string{"APP_ENV=test"}})

	if len(cmd.Env) == 0 {
		t.Fatal("expected inherited environment plus own vars")
	}

	// Собственные переменные идут последними: при совпадении ключей
	// побеждает значение из конфигурации.
	if last := cmd.Env[len(cmd.Env)-1]; last != "APP_ENV=test" {
		t.Errorf("own var must come last, got %q", last)
	}
}

// TestEnvReachesProcess — сквозная проверка: переменная из конфигурации
// действительно видна дочернему процессу.
func TestEnvReachesProcess(t *testing.T) {
	requireIntegration(t)

	mgr := newTestManager(t)

	chain := &flow.CommandChain{Name: "env"}
	chain.Add(flow.Command{
		Name: "show",
		Cmd:  "sh",
		Args: []string{"-c", `test "$PARALLEL_E2E_VAR" = "from-config"`},
		Env:  []string{"PARALLEL_E2E_VAR=from-config"},
	})

	if err := mgr.Execute(t.Context(), chain, chain.Commands()[0]); err != nil {
		t.Fatalf("переменная окружения не дошла до процесса: %v", err)
	}
}

func TestEnvOverridesInherited(t *testing.T) {
	requireIntegration(t)

	t.Setenv("PARALLEL_E2E_OVERRIDE", "from-parent")

	mgr := newTestManager(t)

	chain := &flow.CommandChain{Name: "env"}
	chain.Add(flow.Command{
		Name: "show",
		Cmd:  "sh",
		Args: []string{"-c", `test "$PARALLEL_E2E_OVERRIDE" = "from-config"`},
		Env:  []string{"PARALLEL_E2E_OVERRIDE=from-config"},
	})

	if err := mgr.Execute(t.Context(), chain, chain.Commands()[0]); err != nil {
		t.Fatalf("значение из конфигурации не перебило унаследованное: %v", err)
	}
}

// execCommandForTest собирает команду-заглушку для проверки setEnv.
func execCommandForTest() *exec.Cmd {
	return exec.Command("echo")
}
