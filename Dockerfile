# syntax=docker/dockerfile:1
# opencloud: one-click OpenCloud wrapper image for Unraid
#
# A thin wrapper around the official OpenCloud image. That image is Alpine-based
# and runs its `opencloud` binary directly with no PUID/PGID support, so on a
# fresh Unraid install (root-owned bind mounts) the first boot fails with
# "permission denied" writing /etc/opencloud/opencloud.yaml and
# /var/lib/opencloud/nats, and it never runs the required one-time
# `opencloud init`. This wrapper fixes both without forking OpenCloud:
#   * runs `opencloud init` once (writes the config on first boot, idempotent)
#   * heals bind-mount ownership so root-created appdata becomes writable
#   * honours Unraid's PUID / PGID (default 99:100 = nobody:users) and drops
#     privileges to that user for the server via a static, dependency-free gosu
#
# Two channels, selected with --build-arg BASE=...:
#   :production  ->  opencloudeu/opencloud:latest          (default, BASE below)
#   :rolling     ->  opencloudeu/opencloud-rolling:latest  (BASE_ROLLING, CI reads it)
# Both track upstream's floating tag without a version pin; the weekly cron
# rebuild in build.yml republishes whatever each tag points to.
#
# The :production line is OpenCloud's slow, stable train and does not carry
# reva#720 (the incremental-fsync fix for large-folder sync aborts, issue #3027)
# until OpenCloud cuts a stable release that includes it; the :rolling image has
# it since 7.3.0. For the newest OpenCloud, and to avoid the sync-abort bug on
# slow (array or FUSE) storage, run the :rolling channel.
#
# Licensing: wrapper scripts MIT; branding/web (the web extension) AGPL-3.0;
# brandingd MIT; the OpenCloud binary Apache-2.0. See LICENSE / NOTICE.
# =============================================================================

# Floating upstream tags. BASE feeds `FROM ${BASE}`; BASE_ROLLING is a marker CI
# extracts from this file to build the :rolling channel with
# `--build-arg BASE=<rolling>`, so both tags live in one place. Neither is
# Renovate-tracked (see renovate.json); the weekly cron rebuild keeps them current.
ARG BASE=opencloudeu/opencloud:latest
ARG BASE_ROLLING=opencloudeu/opencloud-rolling:latest

# Static gosu for the privilege drop, copied from the upstream multi-arch image
# so the build does not depend on the base image having apk or apt.
FROM tianon/gosu:1.19 AS gosu

# Base theme of the bundled OpenCloud version, for brandingd's dark-mode logo.
# hadolint ignore=DL3006
FROM --platform=$BUILDPLATFORM ${BASE} AS basetheme
RUN opencloud version --skip-services > /tmp/opencloud-version \
 && version="$(awk '/^Version:/ {print $2; exit}' /tmp/opencloud-version)" \
 && test -n "$version" \
 && wget -q -O /tmp/base-theme.json "https://raw.githubusercontent.com/opencloud-eu/opencloud/v${version}/services/web/assets/themes/opencloud/theme.json"

# brandingd is static, so it cross-compiles on the build host.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS brandingd
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY branding/server/ ./
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/brandingd ./cmd/brandingd

# The web extension is plain JS and CSS, so one build serves every platform.
FROM --platform=$BUILDPLATFORM node:24-alpine AS brandingweb
WORKDIR /src
ENV COREPACK_ENABLE_DOWNLOAD_PROMPT=0
RUN corepack enable
COPY branding/web/package.json branding/web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY branding/web/ ./
RUN pnpm build

# hadolint ignore=DL3006
FROM ${BASE}

# The published base image runs as a non-root user (uid 1000). Root is needed so
# the build can write under /usr/local, and so the entrypoint starts privileged
# to heal ownership before dropping to PUID:PGID with gosu.
# hadolint ignore=DL3002
USER root

LABEL org.opencontainers.image.title="opencloud (Unraid wrapper)" \
      org.opencontainers.image.description="One-click OpenCloud for Unraid: auto-init, permission heal, PUID/PGID." \
      org.opencontainers.image.source="https://github.com/junkerderprovinz/opencloud" \
      org.opencontainers.image.licenses="AGPL-3.0-only" \
      org.opencontainers.image.vendor="junkerderprovinz"

COPY --from=gosu /gosu /usr/local/bin/gosu
COPY entrypoint.sh print-banner.sh /usr/local/bin/
COPY .github/assets/banner-raw.txt /usr/local/share/banner-raw.txt

# Optional branding admin app (BRANDING_APP=true), installed by the entrypoint.
COPY --from=brandingd /out/brandingd /usr/local/bin/brandingd
COPY --from=brandingweb /src/dist/ /usr/local/share/opencloud-branding/app/
COPY --from=basetheme /tmp/base-theme.json /usr/local/share/opencloud-branding/base-theme.json

# Install the shared banner art (strip any CRLF -> a clean log block; .gitattributes
# already pins the scripts to LF) and make everything executable. The base ships
# BusyBox coreutils (tr/chmod), so no package manager is needed here.
RUN tr -d '\r' < /usr/local/share/banner-raw.txt > /usr/local/share/banner.txt \
 && rm /usr/local/share/banner-raw.txt \
 && chmod +x /usr/local/bin/entrypoint.sh /usr/local/bin/print-banner.sh /usr/local/bin/gosu /usr/local/bin/brandingd

# OpenCloud config and data; the Unraid template bind-mounts these two paths.
VOLUME ["/etc/opencloud", "/var/lib/opencloud"]
# HTTPS WebUI / API. With PROXY_TLS=true the OpenCloud proxy serves TLS on 9200.
EXPOSE 9200

# The wrapper replaces the base ENTRYPOINT and execs the server itself, so the
# base CMD (["server"]) is ignored.
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
