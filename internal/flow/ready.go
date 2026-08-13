package flow

import (
	"errors"
	"fmt"
	"time"
)

var (
	// ErrReadyEmpty — секция ready не задаёт ни одного условия.
	ErrReadyEmpty = errors.New("ready must define one of tcp, exec or logLine")
	// ErrReadyAmbiguous — задано больше одного условия сразу.
	ErrReadyAmbiguous = errors.New("ready must define exactly one condition")
)

// DefaultReadyTimeout — сколько ждать готовности, если срок не задан.
const DefaultReadyTimeout = 30 * time.Second

// ReadyCondition описывает, по какому признаку команда считается готовой.
//
// Домен только описывает условие; как его проверять — дело раннера. Здесь нет
// ни сокетов, ни запуска процессов, иначе слой перестал бы быть доменным.
type ReadyCondition struct {
	// TCP — адрес вида host:port, который должен начать принимать соединения.
	TCP string
	// Exec — команда, которую надо повторять до нулевого кода возврата.
	Exec []string
	// LogLine — подстрока, появление которой в выводе означает готовность.
	LogLine string
	// Timeout ограничивает ожидание; ноль означает DefaultReadyTimeout.
	Timeout time.Duration
}

// Validate проверяет, что задано ровно одно условие.
//
// Ни одного — почти наверняка недосмотр при правке, и трактовать это как
// «готова сразу» значило бы тихо снять ожидание. Больше одного — неясно, какое
// из них главное, и выбирать за пользователя нельзя.
func (r *ReadyCondition) Validate() error {
	if r == nil {
		return nil
	}

	defined := 0

	for _, set := range []bool{r.TCP != "", len(r.Exec) > 0, r.LogLine != ""} {
		if set {
			defined++
		}
	}

	switch {
	case defined == 0:
		return ErrReadyEmpty
	case defined > 1:
		return fmt.Errorf("%w, got %d", ErrReadyAmbiguous, defined)
	}

	if r.Timeout < 0 {
		return fmt.Errorf("%w: ready timeout is %s", ErrNegativeTimeout, r.Timeout)
	}

	return nil
}

// Limit возвращает срок ожидания с учётом умолчания.
func (r *ReadyCondition) Limit() time.Duration {
	if r == nil || r.Timeout <= 0 {
		return DefaultReadyTimeout
	}

	return r.Timeout
}

// Describe коротко называет условие — для сообщений об истёкшем сроке.
func (r *ReadyCondition) Describe() string {
	switch {
	case r == nil:
		return "none"
	case r.TCP != "":
		return fmt.Sprintf("tcp %s", r.TCP)
	case len(r.Exec) > 0:
		return fmt.Sprintf("exec %v", r.Exec)
	case r.LogLine != "":
		return fmt.Sprintf("log line %q", r.LogLine)
	default:
		return "none"
	}
}
