# syntax=docker/dockerfile:1
#
# Docker image build stage
# ------------------------
#
# This Dockerfile builds hermetic, reproducible and distroless Docker images for qrysm, from the binaries
# `make dist` cross-built into dist/. It mirrors the images Bazel built with tools/qrysm_image.bzl:
#   --target single   (BIN=beacon-chain|validator): /<BIN> as entrypoint, plus the legacy /app/cmd/<BIN>/<BIN> path.
#   --target alltools: every tool under /usr/local/bin, no entrypoint.

ARG BASE=gcr.io/prysmaticlabs/distroless/cc-debian11@sha256:55a5e011b2c4246b4c51e01fcc2b452d151e03df052e357465f0392fcd59fddf

FROM debian:bookworm-slim AS rootfs
ARG TARGETARCH
RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates && rm -rf /var/lib/apt/lists/*
COPY tools/docker/debian-pkgs.sh /usr/local/bin/debian-pkgs.sh
RUN /usr/local/bin/debian-pkgs.sh "$TARGETARCH" /rootfs
RUN set -eu; \
    mkdir -p /rootfs/etc /rootfs/bin; \
    printf 'root:x:0:0:root:/root:/bin/bash\nnonroot:x:1001:1001:nonroot:/home/nonroot:/bin/bash\n' > /rootfs/etc/passwd; \
    ln -sf /bin/bash /rootfs/bin/sh

FROM rootfs AS rootfs-single
ARG BIN
RUN set -eu; \
    mkdir -p "/rootfs/app/cmd/$BIN"; \
    ln -sf "/$BIN" "/rootfs/app/cmd/$BIN/$BIN"; \
    ln -sf "/$BIN" /rootfs/entrypoint

FROM ${BASE} AS single
ARG BIN
ARG TAG
ARG TARGETARCH
LABEL org.opencontainers.image.source="https://github.com/theQRL/qrysm"
COPY --from=rootfs-single /rootfs/ /
COPY dist/${BIN}-${TAG}-linux-${TARGETARCH} /${BIN}
ENTRYPOINT ["/entrypoint"]

FROM ${BASE} AS alltools
ARG TAG
ARG TARGETARCH
LABEL org.opencontainers.image.source="https://github.com/theQRL/qrysm"
COPY --from=rootfs /rootfs/ /
COPY dist/beacon-chain-${TAG}-linux-${TARGETARCH} /usr/local/bin/beacon-chain
COPY dist/client-stats-${TAG}-linux-${TARGETARCH} /usr/local/bin/client-stats
COPY dist/deposit-${TAG}-linux-${TARGETARCH} /usr/local/bin/deposit
COPY dist/qrysmctl-${TAG}-linux-${TARGETARCH} /usr/local/bin/qrysmctl
COPY dist/validator-${TAG}-linux-${TARGETARCH} /usr/local/bin/validator
