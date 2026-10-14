# Spec Tests

Spec testing vectors: https://github.com/ethereum/consensus-spec-tests

The spec tests of `mainnet/` and `minimal/` are disabled until their test vectors are published for QRL (TQ-4). Once they are, `go test -tags develop ./testing/spectest/mainnet/...` runs the mainnet ones and `go test -tags develop,minimal ./testing/spectest/minimal/...` the minimal ones; `make test mainnet` already includes the mainnet packages.
