# OpenCloud guide

Everything beyond the first start: configuration, Files and S3, the web office, production and rolling images, the reverse proxy, branding, building, updating, troubleshooting and how the image is put together.

## Configuration

| Variable | Default | Description |
|---|---|---|
| `IDM_ADMIN_PASSWORD` | *(required)* | Password for the built-in `admin` user, applied on first init. **Set this.** |
| `OC_URL` | `https://192.168.1.10:9200` | **Required.** Public URL clients use to reach OpenCloud, and the OIDC login issuer. Must be `https`. Set your server's real LAN `IP:9200`, or your external hostname behind a reverse proxy. Unraid does not auto-fill this. |
| `OC_INSECURE` | `true` | Accept the container's self-signed cert. Set `false` when a proxy provides a valid cert. |
| `OC_LOG_LEVEL` | `info` | Log verbosity: `info`, `warn`, `error`, `debug`. |
| `IDM_CREATE_DEMO_USERS` | `false` | Seed demo users (test only, unsafe for real use). |
| `PROXY_TLS` | `true` | OpenCloud terminates TLS itself on 9200. Set `false` behind a TLS-terminating proxy (see [§6](#reverse-proxy)). |
| `PROXY_ENABLE_APP_AUTH` | `false` | Let WebDAV clients sign in with a username and an **app token**. Needed by rclone and by phone sync apps, which cannot do the browser sign-in. Off by default; see [§10](#troubleshooting). |
| `BRANDING_APP` | `false` | Add a **Branding** app where admins set the instance name, slogan, logos, favicon and login background. See [§7](#branding). |
| `PUID` | `99` | User ID OpenCloud runs as, Unraid's *nobody*. |
| `PGID` | `100` | Group ID, Unraid's *users*. |

| Port | Purpose | | Volume | Purpose |
|---|---|---|---|---|
| `9200` | HTTPS WebUI / API (self-signed by default) | | `/etc/opencloud` | Config (`opencloud.yaml` + secrets) |
| | | | `/var/lib/opencloud` | Data: user files, index, `nats` bus |
| | | | `/files` | Files (optional): user files kept apart from Data, see [below](#files-in-a-separate-folder-optional) |

> **No database.** OpenCloud is *not* Nextcloud. It has no MySQL/Postgres and needs none. State lives in the local storage tree on the `/var/lib/opencloud` volume plus an embedded NATS bus. Don't add a database container; there's nothing to point it at.

> **Files show up but are greyed out / won't open?** The storage driver is wrong. Keep `STORAGE_USERS_DRIVER=posix` (the default): never leave it blank and never use `local`; both leave files visible but unreadable. Also make sure the Data volume is on a filesystem with extended-attribute support (the Unraid array and cache/pool disks have it). The driver is fixed at first init. To change it, start with a fresh Data folder.

<br>

### Files in a separate folder (optional)

Data holds OpenCloud's message bus, search index and accounts. They write to disk all the time, and on the Unraid array that is slow enough to stall large syncs, so Data belongs on an SSD or cache pool. Your files can live elsewhere. If the pool is too small for them, set **Files** in the template to a share on the array, for example `/mnt/user/opencloud`. The files go there, and Data stays small: in a test with 4 GB in 2,000 files it held 41 MB.

The wrapper notices the mount at `/files`, points OpenCloud's storage there (`STORAGE_USERS_POSIX_ROOT`) and hands the folder to `PUID`:`PGID`. Uploads are staged next to the files, so they still land on the array, but the message bus does not. With an S3 backend this field does nothing, because the files go to the bucket.

Set Files before the first start. On an install that already has files, OpenCloud starts on the new, empty folder, the old files stay in Data without showing up, and the log says so on every start. To move them over:

1. Stop the container: **Docker** tab, click the OpenCloud icon, **Stop**.
2. Open the Unraid terminal (`>_` at the top right). Each account has a folder with the same name in both places:
   ```bash
   ls /mnt/user/appdata/opencloud/data/storage/users/users/ /mnt/user/opencloud/users/
   ```
   Use your own Data and Files paths if they differ from these.
3. Copy the files over without OpenCloud's own `.oc-*` folders, and give them to the user OpenCloud runs as:
   ```bash
   rsync -r --exclude='.oc-*' "/mnt/user/appdata/opencloud/data/storage/users/users/<id>/" "/mnt/user/opencloud/users/<id>/"
   chown -R nobody:users "/mnt/user/opencloud/users/<id>"
   ```
   With your own `PUID`/`PGID`, use those numbers instead of `nobody:users`.
4. Start the container and check that the files are there. Then delete `/mnt/user/appdata/opencloud/data/storage/users/users` to free the space, and the warning goes away.

<br>

### S3 object storage (optional)

OpenCloud can keep file **blobs** in any S3-compatible bucket while the metadata stays local. Set the storage driver to `decomposeds3` and add the connection variables. Keep the driver on `posix` (the default) for normal local storage. Do **not** leave it blank.

For self-hosted, two genuine, actively-maintained S3-compatible stores work well here: **[SeaweedFS](https://github.com/junkerderprovinz/unraid-apps/tree/main/seaweedfs)** (recommended for a single node) and **[Garage](https://github.com/junkerderprovinz/garage)** (built for geo-distributed multi-node clusters, but runs single-node here too). AWS S3, Backblaze B2 and Wasabi work the same way against their own endpoints.

| Variable | Example | Description |
|---|---|---|
| `STORAGE_USERS_DRIVER` | `decomposeds3` | Set to `decomposeds3` for S3 blob storage. Default is `posix` (local): never leave it blank or use `local`, both grey out files. |
| `STORAGE_USERS_DECOMPOSEDS3_ENDPOINT` | `http://192.168.1.10:8333` | S3 endpoint. Internal `http://` URL for self-hosted SeaweedFS/Garage; the provider's `https://` endpoint for AWS/B2/Wasabi. |
| `STORAGE_USERS_DECOMPOSEDS3_REGION` | `default` | `default` for SeaweedFS, `garage` for Garage (its default region), or the provider region (`us-east-1`, …) otherwise. |
| `STORAGE_USERS_DECOMPOSEDS3_ACCESS_KEY` | `…` | Access key ID. |
| `STORAGE_USERS_DECOMPOSEDS3_SECRET_KEY` | `…` | Secret access key. |
| `STORAGE_USERS_DECOMPOSEDS3_BUCKET` | `opencloud` | Bucket name: **create it first**, the container does not. |

**Where those values come from:**

- **SeaweedFS (self-hosted).** In that template, set an **Access Key** and **Secret Key** (see its README's Security note) and optionally a **Pre-create Bucket** name. Those become your access key, secret key and bucket directly, no separate service-account step. Point the endpoint at its S3 port, `http://<seaweedfs-ip>:8333`, with region `default`.
- **Garage (self-hosted).** In that template, set an **Access Key** and **Secret Key** (and optionally a **Bucket**). They are pre-seeded on first boot, no separate CLI step. Point the endpoint at its S3 API port, `http://<garage-ip>:3900`, with region `garage`.
- **AWS S3 / Backblaze B2 / Wasabi.** Create a bucket in the provider console, then create an access key (AWS: an IAM access key; B2/Wasabi: an application/API key). Use the provider's `https://` endpoint and the bucket's region.

> **The metadata always stays local.** `decomposeds3` puts only the blob bytes in S3; the file tree, xattrs and the blob→object mapping live on `/var/lib/opencloud`. That volume is therefore **required and must be backed up even with S3**. Losing it orphans your S3 objects (they are opaque IDs with no folder structure). There is no all-on-S3 mode. OpenCloud's system/metadata store (`STORAGE_SYSTEM_DRIVER`) stays `decomposed` (local) and needs no change.

If uploads fail with a checksum error on a non-AWS endpoint, add `STORAGE_USERS_DECOMPOSEDS3_PUT_OBJECT_DISABLE_CONTENT_SHA256=true`. Both the SeaweedFS and Garage paths use the same generic `decomposeds3` driver this wrapper's S3 support was originally built and verified against. SeaweedFS has since been re-verified live against a real OpenCloud instance after the switch away from MinIO (connectivity, boot health and unauthenticated bucket reachability all confirmed; a fully authenticated file-read round-trip is the one check still outstanding). Garage has not yet been separately re-verified end-to-end.

<br>

### Web office (optional)

OpenCloud can edit documents in the browser, but it ships no office engine: it speaks the **WOPI** protocol to a **separate document-server container**. This wrapper wires that up from three template fields (default off).

**With Euro Office** (recommended):

1. Install the [Euro Office](https://github.com/junkerderprovinz/euro-office) template (search **Euro Office** in Community Applications) and set its **JWT secret**.
2. In this template set **Web office suite** = `euro-office`, **Office document server URL** = the address its WebUI button opens, e.g. `http://192.168.1.10:9900`, and **Office WOPI secret** = the same text as that JWT secret.
3. Apply. The **New** button now offers documents, spreadsheets and presentations, and existing Office files open in the editor.

OpenCloud then serves the editor under its own address at `/euro-office/`, so the browser never talks to Euro Office directly. Plain http works, there is no second certificate to accept, and from outside (reverse proxy, Tailscale) only OpenCloud has to be reachable. An `https://` address such as `https://192.168.1.10:9943` works too.

**With Collabora Online (CODE)** (`collabora`, image `collabora/code`, port 9980, community CA template) or **OnlyOffice Document Server** (`onlyoffice`, image `onlyoffice/documentserver`, `WOPI_ENABLED=true`): the browser loads the editor straight from that server, so **Office document server URL** has to be its **https** address. An `http://` address leaves the editor blank, and the startup log says so. For Collabora, set its WOPI host allowlist (`aliasgroup1` / `domain`) to your OpenCloud URL; it needs no WOPI secret. For OnlyOffice the secret must equal its JWT secret.

Under the hood the wrapper turns on OpenCloud's built-in `collaboration` service, sets the `COLLABORATION_*` variables, registers it as the secure-view/edit handler and exposes the secure-view role. It writes a `csp.yaml` and points `PROXY_CSP_CONFIG_FILE_LOCATION` at it. For Euro Office it also adds the `/euro-office/` and `/sdkjs/` routes to the managed `proxy.yaml`, and the `csp.yaml` allows `'unsafe-eval'` scripts, `blob:` workers and `data:` fonts, which the editor needs and OpenCloud's own policy forbids. That loosens OpenCloud's Content-Security-Policy only while **Web office suite** is `euro-office`. If you keep a `proxy.yaml` of your own, the wrapper leaves it alone and Euro Office is reached directly, so its address then has to be https; if you set `PROXY_CSP_CONFIG_FILE_LOCATION` yourself, the startup log prints the directives to add.

### Full-text search (Apache Tika, optional)

OpenCloud already has a built-in search: out of the box it matches **file and folder names** and metadata (tags, media type, …). It does not look **inside file contents** on its own. **Apache Tika** is not a second search engine, it is a text-extractor that OpenCloud's search service uses to read the text out of documents (PDF, Word, Excel, PowerPoint, ODF, …) so a search word inside a file is found too. (The `TIKA=:tika.yml` / `TIKA_IMAGE` lines you may have seen belong to OpenCloud's official *docker-compose* deployment. This Unraid wrapper has no `.env`; the two template fields below do the wiring instead.)

1. **Run Apache Tika** (its own container). Ready-made **Tika templates exist in Community Applications** (search *Tika*). Install one (image `apache/tika`, port `9998`; a `-full` tag additionally does OCR of scanned images). Note its network-reachable address, e.g. `http://<TIKA_IP>:9998`.
2. **Turn it on:** set **Full-text search (Tika)** to `true` and **Tika server URL** to that address, then **Apply**. The wrapper points OpenCloud's search extractor at Tika and switches full-text search on for you.

Under the hood the wrapper sets `SEARCH_EXTRACTOR_TYPE=tika`, `SEARCH_EXTRACTOR_TIKA_TIKA_URL` and `FRONTEND_FULL_TEXT_SEARCH_ENABLED=true` (plus `SEARCH_EXTRACTOR_CS3SOURCE_INSECURE=true` for the internal LAN cert). Only files uploaded or changed **after** this are content-indexed; existing files are **not** re-indexed automatically, so re-upload or edit a file to test. See the [OpenCloud search docs](https://docs.opencloud.eu/docs/dev/server/Services/search/Search-info/).

<br>

## Production vs Rolling

Two channels are built from this wrapper, differing only in the upstream base image:

| Tag | Base image | For |
|---|---|---|
| `junkerderprovinz/opencloud:rolling` | `opencloudeu/opencloud-rolling:latest` | **Default.** Newest OpenCloud releases (currently 8.x), published about every three weeks. |
| `junkerderprovinz/opencloud:latest` | `opencloudeu/opencloud:latest` | OpenCloud's production line (currently the 7.2.x train), fully QA'd and cut about every six months. `:production` is kept as an alias, same image. |

**Which channel?** Rolling is the default because the production line still carries two problems that bite on Unraid. As of 7.2.x it lacks the incremental-fsync fix (reva#720) for the large-folder sync abort on slow storage (issue #3027), which shipped in 7.3.0. More seriously, it treats a failed postprocessing event publish as fatal and ends the whole server process, so a single transient `nats: timeout` can take the container down; that was fixed in 7.5.0 ([#3347](https://github.com/opencloud-eu/opencloud/pull/3347)), which retries the publish. A bus that stays silent for longer still stops the server on either channel, see [Troubleshooting](#the-log-ends-with-unable-to-publish-event-and-fatal-error---exiting). Since production is cut roughly twice a year, the stable line will not carry the fix for months.

Pick `:latest` instead if you would rather have OpenCloud's fully QA'd line and your data volume already sits on a fast SSD/NVMe pool, which avoids the stall on its own. Switch by changing the **Repository** tag in the Unraid template. Back up your appdata before switching channels. Both channels track OpenCloud's own upstream `:latest` tag directly, and the weekly rebuild picks it up automatically alongside Alpine security patches, with no waiting on a version-bump PR to get merged.

<br>

## How the Wrapper Works

The entrypoint runs as root only long enough to prepare the volumes, then drops to your user:

1. **Permission heal.** Creates `/etc/opencloud` + `/var/lib/opencloud` if missing and `chown`s them to `PUID:PGID`. The config dir is small and always fully healed; the data dir is only `chown -R`'d once (or after a `PUID`/`PGID` change), tracked by a `.uid-heal` sentinel, so a large data set is never recursively re-owned on every boot. The `nats` bus dir is always re-asserted (small, must stay writable).
2. **Branding app.** With `BRANDING_APP=true` the entrypoint copies the web extension into the data volume, writes the managed `proxy.yaml` (unless you have your own, see [§7](#branding)) and starts `brandingd` as `PUID:PGID` on `127.0.0.1:9299`. It also points `IDP_ASSET_PATH` at the image's copy of OpenCloud's login page, unless you set that variable yourself. The copy adds one script, which loads the saved branding from `brandingd`. With `false` it removes the extension and the managed `proxy.yaml`. Whenever a saved branding exists, it also runs `brandingd -regenerate` once, app on or off, so the branding follows the base theme of the current image.
3. **Search index check.** A bleve search index that OpenCloud cannot open would make the `search` service fail five times, and then the whole server stops. `searchindex` reads each index the way bleve loads it, and moves one that would fail to `<name>.broken`, so the service starts with a new, empty index. When a new index appears on data that already had one (after that move, or when an image update changes the index schema), the entrypoint indexes all spaces again in the background once the server is up. See [§10](#troubleshooting).
4. **Init.** Runs `opencloud init` as the target user (writes `opencloud.yaml`, consuming `IDM_ADMIN_PASSWORD`). It is idempotent and harmlessly errors once the config exists.
5. **Hand-off.** Prints the ready banner, then `exec`s `opencloud server` dropped to `PUID:PGID` via a static `gosu` (copied from the upstream `tianon/gosu` image, so the base needs no package manager).

<br>

## Reverse Proxy

By default OpenCloud serves HTTPS itself on `9200` with a self-signed certificate, ideal for a direct LAN install. To put it behind a reverse proxy that terminates TLS (Traefik, NGINX Proxy Manager, SWAG, …):

- set **`PROXY_TLS=false`** (OpenCloud then serves plain HTTP for the proxy to wrap),
- set **`OC_URL`** to your external URL, e.g. `https://cloud.example.com`,
- set **`OC_INSECURE=false`** (your proxy presents a valid certificate),
- point the proxy upstream at the container's port `9200`.

**Desktop or mobile client login returns `403 Forbidden` while the browser works?** The native clients sign in through a loopback OIDC redirect (`redirect_uri=http://127.0.0.1:<port>`, per RFC 8252). Many reverse-proxy "block exploits" filters reject a literal `http://` inside a query string. In **NGINX Proxy Manager** this is the **"Block Common Exploits"** toggle: its `block-exploits.conf` contains `if ($query_string ~ "[a-zA-Z0-9_]=http://") { return 403; }`. The web UI avoids it (its redirect is URL-encoded), the desktop client trips it (plain `http://` loopback). Switch that toggle **off** for the OpenCloud host and the client login succeeds. OpenCloud brings its own auth and CSRF protection, so the crude regex filter is redundant here.

<br>

## Branding

Set **Branding admin app** (`BRANDING_APP`, in the advanced view of the template) to `true` and restart the container. Accounts with the Admin role then find **Branding** in the app menu. Other accounts do not get the entry, and the service behind it refuses their changes. In the app an admin can set:

- the instance name and slogan
- a logo, plus an optional one for dark mode (without it, dark mode uses the logo)
- the favicon
- the background of the login page
- whether the sign-in card is light, dark, or follows the browser

In the web UI, the name appears in the browser tab and the slogan on public link pages and the sign-out page. On the login page, the name goes into the tab title, and name and slogan replace OpenCloud's in the footer. With a name and no slogan, the footer shows only the name. The login page also shows your logo and favicon.

OpenCloud's sign-in card is white. Under **Login page** you can turn it dark in the colours of the web UI's dark theme, or let it follow the visitor's browser setting. The background behind the card stays as it is either way.

Images can be PNG, JPEG, GIF, WebP or SVG, up to 5 MB for each logo, 2 MB for the favicon and 25 MB for the background. SVG files are rebuilt on upload from shapes, paths, text, groups, symbols, gradients, masks, clip paths and embedded images, with their styling in attributes or `style=`. The rebuild drops `<style>` blocks, filters, patterns, markers and anything that could run code, so export logos with presentation attributes rather than CSS classes, or their colours are lost. An SVG has to be UTF-8 without DOCTYPE entities. One that would freeze the browser, such as masks nested in masks or references that loop back on themselves, is refused.

To upload an image, click its preview in the app. The menu next to its heading also resets it to the OpenCloud default. Saved changes need no restart: the app updates the page you have open, and every other page, the login page included, picks them up on its next load. Only switching `BRANDING_APP` on or off needs a container restart.

If you set `IDP_ASSET_PATH` yourself, the wrapper leaves it alone, and the app does not change the login page's title, footer, favicon or card. The page still shows your logo and background, but it learns only at container start whether there is a background, so adding the first one or removing it again needs a restart. The wrapper looks only at the environment variable: while the app is on, its own `IDP_ASSET_PATH` wins over an asset path in `/etc/opencloud/idp.yaml`, so set yours through the variable.

Switching `BRANDING_APP` back to `false` removes the app but keeps your branding. The login page then keeps your logo and background, and its title, footer, favicon and white card go back to OpenCloud's. To go back to the OpenCloud defaults, reset the fields in the app first.

Before you go back to an image without the app, set `BRANDING_APP=false` and start the container once so it removes the app and its route from the managed `proxy.yaml`. An older image leaves both behind: **Branding** stays in the app menu, and its page cannot load. Your name, slogan, logos and favicon carry over to the web UI, but the login page keeps only the logo. If you already switched, delete `/var/lib/opencloud/web/assets/apps/branding` by hand, and `/etc/opencloud/proxy.yaml` too if it starts with `# managed by the opencloud Unraid wrapper`.

The app owns the name, slogan, logo, favicon and the whole `clients.web.themes` list in `/var/lib/opencloud/web/assets/themes/_branding/theme.json`. Other keys in that file, such as `common.urls`, stay as they are. From the first start with the app on, the wrapper rewrites the app's keys from its settings at every start, even with `BRANDING_APP=false`. Hand edits to those keys are replaced, and so is anything OpenCloud's own `/branding/logo` endpoint writes there. A hand-made themes list with custom colours does not survive either. On that first start, a `theme.json` that already sets any of these keys is copied to `/var/lib/opencloud/branding/theme.json.before-branding-<time>`, and the app takes over its name and slogan.

The app talks to its service through the proxy route `/brandingsvc/`. While the app is on, the wrapper writes that route to `/etc/opencloud/proxy.yaml` at every start, and it deletes the file again when you set `BRANDING_APP=false`. To add routes of your own to that file, delete its first line (the marker comment). The file is then yours, and the wrapper leaves it alone.

With your own `proxy.yaml`, the app turns on only if the file carries the route. Add it as one more item under `routes:` of the `- name: default` entry in your `additional_policies`, indented like the items already there:

```yaml
      - endpoint: /brandingsvc/
        backend: http://127.0.0.1:9299
        unprotected: true
```

If the file has no `additional_policies` key yet, add the whole block below instead. Do not add a second `additional_policies` key: OpenCloud then ignores the whole file, your own routes included.

```yaml
additional_policies:
  - name: default
    routes:
      - endpoint: /brandingsvc/
        backend: http://127.0.0.1:9299
        unprotected: true
```

The proxy only uses the route from the `default` policy; anywhere else the app could not load. The route also needs `unprotected: true`, because the login page loads the branding through it before anyone has signed in. When either is missing, the app stays off and the log says why. `unprotected` only skips the proxy's sign-in check, and the service still asks OpenCloud about every change.

The service finds OpenCloud's port through `PROXY_HTTP_ADDR`. To move OpenCloud to another port, set it there, not with `http.addr` in a `proxy.yaml`.

<br>

## Building Locally

```bash
git clone https://github.com/junkerderprovinz/opencloud.git
cd opencloud

# production/latest channel (default base)
docker build -t opencloud:dev .

# rolling channel (reads the BASE_ROLLING pin from the Dockerfile)
docker build --build-arg BASE="$(grep -oE 'ARG BASE_ROLLING=[^[:space:]]+' Dockerfile | cut -d= -f2)" -t opencloud:rolling .

# multi-arch (amd64 + arm64), needs buildx
docker buildx build --platform linux/amd64,linux/arm64 -t opencloud:dev --load .
```

`just build` and `just build-rolling` build the two channels like CI does, and `just lint` runs the checks of the Lint workflow on the Dockerfile, the scripts, brandingd (Go tests without `-race`) and the web extension. `just smoke` runs only the base boot gate, not the branding smoke from CI.

<br>

## Updating

```bash
docker pull junkerderprovinz/opencloud:latest
docker stop opencloud && docker rm opencloud
# re-create with the same template / docker run args
```

On Unraid: **Docker** tab → the container → **Force Update**. Your `/etc/opencloud` and `/var/lib/opencloud` are untouched. The image is rebuilt **weekly** for upstream OpenCloud and Alpine patches.

<br>

## Troubleshooting

### The log says `search index ... moved it to bleve-v5.broken`

OpenCloud's search index was damaged: a file it needs was missing or unreadable. Older images showed this as `error parsing mapping JSON`, `unable to load snapshot` or `metadata missing` in the log, and the `search` service then failed five times and took the whole server down. The wrapper moved the damaged index aside, OpenCloud started with a new one, and the wrapper then indexed all spaces again. `filling the new search index` and `search index rebuilt` in the log mark the start and the end of that run. Until it finishes, search misses some files, and with many files that can take a while.

The `.broken` folder under `search/` in your Data folder is only kept for inspection and can be deleted. If the log says `indexing the spaces failed`, the run is repeated on the next start, or you can start it yourself in the container console:

```bash
opencloud search index --all-spaces --force-rescan --insecure
```

What damages the index is not known yet. Killing the server in the middle of indexing, hundreds of times, did not reproduce it. If it happens to you, please open an issue and attach the file list of the `.broken` folder together with its `store/root.bolt`, which holds the list of index files and no file contents:

```bash
stat -c '%y %s %n' /var/lib/opencloud/search/bleve-v5.broken/store/*
```

A Data folder from another install or storage backend (local vs S3) is a different matter. Those layouts are not interchangeable and there is no in-place migration between backends, so give the container a fresh, empty Data folder and re-upload the files through the web UI.

### "Personal" is gone after copying files into the Data folder

With the default `posix` driver each account's files live in `storage/users/users/<id>/` inside the Data folder, and OpenCloud keeps its own bookkeeping in extended attributes on every file and folder. If the folder of a personal space is replaced or loses those attributes, for example because a folder was copied over it, OpenCloud stops listing that space and **Personal** disappears from the sidebar. OpenCloud does not repair this by itself, and the log shows `node.Xattr .../storage/users/users/<id> user.oc.name: no data available`.

Uploading through the web UI or a sync client is always safe. To copy a large collection in directly, do it like this (tested with OpenCloud 8.0.1 on Unraid 7.3.2):

1. Sign in to OpenCloud once with the account the files are for, so that its personal space exists.
2. Stop the container: **Docker** tab, click the OpenCloud icon, **Stop**. OpenCloud picks up files added from outside when it starts.
3. Open the Unraid terminal (`>_` at the top right) and find the account's folder. The path below is the template's default Data path; use yours if you changed it.
   ```bash
   getfattr -n user.oc.space.alias /mnt/user/appdata/opencloud/data/storage/users/users/*
   ```
   Each folder is listed with `personal/<user name>`. A folder that answers `No such attribute` has lost its attributes, which is the problem described here.
4. Copy into that folder, never onto it, and give the files to the user OpenCloud runs as:
   ```bash
   cp -r "/mnt/user/Documents/." "/mnt/user/appdata/opencloud/data/storage/users/users/<id>/"
   chown -R nobody:users "/mnt/user/appdata/opencloud/data/storage/users/users/<id>"
   ```
   If you set your own `PUID`/`PGID`, use those numbers instead of `nobody:users`. OpenCloud skips files owned by root and logs `permission denied` for them.
5. Start the container. The files appear once the startup scan is done, which takes a few minutes with many files.

If **Personal** is already gone, the dependable way back is a fresh start: stop the container, delete or rename its `config` and `data` folders, start it again, sign in and add the files as above. This also removes the accounts, shares and settings stored in OpenCloud, so only do it on a new install.

### A WebDAV client gets `401 Unauthorized` with an app token that is definitely correct

rclone, a phone sync app or any other WebDAV client is refused with `401 Unauthorized`, while the same account signs in through the browser without trouble. The token is not the problem: **`PROXY_ENABLE_APP_AUTH` is `false` by default**, and with it off the proxy refuses the request before the token is read at all.

Two details give it away, and both are visible in the container log. The `401` comes back in well under a millisecond, far too fast for anything to have been checked, and no line from `auth-app` or `auth-basic` appears anywhere near it: the services are running, they are simply never asked.

Fix: set `PROXY_ENABLE_APP_AUTH=true` and restart the container. Then create the token under **Account → App tokens** in the web interface, and give the client the account's **user name** with that token as the password. The token is a handful of words separated by spaces, so paste it whole rather than retyping it.

This does **not** open WebDAV to account passwords. That is the separate `PROXY_ENABLE_BASIC_AUTH`, which stays off, and an app token can be revoked on its own without touching the password.

### Large-folder sync from the desktop client stalls or aborts

Syncing a large folder (tens of GB) from the desktop client stalls partway and the client connection just drops. On older OpenCloud versions the cause was **slow `fsync` on the Data volume**, not the network. On OpenCloud 8.0.1 the Unraid array did not reproduce it, so if it happens on a current version, also check [the next entry](#the-log-ends-with-unable-to-publish-event-and-fatal-error---exiting). The **Data** volume holds the embedded NATS message bus, the file-tree metadata and the transient upload staging, all of them fsync-heavy. On slow storage (the Unraid array, or any `/mnt/user` share through the shfs FUSE union) the fsync storm freezes, postprocessing fails and the server drops the client (upstream issue [#3027](https://github.com/opencloud-eu/opencloud/issues/3027)).

If every container hangs and only a reboot helps, the share layer itself has hung. That happens even with `appdata` on an SSD: a `/mnt/user/appdata/...` path still goes through shfs unless the share is exclusive (**Settings → Global Share Settings → Permit exclusive shares**). The container log warns about it at start (`Data goes through Unraid's share layer`).

Fix: put the **Data** volume on a **fast SSD/NVMe pool**, not the array, and use the pool path itself, for example `/mnt/cache/appdata/opencloud/data`. If the pool is too small for all your files, keep them on the array with [Files](#files-in-a-separate-folder-optional). With a decomposeds3/S3 backend only this small metadata volume needs fast storage (the file blobs go to your S3 bucket, so it stays small and grows with file count, not size). The reva incremental-fsync change (reva#720) also helps and ships from OpenCloud 7.3.0, which is on the `:rolling` channel ([§4](#production-vs-rolling)).

### The log ends with `unable to publish event` and `fatal error - exiting`

Before those two lines, `postprocessing` and `search` log `failed to get consumer: context deadline exceeded` every five seconds. OpenCloud's internal message bus (NATS, kept in the Data folder) has stopped answering. Postprocessing retries its publish a few times, then ends the whole server on purpose, and the template's `--restart=unless-stopped` starts it again a few seconds later. In a test with OpenCloud 8.0.1, uploads carried on after that restart without any help.

A slow disk alone did not trigger it. Uploads of several GB kept going on the Unraid array at about 20 MB/s and on a disk throttled to 15 MB/s, and a 90 second storage stall only failed the uploads that were running at that moment. OpenCloud did not stop in any of these tests. If you see this log, look at what sits under the Data folder.

Check that the Data path is on a disk. On Unraid, a folder that is not on the array or on a pool lives in RAM, and that includes `/mnt/<name>` when no pool of that name exists. OpenCloud starts there without complaint, but a few GB of uploads fill the memory until the server freezes. Compare the Data path in the template with the pool names on the **Main** tab.

If the container can't be stopped afterwards and only a reboot helps, the storage under Data has stopped responding. A container whose disk hangs ignores even a kill until the disk answers again. Check the SMART status of that disk on the **Main** tab, and before you reboot, download **Tools → Diagnostics**, which holds the system log from the time of the freeze.



<details>
<summary><b>First start seems stuck / WebUI not reachable yet</b></summary>

The first boot runs `opencloud init` and generates a self-signed certificate, so give it a moment. Watch the log for the **OPENCLOUD IS READY** banner, then open `https://<ip>:9200/`.
</details>

<details>
<summary><b>Browser warns about the certificate</b></summary>

That is expected with the default self-signed certificate (`OC_INSECURE=true`). Accept it once, or put OpenCloud behind a reverse proxy with a real certificate (see [§6](#reverse-proxy)).
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

`OC_URL` must exactly match the URL in your browser (scheme + host + port). Set `OC_URL` to your external https URL and `PROXY_TLS=false` (see [§6](#reverse-proxy)).
</details>

<details>
<summary><b>The saved branding is gone</b></summary>

If `/var/lib/opencloud/branding/state.json` is not valid JSON, the container moves it aside as `state.json.invalid-<time>`, names it in the log and falls back to the OpenCloud defaults. Your images stay. Fix the file, rename it back to `state.json` and restart. Do that before you save anything in the app, because a save deletes every image the new settings do not use.
</details>

<br>

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│  opencloudeu/opencloud[:latest] | opencloud-rolling          │
│  (Alpine base + the OpenCloud binary, unmodified)            │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  entrypoint.sh  (runs as root)                         │  │
│  │   ↓ mkdir + chown /etc/opencloud, /var/lib/opencloud   │  │
│  │   ↓ one-time data heal (sentinel-guarded)              │  │
│  │   ↓ BRANDING_APP: extension + proxy route              │  │
│  │   ↓ brandingd -regenerate  (if a branding is saved)    │  │
│  │   ↓ BRANDING_APP: IDP_ASSET_PATH  (login page copy)    │  │
│  │   ↓ BRANDING_APP: brandingd on 127.0.0.1:9299 &        │  │
│  │   ↓ gosu PUID:PGID  opencloud init  (|| true)          │  │
│  │   ↓ print "OPENCLOUD IS READY" banner                  │  │
│  │   ↓ exec gosu PUID:PGID  opencloud server              │  │
│  └────────────────────────────────────────────────────────┘  │
│      multi-stage:  static gosu    ← tianon/gosu              │
│                    brandingd      ← Go build stage           │
│                    web extension  ← Node build stage         │
│                    base theme     ← download from GitHub     │
│                    login page     ← the OpenCloud binary     │
└──────────────────────────────────────────────────────────────┘
```

<br>
