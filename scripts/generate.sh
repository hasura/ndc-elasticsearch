#!/usr/bin/env bash

VERSION="${VERSION:-v1.0.0}"
EXEC_DIR=".codegen/$VERSION"

case $(uname -m) in
    x86_64)   ARCH="amd64" ;;
    arm)      ARCH="arm64" ;;
    aarch64)  ARCH="arm64" ;;
    *) 
      echo "unsupported archtecture $(uname -m)"
      exit 1
    ;;
esac

if [ "$(uname)" == "Darwin" ]; then
  PLATFORM=darwin
elif [ "$(expr substr $(uname -s) 1 5)" == "Linux" ]; then
  PLATFORM=linux
else
  echo "unsupported platform $(uname -s). If you are using Windows, download the exe file at https://github.com/hasura/ndc-sdk-go/releases"
  exit 1
fi

EXEC_FILE="$EXEC_DIR/hasura-ndc-go-$PLATFORM-$ARCH"

if [ ! -f $EXEC_FILE ]; then
  echo "hasura-ndc-go does not exist, downloading...."
  mkdir -p "$EXEC_DIR"
  curl -o $EXEC_FILE -L "https://github.com/hasura/ndc-sdk-go/releases/download/$VERSION/hasura-ndc-go-$PLATFORM-$ARCH"
  chmod +x $EXEC_FILE
fi

$EXEC_FILE generate