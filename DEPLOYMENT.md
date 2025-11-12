# The New Wiki Deployment Guide

> This document replaces the legacy Gollum/ExaBGP setup notes. It explains how to deploy the Go-based wiki mirror while keeping the original structure for easy comparison.

Thanks to [wiki-ng](https://git.dn42.dev/wiki/wiki-ng), based on their work, [dn42-wiki-go](https://github.com/iedon/dn42-wiki-go) can provide both static site generation and live edit.

## Overview

`dn42-wiki-go` provides a statically rendered, Git-backed documentation site for dn42. The binary can run in two modes:

- **Live mode** (the default) serves HTTP directly, watches the upstream Git repository, and rebuilds pages on change.
- **Static build mod (`live=false or run with --build`)e** produces pre-rendered HTML into `dist/` once, suitable for delivery by any web server.

Nginx or another TLS proxy is still recommended for public access, but the application ships with native HTTPS and webhook support.

## Prerequisites

- A stable dn42-connected host with enough CPU and disk to cache the Git repository and generated site (`~500 MB`).
- Operational Git access to `git.dn42.dev/wiki/wiki` (read-only for mirrors, write access if you contribute edits).
- Software:
  - [Go ≥ 1.24](https://go.dev/dl/) (only needed for building from source).
  - [Git](https://git-scm.com/).
  - Optional but recommended: [Nginx](https://nginx.org/) or [Caddy](https://caddyserver.com/) for TLS termination.
  - Optional: [Docker](https://docs.docker.com/) / [Docker Compose](https://docs.docker.com/compose/) if you prefer containers.
- Solid operational knowledge—production mirrors are expected to run reliably and coordinate with the dn42 maintainers.

## Network

- The daemon listens on the address configured in `config.json` (`listen`, default `:8080`). Use firewalling or a reverse proxy to expose the service on your chosen anycast/unicast IPs.
- You may terminate HTTPS in-process (set `enableTLS`, `tlsCert`, `tlsKey`) or offload TLS to Nginx/Caddy.
- If you operate an anycast mirror, advertise `172.23.0.80/32` and `fd42:d42:d42:80::1/64` only when the health checks succeed. ExaBGP or native BGP daemons work well; see the _Monitoring & Routing_ section.

## Repository Synchronisation

The wiki content lives in Git. The application already performs periodic pulls (configured via `git.pullIntervalSec`) and exposes a webhook for faster updates. You still need an initial clone and credentials.

### Initial clone

```bash
git clone git@git.dn42.dev:wiki/wiki.git repo
```

Populate `config.json` with the same `remote` URL and point `git.localDirectory` at the clone (default `./repo`).

### Optional external sync script

If you want an **independent** watchdog, a minimal cron job keeps the repository fresh:

```bash
#!/bin/sh
set -euo pipefail
cd /srv/dn42-wiki/repo
/usr/bin/git pull --ff-only
/usr/bin/git push  # if you have write access
```

Schedule it every 5–15 minutes. Avoid overlapping with the built-in poll interval to reduce churn.

## Application Setup

1. Copy `config.example.json` to `config.json` and adjust:
   - `live`: set `true` for mirrors that serve HTTP directly; set `false` to produce static HTML.
   - `editable`: mirrors that allow edits should be reachable from dn42 only.
   - `git.remote`: use your git.dn42 credentials. Leave empty to run against a local repository only.
   - `webhook.enabled`: enable for fast sync and provide a shared secret.
   - `webhook.polling`: set `enabled` and `endpoint` if you need active polling. `skipRemoteCert` lets you disable TLS verification for trusted endpoints. See also `dn42notifyd`.
   - `trustedProxies`: add your reverse proxy or load balancer networks.

2. Compile the binary:

```bash
./build.sh
```

3. Launch the service:

```bash
./dn42-wiki-go --config ./config.json
```

The first run performs a full static build into `dist/`. Subsequent requests serve directly from disk, and background pulls rebuild the output as needed.

### Static-only build

If you just need HTML assets:

```bash
./dn42-wiki-go --config ./config.json --build
```

Deploy `dist/` with any web server or object store.

### Systemd unit (example)

See [dn42-wiki-go.service](dn42-wiki-go.service) and [dn42-wiki-go.socket](dn42-wiki-go.socket).

Reload systemd, enable, and start the service.

## Reverse Proxy / TLS

Place a reverse proxy in front of the daemon for certificate management and to present the canonical hostnames. 

The repository includes `nginx-vhost.conf` with a more complete example (HTTP -> HTTPS redirect, QUIC, static asset caching, and API proxying). Adjust:

- `root` so it points at your rendered `dist/` directory when serving static files directly.
- `proxy_pass` directives if you run the Go binary on a different port or on a Unix socket.
- Certificate paths, `X-SiteID`, and anycast/unicast listener addresses to match your deployment.

To keep HPKP/HSTS behaviour consistent with the legacy setup, reuse the same headers. Coordinate DNS and certification with the dn42 Automatic CA workflow when exposing official mirrors.

## Docker & Compose

The repository ships with a multi-stage `Dockerfile` and `docker-compose.yml`.

```bash
# Build and start
docker compose up --build -d

# Logs
docker compose logs -f
```

Bind-mount `./config/config.json` to override settings and use `./data/dist` plus `./data/repo` for persistent state. The container runs as a non-root user and exposes port `8080`.

## Webhooks & Polling

- `/api/webhook/pull` and `/api/webhook/push` require the shared secret. Integrate them with your Git hosting or ExaBGP watchdog to trigger immediate pulls or pushes.
- The optional poller registers with a remote notification service specified in `webhook.polling.endpoint`. Set `skipRemoteCert` only if the endpoint uses self-signed certificates you trust.

## Monitoring & Routing

For anycast mirrors, advertise the service prefix only when the local HTTP endpoint is healthy. A trimmed-down watchdog using ExaBGP:

```bash
#!/bin/sh
set -eu
check() {
  curl -fsS --max-time 5 https://127.0.0.1:8443/ | grep -qi "dn42"  # adjust scheme/port
}

announce() {
  printf 'announce route 172.23.0.80/32 next-hop %s\n' "$NEXT_HOP"
  printf 'announce route fd42:d42:d42:80::/64 next-hop %s\n' "$NEXT_HOP6"
}

withdraw() {
  printf 'withdraw route 172.23.0.80/32 next-hop %s\n' "$NEXT_HOP"
  printf 'withdraw route fd42:d42:d42:80::/64 next-hop %s\n' "$NEXT_HOP6"
}

state=down
while sleep 30; do
  if check; then
    [ "$state" = down ] && { announce; state=up; }
  else
    [ "$state" = up ] && { withdraw; state=down; }
  fi
done
```

Run the script under an ExaBGP `process` stanza. Ensure your IGP routes traffic correctly to the service when announced.

## Maintenance Checklist

- Monitor `dist/` age and `dn42-wiki-go` logs for build errors.
- Keep Go and system packages patched; rebuild after Go security releases.
- Track upstream configuration changes (`config.example.json`) and merge them into your `config.json`.
- Verify TLS certificates before expiry and renew via the Automatic CA process.
- Coordinate with other maintainers (mailing list/IRC) when adding or retiring mirrors.
