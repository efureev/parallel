#!/usr/bin/env bash
#
# Проверка покрытия по пакетам против .coverage-thresholds.
#
# Общий порог на репозиторий скрывает именно то, что важно: падение покрытия
# домена компенсируется ростом в другом месте, и цифра остаётся зелёной.
# Поэтому планка у каждого пакета своя.

set -euo pipefail

THRESHOLDS="${THRESHOLDS:-.coverage-thresholds}"
PROFILE="${PROFILE:-coverage.out}"
MODULE="github.com/efureev/parallel"

if [[ ! -f "$THRESHOLDS" ]]; then
  echo "не найден файл порогов: $THRESHOLDS" >&2
  exit 2
fi

# -covermode=atomic, а не set: пакеты насквозь конкурентные, и при -race
# счётчики без атомарности дают неверные числа.
go test ./internal/... -covermode=atomic -coverprofile="$PROFILE" >/dev/null

# Покрытие считается ПО ОПЕРАТОРАМ из самого профиля, а не усреднением
# процентов по функциям: маленькая функция весит там столько же, сколько
# большая, и число расходится с тем, что показывает `go test -cover`.
#
# Строка профиля: файл:начало,конец <число операторов> <счётчик>
coverage_of() {
  local pkg="$1"
  awk -v pkg="$MODULE/$pkg/" '
      index($1, pkg) == 1 {
        total += $2
        if ($3 > 0) { covered += $2 }
      }
      END { if (total > 0) printf "%.1f", 100 * covered / total; else print "n/a" }
    ' "$PROFILE"
}

failed=0

while read -r pkg want; do
  [[ -z "$pkg" || "$pkg" == \#* ]] && continue

  got=$(coverage_of "$pkg")

  if [[ "$got" == "n/a" ]]; then
    printf '  %-20s НЕТ ДАННЫХ (пакет отсутствует в профиле)\n' "$pkg"
    failed=1
    continue
  fi

  if awk -v g="$got" -v w="$want" 'BEGIN { exit !(g + 0 < w + 0) }'; then
    printf '  %-20s %5s%%  < %s%%  ПОРОГ\n' "$pkg" "$got" "$want"
    failed=1
  else
    printf '  %-20s %5s%%  ≥ %s%%\n' "$pkg" "$got" "$want"
  fi
done < "$THRESHOLDS"

if [[ "$failed" -ne 0 ]]; then
  echo >&2
  echo "покрытие ниже порога. Либо верните тесты, либо правьте .coverage-thresholds" >&2
  echo "тем же коммитом — с объяснением, почему планка опущена." >&2
  exit 1
fi
