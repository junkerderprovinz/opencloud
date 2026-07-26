# justfile - OpenCloud for Unraid (wrapper image)
# Recipes mirror the real CI flows (see .github/workflows/ and the Dockerfile).
# Run `just --list` to see everything. POSIX sh recipes.

set shell := ["sh", "-euc"]

# Local image tag used by build/smoke/run (CI uses opencloud:smoke-<channel>-<arch>).
IMAGE := "opencloud:dev"

# Show available recipes.
default:
    @just --list

# ---------------------------------------------------------------------------
# Build (two channels select the upstream base via --build-arg BASE=)
# ---------------------------------------------------------------------------

# Build the :production channel (default base pin from the Dockerfile).
build:
    docker build -t {{IMAGE}} .

# Build the :rolling channel (reads the BASE_ROLLING pin from the Dockerfile).
build-rolling:
    #!/usr/bin/env sh
    set -eu
    base=$(grep -oE 'ARG BASE_ROLLING=[^[:space:]]+' Dockerfile | head -1 | cut -d= -f2)
    echo "rolling base: $base"
    docker build --build-arg BASE="$base" -t opencloud:rolling .

# Multi-arch production build (amd64 + arm64) - needs buildx.
build-multi:
    docker buildx build --platform linux/amd64,linux/arm64 -t {{IMAGE}} --load .

# ---------------------------------------------------------------------------
# Smoke / run  (mirrors the CI smoke gate)
# ---------------------------------------------------------------------------

# Assert gosu/opencloud/entrypoint are present, then boot and wait for the banner.
smoke: build
    #!/usr/bin/env sh
    set -eu
    img="{{IMAGE}}"
    echo "== presence gate =="
    docker run --rm --entrypoint /bin/sh "$img" -c 'command -v gosu; command -v opencloud; test -x /usr/local/bin/entrypoint.sh; gosu --version'
    echo "== boot gate =="
    name=oc-smoke
    docker rm -f "$name" >/dev/null 2>&1 || true
    docker run -d --name "$name" -e IDM_ADMIN_PASSWORD=smoketest -e OC_URL=https://localhost:9200 -e OC_INSECURE=true "$img" >/dev/null
    deadline=$((SECONDS + 120))
    while [ "$SECONDS" -lt "$deadline" ]; do
        if docker logs "$name" 2>&1 | grep -q 'OPENCLOUD IS READY'; then
            echo "READY after ${SECONDS}s; server must stay up..."; sleep 8
            [ -n "$(docker ps -q --filter name=$name)" ] && { echo "OK"; docker rm -f "$name" >/dev/null; exit 0; }
            echo "server exited after banner"; docker logs "$name"; docker rm -f "$name" >/dev/null; exit 1
        fi
        [ -n "$(docker ps -q --filter name=$name)" ] || { echo "container exited early:"; docker logs "$name"; docker rm -f "$name" >/dev/null; exit 1; }
        sleep 3
    done
    echo "READY banner not seen within 120s:"; docker logs "$name"; docker rm -f "$name" >/dev/null; exit 1

# Run the image interactively (WebUI on https://localhost:9200, self-signed).
run:
    docker run --rm -it -p 9200:9200 \
        -e IDM_ADMIN_PASSWORD=changeme -e OC_URL=https://localhost:9200 -e OC_INSECURE=true \
        -v "$PWD/.dev-config:/etc/opencloud" -v "$PWD/.dev-data:/var/lib/opencloud" {{IMAGE}}

# ---------------------------------------------------------------------------
# Lint  (mirrors lint.yml)
# ---------------------------------------------------------------------------

# All lint checks.
lint: hadolint shellcheck

# Hadolint the Dockerfile.
hadolint:
    hadolint Dockerfile

# ShellCheck the wrapper scripts.
shellcheck:
    shellcheck -S warning entrypoint.sh print-banner.sh

# ---------------------------------------------------------------------------
# Assets
# ---------------------------------------------------------------------------

# Regenerate the README banners (Node + resvg + opentype.js, global installs).
banner:
    node .github/assets/gen-banner.mjs && node .github/assets/gen-assets.mjs

# Remove the local dev image and smoke container.
clean:
    -docker rm -f oc-smoke 2>/dev/null
    -docker rmi {{IMAGE}} 2>/dev/null
