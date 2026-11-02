# P03 v1.2 acceptance review

Date: 2026-11-02 (Asia/Kuala_Lumpur). Reviewer: Codex. Worker stopped; Codex authored only review fixtures and documentation, not production fixes.

**Outcome: changes_required.** D30's live Twitch deferral is preserved; the failures below are local implementation defects, not account prerequisites. P04 remains held. Correction: [P03-R01](../corrections/P03-R01.md), tracked in [issue #8](https://github.com/desanv01/cuemender/issues/8), implementation [issue #7](https://github.com/desanv01/cuemender/issues/7). No full P03 acceptance or live Twitch claim.

## Independently executed checks

Working directory: project root. Windows Go 1.27.3, workspace toolchain/cache, GOTOOLCHAIN=local, GOPROXY=off, CGO_ENABLED=0. No system installation, live Twitch login or OBS changes.

| Check | Actual result |
|---|---|
| `go test ./... -count=1 -timeout=120s` | Exit 0; all existing packages pass (app 6.562s, incidents 0.884s, store 0.849s). |
| `go vet ./...` | Exit 0. |
| `gofmt -l internal cmd web` | No output. |
| `go build -trimpath -o .tools/p03-review-cuemender.exe ./cmd/app` | Exit 0; 19,954,176 bytes; SHA256 `85EDB18E6FFE839293F1E93AAD8C5B7FA70432813AB7EABF02467B1DBCDA884A`. Review artifact only, never launched; worker binary untouched. |
| `go test ./internal/app -run 'TestRunServesAndShutsDownWithActiveSSE\|TestRunBlockedOBSProviderShutsDownWithActiveSSE' -count=5 -timeout=60s` | Exit 0, 1.949s. Existing P02-style shutdown cases; does not establish a blocked P03 diagnostic path. |
| Existing Node fixtures: control_csrf_reload.js, p02_setup_render.js, p03_incidents.js | Each exits 0. Actual script in Node VM, not rendered-browser proof. |
| Existing WSL Linux toolchain: `go test -race ./... -count=1 -timeout=120s` | Exit 0; all existing packages pass (app 14.547s, incidents 5.203s). No race report. Does not exclude logical races/lock inversion in unexercised paths. |
| New Go fixtures: `go test -tags reviewrepro ./internal/diagnostics ./internal/incidents ./internal/store -run TestReviewP03 -count=1 -timeout=45s` | 13 top-level tests fail; identity test includes five failing subcases. Initial 11 tested together, final two tested separately, then complete set rerun for handoff. |
| `node testdata/browser/p03_review_evidence.js` | Exit 1; missing per-finding manual guidance, unknown rendered as no supported conflict, omitted unavailable reason. |

WSL command/environment is in the P03 packet and reused without provisioning. Reported worker timing flake did not reproduce in this existing-suite review; the untested diagnostic lifecycle still needs explicit coverage. All review commands finished; no application server was launched. No rendered-browser or new live OBS test was performed in this review.

## Reproduced failures

| Test | Observed behavior |
|---|---|
| TestReviewP03RejectUnresolvedMicrophone | Missing microphone, mismatched observed identity, unknown/changed context still yield audio findings; absent track confirmation still yields track-missing. |
| TestReviewP03WrongSceneIsLimitation | Wrong program scene is marked `ok`, allowing a sufficient/healthy result instead of a limitation. |
| TestReviewP03CategoryIsolation | A video incident contains MIC_MUTED (and audio track findings); unconfigured category is not handled as insufficient. |
| TestReviewP03ActiveManualJoinsSession | Manual check while monitoring creates a detached manual session, so later viewer report cannot join it. |
| TestReviewP03ActiveStartChecksRevision | Repeating start with stale expected revision succeeds. |
| TestReviewP03RejectIneligibleFeedback | `finding_available` track-mismatch incident accepts feedback before C04/C09 eligibility. Existing worker test asserts this incorrect behavior. |
| TestReviewP03DismissDoesNotCancelOtherIncident | Dismissing an older incident cancels the different current job with the same category. |
| TestReviewP03ReportsCoalesceCompletedDiagnosis | Another report against an existing diagnosed incident starts a second read without explicit recheck. |
| TestReviewP03ProviderStatusOutsideCoordinatorLock | `schedule` invokes status while holding coordinator mutex. Production status takes app.stateMu, while app snapshot holds stateMu before taking coordinator mutex: source-established lock inversion. Fixture proves unsafe callback placement, not a recorded live hang. |
| TestReviewP03FailedStopDoesNotSilentlyDiverge | A failed durable stop changes memory to inactive; repeat stop reports success although persistence never succeeded. |
| TestReviewP03DedupExpiresAt24Hours | Delivery remains rejected after 25h because expiry timestamp is compared to now minus 24h rather than now. |
| TestReviewP03DedupBoundAfterInsertion | Commit contains 10,002 keys because pruning happens before two inserts. |
| TestReviewP03FeedbackFailureIsAtomic | SQL fault in incident update leaves one committed feedback row despite the failed operation. |

Review-only Go files are internal/diagnostics/p03_review_test.go, internal/incidents/p03_review_test.go, internal/store/p03_review_test.go. Keep assertions; promote corrected equivalents to ordinary coverage. Actual-script regression is testdata/browser/p03_review_evidence.js.

## Additional source findings requiring closure

- Coordinator mutations are not serialized across read/transaction/cache/publication. Start unlocks before validation/commit/install; concurrent starts can create competing sessions. Check creates before reserving job; run deletes admission and unlocks before SaveIncident, allowing dismissal/stop/new report to be overwritten. currentRevision/status checks are separated from commit. Manual jobs skip the current-generation guard. Settings/integration callbacks do not invalidate jobs proactively. These require deterministic barriers, not merely a passing race detector.
- `Stop` returns immediately when no monitoring session exists, leaving offline manual jobs running; Close/Check/HandleIssue admission and joins need lifecycle coverage. Failure paths currently swallow restart/list errors and can leave investigating records without a job. Single SQLite connection does not serialize the whole application operation.
- HandleIssue's Twitch generation parameter is unused; app uses context.Background and EnqueueReply has no originating session/generation argument. The callback waits for SQLite, risking EventSub/close liveness under blocked storage. Keep the existing provider lifecycle and use bounded intake with owned cancellation.
- SaveIncident overwrites an entire incident JSON snapshot, potentially losing new report/feedback counts while indexed count columns retain different values. AddFeedback uses two transactions. Restart loads only 500 total records and ignores store errors; retention only runs at startup; completed-history limits are not continuously enforced.
- Mode is stored as `monitoring`/`manual`, contrary to C01 live/rehearsal; origin already distinguishes viewer/manual. DiagnosticRead does not invalidate transport on observation-phase failure as discovery does; test that path with the actual protocol adapter.
- App public service_status is always disconnected. UI ignores finding.manual_guidance, checks/limitations, expected revision and evidence time, and shows Recheck even for ineligible history. These are functional P03 evidence requirements, not a request for P05 redesign.
- Worker app integration test overrides readiness, uses an OBS backend without SceneCollection, and directly invokes the command callback. That bypasses the production readiness/lock path and actual EventSub/ReplyQueue interaction. It is composition evidence only, not the packet's full local protocol path. New tests must cover that path without inventing live evidence.

## Decision and next step

Publication: worker handoff checkpoint `9fa1652`, independent review fixtures/docs checkpoint `a09caaa`, [draft PR #9](https://github.com/desanv01/cuemender/pull/9). These are unaccepted review checkpoints; nothing merged to main. Issues #7/#8 remain open and live gate #6 remains deferred.

Release only P03-R01 v1.0 to DeepSeek on the current feature/p03-diagnostics branch. Complete all six groups and the required missing local boundaries in one correction handoff. No production fixes made by Codex this cycle. No P04 release or recording now. P02-A/V06 and P03-D live remain deferred/unmet under D30/[issue #6](https://github.com/desanv01/cuemender/issues/6).
