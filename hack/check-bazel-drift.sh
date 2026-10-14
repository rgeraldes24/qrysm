#!/bin/bash
. "$(dirname "$0")"/common.sh

set -eo pipefail

# Regenerates the .pb.go, .pb.gw.go and SSZ files with Bazel, copies them over
# the ones `make gen` wrote, and fails if they differ beyond what is known to
# differ between the two. While Bazel builds the repository it generates this
# code itself, so the checked-in files must match what it generates.

# --- pb.go and pb.gw.go files --------------------------------------------

bazel query 'attr(testonly, 0, //proto/...)' | xargs bazel build "$@"

searchstring="theQRL/qrysm/"
while IFS= read -d $'\0' -r file; do
    # rules_go writes a "//go:build ignore" placeholder when the grpc-gateway
    # plugin generates nothing for a proto; make gen writes no file.
    if head -n 1 "$file" | grep -q '^//go:build ignore$'; then
        continue
    fi
    destination=${file#*$searchstring}
    color "34" "$destination"
    cp -L "$file" "$destination"
    chmod 755 "$destination"
done < <($findutil -L "$(bazel info bazel-bin)"/proto -type f -regextype sed -regex ".*pb\.\(gw\.\)\?go$" -print0)

# The checked-in files have always been run through goimports and gofmt -s.
go run golang.org/x/tools/cmd/goimports -w proto
gofmt -s -w proto

# --- ssz.go files --------------------------------------------------------

bazel query 'kind(ssz_gen_marshal, //proto/...)' | xargs bazel build "$@"

# ssz_gen_marshal writes "<name>.go"; every target is "ssz_generated_files"
# and is checked in as generated.ssz.go.
while IFS= read -d $'\0' -r file; do
    destination="$(dirname "${file#*/bin/}")/generated.ssz.go"
    color "34" "$destination"
    sed '/^\/\/ Hash: /d' "$file" > "$destination"
    chmod 644 "$destination"
done < <($findutil -L "$(bazel info bazel-bin)"/proto -type f -name "ssz_generated_files.go" -print0)

# --- drift check ---------------------------------------------------------
# Bazel generates the mainnet configuration only, and runs the plugins under
# protoc, so the only differences tolerated are:
#   - the protoc version in the .pb.go header (make gen runs no protoc):
#       -// 	protoc        (unknown)
#       +// 	protoc        v5.29.1
#   - the build constraint of the files that have a minimal twin:
#       -//go:build !minimal
#       -
# Any other added or removed line under proto/ is drift.
color "34" "Checking generated-code drift..."

drift=$(git diff -U0 -- proto |
    grep -E '^[-+]' |
    grep -vE '^(---|\+\+\+) ' |
    grep -vE '^-[[:space:]]*$' |
    grep -vE '^-//go:build !minimal$' |
    grep -vE '^[-+]//[[:space:]]+protoc[[:space:]]+(\(unknown\)|v[0-9.]+)$' || true)
# `|| true`: with no drift the last grep matches nothing and exits non-zero.

if [ -n "$drift" ]; then
    color "31" "Unexpected generated-code drift (run 'git diff -- proto' to inspect):"
    echo "$drift"
    exit 1
fi

color "32" "No unexpected generated-code drift."
