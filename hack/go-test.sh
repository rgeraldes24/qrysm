#!/usr/bin/env bash
#
# Build, vet and test the Go packages of this repository with the same
# settings CI uses. Usage:
#
#   hack/go-test.sh build   # go build of every package that is expected to build
#   hack/go-test.sh vet     # go vet of the same packages
#   hack/go-test.sh test    # go test of the same packages
#   hack/go-test.sh all     # all of the above, in that order
#
# Packages that do not build with plain `go build` (tools that only build
# under Bazel because their `main` lives in generated code), end-to-end tests
# that need pre-built node binaries, and the Bazel self-test are excluded.
# Tests that are known to fail outside Bazel are skipped by name below so that
# they can be re-enabled one by one.
#
# `go test ./...` builds and runs one test binary per CPU, each of which may
# spin up libp2p hosts, beacon states and databases; on a machine without
# much free memory that can exhaust RAM. Serialize the work with:
#
#   GOMAXPROCS=2 GOFLAGS=-p=1 GO_TEST_FLAGS='-parallel 1' hack/go-test.sh all
#
# GO_TEST_FLAGS is appended verbatim to every `go test` invocation.
set -euo pipefail

cd "$(dirname "$0")/.."

# Packages excluded from build / vet / test.
EXCLUDED_PACKAGES=(
    # Their main lives in Bazel-generated code; `go build` reports
    # "function main is undeclared in the main package".
    github.com/theQRL/qrysm/pkg/tx-fuzz/cmd/london
    github.com/theQRL/qrysm/tools/benchmark-files-gen
    github.com/theQRL/qrysm/tools/forkchecker
    github.com/theQRL/qrysm/tools/interop/split-keys
    github.com/theQRL/qrysm/tools/qcli
    # Needs beacon-chain / validator / execution binaries built beforehand.
    github.com/theQRL/qrysm/testing/endtoend
    # Bazel self-test.
    github.com/theQRL/qrysm/build/bazel
)

# Packages whose tests refuse to run without the `develop` build tag.
DEVELOP_TAG_PACKAGES=(
    github.com/theQRL/qrysm/beacon-chain/blockchain
    github.com/theQRL/qrysm/config/params
)

# Tests known to fail with `go test` (they pass under Bazel, whose minimal
# preset / sandbox they depend on, or depend on the environment's umask).
# Keep this list short; remove entries as the tests are fixed.
SKIP_TESTS=(
    # beacon-chain/rpc/qrysm/v1alpha1/{beacon,validator}: minimal-preset fixtures
    # run against the mainnet SSZ layout.
    'TestServer_ListAssignments_'
    'TestServer_ListBeaconCommittees_(Current|Previous)Epoch'
    'TestServer_ListIndexedAttestations_OldEpoch'
    'TestRetrieveCommitteesForRoot'
    'TestServer_GetIndividualVotes_(ValidatorsDontExist|Working$)'
    'TestServer_GetValidatorActiveSetChanges$'
    'TestServer_GetValidatorParticipation_(CurrentAndPrevEpochWithBits|OrphanedUntilGenesis)'
    'TestServer_ListValidatorBalances_(NoResults|DefaultResponse_NoArchive|Pagination|OutOfRange|UnknownValidatorInResponse)'
    'TestServer_ListValidators_(NoResults|OnlyActiveValidators|InactiveInTheMiddle|Pagination$|FromOldEpoch|ProcessHeadStateSlots)'
    'TestProposer_GetSyncAggregate_OK'
    'TestServer_GetAttestationData_HeadStateSlotGreaterThanRequestSlot'
    'TestServer_GetBeaconBlock_Zond'
    'TestServer_Stream(Altair|Zond)Blocks(Verified)?_OnHeadUpdated'
    # umask-dependent file permission expectations.
    'TestHandleBackupDir_AlreadyExists_'
    'TestExitAccountsCli_WriteJSON_NoBroadcast'
    'TestWriteSignedVoluntaryExitJSON'
)

skip_regex=$(IFS='|'; echo "${SKIP_TESTS[*]}")

list_packages() {
    local exclude
    exclude=$(printf '%s\n' "${EXCLUDED_PACKAGES[@]}" "${DEVELOP_TAG_PACKAGES[@]}")
    go list ./... | grep -v -x -F "$exclude"
}

# Packages with non-test Go files only: `go build` rejects test-only packages.
list_buildable_packages() {
    local exclude
    exclude=$(printf '%s\n' "${EXCLUDED_PACKAGES[@]}" "${DEVELOP_TAG_PACKAGES[@]}")
    go list -f '{{if or .GoFiles .CgoFiles}}{{.ImportPath}}{{end}}' ./... | grep -v -x -F "$exclude"
}

cmd_build() {
    echo "--- go build"
    list_buildable_packages | xargs go build
    go build -tags develop "${DEVELOP_TAG_PACKAGES[@]}"
}

cmd_vet() {
    echo "--- go vet"
    list_packages | xargs go vet
    go vet -tags develop "${DEVELOP_TAG_PACKAGES[@]}"
}

cmd_test() {
    local -a extra_test_flags=()
    read -ra extra_test_flags <<<"${GO_TEST_FLAGS:-}"
    echo "--- go test"
    list_packages | xargs go test -timeout 40m -skip "$skip_regex" "${extra_test_flags[@]}"
    echo "--- go test -tags develop"
    go test -tags develop -timeout 40m -skip "$skip_regex" "${extra_test_flags[@]}" "${DEVELOP_TAG_PACKAGES[@]}"
}

case "${1:-all}" in
    build) cmd_build ;;
    vet) cmd_vet ;;
    test) cmd_test ;;
    all)
        cmd_build
        cmd_vet
        cmd_test
        ;;
    *)
        echo "usage: $0 {build|vet|test|all}" >&2
        exit 2
        ;;
esac
