# Running analyzer unit tests

The analyzers run through `make lint` (see `tools/qrysm-vet`). Their unit tests run with the other unit tests
(`make test mainnet`), or alone with `go test ./tools/analyzers/...`.