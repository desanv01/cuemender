# P02-R02 acceptance review

Date: 2026-11-01. Reviewer: Codex. Outcome: **changes_required**.
Reviewed P02 v1.1 / CONTRACTS v1.2 against P02-R02 v1.0, actual source, worker evidence and local fixtures. DeepSeek had stopped; Codex took ownership for review fixtures/documents only. No production implementation was changed in this review. P03 remains held.

## Confirmed progress

All fourteen earlier review assertions pass, including wire-track normalization, saved-password reuse, candidate closure during initial reads, initial observation revision rejection, token duration, validated rotated-pair retention after persistence failure, and queue rejection after close. The ordinary Go suite, both actual-JavaScript Node fixtures, vet, formatting, CGo-free build and five active-SSE shutdown runs also pass. UI identity preservation and deterministic config replacement failure coverage are present.

Those results do not cover all R02 requirements. Six additional tests reproduce concrete failures in the paths the worker marked incomplete. These are offline failures; no Twitch account or OBS installation is needed to fix them.

## Reproduced blockers

| Priority / requirement | Source | Reproduction and impact |
|---|---|---|
| P1 — B: cancellation during OBS handshake | internal/obs/obs.go:50 | `TestReviewR03OBSHelloCancellation`: after a successful WebSocket upgrade, cancel while OBS Hello is withheld. Connect remains blocked until the SDK's five-second authentication timeout. The candidate is not available to manager Disconnect/Close during this stage. |
| P1 — C: readiness freshness | internal/obs/manager.go:622 | `TestReviewR03OBSStaleCannotBeReady`: advance the injected clock beyond staleAfter. SnapshotBundle returns `Stale=true` and `ready=true` together. Later diagnostic/monitoring readiness must not trust this evidence. |
| P1 — C: unknown collection boundary | internal/obs/manager.go:397 | `TestReviewR03OBSUnknownStartContextStaysUnknown`: the first collection read fails, the final one succeeds; discovery nevertheless returns `CollectionKnown=true`. The final read cannot establish the context of earlier identities. Also, Inputs/Scenes currently run before the first context read. |
| P1 — C: explicit refresh revision | internal/obs/manager.go:740 | `TestReviewR03OBSRefreshCannotRelabelRevision`: pause Inputs under revision 1, save revision 2, then finish the read. The old operation returns and installs discovery labelled revision 2. Both readDiscovery and RefreshDiscovery assign the revision after work has begun. |
| P1 — D: requested handover | internal/twitch/manager.go:464 | `TestReviewR03HandoverProcessesOldNotifications`: withhold replacement welcome and send an old-socket command. It is not processed. runSocket blocks its only read loop on replacement Dial/Read; after welcome it closes the old socket without consuming that queued notification. |
| P1 — D: disconnect ownership | internal/twitch/manager.go:262,852 | `TestReviewR03DisconnectCancelsMaintenance`: a local validation request remains active after Disconnect returns. maintenance uses manager-lifetime m.ctx, not authorization-lifetime context, and Disconnect does not join this work. |

New fixtures: `internal/obs/p02_r02_review_test.go` and `internal/twitch/p02_r02_review_test.go`, currently gated by `reviewrepro`. They have bounded fixture cleanup. Promote equivalent coverage after fixing behavior; do not weaken assertions.

## Additional source findings and incomplete evidence

These are source inspection or coverage findings, not additional claims of experimentally reproduced races:

- R02 B still has unjoined SDK read/close goroutines in callRead (obs.go:81). RefreshDiscovery has no admission/concurrency bound or registration in manager worker shutdown. OBS Close lacks the lifecycle serialization used by Connect/Disconnect. A returned read error is not proof of socket/worker cleanup.
- R02 C still uses pointer-containing `fmt.Sprintf("%#v", expectations)` for equality (manager.go:594). Discovery revision is captured after reads, so the poll check also cannot prove operation-start provenance. SnapshotBundle's readiness predicate lacks connection status, stale age and discovery revision/generation checks. The six tests are examples, not the entire predicate specification.
- R02 D report says Disconnect uses lifecycleMu, but actual Twitch Disconnect does not. handleChat checks generation before taking the later receipt mutation lock; runSocket increments connGen without a same-lock authorization check. maintenance/RefreshNow retain m.creds pointers across unlocked work. ReplyQueue's sender uses current credentials without checking its captured generation, and the queue is parented to m.ctx. These require the existing concurrency barriers, not a claim that all lifecycle paths are serialized.
- Startup resume's second Validate error is still unconditionally treated as authorization invalid (manager.go:381); a transient error is not evidence of revocation. Short-token tests call maintenance directly; they do not prove the scheduling loop. Required startup changed-subject/expired-rotation and transient-path cases remain incomplete.
- Retry-After seconds are propagated, but oauth.go:462 has no applicable remaining/reset-header handling; reconnect backoff is never reset after stable recovery. Both were already specified in R02 D.
- The config failure test proves prior file bytes survive. It does not yet prove the actual setup handler preserves cache and avoids publishing connection success on that failure. Blocked-provider production-app shutdown, startup credential seam, delayed real-socket handover and slow-read boundaries remain required local evidence.

## Commands and results

Environment: Go 1.27.3 windows/amd64 at `.tools/go1.27.3/go`; Node v24.14.1. Workspace-local GOPATH/GOMODCACHE/GOCACHE/GOTMPDIR; GOTOOLCHAIN=local, GOPROXY=off, CGO_ENABLED=0. Local networking checks ran with loopback access outside sandbox restrictions.

| Check | Actual result |
|---|---|
| `go test ./... -count=1 -timeout=90s` before new tagged fixtures | exit 0, all packages |
| `go test -tags reviewrepro ./internal/obs ./internal/twitch -run TestReview -count=1 -timeout=60s -v` before new fixtures | exit 0, fourteen passed |
| `go vet ./...`; `gofmt -l cmd internal web` | clean |
| `node testdata/browser/control_csrf_reload.js` | exit 0; failures=[] |
| `node testdata/browser/p02_setup_render.js` | exit 0; failure=null |
| `go test ./internal/app -run TestRunServesAndShutsDownWithActiveSSE -count=5 -timeout=60s` | exit 0, five runs |
| `go build -trimpath -o dist/cuemender.exe ./cmd/app` with CGO_ENABLED=0 | exit 0; 19,709,440 bytes |
| Final `go test -tags reviewrepro ./internal/obs ./internal/twitch -run TestReview -count=1 -timeout=30s -v` | exit 1; fourteen earlier assertions pass, all six new tests fail as above |

The first maintenance test attempt had incomplete injected endpoint configuration and timed out; corrected fixture endpoints before the final complete run. That attempt is not evidence of a product defect. Fixture credentials are synthetic. No real authorization was performed. New review files were gofmt-formatted. This review did not change production code, install dependencies, or perform OBS writes.

## Decision and next step

Dispatch [P02-R03](../corrections/P02-R03.md), which consolidates the remaining R02 work. Keep fixed behavior and complete the boundary paths, rather than implementing only the six new assertions. Historical worker claims are preserved; append corrected evidence instead of deleting history.

P02-A/V06 (registered Twitch Public client, own-channel authorization and separate viewer) and P02-B/V07 (live OBS discovery/six-track reads) remain unmet. No rendered browser verification was done. Race instrumentation remains unavailable because gcc is absent. Ordinary active-SSE shutdown is not blocked-provider shutdown evidence.

P02 remains on feature/p02-live-connections, unaccepted and unmerged. Existing issue #4 tracks corrections; issue #2 tracks the whole packet. No accepted checkpoint/PR/merge is appropriate yet. All test commands completed; no application server was left running. P04 recording checkpoint remains unchanged.

GitHub issue #4 updated with this outcome: https://github.com/desanv01/cuemender/issues/4#issuecomment-6558646174. Review document links/fences, new-file formatting and tracked diff whitespace checked; no errors. Final process check found no go, cuemender, obs.test, twitch.test or app.test processes running.
