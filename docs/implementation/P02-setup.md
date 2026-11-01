# P02 Twitch and OBS setup

This guide covers the P02 setup path. It is intentionally operator-driven: the
application never starts/stops a stream, changes an OBS scene or track, or
requests a client secret.

## 1. Start CueMender

```powershell
.\dist\cuemender.exe --no-browser
```

Open the single-use private control link printed in the console within five
minutes. The browser clears the link fragment and uses the local session cookie.
The default data directory is `%AppData%\LiveCheck` for storage compatibility;
use `--data-dir` for an isolated setup/test.

## 2. Register a Twitch Public client

1. Sign in to the [Twitch Developer Console](https://dev.twitch.tv/console) with
   the broadcaster's own Twitch account.
2. Complete Twitch's required verified-email and two-factor prerequisites for
   the account.
3. Register an application as a **Public** client. The Device Code Grant uses a
   client ID only; do not generate or paste a client secret into CueMender or
   either harness.
4. Device flow does not use an application callback. If the registration form
   requires a redirect value, treat it as registration-only and verify the
   current form's accepted value against Twitch's documentation; CueMender does
   not expose a redirect route.
5. Copy only the **client ID** into the Twitch panel of CueMender and press
   **Connect Twitch**. The client ID is non-secret and may be persisted in
   `config.json`.
6. CueMender displays a verification URI and a user code in the authenticated
   private panel. Open Twitch's official verification page, enter the code, and
   approve access for the broadcaster's own account. Do not paste tokens,
   device codes, or authorization responses into chat or a report.
7. The panel reports authorizing/pending, connected, expired, denied, or
   needs-attention states. A reload of the control page recovers pending
   instructions from private state. The user code is not a password; still keep
   the private page local.

The requested scopes are `user:read:chat` and `user:write:chat`. Tokens are
validated at startup and by the maintenance loop (client ID, user subject,
scopes and expiry), refreshed before expiry — including short-lived tokens —
and rotated tokens are validated before use. A persistence failure after a
successful rotation keeps the new pair for the current session with a degraded
marker. A revoked/invalid authorization stops chat intake, replies and
reconnect work; transient network/5xx failures do not erase a still-valid
authorization. Credentials are stored as one application-owned credential
entry, and the legacy keyring service namespace `LiveCheck` is retained. If OS
credential storage is unavailable, Twitch connect is refused unless
`--session-only-secrets` was explicitly selected; there is no silent memory-only
fallback.

## 3. Enable OBS WebSocket locally

1. In OBS, open **Tools → WebSocket Server Settings**.
2. Enable the WebSocket server and its authentication/password option. Keep the
   server bound to localhost; do not expose it to the network.
3. Enter the endpoint (default `ws://127.0.0.1:4455`) and password in the OBS
   panel. Do not paste the password into chat or a report.
4. Press **Connect OBS**. CueMender shows connecting/connected/needs-attention
   separately from OBS broadcasting status.
5. Leaving the password input blank with **Reuse saved password** checked uses
   the credential for the same endpoint. Unchecking it and submitting an empty
   password explicitly selects unauthenticated OBS. A different endpoint never
   reuses another endpoint's password.

CueMender is read-only in P02. It reads the OBS version, input list, input mute,
six-track map, program scene, program-scene UUID, direct scene items, item
enabled state, stream status and scene-collection name. OBS's wire track keys
are `"1"`–`"6"`; the UI and stored expectations use `track1`–`track6`. It has no
route or adapter method that mutates OBS. The private page displays the latest
mute, six-track, scene, visual and streaming observations with a separate stale/
partial marker.

OBS does not auto-dial at startup (D25: explicit private Connect after restart).
The default endpoint does not prove intentional setup. Press **Connect OBS** to
explicitly start the saved endpoint; omission of the password reuses the saved
credential for that exact endpoint, including after a restart. A changed
endpoint never inherits another endpoint's password. Established transport loss
is recovered automatically until explicit disconnect. Connect/Disconnect and
shutdown own the active socket from handshake through reads; cancellation closes
the transport rather than leaving a hidden background connection.

Twitch authorization work is tied to the active session: disconnect cancels and
joins pending validation/refresh, and requested EventSub handover keeps processing
old updates until the replacement connection is usable. Screenshot-style
evidence of readiness must use the private readiness plus observation time, not
just a stale field.

## 4. Select a dedicated test setup

Use an OBS scene collection and microphone/input dedicated to this test. Do not
use an active client broadcast for fault injection. In the private control page:

1. Press **Refresh discovery** after OBS connects.
2. Select the microphone input and a scene/direct scene item from discovery, or
   enter identities manually if discovery is intentionally unavailable.
3. Select and explicitly confirm the streaming track 1–6. Do not infer the track
   from meter activity; it is the broadcaster's declared routing expectation.
   Discovery selections also retain stable UUIDs for input, source and scene plus
   scene-collection context. A same-name source/scene recreated with a different
   UUID, or a changed collection, requires reselection instead of inheriting
   ready status. A timestamp-only discovery poll does not reset the current
   selector; manually editing a name clears its hidden identity.
4. Save expectations. The live-resolution indicator is separate from structural
   save success: it is only ready when the selected identities resolve against
   current discovery at the same settings revision and connection generation.
   The page shows readiness, observation time and current mute, six-track,
   program-scene, visual and streaming values separately from discovery; partial
   and stale warnings are retained together.

## 5. Real Twitch chat check (when available)

Use a **separate viewer account** in the broadcaster's channel. The broadcaster's
own messages are ignored.

```text
!streamhelp
!issueaudio
!issuevideo
!fixed AB12
!stillbroken AB12
```

The private panel shows only an accepted-command count, last command type,
category and time. P02 does **not** create incidents, process reports, or open a
monitoring session. Replies are fixed, bounded and rate-limited; the public
overlay remains idle/disconnected and never shows authorization instructions,
input/scene names or chat identities.

Record the normalized receipt and a bounded reply without logging raw chat. A
live check is not complete until a separate viewer's command is observed and a
chat reply is actually marked sent by Helix.

## 6. Real OBS comparison (when available)

Compare private observations with OBS's own UI:

- input mute state for the selected microphone;
- all six track enable flags, not only the confirmed track;
- current program scene;
- selected direct scene item's enabled state;
- OBS stream status.

Record the exact OBS version, obs-websocket version and observation timestamp.
Do not modify the scene/track as part of this comparison. The P02 live gate is
unmet until this is done on the dedicated setup.

## 7. Disconnect and recovery

- **Twitch disconnect** cancels polling/socket/reply work and deletes this
  application's credential. If deletion fails, the panel reports
  needs-attention and does not claim the credential was removed.
- **OBS disconnect** cancels reads and invalidates discovery/observations but
  retains the endpoint-scoped password for an explicit reconnect or next
  startup. A lost established transport is re-dialed with bounded backoff and a
  new generation; a manual disconnect stops that retry.
- A new connection attempt while that provider is connecting/connected returns
  a conflict; disconnect first or change the endpoint/client deliberately.
- Restarting the process creates a new private launch link. Validated saved
  Twitch credentials may resume; no credentials means idle, not a spontaneous
  device flow. OBS requires an explicit private Connect because the default
  endpoint alone is not usable saved setup.

## Troubleshooting

| Symptom | Meaning / action |
|---|---|
| No Twitch connect | Confirm Public client ID and that the app is registered as Public; check the private detail code. |
| Device code expired/denied | Start Connect again. CueMender does not reuse a consumed code. |
| Twitch needs attention | The saved token was rejected or scopes/client/identity changed. Reauthorize; no endless invalid-token loop. |
| Credential storage unavailable | OS keyring did not answer. Restart with `--session-only-secrets` if a memory-only run is explicitly acceptable; there is no plaintext fallback. |
| OBS 503 discovery | OBS is not connected or discovery has not completed; connect and refresh. |
| OBS authentication failure | Check the local WebSocket server password and that authentication is enabled. The raw provider error is not exposed. |
| OBS endpoint conflict | One provider lifecycle is busy; disconnect first. |
| 409 on connect | The provider is already connecting/connected or a configuration update is in flight. |
| Public overlay empty | Expected in P02: monitoring is false and incidents are empty. |

## Remaining live gates

P02-B/V07 passed on 2026-11-01 using the final CueMender build, OBS 32.1.1 /
obs-websocket 5.7.3 and the user-authorized Mic/Aux / Scene / CueMender Test Card
setup. All six flags matched; live disconnect/reconnect and real-browser
save/reload/save passed. See [the closure review](../execution/handoffs/P02-codex-closure-review.md).

Only P02-A/V06 remains: no registered Public client, broadcaster device
authorization or separate-viewer live command receipt/reply. The user explicitly
deferred this work due to 2FA/phone setup. Local fixtures do not satisfy that gate;
Full live Twitch acceptance remains pending. Under the user's D30 decision, P03
v1.2 may proceed on the verified local/live-OBS baseline; P02-A/V06 and P03-D's
real-chat portion remain tracked in issue #6 before final readiness. Do not resume
Twitch account work until the user lifts the hold.
