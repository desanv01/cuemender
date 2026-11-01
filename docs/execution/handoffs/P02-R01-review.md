# P02-R01 re-review

Date: 2026-11-01, Asia/Kuala_Lumpur. Reviewer: Codex, current main chat.
Decision: **CHANGES REQUIRED**. P02 not accepted; P03 held.
Scope: P02 v1.1 / CONTRACTS v1.2 / P02-R01 v1.0.
DeepSeek stopped. Codex changed review tests/documents only, no production code.

## Verified progress

The seven original Codex review regressions are now ordinary tests and all pass independently. The correction fixes the real OBS wire map, fresh-manager password reuse, basic source recreation detection, all-read-failure status, hourly validate invocation, late credential persistence after disconnect, and final handover-socket closure.

Also independently passed before adding new review cases:
- go test ./... -count=1 -timeout=60s
- go test -tags reviewrepro ./internal/obs ./internal/twitch -run TestReview -count=1 -timeout=30s -v
- go vet ./...
- gofmt -l cmd internal web
- node testdata/browser/control_csrf_reload.js
- node testdata/browser/p02_setup_render.js (including savedIdentity=true)

The ordinary suite includes protocol/read-only allowlist and reconnect fixtures added by DeepSeek. Passing these does not establish every R01 requirement. The later review-only cases below deliberately fail; do not summarize the final tagged suite as passing.

## Reproduced remaining failures

New opt-in review fixtures: internal/obs/p02_r01_review_test.go and internal/twitch/p02_r01_review_test.go, build tag reviewrepro.

Command: go test -tags reviewrepro ./internal/obs ./internal/twitch -run TestReviewR02 -count=1 -timeout=30s -v

| Check | Result on handed-back code | Source / consequence |
|---|---|---|
| TestReviewR02MaintenanceExpiryUnits | Failed: 3600 seconds becomes 801362h40m26.63743488s instead of 1h | OAuth.Validate already returns time.Duration; maintenance multiplies it by time.Second again. Token refresh timing becomes wrong. |
| TestReviewR02RotationSaveFailureUsesRotatedPair | Failed | RefreshNow reports persistence degraded but returns before installing the validated rotated pair. The established session keeps the old pair. |
| TestReviewR02ReplyQueueRejectsAfterClose | Failed | Enqueue never checks closed/cancelled state. |
| TestReviewR02ActualSDKReadCancellation | Failed; fixture took about 5 seconds | Pinned GoobsDialer/backend with a stalled local protocol response ignores cancelled request context. Even manual close did not finish the request until its SDK timeout in this run. |
| TestReviewR02DisconnectClosesInitialReadTransport | Failed | Manager publishes m.backend only after Version/discovery/observation; Disconnect has no handle to close during that initial work and waits. Same structure exists on reconnect. |
| TestReviewR02InitialReadRejectsOldSettings | Failed: revision 1 observation exposed under revision 2 | Initial connect and reconnect installs lack the revision guard used by pollLoop; SnapshotBundle still returns the mismatched observation. |
| TestReviewR02UnknownCollectionCannotResolve | Failed | readDiscovery ignores SceneCollection error; empty collection then bypasses context validation and reports ready. |

Tests use only synthetic data and local/fake providers. They do not exercise a real OBS installation or Twitch account.

## Incomplete R01 items established by source inspection

- Twitch manager lifecycleMu is taken in Connect/StartSaved but not Disconnect/Close. Several async installs still check current(gen) before, rather than within, their mutation lock. Maintenance uses manager context and mutable shared credentials; disconnect doesn't cancel/join it as connection work.
- Startup resume overwrites the original credential struct after refresh, losing the original subject binding before its comparison. Device/resume/rotated validation do not all reject nonpositive expiry. A generic network validation failure is treated as terminal; short token lifetime scheduling still depends on an hourly ticker.
- SendChat's 401/403 is swallowed by ReplyQueue rather than forwarded to terminal authorization handling. Queue Close does not join its sender; expired-head handling can strand remaining work. Applicable rate-limit reset/remaining headers are not handled.
- Requested EventSub handover still synchronously dials/waits for replacement without reading old notifications. Source/queue generation suppression and backoff reset remain incomplete. Connected is still published during device/resume before EventSub is usable.
- OBS initial/reconnect transport ownership and failed authentication cleanup are not solved by close-before-wait on the already-published backend. Request context checks do not interrupt SDK calls.
- Discovery context is read only after other identity reads; mixed collection snapshots are not rejected. Visual scene UUID is discovered but not bound in persisted expectations. Snapshot readiness does not reject stale time itself; name-only references can be accepted after context loss.
- UI discovery signature includes captured_at; every poll rebuilds the selectors and loses selections. Manual name edits retain old hidden identities. Private readiness and observation timestamp are still not rendered; partial text can overwrite stale warning.
- Required blocked-provider, old-slow-read, real coder socket, concurrent transition and filesystem failure tests remain absent. Config retries are not evidence of failure preservation.

These are unfinished requirements of the first correction, not new feature scope. See P02-R02 for a grouped completion plan.

## Design decision and live gates

Codex approves manual OBS Connect after process restart for this MVP. The default endpoint does not prove intentional setup. Saved exact-endpoint password reuse works; do not add a new startup preference merely to automate connection. Record this as D25; it is not an acceptance blocker.

P02-A/V06 and P02-B/V07 remain unmet: Twitch Public client/own authorization/separate viewer and live OBS evidence. No registration, real authorization or OBS configuration was performed by Codex here. Rendered-browser testing and race instrumentation remain unverified; both Node results are simulated-DOM actual-script tests. No compiler installed.

## Next action

Dispatch [P02-R02](../corrections/P02-R02.md). Existing issue #4 remains open for correction completion; issue #2 remains open for overall P02. Do not merge P02 or release P03. Preserve completed R01 fixes and history; append the new worker evidence rather than replacing old claims without a correction note.
