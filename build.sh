#!/usr/bin/env bash
# shellcheck disable=SC2043

# Build configuration
readonly APP_NAME="parallel"
readonly BUILD_DIR=${BUILD_PATH:-"build"}
readonly TARGET_OS="darwin"
readonly TARGET_ARCH="amd64"
readonly MAPPED_ARCH="x64"
readonly CGO_SETTING=0

# Clean previous build
rm -rf "./${BUILD_DIR}"

echo "Building options"
echo "- TARGET_OS: $TARGET_OS"
echo "- TARGET_ARCH: $TARGET_ARCH"
echo " "

# Build for target platform
EXECUTABLE_NAME="$APP_NAME.$TARGET_OS.$MAPPED_ARCH"
echo "Building: OS: $TARGET_OS ARCH: $TARGET_ARCH file: $EXECUTABLE_NAME"

CGO_ENABLED=$CGO_SETTING GOOS=$TARGET_OS GOARCH=$TARGET_ARCH \
  go build -a -installsuffix cgo \
  -o "$BUILD_DIR/$EXECUTABLE_NAME"

echo "Done!"