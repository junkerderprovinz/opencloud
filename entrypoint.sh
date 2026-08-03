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

# --- external URL / TLS defaults --------------------------------------------
# OC_URL is the public URL clients use AND the built-in IDP's OIDC issuer, so it
# must be a valid https URL. Unraid only substitutes its [IP]/[PORT] tokens in the
# template's WebUI field, NOT in env vars - a template default like
# https://[IP]:[PORT:9200] therefore arrives here verbatim and crashes the reva
# gateway ("invalid IP-literal"). If OC_URL is empty or still holds a bracket /
# placeholder value, derive a usable one from the container's own IP so the server
# boots instead of crash-looping. Set OC_URL to your real server (or reverse-proxy)
# address for logins to work from every client.
case "${OC_URL:-}" in
    ""|*"["*|*YOUR-SERVER-IP*)
        _ip="$(hostname -i 2>/dev/null | awk '{print $1}')"
        [ -z "${_ip}" ] && _ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
        [ -z "${_ip}" ] && _ip="localhost"
        OC_URL="https://${_ip}:9200"
        echo "[entrypoint] WARNING: OC_URL was unset or still a placeholder -> using ${OC_URL}."
        echo "[entrypoint]          Set the 'Public URL' to your real server address (or reverse-proxy URL) so client logins work everywhere."
        ;;
esac
export OC_URL
# Direct-install defaults: OpenCloud serves its own (self-signed) HTTPS on 9200 and
# tolerates that cert on its internal self-calls. A reverse-proxy setup overrides
# both of these to false in the template.
export OC_INSECURE="${OC_INSECURE:-true}"
export PROXY_TLS="${PROXY_TLS:-true}"

# --- optional web-office (WOPI) wiring ---------------------------------------
# OFFICE selects a browser document editor: off (default) | collabora | onlyoffice.
# The document server ALWAYS runs as a SEPARATE container (Collabora CODE, or an
# OnlyOffice/Euro Office Document Server); this only turns on OpenCloud's built-in
# 'collaboration' (WOPI) service and points it at that server. OFFICE_SERVER_URL =
# the browser-reachable URL of that container; OFFICE_WOPI_SECRET = a shared secret.
# Values verified against opencloud-compose weboffice/collabora.yml. Default off, so
# a normal install is unaffected.
_office="$(printf '%s' "${OFFICE:-off}" | tr '[:upper:]' '[:lower:]')"
case "${_office}" in
    collabora)                               _oc_app_name="CollaboraOnline"; _oc_app_product="Collabora" ;;
    onlyoffice|euro-office|eurooffice)        _oc_app_name="OnlyOffice";      _oc_app_product="OnlyOffice" ;;
    *)                                        _oc_app_name="" ;;
esac
if [ -n "${_oc_app_name}" ] && [ -n "${OFFICE_SERVER_URL:-}" ]; then
    # append 'collaboration' to the supervised services, preserving any user value
    export OC_ADD_RUN_SERVICES="${OC_ADD_RUN_SERVICES:+${OC_ADD_RUN_SERVICES},}collaboration"
    export COLLABORATION_APP_NAME="${_oc_app_name}"
    export COLLABORATION_APP_PRODUCT="${_oc_app_product}"
    export COLLABORATION_APP_ADDR="${OFFICE_SERVER_URL}"
    export COLLABORATION_WOPI_SRC="${OC_URL}"
    [ -n "${OFFICE_WOPI_SECRET:-}" ] && export COLLABORATION_WOPI_SECRET="${OFFICE_WOPI_SECRET}"
    # tolerate self-signed certs on the doc server + internal data gateway (LAN default)
    export COLLABORATION_APP_INSECURE="${COLLABORATION_APP_INSECURE:-true}"
    export COLLABORATION_CS3API_DATAGATEWAY_INSECURE="${COLLABORATION_CS3API_DATAGATEWAY_INSECURE:-true}"
    # OnlyOffice signs with its own JWT rather than Collabora-style proof keys
    [ "${_oc_app_product}" = "OnlyOffice" ] && export COLLABORATION_APP_PROOF_DISABLE="${COLLABORATION_APP_PROOF_DISABLE:-true}"
    # register the collaboration app as the secure-view/edit handler + expose the
    # secure-view role (exact default role set incl. secure-view, from opencloud-compose)
    export FRONTEND_APP_HANDLER_SECURE_VIEW_APP_ADDR="eu.opencloud.api.collaboration"
    export GRAPH_AVAILABLE_ROLES="${GRAPH_AVAILABLE_ROLES:-b1e2218d-eef8-4d4c-b82d-0f1a1b48f3b5,a8d5fe5e-96e3-418d-825b-534dbdf22b99,fb6c3e19-e378-47e5-b277-9732f9de6e21,58c63c02-1d89-4572-916a-870abc5a1b7d,2d00ce52-1fc2-4dbc-8b95-a73b73395f5a,1c996275-f1c9-4e71-abdf-a42f6495e960,312c0871-5ef7-4b3a-85b6-0e4074c64049,aa97fe03-7980-45ac-9e50-b325749fd7e6}"
    echo "[entrypoint] web-office enabled: ${_oc_app_product} at ${OFFICE_SERVER_URL} (collaboration service on)"
elif [ -n "${_oc_app_name}" ]; then
    echo "[entrypoint] OFFICE=${OFFICE} set but OFFICE_SERVER_URL is empty -> web-office NOT enabled"
fi

# First-boot init writes ${CONFIG_DIR}/opencloud.yaml and consumes
# IDM_ADMIN_PASSWORD. Idempotent: on later boots the file exists and init exits
# non-zero, which we deliberately ignore (|| true).
#
# 'opencloud init' reads --insecure, a STRING flag whose default value "ask" opens
# an interactive stdin prompt. In a container with no TTY that prompt never gets an
# answer and spins forever (it loops on EOF), so the wrapper hangs on a fresh
# install. We ALWAYS pass an explicit value so it can never be "ask", and redirect
# stdin from /dev/null as belt-and-suspenders. Do NOT add --force-overwrite: it
# regenerates every service secret and the admin password on each boot.
echo "[entrypoint] running 'opencloud init' (harmless error if already initialised)"
# shellcheck disable=SC2086
${DROP} opencloud init --insecure "${OC_INSECURE}" </dev/null || true

# House ready banner - the LAST block this wrapper prints before handing off to
# the OpenCloud server (which then streams its own logs).
/usr/local/bin/print-banner.sh "OpenCloud" "OPENCLOUD IS READY"

# Hand off: replace the shell with the server, dropped to the target user.
# shellcheck disable=SC2086
exec ${DROP} opencloud server
