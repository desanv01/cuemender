# P03-R01 re-review

Date: 2026-11-02 (Asia/Kuala_Lumpur). Reviewer: Codex. DeepSeek stopped. Codex added tests/docs only; no production fixes.

**Outcome: changes_required; R01 is partially complete.** The original 13 regressions and three actual-script evidence failures are fixed. Six additional tests reproduce unfinished requirements already listed in R01. The remaining full local EventSub/readiness/reply path is required local evidence, independent of deferred live Twitch. Continue under [P03-R02](../corrections/P03-R02.md), existing [issue #8](https://github.com/desanv01/cuemender/issues/8) / [draft PR #9](https://github.com/desanv01/cuemender/pull/9). P04 held; D30/issue #6 unchanged.

## Independent evidence

Workspace root; existing portable Go 1.27.3 environment, CGO_ENABLED=0, GOTOOLCHAIN=local, GOPROXY=off. No live services modified or account work attempted.

| Check | Result |
|---|---|
| `go test ./... -count=1 -timeout=120s` | Exit 0, all existing/promoted tests passed (app 6.695s, incidents 1.643s, store 1.556s). |
| `go test -tags reviewrepro ./internal/diagnostics ./internal/incidents ./internal/store -run 'TestReviewP03\|TestReviewR01ExactReporterFeedbackBounds' -count=1 -timeout=45s` | Exit 0. Original 13 review tests pass; new exact-bound test also passes. |
| `go test -tags reviewrepro ./internal/incidents ./internal/diagnostics ./internal/store -run 'TestReviewP03\|TestReviewR01' -count=1 -timeout=45s` | Exit 1; original tests and exact-bound test pass, six new tests fail as below. |
| Four Node fixtures: control_csrf_reload.js, p02_setup_render.js, p03_incidents.js, p03_review_evidence.js | All exit 0. Actual script, not rendered browser. |
| `go vet ./...`, `gofmt -l internal cmd web` | Exit 0, no formatting output. |
| `go build -trimpath -o .tools/p03-r01-review-cuemender.exe ./cmd/app` | Exit 0, 19,981,824 bytes; SHA256 `D1086C4FB59BEB42FE809BC3716CB3233ADA31AA244ADD7BFC914FAAB8C5E1E5`, matches worker. Review binary not launched. |

No new full race/lifecycle repetition or rendered-browser/live OBS check in this review after deterministic acceptance failures were established. Worker race results remain worker evidence, not a fresh Codex run. All review commands completed; no application server launched or owned process left.

## Six reproduced failures

| Test | Observed failure / existing requirement |
|---|---|
| TestReviewR01ClosedCoordinatorRejectsCheck | Check after Close succeeds and leaves one durable investigating incident with no worker. R01-R02 required closed admission rejection. |
| TestReviewR01ManualResultRejectsReplacedOBS | Stopped/manual job admitted in OBS generation 1 installs two findings after readiness advances to generation 2. Manual jobs still skip the generation guard. |
| TestReviewR01ManualStopRecordsInterrupted | Stop cancels a manual read but leaves its SQLite record investigating forever. |
| TestReviewR01CanceledReadRetainsCategoryAdmission | A second audio read starts while the canceled first read is still unwinding. Fixture allows either joined Stop or retained admission; it does not require cancellation to finish synchronously. |
| TestReviewR01MissingProgramUUIDIsUnknown | A configured scene UUID absent from the program observation still produces a visual finding. Same display name does not establish the required identity. |
| TestReviewR01DedupFractionalExpiry | An ID expiring at an exact UTC second remains retained at +500ms: variable-width RFC3339Nano text comparisons are not chronological across fractional/no-fraction representations. |

Files: internal/incidents/p03_r01_review_test.go, internal/diagnostics/p03_r01_review_test.go, internal/store/p03_r01_review_test.go; opt-in reviewrepro tag. Preserve assertions and promote after correction.

## One gap independently closed

TestReviewR01ExactReporterFeedbackBounds seeds a real temporary SQLite database with exactly 10,000 reporter records and 10,000 feedback actors. It confirms rejection of a new distinct actor, acceptance of an existing reporter without distinct-count inflation, existing-actor vote replacement (9,999 positive/1 negative), exact table bounds and no dedup residue from rejected reports. This passed. It is a real storage boundary test with seeded rows, not 10,000 real Twitch events or a concurrent transport stress test. Retain it in ordinary coverage; do not keep reporting this exact storage bound as untested.

## Remaining source findings

- `run` samples settings/status before acquiring opMu; settings/provider updates do not share that operation gate and do not proactively invalidate jobs. Revision/generation guards remain incomplete at durable commit; manual-generation failure above is concrete evidence. Stop/dismiss delete jobs before joined cleanup; Close rejection is absent from Check/HandleIssue/HandleFeedback/Dismiss. Stop with manual-only work does not record interruptions or publish.
- The new bounded app intake queue is useful, but processTwitchCommand still calls both handlers with context.Background. Close cancels the queue loop then waits for it before other cleanup; cancellation cannot interrupt in-flight store work or opMu waits. Feedback carries neither generation nor receipt time into its handler. Issue generation validation bypasses zero/current-disconnected cases; it does not require a connected Twitch state. Reply admission validates only Twitch generation, not the originating monitoring session.
- Startup recovery still ignores errors and loads only 500 total rows; more old active rows can remain unrecovered. Stop updates session and incidents in separate operations, leaving partial durable state on failure. Retention deletion does not remove cached history after dismiss; memory/private history can grow and show deleted records. These were explicit R01-R03 requests, not new scope.
- DiagnosticRead in internal/obs/manager.go is unchanged: observation-stage failures do not invalidate/recover the transport as discovery failures do. Actual protocol coverage is still required.
- publicLocked still obtains several separate coordinator snapshots and marks an active session ready/checking without observing provider disconnects. Private card streaming text is hard-coded unknown. Coherent snapshots and real true/false/unknown streaming evidence remain unfinished.
- The local app test still injects readiness and calls the normalized callback. It does not exercise the production status path, actual EventSub socket intake or outgoing reply transport. No such full local fixture was added. The report correctly admits this gap; R06 cannot be marked complete.

## Scope and decision

Do not restart P03 or undo passing fixes. P03-R02 consolidates unfinished R01 requirements into four groups. Codex accepts the original regression fixes as verified progress, not the entire packet. P03 remains unaccepted/unmerged, P04 held with recording checkpoint, and P02-A/V06 plus P03-D live Twitch deferred/unmet under D30.
