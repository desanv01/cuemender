# Verification matrix

Revision: 1.1 • Initial status of every row: NOT RUN.

Current sequencing exception (D30): V06 and P03-D real-chat integration are deferred/unmet in issue #6 while the user's Twitch 2FA is blocked. P03 local implementation may proceed; neither these gates nor V18's full live workflow are waived. See STATUS and actual reports for current evidence rather than the original initial-status label.

For DeepSeek packets, the worker fills its report and Codex reviews the implementation. For Codex-owned P04 and direct Codex fixes, record build evidence and self-verification explicitly; do not call it independent review. Mocked tests, local browser checks and actual Twitch/OBS checks are separate evidence classes.

| ID | Packet | Required evidence | Product coverage |
|---|---|---|---|
| V01 | P01 | Clean build, embedded pages when binary runs from a different working directory | M01 |
| V02 | P01 | Defaults, invalid config, SQLite revision conflict and restart persistence | M04 foundation |
| V03 | P01 | Private session/CSRF/Host/Origin rejection; public projection leak test | M11 foundation |
| V04 | P01 | Keyring abstraction unavailable vs not-found; explicit memory-only behavior | Secret boundary |
| V05 | P01 | SSE initial/update/cancel and shutdown without stuck process | M01/M11 |
| V06 | P02 | Actual own-channel chat delivery with correct identity and authorized scopes | M02 |
| V07 | P02 | Actual OBS discovery/read matches dedicated fixture configuration | M03 |
| V08 | P02 | Fake protocol reconnect/revocation/keepalive/refresh handling; real reconnect smoke if available | M12 |
| V09 | P03 | Three pure checks with absent/stale/wrong-scene/partial-read cases | M07 |
| V10 | P03 | Duplicate/cooldown/grouping/session boundary and persistent restart | M05/M06 |
| V11 | P04 | Exact write; other tracks preserved; real OBS read-back | M08/M09 |
| V12 | P04 | Expired/revised/duplicate approval, changed map, wrong generation, persistence failure => no mutation | M08/M12 |
| V13 | P04 | Timeout-after-write, unreadable outcome, restart after intent => no blind replay | M09/M12 |
| V14 | P05 | Setup -> report -> evidence -> approval -> local result -> viewer feedback in browser | M04/M10/M11 |
| V15 | P05 | Rehearsal's adapters/credentials/data cannot issue live effects | M13 |
| V16 | P06 | New Windows release folder, paths with spaces, no Go/Node required at runtime | M01/M14 |
| V17 | P06 | Concurrent-instance guard and clean shutdown/restart | M12 |
| V18 | P06 | Full real Twitch + OBS run and appropriately configured race check | M02–M12 |
| V19 | Final | Actual <3-minute demo, repo instructions, contribution evidence, submission review | M14 |

## Review scope by risk

P01: inspect actual middleware order, cookie/bootstrap behavior, settings authority, persistence errors and public serialization. Don't accept a leak test that checks only a handpicked example while handlers serialize a private struct.

P02: inspect token subject/scopes, secret redaction, refresh synchronization, old-connection events and command trust boundary.

P03: inspect unknown vs false, time/generation checks, repeated events and distinct viewer handling.

P04: inspect all mutation routes and adapter writes, not only action executor tests. No hidden administrative route may bypass approvals.

P05: exercise actual browser controls and overlay states, including disconnected/rehearsal labels; keyboard use and sensible focus.

P06: test the shipped artifact and actual run instructions. Don't assume source test success implies a runnable ZIP.

## Reporting conventions

Each command records working directory, command, exit code/result and relevant summary. Include failure output only after redacting secrets. Skipped means skipped, with cause and owner for follow-up. Live setup problems are reported without blocking independent fixture checks, but a required live acceptance gate is not marked passed.

For Go race testing, use a supported runner/toolchain with the necessary C compiler. Race instrumentation may require CGo even though the release build does not; report this distinction. Do not install an unrelated compiler merely to remove a skipped label under time pressure.
