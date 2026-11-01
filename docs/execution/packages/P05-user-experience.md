# P05 — Complete interactive experience

Revision: 1.0 • PLANNED / HELD
Dispatch requires: P04 accepted; Codex releases a short visual brief using actual working screens and records it in this packet/linked implementation brief.

## Outcome and scope

A new user completes setup, sees a report, understands evidence, approves the supported correction and distinguishes local verification from viewer feedback. A compact overlay makes the process watchable. Rehearsal is explicitly isolated.

Own web/control, web/overlay, narrowly needed HTTP projection handlers, internal/rehearsal and user-facing documentation. Preserve the proven backend and access boundaries.

## Tasks

### P05.1 — Design checkpoint

- Codex reviews actual existing screens before choosing layout/color/type; apply available required UI skill for Codex's design work.
- Choose a compact, readable streamer utility style, clear focus/contrast, restrained motion and quiet idle overlay.
- Keep LiveCheck working name unless user finalizes branding; no unofficial Twitch affiliation/branding claims.
- Worker implements the released brief. If its harness lacks a referenced skill, report that fact and use the actual brief; don't invent skill execution.

### P05.2 — Setup and readiness

- Guide through Twitch authorization, OBS connection, mic, explicit track confirmation, optional direct visual item and overlay URL.
- Show actual Twitch/OBS/broadcasting/credential state independently.
- Explain session-only secret choice and reauthorization requirements.
- Validate fields accessibly and keep entered nonsecret preferences on recoverable errors.
- Monitoring start/stop is explicit, with remaining prerequisites listed.

### P05.3 — Private incidents and overlay

- Evidence: expectation, observation, timestamp, limitations and other findings.
- Approval: exact source/track, expiry, changed-state warning and pending state; prevent repeated-click confusion while backend remains authoritative.
- Overlay: concise progress and public incident code; hidden/quiet when idle, readable at streaming scale.
- Render live values as text; no secret values or private OBS names in public output.
- Reconnects/unknown outcomes get honest copy, never a green success by timeout.
- Keyboard navigation, labeled controls, error focus and reduced motion.

### P05.4 — Feedback

- Wire !fixed and !stillbroken per C04/C05, replacing a viewer's prior feedback rather than accumulating unlimited votes.
- Show positive and unresolved counts separately.
- Permit appropriate explicit broadcaster closure with supported outcome.
- Unknown/closed/old-session codes do not reopen incidents; replies remain rate-limited.
- Demonstrate from a separate viewer identity since own messages are ignored.

### P05.5 — Rehearsal

- Implement --mode rehearsal with separate data path, keyring namespace and adapter construction.
- Do not load live credentials or instantiate live Twitch sender/OBS writer.
- Fixtures: missing track then corrected, muted mic/manual finding, wrong-scene/insufficient information and unknown-write explanation.
- Watermark private/public surfaces and generated reports as REHEARSAL.
- Fake feedback may illustrate flow but is never recorded as real Twitch evidence.
- A requested switch to live requires restart and explicit live setup.

## Acceptance

- P05-A / V14: Browser journey works from setup to feedback; actual Twitch/OBS behavior retained.
- P05-B: Keyboard/contrast/error/reconnect states usable; no giant idle overlay.
- P05-C / V15: Tests prove rehearsal cannot call live network mutation/sender adapters.
- P05-D: Viewers can distinguish reports, checking, local change and feedback.
- P05-E: Existing session/access/approval tests remain valid.

Handoff includes screenshots of representative states if the harness can capture them without exposing secrets; otherwise precise steps for Codex browser review. Visual approval is independent of source compilation.
