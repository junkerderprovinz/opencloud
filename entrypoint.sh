#!/bin/sh
# One-click init and privilege drop for OpenCloud on Unraid.
#
# Runs as root (see `USER root` in the Dockerfile) so it can, in order:
#   1. create the config/data dirs and heal their ownership for the target user
#   2. run `opencloud init` once as that user (writes the config on first boot)
#   3. drop to PUID:PGID via gosu and exec `opencloud server`
#
# PUID/PGID default to Unraid's nobody:users (99:100). The OpenCloud env vars
# (IDM_ADMIN_PASSWORD, OC_URL, OC_INSECURE, OC_LOG_LEVEL, PROXY_TLS,
# IDM_CREATE_DEMO_USERS, ...) are preserved across the gosu drop and read by
# OpenCloud itself; init consumes IDM_ADMIN_PASSWORD on the first run.
set -eu

PUID="${PUID:-99}"
PGID="${PGID:-100}"
CONFIG_DIR="/etc/opencloud"
DATA_DIR="/var/lib/opencloud"
SENTINEL="${DATA_DIR}/.uid-heal"

if [ "$(id -u)" = "0" ]; then
    # Run everything below dropped to the target user.
    DROP="gosu ${PUID}:${PGID}"

    # On a fresh Unraid install the bind mounts arrive root-owned. Create them if
    # missing and hand them to the target user.
    mkdir -p "${CONFIG_DIR}" "${DATA_DIR}"

    # The config dir is small (a few YAML files and secrets), so a recursive
    # chown on every boot is cheap.
    chown -R "${PUID}:${PGID}" "${CONFIG_DIR}"

    # The data dir can grow huge (all user blobs live under it), so it is not
    # chowned recursively on every boot. The server runs as the target user and
    # creates files with the right owner; a recursive pass is only needed to
    # repair a tree written earlier as root, or after a PUID/PGID change. A
    # sentinel records the last-healed owner so that pass runs once per PUID:PGID.
    chown "${PUID}:${PGID}" "${DATA_DIR}"          # top level only
    want="${PUID}:${PGID}"
    if [ ! -f "${SENTINEL}" ] || [ "$(cat "${SENTINEL}" 2>/dev/null)" != "${want}" ]; then
        echo "[entrypoint] healing ownership of ${DATA_DIR} -> ${want} (first run or PUID/PGID change)"
        chown -R "${PUID}:${PGID}" "${DATA_DIR}"
        printf '%s' "${want}" > "${SENTINEL}"
        chown "${PUID}:${PGID}" "${SENTINEL}"
    fi
    # NATS (the internal message bus) is small and has to stay writable by the
    # user even if it first appears after the one-time heal.
    if [ -d "${DATA_DIR}/nats" ]; then
        chown -R "${PUID}:${PGID}" "${DATA_DIR}/nats"
    fi
else
    echo "[entrypoint] not running as root (uid $(id -u)), skipping permission heal"
    DROP=""
fi

# HOME must be writable by the target user for a few Go libraries; point it at the
# data volume instead of the image's /root.
export HOME="${DATA_DIR}"

# OC_URL is both the public URL clients use and the built-in IDP's OIDC issuer,
# so it has to be a valid https URL. Unraid substitutes its [IP]/[PORT] tokens
# only in the template's WebUI field, not in env vars, so a default like
# https://[IP]:[PORT:9200] arrives here verbatim and crashes the reva gateway
# ("invalid IP-literal"). If OC_URL is empty or still a placeholder, derive one
# from the container's own IP so the server boots instead of crash-looping.
# Logins from every client need OC_URL set to the real server or reverse-proxy
# address.
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

# Optional web office (WOPI). OFFICE selects a browser document editor: off
# (default), collabora, onlyoffice or euro-office. The document server runs as its
# own container (Collabora CODE, or an OnlyOffice/Euro Office Document Server);
# this only turns on OpenCloud's built-in 'collaboration' (WOPI) service and
# points it at that server. OFFICE_SERVER_URL is the browser-reachable URL of that
# container, OFFICE_WOPI_SECRET a shared secret. Values checked against
# opencloud-compose weboffice/collabora.yml and weboffice/euro-office.yml. Euro
# Office is an ONLYOFFICE fork, so it is driven as the "OnlyOffice" product with
# its own display name.
_office="$(printf '%s' "${OFFICE:-off}" | tr '[:upper:]' '[:lower:]')"
case "${_office}" in
    collabora)                 _oc_app_name="CollaboraOnline"; _oc_app_product="Collabora" ;;
    onlyoffice)                _oc_app_name="OnlyOffice";      _oc_app_product="OnlyOffice" ;;
    euro-office|eurooffice)    _oc_app_name="Euro-Office";     _oc_app_product="OnlyOffice" ;;
    *)                         _oc_app_name="" ;;
esac
if [ -n "${_oc_app_name}" ] && [ -n "${OFFICE_SERVER_URL:-}" ]; then
    # append 'collaboration' to the supervised services, preserving any user value
    export OC_ADD_RUN_SERVICES="${OC_ADD_RUN_SERVICES:+${OC_ADD_RUN_SERVICES},}collaboration"
    export COLLABORATION_APP_NAME="${_oc_app_name}"
    export COLLABORATION_APP_PRODUCT="${_oc_app_product}"
    export COLLABORATION_APP_ADDR="${OFFICE_SERVER_URL}"
    export COLLABORATION_WOPI_SRC="${OC_URL}"
    [ -n "${OFFICE_WOPI_SECRET:-}" ] && export COLLABORATION_WOPI_SECRET="${OFFICE_WOPI_SECRET}"
    # OnlyOffice and Euro Office reject every document with "document security
    # token is not correctly formed" unless the WOPI secret here matches the doc
    # server's own JWT secret exactly.
    if [ "${_oc_app_product}" = "OnlyOffice" ] && [ -z "${OFFICE_WOPI_SECRET:-}" ]; then
        echo "[entrypoint] WARNING: OFFICE=${OFFICE} but OFFICE_WOPI_SECRET is empty; documents will fail with 'document security token is not correctly formed' unless it exactly matches the document server's JWT secret"
    fi
    # tolerate self-signed certs on the doc server and the internal data gateway (LAN default)
    export COLLABORATION_APP_INSECURE="${COLLABORATION_APP_INSECURE:-true}"
    export COLLABORATION_CS3API_DATAGATEWAY_INSECURE="${COLLABORATION_CS3API_DATAGATEWAY_INSECURE:-true}"
    # OnlyOffice signs with its own JWT rather than Collabora-style proof keys
    [ "${_oc_app_product}" = "OnlyOffice" ] && export COLLABORATION_APP_PROOF_DISABLE="${COLLABORATION_APP_PROOF_DISABLE:-true}"
    # register the collaboration app as the secure-view/edit handler and expose the
    # secure-view role (exact default role set incl. secure-view, from opencloud-compose)
    export FRONTEND_APP_HANDLER_SECURE_VIEW_APP_ADDR="eu.opencloud.api.collaboration"
    export GRAPH_AVAILABLE_ROLES="${GRAPH_AVAILABLE_ROLES:-b1e2218d-eef8-4d4c-b82d-0f1a1b48f3b5,a8d5fe5e-96e3-418d-825b-534dbdf22b99,fb6c3e19-e378-47e5-b277-9732f9de6e21,58c63c02-1d89-4572-916a-870abc5a1b7d,2d00ce52-1fc2-4dbc-8b95-a73b73395f5a,1c996275-f1c9-4e71-abdf-a42f6495e960,312c0871-5ef7-4b3a-85b6-0e4074c64049,aa97fe03-7980-45ac-9e50-b325749fd7e6}"
    # Registering a WOPI app does not add its origin to OpenCloud's own
    # Content-Security-Policy: COLLABORATION_APP_ADDR and the proxy's frame-src
    # allowlist are separate settings that need the same value. Without this the
    # browser blocks the editor iframe with a CSP frame-src violation
    # (junkerderprovinz/unraid-apps#7). opencloud-compose's weboffice/*.yml and
    # config/opencloud/csp.yaml wire the same origin into both places by hand;
    # this writes the second half automatically. PROXY_CSP_CONFIG_FILE_LOCATION
    # entries are merged into OpenCloud's built-in CSP, so the file only needs
    # the addition.
    if [ -z "${PROXY_CSP_CONFIG_FILE_LOCATION:-}" ]; then
        # Reduce OFFICE_SERVER_URL to scheme://host[:port]/, dropping any path or
        # query, with parameter expansion rather than a sed dialect.
        _oc_office_rest="${OFFICE_SERVER_URL#*://}"
        _oc_office_hostport="${_oc_office_rest%%/*}"
        _oc_office_origin="${OFFICE_SERVER_URL%%://*}://${_oc_office_hostport}/"
        cat > "${CONFIG_DIR}/csp.yaml" <<EOF
directives:
  frame-src:
    - '${_oc_office_origin}'
  img-src:
    - '${_oc_office_origin}'
EOF
        chown "${PUID}:${PGID}" "${CONFIG_DIR}/csp.yaml" 2>/dev/null || true
        export PROXY_CSP_CONFIG_FILE_LOCATION="${CONFIG_DIR}/csp.yaml"
        echo "[entrypoint] web-office enabled: ${_oc_app_product} at ${OFFICE_SERVER_URL} (collaboration service on, CSP frame-src updated)"
    else
        # The user already points OpenCloud at their own CSP file, so it stays
        # untouched and they add ${OFFICE_SERVER_URL} to its frame-src/img-src
        # themselves.
        echo "[entrypoint] web-office enabled: ${_oc_app_product} at ${OFFICE_SERVER_URL} (collaboration service on)"
        echo "[entrypoint] NOTE: PROXY_CSP_CONFIG_FILE_LOCATION is already set; add ${OFFICE_SERVER_URL} to its frame-src/img-src yourself or the editor iframe will be CSP-blocked"
    fi
elif [ -n "${_oc_app_name}" ]; then
    echo "[entrypoint] OFFICE=${OFFICE} set but OFFICE_SERVER_URL is empty -> web-office not enabled"
fi

# Optional full-text search. FULLTEXT_SEARCH=true turns on searching inside files,
# not just their names. Apache Tika runs as its own container (apache/tika, port
# 9998); this points OpenCloud's search extractor at it and tells the web UI that
# full-text search is available. TIKA_URL is that container's network-reachable
# URL. Values checked against the OpenCloud search docs. Only files uploaded or
# changed after this get their contents indexed.
_fts="$(printf '%s' "${FULLTEXT_SEARCH:-false}" | tr '[:upper:]' '[:lower:]')"
if [ "${_fts}" = "true" ] && [ -n "${TIKA_URL:-}" ]; then
    export SEARCH_EXTRACTOR_TYPE="tika"
    export SEARCH_EXTRACTOR_TIKA_TIKA_URL="${TIKA_URL}"
    # the extractor fetches file content from the internal CS3 gateway; tolerate
    # its self-signed cert (LAN default, mirrors the web-office data-gateway flag)
    export SEARCH_EXTRACTOR_CS3SOURCE_INSECURE="${SEARCH_EXTRACTOR_CS3SOURCE_INSECURE:-true}"
    export FRONTEND_FULL_TEXT_SEARCH_ENABLED="true"
    echo "[entrypoint] full-text search enabled: Apache Tika at ${TIKA_URL} (content indexing on for new/changed files)"
elif [ "${_fts}" = "true" ]; then
    echo "[entrypoint] FULLTEXT_SEARCH=true but TIKA_URL is empty -> full-text search not enabled"
fi

# BRANDING_APP=true adds an admin-only "Branding" app to the web UI: the extension
# in the apps folder, the proxy route /brandingsvc/ and brandingd on loopback.
# Switching it off removes the editor and keeps the saved branding in effect.
BRANDING_SHARE="/usr/local/share/opencloud-branding"
BRANDING_APPS_DIR="${DATA_DIR}/web/assets/apps/branding"
BRANDING_STATE="${DATA_DIR}/branding/state.json"
BRANDING_PROXY_MARKER="# managed by the opencloud Unraid wrapper (BRANDING_APP)"
_branding="$(printf '%s' "${BRANDING_APP:-false}" | tr '[:upper:]' '[:lower:]')"

# Any IDP_LOGIN_BACKGROUND_URL hides OpenCloud's login artwork, so it needs a saved image.
_bg_file=""
if [ -f "${BRANDING_STATE}" ]; then
    _bg_file="$(sed -n 's/^  "background": "\([^"]*\)".*/\1/p' "${BRANDING_STATE}")"
    case "${_bg_file}" in
        *[!a-z0-9.-]*) _bg_file="" ;;
    esac
fi

if [ "${_branding}" = "true" ] && [ -f "${CONFIG_DIR}/proxy.yaml" ] \
    && ! grep -qF "${BRANDING_PROXY_MARKER}" "${CONFIG_DIR}/proxy.yaml"; then
    echo "[entrypoint] WARNING: BRANDING_APP=true but ${CONFIG_DIR}/proxy.yaml is your own file - branding app not enabled (add the /brandingsvc/ route from the README to it yourself)"
    _branding="false"
fi

if [ "${_branding}" = "true" ]; then
    mkdir -p "${BRANDING_APPS_DIR}" "${DATA_DIR}/web/assets/themes/_branding" "${DATA_DIR}/branding"
    rm -rf "${BRANDING_APPS_DIR:?}/"*
    cp -R "${BRANDING_SHARE}/app/." "${BRANDING_APPS_DIR}/"
    cat > "${CONFIG_DIR}/proxy.yaml" <<EOF
${BRANDING_PROXY_MARKER}
additional_policies:
  - name: default
    routes:
      - endpoint: /brandingsvc/
        backend: http://127.0.0.1:9299
        unprotected: true
EOF
    if [ "$(id -u)" = "0" ]; then
        chown -R "${PUID}:${PGID}" "${DATA_DIR}/web" "${DATA_DIR}/branding" "${CONFIG_DIR}/proxy.yaml"
    fi
    _bg_active="false"
    if [ -n "${_bg_file}" ]; then
        export IDP_LOGIN_BACKGROUND_URL="/brandingsvc/login-background"
        _bg_active="true"
    fi
    _scheme="https"
    [ "$(printf '%s' "${PROXY_TLS}" | tr '[:upper:]' '[:lower:]')" = "false" ] && _scheme="http"
    (
        while :; do
            # shellcheck disable=SC2086
            BRANDING_OPENCLOUD_URL="${_scheme}://127.0.0.1:9200" \
            BRANDING_DATA_DIR="${DATA_DIR}" \
            BRANDING_LOGIN_BACKGROUND_ACTIVE="${_bg_active}" \
                ${DROP} /usr/local/bin/brandingd || echo "[entrypoint] brandingd exited with $?, restarting in 5s"
            sleep 5
        done
    ) &
    echo "[entrypoint] branding admin app enabled (app menu -> Branding, admins only)"
else
    rm -rf "${BRANDING_APPS_DIR}"
    if [ -f "${CONFIG_DIR}/proxy.yaml" ] && grep -qF "${BRANDING_PROXY_MARKER}" "${CONFIG_DIR}/proxy.yaml"; then
        rm -f "${CONFIG_DIR}/proxy.yaml"
    fi
    if [ -n "${_bg_file}" ]; then
        export IDP_LOGIN_BACKGROUND_URL="/themes/_branding/${_bg_file}"
    fi
fi

# First-boot init writes ${CONFIG_DIR}/opencloud.yaml and consumes
# IDM_ADMIN_PASSWORD. On later boots the file exists and init exits non-zero,
# which is ignored.
#
# 'opencloud init' reads --insecure, a string flag whose default "ask" opens an
# interactive prompt. Without a TTY that prompt loops on EOF forever and the
# container hangs on a fresh install, so an explicit value is always passed and
# stdin comes from /dev/null. Leave out --force-overwrite: it regenerates every
# service secret and the admin password on each boot.
echo "[entrypoint] running 'opencloud init' (harmless error if already initialised)"
# shellcheck disable=SC2086
${DROP} opencloud init --insecure "${OC_INSECURE}" </dev/null || true

# House ready banner, the last block this wrapper prints before the OpenCloud
# server takes over the log. Nothing can run after the exec below, so the status
# line marks the handoff rather than a verified health check.
/usr/local/bin/print-banner.sh "OpenCloud" "Cloud storage & collaboration platform, plug-and-play for Unraid"
printf '  \033[0;32m✓ OPENCLOUD IS READY\033[0m - Handing off to the OpenCloud server now\n'
echo ""

# Hand off: replace the shell with the server, dropped to the target user.
# shellcheck disable=SC2086
exec ${DROP} opencloud server
