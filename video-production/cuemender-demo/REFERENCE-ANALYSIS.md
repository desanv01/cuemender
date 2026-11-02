# ErrataScope reference analysis

The user rejected the pale first cut and selected their `ErrataScope_Demo_Desan_Vasu.mp4` as the exact visual/motion reference. Publication is held until the revised concrete cut is reviewed.

## What was analyzed

- Reference MP4:152.000seconds,1920x1080,30fps,H.264,4,560frames.
- FFmpeg decoded all4,560frames and computed per-frame mean-difference/scene scores. Raw metrics are local ignored production data, not manually viewed frames.
- Root visually inspected the entire four-second contact sheet and every frame of the opening's first1.2seconds. Read-only Sol6.1high audit inspected encoded samples at0,.3,.7,1.2,10,28,65and146seconds.
- The actual HTML/CSS/timeline and generator were inspected. The reviewer compared14generated outputs in memory: actual source equals generator output after removing HyperFrames IDs. This gives exact geometry and animation timings rather than estimates from thumbnails.
- No claim that a human visually watched all4,560individual full-resolution frames. Mechanical frame analysis plus dense motion inspection and source comparison establish the reference behavior.

## Copied design system

`assets/reference-theme.css` is the user's original source CSS, with the same local Inter font assets. The CueMender generator adds only media containment for already-cropped assets and block lines in title text.

Navy#0d151c; primary#f8f5ee; cyan#65d2f4; simulation amber#edbf65; captions#e6e9ed; detail#ced7df; borders#344956. Exact coordinates/type sizes are in SPEC.md. Full footage uses the reference1728x864panel; split focus uses its700pxtext and824pxmedia columns. Media content remains genuine CueMender footage; ErrataScope's product content is not reused.

Intro movement is y24→0 over0.55s power2.out, with subtitle opacity/y16 reveal at0.6–1.1s. Closing movement is y18→0 over0.5s. Every other reference timeline is empty. The revision preserves those direct cuts and reading holds, rather than adding unobserved effects.

## Content adaptations

- CueMender name, creator, claims, repository and real source recordings replace ErrataScope content.
- Cyan identifies real chat/read-only evidence; amber identifies simulated rehearsal.
- Exact reference layout is retained, but source media keeps its own aspect ratio; no stretching or fake UI.
- Fourteen scenes total165seconds, versus the reference's twelve scenes/152seconds. The extra time preserves required Codex footage and capability boundaries under the same three-minute maximum.
- Silence remains explicitly requested. No voice or music.

Original ErrataScope files were read only and left unchanged. The superseded pale render is retained locally as `renders/cuemender-demo-v1-superseded.mp4`; it is not approved for publication.
