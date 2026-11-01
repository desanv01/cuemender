# P01 worker report

## Identity and ownership

- Packet / revision: P01 — Runnable local foundation, v1.1
- Worker/harness: DeepSeek (`deepseek-flash` via DeepSeek Harness)
- Started / finished UTC: 2026-11-01T06:03Z / 2026-11-01T06:32Z
- Starting checkpoint or file baseline: 28 Markdown files; no `.git`, no application code. The three original root planning hashes matched `BASELINE.md` before and after.
- Result: **ready_for_review**
- Editing stopped: yes
- Processes stopped: yes; no LiveCheck process remained after smoke tests
- STATUS updated: yes — `in_progress` claimed, then handed to Codex with `code_writer_active=none`

## What changed

| Path | Added/modified/deleted | Purpose |
|---|---|---|
| `.gitignore` | added | Ignore `.tools`, `dist`, local DB/WAL/SHM, logs, local test data |
| `README.md` | added | Build/run/flags/data/security/limitations and P02 boundary |
| `go.mod`, `go.sum` | added | Module `livecheck`; pinned SQLite and keyring dependencies |
| `cmd/app/main.go` | added | Flags, version, signal context, exit codes |
| `internal/app/app.go` | added | Lifecycle, listener binding, SQLite wiring, secrets status, SSE publication, graceful shutdown |
| `internal/app/config.go` | added | JSON config defaults/validation and data-directory resolution |
| `internal/app/events.go` | added | Bounded latest-snapshot SSE hub |
| `internal/app/browser.go` | added | Argument-safe platform browser launch |
| `internal/app/version.go` | added | Version/build metadata |
| `internal/app/app_test.go` | added | Actual listener/SSE shutdown lifecycle test |
| `internal/app/config_test.go` | added | Config/default/malformed/validation/data-dir tests |
| `internal/domain/settings.go` | added | Structural expectations, settings and validation |
| `internal/domain/projections.go` | added | Separate private/public projection types |
| `internal/store/store.go` | added | Transactional migrations, SQLite settings repository, revision compare |
| `internal/store/store_test.go` | added | Reopen, conflict, migration repeat/newer, validation tests |
| `internal/secrets/secrets.go` | added | Typed credential boundary, OS keyring adapter, memory mode |
| `internal/secrets/secrets_test.go` | added | Fake backend failures, not-found, memory isolation, optional real keyring |
| `internal/httpui/server.go` | added | Routes, host/origin/CSRF/session middleware, SSE, shutdown close signal |
| `internal/httpui/session.go` | added | One-time bootstrap, sessions, rate limiter |
| `internal/httpui/respond.go` | added | Shared JSON envelopes, request IDs, bounded/unknown-field decoding |
| `internal/httpui/server_test.go` | added | Real temp SQLite + `httptest` boundary/SSE/projection tests |
| `web/embed.go` | added | Embedded control/overlay assets |
| `web/control/index.html`, `app.js`, `style.css` | added | Foundation private control shell and settings form |
| `web/overlay/index.html`, `app.js`, `style.css` | added | Quiet public overlay with public SSE only |
| `docs/implementation/P01-environment.md` | added | Toolchain, pins, licenses, compatibility and limitations |
| `docs/execution/handoffs/P01-worker-report.md` | added | This report |
| `docs/execution/STATUS.md` | modified | Operational claim and handoff fields only |

Ignored, not source changes: `.tools/` (portable Go and module cache) and `dist/livecheck.exe` (built artifact). No authoritative specification, planning document or `CONTRIBUTIONS.md` was modified. No Git operation occurred.

## Acceptance mapping

| Criterion | Evidence / reproduction | Result |
|---|---|---|
| P01-A — pinned Go/SQLite/keyring builds CGo-free; README reproduces it | `go1.27.3`; `modernc.org/sqlite v1.61.1`; `zalando/go-keyring v0.2.8`; `CGO_ENABLED=0 go build`; README build block; real keyring test passed | pass |
| P01-B — binary serves embedded shells from another working directory and exits cleanly | Final binary smoke from an ignored workspace-local working directory with a Unicode/space data path: `/control/`, `/control/app.js`, `/control/style.css`, `/overlay/`, `/overlay/app.js`, `/healthz` all 200; restart preserved settings. Clean shutdown is covered by the in-process active-SSE test, **not** by a successful binary Ctrl+C test (see limitations) | pass for embedded serving; binary signal check not fully verified |
| P01-C — validated settings persist with migration/revision semantics; malformed config preserved | Store reopen/conflict/migration tests; HTTP save/revision test; malformed-config test confirms byte-for-byte preservation; binary restart preserved revision 1 and settings | pass |
| P01-D — bootstrap/session/CSRF/Host/Origin rules prevent unauthorized mutation | `httptest` tests: no session 401, replay/expired/invalid 401, wrong Origin 403, foreign Host 403, missing/wrong CSRF 403, GET settings 405, invalid/oversized/unknown-field bodies 400 | pass |
| P01-E — public JSON/SSE contains only allowlisted projection | Sentinel source name and CSRF token absent from public JSON and public SSE; public state still carries mode/revision/status/empty incidents; private state carries the saved sentinel | pass |
| P01-F — secret backend failure explicit; no silent plaintext fallback | Fake OS backend tests distinguish not-found from unavailable; backend errors do not expose values; memory mode is isolated and explicit; real OS keyring round trip passed | pass |
| P01-G — SSE update/cancellation/shutdown behavior | Initial snapshot, settings update revision 2, slow non-reading subscriber, second subscriber, cancellation, and active-SSE shutdown test | pass |
| P01-H — required checks run or exact blockers; no external service falsely shown live | `go test`, `go vet`, `gofmt`, CGo-free build, binary smoke passed; race build skipped because `gcc` is missing; OBS/Twitch explicitly not connected | pass with recorded skips |
| P01-I — original documents preserved; file list and environment note complete | All three root SHA-256 hashes match `BASELINE.md`; every changed path is listed above; environment note present | pass |
| V01 — clean build, embedded pages from different working directory | Final binary smoke from a separate working directory; embedded assets 200 without repository-relative files | pass |
| V02 — defaults, invalid config, SQLite revision conflict and restart persistence | Config tests, store conflict/reopen tests, binary restart settings revision 1 | pass |
| V03 — private session/CSRF/Host/Origin rejection; public leak test | HTTP boundary and sentinel tests above; no CORS header | pass |
| V04 — keyring unavailable vs not-found; memory-only behavior | Secrets unit tests; real keyring round trip passed with disposable key | pass |
| V05 — SSE initial/update/cancel and shutdown without stuck process | `TestSSEInitialUpdateCancellationAndSlowReader`; `TestRunServesAndShutsDownWithActiveSSE` | pass in-process |

## Exact checks

Commands were run from the repository root. The portable Go environment was set per command as documented in the README; no machine PATH change.

| Exact command/action | Exit/result | Relevant evidence |
|---|---|---|
| `go version` | 0 | `go version go1.27.3 windows/amd64` |
| `gofmt -l ./cmd ./internal ./web` | 0, no output | No outstanding unformatted Go files |
| `go mod tidy` | 0 | `go.mod`/`go.sum` retained; direct requirements are SQLite and keyring |
| `go test ./... -count=1` | 0 | `cmd/app` no tests; `internal/app` ok 0.280s; `httpui` ok 0.907s; `secrets` ok 0.041s; `store` ok 0.184s; `web` no tests |
| `go vet ./...` | 0 | clean |
| `CGO_ENABLED=0 go build -trimpath -o .\dist\livecheck.exe .\cmd\app` | 0 | 17,365,504-byte artifact |
| `.\dist\livecheck.exe --version` | 0 | `livecheck 0.1.0-p01 (commit unknown, built unknown, go1.27.3)` |
| `LIVECHECK_KEYRING_TEST=1 go test ./internal/secrets -run TestOSKeyringRoundTrip -count=1 -v` | 0 | Real disposable OS keyring round trip passed; key deleted in cleanup |
| `CGO_ENABLED=1 go test -race ./...` | build failed | `gcc` not installed; race check skipped, not marked passed |
| Final binary smoke: start with `--no-browser --data-dir "<ignored>\\data ü" --listen 127.0.0.1:<port>` from a separate working directory | 0 | `/control/`, app.js, style.css, `/overlay/`, overlay app.js, `/healthz` all 200; bootstrap/session/settings revision 1; public SSE revision 2, `service_status=disconnected`, zero incidents; no public sentinel |
| Binary restart with the same data directory | 0 | Settings revision 1 and microphone sentinel persisted; new bootstrap capability was different; state revision reset to 1 |
| `livecheck.exe --mode rehearsal --no-browser --data-dir <path>` | 2 | Unimplemented mode rejected; no data directory created |
| `--version --data-dir <path>` | 0 | No data directory created |
| Root planning SHA-256 comparison | match | All three hashes exactly match `BASELINE.md` |
| Process cleanup | clean | No `livecheck` process remained; disposable smoke data removed |

## Environment and dependencies

Full details are in `docs/implementation/P01-environment.md`.

- Host: Windows 11-class build `26300.9711`, x64.
- Go: portable official ZIP `go1.27.3.windows-amd64.zip`, SHA-256
  `f26081f438319757f8a1b46d7d0b2c5ec7583d4038e485e0aa725cfde7885fff`,
  verified against `go.dev/dl` metadata, extracted to `.tools\go1.27.3\`.
- Modules: `modernc.org/sqlite v1.61.1` (BSD-3-Clause),
  `github.com/zalando/go-keyring v0.2.8` (MIT); full graph in `go.sum`.
- Build: CGo disabled; `-trimpath`; no Node or frontend build step.
- Data: no runtime data is committed. Default path is `%AppData%\LiveCheck`;
  smoke used ignored workspace-local directories and removed them.
- OS keyring: real round trip passed; no real credentials exist.
- OBS: 32.1.1 present; obs-websocket `server_enabled=false`,
  `auth_required=true`, port 4455. Not modified or connected.
- Twitch: no public client/client ID or live auth tested; P02 prerequisite.

## Behavior demonstrated

Central path: bootstrap a private session from a one-time fragment link; load
private state; save structurally valid microphone/track/visual expectations
with revision compare; observe state revision advance; verify public JSON and
public SSE do not contain the private source name or CSRF token; restart and
recover the saved settings.

Failure paths covered by tests: replay/expired/invalid bootstrap; rate limit;
missing session; wrong Origin/Host; missing/wrong CSRF; non-JSON, unknown-field,
oversized and malformed bodies; stale revision; invalid track; migration repeat
and newer-schema refusal; unavailable vs missing keyring secret; slow SSE
reader; active-SSE shutdown.

Live evidence: none. OBS/Twitch are intentionally unconnected in P01. Browser
rendering was not exercised; the HTTP and embedded-asset checks are separate
from visual browser verification.

## Deviations / unresolved issues

- **Browser rendering unverified.** No browser automation was available. The
  HTML/CSS/JS is embedded and served, and the HTTP flow was exercised with
  manual requests; visual rendering and keyboard use remain for Codex/manual
  review. The control page is intentionally a foundation form, not final UI.
- **Binary Ctrl+C not verified.** The in-process `Run` path exits cleanly with
  an active SSE stream (`TestRunServesAndShutsDownWithActiveSSE`). A console
  helper intended to send `CTRL_C_EVENT` did not make the compiled child exit;
  the probe was stopped and the binary smoke used `taskkill`/forced stop for
  restart. Codex should perform a manual Ctrl+C check on a normal console.
- **Race check skipped.** `CGO_ENABLED=1 go test -race ./...` fails to build
  because `gcc` is absent. This is not a product build failure; the release
  build is CGo-free.
- **Harness temp-path limitation.** A harness-spawned child could not create
  SQLite under `%TEMP%` (SQLite error 14); store tests using `t.TempDir()` pass,
  and the binary works with a workspace-local data directory. This is recorded
  as an environment/process-confinement limitation for Codex to recheck.
- **State revision semantics.** `state_revision` is in-memory, starts at 1, and
  resets on restart; the persisted `settings.revision` does not. The contracts
  require a monotonic `state_revision` for the live stream but do not state
  persistence across process restarts. If Codex expects restart persistence,
  this needs a contract decision.
- **P02 prerequisites:** enable OBS WebSocket, register a Twitch public client,
  authorize the broadcaster's own channel, and provide a separate viewer. OBS
  version and current WebSocket config are recorded in the environment note.
- **No Git baseline:** no repository exists, so there is no commit/diff; the
  file list above is the change record.
- No license selected; final naming/repository path and P04 remain outside this
  packet.

## Reviewer reproduction

1. From the repository root, set the portable Go environment exactly as shown in
   `README.md`, then run:
   `gofmt -l ./cmd ./internal ./web`, `go test ./...`, `go vet ./...`,
   `CGO_ENABLED=0 go build -trimpath -o .\dist\livecheck.exe .\cmd\app`.
2. Run the built binary with a disposable data directory and an unused loopback
   port. Use `--no-browser`; open the printed single-use control link promptly.
   Do not copy that link into the report. The default data directory is
   `%AppData%\LiveCheck`.
3. Exercise `/control/`, save a clearly fictional microphone/track, inspect
   `/public/v1/state` and the overlay, restart with the same `--data-dir`, and
   confirm settings revision persistence and a new bootstrap capability.
4. Stop the process. A normal console Ctrl+C check is specifically requested;
   the worker's automated signal probe was not successful.
5. To run the optional real keyring check, set
   `LIVECHECK_KEYRING_TEST=1` and run the secrets test; it creates and deletes a
   disposable application-owned key. Do not inspect unrelated credentials.
6. A race run requires a supported `gcc`/C toolchain; otherwise record it as
   skipped.

## Contribution and handoff

DeepSeek wrote the P01 implementation, tests and this report. Codex wrote the
specifications, contracts, packets and acceptance process; Codex has not
implemented or accepted P01 yet. P04's approval/repair engine was not
implemented, by instruction. No claim is made that Codex authored this P01 code
or that the packet is accepted.

Editing is stopped. Next owner: Codex.

P01 worker handoff is ready. Read docs/execution/handoffs/P01-worker-report.md and docs/implementation/P01-environment.md, inspect the actual files against P01 v1.1 and the shared contracts, and perform the focused acceptance review. DeepSeek has stopped editing. Record accepted, changes required, or blocked. If you directly fix integration issues, take sole edit ownership and record your changes/tests. Then provide the consolidated correction prompt or release the next packet.

## Correction history

None. This is the first P01 v1.1 handoff.

## P01-R01 — consolidated foundation corrections

- Correction: P01-R01 v1.0, parent P01 v1.1
- Worker/harness: DeepSeek (`deepseek-flash` via DeepSeek Harness)
- Claimed / finished UTC: 2026-11-01T06:45Z / 2026-11-01T06:56Z
- Starting state: P01 changes_required; no active writer; no Git repository
- Result: **ready_for_review**
- Editing stopped: yes
- Processes stopped: yes; no LiveCheck process remained

### R01 — Authenticated reload restores the CSRF token

Before: `web/control/app.js` loaded the authenticated private state after a
reload but never copied `privateState.csrf_token` into the in-memory save
state. A reload therefore sent an empty `X-CSRF-Token`; only the fragment
bootstrap path had set a token.

After: `render()` copies a non-empty `csrf_token` from the authenticated
private-state response into memory. The token remains in memory only; it is
not placed in public SSE, localStorage or logs. A 401 state response clears
the in-memory token. The invalid-session and expired-bootstrap paths are
unchanged.

Changed: `web/control/app.js`; added
`testdata/browser/control_csrf_reload.js` (narrow browser-logic fixture).

Evidence: Node `v24.14.1` executed the actual `web/control/app.js` with a DOM
and fetch fixture. It covered bootstrap -> save, an authenticated reload with
the cookie (no fragment) -> save, and a locked 401 reload. Result:

```text
bootstrapCSRF: true
reloadCSRF: true
reloadSaveStatus: 200
lockedReload: true
failures: []
```

This is actual browser-script evidence, not a rendered-browser or visual test.

### R02 — Settings commit, cache and publication ordering

Before: `SaveSettings` performed the SQL commit before acquiring `stateMu`,
updated the cache under that lock, and then released the lock before publishing.
A paused older writer could overwrite a newer cache, and an older snapshot
could publish after a newer one. Existing HTTP tests used a separate test
backend/hub, so they did not exercise this production ordering.

After: `App.saveMu` serializes each successful commit with installation of its
cached settings and ordered publication. The state/publication critical section
now holds `stateMu` across cache update, `state_revision` increment, both
projection marshals and both production hub publications. Hub sends are bounded
and non-blocking; no network operation is held under `stateMu`. Failed
persistence and revision conflicts return before any cache or publication
change. `SubscribePrivate`/`SubscribePublic` take the same `stateMu` lock, so an
initial snapshot cannot be followed by an older publication.

Changed: `internal/app/app.go`; added `internal/app/state_test.go`.

Evidence, using the real `App` and production hub:

- concurrent saves with expected revision 0: one success, one
  `domain.ErrRevisionConflict`; final settings revision 1, state revision 2;
- sequential saves with subscribers installed between them: initial snapshots
  at revisions 1 and 2, updates at revision 3, no stale update or cached
  settings regression;
- failed stale save produced no event on either private or public subscription
  and left cached state at settings revision 2/state revision 3;
- after closing the App, reopening the same SQLite database returned persisted
  settings revision 2 and the latest microphone.

### R03 — Shutdown investigation and active-client verification

Reproduction: the focused lifecycle test initially reproduced a **5.2-second**
shutdown with `http shutdown incomplete: context deadline exceeded`. The SSE
handler had exited (`active_streams` went from 1 to 0), but `http.Server.Shutdown`
waited for a newly accepted connection that had no request; connection-state
logs showed an extra `StateNew` socket. The old test also canceled the SSE
request before canceling the app, and its 5-second test budget equaled the
application's shutdown budget.

Fix and verification:

- Production shutdown now closes the listener, closes new/idle connections,
  disables keep-alives, and actively closes SSE handlers before bounded
  `Shutdown`. Active request handlers are not force-closed by the pre-sweep;
  the bounded fallback remains.
- The lifecycle test keeps the SSE request/body open and continuously reads the
  stream while canceling the app. It uses an 8-second test budget, but fails if
  the production shutdown warning occurs, so the larger budget does not conceal
  a stuck handler.
- It also verifies the port can be rebound and the database can be reopened and
  read after shutdown.
- 20 fresh-process repetitions after the fix completed without any slow run;
  the required five-repetition run passed (0.21s first, 0.02–0.03s subsequent).
- Real compiled-binary Ctrl+C: a helper created a new console for the final
  binary, attached to it and sent a real `CTRL_C_EVENT`. Result: helper exit 0,
  `PASS: child exited cleanly after a real console Ctrl+C`, port released, and
  `livecheck.db` present. The helper and its data were removed. This is separate
  from the Node fixture and from the earlier forced-termination smoke.

Changed: `internal/app/app.go`, `internal/app/app_test.go`; `internal/httpui`
gained an active-stream diagnostic counter used by shutdown logging.

### R04 — Credential error redaction

Before: `secrets.Error.Error()` appended the backend cause with `%v`, so a
backend error containing credential text could appear in formatted errors/logs.

After: `Error()` contains only `Op`, `Key` and `Kind` metadata. The backend cause
is retained in an unexported field for `Unwrap`/programmatic inspection but is
excluded from normal formatted output; a `Format` method keeps `%v`, `%+v`,
`%#v`, `%s` and `%q` on the safe text. Changed:
`internal/secrets/secrets.go`, `internal/secrets/secrets_test.go`.

Evidence: fake Get, Set and Delete backend failures each carried a unique
sentinel secret. Tests asserted typed `unavailable` behavior and checked
`Error()`, `fmt` variants, JSON and structured `slog` output; none contained the
sentinel. `errors.Unwrap` still exposed a cause for programmatic inspection.
Existing not-found and memory-mode tests continue to pass.

### Exact checks and results after P01-R01

| Command | Result |
|---|---|
| `gofmt -l ./cmd ./internal ./web` | clean |
| `go test ./... -count=1` | all packages passed |
| `go test ./internal/app -run TestRunServesAndShutsDownWithActiveSSE -count=5 -v` | 5/5 passed |
| 20 fresh-process lifecycle runs | no slow run; no shutdown-timeout warning |
| `node testdata/browser/control_csrf_reload.js` | exit 0; fixture JSON above |
| Real-console helper on final `dist/livecheck.exe` | exit 0; clean Ctrl+C, port released, database present |
| `go vet ./...` | exit 0 |
| `CGO_ENABLED=0 go build -trimpath -o .\dist\livecheck.exe .\cmd\app` | exit 0; 17,372,160 bytes |
| `go test ./...` with race | not rerun; `gcc` remains absent (optional limitation) |

No dependency, toolchain, module-path or branding changes were made. The
`livecheck` module and binary identifiers remain for P01-R01.

### Remaining limitations

- Visual browser rendering was not exercised. R01 is verified against the actual
  control script in a Node VM fixture, not a rendered browser.
- The binary Ctrl+C check used a purpose-built console helper; it is a real
  console-control event, not a forced termination. The in-process test remains
  the repeatable CI-style evidence.
- The optional race check remains skipped because `gcc` is absent.
- OBS/Twitch stay unconnected; P02 prerequisites remain unchanged.
- `state_revision` remains process-local and resets on restart; persisted
  `settings.revision` is independent, per the review clarification.
- The harness child temp-directory SQLite limitation remains; binary checks use
  an ignored workspace-local data directory.
- No Git repository exists; no Git/GitHub or branding operation was performed.
- Browser fixture uses Node as a test tool only; no Node runtime dependency was
  added to the product.

P01-R01 corrections are ready for review. Read the correction section of docs/execution/handoffs/P01-worker-report.md, inspect actual changes against docs/execution/corrections/P01-R01.md, and reproduce the focused regressions. DeepSeek has stopped editing. Record acceptance, remaining corrections or specific blockers. P02 remains held until accepted.
