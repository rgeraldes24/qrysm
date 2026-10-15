# syntax=docker/dockerfile:1
#
# The qrysm images: the distroless cc-debian11 base (the digest the images
# have always used), bash and coreutils from Debian 11, /bin/sh -> /bin/bash,
# and the binaries `make dist` built for linux/<arch>. Use `make image` /
# `make image-push`. /etc/passwd is the base image's (root, nobody, nonroot
# with uid 65532).
#
# Targets: beacon-chain, validator, alltools.

ARG BASE=gcr.io/prysmaticlabs/distroless/cc-debian11@sha256:55a5e011b2c4246b4c51e01fcc2b452d151e03df052e357465f0392fcd59fddf

# The root filesystem layer runs on the build platform: it only downloads and
# unpacks files, so a multi-arch build needs no emulation.
FROM --platform=$BUILDPLATFORM debian:bookworm-slim AS rootfs
ARG TARGETARCH
RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates && rm -rf /var/lib/apt/lists/*
COPY tools/docker/debian-pkgs.sh /usr/local/bin/debian-pkgs.sh
RUN /usr/local/bin/debian-pkgs.sh "$TARGETARCH" /rootfs
RUN set -eu; \
    mkdir -p /rootfs/bin; \
    ln -sf /bin/bash /rootfs/bin/sh; \
    for bin in beacon-chain validator; do \
      mkdir -p "/links/$bin/app/cmd/$bin"; \
      ln -sf "/$bin" "/links/$bin/app/cmd/$bin/$bin"; \
    done

FROM ${BASE} AS base
LABEL org.opencontainers.image.source="https://github.com/theQRL/qrysm"
COPY --from=rootfs /rootfs/ /

FROM base AS beacon-chain
ARG TAG
ARG TARGETARCH
# Backwards compatibility for images that depended on the old file path.
COPY --from=rootfs /links/beacon-chain/ /
COPY dist/beacon-chain-${TAG}-linux-${TARGETARCH} /beacon-chain
ENTRYPOINT ["/beacon-chain"]

FROM base AS validator
ARG TAG
ARG TARGETARCH
# Backwards compatibility for images that depended on the old file path.
COPY --from=rootfs /links/validator/ /
COPY dist/validator-${TAG}-linux-${TARGETARCH} /validator
ENTRYPOINT ["/validator"]

FROM base AS alltools
ARG TAG
ARG TARGETARCH
COPY dist/beacon-chain-${TAG}-linux-${TARGETARCH} /usr/local/bin/beacon-chain
COPY dist/client-stats-${TAG}-linux-${TARGETARCH} /usr/local/bin/client-stats
COPY dist/qrysmctl-${TAG}-linux-${TARGETARCH} /usr/local/bin/qrysmctl
COPY dist/deposit-${TAG}-linux-${TARGETARCH} /usr/local/bin/deposit
COPY dist/validator-${TAG}-linux-${TARGETARCH} /usr/local/bin/validator
