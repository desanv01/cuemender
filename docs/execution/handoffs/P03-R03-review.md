# P03-R03 acceptance review

Date: 2026-11-02 (Malaysia). Reviewer: Codex. **Outcome: changes_required.**

Worker checkpoint: `36845378cece73847e27198d9841b4cb25714dbf`, branch feature/p03-diagnostics. DeepSeek stopped before review. Codex authored review tests/docs only, no production fixes. P04 remains held; live Twitch remains deferred/unmet under D30/issue #6.

## Verified progress

The ordinary full Go suite passes, including earlier regression coverage and the actual local OAuth/Helix/EventSub/OBS app test. Startup now propagates recovery failures; startup retained loading includes non-terminal records; current-generation rejection and idle help work in the exercised paths. The local protocol success test checks SQLite findings and joins the EventSub handler on its happy path. All four Node actual-script fixtures, vet, formatting and the CGo-free build pass.

The new context-aware production gate works in a focused occupied-gate cancellation test. The promoted old test still locks unused opMu, so it no longer exercises the production gate; fix that obsolete test mechanism rather than counting it as occupied-gate evidence. Codex's new test supplies the missing check.

## Reproduced failures

New review-tagged coverage: `internal/incidents/p03_r03_review_test.go`. Four failing tests and one passing occupied-gate check. Tests use isolated real SQLite and deterministic synchronization; no live credentials.

| Test | Observed result | Required correction |
|---|---|---|
| TestReviewR03RetainedHistorySurvivesDismissReload | Startup loads 502 retained records; one dismissal/reload leaves only 500 visible | Use retained loading after startup as well; preserve open records and propagate reload errors |
| TestReviewR03ManualStopRetriesAfterRecoverableSQLFault | After removing an injected SQLite UPDATE failure, Stop returns nil but durable incident remains investigating | Keep failed interruption retryable after worker exit; synchronize durable/cache state |
| TestReviewR03SuccessfulJobPublishesSettledCount | ActiveJobs becomes zero, but the last OnChange publication recorded one | Publish the settled job/service state after successful job removal |
| TestReviewR03ClosePreventsLateStartCommit | Close returns while an admitted Start is blocked; releasing Start then commits an active monitoring session | Cancel or join admitted operations; no commit/publication after Close returns |

The close regression accepts an implementation that joins the admitted operation before returning; it does not demand that every in-flight operation fail. The tested defect is a successful durable write after Close has already returned.

The recoverable SQL failure is injected through a second database/sql connection to the same temporary database using CREATE/DROP TRIGGER. No access to the store's unexported DB handle is needed. Therefore the worker's stated obstacle is not a prerequisite blocker. The closed-store regression alone could not establish retry behavior.

## Source findings / incomplete evidence (not additional executed failures)

- `finishJob` removes ownership before attempting interruption persistence and suppresses the save failure. Cleanup also runs outside the operation gate; preserve newer dismissal/recheck/session state when fixing it. A successful job sets settled and therefore skips the final publication entirely.
- `reload` still uses capped mustList and turns query errors into empty history. Startup-only ListRetainedIncidents does not repair this runtime path.
- Intake still carries CommandEvent only, and reply validation reads the then-current session plus receipt time. There is no captured originating monitoring-session ID. The report's stronger session-ownership claim is not established by this code. Complete the already-required queue-across-stop/start and reply-session-change cases.
- `run` still saves its result with context.Background. Closing no longer serializes with ordinary coordinator operations. The close regression establishes one concrete late-write failure; blocked database/intake shutdown remains required.
- `TestP03BlockedDiagnosticWholeAppShutdown` calls App.Close but never starts App.Run or opens HTTP/SSE. It validates a blocked diagnostic, listener release and database reopen; it does not establish shutdown with active HTTP/SSE. Extend the existing fixture for that combined boundary.
- The SaveSettings test exercises cancellation/invalidation and a later recheck. It does not explicitly assert the recheck evidence revision or model a delayed result winning a commit race; add those assertions/barriers. No new automatic-provider-loss test was supplied.
- Protocol happy-path EventSub cleanup is improved. The OBS fixture still returns success for unknown request types; an explicit observed-request allowlist and failure-path socket cleanup remain required by R03 D. No actual OBS-write defect is claimed from this omission.

## Independent commands/results

Existing workspace toolchain: Go 1.27.3, Node 24.14.1; Windows CGO_ENABLED=0, GOTOOLCHAIN=local, GOPROXY=off, workspace caches.

| Command | Result |
|---|---|
| go test ./... -count=1 -timeout=120s | PASS all packages; app 9.580s, incidents 9.959s |
| go test -tags reviewrepro ./internal/incidents -run TestReviewR03 -count=1 -timeout=30s | FAIL: four failures above, before the separate passing occupied-gate check was added |
| go test -tags reviewrepro ./internal/incidents -run 'TestReviewR03(CanceledProductionGateReturns\|ClosePreventsLateStartCommit)' -count=1 -timeout=15s | Gate check passes; close regression fails again after its assertion was clarified to allow joined operations |
| go vet ./... | PASS |
| gofmt -l internal cmd web | PASS; new review test formatted afterward |
| go build -trimpath -o .tools/p03-r03-review-cuemender.exe ./cmd/app | PASS; SHA256 below matches worker binary |
| node testdata/browser/control_csrf_reload.js | PASS |
| node testdata/browser/p02_setup_render.js | PASS |
| node testdata/browser/p03_incidents.js | PASS |
| node testdata/browser/p03_review_evidence.js | PASS |

Build SHA256: `16A0BB16CEBE3ECA6754F5D491A899F723D8432AAF9D451CE0263DD84844CA9B`. No full WSL race rerun after deterministic failures; worker race remains worker evidence. No rendered-browser or fresh live OBS verification this review. No Twitch account work, production data changes or user-owned process changes. Review commands have exited; P04 implementation/recording has not started.

## Disposition

[P03-R04](../corrections/P03-R04.md) is the consolidated remaining correction packet. Preserve the R03 improvements. Keep issue #8 and draft PR #9 open/unmerged, with separate worker/reviewer commits. Local failures are not waived by D30, which applies only to live Twitch sequencing. The next handoff must finish the named boundaries, not silently move them to another limitations list.
