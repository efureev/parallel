#!/usr/bin/env bash
# shellcheck disable=SC2043

# Build configuration constants
readonly APP_NAME="parallel"
readonly BUILD_DIR=${BUILD_PATH:-"build"}
readonly TARGET_OS=${GOOS:-$(go env GOHOSTOS)}
readonly TARGET_ARCH=${GOARCH:-$(go env GOHOSTARCH)}
readonly CGO_SETTING=${CGO_ENABLED:-0}

# arch_suffix переводит GOARCH в суффикс имени файла.
#
# Раньше суффикс был константой "x64" независимо от архитектуры: сборка под
# arm64 давала файл parallel.darwin.x64 с arm64-бинарником внутри. Схема имён
# теперь совпадает с матрицей .github/workflows/build.yml.
arch_suffix() {
    case "$1" in
        amd64) echo "x64" ;;
        *)     echo "$1" ;;
    esac
}

# Version metadata (can be overridden via environment)
VERSION=${VERSION:-"dev"}
COMMIT=${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "local")}
DATE=${DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}
BUILT_BY=${BUILT_BY:-"local"}

LD_FLAGS="-s -w -X github.com/efureev/parallel/internal/buildinfo.Version=${VERSION} -X github.com/efureev/parallel/internal/buildinfo.Commit=${COMMIT} -X github.com/efureev/parallel/internal/buildinfo.Date=${DATE} -X github.com/efureev/parallel/internal/buildinfo.BuiltBy=${BUILT_BY}"

print_build_info() {
    echo "Building options"
    echo "- TARGET_OS: $TARGET_OS"
    echo "- TARGET_ARCH: $TARGET_ARCH"
    echo " "
}

clean_build_directory() {
    rm -rf "./${BUILD_DIR}"
}

build_executable() {
    local suffix
    suffix=$(arch_suffix "$TARGET_ARCH")

    local executable_name="$APP_NAME.$TARGET_OS.$suffix"

    if [ "$TARGET_OS" = "windows" ]; then
        executable_name="$executable_name.exe"
    fi
    
    echo "Building: OS: $TARGET_OS ARCH: $TARGET_ARCH file: $executable_name"
    
    CGO_ENABLED=$CGO_SETTING \
    GOOS=$TARGET_OS \
    GOARCH=$TARGET_ARCH \
        go build -trimpath -ldflags "$LD_FLAGS" -o "$BUILD_DIR/$executable_name" ./cmd/parallel
}

main() {
    clean_build_directory
    print_build_info
    build_executable
    echo "Done!"
}

main "$@"