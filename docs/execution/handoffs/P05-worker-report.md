# P05 worker report

Worker: DeepSeek. Branch: `feature/p05-user-experience`. Starting HEAD: `7210745db0ad557121a098eabad98985a04efa86` (P05 dispatch docs on P04 merge `6c09f11178f8bed8230d000cfd2652b74ea68265`).

Outcome: local P05 v1.1 implementation and local verification are ready for Codex review. Live Twitch (#6), real private-UI OBS approval trial (#10), dependent P05-A/V14 real journey and final V18/V19 remain **unmet**. Rehearsal and local protocol/Node/browser fixtures are not live evidence.

## P05-A — Preflight and visual implementation

- Confirmed released branch, no writer, clean starting worktree before edits.
- Implemented written brief: max-width private surface, LIVE/REHEARSAL badge, compact Setup/Monitor tabs plus Rehearsal only in rehearsal mode, text watermark, overlay URL/copy in Setup, accessible focus ring/reduced-motion CSS, closure controls, fixed public stage copy.
- 12ui retry/brief returned sponsored-budget-exhausted; no generated design/HTML was used or claimed. Codex-authored written brief was followed.
- Rendered local evidence: actual rehearsal binary + headless Chrome 1280×800 screenshot `.tools/p05-rehearsal-1280.png` (monitor/rehearsal state). A 390×844 capture `.tools/p05-rehearsal-390.png` exposed right-edge clipping; CSS was corrected, but Chrome headless's minimum effective viewport still clipped the capture. Mobile/no-overflow, 200% zoom, keyboard and measured contrast are **not verified**.
- Existing five Node actual-script fixtures were preserved; p03 fixture was adapted to locate Recheck/Dismiss/Close by text rather than requiring exactly two action children, preserving and strengthening the behavioral assertion. New `p05_overlay_rehearsal.js` covers watermark, fixed stage copy, public code/category and private-field non-leakage.

## P05-B — Setup and readiness

- Preserved Public-client device flow, OBS endpoint/password reuse, explicit connect after restart, stable discovery, track confirmation and nonsecret form persistence.
- Setup/Monitor tabs are page-session state and do not switch on SSE. Setup includes overlay URL and selectable copy fallback; submitted password fields remain cleared by existing code.
- Disconnected states remain explicit; real monitoring is not bypassed. Live credential/connect/disconnect/OBS discovery remain unchanged. Rehearsal rejects live setup routes with `409 mode_conflict`.

## P05-C — Incidents, approval, feedback, closure

- Preserved existing private approval engine/proposal TTL, feedback/!fixed/!stillbroken intake, latest-vote replacement and bounded replies.
- Added C11 `POST /api/v1/incidents/{id}/close` strict outcome route and transactional `Store.CloseIncident`: stage eligibility, pending-action rejection, viewer/local predicates, idempotent same outcome, conflicting reuse rejection, unused-proposal invalidation, closure outcome/closed_at persistence and cache publication only after commit.
- Control UI shows findings/limitations/feedback, approval details and closure outcome controls; closure failure reloads truthful state.
- Tests: `TestCloseIncidentTransactionalPredicates`, `TestP05ClosureAndRehearsalRouteBoundaries` (closure success/malformed/CSRF/live-mode rehearsal rejection), existing P04 approval/closure tests.

## P05-D — Public overlay

- Overlay now renders public code/category plus fixed stage copy (checking, manual attention, awaiting approval, no supported conflict, incomplete, local verified/viewer confirmed, unknown/unresolved), separate feedback summary, private-field-free allowlist.
- Idle live overlay is hidden; rehearsal watermark remains visible and labeled. Transparent body, compact 420–520px panel, reduced-motion CSS.
- `p05_overlay_rehearsal.js` passes; GET/SSE allowlist canaries from prior P03/P04 tests remain preserved.

## P05-E — Isolated rehearsal

- `--mode live|rehearsal` immutable CLI/App option. Unknown mode fails before listeners/data. Rehearsal effective data directory is `<base>/rehearsal`.
- Migration 4 adds DB mode ownership. Existing unmarked P01–P04 DBs are live-only; marked rehearsal DBs reject live and vice versa. `TestModeOwnershipRejectsCrossMode`.
- Rehearsal App skips Twitch/OBS manager construction and OS-keyring construction entirely; uses memory secrets and in-process simulated provider. `TestP05RehearsalIsolationAndScenario` uses injected counters (keyring/Twitch/OBS config) and asserts zero, child data directory, no managers, `track_missing` → real proposal → one simulated approval/read-back → verified, and closure.
- Private rehearsal scenario/report/feedback routes are strict JSON, private/CSRF, live-mode 409. App-level paths support the four fixed scenarios. Rehearsal feedback uses one simulated actor and shared coordinator.
- Tests do not separately exercise mic_muted/wrong_scene/unknown_write end-to-end, nor full HTTP 202 rehearsal flow; there is no separate forged/cross-mode coordinator rehearsal test beyond the live visibility check.

## P05-F — Verification

| Check | Result |
|---|---|
| `go test ./... -count=1 -timeout=300s` | pass all packages |
| `go vet ./...`; `gofmt -l ./cmd ./internal ./web` | clean |
| CGo-free build `go build -trimpath -o dist/cuemender.exe ./cmd/app` | pass; size 20,253,696; SHA256 `19F611CC65BF8A88C26AFF601F46F0DC219C0C479CE6F1F447A9D1D70393A6BB`; `cuemender 0.5.0-p05` |
| Node fixtures | 5 prior + `p05_overlay_rehearsal.js` pass; Node is not rendered evidence |
| WSL Linux race `go test -race ./... -count=1 -timeout=420s` | pass all packages |
| Rendered Chrome rehearsal 1280 screenshot | captured and inspected; monitor/rehearsal state |

No new dependencies, SDK changes, OBS writes, Git mutations, account work or recording. No owned processes remain; temporary process output was removed.

## Limits and unmet gates

- Live Twitch P02-A/V06, P03-D real chat, P04-A/V11 real OBS approved correction, P05-A/V14 real end-to-end journey, final V18/V19: **unmet/deferred under D34/issues #6/#10**.
- No full rehearsal HTTP/UI end-to-end flow or all four scenario renderings; no separate-viewer actual-app protocol extension for feedback/closure (prior P04 approval protocol preserved).
- No completed mobile/no-horizontal-overflow, 200% zoom, keyboard/screen-reader or measured contrast audit. 390px capture clipped.
- No feedback/closure concurrency stress beyond transactional store and existing coordinator tests.

DeepSeek stopped editing for Codex review. P06 remains held.

## P05-R01 correction evidence (appended)

Correction started/finished UTC: 2026-11-02T04:55Z / 2026-11-02T05:05Z. Base worker checkpoint 3ebf6be plus Codex review tests/docs. Eight supplied `TestReviewP05*` regressions reproduced and fixed; all promoted to ordinary coverage.

### A — Mode ownership and transactional closure

- `OpenMode` now reads `schema_migrations`/`db_meta` before migration. Existing unmarked schema 1/3 DBs are rejected for rehearsal without any schema change; marked rehearsal restart and normal live upgrade remain supported. Tests: `TestReviewP05LegacyModeRejectedBeforeMigration` (1/3), existing `TestModeOwnershipRejectsCrossMode`.
- Closure updates both proposal status and serialized `proposal_json` in the same transaction; coordinator caches are marked invalidated from the committed outcome without a second correctness transaction. `TestReviewP05ClosureInvalidatesSerializedProposalAtomically` passes; prior closure/feedback/store tests remain.

### B — Simulated repair/scenario ownership

- Simulated repair validates source identity, full six-track map and selected track, compares expected to actual fixture state, admits exactly the desired map, honors cancellation before admission/write, mutates only the selected track and returns observed readback. Unknown-write returns unknown/possibly-transmitted without replay.
- Failed scenario persistence no longer changes provider scenario; active diagnostics/pending actions guard transitions. Tests: `TestReviewP05RehearsalSelectedTrackAndReadback`, `PreflightUsesActualMap`, `CanceledRepairCannotWrite`, `FailedScenarioDoesNotMutateProvider`; `TestP05RehearsalAllScenarios` covers all four scenarios end to end through the actual App composition (track-missing approval/verified, mic_muted manual-only, wrong_scene insufficient, unknown_write unknown).

### C — Typed simulated intake/feedback

- Rehearsal report returns the exact current-session/category incident rather than snapshot position; feedback normalizes `stillbroken` to `still_broken`, replaces the single simulated viewer's vote and rejects terminal incidents. Tests: `TestReviewP05ReportResponseMatchesCategory`, `TestReviewP05RehearsalFeedbackRejectsClosedAndRecordsNegative`, plus the all-scenarios test.

### D — Functional visual corrections

- Unconfigured app opens Setup; user section choice is remembered. Simulated providers are labeled simulated. Monitor shows a compact Twitch/OBS/broadcast/monitoring strip. Advanced identity fields collapsed. Closure selects have associated labels, blank default, persisted outcomes and stable IDs; focused control IDs are restored after SSE rebuild.
- Overlay covers applying, unknown/reconciled, local-verified/manual-remaining, closed outcomes and disconnected; idle live remains quiet, rehearsal watermark persists.
- Rendered actual rehearsal binary: `.tools/p05-r01-setup-1280.png` (Setup, 1280×800) and `.tools/p05-r01-overlay.png` (rehearsal overlay). Actual 390×844 capture `.tools/p05-r01-390.png` still clips at the right edge after removing overflow hiding; mobile/no-overflow is **not** claimed.

### E — Verification

| Check | Result |
|---|---|
| `go test ./... -count=1 -timeout=360s` | pass all packages, including promoted eight |
| `go vet ./...`; `gofmt -l ./cmd ./internal ./web` | clean |
| CGo-free build | pass; version `0.5.0-p05`; size 20,270,592; SHA256 `93AFACB1FD2F449BC0263D673382C67841CA3A6B1AB706F1C615A117FB7825CA` |
| Node scripts | six prior/new control/overlay fixtures pass; Node is not rendered evidence |
| WSL `go test -race ./... -count=1 -timeout=480s` | pass all packages |
| Rendered Chrome | Setup 1280 and overlay inspected; 390 clipped/unverified; no 200% zoom/keyboard/contrast audit |

Remaining limits: no full private-HTTP rehearsal 202/error test, no complete production-protocol separate-viewer feedback/closure journey, no deterministic blocked scenario/approval/reconciliation interleaving, no mobile/zoom/keyboard/contrast acceptance. Live Twitch/real OBS gates remain unmet under D34/#6/#10. No Git, real services, dependencies, recording or owned processes.

P05-R01 corrections are ready for review. Read the appended worker evidence, docs/execution/corrections/P05-R01.md and docs/execution/handoffs/P05-review.md. Rerun the eight preserved regressions and required mode/migration, closure/feedback, simulated adapter/scenario, actual-app protocol and rendered UI boundaries. DeepSeek has stopped editing. Record acceptance or consolidated remaining corrections; live Twitch and real OBS approval remain deferred under D34/#6/#10, and P06 stays held.

## P05-R02 correction evidence (appended)

Correction started/finished UTC: 2026-11-02T05:51Z / 2026-11-02T06:07Z. Three supplied failures reproduced and fixed; all promoted. Original eight and prior regressions preserved. No Codex production fixes.

### A — Exact intake/results and feedback

- Rehearsal feedback now requires the current active session and eligible `awaiting_feedback`/`unresolved` stage before coordinator mutation; rejected votes return non-success and never "recorded". `stillbroken` normalized to `still_broken`; one simulated viewer's negative replaces positive.
- `TestReviewP05R01FeedbackRejectsUnacceptedVotes` (both subcases) passes/promoted. `TestP05RehearsalPrivateHTTPFlow` covers actual private HTTP scenario/report 202/feedback positive→negative/close 200/post-close 409/wrong-scene video.
- Production local OAuth/Helix/EventSub/OBS fixture now sends a separate viewer `!fixed CM-...` and closes viewer-confirmed. This exposed and fixed a real parser defect: `validIncidentCode` rejected the hyphen in CueMender public codes. `TestP04ActualAppPrivateApprovalProtocolFlow` passes with SQLite feedback/closure and idempotent/conflicting close.
- UI report uses selected scenario (wrong_scene→video), stores accepted incident, and feedback targets current-session eligible incident rather than history[0].

### B — Coherent scenario/provider and fixture provenance

- Rehearsal state owns generation/revision, advances generation on scenario, and Read/Repair/ReadTrack validate actual fixture source/context/generation/revision; readback returns fixture-owned observed state. Discovery includes a direct visual item.
- Scenario applies provider before settings publication and restores the previous provider on persistence failure; active diagnostics/pending actions reject transition. `TestReviewP05R01ScenarioPublishesMatchingProvider` now sees matching published settings/observation and generation advance; `TestReviewP05R01ReconciliationRejectsForeignSource` passes/promoted.
- OpenMode now distinguishes missing metadata tables/rows from query failures.
- Gap: admission guard plus stop/save is not a single coordinator-owned atomic transaction; no deterministic barrier test versus blocked diagnosis/approval/reconciliation.

### C — UI and remaining evidence

- Incident rendering uses stable signatures to reuse unchanged cards; closure selects/labels/blank outcome/persisted value remain; focus IDs restore. Setup initial/unconfigured and section memory retained. Overlay EventSource error unhides disconnected panel.
- Existing six Node fixtures pass, including adapted p03 controls and overlay error assertion. No full all-scenario rendered matrix, 390 no-overflow/200% zoom/keyboard/measured contrast; 390 capture still clips.
- No deterministic concurrent scenario/approval/reconciliation test. Private HTTP rehearsal journey is covered; production protocol separate-viewer flow is now covered.

### Verification

| Check | Result |
|---|---|
| `go test ./... -count=1 -timeout=360s` | pass all packages |
| `go vet`; `gofmt -l cmd internal web` | clean |
| CGo-free build | `0.5.0-p05`; 20,281,856 bytes; SHA256 `4613FE6990AD8639E733708C27864FD5FBA1CF5DBACCC1480929E31D79B5453E` |
| Six Node actual-script fixtures | pass |
| WSL `go test -race ./... -count=1 -timeout=480s` | pass all packages |
| Rendered Chrome | prior Setup/overlay captures; new captures attempted but 390 still clips. No 200% zoom/keyboard/contrast. |

No new dependencies, real services, Git actions, recording or owned processes. Live Twitch/real OBS gates remain unmet under D34/#6/#10; P06 held.

P05-R02 corrections are ready for review. Read the appended worker evidence, docs/execution/corrections/P05-R02.md and docs/execution/handoffs/P05-R01-review.md. Verify typed acceptance, scenario/provenance ownership, actual HTTP/protocol journey and rendered UI boundaries while preserving all prior regressions. DeepSeek has stopped editing. Record acceptance or remaining corrections; D34/#6/#10 live holds and P06 hold remain.
