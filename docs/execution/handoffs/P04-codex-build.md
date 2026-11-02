# P04 Codex implementation record

Status: in progress, 2026-11-02. Starting baseline: P03 merge `01a6e3d68cd919b5e90e80bc96e5d61e8c0204e4`. Branch: feature/p04-approved-repair. User explicitly requested Codex takeover, actual screen recording first, and GPT-6.1 Sol high subagents. DeepSeek remains stopped. See D32 and STATUS for writer ownership. No acceptance claim yet.

## Actual recording checkpoint

Used the computer-use skill to inspect existing windows without activation, then the existing FFmpeg installation as the background recorder. No OBS configuration, audio track, streaming state or system setting was changed. FFmpeg window GDI capture produced black frames and was rejected. Initial Windows Graphics Capture received no frames until the user restored the chat. The stopped failed tests are not build evidence.

After the user replied ready, Windows Graphics Capture produced readable actual Codex main-chat frames: H.264, 1920x1034, 15 fps, 7.734 seconds, silent. Inspected extracted frame visually. Test file: `.tools/p04-recording/test-wgc-ready.mkv`; frame: `.tools/p04-recording/test-wgc-ready.png`. Audio is deliberately absent; narration can be added later. Recordings are ignored and never uploaded automatically.

Actual build recording: `.tools/p04-recording/p04-build-01.mkv`, started before production edits, using `gfxcapture` against the observed Codex window handle. Maximum duration one hour, no microphone/desktop capture. Keep this chat restored/unminimized. Capture correctness is checked from saved frames, not inferred from process success. Stop and verify completed footage before claiming the final artifact. Raw footage includes actual tool/agent activity; do not reconstruct or label unrecorded work as original footage.

## Allocation and verified preflight

- Root: planner, recording operation, contract decisions, integration verification and Git/GitHub.
- GPT-6.1 Sol high p04_obs_audit: read-only SDK audit, then sole writer for the narrow OBS adapter and protocol tests.
- GPT-6.1 Sol high p04_contract_audit: read-only app/coordinator/store design during the first writer's work; implementation only after explicit transfer.
- SDK audit: existing nonnil map serializes explicit false values. OBS requires numeric string keys 1 through 6; domain keys track1 through track6 must be converted. No SDK patch is pre-authorized or indicated.
- Preserve active-monitoring repair gate. While Twitch is deferred, fixture implementation may proceed, but full live product approval may remain unavailable. Do not bypass readiness or mislabel the real gate.

Implementation changes, tests, contribution provenance, recording stop/verification and useful clip timestamps will be appended as completed. Codex-authored code receives self-verification and separately described peer inspection; neither is a claim of independent external acceptance.
