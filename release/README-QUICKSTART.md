# CueMender 1.0.0 — Windows x64 release

CueMender is a local Go application for community-assisted Twitch and OBS diagnostics. It runs only on loopback and stores data locally.

## Start a receive-only session (no developer app or OAuth)

~~~powershell
.\cuemender.exe --receive-only-channel YOUR_TWITCH_LOGIN
~~~

Use your own lowercase Twitch channel login. No Twitch client ID or token is used, and no automatic replies are sent. Receive-only mode is read-only: it cannot prove channel ownership and cannot perform OBS repairs.

1. Open the one-time private control link printed by the executable.
2. In Setup, confirm the intended channel with `Receive-only chat` and explicitly confirm that this chat reports on your local OBS.
3. Connect OBS with the WebSocket address/password and select/confirm your microphone and track.
4. Start monitoring, then have a separate viewer send `!issueaudio` in chat.
5. CueMender reads OBS locally and shows evidence. It never sends chat messages in receive-only mode.

Data is stored under `%AppData%\LiveCheck\receive-only\YOUR_TWITCH_LOGIN` unless `--data-dir` is supplied. Do not copy an authenticated database into a receive-only directory.

## Isolated rehearsal (no account, no OBS)

~~~powershell
.\cuemender.exe --mode rehearsal --no-browser
~~~

Rehearsal uses simulated in-process providers and a child `rehearsal` data directory. Every private/public surface is labeled REHEARSAL. It is not live evidence and cannot contact Twitch or OBS.

## Authenticated EventSub mode

Authenticated EventSub requires a Twitch Public client ID, broadcaster authorization in the private Setup page, and a connected OBS WebSocket. It supports the same local diagnostics and the separately approved experimental track-addition workflow. Live Twitch authorization and real OBS approval remain environment-dependent and are not claimed by this ZIP.

## Data, privacy and cleanup

No credentials are included in this archive. Never copy runtime databases or `config.json` into a support bundle. To reset a directory, stop CueMender and remove only that app-owned directory after exporting anything you need. See the project README for full privacy and setup detail.

## Verification

This archive was built from the current source with CGo disabled and embedded web assets. See `THIRD_PARTY_NOTICES.md`, `licenses/`, and the project repository for full documentation. The MIT license applies to CueMender; third-party notices are preserved separately.
