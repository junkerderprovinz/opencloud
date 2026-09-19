# justfile for the OpenCloud Unraid wrapper image.
# `just lint` runs the checks of lint.yml locally, with the Go tests run without
# -race, which needs cgo. `just smoke` runs only the base boot gate of
# build.yml, not the branding smoke. Run `just --list` to see everything.
# POSIX sh recipes.

set shell := ["sh", "-euc"]

# Local image tag used by build/smoke/run (CI uses opencloud:smoke-<channel>-<arch>).
IMAGE := "opencloud:dev"

# Show available recipes.
default:
    @just --list

# Build the :production channel (the default BASE from the Dockerfile).
build:
    docker build -t {{IMAGE}} .

# Build the :rolling channel (reads the BASE_ROLLING pin from the Dockerfile).
build-rolling:
    #!/usr/bin/env sh
    set -eu
    base=$(grep -oE 'ARG BASE_ROLLING=[^[:space:]]+' Dockerfile | head -1 | cut -d= -f2)
    echo "rolling base: $base"
    docker build --build-arg BASE="$base" -t opencloud:rolling .

# Multi-arch production build (amd64 and arm64), needs buildx.
build-multi:
    docker buildx build --platform linux/amd64,linux/arm64 -t {{IMAGE}} --load .

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
    i=0
    while [ "$i" -lt 40 ]; do
        if docker logs "$name" 2>&1 | grep -q 'OPENCLOUD IS READY'; then
            echo "READY after about $((i * 3))s; server must stay up..."; sleep 8
            [ -n "$(docker ps -q --filter name=$name)" ] && { echo "OK"; docker rm -f "$name" >/dev/null; exit 0; }
            echo "server exited after banner"; docker logs "$name"; docker rm -f "$name" >/dev/null; exit 1
        fi
        [ -n "$(docker ps -q --filter name=$name)" ] || { echo "container exited early:"; docker logs "$name"; docker rm -f "$name" >/dev/null; exit 1; }
        sleep 3
        i=$((i + 1))
    done
    echo "READY banner not seen within 120s:"; docker logs "$name"; docker rm -f "$name" >/dev/null; exit 1

# Run the image interactively (WebUI on https://localhost:9200, self-signed).
run:
    docker run --rm -it -p 9200:9200 \
        -e IDM_ADMIN_PASSWORD=changeme -e OC_URL=https://localhost:9200 -e OC_INSECURE=true \
        -v "$PWD/.dev-config:/etc/opencloud" -v "$PWD/.dev-data:/var/lib/opencloud" {{IMAGE}}

# The checks of lint.yml.
lint: hadolint shellcheck test-branding build-branding-web

# Hadolint the Dockerfile, failing on warnings like CI.
hadolint:
    hadolint --failure-threshold warning Dockerfile

# ShellCheck the wrapper scripts.
shellcheck:
    shellcheck -S warning entrypoint.sh print-banner.sh

# gofmt, vet and tests for brandingd.
test-branding:
    cd branding/server && { test -z "$(gofmt -l .)" || { gofmt -l .; exit 1; }; } && go vet ./... && go test ./...

# Type check, test and build the web extension; OpenCloud 7.2 lacks web-client/ox.
build-branding-web:
    cd branding/web && pnpm install --frozen-lockfile && pnpm check:types && pnpm test:unit --run && pnpm build && ! grep -rqE "@opencloud-eu/web-client/ox" src tests

# Regenerate the README banners (Node + resvg + opentype.js, global installs).
banner:
    node .github/assets/gen-banner.mjs && node .github/assets/gen-assets.mjs

# Remove the local dev image and smoke container.
clean:
    -docker rm -f oc-smoke 2>/dev/null
    -docker rmi {{IMAGE}} 2>/dev/null
