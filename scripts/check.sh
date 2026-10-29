#!/bin/sh
set -eu
cd "$(dirname "$0")/../backend"
unformatted="$(gofmt -l .)"
if [ -n "$unformatted" ]; then
  printf 'Run gofmt on:\n%s\n' "$unformatted" >&2
  exit 1
fi
go vet -mod=readonly ./...
go test -mod=readonly -race -count=1 ./...
mkdir -p build
go build -mod=readonly -trimpath -o build/futryva .
