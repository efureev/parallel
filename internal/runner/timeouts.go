package runner

import "time"

// Timeouts — тайминги завершения дочернего процесса.
//
// Раньше это были константы, и тест на принудительное убийство честно ждал
// три секунды — 3.3 с из 5.4 с всего прогона. Теперь значения
// задаются при создании менеджера: прод берёт умолчания, тесты — миллисекунды.
type Timeouts struct {
	// ForceKill — сколько ждать после сигнала завершения, прежде чем убивать группу.
	ForceKill time.Duration
	// Drain — сколько ждать дочитывания пайпов после убийства группы.
	//
	// Нужен на случай, когда открытый конец пайпа удерживает кто-то, кого
	// убийство группы не задело: отцепившийся внук процесса. Без этого предела
	// чтение до EOF могло бы не завершиться никогда.
	Drain time.Duration
}

// Значения по умолчанию.
const (
	defaultForceKillTimeout = 3 * time.Second
	defaultDrainTimeout     = 2 * time.Second
)

// DefaultTimeouts возвращает тайминги для обычной работы утилиты.
func DefaultTimeouts() Timeouts {
	return Timeouts{
		ForceKill: defaultForceKillTimeout,
		Drain:     defaultDrainTimeout,
	}
}

// normalize подставляет умолчания вместо неположительных значений.
func (t Timeouts) normalize() Timeouts {
	def := DefaultTimeouts()

	if t.ForceKill <= 0 {
		t.ForceKill = def.ForceKill
	}

	if t.Drain <= 0 {
		t.Drain = def.Drain
	}

	return t
}
