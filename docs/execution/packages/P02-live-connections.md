# P02 — Real Twitch and OBS connections

Revision: 1.1 • RELEASED for DeepSeek implementation when STATUS assigns P02.
Prerequisite: P01 accepted in [P01-R01 review](../handoffs/P01-R01-review.md).
Tracking: https://github.com/desanv01/cuemender/issues/2

## Outcome and limits

A broadcaster authorizes their own Twitch channel, receives normalized chat commands, and connects to local OBS to discover and read selected sources. The private control page supports that setup. Public state remains sanitized; monitoring_active stays false and incidents stay empty.

No diagnostics/incident engine, proposals, approval code, OBS writes, streaming start/stop, runtime AI, second backend or final UI redesign. P03 owns diagnostics; Codex owns P04. Do not generate P04 code in advance.

## Starting point and allowed files

- Accepted foundation commit: a07b3df3c33f0284c1da450a323fbba3e9ed1771. Codex's subsequent presentation/module rename is part of your starting checkout; do not revert it.
- Module: github.com/desanv01/cuemender. Build output: dist/cuemender.exe.
- Preserve legacy storage identifiers: LiveCheck data directory/keyring namespace, livecheck.db, livecheck_session cookie. No data migration in this packet.
- Existing app composition, config and state: internal/app/{app,config,events}.go. Existing HTTP boundary: internal/httpui/{server,respond,session}.go. Models: internal/domain/{settings,projections}.go.
- Add internal/twitch and internal/obs with narrowly scoped adapters; extend app, domain, HTTP and embedded control assets as needed. Keep HTTP -> domain interfaces -> app wiring; no httpui -> app circular import.
- Extend tests and testdata; write docs/implementation/P02-compatibility.md, docs/implementation/P02-setup.md and docs/execution/handoffs/P02-worker-report.md. Update factual development instructions if needed; preserve README presentation.
- go.mod/go.sum may add the selected goobs and coder/websocket dependencies and their required transitives, pinned to actual verified versions. No replacement SDK/framework/service without Codex decision.
- Specifications are read-only to the worker. STATUS operational fields may be updated. Other packets remain held.

## P02.0 — Preflight and ownership

1. Read AGENTS, STATUS, WORKFLOW, PROJECT_SPEC, ARCHITECTURE, CONTRACTS including C08, this packet and P01-R01-review.
2. Verify no active writer, then claim P02 in STATUS. Record current Git branch/head read-only; Codex prepares feature/p02-live-connections.
3. Use the verified portable Go at .tools/go1.27.3/go; see docs/DEVELOPMENT.md. Dependency downloads need network access; offline GOPROXY=off cannot fetch new modules. Use an official module proxy/checksum verification when permitted. Do not disable TLS or checksum verification.
4. Compile a small in-package compatibility test for the real pinned SDK methods before building elaborate integration code. Record Go/module/SDK/OBS protocol versions and method-to-contract mapping.
5. User has no registered Twitch Public client yet. Worker-reported OBS 32.1.1 has WebSocket disabled. Complete fixture-based work while these prerequisites are missing; do not mark live gates passed.

## P02.1 — App seams, lifecycle and safe setup

- Implement C08 routes through the existing session/CSRF/Host/Origin/body wrappers. A new endpoint must not bypass them.
- Add an app-owned connection lifecycle for each provider with cancellation, generation, bounded retry and a stop/wait hook. Keep SDK types private to adapters.
- Build typed connection status: not_connected, connecting, authorizing, connected, reconnecting, needs_attention. Private details use fixed safe codes/copy; provider errors must not leak request bodies or tokens.
- Never hold stateMu or the publication lock during network calls. Compute results outside locks; apply only if their generation/config still matches, then serialize revision/projection/publication through the accepted ordering boundary.
- A connect/disconnect/replacement invalidates old results. Serialize lifecycle transitions so disconnect cannot be undone by a late OAuth exchange, refresh write or old socket callback.
- Shutdown cancels polling, sockets, HTTP calls, retry timers and reply workers, waits within a bound, and retains the P01 active-SSE/port-release guarantees.
- Add atomic startup config persistence in internal/app/config.go. Serialize independent client-ID/OBS-address updates; validate before writing, write/sync/close temporary file in the same directory, replace safely on Windows, preserve the prior valid file on failure. Never persist CLI --listen overrides as an incidental connect side effect.
- No credentials in config/SQLite/URLs. Use secrets.Store and redact wrapped SDK errors at the boundary. If OS storage is unavailable, refuse credential operations with a safe needs_attention state; require an explicit restart with --session-only-secrets.

## P02.2 — Twitch authorization and credentials

- Implement official Device Code Grant for a Public client using user:read:chat and user:write:chat. Never require a client secret.
- Begin one flow at a time. Display the provider verification URI/user code/expiry only in authenticated private state; device_code and access/refresh tokens remain backend-only.
- Respect provider polling intervals, pending/slow_down/denial/expiry errors and cancellation. Inject clocks/timers/HTTP endpoints in tests; production endpoints are fixed official Twitch hosts, not caller-supplied URLs.
- Validate returned token subject, client ID, scopes and expiry using the official validation endpoint. Use the broadcaster token subject for broadcaster/user/sender identity; do not accept arbitrary channel IDs from a browser body.
- Persist access+refresh pair and binding metadata as one credential entry. Serialize refresh and store rotated credentials together. Treat save failure as persistence degraded; never silently claim durable authorization or fall back to plaintext.
- Validate stored credentials at startup and hourly; refresh through the documented Public-client flow when appropriate. A revoked token or client/scope/identity mismatch stops dependent workers and requires authorization; no endless invalid-token loop.
- Startup may resume a validated saved Twitch authorization. No stored credentials means idle, not a spontaneous device flow. Changing client ID invalidates old authorization before the replacement can connect.
- Disconnect cancels all Twitch work, drains/discards pending replies and deletes app-owned credentials. If deletion fails, remain disconnected for this run and report the failure explicitly; do not claim credentials removed. Guard late refresh persistence with lifecycle serialization.
- Tests use synthetic sentinel credentials and a local HTTP fixture. Never record a real authorization response.

## P02.3 — EventSub and normalized command intake

- Use coder/websocket for EventSub and net/http for Helix. Handle welcome, notification, keepalive, reconnect and revocation. Honor server-provided welcome/keepalive deadlines and subscribe promptly after welcome.
- Subscribe to channel.chat.message version 1 with broadcaster_user_id and user_id equal to the validated own-channel identity, using the new session ID. Use the user access token for this installed-client model.
- Requested reconnect uses the supplied secure Twitch reconnect endpoint and preserves the subscription handover. Ordinary loss creates a fresh session and subscriptions. Retire the old session only after the new handover welcome; distinguish duplicate delivery from missing-message recovery. There is no replay guarantee after loss.
- Reject unexpected reconnect URL schemes/hosts and oversized/malformed payloads. Do not log raw chat payloads. Let the WebSocket client handle protocol ping/pong; never send application messages to EventSub.
- Bound retry backoff, queue sizes and message payloads; cancel promptly. Tag callbacks with the active generation; superseded deliveries cannot update state.
- Parse exact supported commands: !issueaudio, !issuevideo, !fixed CODE, !stillbroken CODE, !streamhelp. Normalize case/whitespace deliberately, reject excess arguments and bound public-code length. No arbitrary commands/HTML or OBS request execution.
- Normalized event includes C04 fields. Ignore the broadcaster's own outgoing identity. Deduplicate delivery and message IDs (24 hours, maximum 10,000 retained IDs), apply 10-second per-actor issue cooldown; expire/evict deterministically and test with injected time.
- P02 private diagnostic receipt only: count accepted normalized commands and last command/category/time. No raw text or actor identifiers in this summary; no persisted report/incident counts and no monitoring side effects.
- Preserve a small typed callback seam for P03. P02 may observe diagnostic receipt while idle, but must not claim reports are being processed into incidents.
- Send !streamhelp and idle/setup acknowledgments via Helix Send Chat Message. Fixed bounded copy, one aggregate reply per 3 seconds, queue capacity 20 with expiry, obey stricter rate limits/Retry-After; no reply storms. Check the API's actual send result, not just HTTP success. Do not retry an ambiguous send blindly or echo raw user content.

## P02.4 — OBS read adapter and identity

- Verify goobs transport/authentication arguments against the pinned SDK; config stores a ws/wss URL but SDK may expect host:port plus options. Support valid selected transports or report unsupported configuration before connecting; never silently downgrade TLS.
- Required reads: GetVersion, GetInputList, GetInputMute, GetInputAudioTracks, GetSceneList/current program scene, GetSceneItemList, GetSceneItemEnabled and GetStreamStatus. Record actual SDK names/types and protocol availability, including UUID and scene-collection context support. Do not invent fields.
- Keep one connection owner and a narrow read interface. Handle authentication failure, disconnect and bounded retry; manual disconnect stops retries. Startup may resume the saved loopback endpoint with stored password; absent usable setup stays idle.
- Store OBS password through secrets.Store only, scoped to the endpoint. An endpoint change must not reuse a password belonging to another endpoint. Empty explicitly submitted password means unauthenticated OBS; omitted means reuse saved credential for the same endpoint.
- Discovery returns input identities and direct scene/item identities with generation/context and timestamps. Bound list sizes, avoid nested/group recursion. Reject ambiguous or missing identities; do not silently select first/name-only matches after deletion/recreation.
- Return complete six-track values when known. Missing values and failed reads are unknown, not false. Each observation carries captured config revision, generation/context and observed_at; partial errors are typed per field.
- Refresh private observations at a bounded cadence (suggested 2 seconds) and after relevant events; no overlapping unbounded poll work. Mark cache stale after 5 seconds and invalid immediately on disconnect/context change. Diagnosis later performs its own fresh reads.
- Rediscover after reconnect/scene-collection changes. UUIDs can assist resolution but do not justify automatic retargeting; name/context mismatch requires reconfiguration. If the SDK cannot reliably detect same-name recreation, explicitly require reselection after context loss and document the limit.
- P01 settings allow incomplete/offline setup; keep structural saves possible but resolution false while offline. When live, validate selected refs against discovery before marking ready. Recheck revision/generation after asynchronous resolution before publishing readiness.
- Stream-track selection requires explicit user confirmation of 1–6. Do not infer the output track from level activity or set a default. Track maps describe configuration, not measured audience playback.
- No SetInputAudioTracks or any other OBS mutation in implementation or exposed routes. Tests must inspect actual outbound protocol requests and reject non-allowlisted requests.

## P02.5 — Minimal private controls

Use existing vanilla assets/components, with small accessible labeled controls and fixed safe text, not a new frontend stack.

- Twitch client ID, Connect/Disconnect, pending device instructions, truthful state and safe error/retry guidance.
- OBS endpoint, password input, Connect/Disconnect; never refill password from backend or preserve it in localStorage/URL/HTML. Clear submitted password from the form.
- Input and direct scene-item selectors using discovery IDs; explicit confirmed stream track; save through existing revisioned settings.
- Show OBS connected/broadcasting separately from setup readiness, stale/unknown fields and selected identities.
- Show private normalized-command count/last-type/time for live verification. Public overlay remains empty/idle and never exposes authorization, input/scene names or diagnostic identities.
- Preserve bootstrap -> save -> reload -> save, CSRF recovery, locked state and request error handling. Use textContent for provider/source text, never innerHTML.
- A reload recovers pending device instructions via private state. Expired flow and failed storage are visibly different from connected.
- This is functional setup only. Final visual design belongs to P05.

## P02.6 — Setup guide and real validation

Write docs/implementation/P02-setup.md with these operator steps, adapting to the actual UI:

1. Sign into Twitch Developer Console with the broadcaster account; complete required verified-email/2FA prerequisites.
2. Register/manage an application as Public client. Use official registration guidance for required fields. Device flow does not use an app callback; do not claim a callback route exists. If the console requires a redirect value, document it as registration-only and verify what the actual form accepts.
3. Copy only the client ID into CueMender. No client secret is needed. Approve the device flow on Twitch's official page; do not paste tokens into either harness.
4. In OBS enable its authenticated WebSocket server locally, enter its endpoint/password privately in CueMender, and select a dedicated test setup. The user controls OBS settings and broadcasting. Worker must not start/stop a stream or change scene/mic state automatically.
5. Use a separate viewer account to send !streamhelp and one issue command in the broadcaster channel. Record normalized receipt/reply evidence without tokens/raw chat history.
6. Compare live mic mute, all six track assignments, scene-item visibility and stream status with OBS. Record actual versions/observations and cleanup, not just "connected".
7. Disconnect/reconnect and verify stale state/generation behavior. Live reconnect tests must not interrupt an active broadcast without the user's coordination.

If prerequisites are unavailable, finish independent development/tests, list the exact missing live checks and hand back as ready_for_review with unmet gates (or blocked only if implementation cannot continue). Acceptance remains with Codex; fixtures are not live evidence.

## Required checks / acceptance mapping

| ID | Required evidence |
|---|---|
| P02-A / V06 | Actual own-channel authorization and separate-viewer command receipt/reply; record live or unmet explicitly |
| P02-B / V07 | Actual OBS reads match selected dedicated setup including all six track flags; record live or unmet |
| P02-C | HTTP fixtures for device pending/slowdown/expiry/cancel, token binding/scopes/revocation, serialized refresh/rotation/save failure; late work cannot undo disconnect |
| P02-D / V08 | WebSocket fixtures for welcome/subscribe, keepalive deadline, requested handover vs ordinary resubscribe, duplicate IDs and old-generation rejection |
| P02-E | OBS fixtures for auth failure, partial reads, reconnect/context invalidation, name/ID ambiguity and stale cache; no wrong-source readiness |
| P02-F | All new routes enforce private boundary; public HTTP/SSE sentinel tests exclude credentials/device instructions/source names; outbound OBS request allowlist |
| P02-G | Config atomic persistence/failure/concurrent updates; startup persistence and explicit memory-only mode; settings publication ordering preserved |
| P02-H | Actual control JS regression and minimal browser setup checks when available; clearly distinguish Node VM from rendered browser |
| P02-I | Full go test ./... -count=1, go vet ./..., gofmt clean, CGo-free build; P01 active-SSE shutdown at least 5 runs with new workers cancelled; no lingering processes |

Run the existing node testdata/browser/control_csrf_reload.js and extend fixtures only as needed for the actual control code. Run race tests only on a supported configured toolchain; report unavailable gcc accurately, do not install system software to make a badge green.

Report exact commands/outcomes, pinned dependencies, protocol/request mapping, changed files, live vs fixture checks, remaining gaps, environment changes and stopped processes. Preserve P01 evidence. Transfer ownership to Codex, stop editing, do not begin P03.

## Official references

- [Registering a Twitch app](https://dev.twitch.tv/docs/authentication/register-app/)
- [OAuth / Device Code Grant](https://dev.twitch.tv/docs/authentication/getting-tokens-oauth/)
- [Token validation](https://dev.twitch.tv/docs/authentication/validate-tokens/)
- [Chat authentication](https://dev.twitch.tv/docs/chat/authenticating/)
- [EventSub WebSocket handling](https://dev.twitch.tv/docs/eventsub/handling-websocket-events/)
- [EventSub subscription types](https://dev.twitch.tv/docs/eventsub/eventsub-subscription-types/#channelchatmessage)
- [Helix Send Chat Message](https://dev.twitch.tv/docs/api/reference/#send-chat-message)
- [goobs](https://github.com/andreykaipov/goobs)
- [OBS WebSocket protocol](https://github.com/obsproject/obs-websocket/blob/master/docs/generated/protocol.md)

These define provider behavior. Pin and verify actual SDK signatures; no version is claimed compatible merely because its README contains an example.
