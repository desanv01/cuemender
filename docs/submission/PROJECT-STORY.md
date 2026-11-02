## Inspiration

An audio problem often reaches a streamer as a burst of chat messages while they are already presenting. “No sound” is useful feedback, but it does not explain whether OBS has a muted input, a missing track assignment, or a different problem. CueMender gives that report a small, inspectable workflow: collect it, check the relevant local configuration, show the evidence, and keep human judgment in control.

## What it does

CueMender is a local Windows application with private browser controls and a separate public overlay. It checks three specific conditions: the selected microphone is muted, the microphone is absent from the confirmed streaming track, or a configured direct scene item is hidden. Unknown, stale, disconnected, or mismatched evidence stays unknown rather than becoming a confident diagnosis.

The demonstrated receive-only mode receives real Twitch chat anonymously. A separate viewer sends `!issueaudio`; CueMender performs a fresh read of the operator's actual OBS and creates an incident with evidence and manual guidance. Repeated reports are grouped, commands are bounded and deduplicated, and the public overlay exposes only redacted status. This mode cannot send chat replies, prove channel ownership through OAuth, or approve changes to OBS.

The authenticated implementation includes one narrowly scoped repair: add the selected microphone to the confirmed track after a private, expiring approval and fresh checks, preserving all other track assignments. An isolated rehearsal lets judges exercise that approval, simulated write/read-back, feedback and closure without credentials or a live stream. It never connects to real providers. CueMender never automatically unmutes an input, reveals a source, or starts/stops streaming.

## How I built it with Codex

The backend is Go, with SQLite persistence, an embedded HTML/CSS/vanilla JavaScript interface, local HTTP/SSE, OBS WebSocket integration, an authenticated Twitch EventSub path, and an anonymous receive-only IRC adapter. No runtime language-model service is required.

Codex owned the architecture, shared contracts, reviews and release coordination, and substantively implemented the private approval/repair engine, receive-only integration, integration corrections, debugging and verification. DeepSeek implemented the other worker packets and corrections against those contracts. Genuine Codex development footage is included in the video. Verification of Codex-authored code is documented as self-verification; it is not presented as independent review.

The main Codex build session is `01a191ff-ada4-7970-bf35-ae0a76942bed`.

## Challenges and lessons

The hard parts were lifecycle ownership and evidence validity: a settings change, disconnect or provider replacement must invalidate old work; interrupted writes must not be blindly replayed; and local configuration verification must remain separate from a viewer saying the broadcast sounds fixed. These boundaries are covered with application/protocol fixtures, durable-state tests, actual-script checks and a Linux race suite.

Twitch phone/2FA problems blocked developer-app registration. I preserved the authenticated implementation and added an explicitly limited receive-only path instead of pretending OAuth had succeeded. That path was verified with my main channel and a separate viewer account.

## What the demo actually verifies

The real segment uses an **offline Twitch channel** and **actual local OBS in read-only mode**. For a controlled test, I temporarily muted Mic/Aux, sent a real viewer command, observed the `MIC_MUTED` finding, and restored the original mute state. The public overlay shows that same incident. This is real chat-to-diagnostic evidence, not proof of broadcast audio recovery.

The repair segment is clearly labeled **REHEARSAL — SIMULATED OBS REPAIR**. Authenticated EventSub verification and the real private-approval OBS repair trial remain pending in GitHub issues #6 and #10. Rehearsal and protocol fixtures do not substitute for those gates.

## Try it

Download the public [Windows v1.0.0 preview](https://github.com/desanv01/cuemender/releases/tag/v1.0.0), extract it, and follow the included quickstart. Rehearsal works without Go, Node, Twitch credentials or OBS. The repository includes source, setup/build instructions, MIT licensing, dependency notices and checksums. The ZIP was tested after fresh extraction on the development machine; it is unsigned and has not been validated on a separate clean VM.

Source: https://github.com/desanv01/cuemender

## What's next

Complete authenticated Twitch and real approved-repair verification once account setup is available, then gather streamer usability feedback. Keep the product focused on evidence and narrowly authorized actions rather than adding broad automatic control.
