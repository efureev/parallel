// Command parallel запускает цепочки консольных команд параллельно
// с читаемым цветным выводом и корректным завершением по сигналу.
package main

import (
	"os"

	"github.com/efureev/parallel/internal/cli"
)

func main() {
	os.Exit(cli.Run())
}
