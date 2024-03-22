#!/usr/bin/env bash
# shellcheck disable=SC2043

APP_NAME="parallel"

BUILD_PATH=${BUILD_PATH:-"build"}

#VERSION_COMMIT=$(git log --pretty="%h" -n1 HEAD)
#VERSION_DEFAULT=$(git tag --sort=-v:refname --list "v[0-9]*" | head -n 1)
#VERSION=${VERSION:-$VERSION_DEFAULT}

rm -rf "./${BUILD_PATH}"

echo "Building options"
echo "- VERSION: $VERSION"
echo "- COMMIT: $VERSION_COMMIT"
echo " "

#BUILDING_FLAGS="\
#    -X $NS/$SLUG/src/config.version='$VERSION-$VERSION_COMMIT' \
#"

#for OS in darwin linux; do
for OS in darwin; do
  for ARCH in amd64; do
    ARCHX=x86
    if [ $ARCH == "amd64" ]; then
      ARCHX=x64
    fi
    CURR_NAME="$APP_NAME.$OS.$ARCHX"
    echo "Building: OS: $OS ARCH: $ARCH file: $CURR_NAME"

    CGO_ENABLED=0 GOOS=$OS GOARCH=$ARCH go build -a -installsuffix cgo -ldflags="$BUILDING_FLAGS" \
      -o "$BUILD_PATH/$CURR_NAME"
  done
done

echo "Done!"
