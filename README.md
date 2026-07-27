<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://raw.githubusercontent.com/junkerderprovinz/opencloud/main/.github/assets/opencloud-banner-dark.png">
    <img src="https://raw.githubusercontent.com/junkerderprovinz/opencloud/main/.github/assets/opencloud-banner.png" alt="OpenCloud" width="100%">
  </picture>
</p>

<p align="center">
  <a href="https://github.com/junkerderprovinz/opencloud/actions/workflows/build.yml"><img src="https://img.shields.io/github/actions/workflow/status/junkerderprovinz/opencloud/build.yml?branch=main&label=Build&style=for-the-badge&logo=githubactions&logoColor=white" alt="Build" height="36"></a>&nbsp;
  <a href="https://github.com/junkerderprovinz/opencloud/actions/workflows/lint.yml"><img src="https://img.shields.io/github/actions/workflow/status/junkerderprovinz/opencloud/lint.yml?branch=main&label=Lint&style=for-the-badge&logo=githubactions&logoColor=white" alt="Lint" height="36"></a>&nbsp;
  <a href="https://hub.docker.com/r/junkerderprovinz/opencloud"><img src="https://img.shields.io/docker/pulls/junkerderprovinz/opencloud?style=for-the-badge&logo=docker&logoColor=white&label=Pulls&color=20434f" alt="Docker Pulls" height="36"></a>&nbsp;
  <a href="https://hub.docker.com/r/junkerderprovinz/opencloud"><img src="https://img.shields.io/docker/image-size/junkerderprovinz/opencloud/production?style=for-the-badge&logo=docker&logoColor=white&label=Size&color=20434f" alt="Image Size" height="36"></a>&nbsp;
  <a href="https://github.com/junkerderprovinz/opencloud/pkgs/container/opencloud"><img src="https://img.shields.io/badge/Arch-amd64%20%7C%20arm64-success?style=for-the-badge&logo=linux&logoColor=white" alt="Arch" height="36"></a>&nbsp;
  <a href="https://opencloud.eu"><img src="https://img.shields.io/badge/Upstream-OpenCloud-20434f?style=for-the-badge&logo=owncloud&logoColor=white" alt="OpenCloud" height="36"></a>&nbsp;
  <a href="https://unraid.net"><img src="https://img.shields.io/badge/Unraid-Template-f15a2c?style=for-the-badge&logo=unraid&logoColor=white" alt="Unraid" height="36"></a>&nbsp;
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow?style=for-the-badge&logo=opensourceinitiative&logoColor=white" alt="License" height="36"></a>
</p>

<br>

<p align="center">
A plug-and-play Docker image that turns the official <b>OpenCloud</b> server into a genuine
one-click Unraid app: it runs the required first-boot <code>init</code> for you, heals the
appdata permissions and honours Unraid's <code>PUID</code>/<code>PGID</code> — no console,
no <code>chown</code>, no config-file editing required.
</p>

<br>

<p align="center">
  <a href="https://buymeacoffee.com/junkerderprovinz">
    <img src=".github/assets/button-buy-me-a-coffee.svg" alt="Buy me a coffee" width="220">
  </a>
</p>

<br>

## Table of Contents

1. [Overview](#1-overview)
2. [Quick Start](#2-quick-start)
3. [Configuration](#3-configuration)
4. [Production vs Rolling](#4-production-vs-rolling)
5. [How the Wrapper Works](#5-how-the-wrapper-works)
6. [Reverse Proxy](#6-reverse-proxy)
7. [Building Locally](#7-building-locally)
8. [Updating](#8-updating)
9. [Troubleshooting](#9-troubleshooting)
10. [Architecture](#10-architecture)
11. [Contributing / License](#11-contributing--license)
12. [Support this project](#12-support-this-project)
<br>

## 1. Overview

[OpenCloud](https://opencloud.eu) is a modern, self-hosted file sync-and-share platform (an actively developed member of the ownCloud/Infinite-Scale family). The official [`opencloudeu/opencloud`](https://hub.docker.com/r/opencloudeu/opencloud) image is excellent, but it is not built for a one-click NAS install:

- it runs its binary as a **fixed UID with no `PUID`/`PGID` support**, so on a fresh Unraid box the root-owned bind mounts make the very first boot fail with *"permission denied"* writing `/etc/opencloud/opencloud.yaml` and `/var/lib/opencloud/nats`;
- it requires a **one-time `opencloud init`** to be run by hand before `opencloud server` will start.

This image is a **thin wrapper** around the official one that fixes exactly those two things and nothing else:

- **Auto-init** — runs `opencloud init` once on first boot (idempotent on later boots).
- **Permission heal** — creates the config/data dirs and hands them to your `PUID:PGID`, and repairs a previously root-owned tree once (sentinel-guarded, so it never recursively re-`chown`s your whole data set on every start).
- **PUID / PGID** — drops privileges to Unraid's `nobody:users` (99:100) by default via a static `gosu`.
- **Two channels** — `:production` (stable, default) and `:rolling` (newest builds), from the same wrapper.
- **Multi-arch** — amd64 and arm64.

The wrapper does **not** fork, patch or repackage OpenCloud itself — it layers a tiny entrypoint on top of the unmodified upstream image, so you always run real, current OpenCloud.

<br>

## 2. Quick Start

### Step 1 — Install the template

On Unraid: **Apps** → search for **OpenCloud** → **Install**. The Community Applications template is published from the [`unraid-apps`](https://github.com/junkerderprovinz/unraid-apps) feed.

To load it by hand:

```bash
mkdir -p /boot/config/plugins/dockerMan/templates-user && \
curl -fsSL -o /boot/config/plugins/dockerMan/templates-user/my-OpenCloud.xml \
  https://raw.githubusercontent.com/junkerderprovinz/unraid-apps/main/opencloud/opencloud.xml
```

### Step 2 — Set the admin password and paths

In the template, the only field you **must** set is **Admin Password** (`IDM_ADMIN_PASSWORD`) — it becomes the password for the built-in `admin` user on first start. The two volumes default to `/mnt/user/appdata/opencloud/{config,data}`; adjust the data path to a share with room to grow.

### Step 3 — Start and wait for the banner

Hit **Apply**. The first start takes a moment while the container generates its config and a self-signed certificate. Watch the container log for:

```
  OpenCloud
  OPENCLOUD IS READY
```

### Step 4 — Open the WebUI

Open `https://<unraid-ip>:9200/` and accept the self-signed certificate once. Log in as **`admin`** with the password you set.

<details>
<summary>Plain Docker (no Unraid)</summary>

```bash
docker run -d \
  --name opencloud \
  --restart unless-stopped \
  -p 9200:9200 \
  -e PUID=99 -e PGID=100 \
  -e IDM_ADMIN_PASSWORD='change-me-please' \
  -e OC_URL='https://192.168.1.10:9200' \
  -e OC_INSECURE=true \
  -v /mnt/user/appdata/opencloud/config:/etc/opencloud \
  -v /mnt/user/appdata/opencloud/data:/var/lib/opencloud \
  junkerderprovinz/opencloud:production
```

Set `OC_URL` to how clients reach the server (its IP:port, or your proxied hostname).

</details>

<br>

## 3. Configuration

| Variable | Default | Description |
|---|---|---|
| `IDM_ADMIN_PASSWORD` | *(required)* | Password for the built-in `admin` user — applied on first init. **Set this.** |
| `OC_URL` | `https://[IP]:[PORT:9200]` | Public URL clients use to reach OpenCloud. Set to your proxied hostname behind a reverse proxy. |
| `OC_INSECURE` | `true` | Accept the container's self-signed cert. Set `false` when a proxy provides a valid cert. |
| `OC_LOG_LEVEL` | `info` | Log verbosity — `info`, `warn`, `error`, `debug`. |
| `IDM_CREATE_DEMO_USERS` | `false` | Seed demo users (test only — unsafe for real use). |
| `PROXY_TLS` | `true` | OpenCloud terminates TLS itself on 9200. Set `false` behind a TLS-terminating proxy (see [§6](#6-reverse-proxy)). |
| `PUID` | `99` | User ID OpenCloud runs as — Unraid's *nobody*. |
| `PGID` | `100` | Group ID — Unraid's *users*. |

| Port | Purpose | | Volume | Purpose |
|---|---|---|---|---|
| `9200` | HTTPS WebUI / API (self-signed by default) | | `/etc/opencloud` | Config (`opencloud.yaml` + secrets) |
| | | | `/var/lib/opencloud` | Data — user files, index, `nats` bus |

> **No database.** OpenCloud is *not* Nextcloud — it has no MySQL/Postgres and needs none. State lives in the decomposed metadata tree on the `/var/lib/opencloud` volume plus an embedded NATS bus. Don't add a database container; there's nothing to point it at.

<br>

### S3 object storage (optional)

OpenCloud can keep file **blobs** in any S3-compatible bucket (MinIO, AWS S3, Backblaze B2, Wasabi) while the metadata stays local. Set the storage driver to `decomposeds3` and add the connection variables — leave them unset for normal local storage.

| Variable | Example | Description |
|---|---|---|
| `STORAGE_USERS_DRIVER` | `decomposeds3` | Switches blob storage to S3. Unset = local storage (default). |
| `STORAGE_USERS_DECOMPOSEDS3_ENDPOINT` | `http://192.168.1.10:9000` | S3 endpoint. Internal `http://` URL for self-hosted MinIO; the provider's `https://` endpoint for AWS/B2/Wasabi. |
| `STORAGE_USERS_DECOMPOSEDS3_REGION` | `default` | `default` for most MinIO installs; the provider region (`us-east-1`, …) otherwise. |
| `STORAGE_USERS_DECOMPOSEDS3_ACCESS_KEY` | `…` | Access key ID. |
| `STORAGE_USERS_DECOMPOSEDS3_SECRET_KEY` | `…` | Secret access key. |
| `STORAGE_USERS_DECOMPOSEDS3_BUCKET` | `opencloud` | Bucket name — **create it first**, the container does not. |

**Where those values come from:**

- **MinIO (self-hosted).** In the MinIO console click **Create Bucket** (top left) and name it (e.g. `opencloud`) — that is your bucket name. The current MinIO **Community Edition** console has **no Access Keys page** (only the object browser), so for the keys either:
  - **Easiest:** use your MinIO **root** credentials directly — access key = `MINIO_ROOT_USER`, secret key = `MINIO_ROOT_PASSWORD` (the values you set on your MinIO container; check its Docker template).
  - **Cleaner (dedicated, revocable key):** create a service account with the `mc` CLI — `mc alias set my http://<minio-ip>:9000 <root-user> <root-pass>` then `mc admin user svcacct add my <root-user>`, which prints a fresh **Access Key** + **Secret Key**.
  
  Point the endpoint at the **API** port `http://<minio-ip>:9000` (not the `9001` console) with region `default`.
- **AWS S3 / Backblaze B2 / Wasabi.** Create a bucket in the provider console, then create an access key (AWS: an IAM access key; B2/Wasabi: an application/API key). Use the provider's `https://` endpoint and the bucket's region.

> **The metadata always stays local.** `decomposeds3` puts only the blob bytes in S3; the file tree, xattrs and the blob→object mapping live on `/var/lib/opencloud`. That volume is therefore **required and must be backed up even with S3** — losing it orphans your S3 objects (they are opaque IDs with no folder structure). There is no all-on-S3 mode. OpenCloud's system/metadata store (`STORAGE_SYSTEM_DRIVER`) stays `decomposed` (local) and needs no change.

MinIO note: if uploads fail with a checksum error on a non-AWS endpoint, add `STORAGE_USERS_DECOMPOSEDS3_PUT_OBJECT_DISABLE_CONTENT_SHA256=true`. This wrapper's S3 path is verified end-to-end against MinIO (blob lands in the bucket, metadata on the local volume).

<br>

## 4. Production vs Rolling

Two channels are built from this wrapper, differing only in the upstream base image:

| Tag | Base image | For |
|---|---|---|
| `junkerderprovinz/opencloud:production` | `opencloudeu/opencloud` (pinned stable) | **Default.** The stable OpenCloud release line. |
| `junkerderprovinz/opencloud:rolling` | `opencloudeu/opencloud-rolling` (pinned) | Newest features and fixes, faster moving. |

Switch by changing the **Repository** tag in the Unraid template (`:production` → `:rolling`). Back up your appdata before switching channels. Renovate keeps both base pins current, and the weekly rebuild picks up upstream and Alpine security patches.

<br>

## 5. How the Wrapper Works

The entrypoint runs as root only long enough to prepare the volumes, then drops to your user:

1. **Permission heal.** Creates `/etc/opencloud` + `/var/lib/opencloud` if missing and `chown`s them to `PUID:PGID`. The config dir is small and always fully healed; the data dir is only `chown -R`'d once (or after a `PUID`/`PGID` change), tracked by a `.uid-heal` sentinel — so a large data set is never recursively re-owned on every boot. The `nats` bus dir is always re-asserted (small, must stay writable).
2. **Init.** Runs `opencloud init` as the target user (writes `opencloud.yaml`, consuming `IDM_ADMIN_PASSWORD`). Idempotent — it harmlessly errors once the config exists.
3. **Hand-off.** Prints the ready banner, then `exec`s `opencloud server` dropped to `PUID:PGID` via a static `gosu` (copied from the upstream `tianon/gosu` image — no package manager needed in the base).

<br>

## 6. Reverse Proxy

By default OpenCloud serves HTTPS itself on `9200` with a self-signed certificate — ideal for a direct LAN install. To put it behind a reverse proxy that terminates TLS (Traefik, NGINX Proxy Manager, SWAG, …):

- set **`PROXY_TLS=false`** (OpenCloud then serves plain HTTP for the proxy to wrap),
- set **`OC_URL`** to your external URL, e.g. `https://cloud.example.com`,
- set **`OC_INSECURE=false`** (your proxy presents a valid certificate),
- point the proxy upstream at the container's port `9200`.

<br>

## 7. Building Locally

```bash
git clone https://github.com/junkerderprovinz/opencloud.git
cd opencloud

# production channel (default base pin)
docker build -t opencloud:dev .

# rolling channel (reads the BASE_ROLLING pin from the Dockerfile)
docker build --build-arg BASE="$(grep -oE 'ARG BASE_ROLLING=[^[:space:]]+' Dockerfile | cut -d= -f2)" -t opencloud:rolling .

# multi-arch (amd64 + arm64) — needs buildx
docker buildx build --platform linux/amd64,linux/arm64 -t opencloud:dev --load .
```

`just` recipes mirror the CI flows — `just build`, `just build-rolling`, `just smoke`, `just lint`.

<br>

## 8. Updating

```bash
docker pull junkerderprovinz/opencloud:production
docker stop opencloud && docker rm opencloud
# re-create with the same template / docker run args
```

On Unraid: **Docker** tab → the container → **Force Update**. Your `/etc/opencloud` and `/var/lib/opencloud` are untouched. The image is rebuilt **weekly** for upstream OpenCloud and Alpine patches.

<br>

## 9. Troubleshooting

<details>
<summary><b>First start seems stuck / WebUI not reachable yet</b></summary>

The first boot runs `opencloud init` and generates a self-signed certificate — give it a moment. Watch the log for the **OPENCLOUD IS READY** banner, then open `https://<ip>:9200/`.
</details>

<details>
<summary><b>Browser warns about the certificate</b></summary>

That is expected with the default self-signed certificate (`OC_INSECURE=true`). Accept it once, or put OpenCloud behind a reverse proxy with a real certificate (see [§6](#6-reverse-proxy)).
</details>

<details>
<summary><b>"permission denied" in the log</b></summary>

The wrapper heals ownership on start, but a data set created earlier as a different user can need a one-time repair. Stop the container, delete `/var/lib/opencloud/.uid-heal`, and start again to force a full re-`chown` to your `PUID:PGID`.
</details>

<details>
<summary><b>I forgot / want to change the admin password</b></summary>

`IDM_ADMIN_PASSWORD` is read on every start and overrides the stored admin password, so just set it in the template and restart.
</details>

<details>
<summary><b>Login loops or "redirect URI" errors behind a proxy</b></summary>

`OC_URL` must exactly match the URL in your browser (scheme + host + port). Set `OC_URL` to your external https URL and `PROXY_TLS=false` (see [§6](#6-reverse-proxy)).
</details>

<br>

## 10. Architecture

```
┌──────────────────────────────────────────────────────────────┐
│  opencloudeu/opencloud[:production] | opencloud-rolling        │
│  (Alpine base + the OpenCloud binary, unmodified)             │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  entrypoint.sh  (runs as root)                          │  │
│  │   ↓ mkdir + chown /etc/opencloud, /var/lib/opencloud    │  │
│  │   ↓ one-time data heal (sentinel-guarded)               │  │
│  │   ↓ gosu PUID:PGID  opencloud init  (|| true)           │  │
│  │   ↓ print "OPENCLOUD IS READY" banner                   │  │
│  │   ↓ exec gosu PUID:PGID  opencloud server               │  │
│  └────────────────────────────────────────────────────────┘  │
│      static gosu  ← COPY --from=tianon/gosu (multi-stage)     │
└──────────────────────────────────────────────────────────────┘
```

<br>

## 11. Contributing / License

Pull requests welcome. Issues: <https://github.com/junkerderprovinz/opencloud/issues>.

**Licensing — dual:**

- This **wrapper repository** (Dockerfile, `entrypoint.sh`, `print-banner.sh`, Unraid template, README and banner/icon artwork) is licensed under the [MIT License](LICENSE).
- **OpenCloud itself** and the bundled `gosu` binary are **Apache-2.0**; the Alpine base and its packages keep their own licenses. When you run, redistribute or rebuild the resulting image you must comply with **all** of those, not only this wrapper's MIT license. See [`NOTICE`](NOTICE).

The OpenCloud logo and wordmark are the property of OpenCloud GmbH, used unmodified to identify the upstream project. This is an independent, community-maintained packaging and is **not affiliated with or endorsed by OpenCloud GmbH**.

### Credits

- [**OpenCloud**](https://opencloud.eu) — the file sync-and-share platform this image wraps
- [**gosu**](https://github.com/tianon/gosu) — clean, static privilege-drop for the entrypoint

<br>

## 12. Support this project

If this template saves you a setup hassle or a debug night, consider buying me a coffee:

<p align="center">
  <a href="https://buymeacoffee.com/junkerderprovinz">
    <img src=".github/assets/button-buy-me-a-coffee.svg" alt="Buy me a coffee" width="220">
  </a>
</p>

---

<sub>Part of a family of self-hosted Unraid apps + plugins by <b>junkerderprovinz</b> — see them all at <a href="https://github.com/junkerderprovinz">github.com/junkerderprovinz</a>, or install from <a href="https://unraid.net/community/apps">Community Applications</a>.</sub>
