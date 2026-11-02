# P03 worker report

## Identity and ownership

- Packet / revision: P03 — Evidence and incident lifecycle v1.2
- Worker/harness: DeepSeek (`deepseek-flash` via DeepSeek Harness)
- Started / finished UTC: 2026-11-01T15:46Z / 2026-11-01T16:05Z
- Starting checkpoint: branch `feature/p03-diagnostics`, HEAD `ffc71d8b55b7d5cdb09b27117f1731d946fac56e`; clean worktree. P03 baseline `af2ec69ce95cdba6fc9f080cc177da34695fbfb0`.
- Result: **ready_for_review with live Twitch gates deferred**
- Editing stopped: yes
- Processes stopped: yes; no owned server/test process left running
- STATUS updated: claimed P03, then transferred to Codex

## What changed

| Path | Change | Purpose |
|---|---|---|
| `internal/domain/incidents.go` | added | Typed checks, findings, incidents, sessions, stages, feedback counts |
| `internal/domain/errors.go`, `projections.go` | modified | Safe P03 errors; private session/incidents projection |
| `internal/diagnostics/diagnostics.go`, tests | added | Three pure checks, limits/unknown rules |
| `internal/incidents/coordinator.go`, tests | added | Sessions, jobs, intake, dedup grouping, recheck, dismiss, feedback, projections |
| `internal/store/store.go` | modified | Forward migration 2 (sessions/incidents/findings/reports/dedup/feedback) |
| `internal/store/incidents.go`, tests | added | Atomic report/dedup/grouping, settings-safe migration, history, feedback, retention |
| `internal/obs/manager.go` | modified | `DiagnosticRead` fresh five-second read with admission/cancellation/provenance |
| `internal/twitch/manager.go` | modified | P03 reply ownership seam (`SuppressStaticReplies`, `EnqueueReply`) |
| `internal/app/app.go`, `diagnostics.go` | modified/added | Coordinator/read/status wiring, command callback, P03 app methods, projections |
| `internal/app/p03_test.go` | added | Real app composition, fake OBS, command→finding, persistence, restart, public redaction |
| `internal/httpui/server.go`, `server_test.go`, `p03_routes_test.go` | modified/added | Four C09 private routes, errors, boundary/public tests |
| `web/control/index.html`, `app.js` | modified | Start/stop, manual checks, incident cards, recheck/dismiss, counts/evidence |
| `testdata/browser/p03_incidents.js` | added | Actual-script P03 control fixture |
| `docs/implementation/P03-diagnostics.md` | added | Implemented behavior/storage/limits |
| `docs/implementation/P02-compatibility.md`, `P02-setup.md` | modified | P03 integration and operator facts |
| `internal/app/version.go` | modified | `0.3.0-p03` |

No new dependency versions; patched goobs unchanged. No specification edits, OBS writes, P04 code or Git mutation.

## Acceptance mapping

| Criterion | Evidence | Result |
|---|---|---|
| P03-A / V09 | `internal/diagnostics` table tests: all three checks, simultaneous mute+track, clean, partial, stale/future, revision/generation mismatch, wrong scene, missing source, nested group | pass local |
| P03-B / V10 | Store dedup/grouping/counts tests; coordinator duplicate ack suppression, start/session rules; existing Twitch cooldown/own-account/queue tests | pass local; no live chat |
| P03-C | `TestCoordinatorStopRejectsLateBlockedRead`, revision/generation check at job commit, dismissal cancels job, stop interrupts work | pass local |
| P03-D local | `TestP03ApplicationWiringPersistsIncidentAndRestart`: actual app composition with fake OBS provider, normalized command callback, fresh diagnostic read, persisted findings, private/public projections, stop, SQLite reopen | pass local |
| P03-D live | Real Twitch chat driving a real OBS-backed finding | **deferred/unmet under D30/issue #6** |
| P03-E | Clean evidence → `no_supported_finding`; unknown evidence never healthy | pass local |
| P03-F | Version-1 database upgrade preserves settings/revision; repeat migration/schema-newer tests | pass local |
| Routes/UI | route tests incl. 401/403/400/404/409/429/503, public redaction; actual-script P03 start/stop/manual/recheck/dismiss/reload/CSRF/SSE | pass actual-script; no rendered browser |
| Provider boundary | `DiagnosticRead` hits fresh read-only OBS adapter; existing 20 P02 review assertions still pass; outbound OBS allowlist unchanged | pass local; no live OBS |

## Exact checks

| Command/action | Result |
|---|---|
| `gofmt -l ./cmd ./internal ./web` | clean |
| `go test ./... -count=1 -timeout=180s` | exit 0; all packages |
| `go vet ./...` | exit 0 |
| `CGO_ENABLED=0 go build -trimpath -o dist/cuemender.exe ./cmd/app` | exit 0; 19,954,176 bytes |
| `dist/cuemender.exe --version` | `cuemender 0.3.0-p03 (commit unknown, built unknown, go1.27.3)` |
| `go test ./internal/app -run TestRunServesAndShutsDownWithActiveSSE -count=5` | 5/5 passed |
| `node testdata/browser/control_csrf_reload.js` | `failures=[]` |
| `node testdata/browser/p02_setup_render.js` | `failure=null` |
| `node testdata/browser/p03_incidents.js` | start/stop/manual/recheck/dismiss/reload/CSRF requests, `problems=[]` |
| WSL Linux `go test -race ./... -count=1 -timeout=240s` | final rerun exit 0; all packages race-clean. One earlier full-race run hit the existing active-SSE shutdown timing edge; focused race run passed 5/5 and the full suite passed on rerun. |
| Go 1.27.3 Windows amd64; Node v24.14.1; local CGo-free build | used |

## Behavior demonstrated

Local fixture path: app starts with isolated data/listen port, connects a fake read-only OBS backend, saves confirmed mic/track expectations, starts a monitoring session, accepts a normalized vendor command event through the real app callback, persists the report, executes a fresh diagnostic read (blocked/partial variants return insufficient), stores two findings (mute + missing track), publishes private incidents and a redacted public incident, stops monitoring, closes, then reopens SQLite and verifies settings/history. Duplicate delivery/message IDs are rejected across restart. Manual checks while stopped create zero-report manual incidents. Dismissal and negative feedback transition correctly; old-session/terminal feedback is ignored.

## Deviations / unresolved issues

- **P03-D live Twitch remains deferred/unmet** under D30/issue #6. No login, registration, authorization, credential request, real chat or live Twitch evidence was performed. Local command callback is not live evidence.
- P02-A/V06 remains deferred/unmet. P03 did not repeat live OBS comparison; no OBS configuration was changed.
- No rendered-browser check was available; actual-script Node VM fixtures are not browser proof.
- Boundary coverage still thin for: true concurrent report transactions, 10,000-report/dedup exact bounds, blocked whole-app diagnostic shutdown, and recheck after a settings change. The code guards these, but they were not independently reproduced.
- Queue/feedback is minimal; P05 owns final UX. P04 remains absent.
- No dependency changes. Migration 2 is forward-only.

## Reviewer reproduction

1. Use the portable Go environment; run gofmt, `go test ./...`, vet, CGo-free build.
2. Run `node testdata/browser/control_csrf_reload.js`, `p02_setup_render.js`, `p03_incidents.js`.
3. For focused P03: `go test ./internal/diagnostics ./internal/store ./internal/incidents ./internal/app -count=1 -v`.
4. Optionally run the workspace Linux race command from the packet.
5. Do not perform live Twitch login under the current hold. A manual OBS check may be repeated only without changing the user's OBS.

No owned processes remain. Bootstrap URLs/credentials are not included.

## Contribution and handoff

DeepSeek implemented P03 diagnostics, migration, coordinator, routes, projections, controls and tests against Codex's P03/C09 contracts, building on Codex's P02 closure. P03 is not accepted by this report. P04 remains held for Codex and must not be started by DeepSeek.

P03 worker handoff is ready. Read docs/execution/handoffs/P03-worker-report.md and docs/implementation/P03-diagnostics.md; inspect actual changes against P03 v1.2 and CONTRACTS v1.3/C09; reproduce focused diagnostics, persistence, session/lifecycle, route and UI boundary checks. DeepSeek has stopped editing. Record acceptance, consolidated corrections or specific blockers. Keep P02-A/V06 and P03-D live Twitch deferred under D30/issue #6; do not count fixtures as live evidence. P04 remains held for Codex's explicit release and recording checkpoint.

## P03-R01 correction evidence (appended)

Correction started/finished UTC: 2026-11-01T17:24Z / 2026-11-01T17:34Z. Parent P03 v1.2/C09. Codex review fixtures/documents were preserved; no production fixes from Codex. All 13 review Go regressions and the 3 actual-script failures were reproduced first, then fixed; review Go files were promoted by removing the opt-in tag. Existing worker tests contrary to C09 were corrected (wrong-scene sufficiency and feedback eligibility).

### R01–R06 matrix

| Group | Implemented evidence | Result |
|---|---|---|
| R01 identity/context/category | Microphone requires discovered+observed identity, known/matching collection, revision/generation, age; track diagnosis requires explicit confirmation; mute survives missing confirmation. Wrong scene/unknown context is limitation. `EvaluateFor` filters by incident category; unconfigured category is insufficient. Mode is live with viewer/manual origin. Tests: `TestReviewP03RejectUnresolvedMicrophone` (5 subcases), `WrongSceneIsLimitation`, `CategoryIsolation`, diagnostics worker tests. | pass local |
| R02 coordinator ownership | Operation gate serializes start/stop/check/report/dismiss/result commit. Provider status/settings outside state lock. Jobs reserved before scheduling; repeat/concurrent start one session and revision checked; closed coordinator rejects. Manual active joins session; stopped manual separate scope. Dismiss cancels only target job; joining reports don't re-read completed diagnosis. Late commit revalidates job/session/revision/generation; stop persists before memory change and retries errors. Tests: `ActiveStartChecksRevision`, `ActiveManualJoinsSession`, `DismissDoesNotCancelOtherIncident`, `ReportsCoalesceCompletedDiagnosis`, `ProviderStatusOutsideCoordinatorLock`, `FailedStopDoesNotSilentlyDiverge`, concurrency/manual-stop tests, existing late-read barrier. | pass local |
| R03 persistence/bounds | AddFeedback single transaction (vote/count/transition); SaveIncident merges live report/feedback counters instead of overwriting; durable dedup expires at the retained instant, prunes to 10,000 after insertion, and survives reopen. Retained duplicates inert. Tests: `DedupExpiresAt24Hours`, `DedupBoundAfterInsertion`, `FeedbackFailureIsAtomic`, migration/reopen/retention tests. | pass local |
| R04 Twitch intake/replies | Bounded app intake queue owns command work and is canceled/joined on close; generation/receipt-time validated before durable report/feedback; reply uses existing queue with generation check; feedback only awaiting_feedback/unresolved; ack distinguishes new diagnosis vs joined incident; P02 static ack suppressed only in coordinator wiring. | implemented; actual EventSub protocol→reply fixture remains a gap (below) |
| R05 truthful controls | Cards render per-finding guidance, limitations and stage-sensitive wording; unknown is not “no supported conflict”; category/origin/evidence time/revision/feedback visible; recheck hidden for terminal/history; public service status derived from monitoring/job state. `p03_review_evidence.js` and p03 actual-script pass. | pass actual-script; no rendered browser |
| R06 integration/evidence | 13 review tests promoted ordinary and passing; original P02/P03 suites preserved; Node fixtures; WSL race; repeated lifecycle. App composition test exercises coordinator/store/fresh OBS/command/reply/private/public/restart, but still injects readiness and invokes the normalized callback rather than a full EventSub socket chain. | partial; explicit gap below |

### Exact checks

| Command/action | Result |
|---|---|
| `go test ./... -count=1 -timeout=180s` | exit 0; all packages |
| `go test -tags reviewrepro ./internal/diagnostics ./internal/incidents ./internal/store -run TestReviewP03 -count=1 -timeout=45s` | exit 0; 13/13 review tests (now also ordinary) |
| `go vet ./...`; `gofmt -l ./cmd ./internal ./web` | clean |
| `CGO_ENABLED=0 go build -trimpath -o dist/cuemender.exe ./cmd/app` | exit 0; 19,981,824 bytes; SHA256 `D1086C4FB59BEB42FE809BC3716CB3233ADA31AA244ADD7BFC914FAAB8C5E1E5`; `0.3.0-p03` |
| Active-SSE + blocked-provider lifecycle ×5 | 5/5 passed |
| `node control_csrf_reload.js`, `p02_setup_render.js`, `p03_incidents.js`, `p03_review_evidence.js` | all exit 0; review `failures=[]`; p03 `problems=[]` |
| WSL Linux `go test -race ./... -count=1 -timeout=240s` | exit 0; all packages race-clean. Earlier full-race active-SSE timing failure retained: focused race 5/5 and full rerun passed. |

### Changed in this correction

`internal/diagnostics/diagnostics.go` + tests; `internal/incidents/coordinator.go` + tests; `internal/store/incidents.go` + tests; `internal/app/{app,diagnostics}.go` + P03 test; `internal/twitch/manager.go`; `internal/httpui/server.go`/tests; `web/control/{app.js,index.html}`; `testdata/browser/p03*.js`; P03/docs. Review fixtures promoted. No dependency, SDK, spec or OBS-mutation changes; no Git mutation.

### Remaining gates / explicit gaps

- P03-D live Twitch chat-to-finding and P02-A/V06 remain deferred/unmet under D30/issue #6. No Twitch login/authorization/real chat.
- The local app test injects monitoring readiness and invokes the normalized callback; it is composition evidence, not actual application readiness/EventSub-socket→reply evidence. This is still a required local boundary and is not claimed complete.
- No rendered-browser check; Node actual-script only. No fresh live OBS comparison.
- True concurrent reporter transactions and exact 10,000 reporter/feedback bounds are covered by implementation and store review cases but not a high-volume stress fixture.
- Migration 2 is forward-only; no downgrade/reset.

P03-R01 corrections are ready for review. Read the appended P03-R01 evidence and inspect actual changes against docs/execution/corrections/P03-R01.md and docs/execution/handoffs/P03-review.md. Rerun the preserved review regressions and required app/protocol/persistence/lifecycle/UI failure paths. DeepSeek has stopped editing. Record acceptance or consolidated remaining corrections, keeping live Twitch deferred under D30/issue #6. P04 remains held for Codex release and recording.

## P03-R02 correction evidence (appended)

Correction started/finished UTC: 2026-11-01T17:48Z / 2026-11-01T17:57Z. Parent P03 v1.2/C09. Preserved original 13 review tests and R01 fixes. Six new failures were reproduced first, fixed, and promoted; the exact-bound test was preserved/promoted. No production fixes by Codex.

### A–D matrix

| Group | Implemented evidence | Result |
|---|---|---|
| A admission/cancellation/commit ownership | Closed coordinator rejects Start/Check/report/feedback/dismiss before persistence. Job maps are retained until workers exit; canceled/dismissed reads cannot admit an overlapping read. Stop records interrupted manual incidents and publishes; failed durable stop remains retryable. Manual and monitoring commits revalidate job/session/revision/OBS generation/connection inside the serialized gate; settings disconnect/provider invalidation marks affected evidence interrupted. Tests: `TestReviewR01ClosedCoordinatorRejectsCheck`, `ManualResultRejectsReplacedOBS`, `ManualStopRecordsInterrupted`, `CanceledReadRetainsCategoryAdmission`, plus existing late-read/concurrency tests. | pass local |
| B intake/reply/protocol | App intake is a bounded queue with owned cancel context; commands/feedback carry Twitch generation and receipt time; zero generation is not an unconditional bypass; replies require originating generation and active current session. `TestP03ActualAppProtocolReportAndReply` now runs the real app with local OAuth/Helix/EventSub WebSocket and read-only OBS protocol servers: genuine readiness, actual socket report, fresh finding/persistence, and actual bounded outgoing Helix reply request. No readiness override or direct command callback. | pass local protocol; no live Twitch |
| C durable ordering/recovery/retention | Dedup expiry/retention and incident ordering use SQLite Julian-day chronology, fixing fractional RFC3339Nano comparisons; exact TTL test passes. Feedback remains one transaction and merges counters. Startup interrupts all open records, not only the first page; retention reloads cache after dismiss/stop. `TestReviewR01DedupFractionalExpiry`, `TestReviewR01ExactReporterFeedbackBounds` (real 10,000 reports and 10,000 feedback actors) pass and are ordinary coverage. | pass local; recovery-error propagation remains partial |
| D identity/recovery/projections/streaming | Configured scene UUID is required to match observed program UUID; missing identity is unknown. Observation-stage DiagnosticRead failure invalidates/reconnects transport. Incident evidence stores actual streaming true/false/unknown; private cards show it. Public monitoring/incidents/service status use one coordinator public snapshot plus live provider connectivity; stopped/disconnected status is truthful. UI guidance/limitations/recheck eligibility from R01 retained. | pass local; no rendered browser/live OBS |

### Exact checks

| Command/action | Result |
|---|---|
| `go test ./... -count=1 -timeout=180s` | exit 0; all packages |
| `go test -tags reviewrepro ./internal/diagnostics ./internal/incidents ./internal/store -run 'TestReviewP03|TestReviewR01' -count=1 -timeout=60s` | exit 0; original 13 + 6 R02 + exact-bound tests |
| `go vet ./...`; `gofmt -l ./cmd ./internal ./web` | clean |
| `CGO_ENABLED=0 go build -trimpath -o dist/cuemender.exe ./cmd/app` | exit 0; 19,996,160 bytes; SHA256 `7BC644759A96D50360B2C8D9631A607973957F0191493B8BFCBF409667DE60D9`; `0.3.0-p03` |
| App protocol/lifecycle repeats ×5 | `TestP03ActualAppProtocolReportAndReply`, active-SSE and blocked-provider shutdown all passed 5/5 |
| Four Node fixtures | control/p02/p03/review all exit 0; review failures=[]; no rendered browser |
| WSL Linux `go test -race ./... -count=1 -timeout=240s` | exit 0; all packages race-clean |

### Changed in this correction

`internal/incidents/coordinator.go` + tests; `internal/diagnostics/diagnostics.go` + tests; `internal/store/incidents.go` + tests; `internal/app/{app,diagnostics}.go`, new `p03_protocol_test.go`; `internal/twitch/manager.go`; `internal/obs/manager.go`; `internal/domain/incidents.go`; `web/control/app.js`; promoted review tests; P03 docs. No new dependency/SDK/spec/OBS-write/Git changes.

### Remaining gaps

- Live P02-A/V06 and P03-D under D30/issue #6 remain deferred/unmet; local OAuth/EventSub/OBS protocol fixtures are not live evidence.
- Startup recovery still ignores store errors and in-memory history is capped at 500 (all open records are interrupted in SQL); recovery-error propagation/large-history presentation remains partial.
- Settings/provider invalidation is wired but a dedicated blocked-result-after-SaveSettings app test was not added.
- No rendered-browser verification. Live OBS comparison was not repeated.

P03-R02 corrections are ready for review. Read the appended P03-R02 evidence and inspect actual changes against docs/execution/corrections/P03-R02.md and docs/execution/handoffs/P03-R01-review.md. Rerun preserved regressions and the required actual-app/protocol/cancellation/persistence/projection boundaries. DeepSeek has stopped editing. Record acceptance or remaining corrections; live Twitch remains deferred under D30/issue #6, and P04 stays held for Codex release and recording.

## P03-R03 correction evidence (appended)

Correction started/finished UTC: 2026-11-02T01:14Z / 2026-11-02T01:21Z. Parent P03 v1.2/C09. Preserved all earlier regressions, exact-bound and actual protocol tests. Four supplied failures reproduced first, fixed, promoted; no production fixes by Codex.

### A–D matrix

| Group/boundary | Evidence | Result |
|---|---|---|
| A startup recovery failure | `NewChecked` propagates session/recovery/list/interruption errors; App.New fails and releases resources. `TestReviewR02StartupRecoveryErrorPropagates`; ordinary app suite | pass |
| A prior open records/all retained | Recovery interrupts all SQL open rows; `ListRetainedIncidents` loads all non-terminal plus latest 500 completed. `TestCoordinatorLoadsRetainedRecordBeyondCompletedPage` (>500) | pass |
| A manual Stop failure | Stop returns durable failure, does not update cache/publish unsaved outcome; retry remains possible because job ownership is retained. `TestReviewR02ManualStopFailureIsReported` | pass; recoverable-fault retry test not added (store handle unexported) |
| A canceled/timeout settlement | Canceled/failed jobs settle reported/investigating records as interrupted in `finishJob`; job removal publishes settled state. Existing cancel/stop tests | implemented |
| B cancellable admission | Context-aware gate replaces unbounded production opMu wait; canceled Check returns without mutation. `TestReviewR02CanceledGateReturnsPromptly` | pass |
| B disconnected/generation intake | Report/feedback require connected current nonzero generation; old generation is rejected. `TestReviewR02DisconnectedTwitchRejectsQueuedReport` | pass |
| B idle help/session reply | Sessionless `!streamhelp` replies while connected/current; report replies require active originating session. Actual protocol test covers idle help before monitoring | pass local |
| C SaveSettings blocked result | Real App.SaveSettings invalidates blocked job; old evidence interrupted with zero findings; explicit recheck uses new revision. `TestP03SaveSettingsInvalidatesBlockedDiagnosticAndRecheck` | pass |
| C whole-app blocked diagnostic shutdown | Active diagnostic + app Close returns, listener released, DB reopened. `TestP03BlockedDiagnosticWholeAppShutdown` | pass |
| D protocol fixture ownership | EventSub upgraded handler now exits on socket close; handler WG joined; finding verified through SQLite, not only cache. Idle help and report replies verified. `TestP03ActualAppProtocolReportAndReply` | pass local; rendered/live not run |
| D evidence allowlist | OBS fixture still uses read-only methods by construction; explicit request allowlist assertion not added | gap |

### Exact checks

| Command/action | Result |
|---|---|
| `go test ./... -count=1 -timeout=240s` | exit 0; all packages |
| `go vet ./...`; `gofmt -l ./cmd ./internal ./web` | clean |
| `CGO_ENABLED=0 go build -trimpath -o dist/cuemender.exe ./cmd/app` | exit 0; 20,002,816 bytes; SHA256 `16A0BB16CEBE3ECA6754F5D491A899F723D8432AAF9D451CE0263DD84844CA9B`; `0.3.0-p03` |
| Four Node fixtures | all exit 0; review failures=[]; p03 problems=[] |
| Focused app/incidents repeats ×3 | protocol, blocked shutdown, SaveSettings boundary, R02 regressions, retained-history all pass |
| WSL Linux `go test -race ./... -count=1 -timeout=300s` | exit 0; all packages race-clean |

### Remaining gaps

- Live P02-A/V06 and P03-D under D30/issue #6 remain deferred/unmet; local fixtures are not live evidence.
- Recoverable SQL-fault retry after a failed Manual Stop lacks a dedicated test because the store DB handle is unexported; closed-store failure is covered.
- Explicit OBS request allowlist assertion, post-start reload/query-error propagation, rendered-browser verification and fresh live OBS were not executed.
- Startup recovery/list errors now fail App.New; post-start reload still uses the capped helper in some paths.

P03-R03 corrections are ready for review. Read the appended worker evidence, docs/execution/corrections/P03-R03.md and docs/execution/handoffs/P03-R02-review.md. Inspect actual changes and rerun preserved regressions plus the required recovery, cancellation, session, settings/provider, protocol-cleanup and projection boundaries. DeepSeek has stopped editing. Record acceptance or remaining corrections. Live Twitch stays deferred under D30/issue #6; P04 remains held for Codex release and recording.

## P03-R04 correction evidence (appended)

Correction started/finished UTC: 2026-11-02T01:44Z / 2026-11-02T01:48Z. Preserved R03 improvements. Four supplied failures reproduced first, fixed; all five review tests promoted to ordinary coverage. No production fixes by Codex.

### A–C matrix

| Group/boundary | Evidence | Result |
|---|---|---|
| A retained reload after dismiss | `reload` now uses `ListRetainedIncidents` and swaps cache only on success; dismiss/stop surface reload errors. `TestReviewR03RetainedHistorySurvivesDismissReload` (502 retained) | pass |
| A recoverable manual-stop retry | Failed interruption retained in `pending` after worker exit; retry re-reads current durable stage, avoids overwriting newer work, settles interrupted. `TestReviewR03ManualStopRetriesAfterRecoverableSQLFault` using second SQLite connection/trigger | pass |
| A settled job publication | `finishJob` publishes after success and failure; `ActiveJobs` is zero in final publication. `TestReviewR03SuccessfulJobPublishesSettledCount` | pass |
| A cleanup synchronization | `finishJob` re-reads durable incident, skips terminal/newer investigating state, retains failed saves as pending | implemented; concurrent dismiss/recheck-specific test not added |
| B production gate cancellation | Context-aware gate; obsolete opMu removed and old occupied-gate test now holds real gate. `TestReviewR03CanceledProductionGateReturns`, adapted `TestReviewR02CanceledGateReturnsPromptly` | pass |
| B Close admission | Close cancels admission, waits for gate holder, marks closed; no commit/publication after Close returns. `TestReviewR03ClosePreventsLateStartCommit` | pass |
| B originating session | Command envelope captures monitoring session ID; queued report/feedback dropped if that exact session is inactive; replies recheck same session plus generation; idle help stays sessionless. Protocol test covers idle help/report. Dedicated stale-feedback/queue-across-restart test not added | implemented/partial evidence |
| B bounded persistence | Result save uses 3s owned timeout; cleanup uses 2s bounded context; Close joins workers | implemented |
| C active HTTP/SSE shutdown | `TestP03BlockedDiagnosticWholeAppShutdown` now runs App.Run, establishes active SSE snapshot, cancels with blocked diagnostic, asserts shutdown/listener release/DB reopen | pass |
| C settings/recheck | `TestP03SaveSettingsInvalidatesBlockedDiagnosticAndRecheck` covers blocked result invalidation and explicit recheck; evidence revision assertion is implicit in findings, not separately asserted | pass/partial |
| C OBS read-only allowlist | Protocol OBS fixture records every request; test fails any type outside the read-only allowlist | pass |
| C protocol ownership | EventSub upgraded handler/socket joined; finding verified in SQLite; idle help and report replies. Failure-path upgraded-socket test not added | pass happy path |

### Exact checks

| Command/action | Result |
|---|---|
| `go test ./... -count=1 -timeout=300s` | exit 0; all packages |
| `go vet ./...`; `gofmt -l ./cmd ./internal ./web` | clean |
| `CGO_ENABLED=0 go build -trimpath -o dist/cuemender.exe ./cmd/app` | exit 0; 20,007,936 bytes; SHA256 `A35895C31D6A059CF2970EAAF428E27049BB4E9F1B68A0A92CF6C7236BCB396D`; `0.3.0-p03` |
| Four Node fixtures | all exit 0; review failures=[]; p03 problems=[] |
| R04/R03 protocol/lifecycle repeats ×3 | review tests, protocol, SSE shutdown and settings boundary pass |
| WSL Linux `go test -race ./... -count=1 -timeout=360s` | exit 0; all packages race-clean |

### Remaining named gaps

- No dedicated automatic OBS disconnect/replacement test (manual Disconnect invalidation is covered).
- No dedicated queued-report-across-stop/start, stale-feedback-after-replacement, or Close-while-database-admission-blocked test; session-ID capture/validation is implemented.
- No dedicated concurrent cleanup/dismiss/recheck test; conditional durable-stage check is implemented.
- No EventSub upgraded-socket failure-path cleanup test; happy-path join is covered.
- SaveSettings test does not separately assert the evidence revision field.
- No rendered-browser or fresh live OBS check. Live P02-A/V06 and P03-D remain deferred under D30/issue #6.

P03-R04 corrections are ready for review. Read the appended worker evidence, docs/execution/corrections/P03-R04.md and docs/execution/handoffs/P03-R03-review.md. Inspect actual changes and rerun preserved regressions plus retained-state, recoverable-fault, close/session ownership, publication and required actual-app/protocol boundaries. DeepSeek has stopped editing. Record acceptance or remaining corrections. Live Twitch remains deferred under D30/issue #6; P04 stays held for Codex release and recording.

## P03-R05 correction evidence (appended)

Correction started/finished UTC: 2026-11-02T01:59Z / 2026-11-02T02:01Z. Preserved R04 improvements. One supplied failure reproduced first (`TestReviewR04IdleQueuedReportCannotEnterLaterSession`), fixed; both queue tests promoted. No production fixes by Codex.

### A–C matrix

| Group/boundary | Evidence | Result |
|---|---|---|
| A empty captured session rejected | Reports/feedback with empty MonitoringSessionID are dropped; idle help remains eligible. Promoted `TestReviewR04IdleQueuedReportCannotEnterLaterSession` | pass |
| A old captured session rejected | Captured old session is revalidated at process and coordinator durable admission; same identity required for reply. Promoted `TestReviewR04QueuedOldSessionReportIsDropped` | pass |
| A stale feedback/provider replacement | `TestCoordinatorStaleFeedbackAfterProviderReplacementIsDropped`; no vote/ack | pass |
| A idle help no incident | Existing active-protocol idle help; promoted queue tests show no incident creation | pass |
| B atomic cleanup | New `Store.InterruptIncident` conditional transaction tied to observed stage/update version; cleanup cannot overwrite newer recheck/terminal. `finishJob` uses it | implemented; dedicated concurrent dismiss/recheck test not added |
| B bounded close/DB | Existing Close admission/join, active HTTP/SSE diagnostic shutdown, listener release, DB reopen preserved | pass; Close-while-DB-admission-blocked test still absent |
| C automatic provider replacement | `TestCoordinatorAutomaticProviderReplacementInvalidatesJob` (OBS generation changes during blocked read; no finding becomes current) | pass at coordinator seam |
| C settings evidence revision | `TestP03SaveSettingsInvalidatesBlockedDiagnosticAndRecheck` now asserts finding ExpectationRevision equals new revision | pass |
| C protocol failure cleanup | Happy-path EventSub handler/socket join, SQLite finding and OBS read-only allowlist preserved | pass; dedicated failure/cancellation socket test not added |
| C prior evidence | Active HTTP/SSE shutdown, local protocol, Node, exact-bound and earlier regressions preserved | pass |

### Exact checks

| Command/action | Result |
|---|---|
| `go test ./... -count=1 -timeout=360s` | exit 0; all packages |
| `go vet ./...`; `gofmt -l ./cmd ./internal ./web` | clean |
| `CGO_ENABLED=0 go build -trimpath -o dist/cuemender.exe ./cmd/app` | exit 0; 20,014,080 bytes; SHA256 `57383E52A17EA6FFA7D1E0FF4678044E9A10C4A7E89987095DE26D132D2204E5`; `0.3.0-p03` |
| Four Node fixtures | all exit 0; review failures=[]; p03 problems=[] |
| Queue/session/settings/provider/protocol repeats ×3 | all focused tests pass |
| WSL Linux `go test -race ./... -count=1 -timeout=360s` | exit 0; all packages race-clean |

### Remaining named gaps

- No dedicated Close-while-database/admission-blocked test; bounded contexts and Close joining are implemented.
- No dedicated EventSub upgraded-socket failure/cancellation cleanup test.
- No dedicated deterministic concurrent cleanup/dismiss/recheck test; conditional transactional write protects the path.
- Automatic provider replacement is exercised at the readiness-generation seam, not a real socket drop/reconnect.
- No rendered-browser or fresh live OBS check. Live P02-A/V06 and P03-D remain deferred under D30/issue #6.

P03-R05 corrections are ready for review. Read the appended worker evidence, docs/execution/corrections/P03-R05.md and docs/execution/handoffs/P03-R04-review.md. Inspect actual changes and verify captured-session mutation/replies, atomic cleanup and the named provider/database/settings/protocol failure boundaries, preserving all prior regressions. DeepSeek has stopped editing. Record acceptance or remaining corrections. Live Twitch stays deferred under D30/issue #6; P04 remains held for Codex release and recording.
