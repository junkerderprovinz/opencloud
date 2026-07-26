#!/bin/sh
# =============================================================================
# entrypoint.sh - one-click init + privilege drop for OpenCloud on Unraid
#
# Runs as root (see `USER root` in the Dockerfile) so it can, in order:
#   1. create the config/data dirs and heal their ownership for the target user
#   2. run `opencloud init` once as that user (writes the config on first boot)
#   3. drop to PUID:PGID via gosu and exec `opencloud server`
#
# PUID/PGID default to Unraid's nobody:users (99:100). The OpenCloud env vars
# (IDM_ADMIN_PASSWORD, OC_URL, OC_INSECURE, OC_LOG_LEVEL, PROXY_TLS,
# IDM_CREATE_DEMO_USERS, ...) are preserved across the gosu drop and read by
# OpenCloud itself - init consumes IDM_ADMIN_PASSWORD on the first run.
# =============================================================================
set -eu

PUID="${PUID:-99}"
PGID="${PGID:-100}"
CONFIG_DIR="/etc/opencloud"
DATA_DIR="/var/lib/opencloud"
SENTINEL="${DATA_DIR}/.uid-heal"

if [ "$(id -u)" = "0" ]; then
    # Run everything below dropped to the target user.
    DROP="gosu ${PUID}:${PGID}"

    # --- permission heal -----------------------------------------------------
    # On a fresh Unraid install the bind mounts arrive root-owned. Create them if
    # missing and hand them to the target user.
    mkdir -p "${CONFIG_DIR}" "${DATA_DIR}"

    # Config dir is small (a few YAML files + secrets) -> always chown -R, cheap.
    chown -R "${PUID}:${PGID}" "${CONFIG_DIR}"

    # Data dir can grow huge (all user blobs live under it) -> do NOT chown -R on
    # every boot. The server runs AS the target user, so whatever it creates is
    # already owned correctly; a recursive pass is only needed to REPAIR a tree
    # written earlier as root, or after a PUID/PGID change. A sentinel records the
    # last-healed owner so the expensive pass runs at most once per (PUID:PGID).
    chown "${PUID}:${PGID}" "${DATA_DIR}"          # top level only - cheap
    want="${PUID}:${PGID}"
    if [ ! -f "${SENTINEL}" ] || [ "$(cat "${SENTINEL}" 2>/dev/null)" != "${want}" ]; then
        echo "[entrypoint] healing ownership of ${DATA_DIR} -> ${want} (first run or PUID/PGID change)"
        chown -R "${PUID}:${PGID}" "${DATA_DIR}"
        printf '%s' "${want}" > "${SENTINEL}"
        chown "${PUID}:${PGID}" "${SENTINEL}"
    fi
    # NATS (the internal message bus) is small and must always be writable by the
    # user, even if it first appears after the one-time heal - cheap to re-assert.
    if [ -d "${DATA_DIR}/nats" ]; then
        chown -R "${PUID}:${PGID}" "${DATA_DIR}/nats"
    fi
else
    echo "[entrypoint] not running as root (uid $(id -u)) - skipping permission heal"
    DROP=""
fi

# HOME must be writable by the target user for a few Go libraries; point it at the
# data volume instead of the image's /root.
export HOME="${DATA_DIR}"

# First-boot init writes ${CONFIG_DIR}/opencloud.yaml and consumes
# IDM_ADMIN_PASSWORD. Idempotent: on later boots the file exists and init exits
# non-zero, which we deliberately ignore.
echo "[entrypoint] running 'opencloud init' (harmless error if already initialised)"
# shellcheck disable=SC2086
${DROP} opencloud init || true

# House ready banner - the LAST block this wrapper prints before handing off to
# the OpenCloud server (which then streams its own logs).
/usr/local/bin/print-banner.sh "OpenCloud" "OPENCLOUD IS READY"

# Hand off: replace the shell with the server, dropped to the target user.
# shellcheck disable=SC2086
exec ${DROP} opencloud server
