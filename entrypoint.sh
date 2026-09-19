#!/bin/sh
# entrypoint.sh: one-click init and privilege drop for OpenCloud on Unraid.
#
# Runs as root (see `USER root` in the Dockerfile) so it can, in order:
#   1. create the config and data dirs and heal their ownership for the target user
#   2. run `opencloud init` once as that user (writes the config on first boot)
#   3. drop to PUID:PGID via gosu and exec `opencloud server`
#
# PUID/PGID default to Unraid's nobody:users (99:100). The OpenCloud env vars
# (IDM_ADMIN_PASSWORD, OC_URL, OC_INSECURE, OC_LOG_LEVEL, PROXY_TLS,
# IDM_CREATE_DEMO_USERS and the rest) are preserved across the gosu drop and
# read by OpenCloud itself; init consumes IDM_ADMIN_PASSWORD on the first run.
set -eu

PUID="${PUID:-99}"
PGID="${PGID:-100}"
CONFIG_DIR="/etc/opencloud"
DATA_DIR="/var/lib/opencloud"
SENTINEL="${DATA_DIR}/.uid-heal"

if [ "$(id -u)" = "0" ]; then
    # Prefix for the commands that run as the target user.
    DROP="gosu ${PUID}:${PGID}"

    # On a fresh Unraid install the bind mounts arrive root-owned. Create them if
    # missing and hand them to the target user.
    mkdir -p "${CONFIG_DIR}" "${DATA_DIR}"

    # The config dir holds a few YAML files and secrets, so a recursive chown is cheap.
    chown -R "${PUID}:${PGID}" "${CONFIG_DIR}"

    # The data dir holds every user blob and can grow huge, so it is not chowned
    # recursively on every boot. The server runs as the target user and owns
    # whatever it creates; a recursive pass is only needed to repair a tree
    # written earlier as root, or after a PUID/PGID change. A sentinel records
    # the last owner, so the expensive pass runs once per PUID:PGID.
    chown "${PUID}:${PGID}" "${DATA_DIR}"
    want="${PUID}:${PGID}"
    if [ ! -f "${SENTINEL}" ] || [ "$(cat "${SENTINEL}" 2>/dev/null)" != "${want}" ]; then
        echo "[entrypoint] healing ownership of ${DATA_DIR} -> ${want} (first run or PUID/PGID change)"
        chown -R "${PUID}:${PGID}" "${DATA_DIR}"
        printf '%s' "${want}" > "${SENTINEL}"
        chown "${PUID}:${PGID}" "${SENTINEL}"
    fi
    # NATS (the internal message bus) is small and has to stay writable by the
    # user even when it first appears after the one-time heal.
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

# OC_URL is the public URL clients use and the built-in IDP's OIDC issuer, so it
# has to be a valid https URL. Unraid substitutes its [IP]/[PORT] tokens only in
# the template's WebUI field, not in env vars, so a template default like
# https://[IP]:[PORT:9200] arrives here verbatim and crashes the reva gateway
# ("invalid IP-literal"). If OC_URL is empty or still holds a bracket or
# placeholder value, derive a usable one from the container's own IP so the
# server boots instead of crash-looping. Set OC_URL to your real server (or
# reverse-proxy) address for logins to work from every client.
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

# OFFICE selects a browser document editor: off (default), collabora, onlyoffice
# or euro-office. The document server always runs as a separate container
# (Collabora CODE, or an OnlyOffice or Euro Office Document Server); this only
# turns on OpenCloud's built-in 'collaboration' (WOPI) service and points it at
# that server. OFFICE_SERVER_URL is the browser-reachable URL of that container,
# OFFICE_WOPI_SECRET a shared secret. The values follow opencloud-compose
# weboffice/collabora.yml. Euro Office is an OnlyOffice fork, so it is driven as
# the "OnlyOffice" product with its own display name (as in opencloud-compose
# weboffice/euro-office.yml).
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
    # server's own JWT secret exactly, so an unset secret gets a warning.
    if [ "${_oc_app_product}" = "OnlyOffice" ] && [ -z "${OFFICE_WOPI_SECRET:-}" ]; then
        echo "[entrypoint] WARNING: OFFICE=${OFFICE} but OFFICE_WOPI_SECRET is empty; documents fail with 'document security token is not correctly formed' until it matches the document server's JWT secret"
    fi
    # tolerate self-signed certs on the doc server and the internal data gateway (LAN default)
    export COLLABORATION_APP_INSECURE="${COLLABORATION_APP_INSECURE:-true}"
    export COLLABORATION_CS3API_DATAGATEWAY_INSECURE="${COLLABORATION_CS3API_DATAGATEWAY_INSECURE:-true}"
    # OnlyOffice signs with its own JWT rather than Collabora-style proof keys
    [ "${_oc_app_product}" = "OnlyOffice" ] && export COLLABORATION_APP_PROOF_DISABLE="${COLLABORATION_APP_PROOF_DISABLE:-true}"
    # register the collaboration app as the secure-view handler and expose the
    # secure-view role (the default role set plus secure-view, from opencloud-compose)
    export FRONTEND_APP_HANDLER_SECURE_VIEW_APP_ADDR="eu.opencloud.api.collaboration"
    export GRAPH_AVAILABLE_ROLES="${GRAPH_AVAILABLE_ROLES:-b1e2218d-eef8-4d4c-b82d-0f1a1b48f3b5,a8d5fe5e-96e3-418d-825b-534dbdf22b99,fb6c3e19-e378-47e5-b277-9732f9de6e21,58c63c02-1d89-4572-916a-870abc5a1b7d,2d00ce52-1fc2-4dbc-8b95-a73b73395f5a,1c996275-f1c9-4e71-abdf-a42f6495e960,312c0871-5ef7-4b3a-85b6-0e4074c64049,aa97fe03-7980-45ac-9e50-b325749fd7e6}"
    # Registering a WOPI app does not add its origin to OpenCloud's own
    # Content-Security-Policy: COLLABORATION_APP_ADDR and the proxy's frame-src
    # allowlist are unrelated settings that happen to need the same value.
    # Without this the browser blocks the editor iframe with a frame-src
    # violation although the WOPI wiring is right, as seen with Euro Office
    # (junkerderprovinz/unraid-apps#7). opencloud-compose wires the same origin
    # into both places by hand (weboffice/*.yml and config/opencloud/csp.yaml).
    # Entries in PROXY_CSP_CONFIG_FILE_LOCATION are added to the built-in CSP,
    # so the file only needs the addition.
    if [ -z "${PROXY_CSP_CONFIG_FILE_LOCATION:-}" ]; then
        # The origin is the scheme plus host[:port], without any path or query.
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
        # The user's own CSP file stays as it is, so the editor origin has to go
        # into its frame-src and img-src by hand.
        echo "[entrypoint] web-office enabled: ${_oc_app_product} at ${OFFICE_SERVER_URL} (collaboration service on)"
        echo "[entrypoint] NOTE: PROXY_CSP_CONFIG_FILE_LOCATION is already set; add ${OFFICE_SERVER_URL} to its frame-src/img-src yourself or the editor iframe will be CSP-blocked"
    fi
elif [ -n "${_oc_app_name}" ]; then
    echo "[entrypoint] OFFICE=${OFFICE} set but OFFICE_SERVER_URL is empty, so web-office stays off"
fi

# FULLTEXT_SEARCH=true turns on content search (inside files, not just names).
# Apache Tika always runs as a separate container (apache/tika, port 9998); this
# only points OpenCloud's search extractor at it and tells the web UI that
# full-text search is available. TIKA_URL is that container's network-reachable
# URL. The values follow the OpenCloud search docs. Only files uploaded or
# changed afterwards get their contents indexed.
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
    echo "[entrypoint] FULLTEXT_SEARCH=true but TIKA_URL is empty, so full-text search stays off"
fi

# BRANDING_APP=true adds an admin-only "Branding" app to the web UI: the extension
# in the apps folder, the proxy route /brandingsvc/ and brandingd on loopback.
# Switching it off removes the editor and keeps the saved branding in effect.
BRANDING_SHARE="/usr/local/share/opencloud-branding"
BRANDING_APPS_DIR="${DATA_DIR}/web/assets/apps/branding"
BRANDING_STATE="${DATA_DIR}/branding/state.json"
BRANDING_PROXY="${CONFIG_DIR}/proxy.yaml"
BRANDING_PROXY_MARKER="# managed by the opencloud Unraid wrapper (BRANDING_APP)"
_branding="$(printf '%s' "${BRANDING_APP:-false}" | tr '[:upper:]' '[:lower:]')"

# Only a regular file whose first line is the marker is ours to rewrite or
# remove, also after an editor on Windows saved it with CRLF or a BOM. A
# symlink always counts as the user's own, so nothing is written through it.
_own_proxy="false"
if [ -L "${BRANDING_PROXY}" ]; then
    _own_proxy="true"
elif [ -f "${BRANDING_PROXY}" ] && [ "$(head -n 1 "${BRANDING_PROXY}" | tr -d '\r\357\273\277')" != "${BRANDING_PROXY_MARKER}" ]; then
    _own_proxy="true"
fi

# OpenCloud drops the whole file when a top-level key appears twice, and it
# only routes through the policy named default. The route counts when
# endpoint, backend and unprotected sit in the same list item of that policy.
if [ "${_branding}" = "true" ] && [ "${_own_proxy}" = "true" ]; then
    if [ -f "${BRANDING_PROXY}" ] && [ "$(awk '/^additional_policies:/ { n++ } END { print n + 0 }' "${BRANDING_PROXY}")" -gt 1 ]; then
        echo "[entrypoint] WARNING: your own ${BRANDING_PROXY} has more than one additional_policies key, so OpenCloud ignores the whole file and the branding app stays off; merge them into one"
        _branding="false"
    elif [ -f "${BRANDING_PROXY}" ] && awk '
        function item_end() { if (ep && be && un && policy == "default") found = 1; ep = 0; be = 0; un = 0 }
        /^[[:space:]]*-[[:space:]]/ { item_end() }
        /^[[:space:]]*(-[[:space:]]+)?name:/ {
            policy = $0
            sub(/^[[:space:]]*(-[[:space:]]+)?name:[[:space:]]*/, "", policy)
            sub(/[[:space:]]*(#.*)?$/, "", policy)
            gsub(/["\047]/, "", policy)
        }
        /^[[:space:]]*(-[[:space:]]+)?endpoint:[[:space:]]*["\047]?\/brandingsvc\/["\047]?[[:space:]]*(#.*)?$/ { ep = 1 }
        /^[[:space:]]*(-[[:space:]]+)?backend:[[:space:]]*["\047]?http:\/\/127\.0\.0\.1:9299\/?["\047]?[[:space:]]*(#.*)?$/ { be = 1 }
        /^[[:space:]]*(-[[:space:]]+)?unprotected:[[:space:]]*(true|True|TRUE)[[:space:]]*(#.*)?$/ { un = 1 }
        END { item_end(); exit !found }
    ' "${BRANDING_PROXY}"; then
        echo "[entrypoint] branding app uses the /brandingsvc/ route from your ${BRANDING_PROXY}"
    else
        echo "[entrypoint] WARNING: BRANDING_APP=true but your own ${BRANDING_PROXY} has no /brandingsvc/ route in the default policy with unprotected: true, so the branding app stays off; add the route from the README"
        _branding="false"
    fi
fi

# The target user does the writing, so a symlink planted in a volume cannot
# make root write outside it. OpenCloud starts even when that fails.
# shellcheck disable=SC2086
if [ "${_branding}" = "true" ]; then
    # Removing the directory itself drops a symlink instead of following it.
    if ${DROP} rm -rf "${BRANDING_APPS_DIR}" \
        && ${DROP} mkdir -p "${BRANDING_APPS_DIR}" "${DATA_DIR}/web/assets/themes/_branding" "${DATA_DIR}/branding" \
        && ${DROP} cp -R "${BRANDING_SHARE}/app/." "${BRANDING_APPS_DIR}/"; then
        if [ "${_own_proxy}" = "false" ]; then
            ${DROP} sh -c 'cat > "$1"' sh "${BRANDING_PROXY}" <<EOF
${BRANDING_PROXY_MARKER}
additional_policies:
  - name: default
    routes:
      - endpoint: /brandingsvc/
        backend: http://127.0.0.1:9299
        unprotected: true
EOF
        fi
    else
        echo "[entrypoint] WARNING: could not install the branding app into ${BRANDING_APPS_DIR}, so it stays off"
        _branding="false"
    fi
fi
# shellcheck disable=SC2086
if [ "${_branding}" != "true" ]; then
    ${DROP} rm -rf "${BRANDING_APPS_DIR}" || echo "[entrypoint] WARNING: could not remove ${BRANDING_APPS_DIR}"
    if [ -f "${BRANDING_PROXY}" ] && [ "${_own_proxy}" = "false" ]; then
        ${DROP} rm -f "${BRANDING_PROXY}"
        echo "[entrypoint] branding app off: removed the managed ${BRANDING_PROXY}"
    fi
fi

# The saved themes list is a copy of the base theme, which a new image can
# change. brandingd also moves a corrupt state.json aside and drops image names
# it does not trust, so the background below is read after it ran.
if [ -f "${BRANDING_STATE}" ]; then
    # shellcheck disable=SC2086
    if BRANDING_DATA_DIR="${DATA_DIR}" ${DROP} /usr/local/bin/brandingd -regenerate; then
        echo "[entrypoint] saved branding regenerated for this image's base theme"
    else
        echo "[entrypoint] WARNING: brandingd -regenerate exited with $?, saved branding left as it was"
    fi
fi

# Any IDP_LOGIN_BACKGROUND_URL hides OpenCloud's login artwork, so it needs a saved image.
_bg_file=""
if [ -f "${BRANDING_STATE}" ]; then
    _bg_file="$(sed -n 's/^  "background": "\([^"]*\)".*/\1/p' "${BRANDING_STATE}")"
    case "${_bg_file}" in
        *[!a-z0-9.-]*) _bg_file="" ;;
    esac
fi

if [ "${_branding}" = "true" ]; then
    _bg_active="false"
    if [ -n "${_bg_file}" ]; then
        export IDP_LOGIN_BACKGROUND_URL="/brandingsvc/login-background"
        _bg_active="true"
    fi
    _scheme="https"
    [ "$(printf '%s' "${PROXY_TLS}" | tr '[:upper:]' '[:lower:]')" = "false" ] && _scheme="http"
    # brandingd reaches OpenCloud through the proxy, on whatever port it listens.
    _proxy_addr="${PROXY_HTTP_ADDR:-0.0.0.0:9200}"
    # shellcheck disable=SC2086
    BRANDING_OPENCLOUD_URL="${_scheme}://127.0.0.1:${_proxy_addr##*:}" \
    BRANDING_DATA_DIR="${DATA_DIR}" \
    BRANDING_LOGIN_BACKGROUND_ACTIVE="${_bg_active}" \
        ${DROP} sh -c 'while :; do
            /usr/local/bin/brandingd || echo "[entrypoint] brandingd exited with $?, restarting in 5s"
            sleep 5
        done' &
    echo "[entrypoint] branding admin app enabled (app menu -> Branding, admins only)"
elif [ -n "${_bg_file}" ]; then
    export IDP_LOGIN_BACKGROUND_URL="/themes/_branding/${_bg_file}"
fi

# First-boot init writes ${CONFIG_DIR}/opencloud.yaml and consumes
# IDM_ADMIN_PASSWORD. On later boots the file exists and init exits non-zero,
# which `|| true` ignores.
#
# 'opencloud init' reads --insecure, a string flag whose default "ask" opens an
# interactive prompt. Without a TTY that prompt never gets an answer and loops
# on EOF, so a fresh install would hang. An explicit value is always passed, and
# stdin comes from /dev/null as a second guard. --force-overwrite stays out,
# because it regenerates every service secret and the admin password on each boot.
echo "[entrypoint] running 'opencloud init' (harmless error if already initialised)"
# shellcheck disable=SC2086
${DROP} opencloud init --insecure "${OC_INSECURE}" </dev/null || true

# House ready banner, the last block this wrapper prints before the OpenCloud
# server takes over the log. Nothing can run after the exec below, so the status
# line marks the handoff itself rather than a verified health check.
/usr/local/bin/print-banner.sh "OpenCloud" "Cloud storage & collaboration platform, plug-and-play for Unraid"
printf '  \033[0;32m✓ OPENCLOUD IS READY\033[0m - Handing off to the OpenCloud server now\n'
echo ""

# Hand off: replace the shell with the server, dropped to the target user.
# shellcheck disable=SC2086
exec ${DROP} opencloud server
