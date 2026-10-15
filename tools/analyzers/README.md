# Running analyzer unit tests

The analyzers' unit tests run with the other unit tests (`make test mainnet`). To run only them:
`go test ./tools/analyzers/...`

`make lint` runs the analyzers on the code through `tools/qrysm-vet`.