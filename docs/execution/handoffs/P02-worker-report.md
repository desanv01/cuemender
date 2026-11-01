# P02 worker report

## Identity and ownership

- Packet / revision: P02 — Real Twitch and OBS connections, v1.1
- Worker/harness: DeepSeek (`deepseek-flash` via DeepSeek Harness)
- Started / finished UTC: 2026-11-01T09:05Z / 2026-11-01T09:30Z
- Starting checkpoint: `feature/p02-live-connections` at HEAD `cdeb19a`;
  accepted foundation `a07b3df3c33f0284c1da450a323fbba3e9ed1771`; worktree was
  clean before edits. Read-only Git inspection only; no mutation.
- Result: **ready_for_review with explicit unmet live gates** (P02-A/V06 and
  P02-B/V07)
- Editing stopped: yes
- Processes stopped: yes; no `livecheck`/`cuemender` process remained
- STATUS updated: yes — claimed P02, then handed back to Codex with
  `code_writer_active=none`

## What changed

| Path | Added/modified/deleted | Purpose |
|---|---|---|
| `go.mod`, `go.sum` | modified | Pin goobs, coder/websocket and goobs's gorilla dialer |
| `internal/app/app.go` | modified | Wire Twitch/OBS managers, private setup state, startup resume, shutdown |
| `internal/app/integrations.go` | added | Setup routes backend, atomic config updates, manager error mapping |
| `internal/app/config.go` | modified | Atomic config replacement and serialized writes |
| `internal/app/config_test.go` | modified | Atomic/concurrent/invalid config and CLI-listen isolation tests |
| `internal/app/version.go` | modified | Version 0.2.0-p02 |
| `internal/domain/integrations.go` | added | Private Twitch/OBS/device/receipt/discovery/observation types |
| `internal/domain/errors.go` | added | Safe integration error categories |
| `internal/domain/projections.go` | modified | Private setup/integration projection; public projection unchanged |
| `internal/httpui/server.go` | modified | New authenticated Twitch/OBS setup routes and status mapping |
| `internal/httpui/respond.go` | modified | Accepted-response envelope helper |
| `internal/httpui/server_test.go` | modified | Route boundary, busies, discovery and public-redaction tests |
| `internal/obs/obs.go` | added | Read-only goobs backend, endpoint validation, dialer timeout |
| `internal/obs/manager.go` | added | OBS lifecycle, discovery, observations, bounded retry, stale state |
| `internal/obs/manager_test.go` | added | Fake-backend discovery/observation/partial/retry/reconnect fixtures |
| `internal/obs/goobs_compat_test.go` | added | Compile-only actual goobs method/field mapping |
| `internal/twitch/oauth.go` | added | Device flow, validation/refresh, Helix subscribe/send |
| `internal/twitch/credentials.go` | added | Single-entry keyring credential persistence |
| `internal/twitch/commands.go` | added | Exact command parser, dedup, cooldown, bounded reply queue |
| `internal/twitch/eventsub.go` | added | coder/websocket socket and EventSub protocol parsing |
| `internal/twitch/manager.go` | added | Twitch lifecycle, EventSub reconnect/handover, receipts, replies |
| `internal/twitch/*_test.go` | added | OAuth/manager/parser/redaction/reconnect fixtures |
| `web/control/index.html`, `app.js`, `style.css` | modified | Minimal Twitch/OBS setup, device guidance, discovery selectors |
| `testdata/browser/p02_setup_render.js` | added | Actual control-script P02 private-state rendering fixture |
| `docs/implementation/P02-compatibility.md` | added | SDK pins, method/protocol mapping, live limits |
| `docs/implementation/P02-setup.md` | added | Exact operator setup and troubleshooting |
| `docs/DEVELOPMENT.md` | modified | P02 routes, state and limitations |
| `docs/execution/handoffs/P02-worker-report.md` | added | This report |
| `docs/execution/STATUS.md` | modified | Operational ownership/handoff fields only |

Preserved: `README.md`, authoritative specifications, P01 evidence, legacy
storage/keyring/cookie identifiers, and all unrelated source. No branding sweep.
No Git/GitHub operation was performed.

## Acceptance mapping

| Criterion | Evidence / reproduction | Result |
|---|---|---|
| P02-A / V06 — actual own-channel authorization, separate-viewer command receipt/reply | No Twitch Public client is registered; no client ID or own-channel authorization exists. OAuth/EventSub/receipt/reply behavior is covered only by local HTTP and fake-socket fixtures. | **not run / unmet live gate** |
| P02-B / V07 — actual OBS reads match dedicated setup including all six tracks | OBS 32.1.1 is installed but obs-websocket `server_enabled=false`; no listener on 4455. Discovery/observation behavior is covered only by fake backends. | **not run / unmet live gate** |
| P02-C — device pending/slowdown/expiry/cancel, token binding/scopes/revocation, serialized refresh/rotation/save failure, late work after disconnect | Fixture tests: `TestDeviceFlowPendingSlowDownSuccess`, `DeniedAndExpired`, `TokenBindingMismatch`, `StartSavedRevocationNeedsAttention`, `RefreshRotationAndDisconnectRace`, `CredentialSaveFailureIsExplicit`, `DisconnectCancelsPendingDeviceFlow`, `DisconnectDeleteFailureIsExplicit`. | pass on fixtures; not live |
| P02-D / V08 — welcome/subscribe, keepalive deadline, requested handover vs ordinary resubscribe, duplicate IDs, old generation | `TestParseEventSubMessages`, `ReadEnvelopeHonorsDeadline`, `ManagerRequestedReconnectHandoverAndDuplicateDedup`, `EventSubOrdinaryReconnectResubscribes`, `ManagerOldGenerationStateIgnored`; local OAuth subscription fixture. | pass on fixtures; not live EventSub |
| P02-E — OBS auth failure, partial reads, reconnect/context invalidation, name/ID ambiguity, stale cache, no wrong-source readiness | `OBSConnectionFailureIsSafeAndNotReady`, `OBSPartialReadsRemainUnknown`, `OBSInitialDialRetries`, `OBSDisconnectInvalidatesAndCloses`, `OBSPasswordScopedToEndpoint`, `OBSResolutionRejectsRecreatedIdentity`, `OBSStaleSnapshot`. Continuous automatic reconnect after an *established* OBS socket loss is not implemented; manual reconnect is required. | pass on fixtures with noted reconnect limitation |
| P02-F — new routes enforce private boundary; public HTTP/SSE redaction; OBS request allowlist | `TestP02PrivateRoutesBoundaryAndRedaction` covers 401/403/400/405/409/202/200/503; public JSON and public SSE exclude an OBS source sentinel; `TestOBSBackendSurfaceIsReadOnly` and compile-only SDK surface test. No live outbound capture. | pass on fixtures/boundary inspection |
| P02-G — atomic config persistence/failure/concurrency; startup persistence and memory-only; settings ordering | `TestSaveConfigAtomicRoundTrip`, `SaveConfigInvalidPreservesPrior`, `SaveConfigConcurrentUpdatesStayValid`, `AppPersistConfigKeepsCLIListenOutOfFile`, `AppPersistConfigConcurrent`; P01 state-ordering tests remain passing. Memory-only status covered by P01 App test. A real disk write/rename failure was not injected; validation failure and concurrent replacement are covered. | pass on fixtures; disk-failure path not separately injected |
| P02-H — actual control JS regression and minimal setup checks; Node VM vs rendered browser | Node executes actual `web/control/app.js`: `control_csrf_reload.js` (bootstrap/reload/save) and `p02_setup_render.js` (device code, OBS endpoint, broadcasting, discovery selects). No rendered browser or visual review was performed. | pass as Node actual-script evidence; rendered browser not run |
| P02-I — full suite/vet/gofmt/CGo-free build; P01 active-SSE shutdown 5× with new workers; no lingering processes | Final commands below; focused shutdown 5/5 passed; no process remained. Race instrumentation unavailable because gcc is absent. | pass; race skipped |

## Exact checks

Working directory: repository root. Portable Go environment set per command;
`CGO_ENABLED=0` for the build.

| Command/action | Result |
|---|---|
| `gofmt -l ./cmd ./internal ./web` | clean |
| `go test ./... -count=1` | all packages passed: app, httpui, obs, secrets, store, twitch; `cmd/app` and `web` no tests |
| `go vet ./...` | exit 0 |
| `CGO_ENABLED=0 go build -trimpath -o .\dist\cuemender.exe .\cmd\app` | exit 0; 19,641,344-byte artifact |
| `.\dist\cuemender.exe --version` | `cuemender 0.2.0-p02 (commit unknown, built unknown, go1.27.3)` |
| `go test ./internal/app -run TestRunServesAndShutsDownWithActiveSSE -count=5 -v` | 5/5 passed (0.28s first, 0.02–0.04s subsequent) |
| `node testdata/browser/control_csrf_reload.js` | `bootstrapCSRF=true`, `reloadCSRF=true`, `reloadSaveStatus=200`, `lockedReload=true`, `failures=[]` |
| `node testdata/browser/p02_setup_render.js` | device code rendered, OBS endpoint/broadcasting rendered, discovery selects populated, `failure=null` |
| Compiled-binary smoke with isolated data directory | control panels present; initial Twitch/OBS `not_connected`; `POST /api/v1/obs/connect` 202; disabled local OBS reached `needs_attention` after bounded retries; disconnect 200; public state clean; process stopped |
| Twitch/OBS prerequisites inspection | OBS WebSocket `server_enabled=false`, no listener on 4455; no configured Twitch client ID; no live credentials |
| `go test -race` | not run; gcc absent. Optional race limitation retained. |

## Environment and dependencies

- Go `1.27.3 windows/amd64`; portable `.tools/go1.27.3`; no system install.
- OBS SDK `github.com/andreykaipov/goobs v1.11.1` (Apache-2.0),
  `goobs.ProtocolVersion = "5.7.6"`. Actual method/field mapping and compile
  test are in `internal/obs/goobs_compat_test.go` and P02-compatibility.
- WebSocket transport `github.com/coder/websocket v1.8.15` (ISC).
- OBS dialer uses `github.com/gorilla/websocket v1.5.3` (BSD-3-Clause) with a
  2-second handshake timeout through goobs `WithDialer`.
- Existing keyring `go-keyring v0.2.8` and SQLite `modernc.org/sqlite v1.61.1`
  remain.
- OBS executable 32.1.1; WebSocket server disabled; no live handshake.
- Twitch Public client is not registered; no client ID or tokens are present in
  source, config, SQLite, logs or this report.
- No new service, framework, queue or cloud dependency was added.

## Behavior demonstrated

Fixture paths cover device authorization pending/slowdown/denial/expiry and
cancellation; token validation, binding/scopes, revocation, refresh rotation and
credential save/delete failures; EventSub welcome/subscription, keepalive read
deadline, requested handover without resubscription, ordinary reconnect with
fresh subscription, duplicate delivery suppression and old-generation state
rejection; exact command parsing, dedup/cooldown, bounded replies and receipt;
OBS read-only discovery/observation, partial unknown fields, six-track
normalization, auth failure, initial retry, manual disconnect/reconnect,
endpoint-scoped passwords, identity ambiguity and stale data; atomic config
updates; private HTTP boundaries and public redaction.

The compiled binary smoke exercised embedded setup controls, authenticated
routes and a disabled local OBS failure without a live OBS/Twitch dependency.

## Deviations / unresolved issues

- **Live P02-A/V06 and P02-B/V07 are unmet.** No Twitch Public client/client ID,
  no own-channel authorization, no separate viewer, no live EventSub, and OBS
  WebSocket is disabled. Fixtures are not live evidence.
- **Rendered browser not exercised.** Actual control JS was executed in Node VM
  fixtures; visual rendering, focus behavior and real browser EventSource/OAuth
  interaction remain for manual/Codex review.
- **Established OBS socket loss:** initial dialing retries twice and manual
  disconnect/reconnect invalidates state; continuous automatic reconnect after
  an established socket failure is not implemented. Reads expose partial fields
  instead of inventing values. Codex should decide whether this needs a narrow
  correction.
- **Real OBS protocol capture:** no live request/response capture or all-six-track
  comparison was possible. Method mapping is compile-verified only.
- **Disk-level config failure** was not injected; validation failure and
  concurrent replacement are tested. The write path is fsync + same-directory
  rename.
- **Race instrumentation** remains skipped because gcc is absent.
- No incident engine, monitoring, diagnostics, proposals, repair, OBS mutation,
  branding sweep, P03/P04 implementation or Git/GitHub mutation was performed.
- `state_revision` remains process-local, as clarified in P01-R01 review.

## Reviewer reproduction

1. Build with the portable Go environment in `docs/DEVELOPMENT.md`; run
   `gofmt`, `go test ./... -count=1`, `go vet ./...`, and the CGo-free build.
2. Run `dist/cuemender.exe --no-browser --data-dir <isolated>`; open the printed
   link and verify zero incidents/monitoring and the Twitch/OBS setup panels.
3. Without live prerequisites, exercise the fixture tests above and the two Node
   fixtures. Confirm the private route boundary with no cookie/CSRF/foreign
   Origin; confirm public state contains no device code, tokens, source names or
   passwords.
4. When the live gates become available: register the Public client, authorize
   the own channel, use a separate viewer, enable OBS WebSocket, compare all six
   track flags and observation fields with OBS, then record the results and
   disconnect/reconnect behavior without interrupting an active broadcast.
5. Stop the process and remove isolated data. Do not paste tokens, device codes,
   passwords or raw chat into a report.

## Contribution and handoff

DeepSeek implemented the P02 adapters, lifecycle, HTTP/UI setup, tests and
documentation. Codex authored the specifications, contracts, C08 and review
process; Codex did not author this P02 code. P02 is not accepted, and P03 is
held. Live Twitch/OBS evidence remains for Codex/user execution.

P02 worker handoff is ready. Read docs/execution/handoffs/P02-worker-report.md, docs/implementation/P02-compatibility.md and docs/implementation/P02-setup.md; inspect actual changes against P02 v1.1 and CONTRACTS v1.2; reproduce the focused regressions and check live Twitch/OBS evidence separately from fixtures. DeepSeek has stopped editing. Record acceptance, consolidated corrections or specific unmet gates. P03 remains held until accepted.

## P02-R01 — correction evidence (appended)

Correction started/finished UTC: 2026-11-01T09:43Z / 2026-11-01T09:56Z. Parent P02 v1.1, CONTRACTS v1.2, correction packet P02-R01 v1.0. Same branch/HEAD; existing implementation and evidence preserved. The seven Codex review regressions were reproduced failing first, then passed. Their files were promoted into the ordinary suite by removing the opt-in tag; their behavioral assertions were not weakened.

### R01 — actual OBS wire tracks and protocol boundary

- `internal/obs/obs.go` converts actual wire keys `"1"`–`"6"` to domain `track1`–`track6`; malformed/missing entries remain unknown and are never filled with false. `WireTrackKey` documents the inverse for P04.
- `TestOBSProtocolMissingTracksStayUnknown`, `TestOBSProtocolErrorIsReturned`, `TestOBSReadOnlyOutboundAllowlist`, and promoted `TestReviewOBSRealWireTrackMap` use the pinned goobs client against local WebSocket servers. The allowlist test observed only GetVersion/GetInputList/GetInputMute/GetInputAudioTracks/GetCurrentProgramScene/GetSceneList/GetSceneItemList/GetSceneItemEnabled/GetStreamStatus/GetSceneCollectionList.
- Result: **pass** on local protocol fixtures; no live OBS negotiation claimed.

### R02 — OBS loss, cancellation, saved password, SDK logging

- Established transport loss invalidates discovery/observation/readiness, advances generation, closes the transport and redials with bounded backoff. Manual disconnect stops retry. Discovery now refreshes each poll; scene-item iteration is cancellable.
- `Disconnect`/`Close` close the active transport before waiting for workers. A fresh manager loads a saved password for the exact endpoint; endpoint change cannot inherit another password; explicit empty still deletes it.
- goobs now receives a silent logger so provider payloads cannot bypass redaction through stderr. Startup OBS auto-dial was deliberately not added: `config.json` always has a default endpoint and cannot prove intentional setup; manual Connect is required and omission of password loads the saved exact-endpoint credential.
- Tests: promoted `TestReviewOBSReuseSavedPasswordAfterRestart`, `TestReviewOBSClosedSocketNotFreshConnected`, `TestOBSLossReconnectsWithNewGeneration`, `TestOBSManualDisconnectStopsReconnect`, plus existing partial/disconnect/password tests. Result: **pass** on fixtures; actual SDK stalled-response and live transport checks remain Codex/live work.

### R03 — identity, configuration provenance, coherent snapshots

- Discovery provides generation, settings revision, scene-collection context, input UUIDs, scene UUIDs and direct item/source UUIDs. Readiness requires matching context, unique name when no UUID is supplied, direct identity, current settings revision, current generation and an observation not older than discovery.
- `SetSettings(revision, expectations)` invalidates the cached observation. Poll results with an older revision are discarded. `SnapshotBundle` is the single manager snapshot boundary used by the app for connection/discovery/observation/resolution.
- Polling and explicit `GET /api/v1/obs/discovery` both perform a fresh read through `RefreshDiscovery`; GET no longer returns a forever-cached snapshot as refreshed.
- Tests: promoted `TestReviewOBSRejectsRecreatedInputDuringConnection`, `TestOBSSnapshotBundleIsRevisionCoherent`, `TestOBSCollectionChangeRequiresReselection`; result **pass** on fake managers. No live collection/source check.

### R04 — Twitch validation, revocation, storage policy

- StartSaved and the maintenance loop call `/validate` and check client, subject, required scopes and positive expiry. Refresh is scheduled only near expiry; rotated access tokens are validated before use and missing scopes are not copied from old credentials.
- Revocation, invalid binding/scopes and 401/403 EventSub/Helix authorization paths enter `needs_attention` and stop intake/replies/reconnect instead of looping. A different subject is rejected.
- New credential-dependent flow is refused when OS storage is unavailable; `--session-only-secrets` is the explicit memory-only path. App maps the refusal to safe credential-storage error.
- Tests: promoted `TestReviewHourlyLoopActuallyValidates`; `TestManagerStartSavedRevocationNeedsAttention`, `TestManagerEventSubRevocationIsTerminal`, `TestManagerRefusesUnavailableCredentialStorage`, token-binding/revocation tests. Result: **pass** on HTTP/fake fixtures; no live Twitch.

### R05 — lifecycle, credential deletion, config persistence

- Connect/Disconnect/StartSaved serialize on a lifecycle lock; app connect/disconnect calls serialize on `integrationMu` so a losing config cannot be persisted before a busy result.
- Credential save/delete serialize on a credential mutex. Save rechecks generation after the write and deletes the late write; Disconnect waits for in-flight persistence and deletes through the same mutex. The blocking-store regression passes.
- Config replacement retains fsync + same-directory rename and retries transient Windows replacement denials. Validation failures preserve the prior file. Refresh loop is joined on Close.
- Tests: promoted `TestReviewRefreshCannotRestoreDeletedCredential`, existing concurrent/invalid config tests, and connection lifecycle tests. Result: **pass**; full disk-failure injection was not performed.

### R06 — EventSub handover ownership and generations

- `runSocket` owns and closes the final socket on every return; requested handover closes the old socket and installs the replacement under the same lifecycle generation. Connection generation advances on initial welcome and handover and is included in the typed command event/private connection.
- Promoted `TestReviewHandoverClosesReplacementSocket`; existing requested-handover/ordinary-resubscribe/duplicate tests still pass. Result: **pass** on fake sockets; no live EventSub.

### R07 — provider limits and normalized command seam

- `ProviderError` exposes Retry-After; Helix send captures it; the bounded reply queue waits at least that period before the next aggregate item. Cooldown entries expire and are capped at 10,000.
- Added typed `CommandEvent` with delivery/message IDs, broadcaster/actor IDs, received time, command/category/code and connection generation; no raw chat text. It is an optional manager callback for P03.
- Tests: `TestReplyQueueHonorsProviderRetryAfter`, `TestCooldownExpiresAndIsBounded`, promoted command/receipt tests. Result: **pass** on fixtures.

### R08 — setup controls and observations

- Discovery selections now retain microphone UUID/context, visual source UUID/context and scene collection context through fill/save. Null discovery clears old selectors instead of leaving stale options.
- The private page renders current mute, all six track flags, program scene, visual item and streaming state, with stale/partial copy.
- Both Node actual-script fixtures pass, including identity-preserving save (`savedIdentity=true`). No rendered browser was available; visual/browser review remains for Codex.

### Final checks

| Command/action | Result |
|---|---|
| `gofmt -l ./cmd ./internal ./web` | clean |
| `go test ./... -count=1` | all packages passed |
| `go test -tags reviewrepro ./internal/obs ./internal/twitch -count=1 -timeout=60s` | all promoted review regressions passed |
| `go vet ./...` | exit 0 |
| `CGO_ENABLED=0 go build -trimpath -o .\dist\cuemender.exe .\cmd\app` | exit 0; 19,689,984-byte artifact |
| `.\dist\cuemender.exe --version` | `cuemender 0.2.0-p02 (commit unknown, built unknown, go1.27.3)` |
| Active-SSE shutdown repeated 5× | 5/5 passed |
| `node testdata/browser/control_csrf_reload.js` | bootstrap/reload/CSRF/save passed, `failures=[]` |
| `node testdata/browser/p02_setup_render.js` | device/OBS/discovery/observation/identity-save passed, `failure=null` |
| gcc / race | gcc absent; race not run |

### Changed paths in this correction

`internal/obs/{obs,manager}.go`, `internal/obs/{manager_test,goobs_compat_test,p02_review_regression_test,p02_correction_test,p02_protocol_test}.go`, `internal/twitch/{manager,oauth,commands}.go`, `internal/twitch/{manager_test,p02_review_regression_test,p02_correction_test,p02_limits_test}.go`, `internal/domain/integrations.go`, `internal/app/{app,integrations,config}.go`, `internal/app/config_test.go`, `web/control/{index.html,app.js}`, `testdata/browser/{control_csrf_reload,p02_setup_render}.js`, compatibility/setup/development docs and STATUS. No specification edits, OBS mutations, Git operations or P03/P04 work.

### Remaining gates / limitations

P02-A/V06 and P02-B/V07 remain **unmet live gates**: no Twitch Public client/own authorization/separate viewer and OBS WebSocket is disabled; no live EventSub/OBS/Twitch evidence. Local protocol fixtures are not live. No rendered browser; Node actual-script only. Automatic OBS startup was intentionally omitted (documented design choice). A fully blocked provider shutdown, blocked refresh-loop shutdown, and simulated old slow read were not independently recreated in this correction cycle; code checks and the promoted blocking-store/regression paths pass. Race remains unavailable because gcc is absent.

P02-R01 corrections are ready for review. Read the appended correction evidence in docs/execution/handoffs/P02-worker-report.md, inspect actual changes against docs/execution/corrections/P02-R01.md, and rerun the review regressions plus focused lifecycle/protocol/UI checks. DeepSeek has stopped editing. Record remaining corrections or acceptance, keeping live Twitch/OBS gates explicit. P03 remains held until accepted.

## P02-R02 — completion evidence (appended)

Correction started/finished UTC: 2026-11-01T10:03Z / 2026-11-01T10:11Z. Parent P02 v1.1 / CONTRACTS v1.2 / P02-R01. Existing R01 fixes preserved. All seven new review regressions were reproduced failing first, then passed; their assertions were preserved and the files promoted into ordinary coverage. All seven original promoted regressions also still pass.

### A–E completion matrix

| Group | Implemented evidence | Result / remaining local gap |
|---|---|---|
| A — token duration/identity/rotation | `maintenance` adds `TokenInfo.ExpiresIn` once using injected clock; zero/negative expiry requires auth; 30-second max maintenance tick catches short tokens; rotated token validated for client/subject/scopes/expiry; persistence failure retains the validated rotated pair with degraded state; terminal vs transient provider errors separated. `TestReviewR02MaintenanceExpiryUnits`, `TestReviewR02RotationSaveFailureUsesRotatedPair`, `TestMaintenanceDurationAndShortTokenRefresh`. | Supplied regressions pass. Startup changed-subject and expired-rotated-token paths are implemented but not separately fixture-tested this cycle. |
| B — OBS cancellation/transport ownership | Candidate transport published before Version/discovery/initial observation; Disconnect/Close close it mid-setup; reconnect also publishes candidate first. Adapter bounds SDK reads with caller context and closes the transport on cancellation; silent goobs logger. `TestReviewR02ActualSDKReadCancellation`, `TestReviewR02DisconnectClosesInitialReadTransport`, loss/reconnect/manual-disconnect tests. | Supplied regressions pass. A production-app blocked-provider shutdown test and every local failed-auth/reconnect socket case were not independently recreated. SDK methods do not accept context; cancellation closes transport and waits for the SDK's bounded request timer. |
| C — revision/context-safe observations | Initial and reconnect installs require captured settings revision/generation; old data is omitted rather than relabeled. Collection sampled before and after identity reads; failure/switch marks context unknown and blocks readiness. Visual scene UUID added alongside scene/item/source UUID. Explicit discovery refresh and poll follow revision checks. `TestReviewR02InitialReadRejectsOldSettings`, `TestReviewR02UnknownCollectionCannotResolve`, collection/scene/snapshot tests. | Supplied regressions pass. Slow old-read barrier and same-name scene recreation fixture were not separately recreated; code path is revision/context checked. |
| D — Twitch lifecycle/handover/reply ownership | Lifecycle lock on Connect/StartSaved/Disconnect/Close; immutable credential snapshots; terminal auth stops workers; connected state waits for EventSub welcome/subscription; replacement/final sockets closed; connection generation on handover; reconnect URL rejects userinfo/port; ReplyQueue rejects after close, joins sender, drains expired heads, honors Retry-After, and routes Helix 401/403 to terminal handling. `TestReviewR02ReplyQueueRejectsAfterClose` plus existing lifecycle/handover tests. | Supplied regression passes. Delayed handover while continuing old-socket notifications, and stale queued-reply generation barriers, were not fully rebuilt; they remain an explicit local gap. |
| E — stable setup controls/failure evidence | Discovery selector signature excludes captured_at, restores selections and only invalidates on identity/context change; manual name edits clear hidden UUID/context; scene UUID preserved; readiness/observation time/partial+stale rendered privately; config replacement has deterministic failure injection preserving prior bytes. Actual-JS fixture now executes select → timestamp-only update → save → manual rename → disconnect. `TestSaveConfigReplacementFailurePreservesBytes`. | UI fixture and config failure pass. A production-app startup/config/credential blocked-provider seam test was not added. |

### Exact final checks

| Command/action | Result |
|---|---|
| `gofmt -l ./cmd ./internal ./web` | clean |
| `go test ./... -count=1 -timeout=90s` | all packages passed |
| `go test -tags reviewrepro ./internal/obs ./internal/twitch -run TestReview -count=1 -timeout=60s -v` | 14/14 review tests passed (7 original + 7 R02) |
| `go vet ./...` | exit 0 |
| `CGO_ENABLED=0 go build -trimpath -o .\dist\cuemender.exe .\cmd\app` | exit 0; 19,709,440-byte artifact |
| `.\dist\cuemender.exe --version` | `cuemender 0.2.0-p02 (commit unknown, built unknown, go1.27.3)` |
| Active-SSE shutdown ×5 | 5/5 passed |
| `node testdata/browser/control_csrf_reload.js` | bootstrap/reload/CSRF/save passed, `failures=[]` |
| `node testdata/browser/p02_setup_render.js` | select/timestamp-update/save/manual-rename/disconnect plus device/OBS/discovery/observation passed, `failure=null` |
| gcc / race | gcc absent; race not run |

### Remaining gates and limits

P02-A/V06 and P02-B/V07 remain unmet live gates: no Twitch Public client/own authorization/separate viewer, OBS WebSocket disabled and no live OBS/Twitch/EventSub evidence. Local protocol and fake-provider fixtures are not live. No rendered browser; actual-script Node VM only. Race unavailable because gcc is absent. D25 is recorded: explicit OBS Connect after restart; no auto-start preference. The local gaps called out in the matrix (blocked production-app shutdown, delayed handover with old notifications, slow-old-read barrier, full startup/credential seam) are not claimed as completed.

P02-R02 corrections are ready for review. Read the appended P02-R02 evidence and inspect actual changes against docs/execution/corrections/P02-R02.md. Rerun the complete review regression set and the required lifecycle/protocol/UI failure paths. DeepSeek has stopped editing. Record remaining corrections or acceptance with live gates explicit. P03 remains held.

## P02-R03 — completion evidence (appended)

Correction started/finished UTC: 2026-11-01T13:33Z / 2026-11-01T13:39Z. Parent P02 v1.1 / CONTRACTS v1.2 / R02. Existing fourteen regressions and setup/config behavior preserved. The six new review failures were reproduced first, fixed, and promoted into ordinary coverage (their assertions unchanged).

### Four-group matrix

| Group | Implemented evidence | Result / remaining gap |
|---|---|---|
| 1 — OBS transport ownership | Adapter-owned raw network connection via gorilla dialer; cancellation during withheld Hello closes the raw socket and returns promptly. Candidate published before first Version/discovery; initial and reconnect candidates close on Disconnect/Close. Lifecycle serialization added. `TestReviewR03OBSHelloCancellation` plus prior initial-read and actual-read cancellation tests. | Supplied Hello regression passes. The underlying `goobs.New` authentication goroutine is bounded by the SDK timer rather than joined; full callback cleanup is bounded, not synchronous. Production-app blocked-provider shutdown with active HTTP/SSE remains **not implemented/tested**. |
| 2 — OBS revision/context/freshness | Revision captured before reads; collection sampled before Inputs/Scenes and after identity sequence; first-read failure cannot be upgraded; refresh refuses mismatched revision without relabeling; readiness requires connected+fresh+revision+generation+context+resolved identities; `reflect.DeepEqual` equality. `TestReviewR03OBSStaleCannotBeReady`, `UnknownStartContextStaysUnknown`, `RefreshCannotRelabelRevision`. | Supplied regressions pass. Slow initial/reconnect/poll barrier variants and same-name scene replacement were covered by existing/collection tests; no additional dedicated fixture this cycle. |
| 3 — Twitch lifecycle/handover/credentials | Lifecycle lock on Connect/StartSaved/Disconnect/Close; maintenance uses authorization-lifetime context and is canceled/joined by Disconnect and Close; requested handover starts replacement concurrently, keeps one old-socket reader processing notifications until replacement welcome, then retires old socket; replacement errors close all sockets; normalized command/dedup path retained. `TestReviewR03HandoverProcessesOldNotifications`, `TestReviewR03DisconnectCancelsMaintenance`. | Supplied regressions pass. Handover evidence uses synthetic sockets; the prescribed real coder/websocket delayed-handover fixture is **not present**. Startup changed-subject/expired-rotation/transient-second-validate tests remain incomplete. |
| 4 — remaining app/UI evidence | Added `TestAppTwitchConnectConfigFailureDoesNotStartConnection`: deterministic replacement failure through the app setup boundary leaves state revision unchanged, creates no config file, and starts no connection. Existing actual-JS selector/timestamp/save/manual-rename/disconnect and reload fixtures remain passing. | Route-level config failure passes. Production-app blocked-provider startup/shutdown and full credential seam tests remain **not implemented/tested**; the rendered-browser gate remains separate. |

### Exact checks

| Command/action | Result |
|---|---|
| `gofmt -l ./cmd ./internal ./web` | clean |
| `go test ./... -count=1 -timeout=120s` | all packages passed |
| `go test -tags reviewrepro ./internal/obs ./internal/twitch -run TestReview -count=1 -timeout=60s -v` | 20/20 passed (14 prior + 6 R03) |
| `go vet ./...` | exit 0 |
| `CGO_ENABLED=0 go build -trimpath -o .\dist\cuemender.exe .\cmd\app` | exit 0; 19,731,456-byte artifact |
| `.\dist\cuemender.exe --version` | `cuemender 0.2.0-p02 (commit unknown, built unknown, go1.27.3)` |
| Active-SSE shutdown ×5 | 5/5 passed |
| `node testdata/browser/control_csrf_reload.js` | exit 0; `failures=[]` |
| `node testdata/browser/p02_setup_render.js` | exit 0; selector/timestamp/save/rename/disconnect + observation, `failure=null` |
| gcc / race | gcc absent; race not run |

### Remaining gates

P02-A/V06 and P02-B/V07 remain unmet live gates: no registered Twitch Public client, own-channel/separate-viewer authorization, live EventSub, or live OBS WebSocket evidence. Local fake/protocol fixtures are not live evidence. Rendered-browser verification and race instrumentation remain separate limits. D25 still requires explicit OBS Connect after restart. The blocked production-app shutdown, real-socket delayed handover, and full startup/credential seam items above are not claimed complete.

P02-R03 corrections are ready for review. Read the appended evidence and inspect actual changes against docs/execution/corrections/P02-R03.md and P02-R02-review.md. Rerun the complete review set and required boundary checks. DeepSeek has stopped editing. Record acceptance or remaining corrections with live gates explicit. P03 remains held.
