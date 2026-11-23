$ErrorActionPreference = 'Stop'

rtk go test ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

rtk go vet ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$originalGoos = $env:GOOS
$originalGoarch = $env:GOARCH
$originalCgo = $env:CGO_ENABLED

try {
    $env:GOOS = 'linux'
    $env:CGO_ENABLED = '0'

    foreach ($architecture in @('amd64', 'arm64')) {
        $env:GOARCH = $architecture

        rtk go build -o "dist/verify/hddmount-$architecture" ./cmd/hddmount
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

        Write-Output "Linux $architecture build passed"
    }
}
finally {
    $env:GOOS = $originalGoos
    $env:GOARCH = $originalGoarch
    $env:CGO_ENABLED = $originalCgo
}
