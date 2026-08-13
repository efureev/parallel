package config

import (
	"strings"
	"testing"
)

// TestUnmarshal_FailFastKey — три состояния ключа, а не два: отсутствие ключа
// и явное false — разные вещи, потому что флаг командной строки сильнее только
// первого.
func TestUnmarshal_FailFastKey(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want *bool
	}{
		{name: "ключа нет", raw: "commands:\n  c:\n    x:\n      cmd: [ 'echo' ]\n"},
		{name: "false", raw: "failFast: false\ncommands:\n  c:\n    x:\n      cmd: [ 'echo' ]\n", want: ptr(false)},
		{name: "true", raw: "failFast: true\ncommands:\n  c:\n    x:\n      cmd: [ 'echo' ]\n", want: ptr(true)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := YamlFileMarshaller{}.Unmarshal([]byte(tt.raw))
			if err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			switch {
			case tt.want == nil && cfg.FailFast != nil:
				t.Errorf("отсутствие ключа дало %v, ожидался nil", *cfg.FailFast)
			case tt.want != nil && cfg.FailFast == nil:
				t.Error("ключ есть, но значение не разобрано")
			case tt.want != nil && *cfg.FailFast != *tt.want:
				t.Errorf("failFast = %v, ожидалось %v", *cfg.FailFast, *tt.want)
			}
		})
	}
}

func ptr(b bool) *bool { return &b }

// TestUnmarshal_FailFastWrongType: строка вместо булева значения — ошибка,
// а не молчаливое false.
func TestUnmarshal_FailFastWrongType(t *testing.T) {
	raw := []byte("failFast: maybe\ncommands:\n  c:\n    x:\n      cmd: [ 'echo' ]\n")

	_, err := (YamlFileMarshaller{}).Unmarshal(raw)
	if err == nil {
		t.Fatal("нечисловое значение принято без ошибки")
	}
}

// TestUnmarshal_TopLevelHints — опечатка в ключе верхнего уровня иначе не делает
// ничего и об этом не сообщает.
//
// Строгость здесь недоступна: верхний уровень исторически принимал что угодно,
// и запрет сломал бы конфигурации с YAML-якорями. Поэтому предупреждение, и
// только для похожих ключей.
func TestUnmarshal_TopLevelHints(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		wantHint string
	}{
		{name: "опечатка в failFast", key: "failFats", wantHint: "failFast"},
		{name: "опечатка в commands", key: "commnads", wantHint: "commands"},
		{name: "якорь молчит", key: "x-common", wantHint: ""},
		{name: "подчёркивание молчит", key: "_defaults", wantHint: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := []byte(tt.key + ": false\ncommands:\n  c:\n    x:\n      cmd: [ 'echo' ]\n")

			cfg, err := YamlFileMarshaller{}.Unmarshal(raw)
			if err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			joined := strings.Join(cfg.TopLevelHints, "; ")

			if tt.wantHint == "" {
				if joined != "" {
					t.Errorf("лишнее предупреждение для %q: %s", tt.key, joined)
				}

				return
			}

			if !strings.Contains(joined, tt.wantHint) || !strings.Contains(joined, tt.key) {
				t.Errorf("для %q ожидалась подсказка про %q, получено: %s", tt.key, tt.wantHint, joined)
			}
		})
	}
}

// TestUnmarshal_KnownTopLevelKeysAreSilent — обратная сторона: настоящие ключи
// предупреждений давать не должны.
func TestUnmarshal_KnownTopLevelKeysAreSilent(t *testing.T) {
	raw := []byte("failFast: false\ncommands:\n  c:\n    x:\n      cmd: [ 'echo' ]\n")

	cfg, err := YamlFileMarshaller{}.Unmarshal(raw)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(cfg.TopLevelHints) != 0 {
		t.Errorf("предупреждения на известных ключах: %v", cfg.TopLevelHints)
	}
}
