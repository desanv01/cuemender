# Product specification

Revision: 1.1 • Authority: Codex, approved direction from user.

## Product promise

A streamer runs one local Windows executable. Their community reports an audio or video issue through Twitch chat. The application checks the broadcaster's declared OBS expectations, explains supported findings, offers one explicit correction when applicable, and distinguishes a local change from viewers reporting recovery.

Initial target: Windows 11 x64, one broadcaster's own Twitch channel, one local OBS instance using obs-websocket v5. Exact tested OBS and dependency versions are recorded by the worker. CueMender is the user-selected public name; existing livecheck technical identifiers will be renamed consistently by Codex outside the P01 correction.

## MVP requirements

| ID | Required behavior | Observable outcome |
|---|---|---|
| M01 | Launch without Go/Node on end-user machine | Embedded pages work from release executable |
| M02 | Explicit own-channel Twitch authorization | Real channel chat commands reach the application |
| M03 | Authenticated local OBS connection | Correct inputs, tracks, mute state, program scene and scene item visibility are read |
| M04 | Declare microphone, stream track 1–6, optional visual expectation | Validated settings survive restart |
| M05 | Explicit monitoring start/stop | Stopped monitoring accepts no new viewer incident work or repair approval |
| M06 | Audio/video reports group into incidents | At most one active incident per category per monitoring session |
| M07 | Three supported checks | Evidence and limitations match the actual OBS observations |
| M08 | Exact, expiring private approval | Only the supported track-add action can execute |
| M09 | Fresh read, narrow write, read-back | Preserves the other observed track assignments and reports actual outcome |
| M10 | Viewer feedback by incident ID | Local verification and viewer confirmation have separate fields |
| M11 | Private control UI and quiet public overlay | Broadcast sees sanitized progress, never private evidence |
| M12 | Reconnection/restart handling | Stale proposals cannot execute and uncertain writes are reconciled |
| M13 | Labeled rehearsal | Fake observations cannot reach live Twitch or OBS writes |
| M14 | Reproducible setup and demonstration | Clean-folder Windows release and real Twitch/OBS workflow can be shown |

## Diagnosis rules

- **MIC_MUTED:** the selected expected microphone is currently muted. Explain and give manual instructions. Never unmute automatically.
- **MIC_TRACK_MISSING:** that microphone's assignments exclude the track the broadcaster confirmed for streaming. Show the selected track and observation timestamp. Offer only ADD_MIC_TO_TRACK.
- **VISUAL_ITEM_HIDDEN:** a directly configured scene item is disabled in the configured scene while that scene is the current program scene. Give manual instructions. Never reveal content automatically.
- If the relevant scene is not active, the source cannot be resolved, evidence is stale, a read failed, or the expectation is not configured, explain that limitation instead of inventing a finding.
- MVP visual scope is direct scene items. Nested scenes/groups and transitions may obscure visibility; do not claim exhaustive rendering diagnosis.
- Mute and track mismatch may both be found. Fixing the track cannot be described as fixing all audio.

Statuses of evidence: conflict_found, no_supported_conflict, insufficient_information, unavailable. These describe checks, not probabilities or listener audio quality.

## Twitch interaction

| Input | Behavior |
|---|---|
| !issue audio | Open/join current audio incident and schedule bounded diagnostic work |
| !issue video | Open/join current video incident |
| !fixed ID | Record that distinct viewer's current positive feedback for an eligible incident |
| !stillbroken ID | Record that viewer's current unresolved feedback |
| !streamhelp | Rate-limited command explanation |

Normalize whitespace and command case; do not treat arbitrary sentences as commands. Group repeats, deduplicate deliveries, apply cooldowns, ignore the authenticated sending account's messages, and never interpret chat as approval. Manual checks remain available when monitoring is stopped, labeled manual/offline; repair approval requires an explicitly active local monitoring session.

Default policy values and feedback semantics are specified in CONTRACTS, not inferred from frontend copy.

## Limits and exclusions

Local OBS observations do not prove what Twitch viewers hear or see. The chosen stream track is a user-declared expectation, not automatic proof of the active encoder's routing. Show and reconfirm it; invalidate proposals when relevant configuration changes.

No raw audio, speech analysis, CV, OS-device repair, automatic unmute/reveal, OBS plugin, multi-channel service, autonomous agent control, cloud database, Twitch Extension, desktop wrapper, autostart service, mixed backend or required paid AI service.

No undo in required scope. No runtime AI in required scope. No broad source-control commands. No copying another hackathon entry.

## Completion bar

All M01–M14 have mapped evidence in VERIFICATION. A clear blocker remains a blocker even near the deadline. The user can choose a smaller submission after seeing the tradeoff; workers cannot silently remove required features or mislabel simulation as live success.

Use original/authorized assets. Maintain truthful Codex and DeepSeek contribution records for the entry. See SUBMISSION for the final external checklist.


## D37 reduced submission scope

Optional receive-only Twitch chat is now supported per [C12](ANONYMOUS-CHAT.md). It supplies real chat-triggered local OBS diagnostics without developer-app registration, with explicit channel confirmation and unverified ownership. All OBS corrections are manual; no automated replies or repair executor is available in this path. EventSub and labeled rehearsal remain. Real offline chat evidence does not establish streamed audio recovery; original authenticated/approved-write gates remain unmet.
