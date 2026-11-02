# CueMender — development and foundation testing

CueMender is a local Windows stream-diagnostics application. The repository
contains the accepted P01 foundation plus P02 Twitch/OBS connection setup:
embedded private-control and public-overlay pages, a one-time local session,
validated SQLite expectations, public-client Twitch authorization, EventSub chat
intake, and read-only local OBS discovery/observations.

Live Twitch and OBS behavior has **not** been validated in this workspace: no
Twitch Public client is registered and OBS's WebSocket server is disabled.
Fixture tests are not live evidence. Diagnostics, approvals and the supported
track correction remain later packets.

## Requirements

- Windows 11 x64 to run.
- Go 1.27.3 to build. End users do not need Go or Node.
- A modern browser for the private control page.

The original development workspace has a portable, checksum-verified Go toolchain under
`.tools/go1.27.3/`; it is not shipped in Git. Fresh clones should install Go 1.27.3 and follow the root README. `.tools/` and `dist/` are intentionally ignored by Git.

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
go build -trimpath -o .\dist\cuemender.exe .\cmd\app
Remove-Item Env:CGO_ENABLED
```

The production build is CGo-free. A race run needs a supported C toolchain and
is not required for the standard P01 build.

## Run

```powershell
.\dist\cuemender.exe --no-browser
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
| `--mode MODE` | Immutable runtime mode: `live` (default) or `rehearsal`. |
| `--data-dir PATH` | Isolate configuration and SQLite data. Supports spaces and Unicode. In rehearsal this is the base; effective data is `PATH\rehearsal`. |
| `--listen ADDRESS` | Override the loopback listen address, e.g. `127.0.0.1:8788`. |
| `--no-browser` | Do not open a browser; print the link for manual use. |
| `--session-only-secrets` | Explicit memory-only credential mode for this process. Rehearsal always uses memory-only secrets. |
| `--version` | Print version/build metadata and exit without touching data. |

Rehearsal is structurally isolated: it constructs no Twitch/OBS managers, sockets, senders or OS-keyring store. It uses an in-process simulated provider and a child data directory whose SQLite database is marked rehearsal. A live process pointed at that database, or a rehearsal process pointed at an existing unmarked/live database, fails with a mode conflict. Rehearsal UI and projections always carry the REHEARSAL label. Live Twitch and real OBS approval remain deferred; rehearsal is not live evidence.

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

## Private HTTP boundary

- `GET /healthz` — minimal process status.
- `GET /control/` — static private-control shell (no private data).
- `POST /api/v1/session/bootstrap` — one-time token exchange.
- `GET /api/v1/state` — authenticated private snapshot with CSRF token.
- `POST /api/v1/settings` — revision-checked expectations update.
- `GET /api/v1/events` — authenticated private SSE.
- `POST /api/v1/twitch/connect` — persist client ID and begin device flow.
- `POST /api/v1/twitch/disconnect` — cancel and clear Twitch credentials.
- `POST /api/v1/obs/connect` — connect read-only to local OBS WebSocket.
- `POST /api/v1/obs/disconnect` — cancel reads and invalidate observations.
- `GET /api/v1/obs/discovery` — authorized discovery snapshot.
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
- Credentials: P02 stores the Twitch token pair and endpoint-scoped OBS password
  in the OS keyring. With `--session-only-secrets`, credentials remain in memory
  for that process only. If the keyring is unavailable, the UI reports it and
  credential-dependent work is disabled. There is no plaintext fallback.
- The server is loopback-only. Loopback HTTP is not a defense against malware or
  another privileged process on the same computer.
- Private pages use a self-only CSP and anti-framing; the public overlay remains
  embeddable for OBS.
- `--version` exits before creating a database.

## Current limitations

- Live Twitch authorization/chat is deferred by the user due to 2FA/phone
  setup; live OBS reads are verified on OBS 32.1.1 / obs-websocket 5.7.3.
  See [P02 setup](implementation/P02-setup.md) for the remaining gate.
- P02 has no incident, approval or repair engine; P04 belongs to Codex.
- No final UI, rehearsal mode, release ZIP or license decision.
- The final closure build passed hidden-browser bootstrap, live discovery,
  save/reload/save and disconnect/reconnect. Full Windows tests, vet/build and
  Linux race checks passed; see [the closure record](implementation/P02-codex-closure.md).
- OBS reads are intentionally read-only in P02; no mutation route exists.

## Next packet

P03 adds evidence-based diagnostics and the persistent incident lifecycle. It is
not part of P02; P03 v1.2 is released under the user's D30 sequencing decision.
Live Twitch verification remains explicitly deferred in issue #6 until account/2FA
setup is resolved, and is required before final release/submission readiness.
See [execution status](execution/STATUS.md) for its current dispatch state.

## License

No license has been selected. Dependency notices are recorded in
[P02 compatibility](implementation/P02-compatibility.md) and
[P01 environment](implementation/P01-environment.md); final repository
licensing is a release decision.
