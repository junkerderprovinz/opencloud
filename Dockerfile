# syntax=docker/dockerfile:1@sha256:ecfaec9ed6d810b56388c508f4121597bfbba70d41a6dfeee4d8cad5f295fc32
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
#   * moves a search index the server cannot open aside and rebuilds it, where
#     OpenCloud would stop altogether
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
# Licensing: this wrapper (Dockerfile, scripts, banner, brandingd and the web
# extension) is AGPL-3.0-only; the OpenCloud binary in the base image is
# Apache-2.0. See LICENSE and NOTICE.

# Floating upstream tags. BASE feeds `FROM ${BASE}`; BASE_ROLLING is a marker CI
# extracts from this file to build the :rolling channel with
# `--build-arg BASE=<rolling>`, so both tags live in one place. Neither is
# Renovate-tracked (see renovate.json); the weekly cron rebuild keeps them current.
ARG BASE=opencloudeu/opencloud:latest
ARG BASE_ROLLING=opencloudeu/opencloud-rolling:latest

# Static gosu for the privilege drop, copied from the upstream multi-arch image
# so the build does not depend on the base image having apk or apt.
FROM tianon/gosu:1.19@sha256:5afac3970da83806ba3d7789a3a42da92fbe4f703d94984c601e183208d035d3 AS gosu

# Base theme of the bundled OpenCloud version, for brandingd's dark-mode logo.
# hadolint ignore=DL3006
FROM --platform=$BUILDPLATFORM ${BASE} AS basetheme
RUN opencloud version --skip-services > /tmp/opencloud-version \
 && version="$(awk '/^Version:/ {print $2; exit}' /tmp/opencloud-version)" \
 && for attempt in 1 2 3; do \
        wget -q -O /tmp/base-theme.json "https://raw.githubusercontent.com/opencloud-eu/opencloud/v${version}/services/web/assets/themes/opencloud/theme.json" && break; \
        echo "base theme download failed (attempt ${attempt} of 3)"; \
        rm -f /tmp/base-theme.json; \
        if [ "$attempt" -lt 3 ]; then sleep 5; fi; \
    done \
 && test -s /tmp/base-theme.json \
 && grep -q '"themes"' /tmp/base-theme.json

# brandingd and logintemplate are static, so they cross-compile on the build host.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS brandingd
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY branding/server/ ./
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/ ./cmd/brandingd ./cmd/logintemplate

# searchindex sets aside a search index the server cannot open, see the entrypoint.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS searchindex
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY searchindex/ ./
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/searchindex .

# The web extension is plain JS and CSS, so one build serves every platform.
FROM --platform=$BUILDPLATFORM node:24-alpine@sha256:ebfe2f90462722a7a4de65e91990e97fe0d401c70e0e762c5b53302f905ec1c1 AS brandingweb
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
COPY --from=searchindex /out/searchindex /usr/local/bin/searchindex
COPY entrypoint.sh print-banner.sh /usr/local/bin/
COPY .github/assets/banner-raw.txt /usr/local/share/banner-raw.txt

# Optional branding admin app (BRANDING_APP=true), installed by the entrypoint.
COPY --from=brandingd /out/brandingd /usr/local/bin/brandingd
COPY --from=brandingweb /src/dist/ /usr/local/share/opencloud-branding/app/
COPY --from=basetheme /tmp/base-theme.json /usr/local/share/opencloud-branding/base-theme.json

# The sign-in page names the hashed bundles of the binary it ships in, so the
# copy that loads the branding script comes from that binary.
RUN --mount=type=bind,from=brandingd,source=/out,target=/tmp/branding-build \
    /tmp/branding-build/logintemplate /usr/bin/opencloud /usr/local/share/opencloud-branding/idp/identifier/index.html

# A stray CR in the banner art would show up in the log. BusyBox in the base
# provides tr and chmod, so no package manager is needed.
RUN tr -d '\r' < /usr/local/share/banner-raw.txt > /usr/local/share/banner.txt \
 && rm /usr/local/share/banner-raw.txt \
 && chmod +x /usr/local/bin/entrypoint.sh /usr/local/bin/print-banner.sh /usr/local/bin/gosu /usr/local/bin/brandingd /usr/local/bin/searchindex

# OpenCloud config and data; the Unraid template bind-mounts these two paths.
VOLUME ["/etc/opencloud", "/var/lib/opencloud"]
# HTTPS WebUI / API. With PROXY_TLS=true the OpenCloud proxy serves TLS on 9200.
EXPOSE 9200

# The wrapper replaces the base ENTRYPOINT and execs the server itself, so the
# base CMD (["server"]) is ignored.
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
