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

## Limits

- Live Twitch, real private-UI OBS approval trial, V14 real journey and final V18/V19 remain unmet under D34/issues #6/#10.
- Rehearsal HTTP routes are covered by app-level isolation tests and live-mode 409 route tests; full rehearsal 202 route/UI flow has not been rendered end to end.
- No full keyboard/screen-reader/200%-zoom audit or measured contrast; screenshots cover desktop/mobile monitor state only.
- No new dependency/SDK/credential changes. No real OBS contact, login, authorization, chat or recording.
