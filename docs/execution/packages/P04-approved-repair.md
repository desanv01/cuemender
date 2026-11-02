# P04 — One approved correction

Revision: 1.3 • RELEASED — CODEX, see STATUS for sole writer
Implementer: Codex in this main chat with user-requested GPT-6.1 Sol high subagents; one production writer at a time, no DeepSeek dispatch.
Dispatch requires: P03 accepted, exact goobs track-update method and serialization behavior verified, explicit STATUS release.
This is the most correctness-sensitive packet. Narrow scope is intentional.

## Outcome and ownership

A broadcaster can approve ADD_MIC_TO_TRACK for the exact current finding. The executor rereads, records intent, writes once, reads back, and honestly reconciles unknown outcomes.

Own internal/actions, the single OBS write method, action migration/repository, proposal/control UI and focused action tests. No automatic unmute/reveal, undo, raw OBS RPC endpoint or generalized automation.

This packet gives Codex a concrete production implementation role. Do not pre-implement it in P01–P03 or substitute cosmetic edits for the work. Keep the same technical acceptance criteria and one-writer discipline; no eligibility guarantee follows merely from assigning the packet.

## Tasks

### P04.0 — Capture actual Codex build footage before implementation

User requested that Codex help operate the screen recording of this main chat when P04 begins. Nothing is to be recorded during earlier preparation.

1. Before production edits, remind the user of the recording checkpoint and inspect the available recorder/control capabilities.
2. Prepare this Codex window at a readable size; keep credentials, private launch links and unrelated content off-screen.
3. Make a short test capture and verify the saved file's picture/audio. If tool control is unavailable or a user interaction is required, explain the exact step before starting the build footage.
4. Record genuine P04 implementation and test moments. Follow the capture guidance in [SUBMISSION.md](../SUBMISSION.md); no staged reconstruction of work that was not recorded.
5. Stop capture and verify the saved footage. Record its local path and useful timestamps in P04-codex-build.md; keep raw recordings outside Git/release artifacts.
6. Select/edit the short excerpts during final video preparation, not during the implementation itself.

If capture cannot be established, tell the user before proceeding past the moments intended for recording and agree on a practical capture alternative. Do not silently skip the checkpoint or claim footage exists.

### P04.1 — Proposal and approval

- Generate proposal from a fresh track-missing finding with C06 binding fields/60-second expiry.
- Show exact selected source/track, other assignments preserved and remaining findings.
- HTTP approval accepts only ID/revision; verify session+CSRF, monitoring/session/mode/generation/revision/expiry.
- Reserve atomically; two concurrent clicks resolve to one action.
- Reconfiguration, reconnect, monitoring stop and dismissal invalidate unused proposals.

### P04.2 — Preflight, intent and write

- Execute outside coordinator while returning tagged results.
- Freshly resolve source and full six-track map. Abort on stale identity/generation or any precondition change.
- Recheck proposal/config/monitoring after I/O before intent and before dispatch. A canceled/unreserved action must not write.
- Compute only selected-track addition, preserving other observed assignments.
- Commit intent transaction before the OBS call; DB failure yields zero writes.
- Invoke only the verified SDK SetInputAudioTracks equivalent. Inspect its serialization: false values and all assignments must mean what the protocol requires.
- Read back exact map and record verification. Do not claim end-to-end audio recovery.

### P04.3 — Unknown outcomes and restart

- Differentiate definitely-not-dispatched validation failure from an operation possibly transmitted.
- Write timeout, lost socket or process crash after intent is potentially successful; do not automatically retry.
- Reconcile via read only when the correct source/context is available.
- Matching desired map supports "current configuration matches"; after a timeout it does not prove which actor changed it.
- Mismatch remains unresolved; absent reads remain unknown. New attempt requires a new proposal/user approval.
- Startup marks unfinished intents unknown and invalidates all approvals; never autoexecutes saved actions.
- Handle monitoring stop during a write without erasing its audit or sending a second mutation.
- Keep local findings that were not fixed (e.g., mute) visible.

### P04.4 — Actual OBS trial

Use a dedicated mic/test input and user-confirmed stream track. Intentionally arrange a missing-track state through the agreed test setup, approve through the private UI, and inspect all six assignments before/after. No unrelated scene/source modification. Record exact OBS/SDK versions and results without private credentials.

OBS writes have no guaranteed atomic compare-and-set against other clients. Preserve the immediate preflight assignments and expose the external-race limitation. Don't promise a test can eliminate that protocol limitation.

## Required tests

- Correct track enabled, all other true/false assignments preserved.
- Stale/expired/wrong-ID/wrong-mode/wrong-session approval => zero writes.
- Changed expectation, changed map, renamed source, new connection generation => zero writes.
- Double approval or repeated request => one durable action and at most one write.
- Persistence failure before intent => zero writes.
- Write succeeds but response times out => one write, unknown/reconciled, no repeat.
- Read-back mismatch => unresolved, no automatic rollback.
- Crash/restart after intent but before recorded completion => read-only reconciliation.
- Delayed response from old OBS generation cannot mark current action verified.
- Concurrent stop/reconfigure/approve cannot bypass reservation/preconditions.

## Acceptance

P04-A / V11: One real OBS correction matches read-back and preserves the other observed tracks.
P04-B / V12: Rejected approvals and stale cases never reach the mutation adapter.
P04-C / V13: Unknown/restart paths never blindly replay.
P04-D: Public/private text separates local verification, remaining findings and viewer evidence.
P04-E: No alternate route or adapter call bypasses the approval boundary.

Codex writes handoffs/P04-codex-build.md with implemented paths, tests, exact commands, live-vs-fixture evidence, actual session reference when available, and a small real-run reproduction. Codex then records acceptance evidence in handoffs/P04-review.md, checking every path capable of issuing OBS writes before releasing P05. This is verification of Codex's own implementation, not an independent code review. Required real checks remain required; absent evidence is not filled in for submission.

## P04 release clarification — D32 (2026-11-02)

Recording test is verified with FFmpeg Windows Graphics Capture, 1920x1034 at 15 fps, silent. Actual build recording begins before production edits. SDK SetInputAudioTracks accepts a nonnil map and preserves false values; convert track1..track6 to numeric wire keys 1..6. Require exact complete preflight maps and fresh identity/context. Use a narrow mutation capability, not a raw RPC route.

Keep C06 active monitoring, mode/session, revision/generation and expiry checks. With live Twitch deferred, a complete real private-UI approval trial may remain blocked by readiness; do not weaken product gates to manufacture live evidence. Local protocol fixtures remain independent work. Record any real OBS adapter trial separately from full-product/live-Twitch acceptance.

Implementation ordering: narrow OBS adapter and protocol checks first; then action persistence/engine/coordinator/app/UI integration; then focused independent peer inspection and root self-verification. Exactly one writer. Root owns contract decisions, Git and acceptance. Shared source mutations during diagnostics, dismissal, stop, settings and provider replacement must participate in a common authority boundary for reservation and final dispatch; never hold a coordinator/state mutex across provider network waits. Persist intent before dispatch; once dispatch is claimed, treat interruption conservatively as potentially sent and never automatically replay. An action remains auditable after monitoring stop.
