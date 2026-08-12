package flow

import (
	"os"
)

// PathExists сообщает, существует ли путь и доступен ли он для stat.
// Любая ошибка (в т.ч. отсутствие файла или нехватка прав) трактуется как «не существует».
func PathExists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}
