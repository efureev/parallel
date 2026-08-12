package config

import (
	"strings"
	"testing"
)

// TestUnmarshal_RejectsUnknownField — опечатка в имени поля обязана быть ошибкой.
//
// Худший вид ошибки конфигурации — та, что не падает: `pipeline` вместо `pipe`
// принимался молча, команда выполнялась не в потоковом режиме, и узнать об этом
// можно было только по косвенным признакам.
func TestUnmarshal_RejectsUnknownField(t *testing.T) {
	tests := []struct {
		name  string
		field string
		// wantHint — ближайшее известное поле; пусто, если подсказка не нужна.
		wantHint string
	}{
		{name: "дописанное окончание", field: "pipeline: true", wantHint: `"pipe"`},
		{name: "лишняя буква", field: "diir: '/tmp'", wantHint: `"dir"`},
		{name: "перестановка", field: "dokcer: {}", wantHint: `"docker"`},
		{name: "непохожее слово", field: "xyz: 1", wantHint: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := []byte("commands:\n  c:\n    t:\n      cmd: [ 'echo', 'hi' ]\n      " + tt.field + "\n")

			_, err := YamlFileMarshaller{}.Unmarshal(raw)
			if err == nil {
				t.Fatalf("опечатка %q принята без ошибки", tt.field)
			}

			msg := err.Error()

			// Контекст обязателен: без имени команды и цепочки искать придётся глазами.
			for _, want := range []string{`command "t"`, `chain "c"`, "unknown field"} {
				if !strings.Contains(msg, want) {
					t.Errorf("сообщение не содержит %q:\n%s", want, msg)
				}
			}

			if tt.wantHint == "" {
				if strings.Contains(msg, "имелось в виду") {
					t.Errorf("для непохожего слова подсказка вредна:\n%s", msg)
				}

				return
			}

			if !strings.Contains(msg, tt.wantHint) {
				t.Errorf("нет подсказки %s:\n%s", tt.wantHint, msg)
			}
		})
	}
}

// TestUnmarshal_AcceptsAllKnownFields — обратная сторона строгости: ни одно
// настоящее поле не должно оказаться отвергнутым.
func TestUnmarshal_AcceptsAllKnownFields(t *testing.T) {
	raw := []byte(`
commands:
  regular:
    full:
      cmd: [ 'echo', 'hi' ]
      dir: '/tmp'
      pipe: true
      disable: false
      env:
        KEY: value
      format:
        cmdName: '%CMD_NAME%'
  containers:
    ng:
      docker:
        image:
          name: nginx
          tag: alpine
          pull: always
        ports: [ '80:80' ]
        removeAfterAll: false
        cmd: run
`)

	cfg, err := YamlFileMarshaller{}.Unmarshal(raw)
	if err != nil {
		t.Fatalf("настоящее поле отвергнуто: %v", err)
	}

	if len(cfg.Chains) != 2 {
		t.Fatalf("ожидалось 2 цепочки, получено %d", len(cfg.Chains))
	}
}

// TestKnownCommandFieldsMatchSchema сторожит рассинхрон: список для подсказок
// заполняется вручную, и добавленное в структуру поле легко забыть.
func TestKnownCommandFieldsMatchSchema(t *testing.T) {
	for _, field := range knownCommandFields {
		raw := []byte("commands:\n  c:\n    t:\n      " + field + ": null\n")

		_, err := YamlFileMarshaller{}.Unmarshal(raw)
		if err != nil && strings.Contains(err.Error(), "unknown field") {
			t.Errorf("поле %q есть в списке подсказок, но схемой не принимается", field)
		}
	}
}
