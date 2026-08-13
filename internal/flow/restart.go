package flow

import (
	"fmt"
	"strings"
)

// RestartPolicy — правило повторного запуска команды.
//
// Строковый тип, а не число: значение приходит из YAML и в нём же показывается
// пользователю в предпросмотре, поэтому переводить его туда-обратно незачем.
type RestartPolicy string

// Политики перезапуска.
const (
	// RestartNever — умолчание: команда выполняется ровно один раз.
	RestartNever RestartPolicy = "never"
	// RestartOnFailure — перезапуск после любого неуспеха, включая снятие
	// по таймауту: таймаут это отказ, отдельного правила для него нет.
	RestartOnFailure RestartPolicy = "on-failure"
	// RestartAlways — перезапуск и после успешного завершения. В этом всё
	// отличие от RestartOnFailure.
	RestartAlways RestartPolicy = "always"
)

// restartPolicies перечисляет допустимые значения в порядке возрастания
// «настойчивости» — в этом же порядке они показываются в сообщении об ошибке.
//
//nolint:gochecknoglobals // неизменяемый список, массивом объявить нельзя
var restartPolicies = []RestartPolicy{RestartNever, RestartOnFailure, RestartAlways}

// String возвращает имя политики в том виде, в каком его пишут в конфигурации.
func (p RestartPolicy) String() string {
	if p == "" {
		return string(RestartNever)
	}

	return string(p)
}

// ShouldRestart решает, запускать ли команду заново после такого исхода.
func (p RestartPolicy) ShouldRestart(err error) bool {
	switch p {
	case RestartAlways:
		return true
	case RestartOnFailure:
		return err != nil
	case RestartNever:
		return false
	default:
		return false
	}
}

// ParseRestartPolicy разбирает значение поля restart.
//
// Пустая строка — это отсутствие поля, а не ошибка: политика по умолчанию
// «не перезапускать».
func ParseRestartPolicy(s string) (RestartPolicy, error) {
	if s == "" {
		return RestartNever, nil
	}

	policy := RestartPolicy(s)
	for _, known := range restartPolicies {
		if policy == known {
			return policy, nil
		}
	}

	return "", fmt.Errorf("%w %q, allowed: %s", ErrUnknownRestartPolicy, s, strings.Join(policyNames(), ", "))
}

// policyNames возвращает допустимые значения строками — для сообщения об ошибке.
func policyNames() []string {
	names := make([]string, 0, len(restartPolicies))
	for _, p := range restartPolicies {
		names = append(names, string(p))
	}

	return names
}
