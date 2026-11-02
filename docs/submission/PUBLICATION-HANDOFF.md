# Final publication handoff

Prepared 2026-11-02. This is a recovery checklist, not evidence that publication/submission occurred.

## Ready locally

- Revised film: `video-production/cuemender-demo/renders/cuemender-demo-erratascope-style.mp4` (165 seconds, silent1080p30; identical convenience copy `cuemender-demo-final.mp4`; receipt and SHA-256 in that folder). The user rejected the pale v1 and requested this reference-matched revision; final revised-cut approval is pending.
- YouTube text/settings: [YOUTUBE.md](YOUTUBE.md).
- Devpost story: [PROJECT-STORY.md](PROJECT-STORY.md).
- Thumbnail: `video-production/cuemender-demo/captures/youtube-thumbnail.png`.
- Devpost cover: `video-production/cuemender-demo/captures/devpost-cover.png`.
- Real screenshots: same captures folder, `real-finding.png`, `real-overlay.png`; simulated screenshots `rehearsal-proposal.png`, `rehearsal-verified.png`, `rehearsal-closed.png` must retain rehearsal labels.
- Public source: https://github.com/desanv01/cuemender
- Public Windows preview: https://github.com/desanv01/cuemender/releases/tag/v1.0.0
- Actual Codex build session: `01a191ff-ada4-7970-bf35-ae0a76942bed`.

## Saved account state

YouTube Studio channel DV: https://studio.youtube.com/channel/UCiJEhsz_yx-PJCSMycpvEDw

Devpost draft ID 1278636: https://devpost.com/submit-to/31742-codex-x-twitchcon-hackathon/manage/submissions/1278636-cuemender/project_details/edit

Project name and pitch were saved. The story and repository link were filled on the project-details page but not saved, because the video URL was still missing. Do not assume that text survives a reload. Replace the story with PROJECT-STORY.md exactly; do not append it to Devpost's template. The build tags need final verification.

## Finish after the user approves the concrete cut

1. Upload the final MP4 to YouTube; copy the title/description from YOUTUBE.md. Use Public visibility (not unlisted). Verify the ordinary software-demo audience settings. Wait for processing and verify public playback from a signed-out view. Record the actual URL.
2. Finish Devpost's draft with the story, source link, public YouTube URL, cover/screenshots and accurate build tags (Go, SQLite, HTML, CSS, JavaScript, Twitch, OBS, OpenAI Codex, where the form accepts them).
3. In additional information, provide the actual Codex build-session ID and any requested repository/testing details. The Windows preview and README include receive-only and rehearsal setup.
4. Review the final entry with the user before the final rules agreement/submission action. Confirm the project is submitted to the Codex x TwitchCon hackathon, not merely saved as a portfolio draft.
5. Record the submission receipt/time and public links. Only then close tracking issue #18. Keep #6/#10 open and accurately disclosed.

The assigned project preview URL is `https://devpost.com/software/cuemender`; accessibility/submitted state has not been confirmed. Do not treat the existence of that URL as a successful submission.

## Claims to preserve

- Real Twitch receive-only message, actual local OBS diagnostic, offline channel, no broadcast audio recovery.
- Simulated repair/feedback shown separately in rehearsal; no actual approved OBS repair demonstrated.
- Codex and DeepSeek contributions described accurately. No guarantee of organizer eligibility interpretation.
- Deadline previously verified from the hackathon page: November 3, 2026, 06:00 Malaysia time. Complete final account steps well before then.

## Current technical blocker

Background browser control failed while preparing the draft: `failed to write kernel assets: The system cannot find the path specified. (os error 3)`. This is an automation-tool failure, not an account rejection or automatic approval-review rejection. Drafts and local files remain available. No credentials are needed in chat.
