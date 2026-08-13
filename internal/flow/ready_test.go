package flow

import (
	"errors"
	"testing"
	"time"
)

func TestReadyCondition_Validate(t *testing.T) {
	tests := []struct {
		name    string
		ready   *ReadyCondition
		wantErr error
	}{
		{name: "отсутствует вовсе", ready: nil},
		{name: "tcp", ready: &ReadyCondition{TCP: "127.0.0.1:5432"}},
		{name: "exec", ready: &ReadyCondition{Exec: []string{"pg_isready"}}},
		{name: "logLine", ready: &ReadyCondition{LogLine: "ready"}},
		{name: "ни одного", ready: &ReadyCondition{}, wantErr: ErrReadyEmpty},
		{
			name:    "два сразу",
			ready:   &ReadyCondition{TCP: "127.0.0.1:1", LogLine: "ready"},
			wantErr: ErrReadyAmbiguous,
		},
		{
			name:    "отрицательный срок",
			ready:   &ReadyCondition{TCP: "127.0.0.1:1", Timeout: -time.Second},
			wantErr: ErrNegativeTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ready.Validate()
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("неожиданная ошибка: %v", err)
				}

				return
			}

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("получено %v, ожидалась %v", err, tt.wantErr)
			}
		})
	}
}

func TestReadyCondition_Limit(t *testing.T) {
	if got := (*ReadyCondition)(nil).Limit(); got != DefaultReadyTimeout {
		t.Errorf("nil → %s, ожидалось %s", got, DefaultReadyTimeout)
	}

	if got := (&ReadyCondition{}).Limit(); got != DefaultReadyTimeout {
		t.Errorf("ноль → %s, ожидалось умолчание", got)
	}

	if got := (&ReadyCondition{Timeout: time.Second}).Limit(); got != time.Second {
		t.Errorf("заданный срок изменён: %s", got)
	}
}

// TestReadyCondition_Describe: сообщение об истёкшем сроке обязано называть,
// чего именно не дождались.
func TestReadyCondition_Describe(t *testing.T) {
	cases := map[string]*ReadyCondition{
		"tcp 127.0.0.1:5432":   {TCP: "127.0.0.1:5432"},
		`log line "ready"`:     {LogLine: "ready"},
		"exec [pg_isready -q]": {Exec: []string{"pg_isready", "-q"}},
	}

	for want, ready := range cases {
		if got := ready.Describe(); got != want {
			t.Errorf("Describe = %q, ожидалось %q", got, want)
		}
	}
}
