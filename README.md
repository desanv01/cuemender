# LiveCheck — P01 foundation

LiveCheck is a local Windows stream-diagnostics application. This repository
currently contains the P01 foundation only: a small Go executable that serves
embedded private-control and public-overlay pages, protects private state with a
one-time local session, stores validated expectations in SQLite, and shuts down
cleanly.

It does **not** connect to Twitch or OBS yet. Diagnostics, approvals and the
supported track correction are later packets.

## Requirements

- Windows 11 x64 to run.
- Go 1.27.3 to build. End users do not need Go or Node.
- A modern browser for the private control page.

This workspace includes a portable, checksum-verified Go toolchain under
`.tools/go1.27.3/`; `.tools/` and `dist/` are intentionally ignored by Git.

## Build from this workspace

Run PowerShell from the repository root:

```powershell
$env:GOROOT = "$PWD\.tools\go1.27.3\go"
$env:GOPATH = "$PWD\.tools\gopath"
$env:GOMODCACHE = "$env:GOPATH\pkg\mod"
$env:GOCACHE = "$PWD\.tools\gocache"
$env:GOTMPDIR = "$PWD\.tools\gotmp"
$env:PATH = "$env:GOROOT\bin;$env:PATH"

go version
go mod tidy
gofmt -w ./cmd ./internal ./web
go test ./...
go vet ./...
$env:CGO_ENABLED = "0"
go build -trimpath -o .\dist\livecheck.exe .\cmd\app
Remove-Item Env:CGO_ENABLED
```

The production build is CGo-free. A race run needs a supported C toolchain and
is not required for the standard P01 build.

## Run

```powershell
.\dist\livecheck.exe --no-browser
```

The executable:

1. Creates the data directory (default: `%AppData%\LiveCheck`, or `--data-dir`).
2. Loads `config.json` from that directory, or uses loopback defaults.
3. Serves the private control page at `/control/` and the public overlay at
   `/overlay/`.
4. Prints a single-use private control link to the interactive console.

Open the printed link within five minutes. The token is in the URL fragment; the
browser clears it before exchanging it for a local session cookie. Restart the
application if the link expires or has already been used.

### Flags

| Flag | Behavior |
|---|---|
| `--data-dir PATH` | Isolate configuration and SQLite data. Supports spaces and Unicode. |
| `--listen ADDRESS` | Override the loopback listen address, e.g. `127.0.0.1:8788`. |
| `--no-browser` | Do not open a browser; print the link for manual use. |
| `--session-only-secrets` | Explicit memory-only credential mode for this process. |
| `--version` | Print version/build metadata and exit without touching data. |

`--mode` and future integration flags are intentionally not accepted in P01.

## Configuration

`config.json` is optional. Missing configuration uses:

```json
{
  "schema_version": 1,
  "listen_address": "127.0.0.1:8787",
  "twitch_client_id": "",
  "obs_address": "ws://127.0.0.1:4455"
}
```

Only explicit loopback IPs are accepted for `listen_address` and `obs_address`.
URL credentials, remote addresses, unknown fields and unsupported schema versions
are rejected. A malformed file is reported and left untouched.

## Private HTTP boundary (P01)

- `GET /healthz` — minimal process status.
- `GET /control/` — static private-control shell (no private data).
- `POST /api/v1/session/bootstrap` — one-time token exchange.
- `GET /api/v1/state` — authenticated private snapshot with CSRF token.
- `POST /api/v1/settings` — revision-checked expectations update.
- `GET /api/v1/events` — authenticated private SSE.
- `GET /overlay/` — static public overlay.
- `GET /public/v1/state` — sanitized public snapshot.
- `GET /public/v1/events` — sanitized public SSE.

Private mutations require the session cookie, a same-origin `Origin`, JSON
content and the `X-CSRF-Token` header. Request bodies are limited to 64 KiB.
Public responses never include settings, source names, endpoints or credentials.

The settings update is a full replacement of the expectations record:

```json
{
  "expected_revision": 0,
  "microphone": { "name": "Mic/Aux", "uuid": "" },
  "stream_track": 1,
  "visual": {
    "scene": "Main",
    "scene_item_id": "42",
    "source": { "name": "Camera" }
  }
}
```

Use `null` for an unconfigured microphone, track or visual expectation. The
server records `track_confirmed_at` when a track is supplied. Revision conflicts
return HTTP 409 and never overwrite newer data.

## Data and security notes

- SQLite: `<data-dir>\livecheck.db`; schema version 1 creates
  `schema_migrations` and `app_settings` only.
- Credentials: P01 stores none. Without `--session-only-secrets`, the OS keyring
  is probed; if unavailable, the UI reports it and credential-dependent work is
  disabled. There is no plaintext fallback.
- The server is loopback-only. Loopback HTTP is not a defense against malware or
  another privileged process on the same computer.
- Private pages use a self-only CSP and anti-framing; the public overlay remains
  embeddable for OBS.
- `--version` exits before creating a database.

## Current limitations

- No Twitch authorization, EventSub or chat.
- No OBS connection or live source/track resolution.
- No incident, approval or repair engine; P04 belongs to Codex.
- No final branded UI, rehearsal mode, package or license decision.
- Browser rendering has not been exercised in a real browser; HTTP and embedded
  asset tests pass, but visual review remains for Codex.

## Next packet

P02 adds real own-channel Twitch chat and OBS discovery/read evidence. It is not
part of P01 and is not released until P01 is accepted.

## License

No license has been selected. Dependency notices are recorded in
`docs/implementation/P01-environment.md`; final repository licensing is a
release decision.
