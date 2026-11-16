#!/bin/sh
# Boots the image with a stub in place of the opencloud binary and checks the
# environment the entrypoint hands to the server and the csp.yaml it writes.
#
#   tests/entrypoint-test.sh opencloud:dev
set -eu

img="${1:?usage: $0 <image>}"
here="$(cd "$(dirname "$0")" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT

MARKER="# managed by the opencloud Unraid wrapper and rewritten at every start; put your own directives into csp.local.yaml"
failed=0
case_dir=""

# Runs the entrypoint once on a fresh config dir. Files a test puts into
# ${case_dir}/config beforehand show up in /etc/opencloud.
boot() {
    docker run --rm \
        -e PUID="$(id -u)" -e PGID="$(id -g)" \
        -e IDM_ADMIN_PASSWORD=test -e OC_URL=https://cloud.example.com \
        -v "${here}/opencloud-stub.sh:/usr/bin/opencloud:ro" \
        -v "${case_dir}/config:/etc/opencloud" \
        "$@" "${img}" >"${case_dir}/log" 2>&1 || {
        echo "    the container failed:"
        sed 's/^/    | /' "${case_dir}/log"
        return 1
    }
}

new_case() {
    case_dir="${work}/$1"
    mkdir -p "${case_dir}/config"
    current="$1"
    ok=1
}

fail() {
    echo "    $*"
    ok=0
}

done_case() {
    if [ "${ok}" = "1" ]; then
        echo "ok - ${current}"
    else
        echo "not ok - ${current}"
        failed=1
    fi
}

has_env() {
    grep -qxF "ENV $1" "${case_dir}/log" || fail "expected $1, got: $(grep "^ENV ${1%%=*}=" "${case_dir}/log" || echo unset)"
}

lacks_env() {
    if grep -q "^ENV $1=" "${case_dir}/log"; then
        fail "expected $1 unset, got: $(grep "^ENV $1=" "${case_dir}/log")"
    fi
}

csp_is() {
    printf '%s\n' "$1" >"${case_dir}/want"
    if [ ! -f "${case_dir}/config/csp.yaml" ]; then
        fail "no csp.yaml written"
    elif ! diff -u "${case_dir}/want" "${case_dir}/config/csp.yaml" >"${case_dir}/diff"; then
        fail "csp.yaml differs:"
        sed 's/^/    | /' "${case_dir}/diff"
    fi
}

new_case "without a login provider the built-in IdP and its settings stay"
boot && {
    lacks_env OC_OIDC_ISSUER
    lacks_env OC_OIDC_CLIENT_ID
    lacks_env OC_EXCLUDE_RUN_SERVICES
    lacks_env PROXY_AUTOPROVISION_ACCOUNTS
    lacks_env PROXY_OIDC_REWRITE_WELLKNOWN
    lacks_env PROXY_ROLE_ASSIGNMENT_DRIVER
    lacks_env PROXY_USER_OIDC_CLAIM
    lacks_env PROXY_AUTOPROVISION_CLAIM_USERNAME
    lacks_env GRAPH_USERNAME_MATCH
    lacks_env PROXY_OIDC_AUDIENCES
    lacks_env PROXY_CSP_CONFIG_FILE_LOCATION
    [ ! -e "${case_dir}/config/csp.yaml" ] || fail "csp.yaml written without anything to add"
}
done_case

new_case "a login provider exports issuer and client and turns the built-in IdP off"
boot -e OIDC_ISSUER=https://id.example.com/realms/home -e OIDC_CLIENT_ID=OpenCloud -e OIDC_ROLE_CLAIM= && {
    has_env OC_OIDC_ISSUER=https://id.example.com/realms/home
    has_env OC_OIDC_CLIENT_ID=OpenCloud
    has_env OC_EXCLUDE_RUN_SERVICES=idp
    has_env PROXY_OIDC_REWRITE_WELLKNOWN=true
    has_env PROXY_AUTOPROVISION_ACCOUNTS=true
    lacks_env PROXY_ROLE_ASSIGNMENT_DRIVER
    lacks_env PROXY_ROLE_ASSIGNMENT_OIDC_CLAIM
}
done_case

new_case "an empty client ID leaves OpenCloud's own client IDs in place"
boot -e OIDC_ISSUER=https://id.example.com -e OIDC_CLIENT_ID= && {
    has_env OC_OIDC_ISSUER=https://id.example.com
    lacks_env OC_OIDC_CLIENT_ID
}
done_case

new_case "values set by the user win over the login provider defaults"
boot -e OIDC_ISSUER=https://id.example.com \
    -e OC_EXCLUDE_RUN_SERVICES=thumbnails \
    -e PROXY_AUTOPROVISION_ACCOUNTS=false \
    -e PROXY_OIDC_REWRITE_WELLKNOWN=false && {
    has_env OC_EXCLUDE_RUN_SERVICES=thumbnails,idp
    has_env PROXY_AUTOPROVISION_ACCOUNTS=false
    has_env PROXY_OIDC_REWRITE_WELLKNOWN=false
}
done_case

new_case "an IdP the user already excluded is listed once"
boot -e OIDC_ISSUER=https://id.example.com -e OC_EXCLUDE_RUN_SERVICES=idp,thumbnails && {
    has_env OC_EXCLUDE_RUN_SERVICES=idp,thumbnails
}
done_case

new_case "OC_OIDC_ISSUER set by hand counts as the login provider"
boot -e OC_OIDC_ISSUER=https://id.example.com/application/o/opencloud/ && {
    has_env OC_OIDC_ISSUER=https://id.example.com/application/o/opencloud/
    has_env OC_EXCLUDE_RUN_SERVICES=idp
    has_env PROXY_CSP_CONFIG_FILE_LOCATION=/etc/opencloud/csp.yaml
}
done_case

new_case "provider accounts are matched by sub and never by a local username"
boot -e OIDC_ISSUER=https://id.example.com && {
    has_env PROXY_USER_OIDC_CLAIM=sub
    has_env PROXY_AUTOPROVISION_CLAIM_USERNAME=sub
    has_env GRAPH_USERNAME_MATCH=none
    if grep -q "WARNING: .*PROXY_USER_OIDC_CLAIM" "${case_dir}/log"; then
        fail "warned about the default account matching"
    fi
}
done_case

new_case "matching accounts by another claim stays possible and is warned about"
boot -e OIDC_ISSUER=https://id.example.com -e PROXY_USER_OIDC_CLAIM=preferred_username     -e PROXY_AUTOPROVISION_CLAIM_USERNAME=preferred_username -e GRAPH_USERNAME_MATCH=default && {
    has_env PROXY_USER_OIDC_CLAIM=preferred_username
    has_env PROXY_AUTOPROVISION_CLAIM_USERNAME=preferred_username
    has_env GRAPH_USERNAME_MATCH=default
    grep -q "WARNING: .*PROXY_USER_OIDC_CLAIM=preferred_username" "${case_dir}/log" || fail "no warning about local accounts"
}
done_case

new_case "access tokens must be issued to the client ID"
boot -e OIDC_ISSUER=https://id.example.com -e OIDC_CLIENT_ID=OpenCloud && {
    has_env PROXY_OIDC_AUDIENCES=OpenCloud
}
done_case

new_case "without a client ID access tokens must be issued to one of OpenCloud's own clients"
boot -e OIDC_ISSUER=https://id.example.com -e OIDC_CLIENT_ID= && {
    has_env PROXY_OIDC_AUDIENCES=web,OpenCloudDesktop,OpenCloudAndroid,OpenCloudIOS
}
done_case

new_case "own audiences win"
boot -e OIDC_ISSUER=https://id.example.com -e OIDC_CLIENT_ID=OpenCloud -e PROXY_OIDC_AUDIENCES=OpenCloud,account && {
    has_env PROXY_OIDC_AUDIENCES=OpenCloud,account
}
done_case

new_case "no audience check when access tokens are not verified as JWT"
boot -e OIDC_ISSUER=https://id.example.com -e OIDC_CLIENT_ID=OpenCloud -e PROXY_OIDC_ACCESS_TOKEN_VERIFY_METHOD=none && {
    lacks_env PROXY_OIDC_AUDIENCES
}
done_case

new_case "a role claim switches role assignment to the login provider"
boot -e OIDC_ISSUER=https://id.example.com -e OIDC_ROLE_CLAIM=groups && {
    has_env PROXY_ROLE_ASSIGNMENT_DRIVER=oidc
    has_env PROXY_ROLE_ASSIGNMENT_OIDC_CLAIM=groups
}
done_case

new_case "a role claim without a login provider changes nothing"
boot -e OIDC_ROLE_CLAIM=groups && {
    lacks_env PROXY_ROLE_ASSIGNMENT_DRIVER
    lacks_env PROXY_ROLE_ASSIGNMENT_OIDC_CLAIM
}
done_case

new_case "the login provider's origin is allowed for requests and frames"
boot -e OIDC_ISSUER=https://id.example.com:8443/realms/home && {
    has_env PROXY_CSP_CONFIG_FILE_LOCATION=/etc/opencloud/csp.yaml
    csp_is "${MARKER}
directives:
  connect-src:
    - 'https://id.example.com:8443/'
  frame-src:
    - 'https://id.example.com:8443/'"
}
done_case

new_case "office and login provider directives share one csp.yaml"
boot -e OFFICE=collabora -e OFFICE_SERVER_URL=https://office.example.com:9980/hosting \
    -e OIDC_ISSUER=https://id.example.com && {
    has_env PROXY_CSP_CONFIG_FILE_LOCATION=/etc/opencloud/csp.yaml
    csp_is "${MARKER}
directives:
  frame-src:
    - 'https://office.example.com:9980/'
    - 'https://id.example.com/'
  img-src:
    - 'https://office.example.com:9980/'
  connect-src:
    - 'https://id.example.com/'"
}
done_case

new_case "office alone still writes its directives"
boot -e OFFICE=euro-office -e OFFICE_SERVER_URL=http://192.168.1.10:9900 && {
    csp_is "${MARKER}
directives:
  script-src:
    - '''unsafe-eval'''
  worker-src:
    - '''self'''
    - 'blob:'
  font-src:
    - 'data:'"
}
done_case

new_case "csp.local.yaml alone is written to csp.yaml"
cat >"${case_dir}/config/csp.local.yaml" <<'EOF'
directives:
  img-src:
    - 'https://tiles.example.com/'
EOF
boot && {
    has_env PROXY_CSP_CONFIG_FILE_LOCATION=/etc/opencloud/csp.yaml
    csp_is "${MARKER}
directives:
  img-src:
    - 'https://tiles.example.com/'"
}
done_case

new_case "csp.local.yaml from a Windows editor merges per directive without duplicates"
cp "${here}/csp.local.windows.yaml" "${case_dir}/config/csp.local.yaml"
boot -e OFFICE=collabora -e OFFICE_SERVER_URL=https://office.example.com \
    -e OIDC_ISSUER=https://id.example.com && {
    csp_is "${MARKER}
directives:
  frame-src:
    - 'https://office.example.com/'
    - 'https://id.example.com/'
    - 'https://maps.example.com/'
  img-src:
    - 'https://office.example.com/'
    - 'https://tiles.example.com/'
  connect-src:
    - 'https://id.example.com/'
  script-src:
    - '''unsafe-eval'''
  media-src:
    - '''self'''"
}
done_case

new_case "lines in csp.local.yaml that are not directives are skipped with a warning"
printf 'directives:\n  img-src:\n    - https://tiles.example.com/\nfoo: bar\n' >"${case_dir}/config/csp.local.yaml"
boot && {
    csp_is "${MARKER}
directives:
  img-src:
    - 'https://tiles.example.com/'"
    grep -q 'WARNING: .*csp.local.yaml line 4' "${case_dir}/log" || fail "no warning for line 4"
}
done_case

new_case "an own PROXY_CSP_CONFIG_FILE_LOCATION stays untouched"
printf 'directives:\n  img-src:\n    - mine\n' >"${case_dir}/config/my-csp.yaml"
printf 'directives:\n  img-src:\n    - local\n' >"${case_dir}/config/csp.local.yaml"
cp "${case_dir}/config/my-csp.yaml" "${case_dir}/before"
boot -e PROXY_CSP_CONFIG_FILE_LOCATION=/etc/opencloud/my-csp.yaml \
    -e OFFICE=collabora -e OFFICE_SERVER_URL=https://office.example.com \
    -e OIDC_ISSUER=https://id.example.com && {
    has_env PROXY_CSP_CONFIG_FILE_LOCATION=/etc/opencloud/my-csp.yaml
    cmp -s "${case_dir}/before" "${case_dir}/config/my-csp.yaml" || fail "my-csp.yaml was changed"
    [ ! -e "${case_dir}/config/csp.yaml" ] || fail "csp.yaml written next to an own CSP file"
    grep -q "'https://id.example.com/'" "${case_dir}/log" || fail "the log does not name the login provider's origin"
    grep -q "csp.local.yaml is not read" "${case_dir}/log" || fail "the log does not say csp.local.yaml is ignored"
}
done_case

exit "${failed}"
