# P03 — Evidence and incident lifecycle

Revision: 1.1 • PLANNED / HELD
Dispatch requires: P02 accepted, actual connection types/fixtures reviewed, explicit STATUS release.

## Outcome and scope

Real reports create bounded, persistent incidents. The three supported checks compare explicit expectations with fresh observations and explain limits. Manual checks work without chat. No OBS writes.

Own internal/diagnostics, internal/incidents, schema migrations for reports/findings/incidents/dedup, wiring, minimal evidence controls and tests. Use C01/C04/C05/C07; do not design a new event system.

## Tasks and subtasks

### P03.1 — Pure diagnostics

- Implement independent mic-muted, track-missing and direct-scene-item-hidden checks with typed evidence.
- Preserve unknown values; don't turn failed reads into false/disabled booleans.
- Check age, connection generation and configuration revision.
- Visual finding only in configured current program scene; identify unsupported nested situations explicitly.
- Return multiple findings when appropriate; a track fix cannot erase a mute finding.
- Table-driven fixtures cover positive, negative, missing expectation, stale, wrong scene, source missing, partial response and reconfigured cases.

### P03.2 — Coordinator and intake

- Add explicit monitoring start/stop and unique session IDs. Starting verifies needed setup and reports readiness truthfully.
- Normalize accepted viewer reports into one active incident per category. Aggregate counts without unbounded network requests.
- Apply delivery/message dedup, per-actor cooldown and distinct-reporter counts.
- Use bounded diagnostic jobs; slow I/O returns tagged results, not shared-state mutations.
- Manual check records origin=manual and actual online/offline state.
- Stop monitoring rejects new report work/approvals, invalidates pending work and ignores late old-session results.

### P03.3 — Persistence and projections

- Migrate prior P01 SQLite schema without destroying saved settings.
- Persist incident lifecycle, minimal report metadata and findings atomically where related.
- Enforce unique IDs/dedup across restart within the documented retention policy.
- Expose typed evidence privately; public projection uses fixed stage messages.
- Add dismissed/no-supported-finding/insufficient-information/interrupted outcomes and explicit recheck.
- On restart, preserve history, close/interrupt old session work and require fresh monitoring.
- Add feedback intake state semantics now; finished confirmation UI is P05.

### P03.4 — Functional controls

- Show report category, distinct reporters, evidence time, declared expectation and current finding.
- Manual-only findings offer instructions, never action buttons implying unmute/reveal.
- Track mismatch can display that repair is not implemented until P04; don't issue executable proposals yet.
- Confirm no username/source leakage on overlay.

## Acceptance

- P03-A / V09: All three rule families pass positive and limiting-evidence cases.
- P03-B / V10: Repeat delivery does not inflate counts; cooldown and category grouping behave as specified.
- P03-C: Late results after stop/restart/reconfiguration cannot update current incidents.
- P03-D: Real chat drives a real OBS-backed finding; persistence survives restart.
- P03-E: A fresh no-conflict observation reports no supported conflict, not "stream is fixed."
- P03-F: New migrations preserve P01 settings and tests cover failure boundaries.

Report actual commands plus representative input/expected/observed outcomes. Stop before adding any repair implementation. After P03 acceptance, Codex takes implementation ownership of P04; DeepSeek does not dispatch itself into P04.
