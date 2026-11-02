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
