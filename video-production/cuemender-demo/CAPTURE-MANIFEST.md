# Capture and evidence manifest

Accepted application: v1.0.0, release/source fed7787. Captured 2026-11-02 UTC in isolated disposable data directories. Real and rehearsal use separate processes, ports and databases. Headless agent-browser 0.38.1 uses a private Windows desktop; no native window activation. OBS remains not broadcasting.

## Genuine integration
- Channel desanv10; separate viewer dvx6901 manually sent `!issueaudio` at user request.
- User explicitly approved temporary Mic/Aux mute. Original false; prepared true using OBS WebSocket; restored false and read back after diagnostic.
- Incident CM-K8DBDW, evidence timestamp 2026-11-02T12:49:30.482Z, revision 1, origin viewer, one distinct report, MIC_MUTED, local streaming false.
- `real-report.mp4`: 110.766667s at 30fps. Genuine incoming result appears near 77s. `real-arrival.mp4` is source 70–90s, crop only.
- `real-result.mp4`: source 85–110s. `finding-detail.mp4` enlarges the actual recorded incident. It is historical evidence of the prepared mute, not a claim that the mic remains muted after restoration.
- `twitch-proof.mp4`: 15s, 15fps, real Twitch page after receipt. `chat-detail.mp4` enlarges the actual header/message; it does not replay typing or fabricate arrival.
- First simultaneous Twitch recording failed encoder-lag check and produced no usable file. No success claim for that take.
- `real-overlay.mp4`: actual matching public incident; crop in overlay-detail. Local preview, not footage of a broadcast overlay.

## Rehearsal
- Incident CM-YPQWDP, revision1, simulated track-missing scenario. Actual product UI approved simulated track addition, readback verified, simulated !fixed vote, viewer_confirmed closure.
- `rehearsal-approve.mp4`: 13.666667s,15fps; `rehearsal-feedback.mp4`:13.7s,10fps. Detail versions crop actual pixels.
- Initial rehearsal report recording failed encoder-lag; authentic proposal screenshot provides the clearly documented still hold. Closure screenshot is also an authentic still hold.
- `proposal-detail.png` crops the actual proposal screenshot to x285,y620,w1040,h350 for the readable 82–92s hold. No proposal text was redrawn.
- No real provider/keyring connection or OBS write from rehearsal.

## Codex
- Original `.tools/p04-recording/p04-build-01.mkv`:2249.867s,1920x1034,15fps,silent.
- Clips: start source50–58s; work895–904s; debugging2180–2187.533s. Crop x360,y70,w1110,h930 removes unrelated sidebar. Originals unchanged.
- `codex-debug-hold.mp4` pads the final debugging frame to an eight-second clip (about 0.47s hold), avoiding an empty final half-second. It does not invent additional work.
- Actual build session verified through Codex app records:01a191ff-ada4-7970-bf35-ae0a76942bed.
- Final browser/race validation occurred after original recording ended. Captions describe genuine implementation/debugging, not unseen final results.

## Rights and processing
Application/source content belongs to this project; Twitch/OBS/Codex interfaces shown solely as genuine functional evidence. Fonts Inter under its bundled open font license; GSAP 3.14.2 local runtime copied from the user's MajuPilot assets. No third-party music or stock footage. Derived clips use FFmpeg9.0.1 and libx264; no frame content fabricated. Raw clips and runtime data excluded from Git.
