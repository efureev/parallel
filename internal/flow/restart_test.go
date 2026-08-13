package flow

import (
	"errors"
	"strings"
	"testing"
)

// TestRestartPolicy_ShouldRestart — вся разница между политиками в одной таблице.
func TestRestartPolicy_ShouldRestart(t *testing.T) {
	boom := errors.New("boom")

	tests := []struct {
		policy      RestartPolicy
		afterFail   bool
		afterSucess bool
	}{
		{policy: RestartNever, afterFail: false, afterSucess: false},
		{policy: RestartOnFailure, afterFail: true, afterSucess: false},
		{policy: RestartAlways, afterFail: true, afterSucess: true},
		// Пустое значение — это отсутствие поля, а не особая политика.
		{policy: "", afterFail: false, afterSucess: false},
	}

	for _, tt := range tests {
		t.Run(tt.policy.String(), func(t *testing.T) {
			if got := tt.policy.ShouldRestart(boom); got != tt.afterFail {
				t.Errorf("после отказа = %v, ожидалось %v", got, tt.afterFail)
			}

			if got := tt.policy.ShouldRestart(nil); got != tt.afterSucess {
				t.Errorf("после успеха = %v, ожидалось %v", got, tt.afterSucess)
			}
		})
	}
}

// TestParseRestartPolicy: опечатка обязана падать со списком допустимых, иначе
// пользователь получил бы «не перезапускать» там, где просил обратного.
func TestParseRestartPolicy(t *testing.T) {
	valid := map[string]RestartPolicy{
		"":           RestartNever,
		"never":      RestartNever,
		"on-failure": RestartOnFailure,
		"always":     RestartAlways,
	}

	for in, want := range valid {
		got, err := ParseRestartPolicy(in)
		if err != nil {
			t.Errorf("%q отвергнуто: %v", in, err)
		}

		if got != want {
			t.Errorf("%q → %q, ожидалось %q", in, got, want)
		}
	}

	for _, bad := range []string{"on_failure", "onfailure", "ALWAYS", "yes"} {
		_, err := ParseRestartPolicy(bad)
		if !errors.Is(err, ErrUnknownRestartPolicy) {
			t.Errorf("%q принято без ошибки: %v", bad, err)
		}

		for _, want := range []string{"never", "on-failure", "always"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("в сообщении про %q нет допустимого %q: %v", bad, want, err)
			}
		}
	}
}
