# P03 diagnostics and incident lifecycle

Packet: P03 v1.2 • Contracts v1.3/C09 • Baseline P02 merge `af2ec69` • Worker branch `feature/p03-diagnostics`.

## Implemented behavior

### Pure diagnostics

`internal/diagnostics.Evaluate` compares one immutable settings/revision snapshot with one fresh discovery/observation snapshot and a supplied clock. It returns typed checks and findings for:

- **MIC_MUTED** — configured microphone; known `mute=true`.
- **MIC_TRACK_MISSING** — configured microphone plus confirmed stream track; complete `track1`–`track6` map has the selected track disabled.
- **VISUAL_ITEM_HIDDEN** — configured direct scene item is in the current program scene, the scene/name and item/source identity resolve, and known visibility is false.

Missing fields, incomplete track maps, stale or future-invalid timestamps, generation/revision/context mismatch, unresolved identities, missing program scene, and nested/group visuals are `unknown`, never healthy. MIC_MUTED and MIC_TRACK_MISSING can coexist. A visual check outside the current program scene is not treated as a hidden finding. No check claims viewer playback.

### Fresh OBS evidence

`obs.Manager.DiagnosticRead(ctx, settings)` performs a distinct read-only snapshot:

- takes the manager's single diagnostic admission slot (429/busy rather than unbounded accumulation);
- captures backend/generation/revision, uses a five-second deadline, and reuses the existing cancellable/caller-owned transport;
- calls fresh discovery and observation reads against the explicit settings snapshot;
- revalidates backend, generation and revision before returning;
- returns immutable evidence only; it does not install or relabel cached state and exposes no mutation path.

`SnapshotBundle` remains cached; `RefreshDiscovery` still refreshes only discovery. The coordinator uses `DiagnosticRead`.

### Monitoring sessions and incidents

`internal/incidents.Coordinator` owns one process-local session and at most one job per category / two total. Start requires matching revision, connected Twitch/OBS, resolved expectations, at least one configured diagnostic, and explicit track confirmation when a microphone is configured. Repeat Start returns the same session. Stop invalidates and cancels jobs, marks durable session inactive, interrupts open work and rejects new report work.

Accepted chat reports use the existing `twitch.Options.OnCommand`/`CommandEvent` seam; the application suppresses P02's unconditional static idle acknowledgments and returns a coordinator reply through the existing bounded `ReplyQueue`. Reports are durably deduped by delivery/message ID, grouped into one active incident per session/category, and counted with distinct backend actor IDs. Manual checks work while stopped, have report counts of zero, and record their own operation/session identity. Feedback is latest-vote-per-actor; positive only changes counts; negative moves eligible incidents to `unresolved`; terminal incidents reject feedback.

P03 track-mismatch findings remain `finding_available` with `ADD_MIC_TO_TRACK` as descriptive metadata only. No proposal, approval, action record, button or OBS write exists. Manual-only findings enter `awaiting_feedback`. No-conflict fresh evidence becomes `no_supported_finding`; incomplete evidence becomes `insufficient_information`; late/canceled work cannot commit.

### Persistence

Migration **2** adds:

- `monitoring_sessions`
- `incidents` (typed JSON plus indexed identity/stage/count columns)
- `findings`
- `reports`
- `report_dedup` (24h, bounded keys)
- `feedback`

Migration 1 and `app_settings` are untouched. A real version-1 database is upgraded in place and keeps its revision/settings bytes. Report acceptance, dedup, grouping and count changes are one transaction. Dismissal is terminal and idempotent; restart marks nonterminal incidents `interrupted`, stops all monitoring sessions, preserves history/settings/dedup, and never auto-monitors or replays. Completed incidents are retention-pruned to 500/30 days at coordinator start.

### Routes and projections

Private routes use existing host/origin/session/CSRF/strict JSON/envelope boundaries:

- `POST /api/v1/monitoring/start` `{expected_revision}`
- `POST /api/v1/monitoring/stop` `{}`
- `POST /api/v1/incidents/check` `{category, expected_revision, incident_id?}`
- `POST /api/v1/incidents/{id}/dismiss` `{}`

Start/Stop return coherent session state; checks return 202 with incident/public ID/stage; unknown dismiss is 404; revision/setup/terminal conflicts are 409; unavailable provider is 503; diagnostic saturation is 429. All new mutations require session + CSRF + same-origin.

Private state exposes monitoring session, incidents, typed checks/findings, counts and evidence time. Public state exposes only current monitoring-session incidents with fixed public fields and no source names, private IDs, expectations or raw errors. Stopped monitoring exposes no public incidents.

## Verification

| Gate | Evidence |
|---|---|
| P03-A | `internal/diagnostics` tests: all three checks, simultaneous mute + track conflict, negative/clean, incomplete tracks, nil evidence, stale/future, revision/generation mismatch, wrong scene, missing source, nested/group. |
| P03-B | Store tests: first report, same delivery/changed message, same message/new delivery, same actor/report count, distinct reporters/grouping; coordinator duplicate acknowledgment suppression. Existing Twitch cooldown/own-account/queue tests remain passing. |
| P03-C | `TestCoordinatorStopRejectsLateBlockedRead`; generation/revision guard in `run`; dismissal cancels job; settings revision checked at commit. |
| P03-D local | `TestP03ApplicationWiringPersistsIncidentAndRestart`: app composition, fake OBS provider, normalized command callback, fresh read, persisted findings, private/public projection, stop, SQLite reopen preserving settings/history. Live Twitch remains deferred. |
| P03-E | Clean evidence returns `no_supported_finding`; no healthy claim for unknown evidence. |
| P03-F | `TestVersionOneSettingsMigrateToCurrent` preserves version-1 settings/revision; migration repeat/schema-newer tests. |
| Routes/UI | `TestP03MonitoringAndIncidentRoutes`, `TestP03RouteErrorBoundaries`, `TestP03PublicStateRedaction`; actual-script `testdata/browser/p03_incidents.js` start/stop/manual/recheck/dismiss/reload/CSRF, plus existing two fixtures. |

## P03-R01 corrections

- Microphone checks now require current discovered identity, observed identity, known/matching scene-collection context, matching settings revision/generation and evidence age. Track diagnosis additionally requires explicit `TrackConfirmedAt`; an independently supported mute result survives missing track confirmation. Missing/renamed/changed identity is unknown, not conflict or healthy.
- The visual check treats wrong program scene, missing context, unresolved source, unsupported nested item and failed reads as limitations; it never marks another active scene `ok`.
- `EvaluateFor(category)` filters checks/findings and sufficiency to the incident category. Unconfigured category evidence is insufficient, and video incidents cannot acquire audio findings.
- Incidents use C01 mode `live` with origin `viewer`/`manual` (no monitoring/manual mode values). Findings retain observation/generation/revision/context provenance.
- Coordinator mutations serialize through an operation gate; provider status/settings callbacks run outside the state mutex. Job capacity is reserved before durable scheduling; one session per concurrent start; repeat start still checks revision.
- Active manual checks join/create in the current monitoring session; stopped manual checks use a separate manual scope. Dismiss cancels only the matching incident job. Completed diagnoses coalesce joining reports until explicit recheck.
- Stop persists before changing live state; durable failure remains visible/retryable. Stop cancels offline manual jobs and joins intake/Close work. Late results require current job identity, session, settings revision, OBS generation and connection state inside the commit gate.
- Report dedup uses the actual 24-hour retained instant and prunes to exactly 10,000 keys after insertion. Feedback vote, totals and incident transition are one transaction; stale finding saves merge current report/feedback counters. Report/feedback bounds and retention/restart behavior are documented.
- Twitch intake uses a bounded queue owned by the app; EventSub callback does not wait on SQLite. Command generation and receipt time are validated against the active session/current Twitch generation before durable acceptance; replies use the existing queue with a generation check. Issue acknowledgments distinguish new diagnosis from joining an existing incident.
- Feedback is accepted only for current-session `awaiting_feedback`/`unresolved` incidents. `finding_available`, insufficient/reported/investigating, terminal/history/manual-offline feedback is ignored.
- Public service status derives from monitoring/job state (`disconnected`/`ready`/`checking`). Private cards show per-finding guidance, limitations, category/origin, evidence time, expectation revision, feedback and repair-not-available copy; recheck is hidden for terminal/history and valid rechecks remain enabled for current-session nonterminal incidents.

## P03-R02 corrections

- Closed coordinators reject Start/Check/report/feedback/dismiss before persistence.
- Job capacity is retained until the worker exits; canceled/dismissed reads no longer free a category for a second concurrent read. Stop records interrupted manual incidents and publishes truthful state; failed durable stop remains retryable.
- Manual and monitoring commits revalidate session, settings revision, OBS generation and connection state inside the serialized operation gate. Settings and provider disconnects invalidate affected jobs and mark evidence interrupted.
- App intake is a bounded cancellation-owned queue; issue/feedback validate current Twitch generation and receipt time, and replies require both the originating generation and active session.
- Dedup expiry/retention and incident ordering use SQLite Julian-day chronology, fixing variable-width RFC3339Nano fractional comparisons. Recovery interrupts all open records (not the first page). Dismiss/stop reload pruned history.
- Program-scene UUID is required when configured. Observation-stage diagnostic failure now invalidates/reconnects the OBS transport like discovery failure. Incident evidence carries actual streaming true/false/unknown.
- Public monitoring/incidents/service status use one coordinator public snapshot and provider connectivity; private card text carries actual streaming evidence.
- `TestP03ActualAppProtocolReportAndReply` is a full local acceptance path: real App, production OAuth/EventSub manager against local OAuth/Helix/WebSocket server, read-only OBS protocol server, real readiness, normalized chat report, fresh finding and actual Helix reply request. It does not inject readiness or invoke the command callback directly.



## P03-R03 corrections

- Coordinator startup now uses `NewChecked`; App.New fails and releases resources when session recovery, interruption, retention, or retained-history loading fails. Recovery loads all non-terminal/unresolved records plus the latest 500 completed history entries.
- Admission uses a context-aware channel gate instead of an unbounded mutex. Canceled commands return without waiting for the current owner; the legacy `opMu` field remains only for the preserved review fixture and is not used for production admission.
- Jobs settle interrupted records when a read is canceled/times out/fails and no result committed; job removal publishes the settled state. Manual Stop returns persistence errors, does not publish an unsaved interruption, and remains retryable.
- Reports and feedback require a connected, nonzero current Twitch generation. Sessionless `!streamhelp` is replied to while connected/current without creating monitoring state; report/feedback replies still require the originating active session.
- App.SaveSettings and provider disconnects invalidate blocked diagnostics. The new app test holds a real diagnostic read, saves settings, asserts interrupted/old evidence, then explicitly rechecks with the new revision.
- The full local protocol test now owns and joins the upgraded EventSub handler/socket, verifies the finding in SQLite, and covers idle help before monitoring. OBS requests remain read-only by construction; an explicit allowlist assertion is still not present.
- Whole-app blocked diagnostic shutdown is tested with an active job, listener release and database reopen.



## P03-R04 corrections

- Runtime reload after stop/dismiss/retention uses retained loading (all non-terminal/unresolved plus latest 500 completed). Reload errors no longer replace cache with empty history.
- Failed manual interruption is retained as pending durable work after the diagnostic exits; Stop retry settles the stored incident and refuses to overwrite newer terminal/recheck state.
- Job cleanup conditionally settles only older investigating records, retains failed saves as pending, and publishes the settled job count after both success and failure.
- Close cancels admission, waits for the current operation gate holder, then marks closed; no admitted Start/Check/report/feedback/dismiss commits after Close returns. Operation gate is context-aware.
- Intake captures the monitoring session ID; queued report/feedback is dropped if that exact session is no longer active; replies require the same captured session plus current generation.
- Result persistence uses a bounded owned timeout rather than an unbounded Background call.
- The blocked-diagnostic shutdown test now runs the real HTTP server with active SSE before shutdown, then asserts joined shutdown, listener release and database reopen.
- Settings invalidation/recheck remains covered; the protocol fixture asserts all observed OBS request types are in the read-only allowlist.



## Limits

- **P03-D live Twitch chat-to-finding is deferred/unmet under D30/issue #6**: no real Twitch Public client/authorization/chat was attempted. Local normalized command callback and fixtures are not live evidence.
- **P02-A/V06 remains deferred/unmet.** P02-B/V07 live OBS was previously verified by Codex; P03 did not repeat a live OBS comparison or alter the user's OBS.
- The full local protocol acceptance test exercises production readiness/EventSub/OBS/reply wiring with local servers. A separate lower-level composition test still injects readiness; it is not the acceptance path.
- Runtime retained reload, recoverable manual-stop retry and close/late-start boundaries are now covered. Automatic OBS disconnect/replacement, queued-report-across-stop/start, stale-feedback-after-replacement and close-while-database-admission-is-blocked lack dedicated named tests.
- No rendered-browser check was possible; actual-script Node VM fixtures are distinct. No OBS mutations.
- The final WSL Linux race suite passed.
- No new dependencies or SDK changes. Migration 2 is forward-only; no downgrade path.
- Feedback transition UI is minimal; P05 owns final incident experience. P04 repair/approval code is absent.
