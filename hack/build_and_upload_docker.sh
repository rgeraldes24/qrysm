#!/bin/bash

# -----------------------------------------------------------------------------
# This script builds the docker images (`make dist`) and uploads them, as
# multi-architecture (linux/amd64 + linux/arm64) images, to the registry.
#
# Usage: ./hack/build_and_upload_docker.sh <tag>
#
# Pushes <registry>:<image>-<tag> for the beacon-chain, validator and alltools
# images (plus the per-architecture <registry>:<image>-<tag>-<arch> images the
# multi-architecture ones point to). <registry> defaults to
# index.docker.io/qrledger/qrysm (DOCKER_REGISTRY overrides it).
# -----------------------------------------------------------------------------

set -euo pipefail

# Validate that the tag argument exists.
if [ "${1:-}" = "" ]
then
  echo "Usage: $0 <tag>"
  exit 1
fi
TAG=$1
REGISTRY=${DOCKER_REGISTRY:-index.docker.io/qrledger/qrysm}
ARCHES=(amd64 arm64)

# The version the binaries (and so the image tarballs) are named after.
GIT_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo Unknown)

# Build all the binaries the images need, for linux/amd64 and linux/arm64, and
# package the images.
make dist beacon-chain validator client-stats qrysmctl deposit \
  platform=docker/amd64,docker/arm64 \
  DOCKER_REGISTRY="$REGISTRY" DOCKER_TAG="$TAG"

for image in beacon-chain validator alltools; do
  ref="$REGISTRY:$image-$TAG"
  arch_refs=()

  for arch in "${ARCHES[@]}"; do
    # Each tarball holds the image tagged $ref: retag it per architecture.
    docker load -i "dist/$image-$GIT_TAG-linux-$arch.tar"
    docker tag "$ref" "$ref-$arch"
    docker push "$ref-$arch"
    arch_refs+=("$ref-$arch")
  done

  docker buildx imagetools create -t "$ref" "${arch_refs[@]}"
done
