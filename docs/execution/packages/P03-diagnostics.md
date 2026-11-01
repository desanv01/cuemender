# P03 — Evidence and incident lifecycle

Revision: 1.2 • RELEASED under D30; check STATUS before claiming ownership.
Baseline: P02 source checkpoint 1b068843e38bee905ff6ea80c44f612e88ed32d4, integrated through PR #5. Worker branch: feature/p03-diagnostics. Contracts: v1.3 including C09. Tracking: https://github.com/desanv01/cuemender/issues/7.

P02 local implementation and live OBS verification are complete. User explicitly authorized this phase while Twitch 2FA is unresolved. P02-A/V06 and P03-D's real-chat portion remain deferred/unmet in https://github.com/desanv01/cuemender/issues/6. Do not resume Twitch login/registration/authorization or claim fixtures as live evidence. This prerequisite does not block the tasks below. P04 remains held and Codex-owned.

## Outcome and scope

Real reports create bounded, persistent incidents. The three supported checks compare explicit expectations with fresh observations and explain limits. Manual checks work without chat. No OBS writes.

Own internal/diagnostics, internal/incidents, schema migrations for reports/findings/incidents/dedup, wiring, minimal evidence controls and tests. Use C01/C04/C05/C07; do not design a new event system.

## P03.0 — Preflight and actual integration points

1. Read AGENTS, STATUS, WORKFLOW, PROJECT_SPEC, ARCHITECTURE, CONTRACTS v1.3 (especially C09), DECISIONS D30, the final P02-codex-closure-review and this packet. Earlier P02 review holds/correction prompts are historical, superseded by D30 only as described there.
2. Read Git status/diff and branch; require feature/p03-diagnostics and no active writer. Record actual starting HEAD and existing changes. Claim operational STATUS fields as DeepSeek:P03/in_progress. Do not perform Git mutations or spawn workers.
3. Inspect the real source before implementation: internal/domain/{settings,integrations,projections,errors}.go; internal/app/{app,integrations}.go; internal/store/store.go; internal/httpui/server.go; internal/obs/manager.go; internal/twitch/{commands,manager,replies}.go where present; web/control and web/overlay. Use rg to locate exact files instead of assuming a scaffold.
4. P02 exposes twitch.Options.OnCommand(CommandEvent), obs.Manager.SnapshotBundle(settings), RefreshDiscovery(ctx), and private readCycle/readObservation. SnapshotBundle is cached; RefreshDiscovery refreshes discovery only. Add a narrow read-only on-demand diagnostic seam that captures immutable settings/revision/generation, performs fresh discovery/observations and revalidates ownership. Reuse P02 admission/cancellation/transport ownership. Do not expose the vendor client or mutate OBS.
5. P02 handleChat currently sends static replyAck/replyHelp after OnCommand. Adapt this seam under C09 so actual incident acknowledgments are not duplicated or contradicted by those idle replies. Keep the existing bounded ReplyQueue and lifecycle protections.
6. Store schema is currently version 1. Add a forward transactional migration; do not alter migration 1 or reset databases. Update tests expecting the latest version while retaining a real version-1 upgrade fixture. Preserve settings revision and legacy storage identifiers.
7. Use existing pinned dependencies and patched third_party/goobs unchanged. No new modules/services, dependency upgrades or broad SDK edits. Changes needed outside this scope must be reported with evidence to Codex.

Allowed application changes: the two new internal packages, consumed domain/store types, app/provider seams, authenticated HTTP endpoints, small existing page controls and their tests. No actions/rehearsal package, approval/repair endpoint, clear-history endpoint or P05 redesign.

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
- P03-D local: Exercise normalized Twitch intake through the real application wiring using local protocol fixtures, then a fresh OBS read and persisted finding; reopen the real temporary SQLite DB and verify history/settings/dedup and stopped monitoring. P03-D live: real Twitch chat drives a real OBS-backed finding — explicitly deferred under D30/issue #6, not passed by the local result. A manual live OBS check may be recorded separately if available without changing the user's OBS configuration.
- P03-E: A fresh no-conflict observation reports no supported conflict, not "stream is fixed."
- P03-F: New migrations preserve P01 settings and tests cover failure boundaries.

Report actual commands plus representative input/expected/observed outcomes. Stop before adding any repair implementation. After P03 acceptance, Codex takes implementation ownership of P04; DeepSeek does not dispatch itself into P04.

## Required failure and boundary evidence

Implement these as ordinary tests alongside the code, not only review-tag fixtures. Use deterministic barriers and injected time rather than timing guesses.

| Area | Required observations |
|---|---|
| Pure checks | All three positive/negative cases, two simultaneous audio conflicts, incomplete six-track map, nil booleans, stale/future-invalid timestamps, wrong revision/generation/context, renamed/missing source, wrong program scene/UUID, unsupported nested item. Partial evidence never becomes a false healthy result. |
| Intake | Same delivery with changed message ID and same message with new delivery count once; retained duplicate after reopen counts zero; cooldown boundary; own-account/idle rejection; concurrent reporters group into one category incident; manual checks add no viewer count; bounded detail/queue behavior. |
| Ownership | A blocked read released after stop, new session, settings save, disconnect/reconnect or dismissal cannot overwrite state. Explicit recheck has a new request identity; an older result cannot win. Whole-app shutdown with active SSE and a blocked diagnostic joins work, releases listener and closes DB. |
| Durable state | Version-1 settings survive migration; failed report/dedup/finding transaction leaves DB/cache/private/public state consistent and emits no success reply. Restart interrupts open work and does not auto-monitor, recheck or replay. Retention keeps active records and enforces completed/dedup bounds. |
| Feedback | Latest vote per actor replaces prior counts; duplicates inert; positive feedback does not close; negative becomes unresolved; unknown/terminal/old-session/manual-offline codes rejected. |
| Routes/UI | Authenticate and CSRF-check each new mutation; malformed/unknown fields/revisions/invalid category/IDs/busy states return contract errors. Actual control script covers start, stop, manual check, recheck, dismiss, reload/CSRF and SSE refresh without resetting selectors. Public serialized JSON/SSE excludes private identifiers/expectations/evidence/raw errors. |
| Provider boundary | On-demand reads actually hit the read-only protocol fixture; deadline covers admission and blocked reads; no duplicate static acknowledgment; replies are fixed/generated-code-only and generation/session bound. OBS outbound allowlist remains read-only; P04 endpoint remains 404. |

## Execution and verification sequence

Build incrementally: pure typed diagnostics/tests; transactional store migration/tests; coordinator/session/job ownership; fresh-read and Twitch reply seams; HTTP/private/public wiring; minimal controls and actual-script tests; integrated boundary checks and documentation. Run focused tests after each meaningful piece, then one final integrated suite. Preserve existing P02 behavioral assertions; change expected shapes only where C09 deliberately adds behavior.

Portable Windows toolchain (already installed; do not reinstall):

~~~powershell
$env:GOROOT = "$PWD\.tools\go1.27.3\go"
$env:GOPATH = "$PWD\.tools\gopath"
$env:GOMODCACHE = "$env:GOPATH\pkg\mod"
$env:GOCACHE = "$PWD\.tools\gocache"
$env:GOTMPDIR = "$PWD\.tools\gotmp"
$env:CGO_ENABLED = '0'
$env:GOTOOLCHAIN = 'local'
$env:GOPROXY = 'off'
& "$env:GOROOT\bin\go.exe" version
& "$env:GOROOT\bin\go.exe" test ./... -count=1 -timeout=120s
& "$env:GOROOT\bin\go.exe" vet ./...
& "$env:GOROOT\bin\go.exe" build -o dist/cuemender.exe ./cmd/app
~~~

Check gofmt on all changed Go files. Run both existing actual-script fixtures (locate paths with rg --files) and the new P03 actual-script coverage. Run the lifecycle tests affected by coordinator integration repeatedly. Windows production build stays CGo-free. Record exact versions, commands, exit results and binary hash/size. Do not copy older P02 pass claims as if freshly run.

Existing WSL/GCC and workspace Linux Go support race testing; local Windows gcc absence is no longer the full environment finding. If your harness can access WSL, run the following; otherwise explicitly hand the race check to Codex, without installing software or pretending it passed:

~~~powershell
wsl -d Ubuntu --cd '/mnt/c/Users/Dv/Desktop/Twitch Hackathon' -- env CGO_ENABLED=1 GOTOOLCHAIN=local GOPROXY=off 'GOCACHE=/mnt/c/Users/Dv/Desktop/Twitch Hackathon/.tools/linux-gocache' 'GOTMPDIR=/mnt/c/Users/Dv/Desktop/Twitch Hackathon/.tools/linux-gotmp' 'GOPATH=/mnt/c/Users/Dv/Desktop/Twitch Hackathon/.tools/linux-gopath' 'GOMODCACHE=/mnt/c/Users/Dv/Desktop/Twitch Hackathon/.tools/gopath/pkg/mod' '/mnt/c/Users/Dv/Desktop/Twitch Hackathon/.tools/linux-go1.27.3/go/bin/go' test -race ./... -count=1 -timeout=120s
~~~

All computer access is background-only. No Twitch browser work. Browser checks may use a hidden local page if your harness supports it; actual-JS tests are not rendered-browser proof. Do not activate OBS, change sources/mute/tracks, start streaming or stop user-owned OBS. Use isolated temporary data/listen ports and session-only secrets for tests. Do not log bootstrap URLs or credentials. Do not stop unrelated processes.

## Handoff

Write docs/execution/handoffs/P03-worker-report.md using templates/WORKER_REPORT.md and docs/implementation/P03-diagnostics.md with implemented API/state/storage behavior and reproducible checks. Map P03-A through F plus the boundary table to evidence; label executed local, rendered-browser, live OBS, deferred live Twitch and unexecuted checks separately. List changed files, migration version, dependency changes (expected none), limits, failures and owned-process cleanup. Preserve historical P02 evidence.

Only operational STATUS fields may be changed by the worker. Set ready_for_review (or blocked for a specific independent implementation blocker), owner Codex, no active writer. Twitch absence alone does not make a fixture-complete P03 implementation blocked; list the deferred gates. Stop editing. Do not accept your own work, close GitHub issues or begin P04. Use the exact current P03 handoff in PROMPTS.md.
