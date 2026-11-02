# CueMender repository presentation

User preference recorded 2026-11-01: follow the README, About and overall presentation of the user's ErrataScope repository.

## Inspected reference

- Repository: https://github.com/desanv01/ErrataScope
- README: https://github.com/desanv01/ErrataScope/blob/main/README.md
- README blob inspected: 9d5bfd96f69a64c3feb5e2c356ef997e2bebdf23
- Read via GitHub connector: README source, repository About metadata and root directory listing.
- This was a structural/source inspection, not a claim of screenshot-based visual comparison.

## Observed style to carry over

1. Centered product title, short bold tagline and one-sentence description.
2. Centered hackathon attribution and author line.
3. Compact badge row and jump links near the top.
4. Wide actual-product screenshot with an informative provenance caption.
5. Short "Why" explanation of the user problem.
6. Two-column feature table, with concise feature headings and concrete behavior.
7. Demo section with an ordered walkthrough and a two-column screenshot gallery.
8. Practical getting-started instructions, followed by deeper technical details.
9. Dedicated section explaining the relevant development/provider technology.
10. Compact stack table, explicit current scope, documentation links, feedback and license/notice.

ErrataScope's About describes the product, key technology and hackathon in one line. Its inspected metadata had no homepage URL and no topics; those are observations, not requirements to leave useful CueMender metadata empty.

## CueMender adaptation

Use the same presentation pattern with original CueMender content:

- Title: CueMender.
- Suggested tagline: "Hear the report. Check the evidence. Approve the fix."
- Description: community-assisted Twitch/OBS configuration diagnostics with explicit streamer approval.
- Attribution: Codex x TwitchCon Hackathon 2026; author Desan Vasu.
- Jump links: Features, Demo, Get started, Built with Codex, Testing guide. Add video/download links only when the actual targets exist.
- Hero: real private incident panel and/or public overlay, with a clear caption identifying live or rehearsal footage.
- Feature grid: community reports, evidence-based checks, deliberate approval, one narrow correction, viewer feedback and isolated rehearsal. Publish each as implemented only after its acceptance evidence exists.
- Demo: actual report -> observation -> approval -> read-back -> viewer feedback. Gallery can show evidence, approval, public outcome and clearly labeled rehearsal.
- "Built with Codex": accurate Codex and DeepSeek roles, with links to real contribution/verification records; runtime AI is not implied.
- Stack table: Go runtime, embedded vanilla browser UI, SQLite/keyring, Twitch EventSub/Helix and OBS WebSocket.
- Current scope: three diagnoses, exactly one supported repair, Windows target, local observation limits and remaining unverified gates.
- Documentation: installation/testing, troubleshooting/limitations, architecture and release/demo evidence.
- Feedback: actual cuemender issues URL after repository creation.
- License/notice: reflect the license actually selected for CueMender and its dependencies. ErrataScope's MIT badge is not evidence that CueMender already has a LICENSE file.

## About and metadata

At the initial foundation-only publication, use an honest development description, for example:

> Go foundation for community-assisted Twitch/OBS diagnostics | Codex x TwitchCon Hackathon 2026

When the required workflow is implemented and verified, update to:

> Local Twitch chat-to-OBS diagnostics and isolated repair rehearsal | Codex x TwitchCon Hackathon 2026

Suggested relevant topics: twitch, obs-studio, golang, streamers, stream-overlay, diagnostics, codex, hackathon. Use a verified project/demo URL for the homepage when one exists; no placeholder website.

Badges must report actual status/tooling. Do not use a passing-CI, release-ready or license badge without the corresponding evidence/file. Do not copy ErrataScope's model/provider badges, screenshots, download commands or claims.

## Repository organization

Follow ErrataScope's readable separation of source, docs, assets, fixtures and demonstration evidence, adapted to this Go application:

- Keep cmd, internal and web as specified in ARCHITECTURE.
- Keep execution/review material in docs/execution and factual environment notes in docs/implementation.
- Place actual approved screenshots and their provenance notes in docs/assets/demo when captured.
- Link end-user/release instructions from the root README; avoid making the reader navigate worker packets to run the application.
- Keep test fixtures in testdata and packaging helpers in release or the agreed scripts location.
- Keep raw screen recordings, toolchains, runtime databases and credentials outside Git. A short video review/metadata document may be tracked.
- Do not create empty Python/React/infra directories merely to imitate ErrataScope's different stack.

## Execution checkpoints

1. After P01 acceptance, Codex applies the centered README/header/navigation structure and accurate foundation status during public repository setup. Preserve working build/run instructions.
2. As features are accepted, update the feature/scope descriptions and About text through normal Git/GitHub changes.
3. During P05/P06, add real screenshots/gallery, demonstrated workflow, final quickstart and actual demo/release links.
4. Before submission, inspect rendered GitHub Markdown, image paths/alt text, jump links, About, topics and badge destinations against this reference.

This preference does not expand P01-R01's correction scope or change its current ownership. Codex owns repository presentation; DeepSeek follows this brief when a released packet explicitly includes documentation/presentation work.
## Foundation publication

Codex applied the centered title/tagline, truthful badges, jump navigation, feature grid, build/run instructions, contribution explanation and documentation navigation. Actual screenshots, video and release links remain deferred until captured/published. No borrowed ErrataScope assets, false CI badge or unselected license badge were added. Public repository: https://github.com/desanv01/cuemender.
