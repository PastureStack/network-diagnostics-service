#!/bin/sh
set -eu

unformatted=$(gofmt -l .)
if [ -n "$unformatted" ]; then
    echo "gofmt is required" >&2
    echo "$unformatted" >&2
    exit 1
fi

go test -shuffle=on -count=5 ./...
go vet ./...
go mod verify

mkdir -p bin
go build -trimpath -o bin/network-diagnostics-service-first ./cmd/network-diagnostics-service
go build -trimpath -o bin/network-diagnostics-service-second ./cmd/network-diagnostics-service
first=$(sha256sum bin/network-diagnostics-service-first | awk '{print $1}')
second=$(sha256sum bin/network-diagnostics-service-second | awk '{print $1}')
test "$first" = "$second"
echo "validation passed: $first"
