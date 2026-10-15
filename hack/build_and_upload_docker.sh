#!/bin/bash

# -----------------------------------------------------------------------------
# Builds the beacon-chain, validator and alltools images for linux/amd64 and
# linux/arm64 and pushes them to index.docker.io/qrledger/qrysm as
# <image>-<tag> (e.g. beacon-chain-latest). Requires `docker login` and a
# buildx builder that can build for both platforms.
# -----------------------------------------------------------------------------

set -euo pipefail

# Validate that the tag argument exists.
if [ "${1:-}" = "" ]
then
  echo "Usage: $0 <tag>"
  exit 1
fi

make image-push image_tag="$1"
