# End-to-end Testing Package

This is the main project folder of the end-to-end testing suite for Qrysm. This performs a full end-to-end test for Qrysm, including spinning up execution clients, sending deposits to the deposit contract, and making sure the beacon node and its validators are running and performing properly for a few epochs.
It also performs a test on a syncing node, and supports feature flags to allow easy E2E testing of experimental features. 

## How it works

// TODO(now.youtrack.cloud/issue/TQ-1)
Please see our docs page, https://docs.prylabs.network/docs/devtools/end-to-end, to read more about the feature.

## Running the tests

```bash
make e2e [minimal slashing slasher]
```

`make e2e` (default: `minimal`) builds `beacon-chain`, `validator`, `bootnode` and `gqrl` into `dist/`, with `-tags minimal` for the minimal-config scenarios, and runs each scenario with `go test` against them, one at a time. `gqrl` is built from the go-qrl version `go.mod` selects, through its `replace`. Logs go to a fresh `qrysm-e2e-logs-*` directory in the system temporary directory, printed at the start of each scenario.

| Scenario   | Test                                     |
|------------|------------------------------------------|
| `minimal`  | `TestEndToEnd_MinimalConfig`             |
| `slashing` | `TestEndToEnd_Slasher_MinimalConfig`     |
| `slasher`  | `TestEndToEnd_SlasherSimulator`          |

The other tests of this package (`TestEndToEnd_MinimalConfig_WithBuilder`, `TestEndToEnd_MainnetConfig*`, `TestEndToEnd_MultiScenarioRun*`, `TestEndToEnd_MinimalConfig_Web3Signer`, `TestEndToEnd_MinimalConfig_ValidatorRESTApi`) are not wired to `make e2e`: they also need a web3signer binary or a builder, which were never provided.
