# P02 — Real Twitch and OBS connections

Revision: 1.0 • PLANNED / HELD
Dispatch requires: Codex acceptance of P01 and explicit release in STATUS.
Before dispatch, Codex adds accepted baseline/toolchain and verifies route/type adjustments against actual P01 code.

## Outcome and scope

A broadcaster authorizes their own Twitch channel; real chat commands are normalized; OBS inputs/scenes/tracks/mute/stream status are discovered and read. No OBS mutations or diagnostics product claims yet.

Own internal/twitch, internal/obs, integration wiring, minimal connection/setup controls, protocol fixtures and focused tests. Extend shared types only within CONTRACTS. Introduce pinned goobs/coder/websocket and record actual methods/versions in docs/implementation/P02-compatibility.md.

## Ordered tasks

### P02.1 — Compatibility and setup

- Record actual OBS/obs-websocket versions; compile the required SDK reads and confirm request fields.
- Needed reads: version, input list/identity, input mute, input audio tracks, program scene, scene items/enabled state, stream status. Determine supported UUID/context identity behavior.
- Use a dedicated test scene/source configuration selected by the user; never start/stop a stream or alter unrelated scenes.
- Confirm registered Twitch public client and own broadcaster identity. Client ID is non-secret; passwords/tokens enter only the private application UI.
- Missing accounts/credentials block actual live checks, not fixture-based adapter development. Report exact dependency.

### P02.2 — Device authorization and token lifecycle

- Implement the official Device Code Grant for public clients, provider polling interval/error/expiry behavior and cancellation.
- Show verification instructions only privately; never log device/access/refresh codes.
- Validate token identity, client ID, scopes and expiry before use; own-channel subject is authoritative.
- Store token pair together in keyring; serialize refresh, rotate credentials and surface persistence failures.
- Validate startup and hourly; revoked/invalid token disables connection and requires authorization. Don't endlessly retry invalid credentials.
- Private disconnect cancels polling/socket workers and clears this application's credentials.
- Test with an injectable local OAuth HTTP server, not real secrets in fixtures.

### P02.3 — EventSub connection

- Connect via coder/websocket. On welcome, subscribe to channel.chat.message using the session ID and correct user token.
- Enforce the server welcome/keepalive deadlines; distinguish requested session handover from ordinary lost-socket reconnect.
- Handle notifications, keepalive, ping/pong, reconnect and revocation. Don't send application messages through EventSub WebSocket.
- Recreate subscriptions after ordinary reconnect; don't claim missing messages can be replayed.
- Tag events with generation and ignore superseded sockets. Bound exponential reconnect backoff and stop on context cancel.
- Decode only needed payloads, reject malformed events, cap payload sizes and avoid raw-message logging.

### P02.4 — Chat intake/replies

- Parse C04 commands; normalize IDs and timestamps, ignore own outgoing messages.
- Introduce testable cooldown/dedup primitives reused by P03; don't count incidents before the coordinator exists.
- Send help/setup-status replies through Helix with bounded rate-limited queue and provider limit handling.
- Use a private diagnostic count/last-command type to demonstrate receipt; no full chat history or raw user content on overlay.
- No chat message invokes arbitrary OBS requests or approvals.

### P02.5 — OBS read adapter and minimal setup

- Connect/authenticate with configured local endpoint and secret; clear errors on successful reconnect and increase generations correctly.
- Discover inputs and direct scene items; store selected expectations through P01 revisioned settings.
- Allow explicit user confirmation of stream track 1–6; don't infer it from meter activity or silently substitute a detected track.
- Read fresh mute/track map/current scene/item status; mark partial failures unknown.
- Observe relevant invalidations/changes and re-resolve references after reconnect; renamed/deleted sources require reconfiguration.
- UI shows actual connection and broadcasting state, selected identities and stale data. No writes exposed.

## Required checks and acceptance

- P02-A / V06: Real authorized own-channel command from another viewer is visible as normalized input.
- P02-B / V07: Live OBS reads match the dedicated scene/input config, including all six track assignments.
- P02-C: Token mismatch/expiry/refresh rotation/revocation tested; no credentials leak through errors/state/logs.
- P02-D / V08: Welcome/subscription, keepalive, planned handover, ordinary reconnect, duplicate event and old generation covered with fixtures.
- P02-E: OBS disconnect marks data unavailable; reconnect doesn't preserve false freshness or wrong source identity.
- P02-F: No mutation API was introduced; P01 boundaries continue to pass.

Handoff separates real/live observations from protocol fakes. If live gates are missing, identify who/what is needed; no unconditional acceptance claim. Stop for Codex review before P03.
