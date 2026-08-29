# surang

A self-hostable tunnel server. Expose services running on your machine through a public URL, like ngrok, except you own the server.

`surang-server` runs on a VPS with a wildcard domain. `surang-client` runs on your machine, dials out to the server, and every request to your assigned subdomain gets routed down the tunnel to your local port.

## Features

- multiple tunnels per client over one persistent connection
- random 3 word subdomains, minted by the server from a 1024 word list (`crown.cycle.dimmed.surang.xyz`)
- user accounts with email and password, API tokens with selectable expiry (1h, 1d, 1w, never)
- TUI login that saves your token locally, run tunnels without pasting secrets
- single static binary per platform, zero runtime dependencies

## Status

Works over plain HTTP. TLS (LetsEncrypt DNS-01 wildcard) is implemented in `internal/certs` but not wired into the listeners yet. Until then, do not point a public deployment at the internet with real passwords.

## Quick start

### 1. run the server (VPS)

With Docker:

```sh
docker run -d --name surang \
  -p 5555:5555 -p 80:8080 -p 9000:9000 \
  -v surang-data:/data \
  ghcr.io/eulerbutcooler/surang-server:latest \
  -domain surang.xyz -db /data/surang.db
```

Point two DNS A records at the server IP: `surang.xyz` and `*.surang.xyz`.

Or without Docker, grab `surang-server-linux-amd64` from [releases](https://github.com/eulerbutcooler/surang/releases) and run it under systemd. A unit file is in `deploy/`.

### 2. create an account (your machine)

```sh
surang-client login
```

Pick "create an account", enter the server API address, your email and password, and a token expiry. The token gets saved to `~/.surang/config.json`. You do this once per machine.

### 3. open a tunnel

```sh
surang-client -tunnel web=localhost:3000
```

Output:

```
╭─────────────────────────────────────────────────────────╮
│ ✓ web  →  http://crown.cycle.dimmed.surang.xyz          │
╰─────────────────────────────────────────────────────────╯
```

Anyone can now reach your `localhost:3000` through that URL. Add more with another `-tunnel api=localhost:4000`. Stop with ctrl+c; the subdomain is released automatically.

## Ports

| Port | Flag | Who uses it |
|---|---|---|
| 5555 | `-control` | surang-client only, tunnel registration and data |
| 8080 | `-http` | visitors, routed by Host header |
| 9000 | `-api` | signup, login, token issuance |

## API

```sh
# create account
curl -X POST http://surang.xyz:9000/api/signup \
  -d '{"email":"you@example.com","password":"hunter2"}'

# login, returns a session token (10 min)
curl -X POST http://surang.xyz:9000/api/login \
  -d '{"email":"you@example.com","password":"hunter2"}'

# mint an API token, needs the session token as Bearer
curl -X POST http://surang.xyz:9000/api/tokens \
  -H "Authorization: Bearer <session_token>" \
  -d '{"expires":"1d"}'
```

Tokens are stored as sha256 hashes. Passwords are bcrypt. Revoking a token or letting it expire makes it indistinguishable from one that never existed.

## Installing the client

```sh
go build ./...
./scripts/release.sh v0.1.0   # builds all platforms into dist/
```

Cross compiles with `CGO_ENABLED=0`, so every binary is static. CI builds and attaches binaries to GitHub Releases on every `v*` tag.

Or with the install script (client only, linux/macOS):

```sh
curl -fsSL https://raw.githubusercontent.com/eulerbutcooler/surang/main/scripts/install.sh | sh
```

Installs `surang-client` to a PATH directory. Read the script before piping it. The server is not meant to be installed this way; run it under systemd or Docker.

## Docker

```sh
docker build -t surang .
docker run -d --name surang -p 5555:5555 -p 80:8080 -p 9000:9000 \
  -v surang-data:/data surang -domain surang.xyz -db /data/surang.db
```

## How it works

The client dials the server's control port and keeps the connection open (outbound, so NAT is not a problem). That connection is a yamux session: one TCP connection carrying many streams. The server maps each public subdomain to a session in an in-memory registry. When a visitor hits a subdomain, the server opens a new stream on that session, writes a dial frame naming the tunnel label, and relays raw bytes. The client reads the label, dials the local service, and pipes both directions. The tunnel never inspects HTTP; it moves bytes.

```
browser ──▶ server :8080 ──▶ yamux stream ──▶ client ──▶ localhost:3000
browser ◀── server :8080 ◀── yamux stream ◀── client ◀── local response
```

State: the registry is in memory and dies with the process (tunnels are ephemeral by design). Users and tokens live in sqlite on disk, stored as hashes.

## License

MIT
