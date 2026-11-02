# P03-R05 acceptance review

2026-11-02 (Malaysia). Codex outcome: **changes_required**. Worker checkpoint `9d754a83c2f4eac2d4ccc509094a65372e937208`; feature/p03-diagnostics. Codex authored review tests/docs only, no production changes. DeepSeek stopped before review. P04 held; live Twitch deferred under D30/#6.

## Passing evidence

Ordinary full Go suite passes, including prior queue regressions, current session-binding checks, settings evidence-revision assertion, protocol success/OBS allowlist and active HTTP/SSE shutdown. Vet, formatting, CGo-free build and all four Node fixtures pass. Independent build hash matches worker.

New independent tests also establish:

- **TestReviewR05CloseJoinsDatabaseBlockedStart: PASS.** A second SQLite connection holds BEGIN IMMEDIATE while Start is admitted. Close joins the operation; releasing the lock allows completion within the test bound. This proves joining for controlled lock release, not prompt cancellation of every possible indefinitely blocked operation.
- **TestReviewR05ConditionalInterruptionPreservesNewerState: PASS**, both newer-investigating and dismissed subcases. An explicitly ordered newer durable state defeats an old cleanup version. This verifies the store's conditional transaction; it does not prove coordinator cache publication ordering.

## Reproduced failures

| Test | Result / cause |
|---|---|
| TestReviewR05CanceledJobPersistsInterruption | FAIL: canceled diagnostic exits but SQLite remains investigating. finishJob overwrites the observed UpdatedAt with now, then supplies that new timestamp as the expected version to InterruptIncident. The conditional update therefore rejects the intended cleanup. |
| TestReviewR05ProtocolFixtureJoinsCanceledUpgrade | FAIL: after the WebSocket client disconnects immediately after welcome, the local fixture handler does not join within one second; it exits only at its five-second help timer. This is a test-server cleanup defect, not evidence of a production Twitch leak. The test waits for eventual teardown and leaves no running handler. |

Review tests: internal/incidents/p03_r05_review_test.go and internal/app/p03_r05_review_test.go. The canceled-job check uses the actual coordinator/store and an owned blocked read; it does not mutate SQLite to fabricate an orphan.

## Source / evidence limits

Session identity now reaches coordinator admission and is compared before reply enqueue; the previous empty/old-session tests pass. Preserve this correction.

The conditional SQL update is useful progress, but its caller must pass the observed version, not the desired new version. Cleanup/cache installation still happens outside the operation gate: a successful old cleanup can be followed by a newer dismiss/recheck before c.setIncident publishes its earlier result. Pending retry also performs a separate current read and unconditional save. Protect the complete durable/cache transition and prove deterministic interleavings; no additional lost-update execution is claimed here.

Automatic provider replacement was tested only by changing the readiness-generation callback. The actual protocol success path remains passing; a real local socket loss/reconnect combined with an outstanding diagnostic remains unverified. No fresh browser/live OBS check. These are evidence limits, not newly claimed production failures.

## Commands

Workspace Go 1.27.3 / Node 24.14.1; Windows CGO_ENABLED=0, GOTOOLCHAIN=local, GOPROXY=off and workspace caches.

| Command | Result |
|---|---|
| go test ./... -count=1 -timeout=120s | PASS; app 10.467s, incidents 20.155s, all packages |
| go test -tags reviewrepro ./internal/incidents ./internal/app -run TestReviewR05 -count=1 -v -timeout=30s | Canceled-job FAIL; database-blocked close PASS; fixture-cancellation FAIL (before conditional-state test was added) |
| go test -tags reviewrepro ./internal/incidents -run TestReviewR05ConditionalInterruptionPreservesNewerState -count=1 -v -timeout=15s | PASS, both subcases |
| go vet ./...; gofmt -l internal cmd web | PASS |
| go build -trimpath -o .tools/p03-r05-review-cuemender.exe ./cmd/app | PASS |
| node testdata/browser/control_csrf_reload.js | PASS |
| node testdata/browser/p02_setup_render.js | PASS |
| node testdata/browser/p03_incidents.js | PASS |
| node testdata/browser/p03_review_evidence.js | PASS |

Build SHA256: `57383E52A17EA6FFA7D1E0FF4678044E9A10C4A7E89987095DE26D132D2204E5`. WSL race remains separately attributed worker evidence, not rerun after deterministic failures. Review commands have exited. No Twitch account work, user OBS changes, P04 implementation or recording occurred.

## Disposition

[P03-R06](../corrections/P03-R06.md) is the consolidated remaining packet. The blocked-SQLite join and store stale-version protection are now verified and must not be carried forward as wholly untested. Fix the real cleanup defect, finish cache/settlement ordering and cancellation fixture/protocol evidence. Issue #8 and draft PR #9 remain open/unmerged; no P04 release.
