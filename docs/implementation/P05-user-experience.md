# P05 user experience

Revision: P05 v1.1 implementation (local development). Live Twitch and the real OBS approval trial remain deferred under D34/issues #6 and #10. Rehearsal and local protocol fixtures are not live evidence.

## Modes and directories

`--mode live|rehearsal` is immutable after construction. Live is the default. Rehearsal resolves the effective data directory as `<ResolveDataDir(--data-dir)>/rehearsal` and never loads the base live config/database/credentials. The SQLite schema is version 4 (`db_meta` mode ownership; incident closure remains in the incident JSON and indexed stage columns). Existing unmarked P01–P04 databases are live-only; marked rehearsal databases reject a live process and vice versa.

Rehearsal constructs no Twitch manager, OBS manager/dialer/sender or OS-keyring store. It uses memory-only secrets and an in-process simulated provider. The App test injects constructor counters and asserts all are zero. Live Twitch/OBS setup routes return `409 mode_conflict` in rehearsal.

## Control surface

Setup includes connection status, explicit OBS connect/disconnect, discovery selectors, confirmed-stream-track settings, visual expectation, overlay URL/copy fallback and mode badge. Setup/Monitor/Rehearsal are keyboard-operable tabs; SSE does not switch the active section. Rehearsal is visible only in rehearsal mode and always carries a text watermark.

Monitor shows private incidents with code/category/origin/stage/time, findings, limitations, unknown/stale evidence, expectation revision, feedback counts and approval state. It adds C11 closure outcomes and explicit unknown/expired/pending wording. The public overlay uses fixed stage/category copy plus the public code and feedback summary; it is hidden when idle, truthful when disconnected, and keeps the rehearsal watermark.

## Broadcaster closure

`POST /api/v1/incidents/{id}/close` accepts strict JSON `{"outcome":"viewer_confirmed"|"local_only"|"unresolved"}` on the normal private session/CSRF/Host/Origin boundary. `Store.CloseIncident` is transactional: eligible stage check, pending-action rejection, closure predicate check, unused-proposal invalidation, JSON/stage/closed_at update. Same outcome is idempotent; a different outcome is a conflict; DB failure leaves memory/publication unchanged. Viewer-confirmed requires the originating current session plus positive>0 and negative=0. Local-only requires `LocalVerified`. Closure is terminal for feedback/recheck/approval.

## Rehearsal scenarios

Private strict routes: `/api/v1/rehearsal/scenario`, `/report`, `/feedback`, all rejected in live mode and protected by the existing private boundary. Scenarios are `track_missing`, `mic_muted`, `wrong_scene`, `unknown_write`. Selection stops/invalidates prior simulation and writes settings through the normal revision authority; it does not automatically start monitoring or approve. The report route uses the shared coordinator with generated IDs and the one simulated viewer. `track_missing` produces a real proposal and one simulated write/read-back; `unknown_write` returns unknown and does not auto-retry; `mic_muted` is manual-only; `wrong_scene` is an insufficient/limitation case.

## Verification

- `go test ./...`, `go vet ./...`, `gofmt`, CGo-free build (`0.5.0-p05`).
- Existing five Node fixtures plus `p05_overlay_rehearsal.js`; Node is not rendered-browser evidence.
- Rendered local Chrome evidence: `.tools/p05-rehearsal-1280.png` and `.tools/p05-rehearsal-390.png` from the actual rehearsal binary (desktop/mobile monitor state). The 390px capture initially exposed horizontal overflow; CSS was corrected and the screenshot recaptured.
- WSL race suite.

## P05-R01 corrections

- Mode ownership is inspected from `schema_migrations`/`db_meta` before migration. Any existing unmarked schema-1/3 database is rejected for rehearsal without changing its schema; marked rehearsal restart and normal live upgrade remain supported.
- Closure now invalidates proposals' serialized JSON and status in the same transaction, and coordinator caches are updated from the committed outcome without a second correctness transaction.
- Simulated repair validates the actual fixture map and selected track, computes the exact desired change, honors cancellation before admission/write, performs only the requested mutation and returns observed readback. The unknown-write path reports uncertain transmission without replay. Failed scenario persistence no longer mutates the provider; scenario transitions guard active diagnostics/actions.
- Rehearsal report returns the exact current-session/category incident; feedback normalizes `stillbroken` to `still_broken`, replaces the single viewer's prior vote, and rejects terminal incidents.
- Private UI opens Setup when unconfigured, remembers the user's section, preserves stored close selections and restored focus IDs, labels closure selects, requires an explicit closure outcome, shows persisted closure outcomes, marks simulated providers as simulated, and adds a persistent Monitor status strip. Advanced identity fields are collapsed.
- Overlay adds applying, unknown/reconciled, local-verified/manual-remaining and disconnected behavior while retaining the watermark and public allowlist.

Known P05-R01 limits: no full private HTTP rehearsal 202/error flow test, no complete production-protocol separate-viewer feedback/closure journey, no deterministic blocked-scenario/approval/reconciliation interleaving test, and no completed 390px/200%-zoom/keyboard/contrast rendered audit. The actual 390px Chrome capture still clips at the right edge and is not claimed as passing.



## P05-R02 corrections

- Feedback now requires the current active session and an eligible `awaiting_feedback`/`unresolved` incident before calling the coordinator; rejected targets return non-success and never report a recorded vote. `stillbroken` is normalized to the coordinator's `still_broken`.
- Rehearsal provider generation is owned by the fixture and advances on each scenario transition; observations, readiness, proposals and readback use that generation. Reads/repair validate fixture source/context/generation/revision and return fixture-owned observed state. Direct visual-item identity is present.
- Scenario application sets/restores the provider around settings persistence, rejects active diagnostics/pending actions, and publishes settings with the matching provider observation/revision.
- Twitch command parsing now accepts the hyphenated public incident codes used by CueMender (`CM-XXXXXX`); the production OAuth/Helix/EventSub/OBS fixture now covers separate-viewer feedback, closure idempotency/conflict and public projection.
- Private HTTP rehearsal flow covers scenario 200, report 202, feedback 202/replacement, close 200, post-close feedback 409 and wrong-scene video report.
- UI report category follows the selected scenario (wrong_scene→video), report responses select the accepted incident, feedback targets the active-session eligible incident, and keyed render signatures preserve unchanged incident controls; closure selections and focus IDs remain. Overlay EventSource error now unhides a disconnected panel.
- OpenMode distinguishes absent metadata tables/rows from query failures.

Remaining P05-R02 limits: no deterministic barrier test for scenario versus blocked diagnosis/approval/reconciliation; scenario admission is guarded but not implemented as a single coordinator-owned transaction; no full 390/200%-zoom/keyboard/contrast acceptance (the 390 capture still clips); no measured contrast. Live Twitch/OBS approval remain unmet.



## Limits

- Live Twitch, real private-UI OBS approval trial, V14 real journey and final V18/V19 remain unmet under D34/issues #6/#10.
- Private HTTP rehearsal 202/error/closure flow is covered. No deterministic scenario-versus-blocked-diagnosis/approval/reconciliation test; admission is guarded but not one coordinator-owned transaction.
- No full keyboard/screen-reader/200%-zoom audit or measured contrast. The actual 390px Chrome capture still clips at the right edge; desktop Setup/Monitor and overlay captures exist, but mobile no-overflow is not verified.
- No new dependency/SDK/credential changes. No real OBS contact, login, authorization, chat or recording.
