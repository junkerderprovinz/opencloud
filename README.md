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
  <a href="https://hub.docker.com/r/junkerderprovinz/opencloud"><img src="https://img.shields.io/docker/image-size/junkerderprovinz/opencloud/latest?style=for-the-badge&logo=docker&logoColor=white&label=Size&color=20434f" alt="Image Size" height="36"></a>&nbsp;
  <a href="https://github.com/junkerderprovinz/opencloud/pkgs/container/opencloud"><img src="https://img.shields.io/badge/Arch-amd64%20%7C%20arm64-success?style=for-the-badge&logo=linux&logoColor=white" alt="Arch" height="36"></a>&nbsp;
  <a href="https://opencloud.eu"><img src="https://img.shields.io/badge/Upstream-OpenCloud-20434f?style=for-the-badge&logo=owncloud&logoColor=white" alt="OpenCloud" height="36"></a>&nbsp;
  <a href="https://ca.unraid.net/apps/opencloud-0z4cxjl1rm24ul"><img src="https://img.shields.io/badge/Unraid-Template-f15a2c?style=for-the-badge&logo=unraid&logoColor=white" alt="Unraid" height="36"></a>&nbsp;
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-AGPL--3.0-blue?style=for-the-badge&logo=gnu&logoColor=white" alt="License: AGPL-3.0" height="36"></a>
</p>

<br>

<p align="center">
A plug-and-play Docker image that turns the official <b>OpenCloud</b> server into a genuine
one-click Unraid app: it runs the required first-boot <code>init</code> for you, heals the
appdata permissions and honours Unraid's <code>PUID</code>/<code>PGID</code>. No console,
no <code>chown</code>, no config-file editing required.
</p>

<!-- download-buttons: written by scripts/gen_download_buttons.py -->
<p align="center">
  <a href="https://ca.unraid.net/apps/opencloud-0z4cxjl1rm24ul"><img src="https://raw.githubusercontent.com/junkerderprovinz/opencloud/main/.github/assets/download-buttons/buttons.svg?v=a82cc8264e34#svgView(viewBox(0,0,841.9,245.3))" alt="Install from Unraid&#x27;s Community Applications" width="160" height="46.618"></a>
  &nbsp;
  <a href="https://hub.docker.com/r/junkerderprovinz/opencloud/"><img src="https://raw.githubusercontent.com/junkerderprovinz/opencloud/main/.github/assets/download-buttons/buttons.svg?v=a82cc8264e34#svgView(viewBox(866,0,841.9,245.3))" alt="Run it with Docker" width="160" height="46.618"></a>
  &nbsp;
  <a href="https://github.com/junkerderprovinz/opencloud/releases/latest"><img src="https://raw.githubusercontent.com/junkerderprovinz/opencloud/main/.github/assets/download-buttons/buttons.svg?v=a82cc8264e34#svgView(viewBox(1732,0,841.9,245.3))" alt="Download the source archive" width="160" height="46.618"></a>
</p>
<!-- /download-buttons -->

<br>

<p align="center">
A one-knight job: I build it, keep it running, work through the issues and add what people ask for, until nothing is missing. It is free, with no accounts, no telemetry, no ads and no paid tier. No asterisk anywhere. Nothing readable ever leaves your own walls. Forged on evenings and weekends, with heart and stubbornness.
</p>

<p align="center">
If it has earned a place on your server or computer, toss a coin to your knight: it helps cover the costs and keeps the project alive. It also makes this knight's heart beat a little faster. Three ways below, whichever suits you.
</p>

<!-- give-buttons: written by scripts/gen_download_buttons.py -->
<p align="center">
  <a href="https://buymeacoffee.com/junkerderprovinz"><img src="https://raw.githubusercontent.com/junkerderprovinz/opencloud/main/.github/assets/download-buttons/buttons.svg?v=a82cc8264e34#svgView(viewBox(2598,0,841.9,245.3))" alt="Buy me a coffee" width="160" height="46.618"></a>
  &nbsp;
  <a href="https://www.paypal.com/donate/?hosted_button_id=76FVV52TKXTUS"><img src="https://raw.githubusercontent.com/junkerderprovinz/opencloud/main/.github/assets/download-buttons/buttons.svg?v=a82cc8264e34#svgView(viewBox(3464,0,841.9,245.3))" alt="PayPal" width="160" height="46.618"></a>
  &nbsp;
  <a href="https://junkerderprovinz.github.io/junkerderprovinz/"><img src="https://raw.githubusercontent.com/junkerderprovinz/opencloud/main/.github/assets/download-buttons/buttons.svg?v=a82cc8264e34#svgView(viewBox(4330,0,841.9,245.3))" alt="Donate with crypto" width="160" height="46.618"></a>
</p>
<!-- /give-buttons -->

<br>

## ⚠️ Before you start

> [!IMPORTANT]
> **Keep every OpenCloud folder off `/mnt/user`.** OpenCloud has no database. Its metadata and message bus are files that it writes all the time, and a large sync through Unraid's share layer (any `/mnt/user/...` path) can hang that layer. Every container that uses it hangs along, and only a reboot helps.
>
> | Field | Where it goes | Example |
> |---|---|---|
> | **Config** | the SSD pool itself | `/mnt/cache/appdata/opencloud/config` |
> | **Data** | the SSD pool itself | `/mnt/cache/appdata/opencloud/data` |
> | **Files** (optional) | one array disk, if your files don't fit on the pool | `/mnt/disk1/opencloud/files` |
>
> Use the pool and disk names from your **Main** tab, and set the share behind Files to that one disk. At startup the container log warns about any path that still goes through the share layer. [More in the guide](docs/guide.md#large-folder-sync-from-the-desktop-client-stalls-or-aborts).

<br>

## Table of Contents

1. [What it looks like](#1-what-it-looks-like)
2. [What it does](#2-what-it-does)
3. [Getting started](#3-getting-started)
4. [How AI is used here](#4-how-ai-is-used-here)
5. [Support this project](#5-support-this-project)

<br>

## 1. What it looks like

The files and documents in these pictures are made up.

<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/opencloud/main/.github/assets/screenshots/opencloud-1.png" alt="The OpenCloud web interface in dark mode, showing five folders and a file in the personal space" width="100%">
  <br><em>Your personal space in OpenCloud, in any browser on your network</em>
</p>

<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/opencloud/main/.github/assets/screenshots/opencloud-2.png" alt="A Word document open in Euro Office inside the OpenCloud web interface" width="100%">
  <br><em>Office files open in Euro Office, right inside OpenCloud</em>
</p>

<br>

## 2. What it does

[OpenCloud](https://opencloud.eu) is a self-hosted file sync and share platform from the ownCloud Infinite Scale family. The official [`opencloudeu/opencloud`](https://hub.docker.com/r/opencloudeu/opencloud) image runs as a fixed user without `PUID`/`PGID`, so on Unraid's root-owned folders the first start fails with "permission denied", and it needs `opencloud init` run by hand. This image is the unmodified upstream one with a small entrypoint in front:

- **Starts on the first try.** It runs `opencloud init` once, hands the config and data folders to `PUID`:`PGID` (Unraid's `nobody:users` by default) and drops to that user.
- **Two channels.** `:rolling`, the template default, follows OpenCloud's newest releases, and `:latest` its production line. Both are rebuilt every week, for amd64 and arm64.
- **Web office in three fields.** Pick [Euro Office](https://github.com/junkerderprovinz/euro-office), Collabora or OnlyOffice, and the container wires up OpenCloud's collaboration service, the proxy routes and the Content-Security-Policy. Euro Office runs under OpenCloud's own address, so it needs no certificate of its own.
- **Files apart from Data.** An optional **Files** path keeps your files on an array disk while the message bus, search index and accounts stay on a fast pool. The log warns at startup when either path goes through Unraid's share layer.
- **A search index that cannot stop the server.** An index OpenCloud could not open is moved aside before the start and rebuilt in the background.
- **Branding, if you want it.** An optional app lets admins set the name, slogan, logos, favicon and login background from the web interface.

<br>

## 3. Getting started

1. Install **OpenCloud** from [Community Applications](https://ca.unraid.net/apps/opencloud-0z4cxjl1rm24ul).
2. Set **Admin Password**, and set **Public URL** to the address clients use, for example `https://192.168.1.10:9200`. It has to be https, and Unraid does not fill it in.
3. Set **Config**, **Data** and, if you need it, **Files** as shown under **Before you start** at the top, before the first start.
4. Click **Apply** and wait for `OPENCLOUD IS READY` in the log. Then open the Public URL, accept the self-signed certificate once and sign in as `admin`.

Without Unraid:

```bash
docker run -d --name opencloud \
  -p 9200:9200 \
  -e IDM_ADMIN_PASSWORD='change-me-please' \
  -e OC_URL='https://192.168.1.10:9200' \
  -e OC_INSECURE=true \
  -v /path/to/config:/etc/opencloud \
  -v /path/to/data:/var/lib/opencloud \
  --restart unless-stopped \
  junkerderprovinz/opencloud:rolling
```

**Editing documents.** Install [Euro Office](https://github.com/junkerderprovinz/euro-office) and set its JWT secret. Here, set **Web office suite** to `euro-office`, **Office document server URL** to the address its WebUI button opens, such as `http://192.168.1.10:9900`, and **Office WOPI secret** to the same text as that JWT secret. Collabora and OnlyOffice load the editor straight from their own server, so for them the URL has to be https.

**Behind a reverse proxy** that terminates TLS, set `PROXY_TLS=false`, `OC_URL` to the external address and `OC_INSECURE=false`, and point the proxy at port 9200.

Configuration, Files and S3, the web office, the reverse proxy, updating and troubleshooting are explained step by step in the [guide](docs/guide.md).

<br>

## 4. How AI is used here

One knight builds this, and AI is one of the tools I work with, the same way I work with an editor or a compiler. It helps me write code and documentation and it checks my work, and that saves me a good many evenings. It does not make the decisions, though. I read and understand everything before it ships, and if something here breaks, that is on me and not on the tool.

You do not have to take my word for it. The code is open and every release note is written by hand. The issue tracker shows how problems actually get handled, including the ones I got wrong the first time. If you find something that is not right, open an issue and I will look at it.

<br>

## 5. Support this project

Questions? Check the [support thread](https://forums.unraid.net/topic/200022-support-junkerderprovinz-opencloud/). Bugs, ideas or feature requests? Please [open a GitHub issue](https://github.com/junkerderprovinz/opencloud/issues).

A one-knight job: I build it, keep it running, work through the issues and add what people ask for, until nothing is missing. It is free, with no accounts, no telemetry, no ads and no paid tier. No asterisk anywhere. Nothing readable ever leaves your own walls. Forged on evenings and weekends, with heart and stubbornness.

If it has earned a place on your server or computer, toss a coin to your knight: it helps cover the costs and keeps the project alive. It also makes this knight's heart beat a little faster. Three ways below, whichever suits you.

<!-- give-buttons: written by scripts/gen_download_buttons.py -->
<p align="center">
  <a href="https://buymeacoffee.com/junkerderprovinz"><img src="https://raw.githubusercontent.com/junkerderprovinz/opencloud/main/.github/assets/download-buttons/buttons.svg?v=a82cc8264e34#svgView(viewBox(2598,0,841.9,245.3))" alt="Buy me a coffee" width="160" height="46.618"></a>
  &nbsp;
  <a href="https://www.paypal.com/donate/?hosted_button_id=76FVV52TKXTUS"><img src="https://raw.githubusercontent.com/junkerderprovinz/opencloud/main/.github/assets/download-buttons/buttons.svg?v=a82cc8264e34#svgView(viewBox(3464,0,841.9,245.3))" alt="PayPal" width="160" height="46.618"></a>
  &nbsp;
  <a href="https://junkerderprovinz.github.io/junkerderprovinz/"><img src="https://raw.githubusercontent.com/junkerderprovinz/opencloud/main/.github/assets/download-buttons/buttons.svg?v=a82cc8264e34#svgView(viewBox(4330,0,841.9,245.3))" alt="Donate with crypto" width="160" height="46.618"></a>
</p>
<!-- /give-buttons -->

<br>

<sub>The AGPL-3.0 covers this wrapper only. OpenCloud and the bundled `gosu` are Apache-2.0, see [NOTICE](NOTICE). The OpenCloud logo and wordmark belong to OpenCloud GmbH and are used unmodified to name the upstream project. This packaging is not affiliated with or endorsed by OpenCloud GmbH.</sub>
