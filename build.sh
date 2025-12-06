#!/usr/bin/env bash
# shellcheck disable=SC2043

# Build configuration constants
readonly APP_NAME="parallel"
readonly BUILD_DIR=${BUILD_PATH:-"build"}
readonly TARGET_OS="darwin"
readonly TARGET_ARCH="amd64"
readonly MAPPED_ARCH="x64"
readonly CGO_SETTING=0

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
    local executable_name="$APP_NAME.$TARGET_OS.$MAPPED_ARCH"
    
    echo "Building: OS: $TARGET_OS ARCH: $TARGET_ARCH file: $executable_name"
    
    CGO_ENABLED=$CGO_SETTING \
    GOOS=$TARGET_OS \
    GOARCH=$TARGET_ARCH \
        go build -a -installsuffix cgo -o "$BUILD_DIR/$executable_name"
}

main() {
    clean_build_directory
    print_build_info
    build_executable
    echo "Done!"
}

main "$@"