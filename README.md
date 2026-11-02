<h1 align="center">CueMender</h1>

<p align="center">
  <strong>Hear the report. Check the evidence. Approve the fix.</strong><br />
  A local Go application for community-assisted Twitch and OBS diagnostics.
</p>

<p align="center">
  Built for the <a href="https://twitchcon-codex.devpost.com/">Codex x TwitchCon Hackathon</a><br />
  By Desan Vasu
</p>

<p align="center">
  <img src="https://img.shields.io/badge/status-live_verification_pending-9146FF" alt="Local implementation; live verification pending" />
  <img src="https://img.shields.io/badge/Go-1.27.3-00ADD8?logo=go&logoColor=white" alt="Go 1.27.3" />
  <img src="https://img.shields.io/badge/platform-Windows_x64-65d2f4" alt="Windows x64" />
</p>

<p align="center">
  <a href="#features">Features</a> ·
  <a href="#demo">Demo</a> ·
  <a href="#get-started">Get started</a> ·
  <a href="#built-with-codex">Built with Codex</a> ·
  <a href="docs/DEVELOPMENT.md">Testing guide</a>
</p>

> **Development milestone:** connections, diagnostics and the private track-approval engine are implemented. Local tests and protocol fixtures are distinct from live evidence: Twitch verification and the real approved-repair trial remain pending. This is not yet the complete hackathon demo. See the [current verification record](docs/execution/handoffs/P04-review.md).

## Why CueMender

A viewer notices missing audio. The streamer needs to find the cause without disrupting the broadcast or accidentally revealing something private.

CueMender is being built around that interaction: receive a report through Twitch, compare OBS configuration with the streamer's declared expectations, explain the evidence, and request approval for a narrowly supported correction. Local configuration verification and viewer feedback remain separate.

## Features

The foundation below supports connection setup, three diagnoses and private approval of one microphone-track addition.

<table>
<tr>
<td width="50%" valign="top">

### Private local controls

Open a single-use launch link to establish a private session. Settings changes require the session and a CSRF token; the server listens on loopback.

</td>
<td width="50%" valign="top">

### Persisted expectations

Save microphone, stream-track and direct scene-item expectations in SQLite. Revision checks prevent stale updates from silently replacing newer settings.

</td>
</tr>
<tr>
<td width="50%" valign="top">

### A separate public overlay

Embedded control and overlay pages receive separate state projections. Public state excludes private source names, settings and credentials.

</td>
<td width="50%" valign="top">

### Explicit credential storage

The foundation supports OS credential storage or an explicitly selected memory-only mode. Backend errors are redacted; there is no plaintext fallback.

</td>
</tr>
</table>

**Implemented:** Twitch EventSub intake, OBS discovery, microphone mute/track and direct-item visibility diagnoses, durable incidents, viewer feedback, and an expiring private approval for one microphone-track addition. Unknown write outcomes are reconciled by reading, never by automatic replay. See [current scope](#current-scope) for verification limits.

## Demo

The development preview includes connection setup, private evidence and approval controls, and a separate public overlay. The steps below exercise only the foundation. For connection prerequisites, see [setup](docs/implementation/P02-setup.md); real Twitch and full approved-repair evidence remain pending.

1. Build and run the application below.
2. Open the one-time private control link printed in the console.
3. Save a sample microphone name and confirmed track.
4. Reload the control page and verify the saved settings.
5. Open the printed overlay URL to see its public disconnected state.

Real product screenshots, a gallery and the Twitch demonstration video will be added after capture and verification. No simulated screenshot is presented as live integration evidence.

## Get started

### Build the Windows development preview

Install [Go 1.27.3](https://go.dev/dl/) and Git, then run in PowerShell:

~~~powershell
git clone https://github.com/desanv01/cuemender.git
cd cuemender
go version
go mod download
$env:CGO_ENABLED = '0'
go build -trimpath -o .\dist\cuemender.exe .\cmd\app
.\dist\cuemender.exe --no-browser
~~~

The production build is verified with CGo disabled. Go and Node are not needed to run a compiled executable; Node is used only for the optional developer-side JavaScript regression fixture.

A release ZIP has not been published yet. The portable toolchain in the original developer's workspace is ignored by Git and is not part of a fresh clone.

### Open the controls

Open the private link within five minutes. Its launch capability is exchanged once and removed from the browser URL. Reloading keeps the authenticated session; restarting the process requires a new link.

Use Ctrl+C to stop the application. To isolate a trial:

~~~powershell
.\dist\cuemender.exe --no-browser --session-only-secrets --data-dir ".\local-data\trial"
~~~

The default listener is **http://127.0.0.1:8787/**. The overlay is **http://127.0.0.1:8787/overlay/**.

For compatibility with the initial foundation, the default data directory remains **%AppData%\LiveCheck**, the SQLite filename remains **livecheck.db**, and the credential-store namespace remains **LiveCheck**. The CueMender name does not silently move or delete that data.

[Flags, configuration, HTTP behavior and detailed testing →](docs/DEVELOPMENT.md)

## Built with Codex

Codex defined the architecture and execution contracts, reviewed DeepSeek's work, implemented connection/diagnostic closure fixes, and implemented the P04 approval-and-repair engine with its tests.

DeepSeek implemented P01–P03 and their worker corrections. Codex implemented P04 using the main chat and user-requested GPT-6.1 Sol high subagents with one writer at a time. Genuine main-chat build footage was recorded; final video editing and the live demonstration are still pending. Codex's tests of its own code are self-verification, with separately recorded peer inspection.

[Actual contribution record →](docs/execution/CONTRIBUTIONS.md) · [Foundation acceptance →](docs/execution/handoffs/P01-R01-review.md)

The required application workflow has no runtime AI dependency or paid-model requirement.

## Built with

| Runtime | Interface | Persistence | Integrations |
| --- | --- | --- | --- |
| Go 1.27.3 · Windows x64 | Embedded HTML/CSS/vanilla JS · SSE | SQLite via modernc · OS keyring | Twitch EventSub/Helix · OBS WebSocket |

One local process, with no separate frontend server or cloud database.

## Current scope

- **Accepted:** application lifecycle, validated persistence, local private sessions, CSRF protection, separate public/private state, embedded pages, and the corrected reload/concurrency/shutdown/error-redaction paths.
- **Implemented locally:** Twitch/OBS connections, three diagnoses, persistent incidents/feedback, and the private track-approval engine with durable intent, one dispatch, read-back and read-only recovery.
- **Next:** P05 user experience and isolated rehearsal are released for implementation under D34. Real Twitch/approved-repair verification stays pending; P06 release packaging remains held.
- **Outside the MVP:** automatic unmute or source reveal, raw audio analysis, computer vision, OS-device repair, multiple channels and autonomous control.
- **Verification limits:** local OAuth/EventSub/OBS protocol servers and headless rendered-browser checks do not prove live Twitch behavior. P02 live OBS read-only evidence is recorded separately; it is not a P04 live-write trial. Final commands, race results and remaining gates are recorded in [P04 verification](docs/execution/handoffs/P04-review.md). OBS has no atomic compare-and-set against other clients; local track read-back does not prove viewer audio recovery.

## Documentation

- [Development and testing](docs/DEVELOPMENT.md)
- [Product behavior and limitations](docs/execution/PROJECT_SPEC.md)
- [Architecture](docs/execution/ARCHITECTURE.md)
- [Execution status](docs/execution/STATUS.md)
- [Foundation acceptance](docs/execution/handoffs/P01-R01-review.md)
- [Demo capture and submission preparation](docs/execution/SUBMISSION.md)

## Feedback

[Open an issue](https://github.com/desanv01/cuemender/issues) with the expected result and reproducible steps. Use fictional test data and keep credentials, private launch links and personal chat logs out of public reports.

## License

A project license has not been selected yet. Dependency license notes are recorded in [the environment report](docs/implementation/P01-environment.md).
