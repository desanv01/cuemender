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
  <img src="https://img.shields.io/badge/status-foundation_accepted-9146FF" alt="Foundation accepted; integrations under development" />
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

> **Development milestone:** the local foundation is implemented and reviewed. Twitch/OBS connections, diagnostics and approved repairs are the next implementation stages. This is not yet the complete hackathon demo.

## Why CueMender

A viewer notices missing audio. The streamer needs to find the cause without disrupting the broadcast or accidentally revealing something private.

CueMender is being built around that interaction: receive a report through Twitch, compare OBS configuration with the streamer's declared expectations, explain the evidence, and request approval for a narrowly supported correction. Local configuration verification and viewer feedback remain separate.

## Features

The following foundation features are available now.

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

**Next:** own-channel Twitch chat, OBS discovery, three diagnostic checks, one approved microphone-track correction, and viewer confirmation. See [current scope](#current-scope).

## Demo

The current development preview lets you open private controls, save expectations, reload the page and inspect the separate idle overlay. It does not connect to Twitch or OBS yet.

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

Codex defined the architecture and execution contracts, identified foundation defects, independently verified DeepSeek's corrections, and prepared the repository identity and integration handoff.

DeepSeek implemented the foundation and its corrections. Codex is assigned the upcoming approval-and-repair engine and its tests; that implementation is not claimed as completed yet.

[Actual contribution record →](docs/execution/CONTRIBUTIONS.md) · [Foundation acceptance →](docs/execution/handoffs/P01-R01-review.md)

The required application workflow has no runtime AI dependency or paid-model requirement.

## Built with

| Runtime | Interface | Persistence | Planned integrations |
| --- | --- | --- | --- |
| Go 1.27.3 · Windows x64 | Embedded HTML/CSS/vanilla JS · SSE | SQLite via modernc · OS keyring | Twitch EventSub/Helix · OBS WebSocket |

One local process, with no separate frontend server or cloud database.

## Current scope

- **Accepted:** application lifecycle, validated persistence, local private sessions, CSRF protection, separate public/private state, embedded pages, and the corrected reload/concurrency/shutdown/error-redaction paths.
- **Next implementation:** Twitch/OBS connections, microphone-muted and track-missing checks, direct scene-item visibility check, streamer-approved track assignment, viewer feedback and isolated rehearsal.
- **Outside the MVP:** automatic unmute or source reveal, raw audio analysis, computer vision, OS-device repair, multiple channels and autonomous control.
- **Verification limits:** browser-script behavior is tested in a Node VM, not a rendered-browser visual test. Race instrumentation remains pending a configured C toolchain. Worker-reported real Ctrl+C evidence and independently repeated in-process lifecycle tests are distinguished in the acceptance record.

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
