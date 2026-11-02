# CueMender demo delivery receipt

Reference-matched revision rendered and verified 2026-11-02 UTC. It supersedes the pale first cut rejected by the user. Application release remains v1.0.0 / source fed7787; this film adds no application functionality.

## Final artifact

- Local output: `renders/cuemender-demo-erratascope-style.mp4`; identical convenience copy `renders/cuemender-demo-final.mp4`.
- Duration: 165.000 seconds (2:45), below three minutes.
- Size: 5,603,165 bytes.
- SHA-256: `59A381620B853FF4BA49A313CBF9DFDB1B19F4D2B41DAA7D84CE30406D072423`
- Video: H.264, 1920x1080, 30/1 fps, yuv420p, 4,950 frames.
- Audio: no stream, as explicitly requested.
- HyperFrames 0.8.221; FFmpeg/ffprobe 9.0.1; local GSAP 3.14.2; Inter 400/700.
- Render completed successfully in approximately 5m36s. Shutdown timeouts for render-browser cleanup did not prevent artifact assembly/validation.

## Verification

- HyperFrames full check: passed; browser executed, zero runtime/layout/motion/contrast errors. Reviewed composition-local timing warnings and intentional silence.
- ffprobe: properties above verified from the encoded output.
- `ffmpeg -hide_banner -loglevel error -xerror -i renders/cuemender-demo-final.mp4 -f null -`: exit 0, complete decode.
- Root inspected reference-matched pre-render beat/phone sheets, key seam strips and full-resolution opening/chat/proposal frames. Final encoded ten-second contact sheet and full-resolution87second proposal and146.7second Codex frames inspected: expected reference styling, readable proposal, persistent simulation labeling and no closing Codex gap. No uninterrupted human-playback claim.
- GPT-6.1 Sol high performed read-only visual and submission-claim review; root authored the composition. This does not constitute independent testing of the application implementation.
- Revision peer audit inspected all13seams, both beat/phone sheets and full chat/proposal/closure/Codex frames: no actionable P0/P1 issues. Exact reference CSS and intro/outro animation parameters confirmed. This was sampled source visual QA, not a new encoded-playback claim.
- Exported actual-frame YouTube thumbnail and 1200x800 Devpost cover into ignored `captures/`.

## Evidence and cleanup

Real viewer command traveled through Twitch's offline-channel chat into actual local OBS diagnostics. Incident CM-K8DBDW records MIC_MUTED; OBS was not broadcasting. Mic/Aux was restored to its original unmuted state and read back. Rehearsal repair and feedback are separately labeled simulated. Original authenticated EventSub and approved real OBS repair remain unverified (#6/#10).

Capture app/browser processes were stopped; OBS was left open without starting a stream. Editable preview remains available for user review. Raw recordings, runtime state, caches and final media are excluded from Git; lightweight composition and provenance are included.

## Publication state

Revised cut is ready for user review; no approval is inferred from the first-cut review, which requested changes. YouTube publication and final Devpost submission are pending. Background account-browser control encountered a kernel-assets initialization error; no publication or submission success is inferred. Follow `docs/submission/PUBLICATION-HANDOFF.md` from the repository root if browser access remains unavailable.
