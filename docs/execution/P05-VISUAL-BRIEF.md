# P05 visual and interaction brief

Revision 1.0 — Codex release, 2026-11-02. P05 v1.1/C11. Written implementation brief, not a generated-image approval.

## Evidence and tool limitation

Codex inspected `output/playwright/p04-approved-final.png`: actual P04 rendered control UI against local protocol fixtures. The tall column puts advanced setup before the main incident/approval task. Keep the existing charcoal/teal identity; reorganize around setup and monitoring. This ignored local screenshot is optional context, not a required distributed asset.

The 12ui skill was read/applied. Automatic review initially rejected screenshot export; the user explicitly authorized retry. Both retry and generic brief returned sponsored-budget-exhausted. No candidate/selected design/generated HTML/alignment result exists. Do not invent these or depend on absent output. No new purchase/install is needed for this packet. Codex must review actual rendered results at handoff; any later tool alignment is separately recorded.

## Layout and interaction

- Private surface: max width about 1120px, 24px desktop outer spacing/16px mobile. Compact CueMender header with textual LIVE/REHEARSAL badge, separate Twitch/OBS connection statuses, broadcast active/inactive/unknown and monitoring state. Disconnected is not healthy.
- Plain Setup/Monitor/Rehearsal sections or tabs (Rehearsal only in rehearsal mode). Initially unconfigured selects Setup; remember user's selection in the page session. SSE must not switch tabs/reopen sections. Keyboard operation, no decorative sidebar.
- Setup: short readiness checklist, then Twitch and OBS connection, microphone/explicit track confirmation, optional visual item. Advanced UUID/context fields in labeled disclosures. Preserve all existing controls/discovery behavior. Overlay URL and copy/selectable fallback live here; never substitute private launch link. Clear submitted secrets, retain nonsecret edits after recoverable errors.
- Monitor: explicit start/stop and manual checks first. At wide widths use about 2:1 evidence/approval columns; stack below 900px in sensible DOM order. Empty state gives next prerequisite/action, no invented metrics.
- Incident: code/category/stage/origin/time, findings and limitations, expandable technical revision/generation. Distinguish recorded diagnosis, action result, remaining findings, viewer positive/negative reports and closure.
- Approval: exact private source/track, preservation of other flags, expiry and external-writer race limitation. Distinct pending/expired/stale/unknown text. No auto-renewal or optimistic success. Lost HTTP response reloads state, never assumes mutation succeeded. Unknown action has read-only recovery explanation, not a second approval for the same action.
- Public overlay: transparent body, one compact 420–520px panel readable on a 1920x1080 stream. Fixed progress wording, public incident code/category and separately labeled feedback. Quiet/hidden when idle; disconnected is small/truthful. REHEARSAL watermark remains even idle. No private names, tracks, setup links, arbitrary errors or technical IDs.

## Visual and accessibility requirements

Preserve near-black background, charcoal surfaces, teal for actions, off-white text and readable muted gray. Status uses text/icon as well as color. System font stack only; body 14–16px, headings 20–24px, line height around 1.5. 8px spacing rhythm, modest corners/borders/shadows. No remote fonts/images/frameworks, Twitch branding, charts, gradients or marketing hero.

Persistent field labels, aria-describedby errors, focusable submission summary without wiping nonsecret values, visible focus ring, accessible tabs/disclosures. Disable only conflicting actions while busy; preserve focus/input and stable selectors across SSE timestamp updates. Use textContent for untrusted values and retain CSP. Reduced motion, no flashing. Measure contrast rather than claiming compliance by appearance. At 1280x800, 390x844 and 200% zoom, controls remain usable without horizontal overflow; long names/errors wrap. Keep rehearsal visibly simulated in every state.

## Handoff screens

Setup incomplete/session-only/error; connected but broadcast stopped; monitor idle/checking; simultaneous mute/track; valid/expired/pending approval; local verification with mute remaining; unknown/reconciled; conflicting feedback/closure; stale/reconnect; rehearsal idle/all scenarios; public idle/active/reconnect/watermark. Capture representative desktop/mobile states with fictional data when available. Node DOM tests are not rendered evidence. Report unavailable visual checks precisely for Codex; never mark them passed.
