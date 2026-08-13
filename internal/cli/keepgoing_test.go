package cli

import "testing"

// TestResolveKeepGoing — таблица приоритетов целиком.
//
// Явный флаг сильнее файла: он относится к конкретному запуску, а файл — к
// проекту. Отсюда же `-keep-going=false` как способ вернуть fail-fast, когда
// в конфигурации стоит failFast: false.
func TestResolveKeepGoing(t *testing.T) {
	yes, no := true, false

	tests := []struct {
		name      string
		flagSet   bool
		flagValue bool
		failFast  *bool
		want      bool
	}{
		{name: "ничего не задано — как раньше", want: false},
		{name: "failFast: false в файле", failFast: &no, want: true},
		{name: "failFast: true в файле", failFast: &yes, want: false},
		{name: "флаг без файла", flagSet: true, flagValue: true, want: true},
		{name: "флаг перевешивает файл", flagSet: true, flagValue: true, failFast: &yes, want: true},
		{
			name:    "-keep-going=false возвращает fail-fast",
			flagSet: true, flagValue: false, failFast: &no, want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := &Config{KeepGoing: tt.flagValue, KeepGoingSet: tt.flagSet}

			if got := resolveKeepGoing(flags, tt.failFast); got != tt.want {
				t.Errorf("resolveKeepGoing = %v, ожидалось %v", got, tt.want)
			}
		})
	}
}

// TestParseFlags_KeepGoing: без различия «не передавали» и «передали false»
// флаг не смог бы перевесить ключ конфигурации.
func TestParseFlags_KeepGoing(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantSet   bool
		wantValue bool
	}{
		{name: "не передан", args: nil},
		{name: "передан", args: []string{"-keep-going"}, wantSet: true, wantValue: true},
		{name: "передан false", args: []string{"-keep-going=false"}, wantSet: true, wantValue: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := parseArgs(t, tt.args...)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			if cfg.KeepGoingSet != tt.wantSet || cfg.KeepGoing != tt.wantValue {
				t.Errorf("set=%v value=%v, ожидалось set=%v value=%v",
					cfg.KeepGoingSet, cfg.KeepGoing, tt.wantSet, tt.wantValue)
			}
		})
	}
}
