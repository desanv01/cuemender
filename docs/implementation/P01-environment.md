# P01 environment and compatibility findings

Packet: P01 v1.1. Recorded 2026-11-01 UTC by DeepSeek.

## Host and shell

- OS reported by the registry: `Windows 10 Home 26H2`, build `26300.9711`
  (Windows 11-class build; the registry caption is inconsistent with the
  build number). Architecture: x64.
- Shell: Windows PowerShell `5.1.26100.9711` (the harness invokes it as the
  command shell; commands used were PowerShell 5.1-compatible).
- Optional test tool: Node `v24.14.1` is installed and was used only to run
  the actual control `app.js` regression fixture. It is not a product runtime
  or build dependency.
- Working directory: `C:\Users\Dv\Desktop\Twitch Hackathon` (contains spaces).
- Git: `git version 2.53.0.windows.2`; **no repository exists**. No Git
  initialization, commit, branch or other Git mutation was performed.
- Go was not on `PATH` and was not found in common installation locations.

## Portable Go toolchain

| Item | Value |
|---|---|
| Version | `go1.27.3 windows/amd64` |
| Official URL | `https://go.dev/dl/go1.27.3.windows-amd64.zip` |
| SHA-256 | `f26081f438319757f8a1b46d7d0b2c5ec7583d4038e485e0aa725cfde7885fff` |
| Verification | Matched the published `go.dev/dl/?mode=json` checksum exactly before extraction |
| Extracted path | `.tools\go1.27.3\go\bin\go.exe` (ignored workspace-local directory) |
| Environment only | `GOROOT`, `GOPATH`, `GOMODCACHE`, `GOCACHE`, `GOTMPDIR` and process `PATH` were set per command; no machine PATH or system install was changed |

The ZIP size was 79,010,309 bytes and the extracted `go version` was
`go version go1.27.3 windows/amd64`.

## Module and dependency pins

`go.mod` is module `livecheck` with `go 1.27.3` and these direct requirements:

| Module | Version | Purpose | License |
|---|---|---|---|
| `modernc.org/sqlite` | `v1.61.1` | CGo-free SQLite driver | BSD-3-Clause |
| `github.com/zalando/go-keyring` | `v0.2.8` | OS credential boundary | MIT |

Notable transitive modules: `modernc.org/libc v1.79.0` (BSD-3-Clause),
`golang.org/x/sys v0.49.0` (BSD-3-Clause),
`github.com/danieljoos/wincred v1.2.3` (MIT),
`github.com/godbus/dbus/v5 v5.2.2` (see module license file),
`github.com/google/uuid v1.6.0` (BSD-3-Clause). The complete graph is in
`go.sum`; P06 should perform the final release license inventory.

Why these pins:

- `modernc.org/sqlite v1.61.1` was the current stable release at preparation
  time and its `go.mod` declares `go 1.26.0`; Go 1.27.3 satisfies it.
- `go-keyring v0.2.8` was current stable and its Windows backend uses
  `wincred`; the real keyring round trip below passed without CGo.
- No web framework, ORM, migration framework, assertion framework, UUID
  library, OBS or Twitch dependency was added. P02 candidates
  (`github.com/andreykaipov/goobs`, `github.com/coder/websocket`) are not
  installed or claimed as tested.

Verified with `go list -m all`; `go.sum` has 66 lines.

## Build compatibility

- Default `CGO_ENABLED` on this host is `0`.
- Release build used `CGO_ENABLED=0` and `-trimpath`:
  `go build -trimpath -o .\dist\livecheck.exe .\cmd\app`.
- Built artifact: `dist\livecheck.exe`, 17,365,504 bytes, version
  `livecheck 0.1.0-p01 (commit unknown, built unknown, go1.27.3)`.
- The production build does not require a C toolchain.
- A race run was attempted with `CGO_ENABLED=1 go test -race ./...`; it did not
  build because `gcc` is not installed (`cgo: C compiler "gcc" not found`).
  This is recorded as a skipped optional check, not a pass. The release build
  does not require race instrumentation.

## OBS and Twitch prerequisites

- OBS executable found at
  `C:\Program Files\obs-studio\bin\64bit\obs64.exe`, version `32.1.1`.
- The bundled obs-websocket plugin directory exists. At scan time
  `%APPDATA%\obs-studio\plugin_config\obs-websocket\config.json` had
  `server_enabled=false`, `auth_required=true`, `server_port=4455`.
  **OBS was not launched or modified.** P02 must enable the WebSocket server
  and record the actual protocol/version behavior; this is currently a live
  prerequisite, not a P01 failure.
- No Twitch public-client registration or client ID was found in the
  workspace. P01 does not need credentials. P02 must verify the registered
  public client, own-channel authorization, scopes and a separate viewer for
  chat tests.

## Keyring

- The real OS keyring round trip test
  (`TestOSKeyringRoundTrip` with `LIVECHECK_KEYRING_TEST=1`) passed using a
  randomly generated, application-owned disposable key. The key was deleted
  in cleanup; only pass/fail and the test name are recorded here.
- No real credentials exist or are stored by P01.

## Runtime data and cleanup

- Default data directory: `%AppData%\LiveCheck`; P01 did not create it during
  the final tests.
- Binary smoke tests used disposable directories under ignored
  `local-data\` with spaces and `ü` in the path. They were removed after the
  run. No database or runtime data is committed.
- A harness-launched child process could not create SQLite under
  `%TEMP%` (SQLite error 14), while the same store tests pass with
  `t.TempDir()` and the binary works from the workspace. This is treated as a
  harness/process-confinement limitation, not a product behavior claim; Codex
  should recheck a normal shell. The binary smoke used the workspace-local
  ignored directory instead.

## Limitations recorded for review

- Browser rendering was not exercised in a real browser. The actual
  `web/control/app.js` was exercised in a Node VM fixture with DOM/fetch
  stubs, including bootstrap, authenticated reload and save. This is browser
  logic evidence, not visual rendering evidence.
- A normal-console compiled-binary Ctrl+C check now passes: a helper created a
  new console, attached to it, sent `CTRL_C_EVENT`, and the final
  `dist/livecheck.exe` exited with code 0; the port was released and the
  database was present. The helper and its data were removed. The in-process
  `TestRunServesAndShutsDownWithActiveSSE` additionally covers an active SSE
  client and no longer logs a shutdown timeout.
- OBS and Twitch are unconnected; no live product behavior was claimed.
- No Git repository exists, so there is no commit checkpoint or diff.

## P01-R01 environment recheck

- No dependency or toolchain changes were required for P01-R01.
- `gcc` is still absent, so the optional race build remains skipped; the
  CGo-free production build is unaffected.
- The real-console Ctrl+C and Node actual-JS checks above are local harness
  evidence, not live Twitch/OBS or visual-browser evidence.
