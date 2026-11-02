# CueMender setup and judging walkthrough

Prepared for **Codex x TwitchCon Hackathon 2026**. Use the public [v1.0.0 Windows x64 preview](https://github.com/desanv01/cuemender/releases/tag/v1.0.0). The release is unsigned; extraction/launch was verified on the development machine, not a separate clean VM.

## Install

1. Download the ZIP and `SHA256SUMS.txt` from the release.
2. Compare `Get-FileHash .\cuemender-windows-x64-1.0.0.zip -Algorithm SHA256` with the published checksum.
3. Extract into a user-writable folder. No administrator install, Go, Node, Docker or cloud account is required for rehearsal.
4. Run the executable from PowerShell so you can see the private launch link and stop it with Ctrl+C. Keep that link private.

## Account-free rehearsal

```powershell
.\cuemender.exe --mode rehearsal --no-browser --data-dir ".\judge-data"
```

1. Open the printed one-time private control link. Confirm the REHEARSAL label.
2. Select and apply the track-missing scenario; start monitoring.
3. Simulate a viewer report. Inspect the selected-track finding and the exact proposed track addition.
4. Approve in the private UI. Check the simulated read-back result; it is not a real OBS repair.
5. Simulate positive or negative feedback, then choose an eligible closure outcome and close the incident.
6. Inspect the public overlay URL from Setup. Rehearsal remains visibly marked.
7. Other scenarios cover a muted microphone, wrong scene and uncertain write. Unsupported corrections stay manual; uncertain outcomes are not automatically replayed.

Rehearsal stores data only in the `judge-data\rehearsal` child and constructs no real Twitch/OBS manager or OS keyring. Stop with Ctrl+C before changing launch mode.

## Real receive-only diagnostic

Requires OBS with its authenticated WebSocket server enabled and access to the selected Twitch channel's chat. No developer app or OAuth token is required by CueMender's receiver. Use a separate viewer account for reports; comply with the channel's chat restrictions.

```powershell
.\cuemender.exe --receive-only-channel YOUR_TWITCH_LOGIN --no-browser --data-dir ".\judge-data"
```

In Setup, explicitly confirm that this chat is intended to report on your local OBS. Connect chat and OBS, select the correct microphone and expected scene/item, and confirm the actual streaming track. Start monitoring. Send `!issueaudio` or `!issuevideo` from the separate viewer and inspect the fresh evidence.

OBS need not broadcast for a local configuration check. Do not deliberately disrupt an unrelated live production to reproduce the demo. A report from somebody else's channel would still diagnose the local operator's OBS, not that other streamer's setup.

This mode sends no chat replies, cannot authenticate channel ownership, and disables all real repairs. A public incident code supports eligible viewer feedback; local evidence and viewer feedback are different claims.

## Authenticated path and limits

The separate authenticated implementation needs a registered Twitch Public client, broadcaster authorization and OBS. Its real verification is pending in [#6](https://github.com/desanv01/cuemender/issues/6); the actual approved track-add trial is pending in [#10](https://github.com/desanv01/cuemender/issues/10). Local protocol tests and rehearsal do not close those gates. [Detailed integration setup](../../implementation/P02-setup.md)

## Restart, isolation and troubleshooting

- Use Ctrl+C for shutdown. Each effective data directory permits one process only, even when another port is selected.
- Restarting prints a new private launch link; OBS Connect after restart is explicit. Existing same-mode data persists.
- Default port:8787. Use `--listen 127.0.0.1:8788` if occupied.
- Rehearsal, receive-only and authenticated data must remain separate. Do not copy databases between modes.
- If OBS connection fails, verify the server is enabled and the endpoint/password match. Keep the password out of reports.
- If evidence is unknown, verify current scene/source identity, collection and track confirmation; do not treat unknown as healthy.
- In receive-only mode, no automated chat reply is expected. The incident appears in private controls/public projection.
- To reset a trial, stop the app first and remove only its known trial directory after retaining anything needed.

[Development checks](../../DEVELOPMENT.md) · [Known limitations](KNOWN_LIMITATIONS.md) · [Release verification](../../execution/handoffs/P06-review.md)
