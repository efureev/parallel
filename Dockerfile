# syntax=docker/dockerfile:1

# Сборка. Версия Go берётся из go.mod аргументом, чтобы образ не разъезжался
# с тулчейном проекта молча.
ARG GO_VERSION=1.26
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

# Метаданные версии те же четыре переменные, что и в build.sh с build.yml.
# Путь к пакету зашит строкой и компилятором не проверяется: переименование
# internal/buildinfo соберётся молча, а `parallel -v` покажет `dev`.
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

# TARGETOS и TARGETARCH подставляет buildx — отсюда multi-arch без матрицы.
ARG TARGETOS
ARG TARGETARCH

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "-s -w \
      -X github.com/efureev/parallel/internal/buildinfo.Version=${VERSION} \
      -X github.com/efureev/parallel/internal/buildinfo.Commit=${COMMIT} \
      -X github.com/efureev/parallel/internal/buildinfo.Date=${BUILD_DATE} \
      -X github.com/efureev/parallel/internal/buildinfo.BuiltBy=docker" \
    -o /out/parallel ./cmd/parallel

# Проверка на месте сборки. Выполняется только когда целевая платформа совпадает
# со сборочной: кросс-скомпилированный бинарник здесь не запустится.
#
# 2>&1 обязателен: версия печатается через log, то есть в stderr, и без
# перенаправления grep читал бы пустой stdout.
RUN [ "${TARGETARCH}" != "$(go env GOARCH)" ] || \
    /out/parallel -v 2>&1 | grep -q "Version:   ${VERSION}" || \
    { echo "version not injected via ldflags"; exit 1; }

# Итоговый образ.
#
# Alpine, а не scratch и не distroless: утилита
# запускает чужие команды, а строковая форма `run:` и режим ad-hoc
# разворачиваются в `sh -c`.
FROM alpine:3.22

# ca-certificates нужны командам, которые ходят по HTTPS; tzdata — чтобы
# отметки времени в логе совпадали с ожиданиями пользователя, а не с UTC.
RUN apk add --no-cache ca-certificates tzdata

# Непривилегированный пользователь: утилита ничего не пишет в свои каталоги,
# а запускать чужие команды от root в контейнере незачем.
RUN adduser -D -u 10001 parallel

COPY --from=build /out/parallel /usr/local/bin/parallel

# /work — каталог, куда монтируется проект пользователя. Конфигурация ищется
# от рабочего каталога вверх по дереву, поэтому смонтированного проекта
# достаточно, путь указывать не обязательно.
WORKDIR /work

USER parallel

ENTRYPOINT ["parallel"]
