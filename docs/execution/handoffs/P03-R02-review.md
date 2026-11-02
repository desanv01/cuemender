# P03-R02 acceptance review

Date: 2026-11-02 (Malaysia). Reviewer: Codex. Outcome: **changes_required**.

Worker checkpoint: `a4603c9bb5d8492bb6a5055fbe4feff43d393a93` on feature/p03-diagnostics. Review tests/docs are a separate following commit; the branch remains unaccepted.

DeepSeek stopped before review. Codex added review tests and documentation only; no production fixes. P04 is held. Live Twitch P02-A/V06 and P03-D remain deferred/unmet under D30 and issue #6. This is independent verification of DeepSeek implementation, not an independent review of Codex-authored tests.

## Confirmed progress

- Ordinary full Go suite passes, including the promoted earlier regressions and exact 10,000 reporter/feedback boundary coverage.
- The new `TestP03ActualAppProtocolReportAndReply` passes with production app wiring and local OAuth/Helix/EventSub and OBS protocol servers. It establishes the socket-to-finding-to-Helix success path without readiness override/direct callback. It is fixture evidence, not live Twitch/OBS.
- Four Node actual-control-script fixtures pass; vet, formatting and a CGo-free build pass.
- Earlier corrections for chronology, scene identity, closed admission and canceled-read capacity are retained. Do not restart P03 or replace this working implementation.

## Reproduced acceptance failures

The following four new review-tagged tests fail on the handed-back implementation. They are in `internal/incidents/p03_r02_review_test.go` and `internal/app/p03_r02_review_test.go`.

| Test | Observed failure | Consequence |
|---|---|---|
| TestReviewR02StartupRecoveryErrorPropagates | Real App.New succeeds despite a SQLite trigger aborting recovery updates | Startup presents success after durable recovery failed |
| TestReviewR02CanceledGateReturnsPromptly | Already-canceled check waits for held opMu | Passing a cancellation context does not bound intake/operation admission or shutdown |
| TestReviewR02DisconnectedTwitchRejectsQueuedReport | Old generation report is persisted after readiness reports disconnected/generation zero | Disconnected queued events can mutate current-session state |
| TestReviewR02ManualStopFailureIsReported | Manual Stop returns nil after the real store has been closed | UI/cache can claim an interruption that was not saved |

These are existing R02 A/B/C requirements, not new product scope. The closed-store test establishes failure propagation; the correction must additionally use recoverable fault injection to prove retry and cache/storage agreement.

## Source findings and still-required boundary evidence

These are source findings or missing evidence, not additional claimed failing executions:

1. `restart` discards store errors and `mustList` turns query errors into empty history. Its unconditional 500-row limit also hides older retained open/unresolved incidents. C06 bounds completed history, not all retained records. Bulk interruption now reaches all SQL rows, but that alone does not establish retained-record accessibility.
2. `processTwitchCommand` gates every reply on an active monitoring session, including `!streamhelp`; C04 permits help while connected but idle. Report/feedback validation skips mismatches when current generation is zero and does not require connected status. Intake carries receipt time but not the originating monitoring-session identity.
3. `run` samples settings/provider provenance before waiting on opMu, uses Background for result persistence, and returns on expired/canceled work without necessarily finalizing the investigating record. Job removal happens after publication and does not itself publish the new job count. The already-required real-App SaveSettings/blocked-result and blocked diagnostic shutdown boundaries still need deterministic proof; do not infer correctness from mutable-callback tests alone.
4. The new EventSub fixture ends its upgraded handler with `select {}`. Its deferred socket close is unreachable; httptest.Server.Close does not close hijacked WebSockets. Fix fixture ownership and assert handlers/sockets join. The success test currently inspects coordinator cache for findings; also verify the persisted row through the real store. Keep this actual protocol path and strengthen it rather than replacing it with direct callbacks.

## Commands and evidence

Environment: existing workspace Go 1.27.3, Node 24.14.1; Windows CGO_ENABLED=0, GOTOOLCHAIN=local, GOPROXY=off, workspace GOPATH/GOCACHE/GOTMPDIR.

| Command | Result |
|---|---|
| go test ./... -count=1 -timeout=120s | PASS, all packages |
| go test -tags reviewrepro ./internal/diagnostics ./internal/incidents ./internal/store ./internal/app -run 'TestReviewP03\|TestReviewR01\|TestP03ActualAppProtocolReportAndReply' -count=1 -timeout=90s | PASS, preserved regression selection and new protocol success path |
| go test -tags reviewrepro ./internal/incidents ./internal/app -run TestReviewR02 -count=1 -timeout=30s | FAIL, exactly the four tests above |
| go vet ./... | PASS |
| gofmt -l internal cmd | PASS, no paths reported |
| go build -o .tools/p03-r02-review-cuemender.exe ./cmd/app | PASS, CGo-free |
| node testdata/browser/control_csrf_reload.js | PASS |
| node testdata/browser/p02_setup_render.js | PASS |
| node testdata/browser/p03_incidents.js | PASS |
| node testdata/browser/p03_review_evidence.js | PASS |

The review build SHA256 is `9ABD91FB767A84C8078F167990E50AACE4C35F6F267AC042B67888B8A582297B`. This is a separate unversioned review build; it does not replace or contradict the worker's versioned binary hash. An initial reviewer build command targeted nonexistent ./cmd/cuemender and failed; the corrected ./cmd/app build passed. No production defect is attributed to that command error.

Worker-reported WSL race results remain worker evidence. Codex did not repeat full race after deterministic acceptance failures were established. No rendered-browser test or fresh live OBS test was performed in this review. Earlier P02 live OBS evidence remains historical evidence only. No Twitch account work, OBS configuration changes, background application launch or P04 work occurred. Test commands have exited; the identified fixture leak is process-local and is not claimed as clean in-process teardown.

## Disposition

Release [P03-R03 v1.0](../corrections/P03-R03.md) as one consolidated completion packet. Keep issue #8 and draft PR #9 open, unaccepted and unmerged. Preserve worker and reviewer provenance in separate commits. The packet explicitly separates reproduced failures from the remaining required tests. Passing only the four supplied tests is insufficient if those listed R02 requirements remain incomplete.
