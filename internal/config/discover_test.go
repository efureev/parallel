package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeConfig кладёт минимальную рабочую конфигурацию по указанному пути.
func writeConfig(t *testing.T, path string) {
	t.Helper()

	if err := os.WriteFile(path, []byte("commands:\n  c:\n    x:\n      cmd: [ 'echo', 'ok' ]\n"), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestDiscover_WalksUp — конфигурация лежит в корне проекта, а команды
// запускают из любого подкаталога.
//
// Раньше поиск шёл только в текущем каталоге, и запуск из подкаталога падал
// с «config file not found» — при том, что файл в проекте есть.
func TestDiscover_WalksUp(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "services", "api", "internal")

	if err := os.MkdirAll(deep, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	want := filepath.Join(root, DefaultConfigName)
	writeConfig(t, want)

	got, err := Discover(deep)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}

	if got != want {
		t.Errorf("найдено %q, ожидалось %q", got, want)
	}
}

// TestDiscover_AcceptsYmlExtension: обе формы расширения одинаково
// распространены, и .parallelrc.yml раньше не находился вовсе.
func TestDiscover_AcceptsYmlExtension(t *testing.T) {
	dir := t.TempDir()
	want := filepath.Join(dir, ".parallelrc.yml")
	writeConfig(t, want)

	got, err := Discover(dir)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}

	if got != want {
		t.Errorf("найдено %q, ожидалось %q", got, want)
	}
}

// TestDiscover_PrefersNearestAndYaml — два правила приоритета сразу:
// ближний каталог важнее дальнего, а .yaml важнее .yml в одном каталоге.
func TestDiscover_PrefersNearestAndYaml(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "app")

	if err := os.MkdirAll(nested, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	writeConfig(t, filepath.Join(root, DefaultConfigName))
	writeConfig(t, filepath.Join(nested, ".parallelrc.yml"))

	nearest := filepath.Join(nested, DefaultConfigName)
	writeConfig(t, nearest)

	got, err := Discover(nested)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}

	if got != nearest {
		t.Errorf("найдено %q, ожидался ближайший .yaml %q", got, nearest)
	}
}

// TestDiscover_SkipsDirectoryWithConfigName: одноимённый каталог не должен
// обрывать поиск — иначе вместо подъёма выше пользователь получил бы
// невнятную ошибку чтения.
func TestDiscover_SkipsDirectoryWithConfigName(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "app")

	if err := os.MkdirAll(filepath.Join(nested, DefaultConfigName), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	want := filepath.Join(root, DefaultConfigName)
	writeConfig(t, want)

	got, err := Discover(nested)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}

	if got != want {
		t.Errorf("найдено %q, ожидалось %q", got, want)
	}
}

// TestDiscover_NotFoundExplainsSearch: дойдя до корня и не найдя ничего, утилита
// обязана объяснить, что именно она искала, — иначе непонятно, назвать файл
// иначе или положить в другое место.
func TestDiscover_NotFoundExplainsSearch(t *testing.T) {
	// Поиск по определению доходит до корня файловой системы, поэтому исход
	// зависит от машины: конфигурация в /tmp или / сделала бы находку законной.
	// Проверяем формулировку отказа, а не сам факт отказа.
	found, err := Discover(t.TempDir())
	if err == nil {
		t.Skipf("на этой машине конфигурация нашлась выше по дереву: %s", found)
	}

	if !errors.Is(err, ErrConfigNotFound) {
		t.Errorf("ошибка не опознаётся как ErrConfigNotFound: %v", err)
	}

	for _, want := range []string{DefaultConfigName, ".parallelrc.yml", "parent"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("сообщение не содержит %q: %s", want, err)
		}
	}
}

// TestDiscover_ResultFeedsBaseDir связывает поиск с разрешением dir: найденный
// наверху файл задаёт базу, от которой считаются относительные рабочие каталоги.
// Без этого конфигурация из корня проекта ломалась бы ровно так же, как раньше.
func TestDiscover_ResultFeedsBaseDir(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "cmd", "api")

	if err := os.MkdirAll(deep, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	path := filepath.Join(root, DefaultConfigName)
	if err := os.WriteFile(path,
		[]byte("commands:\n  c:\n    x:\n      cmd: [ 'echo', 'ok' ]\n      dir: 'web'\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	found, err := Discover(deep)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}

	data, err := NewFileLoader(YamlFileMarshaller{}).Load(found)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	result, err := NewFlowBuilder().Build(data)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if want := filepath.Join(root, "web"); result.Chains[0].Commands()[0].Dir != want {
		t.Errorf("dir = %q, ожидалось %q", result.Chains[0].Commands()[0].Dir, want)
	}
}
