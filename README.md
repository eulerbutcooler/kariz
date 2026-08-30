# surang

A self-hostable tunnel server. Expose services running on your machine through a public URL, like ngrok, except you own the server.

`surang-server` runs on a VPS with a wildcard domain. `surang-client` runs on your machine, dials out to the server, and every request to your assigned subdomain gets routed down the tunnel to your local port.

## Hosted service

No server needed to try it:

```sh
sh -c "$(curl -fsSL https://raw.githubusercontent.com/eulerbutcooler/surang/main/scripts/install.sh)"
surang-client login
```

Server address: `https://api.surang.eulerbutcooler.xyz`. Pick "create an account", enter your email and password, choose a token expiry (1h, 1d, 1w, never).

Open a tunnel:

```sh
python3 -m http.server 3000   # or any local service
surang-client -tunnel web=localhost:3000
```

Output:

```
✓ web  →  https://crown-cycle-dimmed.surang.eulerbutcooler.xyz
```

Anyone can reach your `localhost:3000` through that URL. Add more with another `-tunnel api=localhost:4000`. Stop with ctrl+c; the subdomain is released automatically.

## Features

- multiple tunnels per client over one persistent outbound connection, works behind NAT
- random 3 word subdomains, hyphenated so one wildcard certificate covers them
- accounts with email and password, API tokens with selectable expiry (1h, 1d, 1w, never)
- TUI login that saves your token locally, run tunnels without pasting secrets
- TLS issued and renewed automatically via Let's Encrypt DNS-01 with Cloudflare

## Modes

The server has two modes, switched by one flag.

**Plaintext (local dev).** Run without `-tls-domain`:

```sh
surang-server -domain surang.xyz -db surang.db
```

Public HTTP on :8080, API on :9000, control on :5555, all plaintext. Client login: `http://localhost:9000`.

**TLS (public deployment).** Add `-tls-domain` and `-tls-email`, and a Cloudflare API token (Zone:DNS:Edit) in the `CF_DNS_API_TOKEN` environment variable:

```sh
surang-server -control :5555 -http :8080 -api :9000 \
  -domain example.com -tls-domain example.com -tls-email you@example.com \
  -db /var/lib/surang/surang.db
```

The server gets one wildcard certificate for `example.com` and `*.example.com`, binds :443 for tunnels and `api.example.com`, :80 redirects to https, and wraps the control port in TLS. Certificates are cached next to the database and renewed automatically.

The client mirrors the mode: an https API address means TLS control on the same host, an http API address means plaintext. Nothing else to configure.

## Quick start (self-hosted)

### 1. run the server

With Docker (TLS):

```sh
docker run -d --name surang \
  -e CF_DNS_API_TOKEN=your_cloudflare_token \
  -p 443:443 -p 80:80 -p 5555:5555 -p 9000:9000 \
  -v surang-data:/data \
  ghcr.io/eulerbutcooler/surang-server:latest \
  -domain surang.xyz -tls-domain surang.xyz -tls-email you@example.com -db /data/surang.db
```

Or build it: `docker build -t surang .`.

Or binary + systemd: download `surang-server-linux-amd64` from [releases](https://github.com/eulerbutcooler/surang/releases), copy `deploy/surang.service` to `/etc/systemd/system/`, put `CF_DNS_API_TOKEN=...` in `/etc/default/surang`, and edit the domain flags in `ExecStart`.

Point two DNS A records at the server IP, DNS-only (grey cloud in Cloudflare; the proxy would break TLS termination and the control port): `surang.xyz` and `*.surang.xyz`.

Open ports 443, 80 and 5555 in the host firewall and in the cloud security list. Ports 443 and 80 are privileged; `deploy/surang.service` runs as an unprivileged user and grants `CAP_NET_BIND_SERVICE` for them.

### 2. create an account

```sh
surang-client login
```

Plaintext server: `http://<server>:9000`. TLS server: `https://api.<domain>` (the `api` subdomain is served on 443). The token is saved to `~/.surang/config.json`; you do this once per machine.

### 3. open a tunnel

```sh
surang-client -tunnel web=localhost:3000
```

The control address is derived from the login config, so no server flag is needed. `-server host:port` overrides it for local dev.

## Ports

| Port | Mode | Role |
|---|---|---|
| 5555 | both | control: tunnel registration and data (TLS when the server runs with `-tls-domain`) |
| 9000 | both | account API: signup, login, token issuance (TLS in TLS mode) |
| 443 | TLS | public traffic: tunnel subdomains and `api.<domain>` |
| 80 | TLS | redirects to https |
| 8080 | plaintext | public traffic in plaintext mode |

## API

```sh
# create account
curl -X POST https://api.surang.eulerbutcooler.xyz/api/signup \
  -d '{"email":"you@example.com","password":"hunter2"}'

# login, returns a session token (10 min)
curl -X POST https://api.surang.eulerbutcooler.xyz/api/login \
  -d '{"email":"you@example.com","password":"hunter2"}'

# mint an API token, needs the session token as Bearer
curl -X POST https://api.surang.eulerbutcooler.xyz/api/tokens \
  -H "Authorization: Bearer <session_token>" \
  -d '{"expires":"1d"}'
```

Self-hosted plaintext: same paths on `http://<server>:9000`. Self-hosted TLS: `https://api.<domain>`.

Tokens are stored as sha256 hashes. Passwords are bcrypt. Revoking a token or letting it expire makes it indistinguishable from one that never existed.

## Installing the client

```sh
sh -c "$(curl -fsSL https://raw.githubusercontent.com/eulerbutcooler/surang/main/scripts/install.sh)"
```

Installs `surang-client` to a PATH directory (linux/macOS), or pin a version with `sh scripts/install.sh v0.2.1` once you have the script. Read the script before piping it.

Or grab the right binary from [releases](https://github.com/eulerbutcooler/surang/releases). CI builds static binaries (CGO_ENABLED=0) for linux, macOS and windows on every `v*` tag. The server is not meant to be installed this way; run it under systemd or Docker.

## How it works

The client dials the server's control port and keeps the connection open (outbound, so NAT is not a problem). That connection is a yamux session: one TCP connection carrying many streams. The server maps each public subdomain to a session in an in-memory registry. When a visitor hits a subdomain, the server opens a new stream on that session, writes a dial frame naming the tunnel label, and relays raw bytes. The client reads the label, dials the local service, and pipes both directions. The tunnel never inspects HTTP; it moves bytes.

```
browser ──▶ server :443 ──▶ yamux stream ──▶ client ──▶ localhost:3000
browser ◀── server :443 ◀── yamux stream ◀── client ◀── local response
```

The public listener is :8080 in plaintext mode, :443 in TLS mode.

State: the registry is in memory and dies with the process (tunnels are ephemeral by design). Users and tokens live in sqlite on disk, stored as hashes.

## License

MIT