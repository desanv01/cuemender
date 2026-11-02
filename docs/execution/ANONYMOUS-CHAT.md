# Receive-only Twitch fallback — D37 / C12

Revision 1.0. Codex owns implementation and self-verification. DeepSeek stopped.

## Probe evidence

2026-11-02 11:05–11:12 UTC: standalone Go TLS IRC probe joined the user-confirmed main channel. The separate user-confirmed viewer sent two distinct harmless test messages, one before and one after a deliberately closed/reopened connection. Both arrived with sender ID, room ID, message ID and server timestamp; no shared-chat origin. Both capability acknowledgments and channel joins succeeded. Probe exited zero and closed both sockets. No OAuth credential, chat send, OBS change or broadcast was used. This establishes connectivity, not end-to-end product verification or organizer acceptance.

## C12 contract

- Opt-in immutable CLI `--receive-only-channel LOGIN`, live mode only, lowercase ASCII channel validation. A separate `receive-only/LOGIN` data directory prevents sharing active authenticated action records or settings. Existing EventSub mode is unchanged.
- Private session/CSRF protected POST `/api/v1/twitch/receive-only/connect` with `{ "channel": "LOGIN", "confirmed": true }` confirms the selected chat is intended to report on this local OBS. It must match the launch channel. No automatic startup connection. Existing disconnect route disconnects the active provider.
- IRC uses certificate-verified TLS to Twitch's fixed endpoint. Bounded lines, strict command parsing, numeric sender/room IDs, message IDs, timestamp freshness, channel/room binding, broadcaster self-exclusion and shared-chat rejection. Missing or ambiguous provenance is rejected. No raw chat logging/storage. Existing bounded queue, dedup, cooldown, monitoring session capture and generation checks apply.
- Generation advances on disconnect/replacement. Disconnect/Close cancel and join transport, setup and retry work. Retry uses bounded delay, heartbeat detects silent loss, callback publishes disconnected status before replacement admission.
- Projected provider is explicitly `anonymous_irc`, receive-only. Private `twitch_authorized` is false; separate `chat_connected` permits diagnostic monitoring only. No identity fabricated as OAuth proof. Public projection adds only safe `chat_mode=receive_only` (no channel/sender IDs).
- Structural write barrier: no authenticated Twitch manager/reply queue and no coordinator repair/reconciliation callbacks in this composition. App approval also rejects receive-only requests. No proposal is offered. Preserve read-only diagnostics/manual guidance. Feedback uses existing eligibility rules; a finding alone does not become viewer-confirmed or eligible merely because it came through IRC.
- Separate-channel offline chat + local OBS is labeled as such. It does not prove broadcast audio recovery. Issues #6/#10 remain open for their original authenticated/live gates.

## Acceptance and P06 impact

Prove parser rejection paths, dedup/cooldown, TLS lifecycle cancellation/reconnect and generation behavior with local fixtures; prove actual App report-to-persisted-diagnostic with read-only OBS protocol fixture and no repair/reply ability. Preserve prior suite, Node fixtures and supported race checks. Verify private connection confirmation and public labeling rendered in background. Record actual Twitch-to-local-OBS test separately if available.

P06 packages this optional path alongside EventSub and rehearsal after Codex verification. Quickstart must explain channel confirmation, no credentials for IRC, separate storage, manual OBS connection, no bot replies and no real repairs in receive-only mode. MIT is selected. Implementation and verification are complete in [the closure record](handoffs/anonymous-chat-review.md). STATUS now controls P06 v1.1 dispatch; no second writer.
