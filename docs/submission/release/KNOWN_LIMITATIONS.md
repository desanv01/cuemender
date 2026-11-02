# Known limitations and attribution

**Codex x TwitchCon Hackathon 2026** preview, v1.0.0.

- Real receive-only Twitch chat and fresh local OBS reads were verified using an offline channel. This is not broadcast audio recovery.
- Receive-only mode has no chat sender, no OAuth channel-ownership proof and no real repair permission.
- Authenticated EventSub live verification remains open in [#6](https://github.com/desanv01/cuemender/issues/6). Real private-approval OBS repair remains open in [#10](https://github.com/desanv01/cuemender/issues/10).
- Rehearsal's providers, repairs and feedback are simulated and explicitly labeled. They cannot contact real Twitch/OBS.
- Checks are limited to microphone mute, confirmed-track membership and configured direct scene-item visibility. Nested-source, wrong-scene, stale and partial evidence may remain unknown or limited.
- The sole implemented correction is selected microphone → confirmed track after private approval and fresh checks. No automatic unmute, source reveal, stream control or operating-system repair exists.
- OBS has no atomic compare-and-set against other clients. Read-back checks local configuration; they cannot prove the audience hears audio.
- The Windows ZIP is unsigned and was fresh-extraction tested on the development machine, not a clean VM. No passing CI badge or separate clean-machine claim is made.
- An earlier intermittent race assertion was followed by focused repetitions and a clean full rerun; see execution records for exact checks and remaining uncertainty.
- Mobile reflow, keyboard-focus and measured-contrast checks were performed during P05 closure; native browser200% zoom and a comprehensive screen-reader audit were not.

Original application code is [MIT licensed](../../../LICENSE). Dependency and bundled upstream notices are in [release notices](../../../release/THIRD_PARTY_NOTICES.md). Fonts, GSAP and genuine interface footage have separate [video notices](../../../video-production/cuemender-demo/ASSET-NOTICES.md). No credentials or runtime databases are distributed.
