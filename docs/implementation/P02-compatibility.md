# P02 integration compatibility

Packet: P02 v1.1. Recorded 2026-11-01 UTC by DeepSeek. This note records what
was compile-verified and what remains a live prerequisite; it is not a claim
that Twitch or OBS worked live.

**Current Codex closure outcome (2026-11-01):** local/full race checks and final
hidden-browser/live OBS verification passed. OBS 32.1.1 negotiated
obs-websocket 5.7.3; this is distinct from the SDK's protocol constant. Only live
Twitch P02-A/V06 is deferred. The historical worker notes below describe earlier
stages; [the closure review](../execution/handoffs/P02-codex-closure-review.md)
and [exact evidence](P02-codex-closure.md) supersede their former limitations.

The pinned goobs v1.11.1 now resolves through `replace` to `third_party/goobs`.
Only upstream `client.go` and `api/client.go` changed: context-aware dial/auth,
joined workers, safe disconnection and bounded request admission/response.
Original generated source and Apache-2.0 license are intact, with modified-file
notices and [patch provenance](../../third_party/goobs/CUEMENDER_PATCH.md).

## Toolchain and pins

| Item | Value |
|---|---|
| Go | `go1.27.3 windows/amd64` (`.tools/go1.27.3/go`) |
| Module | `github.com/desanv01/cuemender` |
| OBS SDK | `github.com/andreykaipov/goobs v1.11.1` (Apache-2.0) |
| OBS dialer | `github.com/gorilla/websocket v1.5.3` (BSD-3-Clause), via goobs `WithDialer` handshake timeout |
| WebSocket transport | `github.com/coder/websocket v1.8.15` (ISC) |
| SQLite | `modernc.org/sqlite v1.61.1` unchanged |
| Keyring | `github.com/zalando/go-keyring v0.2.8` unchanged |
| goobs protocol constant | `goobs.ProtocolVersion = "5.7.6"` |
| Production build | `CGO_ENABLED=0 go build -trimpath -o dist/cuemender.exe ./cmd/app` |

`go.mod` now lists goobs and coder/websocket as direct requirements. `go mod
tidy` was run; `go.sum` retains the complete graph. The SDK go.mod files declare
Go 1.23, so Go 1.27.3 is supported by the pinned modules.

The compatibility test `internal/obs/goobs_compat_test.go` compiles method
expressions and response-field assignments for every production read used by
the adapter. It performs no network I/O and is not live evidence.

## goobs method mapping

The adapter constructs `goobs.New(endpoint, ...)` with the full configured
`ws://`/`wss://` URL, `WithPassword`, `WithResponseTimeoutDuration(5s)`,
`WithEventSubscriptions(0)` and a `gorilla/websocket` dialer with a 2-second
handshake timeout. The URL path/scheme is validated before dialing. SDK types
remain inside `internal/obs`; the rest of the application uses typed domain
structures. OBS startup dialing is bounded to two attempts with a short delay
so a disabled/misconfigured server reaches `needs_attention` rather than
retrying forever.

| Contract read | Actual goobs method/type | Fields used |
|---|---|---|
| OBS version | `Client.General.GetVersion` → `general.GetVersionResponse` | `ObsVersion`, `ObsWebSocketVersion`, `RpcVersion` |
| Input list | `Client.Inputs.GetInputList` → `inputs.GetInputListResponse` | `Inputs[].InputName`, `InputUuid`, `InputKind` |
| Input mute | `Client.Inputs.GetInputMute` → `inputs.GetInputMuteResponse` | `InputMuted` |
| Six-track map | `Client.Inputs.GetInputAudioTracks` → `inputs.GetInputAudioTracksResponse` | `InputAudioTracks` (`typedefs.InputAudioTracks`, `map[string]bool`) |
| Scene list | `Client.Scenes.GetSceneList` → `scenes.GetSceneListResponse` | `Scenes[].SceneName`, `SceneUuid` |
| Current program scene | `Client.Scenes.GetCurrentProgramScene` → `scenes.GetCurrentProgramSceneResponse` | `SceneName`, `SceneUuid` (deprecated fields accepted as fallback) |
| Direct scene items | `Client.SceneItems.GetSceneItemList` → `sceneitems.GetSceneItemListResponse` | `SceneItems[].SceneItemID`, `SourceName`, `SourceUuid`, `SceneItemEnabled`, `IsGroup` |
| Item enabled | `Client.SceneItems.GetSceneItemEnabled` → `sceneitems.GetSceneItemEnabledResponse` | `SceneItemEnabled` |
| Stream status | `Client.Stream.GetStreamStatus` → `stream.GetStreamStatusResponse` | `OutputActive` |
| Scene collection context | `Client.Config.GetSceneCollectionList` → `config.GetSceneCollectionListResponse` | `CurrentSceneCollectionName` |

Parameter builders used: `WithInputName` / `WithInputUuid`,
`WithSceneUuid` / `WithSceneItemId`. OBS scene-item IDs are scoped to their
scene. The actual wire `inputAudioTracks` object uses numeric keys `"1"`–`"6"`;
`InputAudioTracks` converts them at the adapter boundary to the stable domain
keys `track1`–`track6`, validates the six-entry shape and leaves missing or
malformed entries unknown (never false). `WireTrackKey` documents the inverse
mapping for P04. Local protocol tests exercise the real pinned client, assert
only the read-only request allowlist, and cover success, missing response data
and protocol errors.

The adapter configures a silent goobs logger so provider payloads cannot reach
stderr and bypass application redaction. The 2-second handshake and 5-second
request bounds are adapter-level; goobs network methods do not consume the
application context directly. OBS dialing retries twice initially; established
transport loss increments the generation, invalidates discovery/observation and
readiness, closes the transport and redials with bounded backoff until success
or explicit disconnect.

The production adapter exposes no OBS mutation method. `Backend` contains only
read operations; `TestOBSBackendSurfaceIsReadOnly` checks the interface method
names and a compile-time test checks the SDK surface. No `SetInputAudioTracks`,
scene mutation, stream start/stop, or arbitrary request-name route exists.

## Twitch and EventSub

Production endpoints are fixed in `internal/twitch.DefaultEndpoints`:

- Device: `https://id.twitch.tv/oauth2/device`
- Token/refresh: `https://id.twitch.tv/oauth2/token`
- Validation: `https://id.twitch.tv/oauth2/validate`
- Helix base: `https://api.twitch.tv/helix`
- EventSub: `wss://eventsub.wss.twitch.tv/ws`

Required scopes are `user:read:chat` and `user:write:chat`. The Device Code
Grant is used with a Public client; no client secret is read or stored. The
private user code/verification URI are shown only through authenticated private
state. Device code, access token and refresh token never enter a projection,
config, SQLite, log or report.

EventSub uses `github.com/coder/websocket`:

- `websocket.Dial(ctx, endpoint, &websocket.DialOptions{HTTPClient, HTTPHeader})`
- `Conn.Read(ctx)` for text messages, bounded by `SetReadLimit(1 MiB)`
- `Conn.Close(...)`
- welcome/subscription via Helix `POST /eventsub/subscriptions` with
  `channel.chat.message` v1 and the validated own broadcaster/user ID
- `session_keepalive`, `session_reconnect`, revocation and notification handling
- requested reconnect URL must be `wss://eventsub.wss.twitch.tv/...`; no
  resubscription is sent for a valid handover
- ordinary loss creates a fresh session and subscription, with bounded backoff

Helix chat sends use `POST /chat/messages` and require `is_sent == true`; a
rejected or ambiguous send is not blindly retried. The bounded reply queue is
one aggregate reply per 3 seconds, capacity 20, expiry 30 seconds, and it
honours a provider `Retry-After` before the next aggregate item. A typed
`CommandEvent` seam preserves normalized delivery/message/broadcaster/actor,
command, category/code and connection generation without raw chat text.

Startup and the maintenance loop call `/validate` and check client, user
subject, required scopes and positive expiry. Short-lived tokens are refreshed
before expiry; rotated tokens are validated before use, missing scopes are never
restored from the old credential, and revocation enters one terminal
authorization state that stops intake, replies and reconnect work. A new
credential-dependent flow is refused when OS storage is unavailable unless
memory-only mode was explicitly selected. Credential save/delete operations are
serialized; a late write is removed if disconnect won the race.

EventSub requested handover adopts the replacement socket and the socket owner
always closes the final socket on cancellation. Ordinary socket replacement
advances the connection generation; queue/receipt state is not treated as a new
authorization.

## P03 diagnostics integration

- SQLite migration 2 adds monitoring_sessions, incidents, findings, reports,
  report_dedup and feedback. Migration 1 is untouched; a real version-1 database
  preserves app_settings/revision.
- `internal/diagnostics` implements only the three supported checks. Missing,
  incomplete, stale, revision/generation/context-mismatched evidence remains
  unknown and never becomes healthy.
- `internal/incidents` owns monitoring sessions, durable report dedup, category
  grouping, bounded jobs, transition rules, dismissal and feedback. Jobs use a
  fresh `obs.Manager.DiagnosticRead` five-second read, not SnapshotBundle.
- The P03 Twitch seam uses `twitch.Options.OnCommand` and the existing
  `ReplyQueue`; application wiring suppresses P02's static idle acknowledgment
  and queues only the durable coordinator reply. No second bot/queue exists.
- Routes are the four C09 private mutations under the existing session, CSRF,
  Origin, host, strict-body and envelope boundaries. Public incidents contain
  only current monitoring-session fixed fields.
- No new modules, dependency versions, OBS writes or P04 approval endpoints.

## P02-R01 correction evidence

- **R01:** real goobs wire keys normalize at the adapter boundary; protocol
  tests cover success, missing fields, errors and request allowlist.
- **R02:** established-loss reconnect/new generation, manual-disconnect retry
  stop, saved-password fresh-manager reuse, transport close-before-wait and
  silent SDK logger are covered. Automatic re-dial uses bounded backoff.
- **R03:** discovery is refreshed on polling and explicit GET; input UUID/name,
  direct scene/item/source and scene collection context gate readiness; settings
  revision and generation are returned in typed projections; the app consumes
  one manager snapshot bundle. Old results are discarded on revision change.
- **R04:** validation/revocation/storage policy described above; EventSub
  revocation is terminal rather than an endless reconnect.
- **R05:** lifecycle and credential operations serialize; config replacement is
  atomic with short Windows replacement retries; app connect/disconnect calls
  serialize.
- **R06:** handover replacement ownership and connection generation are explicit.
- **R07:** Retry-After is normalized and honored by the queue; cooldown is
  bounded/expiring; typed command events avoid raw chat.
- **R08:** discovery identity UUID/context is preserved through save/reload;
  null discovery clears stale selectors; mute/tracks/scene/visual/streaming
  observation fields render in the private control page.

## Live prerequisites and limits

- **Twitch live gate unmet:** no Twitch Public client is registered, no client
  ID is configured, and no own-channel authorization or separate viewer test
  was possible. All Twitch tests use a local HTTP fixture and synthetic
  sentinels. This note does not claim P02-A/V06 or real EventSub behavior.
## P02-R03 completion evidence

- OBS handshake ownership now uses an adapter-owned underlying network connection
  registered by a custom gorilla dialer. Cancellation closes that raw connection
  and returns promptly even while OBS Hello is withheld; `goobs.New`'s own
  authentication goroutine remains bounded by its SDK timer. A candidate is not
  published until the constructor returns, but the manager owns transport before
  the first Version/discovery read.
- OBS context cancellation closes the owned transport. Manager Connect/Disconnect/
  Close are serialized; the active candidate is closable during initial setup and
  reconnect. `expectationsEqual` uses `reflect.DeepEqual`.
- Readiness now requires connected, fresh/non-stale evidence, matching settings
  revision, matching generation, resolved identities and known context. Collection
  context is sampled before Inputs/Scenes and after the identity sequence; a failed
  start read cannot be upgraded by a later success, and mid-read switches are
  unknown. Read operations capture revision at operation start; explicit discovery
  refresh refuses to install a result whose revision changed.
- Twitch maintenance runs under an authorization-lifetime context and a join
  mutex. Disconnect cancels that context and waits for maintenance before clearing
  credentials; requested handover starts replacement dialing concurrently, keeps
  exactly one old-socket reader processing notifications until replacement welcome,
  then retires the old socket. Lifecycle lock covers Connect/StartSaved/Disconnect/
  Close. Queue close/join, Retry-After, expiry draining and terminal auth routing
  remain as corrected in R02.
- Setup UI keeps identity-based selector signatures (timestamp-only updates do not
  rebuild), restores valid selections, clears hidden UUID/context on manual name
  edits and preserves scene UUID. Readiness/observation time/stale/partial display
  remains private. Config replacement failure has both direct and app-route tests.

## P02-R02 correction evidence

- TokenInfo.ExpiresIn is a `time.Duration`; maintenance uses the injected clock
  and adds it once. The maintenance loop checks at most every 30 seconds so a
  60-second token is refreshed before expiry rather than waiting for the hourly
  maximum. Zero/negative expiry requires authorization. Rotated tokens are
  validated for client/subject/scopes/positive expiry; a persistence failure
  keeps the validated rotated pair in memory with explicit degraded state.
- OBS SDK calls are bounded by the caller context through the adapter; on
  cancellation the adapter closes the transport and the SDK request is bounded
  by its own timeout. The candidate transport is published before Version and
  discovery so Disconnect/Close can close it mid-setup and during reconnect.
- Discovery records whether the scene-collection read was known. The collection
  is sampled before and after identity reads; a failure or mid-read switch makes
  context unknown and prevents readiness. Visual identity now includes scene
  UUID in addition to direct item/source UUID.
- Initial connect/reconnect installs only data captured under the current
  settings revision and generation; old data is omitted rather than relabeled.
- ReplyQueue rejects enqueue after close, joins an active sender on close, drains
  expired heads, honors Retry-After, and never replays an ambiguous send.
  Helix 401/403 in a reply sender is routed to terminal authorization handling.
- Setup selectors use an identity-based signature (not captured_at), preserve
  existing selections, and clear hidden UUID/context when a name is manually
  edited. Readiness, observation time, unknown/partial and stale states are
  rendered in private control only.
- Programmatic config replacement is injectable for deterministic filesystem
  failure tests; a failed replacement preserves prior bytes. D25 remains:
  explicit OBS Connect after restart, no auto-start preference.

- Historical worker limitations were closed by Codex: actual OBS handshake,
  six-track/mute/scene/visibility/stream comparison, hidden-browser tests and
  whole-app/patched-SDK race suites all passed. Race used existing WSL Ubuntu
  GCC 13.3.0 with a checksum-verified workspace-local Go 1.27.3; Windows
  production builds remain CGo-free. Live Twitch remains deferred, not passed.
