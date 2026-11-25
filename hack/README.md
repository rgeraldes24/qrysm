# Bash Scripts

This subproject contains useful bash scripts for working with our repository. We have a simple tool that outputs coverage, release and docker image publishing helpers, and more.

## Generated code

The `*.pb.go`, `*.pb.gw.go`, SSZ (`generated.ssz.go`) and mock files are generated with `make gen` (see `build/gen`).
Files differing between the mainnet and the minimal configurations come in pairs: `<name>.pb.go`
(`//go:build !minimal`) and `<name>.minimal.pb.go` (`//go:build minimal`), likewise for `generated.ssz.go`.
