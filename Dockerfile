# syntax=docker/dockerfile:1
# =============================================================================
# opencloud - one-click OpenCloud wrapper image for Unraid
#
# A thin wrapper around the official OpenCloud image that turns it into a genuine
# one-click Unraid app. The official image is Alpine-based and runs its
# `opencloud` binary directly with NO PUID/PGID support, so on a fresh Unraid
# install (root-owned bind mounts) the first boot fails with "permission denied"
# writing /etc/opencloud/opencloud.yaml and /var/lib/opencloud/nats, and it never
# runs the required one-time `opencloud init`. This wrapper fixes both without
# forking OpenCloud:
#   * runs `opencloud init` once (writes the config on first boot, idempotent)
#   * heals bind-mount ownership so root-created appdata becomes writable
#   * honours Unraid's PUID / PGID (default 99:100 = nobody:users) and drops
#     privileges to that user for the server via a static, dependency-free gosu
#
# TWO CHANNELS (selected with --build-arg BASE=...):
#   :production  ->  opencloudeu/opencloud:7.2.3          (default, BASE below)
#   :rolling     ->  opencloudeu/opencloud-rolling:7.4.0  (BASE_ROLLING, CI reads it)
# Renovate tracks BOTH pins and auto-merges the bumps - see renovate.json.
#
# NOTE: the :production line (7.2.x) is OpenCloud's slow, stable train and does
# NOT yet carry reva#720 (the incremental-fsync fix for large-folder sync aborts,
# issue #3027). That fix ships from 7.3.0, which OpenCloud publishes only on the
# ROLLING image. So for the newest OpenCloud - and to avoid the sync-abort bug on
# slow (array/FUSE) storage - run the :rolling channel.
#
# Licensing: this wrapper (Dockerfile + scripts + banner) is MIT; the OpenCloud
# binary baked into the base image is Apache-2.0. See LICENSE / NOTICE.
# =============================================================================

# Concrete, Renovate-tracked base pins. BASE feeds `FROM ${BASE}`; BASE_ROLLING is
# a pin marker that CI extracts from this file to build the :rolling channel via
# `--build-arg BASE=<rolling>`, so both tags stay in one place.
ARG BASE=opencloudeu/opencloud:7.2.3
ARG BASE_ROLLING=opencloudeu/opencloud-rolling:7.4.0

# Static gosu for the privilege drop, copied from the upstream multi-arch image
# so we never depend on the base image having apk/apt at build time.
FROM tianon/gosu:1.19 AS gosu

# hadolint ignore=DL3006
FROM ${BASE}

# The published base image runs as a non-root user (uid 1000); switch to root
# BEFORE any COPY/RUN so the build can write under /usr/local, AND so the
# entrypoint later starts privileged to heal ownership before dropping to
# PUID:PGID with gosu.
# hadolint ignore=DL3002
USER root

# OCI provenance. Wrapper assets = MIT; the bundled OpenCloud binary = Apache-2.0.
LABEL org.opencontainers.image.title="opencloud (Unraid wrapper)" \
      org.opencontainers.image.description="One-click OpenCloud for Unraid: auto-init, permission heal, PUID/PGID." \
      org.opencontainers.image.source="https://github.com/junkerderprovinz/opencloud" \
      org.opencontainers.image.licenses="AGPL-3.0-only" \
      org.opencontainers.image.vendor="junkerderprovinz"

# Static gosu + our init wrapper + the shared house log banner.
COPY --from=gosu /gosu /usr/local/bin/gosu
COPY entrypoint.sh print-banner.sh /usr/local/bin/
COPY .github/assets/banner-raw.txt /usr/local/share/banner-raw.txt

# Install the shared banner art (strip any CRLF -> a clean log block; .gitattributes
# already pins the scripts to LF) and make everything executable. The base ships
# BusyBox coreutils (tr/chmod), so no package manager is needed here.
RUN tr -d '\r' < /usr/local/share/banner-raw.txt > /usr/local/share/banner.txt \
 && rm /usr/local/share/banner-raw.txt \
 && chmod +x /usr/local/bin/entrypoint.sh /usr/local/bin/print-banner.sh /usr/local/bin/gosu

# OpenCloud config + data - the Unraid template bind-mounts these two paths.
VOLUME ["/etc/opencloud", "/var/lib/opencloud"]
# HTTPS WebUI / API. With PROXY_TLS=true the OpenCloud proxy serves TLS on 9200.
EXPOSE 9200

# Our wrapper replaces the base ENTRYPOINT; it runs init then execs the server,
# so the base CMD (["server"]) is ignored.
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
