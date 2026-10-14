# Benchmarks for Qrysm State Transition
This package contains the functionality needed for benchmarking Qrysm state transitions, this includes its block processing (with and without caching) and epoch processing functions. There is also a benchmark for HashTreeRoot on a large beacon state.

## Benchmark Configuration
The following configs are in `config.go`:
* `ValidatorCount`: Sets the amount of active validators to perform the benchmarks with. Default is 16384.
* `AttestationsPerEpoch`: Sets the amount of attestations per epoch for the benchmarks to perform with, this affects the amount of attestations in a full block and the amount of attestations per epoch in the state for the `ProcessEpoch` and `HashTreeRoot` benchmark. Default is 128.

## Generating new SSZ files
Due to the sheer size of the benchmarking configurations (16384 validators), the files used for benchmarking are pregenerated so there's no wasted computations on generating a genesis state with 16384 validators. This should only be needed if there is a breaking spec change and the tests fail from SSZ issues.

The generator, `tools/benchmark-files-gen`, was never ported to qrysm (TQ-7) and has been removed. Restore it from the git history when the files need regenerating.

## Running the benchmarks
To run the ExecuteStateTransition benchmark:

```go test ./beacon-chain/core/transition -run=^$ -bench=BenchmarkExecuteStateTransition_FullBlock```

To run the ExecuteStateTransition (with cache) benchmark:

```go test ./beacon-chain/core/transition -run=^$ -bench=BenchmarkExecuteStateTransition_WithCache```

To run the ProcessEpoch benchmark:

```go test ./beacon-chain/core/transition -run=^$ -bench=BenchmarkProcessEpoch_2FullEpochs```

To run the HashTreeRoot benchmark:

```go test ./beacon-chain/core/transition -run=^$ -bench=BenchmarkHashTreeRoot_FullState```

To run the HashTreeRootState benchmark:

```go test ./beacon-chain/core/transition -run=^$ -bench=BenchmarkHashTreeRootState_FullState```

Extra flags needed to benchmark properly:

```-count=1 -v -timeout=2000s -cpuprofile=/tmp/cpu.profile -memprofile=/tmp/mem.profile```

## Current Results as of January 2020
```
BenchmarkExecuteStateTransition_FullBlock-4           20	  2031438030 ns/op
BenchmarkExecuteStateTransition_WithCache-4   	      20	  1857290454 ns/op
BenchmarkHashTreeRoot_FullState-4   	              50	   297655834 ns/op
BenchmarkHashTreeRootState_FullState-4                50           155535883 ns/op
```