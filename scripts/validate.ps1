$ErrorActionPreference = 'Stop'

$unformatted = gofmt -l .
if ($unformatted) {
    throw "gofmt is required: $($unformatted -join ', ')"
}

go test -shuffle=on -count=5 ./...
go vet ./...
go mod verify

New-Item -ItemType Directory -Force -Path bin | Out-Null
go build -trimpath -o bin/network-diagnostics-service-first.exe ./cmd/network-diagnostics-service
go build -trimpath -o bin/network-diagnostics-service-second.exe ./cmd/network-diagnostics-service

$first = (Get-FileHash -Algorithm SHA256 -LiteralPath bin/network-diagnostics-service-first.exe).Hash
$second = (Get-FileHash -Algorithm SHA256 -LiteralPath bin/network-diagnostics-service-second.exe).Hash
if ($first -ne $second) {
    throw 'reproducible build check failed'
}

Write-Output "validation passed: $first"
