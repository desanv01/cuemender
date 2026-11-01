# P01 — Runnable local foundation

Packet revision: 1.1
Dispatch status: RELEASED
Prerequisites: Read shared documents; no previous implementation packet.
Worker: one DeepSeek chat
Acceptance authority: Codex
Workflow: Codex also implements P04 and needed integration fixes after handoff; DeepSeek must not implement the reserved repair engine.
Expected outcome: a small executable can serve embedded local pages, protect private state, persist validated settings and shut down cleanly.
Not the outcome: a complete diagnostics product or generic application framework.

## 1. Read and claim

Read [AGENTS.md](../../../AGENTS.md), then [README](../README.md), [STATUS](../STATUS.md), [WORKFLOW](../WORKFLOW.md), [PROJECT_SPEC](../PROJECT_SPEC.md), [ARCHITECTURE](../ARCHITECTURE.md), [CONTRACTS](../CONTRACTS.md) and this packet. The repository root is C:\Users\Dv\Desktop\Twitch Hackathon.

Verify STATUS still says P01 released and no active code writer. Change only its operational ownership fields to DeepSeek:P01, in_progress, started_at UTC. Record the initial file inventory. Preserve the three original root planning documents and all authoritative specifications.

Do not initialize Git. No repository is present at preparation time. If a repository now exists, read status and preserve unrelated modifications.

## 2. Allowed file scope

Create only consumed foundation files, typically:
- go.mod, go.sum, .gitignore, README.md
- cmd/app/main.go
- internal/app lifecycle/config files and tests
- internal/domain minimal settings/projection/error types
- internal/store SQLite migration/settings implementation and tests
- internal/secrets OS-keyring/memory boundary and tests
- internal/httpui routes/middleware/SSE and tests
- web/embed.go, web/control static files, web/overlay static files
- testdata/config only if fixtures improve meaningful verification
- docs/implementation/P01-environment.md for factual environment/compatibility findings
- docs/execution/handoffs/P01-worker-report.md
- operational fields in docs/execution/STATUS.md

Build output goes to ignored dist; temporary databases/logs must be ignored or outside source. Optional portable Go is confined to ignored .tools. Do not modify specifications, earlier planning documents, or invent future implementations.

Use module path livecheck initially; Codex can set the final repository module path when the public name/repository is decided. No claim that a GitHub repository already exists.

## 3. Preflight and dependency selection

### P01.1 — Inspect actual prerequisites

1. Record OS version/architecture, shell, current working directory, go version if available, and Git availability. Do not inventory unrelated personal files.
2. Check Go on PATH and common Go installation locations. A full executable path is acceptable; don't alter the machine PATH.
3. If Go is missing, a workspace-local portable toolchain is permitted:
   - Fetch a stable supported Windows amd64 Go ZIP from go.dev/dl using the harness's permitted network tools.
   - Verify SHA-256 against the official published checksum before extraction/use.
   - Extract into a new versioned .tools directory; do not overwrite an existing installation.
   - Use its executable directly or prepend PATH only for the worker process.
   - Record version, official URL, checksum verification and exact local path.
   - Do not use an administrator installer, third-party download mirror or machine-level settings.
   - If network/permission/checksum verification prevents this, report the concrete blocker and stop dependent implementation; do not generate a large uncompiled scaffold.
4. Inspect OBS availability/version without launching a stream or changing scenes. Missing OBS does not block P01. Record whether a Twitch public-client registration/client ID is supplied; absence does not block P01.
5. Do not request real credentials. P01 checks require none.

### P01.2 — Pin only needed dependencies

1. Inspect modernc.org/sqlite and zalando/go-keyring current stable module metadata/requirements.
2. Choose explicit compatible stable versions and a supported Go version, recording the reasoning. Do not use pseudo-random latest tags or a floating latest in a reproducibility instruction.
3. Initialize go.mod, fetch these exact versions, retain go.sum, and run go mod tidy after imports exist.
4. Use Go standard library for other P01 needs. No web framework, ORM, migration framework, assertion framework or UUID library.
5. List the planned goobs/coder dependencies in the environment note as P02 candidates; do not claim they are tested or add unused imports just to retain modules.

Deliverable: environment note separating verified toolchain/SQLite/keyring build compatibility from unverified OBS/Twitch setup.

## 4. Implementation tasks

### P01.3 — Entry point and lifecycle

1. Parse only implemented flags from C02 using the standard library. --version exits without creating app data.
2. Resolve the explicit or default data directory; support spaces and Unicode paths.
3. Load/validate config before opening services. Bind the selected loopback listener early and fail clearly on conflict.
4. Initialize persistence, secrets mode and HTTP handler. No Twitch/OBS outbound connections in this packet.
5. Use context cancellation and signal handling. Stop SSE clients and HTTP work, close DB and exit cleanly on Ctrl+C. Startup failure cleans up what it opened.
6. Default browser launch may use the platform's normal open operation with an argument-safe API, no concatenated untrusted shell command. If browser launch fails, keep the service running and show the local launch URL to the interactive console. --no-browser makes console recovery deterministic.
7. Never emit the one-time capability into structured/file logs. Test output/handoff must redact it.
8. Print a truthful ready/listening message; it must not say Twitch connected or OBS healthy.

### P01.4 — Configuration and validated settings

1. Implement JSON startup config as C02. Missing config gets defaults; malformed config fails with a useful non-secret error and is not overwritten.
2. Reject non-loopback listeners/OBS endpoints, URL credentials, invalid schema versions, unknown fields and invalid port/track values.
3. Model initial expectations as unconfigured. Validation permits an empty first-run state and forbids a track of 0/7 or a half-specified enabled visual expectation.
4. Separate structural validity from live existence: P01 can accept syntactically valid names/IDs for tests but must display that live OBS resolution is not yet implemented.
5. Persistence of expectation changes uses expected_revision. A stale caller receives 409 and cannot overwrite newer data.
6. No tokens/passwords in config or settings structures. No environment-variable dumps in logs.

### P01.5 — SQLite foundation

1. Open modernc SQLite with deliberate busy timeout and foreign-key configuration.
2. Implement numbered transactional migration runner; create schema_migrations and app_settings only.
3. Running migrations twice is harmless; unsupported newer schema returns an explicit error rather than resetting the database.
4. Provide load/save settings methods with transactionally increasing revision. Return errors up to HTTP/app; never publish success after failed persistence.
5. Initial settings can be inserted lazily or during first migration, but first-run and subsequent reads must agree.
6. Test with real temporary SQLite files, including close/reopen. Use narrow repository tests, not an ORM abstraction layer.
7. No pretend incident/action tables or broad CRUD generator.

### P01.6 — Secret boundary

1. Implement a small interface for Get/Set/Delete and typed not_found/unavailable outcomes.
2. OS implementation uses zalando/go-keyring and an application-owned namespace. Values must not be part of generic formatted error messages.
3. Memory implementation is selected only by explicit --session-only-secrets. It disappears when the process ends.
4. When the OS backend is unavailable, the shell still runs and reports credential storage needs attention. Credential-dependent future operations remain disabled; no silent plaintext or in-memory fallback.
5. Tests inject a fake backend, exercising missing keys, backend failure, set/delete and nonpersistent memory mode. A real OS keyring round trip is optional with a generated disposable application key; delete it and record only pass/fail. Do not read unrelated credentials.
6. Tokens do not yet exist. Do not implement Twitch OAuth in this packet.

### P01.7 — Private HTTP boundary

Implement C03 routes assigned to P01 and nothing with external effects.

1. Generate the expiring one-time bootstrap token and issue the fragment-based entry URL.
2. Serve static /control/ shell, exchange a valid token via POST, set session cookie and return CSRF token.
3. In JS, remove fragment before further navigation; never persist credentials/capabilities in localStorage.
4. Reject bad/expired/replayed bootstrap token and foreign Origin/Host. Use constant-time comparison for fixed-length secret capabilities and a modest bounded attempt limiter.
5. Private state/SSE/settings require a valid session. Settings mutation additionally requires CSRF and same-origin JSON.
6. Reject mutation on GET and malformed/oversized/unknown-field JSON. Responses use the common envelope and safe error codes.
7. Apply appropriate cache/security headers and no permissive CORS. Don't attach a private anti-framing rule globally if it would prevent the public overlay's intended use.
8. Private state reports settings revision/readiness, secrets mode, disconnected integrations and no incidents. No future features are reported as ready.

### P01.8 — Embedded shells and public projection

1. Place embed.go in web so its directive can embed child assets.
2. Use plain external JS/CSS files. No frontend package manager/build step.
3. Control shell shows application mode/status, setup readiness and clear "Twitch/OBS connection comes next" state. A small readable foundation screen is enough.
4. A public overlay shell shows a quiet idle/disconnected state, not invented incident data.
5. Implement private/public projections as separate explicit types or serialization functions. Public must use C03 allowlist, never a private object with a few properties deleted.
6. All data renders as text. Do not use innerHTML with user/config/server strings.
7. Do not design the final branded UI, animations or complex widgets now.

### P01.9 — SSE and shutdown

1. Implement separate private/public state streams with the correct access boundary.
2. Send initial full snapshot and heartbeat comments.
3. A successful settings change advances state_revision and broadcasts private state plus a sanitized public snapshot. Public may reflect a new revision without revealing settings.
4. Use bounded latest-state delivery; slow clients cannot block writes or shutdown.
5. Unsubscribe on context cancellation; close streams on server shutdown.
6. Treat browser reconnection as a request for current state, not durable replay.

### P01.10 — Developer instructions and hygiene

1. README documents prerequisites, exact build/run commands, flags, data location, one-time link behavior and present limitations.
2. .gitignore covers .tools, dist, local DB/WAL/SHM, credential/config files used for manual testing, logs and local runtime data without ignoring committed fixtures.
3. Add no license pretending to be selected by the user. Record dependency licenses in the environment note; final repository licensing is a release decision.
4. Do not copy real authorization response JSON into fixtures.
5. Stop test/application processes you started before handing off. If a process must remain for an explicit review reason, provide PID and purpose and say it is still running.

## 5. Required verification

Use the selected Go executable; the commands below assume it is available as go in this process.

~~~powershell
go version
go mod tidy
gofmt -w ./cmd ./internal ./web
go test ./...
go vet ./...
go build -o .\dist\livecheck.exe .\cmd\app
~~~

Record exact equivalents if Go was invoked by full path. Check formatting produces no outstanding unformatted .go files. Verify module versions with go list -m all; don't paste verbose download logs as proof of behavior.

Also build with CGO_ENABLED=0 scoped to the worker shell and restored afterward. Standard release must compile without a C toolchain. A configured race run is useful if available; missing race prerequisites do not invalidate this foundation packet when all mandatory checks pass. Report it honestly.

### Required behavior tests

| Case | Assertion |
|---|---|
| Missing vs malformed config | Defaults work; malformed file preserved with clear error |
| Settings validation | Empty first-run accepted; invalid track/endpoint rejected |
| Revision conflict | Concurrent/stale save cannot overwrite accepted revision |
| SQLite reopen | Saved expectation/revision survives closing and reopening |
| Migration repeat/newer version | Repeat safe; unknown future schema refused |
| Bootstrap | Valid once; replay/expiry/invalid token rejected |
| Private requests | No session -> denied; correct session works |
| CSRF/Origin/Host | Missing/wrong values cannot mutate; foreign Host blocked |
| Public projection | Unique sentinel source names/token-like private values absent from public JSON/SSE |
| Request validation | Oversized/malformed/unknown-field body and GET mutation denied |
| Secrets | Unavailable is not "missing"; explicit memory mode never persists |
| SSE | Initial snapshot, update, cancellation and slow reader don't stall application |
| Shutdown | Active SSE does not prevent timely process termination |

Use actual HTTP handlers with httptest and real temp SQLite, not assertions about source text. Never merely mirror the implementation's constant in a test expected value.

### Binary smoke

Build and launch the binary from a separate temporary working directory with a disposable --data-dir, unused loopback port and --no-browser. Verify:
- /healthz and embedded assets load with no repository-relative files needed.
- Browser bootstrap/control/overlay flow works; record if browser access cannot be exercised.
- Private settings save, restart, and retrieval preserve revision; use isolated fixture names.
- Restart requires a new private login capability.
- Ctrl+C/shutdown releases the port and file handles.
- No real integration side effects or secret files were created.

HTTP integration tests can establish boundary behavior; they do not establish that a real browser rendered correctly. Keep those evidence classes separate. If no browser tool is available, provide precise manual steps and mark browser rendering unverified for Codex review.

## 6. Acceptance checklist

- P01-A: Supported pinned Go/SQLite/keyring combination builds with CGo disabled; README reproduces it.
- P01-B: Binary serves embedded shells from another working directory and exits cleanly.
- P01-C: Validated settings persist with migration/revision semantics; malformed config is preserved.
- P01-D: Private bootstrap/session/CSRF/Host/Origin rules prevent unauthorized mutation.
- P01-E: Public JSON/SSE contains only the allowlisted projection.
- P01-F: Secret backend failure is explicit; no silent plaintext fallback.
- P01-G: SSE update/cancellation/shutdown behavior has meaningful tests.
- P01-H: Required commands/tests run or exact blockers are stated; no external service falsely shown as live.
- P01-I: Original planning documents/specifications preserved; changed file list and environment note complete.

If mandatory criteria are unmet, report blocked or ready_for_review with explicit failures; never say all passed.

## 7. Handoff

Write ../handoffs/P01-worker-report.md using ../templates/WORKER_REPORT.md. Map P01-A through P01-I and V01–V05 to evidence. Include Go executable path/version, module pins, any .tools download/checksum, runtime data locations, exact build/test results and browser-vs-HTTP verification.

Record remaining P02 prerequisites: OBS version/WebSocket status, registered Twitch public client, ability to authorize own channel, and separate viewer for command tests.

Set STATUS to ready_for_review (or blocked), owner=Codex, code_writer_active=none. Stop editing. Return the P01 completion prompt from PROMPTS. Do not start P02 or make Git changes.
