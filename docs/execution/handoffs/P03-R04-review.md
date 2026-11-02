# P03-R04 acceptance review

2026-11-02 (Malaysia). Codex outcome: **changes_required**.

Worker checkpoint: `e980d086247f4a03ad31e8b05e99cf6ab54f2b00`. Codex authored review tests/docs only; no production fixes. DeepSeek stopped before review. Live Twitch remains deferred under D30/issue #6; P04 stays held.

## Verified progress

The ordinary full Go suite passes, including all five supplied R04 review tests, retained loading, recoverable stop retry, final job publication, Close joining the earlier admitted-operation fixture, actual local protocol success and running HTTP/SSE shutdown. The OBS protocol fixture now records and checks observed request types. Vet, formatting, CGo-free build and all four Node fixtures pass. Independent binary SHA256 matches the worker artifact.

## New queue-boundary evidence

Codex added `internal/app/p03_r04_review_test.go` with two local app composition tests. The fixture uses an injected readiness provider and fixed clock, pauses only its intake consumer, and invokes the production capture/processing methods around a monitoring transition. These tests are NOT live or full protocol evidence; the separate existing protocol test remains intact.

| Test | Result |
|---|---|
| TestReviewR04IdleQueuedReportCannotEnterLaterSession | FAIL: an event captured with no monitoring session creates one durable incident after monitoring starts when receipt/start timestamps are equal |
| TestReviewR04QueuedOldSessionReportIsDropped | PASS: a nonempty captured old-session ID is rejected after stop/start, even with equal timestamps |

Empty MonitoringSessionID currently bypasses the precheck. Reports/feedback captured while idle must remain ineligible; timestamp comparisons must not substitute for the captured session identity. This is an existing C04/R04 requirement, not new product scope.

## Source findings and incomplete boundaries

These are not presented as additional executed failing tests:

1. Session ownership is only checked before calling HandleIssueAt/HandleFeedbackAt. The captured ID is not passed to those methods, so it cannot be revalidated under coordinator admission at durable mutation. Reply dispatch compares current session activity/time but not event.MonitoringSessionID. The worker's claim that replies revalidate the same session is stronger than the implementation. Finish this existing R04 B requirement and test transitions after precheck and before enqueue.
2. finishJob's GetIncident check and SaveIncident are separate operations outside the coordinator gate. The save's transaction merges counters but does not condition the update on the stage/job version read by cleanup. Consequently the code does not yet provide the required atomic ownership guarantee against concurrent dismiss/recheck. A race is identified in source; no concurrent lost-update execution is claimed in this review.
3. Close now waits for the gate owner, which fixes the supplied late-Start test. Cancellation of opCtx stops admission but does not itself cancel an admitted store call. Background reload/GetIncident calls remain, while result/cleanup writes have fixed timeouts. The requested blocked-database shutdown test is still necessary to establish the overall bound; no deadlock is claimed from source alone.
4. Automatic OBS disconnect/replacement, evidence-revision/late-result assertions, stale feedback after replacement and EventSub failure-path socket cleanup remain named but unexecuted worker requirements. Happy-path protocol cleanup and normal blocked-diagnostic/SSE shutdown are now independently passing and must not be redone as missing work.

## Executed commands

Existing workspace Go 1.27.3 / Node 24.14.1; CGO_ENABLED=0, GOTOOLCHAIN=local, GOPROXY=off and workspace caches.

| Command | Result |
|---|---|
| go test ./... -count=1 -timeout=120s | PASS; app 10.518s, incidents 20.147s, all other packages pass |
| go test -tags reviewrepro ./internal/app -run TestReviewR04 -count=1 -v -timeout=30s | One FAIL and one PASS as above |
| go vet ./... | PASS |
| gofmt -l internal cmd web | PASS |
| go build -trimpath -o .tools/p03-r04-review-cuemender.exe ./cmd/app | PASS |
| node testdata/browser/control_csrf_reload.js | PASS |
| node testdata/browser/p02_setup_render.js | PASS |
| node testdata/browser/p03_incidents.js | PASS |
| node testdata/browser/p03_review_evidence.js | PASS |

Build SHA256: `A35895C31D6A059CF2970EAAF428E27049BB4E9F1B68A0A92CF6C7236BCB396D`. WSL race remains worker evidence; not repeated after the deterministic queue failure. No rendered-browser or fresh live OBS check; no Twitch account work or P04 recording/implementation. All review commands exited; no persistent test processes were launched.

## Disposition

[P03-R05](../corrections/P03-R05.md) consolidates only unfinished R04 scope. One new reproduced defect is distinct from the source ownership findings and missing tests. Keep issue #8 and draft PR #9 open/unmerged. Do not waive local requirements under D30. The worker must complete the named boundaries or provide a concrete blocker, rather than treating their omission as phase acceptance.
