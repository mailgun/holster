#!/bin/sh
# This script assumes that protoc is installed and available on the PATH.

# Make sure the script fails fast.
set -eux

SCRIPT_PATH=$(dirname "$0")              # relative
REPO_ROOT=$(cd "${SCRIPT_PATH}" && pwd)  # absolutized and normalized
SRC_DIR=$REPO_ROOT
GOLANG_DST_DIR=$SRC_DIR

# Build Golang stabs
go get -d google.golang.org/protobuf/cmd/protoc-gen-go
go install google.golang.org/protobuf/cmd/protoc-gen-go
go get -d google.golang.org/grpc/cmd/protoc-gen-go-grpc
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc
go install google.golang.org/protobuf/cmd/protoc-gen-go
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc
export PATH=$PATH:$(go env GOPATH)/bin
protoc -I=$SRC_DIR \
    --go_out=$GOLANG_DST_DIR \
    --go_opt=paths=source_relative \
    --go-grpc_out=$GOLANG_DST_DIR \
    --go-grpc_opt=paths=source_relative \
    $SRC_DIR/*.proto
