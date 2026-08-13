package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoad_ResolvesDirAgainstConfigFile — относительный рабочий каталог должен
// разрешаться от файла конфигурации, а не от текущего каталога процесса.
//
// Иначе конфигурация не самодостаточна: она работает только если её запускают
// из «правильного» места, хотя лежит рядом с проектом и коммитится вместе с ним.
func TestLoad_ResolvesDirAgainstConfigFile(t *testing.T) {
	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, "flow.yaml")

	// В YAML путь пишется через прямые слэши: на Windows обратный слэш внутри
	// кавычек читается неоднозначно. Сравнивать результат надо с этой же
	// строкой, а не с projectDir, — иначе на Windows тест сверял бы C:/x с C:\\x
	// и падал, хотя проверяется совсем другое: что абсолютный путь не тронули.
	absDir := filepath.ToSlash(projectDir)

	raw := "commands:\n  c:\n    rel:\n      cmd: [ 'echo', 'x' ]\n      dir: 'sub'\n" +
		"    abs:\n      cmd: [ 'echo', 'x' ]\n      dir: '" + absDir + "'\n" +
		"    none:\n      cmd: [ 'echo', 'x' ]\n"

	if err := os.WriteFile(configPath, []byte(raw), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Текущий каталог теста заведомо не совпадает с каталогом конфигурации.
	data, err := NewFileLoader(YamlFileMarshaller{}).Load(configPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	result, err := NewFlowBuilder().Build(data)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	dirs := map[string]string{}
	for _, cmd := range result.Chains[0].Commands() {
		dirs[cmd.Name] = cmd.Dir
	}

	if want := filepath.Join(projectDir, "sub"); dirs["rel"] != want {
		t.Errorf("относительный dir = %q, want %q", dirs["rel"], want)
	}

	if dirs["abs"] != absDir {
		t.Errorf("абсолютный dir изменён: %q, want %q", dirs["abs"], absDir)
	}

	if dirs["none"] != "" {
		t.Errorf("пустой dir не должен заполняться: %q", dirs["none"])
	}
}

// TestBuild_WithoutBaseDirKeepsDirAsIs: конфигурация, собранная в памяти
// (например, в тесте), базы не имеет — путь остаётся как записан.
func TestBuild_WithoutBaseDirKeepsDirAsIs(t *testing.T) {
	data := Data{
		Chains: []ChainConfig{
			{
				Name:     "c",
				Commands: []NamedCommand{{Name: "x", Spec: command{Cmd: []string{"echo"}, Dir: "sub"}}},
			},
		},
	}

	result, err := NewFlowBuilder().Build(data)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if got := result.Chains[0].Commands()[0].Dir; got != "sub" {
		t.Errorf("dir = %q, want %q", got, "sub")
	}
}
