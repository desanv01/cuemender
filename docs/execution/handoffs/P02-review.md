# P02 acceptance review

Date: 2026-11-01 (Asia/Kuala_Lumpur). Reviewer: Codex, current main chat.
Worker: DeepSeek; worker stopped before review.
Scope: P02 v1.1 against CONTRACTS v1.2.
Decision: **CHANGES REQUIRED — P02 is not accepted; P03 remains held.**
Tracking: [implementation #2](https://github.com/desanv01/cuemender/issues/2), [corrections #4](https://github.com/desanv01/cuemender/issues/4).

## Findings

| Priority | Finding / actual source | Evidence and impact | Correction |
|---|---|---|---|
| High | OBS track decoding: internal/obs/obs.go InputAudioTracks; manager.go completeTracks | Actual pinned goobs against a local WebSocket protocol server returns keys "1" through "6"; completeTracks expects track1 through track6. All real maps become unknown. Existing fake uses the wrong wire shape and hides the defect. | R01 |
| High | OBS transport lifecycle: manager.go pollLoop/readObservation/Disconnect/Close | readObservation always returns nil error, including when every read fails. Reproduction still reports connected/fresh. No established-loss reconnect; wait-before-close cannot promptly interrupt SDK calls that ignore context. Initial retry tests do not prove connected-provider shutdown. | R02 |
| High | Stale identity/config binding: manager.go readDiscovery/ResolutionReady, domain/integrations.go | Discovery is fetched once, no source/scene-collection invalidation subscribed, GET discovery just returns cache. Recreated input remains resolved. Observations have no config_revision or context; settings changes can publish an old read with new settings. Private snapshot reads manager state in separate lock acquisitions. | R03 |
| High | Twitch token policy: manager.go StartRefreshLoop/RefreshNow/resumeFlow | Timed loop makes refresh calls and zero validate calls (reproduced). Refreshed identity/expiry are not verified, missing scopes replaced by old scopes. fail only changes copy; it does not stop dependent workers. Revocation is treated as ordinary reconnect. | R04 |
| High | Lifecycle/credential persistence: manager.go saveCredentials/Disconnect and app/integrations.go | Generation check and credential write are separate; a blocked refresh Set completes after successful disconnect/Delete (reproduced). Connect/Disconnect aren't serialized end-to-end, refresh loop isn't joined, several updates check generation before acquiring the mutation lock. Concurrent app connects can persist a losing configuration before returning busy. | R05 |
| Medium | OBS saved password after restart: manager.go Connect | Saved password lookup is conditioned on address == m.endpoint, which is empty in a fresh manager. Reuse fails after restart (reproduced), despite an endpoint-specific saved entry. | R02 |
| High | EventSub handover ownership: manager.go runSocket/startEventSub | runSocket replaces its local socket; caller closes the original. Replacement fake socket remains unclosed after cancellation (reproduced). Generation remains unchanged across ordinary socket replacements; queued replies are tied to manager rather than connection lifecycle. | R06 |
| Medium | Chat limits/integration seam: commands.go ReplyQueue, oauth.go SendChat, manager.go handleChat | SendChat doesn't propagate Retry-After/reset; queue ignores sender errors and sends the next item after fixed interval. No typed normalized C04 callback/event preserving actor/message/feedback ID for P03. Cooldown map has no expiry/cap. | R07 |
| Medium | Setup controls: web/control/app.js renderOBS/collectSettings/discovery handlers | Selected visual UUID is discarded on save; scene/context identity is not retained. Null discovery does not clear old selectors. obs_observation and setup resolution are never rendered, preventing the promised comparison of mute/tracks/visibility with OBS through the UI. | R08 |

The review does not accept the worker's "fixture-complete" label as completion of required fixture coverage. The report honestly records live limitations, but reflection on a read-only interface and compile-only SDK mapping are not a real outbound protocol assertion.

## Independently executed checks

Portable Go 1.27.3; CGO_ENABLED=0, GOTOOLCHAIN=local, GOPROXY=off; local caches. Loopback fixture checks used the execution tool's sandbox exception.

- go test ./... -count=1: passed all existing packages.
- go vet ./...: passed.
- gofmt -l cmd internal web: clean before adding review fixtures; new fixtures formatted separately.
- node testdata/browser/control_csrf_reload.js: passed bootstrap/reload/save/locked checks.
- node testdata/browser/p02_setup_render.js: passed its existing simulated-DOM render assertions.
- go test ./internal/app -run TestRunServesAndShutsDownWithActiveSSE -count=5 -timeout=45s: 5/5 passed. These do not establish correctness with blocked real adapter work.
- go build -trimpath -o dist/cuemender.exe ./cmd/app: passed; --version reports 0.2.0-p02.
- go test -tags reviewrepro ./internal/obs ./internal/twitch -run TestReview -count=1 -timeout=30s -v: **seven failures** listed below.

## Reproducible failing checks added by Codex

Files internal/obs/p02_review_regression_test.go and internal/twitch/p02_review_regression_test.go are tagged reviewrepro. They assert correct behavior and intentionally fail on this baseline; ordinary go test does not select them. They are evidence, not production fixes.

1. TestReviewOBSRealWireTrackMap — real SDK + local OBS WebSocket protocol response; numeric keys not normalized.
2. TestReviewOBSReuseSavedPasswordAfterRestart — fresh manager does not use the saved endpoint credential.
3. TestReviewOBSRejectsRecreatedInputDuringConnection — same-name recreated UUID still marked resolved.
4. TestReviewOBSClosedSocketNotFreshConnected — all reads fail, still connected and fresh.
5. TestReviewHourlyLoopActuallyValidates — two scheduled refreshes, zero validate requests.
6. TestReviewRefreshCannotRestoreDeletedCredential — credential present after disconnect returned success.
7. TestReviewHandoverClosesReplacementSocket — replacement socket not closed.

Protocol fixture is not an OBS installation or live Twitch session. No genuine tokens/passwords were used; fixture values are synthetic.

## External gates and limits

- P02-A/V06 remains unmet: registered Public client, authorization and separate-viewer receipt/reply evidence.
- P02-B/V07 remains unmet: enabled/authenticated OBS WebSocket and actual comparisons of all six tracks, mute, program scene, item state and broadcasting.
- Live prerequisites above are from the user/worker report, not independently reconfigured during this review. No OBS settings changed, no broadcast started and no Twitch registration performed.
- Rendered-browser validation remains unmet. Both Node checks execute real control JS in simulated DOMs.
- Race instrumentation remains unavailable in the reported toolchain; no C compiler installed.
- Atomic config failure preservation needs a real filesystem fault or an injected write/sync/replace failure, not just invalid-input validation.
- Public route middleware and projection separation are present and existing checks pass. Extend sentinel evidence to actual adapter errors/logs and new private fields in the correction.

## Provider/source verification

- OBS's [GetInputAudioTracks implementation](https://github.com/obsproject/obs-websocket/blob/master/src/requesthandler/RequestHandler_Inputs.cpp) emits numeric string keys. Local goobs v1.11.1 preserves the map unchanged; the application's boundary must normalize it.
- Twitch requires [startup and hourly validation](https://dev.twitch.tv/docs/authentication/validate-tokens/); refreshing alone is not that check.
- [EventSub lifecycle documentation](https://dev.twitch.tv/docs/eventsub/handling-websocket-events/) distinguishes requested handover, ordinary reconnect and subscription revocation.
- Pinned goobs client.go was inspected locally: network methods do not consume the application's request context; auth/request timeout and socket cleanup need explicit adapter treatment.

## Handoff decision

Dispatch one consolidated [P02-R01 correction](../corrections/P02-R01.md). Keep all worker code/evidence intact. Codex authored only review fixtures/documents in this turn; no production implementation changes. User may prepare live prerequisites in parallel, but they do not waive the required corrections. Do not merge P02 or release P03 until acceptance.
