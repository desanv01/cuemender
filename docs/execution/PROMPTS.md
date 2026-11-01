# Copy-paste prompts

Revision: 1.9

Current dispatch: P03-R02 v1.0. R01 fixed the original regressions but did not complete its required boundaries. D30/CONTRACTS v1.3 unchanged; live Twitch deferred in issue #6.

## Current: P03-R02 v1.0 — paste into DeepSeek

~~~text
Resume CueMender in C:\Users\Dv\Desktop\Twitch Hackathon as the sole DeepSeek implementation worker for P03-R02 v1.0.

Read AGENTS.md; docs/execution/STATUS.md, WORKFLOW.md, CONTRACTS.md v1.3/C09 and DECISIONS.md D30; packages/P03-diagnostics.md; corrections/P03-R01.md and P03-R02.md; handoffs/P03-R01-review.md; and the appended worker report. Paths after docs/execution refer to that directory.

Confirm P03-R02 is released, branch feature/p03-diagnostics, and no active writer; inspect Git status/diff and claim operational ownership. Preserve the working R01 fixes. Codex added review tests/docs only, no production fixes.

Finish all four groups in P03-R02:
A coordinator closed admission, joined cancellation, manual/monitoring provenance and commit ownership;
B cancellable session/generation-bound intake/replies and the actual local EventSub/OBS/app protocol path;
C chronological expiry, complete recovery, atomic stop/failure behavior and coherent retention;
D required scene identity, observation-failure recovery, coherent projections and actual streaming evidence.

The original 13 review regressions now pass. Six new tests fail in internal/incidents/p03_r01_review_test.go, internal/diagnostics/p03_r01_review_test.go and internal/store/p03_r01_review_test.go. A seventh new test, TestReviewR01ExactReporterFeedbackBounds, already passes at exactly 10,000 reporter records and 10,000 feedback actors. Preserve all assertions and promote the new coverage into the ordinary suite. Do not keep reporting that exact storage bound as untested.

Complete the missing full local protocol fixture using the actual app readiness, EventSub socket and reply transport. A readiness override/direct command callback is not a replacement. Finish the other required local failure paths in the packet, not only the supplied failures. Report a precise technical blocker if one genuinely prevents completion rather than silently carrying required checks into another handoff.

No new backend/dependencies/services, OBS writes, P04 work, UI redesign, new workers, specification edits or Git/GitHub operations. All computer access stays background-only. Do not change the user's OBS configuration. Live Twitch P02-A/V06 and P03-D stay deferred under D30/issue #6; no login, registration, authorization or real chat. Local protocol fixtures do not require those accounts.

Run focused checks, the full Go suite, vet, formatting, CGo-free build, original/new actual-script fixtures, relevant repeated lifecycle tests and the configured WSL race suite. Record failing attempts and actual resolutions. Label browser availability and live evidence honestly.

Append a P03-R02 A–D evidence matrix to docs/execution/handoffs/P03-worker-report.md and update docs/implementation/P03-diagnostics.md. Stop owned processes, transfer STATUS to Codex with ready_for_review or a specific blocker and no active writer, then stop editing. P04 remains held.

Return:
"P03-R02 corrections are ready for review. Read the appended P03-R02 evidence and inspect actual changes against docs/execution/corrections/P03-R02.md and docs/execution/handoffs/P03-R01-review.md. Rerun preserved regressions and the required actual-app/protocol/cancellation/persistence/projection boundaries. DeepSeek has stopped editing. Record acceptance or remaining corrections; live Twitch remains deferred under D30/issue #6, and P04 stays held for Codex release and recording."
~~~

## Historical: P03-R01 v1.0 — partial completion; use R02

~~~text
Resume CueMender in C:\Users\Dv\Desktop\Twitch Hackathon as the sole DeepSeek implementation worker for P03-R01 v1.0.

Read AGENTS.md; docs/execution/STATUS.md, WORKFLOW.md, CONTRACTS.md v1.3/C09 and DECISIONS.md D30; packages/P03-diagnostics.md v1.2; handoffs/P03-worker-report.md and P03-review.md; corrections/P03-R01.md; and the worker report template. Paths after docs/execution refer to that directory.

Confirm STATUS releases P03-R01, the branch is feature/p03-diagnostics, and no writer is active; read Git status/diff, preserve current changes and claim operational ownership. Codex added review fixtures and documents only, not production fixes. Do not restart the implementation.

Complete all six correction groups:
R01 identity/context/confirmation and category-specific diagnostics;
R02 coordinator serialization, lock ordering, admission, sessions and late-result invalidation;
R03 atomic persistence/recovery and exact dedup/retention bounds;
R04 feedback eligibility and generation/session-bound Twitch intake/replies;
R05 truthful private evidence, public status and functional controls;
R06 actual app/protocol/concurrency/failure verification.

Thirteen new Go review tests fail in internal/diagnostics/p03_review_test.go, internal/incidents/p03_review_test.go and internal/store/p03_review_test.go. The actual-script testdata/browser/p03_review_evidence.js reports three failures. Preserve their behavioral assertions and promote corrected coverage into the ordinary suite. Fix the existing worker tests that encode behavior contrary to C09, as identified in the review. Passing supplied regressions alone is insufficient: finish the packet's required local failure boundaries this round.

Use the existing Go-only stack, pinned dependencies and patched SDK. No OBS writes, proposal/approval/repair code, P04 work, P05 redesign, new workers, specification changes or Git/GitHub mutations. P04 remains Codex-owned with the recording checkpoint.

Live Twitch P02-A/V06 and P03-D remain deferred/unmet under D30/issue #6. Do not attempt login, registration, authorization or real chat. Use local protocol fixtures with real application wiring; do not bypass production readiness/ownership with overrides in the acceptance path. No credentials in reports. Background computer access only; do not change the user's OBS configuration or stop user-owned processes.

Run meaningful focused checks, then the required full Go suite, vet, formatting, CGo-free build, all original/new Node fixtures, configured WSL race checks and affected repeated lifecycle tests. Record failures and their resolution; do not explain away a flake solely with a passing rerun. If a tool is unavailable, state the exact remaining check instead of claiming success.

Append a P03-R01 R01–R06 evidence matrix to docs/execution/handoffs/P03-worker-report.md and update docs/implementation/P03-diagnostics.md. Include changed files, commands/results, actual failure-path coverage and remaining gates. Stop owned processes, transfer STATUS to Codex with ready_for_review or a specific blocker and no active writer, and stop editing.

Return this exact handoff:
"P03-R01 corrections are ready for review. Read the appended P03-R01 evidence and inspect actual changes against docs/execution/corrections/P03-R01.md and docs/execution/handoffs/P03-review.md. Rerun the preserved review regressions and required app/protocol/persistence/lifecycle/UI failure paths. DeepSeek has stopped editing. Record acceptance or consolidated remaining corrections, keeping live Twitch deferred under D30/issue #6. P04 remains held for Codex release and recording."
~~~

## Historical: P03 v1.2 initial launch — use P03-R01

~~~text
You are the sole DeepSeek implementation worker for CueMender in:
C:\Users\Dv\Desktop\Twitch Hackathon

Implement P03 v1.2 only. Codex owns planning, contracts, acceptance and all Git/GitHub operations. P04 is reserved for Codex. Do not create other agents or parallel editing chats.

Read before editing:
1. AGENTS.md
2. docs/execution/README.md
3. docs/execution/STATUS.md
4. docs/execution/WORKFLOW.md
5. docs/execution/PROJECT_SPEC.md
6. docs/execution/ARCHITECTURE.md
7. docs/execution/CONTRACTS.md v1.3, especially C09
8. docs/execution/DECISIONS.md, especially D30
9. docs/execution/handoffs/P02-codex-closure-review.md
10. docs/execution/packages/P03-diagnostics.md v1.2
11. docs/execution/templates/WORKER_REPORT.md

Verify STATUS releases P03 v1.2, the branch is feature/p03-diagnostics, and no other writer is active. Read Git status/diff, preserve existing work, record the actual starting HEAD, then claim operational ownership as DeepSeek:P03/in_progress. Do not switch branches yourself.

The user explicitly authorized P03 while Twitch phone/2FA setup is unresolved. P02's local implementation and live OBS are verified; live Twitch P02-A/V06 and P03-D's real-chat portion remain deferred/unmet in GitHub issue #6. Do not attempt Twitch login, registration, authorization or real chat, request credentials, invent a client ID, or mark fixtures as live evidence. This hold does not block P03 implementation and local testing.

Implement the packet end to end: three pure diagnostics; fresh read-only OBS evidence; bounded monitoring/report jobs; persistent incidents, findings, dedup and feedback; authenticated start/stop/manual-check/recheck/dismiss routes; coherent private/public projections; and minimal controls within the existing UI.

Follow C09 precisely. SnapshotBundle is cached and RefreshDiscovery refreshes discovery only: add the scoped fresh diagnostic read with immutable revision/generation/context and bounded cancellation. Integrate the existing normalized Twitch OnCommand seam and existing ReplyQueue; remove contradictory/duplicate P02 idle acknowledgments when the coordinator is wired. Persist before publishing or acknowledging. Test failures and stale work across stop, restart, settings changes, reconnect and dismissal. Preserve all P02 lifecycle/security regression assertions.

Keep the Go-only backend, current dependencies and patched SDK. No OBS mutations, proposal/approval engine, automatic unmute/reveal, stream start/stop, rehearsal, P05 redesign or P06 clear-history work. Do not change authoritative specifications or contracts; report a precise blocker/proposed change to Codex if needed. Ordinary in-scope implementation choices are yours.

Use the existing portable Go toolchain and the exact checks in P03. Run focused tests during implementation, then the final Go suite, vet, formatting, CGo-free build, existing/new actual-JS fixtures and affected lifecycle checks. Existing WSL plus workspace Linux Go supports race testing; run it if your harness permits, otherwise explicitly hand that check to Codex. Do not install system-wide tools. All computer access stays background-only; hidden local browser checks only if available. Do not change the user's OBS setup or stop user-owned processes. Use isolated test data and ports; keep credentials and bootstrap URLs out of reports.

No Git mutations: no commits, branch changes, push, stash, reset, clean, rebase, PRs, merges, issues, publishing or submission.

Write docs/execution/handoffs/P03-worker-report.md and docs/implementation/P03-diagnostics.md. Map P03-A through P03-F and every boundary-test group to actual evidence. Include changed files, actual commands/results, tool versions, migration behavior, dependency changes, limitations and cleanup. Separate local fixtures, actual-script tests, rendered-browser checks, live OBS and deferred live Twitch. Do not silently carry untested local requirements forward.

When finished, update only operational STATUS fields to ready_for_review (or blocked with a specific implementation blocker), owner Codex, no active writer. Stop all processes you started, stop editing, and do not begin P04.

End with a short status summary and this exact handoff:
"P03 worker handoff is ready. Read docs/execution/handoffs/P03-worker-report.md and docs/implementation/P03-diagnostics.md; inspect actual changes against P03 v1.2 and CONTRACTS v1.3/C09; reproduce focused diagnostics, persistence, session/lifecycle, route and UI boundary checks. DeepSeek has stopped editing. Record acceptance, consolidated corrections or specific blockers. Keep P02-A/V06 and P03-D live Twitch deferred under D30/issue #6; do not count fixtures as live evidence. P04 remains held for Codex's explicit release and recording checkpoint."
~~~

This replaces the v1.0 launch instructions: Codex is also a production implementer. P04 is assigned to Codex in the main chat; its prompt below must not be sent to DeepSeek.

## Historical: P02-R03 v1.0 — completed; do not dispatch

~~~text
Resume CueMender in C:\Users\Dv\Desktop\Twitch Hackathon as the sole DeepSeek implementation worker for P02-R03 v1.0.

Read AGENTS.md; docs/execution/STATUS.md, WORKFLOW.md and CONTRACTS.md; the P02 v1.1 packet; docs/execution/handoffs/P02-R02-review.md; and docs/execution/corrections/P02-R03.md. Consult P02-R02.md and the existing worker report for the requirements/history being completed.

Confirm STATUS releases P02-R03, no writer is active and the branch is feature/p02-live-connections; then claim ownership. Preserve the fourteen passing regressions and working setup UI. Complete all four groups in P02-R03: OBS transport ownership/cancellation; OBS context/revision/freshness; Twitch lifecycle/handover/credentials; remaining app/UI failure evidence. These finish existing requirements, not new features.

Six review tests in internal/obs/p02_r02_review_test.go and internal/twitch/p02_r02_review_test.go fail on the handed-back implementation. Keep their behavioral assertions, fix production behavior and promote equivalent ordinary coverage. Also finish the packet's required boundary cases: passing only those six is insufficient. Local stalled-provider, delayed handover and filesystem faults do not require live accounts.

No P03/P04, OBS mutations, UI redesign, new workers, specification edits, dependency substitutions or Git/GitHub actions. D25 manual OBS Connect after restart remains approved. If the pinned SDK prevents a correct bounded solution, report the precise constraint and smallest proposal instead of hiding it behind abandoned goroutines.

Run the required meaningful checks, append an accurate P02-R03 evidence matrix to docs/execution/handoffs/P02-worker-report.md, and update compatibility/setup facts. Separate implemented behavior, executed fixtures, missing evidence and live gates. Never place credentials in reports. Stop owned processes, transfer STATUS to Codex with no active writer, and stop editing.

Return:
"P02-R03 corrections are ready for review. Read the appended evidence and inspect actual changes against docs/execution/corrections/P02-R03.md and P02-R02-review.md. Rerun the complete review set and required boundary checks. DeepSeek has stopped editing. Record acceptance or remaining corrections with live gates explicit. P03 remains held."
~~~

## Historical: P02-R02 v1.0 — partial completion; use R03

~~~text
Resume CueMender in C:\Users\Dv\Desktop\Twitch Hackathon as the sole DeepSeek worker for P02-R02 v1.0.

Read AGENTS.md; docs/execution/STATUS.md, WORKFLOW.md and CONTRACTS.md; the P02 v1.1 packet; docs/execution/handoffs/P02-worker-report.md and P02-R01-review.md; and docs/execution/corrections/P02-R01.md and P02-R02.md.

Confirm P02-R02 is released, no writer is active and the branch is feature/p02-live-connections, then claim ownership. Preserve the existing implementation and the seven already-passing promoted regressions.

Complete all five P02-R02 groups: A token duration/identity/rotation; B actual OBS cancellation and transport ownership; C revision/context-safe observations; D Twitch lifecycle/handover/reply ownership; E stable setup controls and failure-path evidence. These finish existing requirements, not new features.

The new review-only tests in internal/obs/p02_r01_review_test.go and internal/twitch/p02_r01_review_test.go reproduce seven remaining defects. Preserve their assertions, fix the production behavior and promote equivalent coverage into ordinary tests. Complete the packet's additional boundary tests as well; passing only the supplied regressions is insufficient.

Codex approved manual OBS Connect after process restart (D25). Do not add an auto-start preference. Established-loss reconnect still remains required.

No production fixes were made by Codex during this review. Do not redo working R01 fixes. No P03/P04, OBS mutations, new workers, specification edits, dependency substitutions or Git/GitHub operations.

Live Twitch/OBS gates remain separate. Local cancellation, slow-read, handover and filesystem failure tests need no real credentials; complete them now. Never place secrets in chat, logs, source or reports.

Append P02-R02 evidence with an A–E completion matrix, exact commands/results and remaining gates to the existing worker report. Update compatibility/setup facts honestly. Run the required Go, actual-script and focused boundary checks, stop processes, transfer STATUS to Codex and stop editing.

Return:
"P02-R02 corrections are ready for review. Read the appended P02-R02 evidence and inspect actual changes against docs/execution/corrections/P02-R02.md. Rerun the complete review regression set and the required lifecycle/protocol/UI failure paths. DeepSeek has stopped editing. Record remaining corrections or acceptance with live gates explicit. P03 remains held."
~~~

## Historical: P02-R01 — partial completion; use R02

~~~text
Resume CueMender in C:\Users\Dv\Desktop\Twitch Hackathon as the sole DeepSeek implementation worker for P02-R01 v1.0.

Read AGENTS.md; docs/execution/STATUS.md, WORKFLOW.md and CONTRACTS.md; docs/execution/packages/P02-live-connections.md; docs/execution/handoffs/P02-worker-report.md and P02-review.md; docs/execution/corrections/P02-R01.md; and both internal/obs/p02_review_regression_test.go and internal/twitch/p02_review_regression_test.go.

Confirm STATUS assigns P02-R01, no writer is active and the checkout is feature/p02-live-connections. Claim ownership. Preserve the existing implementation and evidence; do not restart from a scaffold.

Implement all eight correction groups in the packet:
R01 actual OBS wire-track normalization and protocol tests;
R02 OBS loss/reconnect, cancellation and saved-password recovery;
R03 current identity/configuration provenance and coherent snapshots;
R04 Twitch validation/revocation and explicit storage policy;
R05 serialized lifecycle, credential deletion and config persistence;
R06 EventSub handover socket ownership and generations;
R07 provider rate limits and normalized command seam;
R08 private setup observation display and identity-preserving save/reload.

Codex's seven review regressions currently fail. Keep their behavioral assertions, make them pass and bring equivalent coverage into ordinary tests; do not weaken them or treat the untagged suite passing as sufficient. Add the packet's missing boundary tests and run its required checks. Correct the compatibility/setup documentation and append a P02-R01 evidence section to the existing worker report.

Twitch registration, own-channel/separate-viewer authorization and live OBS remain separate unmet gates unless actually completed. Fixture servers are not live evidence. Missing live credentials do not block these code corrections. Never request a client secret or place tokens/passwords in chat, source, logs or reports.

No P03/P04 features, OBS mutations, scope expansion, new workers, specification edits or Git/GitHub operations. P04 remains Codex-owned with its recording checkpoint.

When finished, record each R01–R08 result and P02 acceptance gate honestly, update operational STATUS fields, transfer ownership to Codex, stop processes you started and stop editing.

Return:
"P02-R01 corrections are ready for review. Read the appended correction evidence in docs/execution/handoffs/P02-worker-report.md, inspect actual changes against docs/execution/corrections/P02-R01.md, and rerun the review regressions plus focused lifecycle/protocol/UI checks. DeepSeek has stopped editing. Record remaining corrections or acceptance, keeping live Twitch/OBS gates explicit. P03 remains held until accepted."
~~~

## Historical: P02 v1.1 initial implementation — do not restart

~~~text
Work in C:\Users\Dv\Desktop\Twitch Hackathon as the sole DeepSeek implementation worker for CueMender P02 v1.1.

Read before editing:
1. AGENTS.md
2. docs/execution/README.md, STATUS.md and WORKFLOW.md
3. docs/execution/PROJECT_SPEC.md, ARCHITECTURE.md and CONTRACTS.md (v1.2, including C08)
4. docs/execution/packages/P02-live-connections.md
5. docs/execution/handoffs/P01-R01-review.md
6. docs/implementation/P01-environment.md and docs/DEVELOPMENT.md
7. docs/execution/templates/WORKER_REPORT.md

Confirm STATUS releases P02 v1.1, P01 is accepted, no writer is active, and the checkout is feature/p02-live-connections. Then claim P02 ownership in STATUS. If the branch differs, report it to Codex; do not switch it yourself.

Implement P02 end to end: public-client Twitch Device Code authorization and validated/rotated credential storage; EventSub own-channel chat intake and bounded replies; local read-only OBS discovery/observations; minimal private setup controls; and the packet's meaningful tests. Preserve the accepted foundation and CueMender module/name changes. Keep legacy data/keyring/database/cookie identifiers unchanged.

User has NOT registered a Twitch Public client yet, and OBS WebSocket was reported disabled. Complete fixture-based implementation without waiting for these prerequisites. Write the exact operator setup instructions, and perform real checks only when setup is available. Do not invent a client ID, request a client secret, paste tokens into chat, or label fixtures as live. Missing live gates must be explicit in your report; Codex decides acceptance.

Use the existing portable Go toolchain. Pin and compile-verify goobs and coder/websocket; document actual SDK methods and versions before relying on them. Keep credentials out of config, SQLite, logs, public state, source and reports. Protect every new private route with the existing auth/CSRF/Host/Origin/body boundary. Test disconnect/refresh races, old generations, public redaction, config persistence and shutdown with active workers.

No incident engine, monitoring behavior, OBS mutations, proposals or repair code. P04 is reserved for Codex, with its recording checkpoint. No new workers, framework/service dependencies, specification edits, Git mutations, commits, pushes, PRs or publishing. Normal implementation choices inside this packet are yours; report any needed contract/dependency-strategy change to Codex.

Run the packet's required checks and fix failures. Write docs/implementation/P02-compatibility.md, docs/implementation/P02-setup.md and docs/execution/handoffs/P02-worker-report.md, mapping P02-A through P02-I to actual evidence. Distinguish executed, failed, skipped, fixture and live checks. Include changed files, dependency pins, commands/results, environment changes and exact unmet prerequisites.

When done or blocked, update only operational STATUS fields, transfer ownership to Codex, stop processes you started and stop editing. Do not begin P03.

Return this handoff:
"P02 worker handoff is ready. Read docs/execution/handoffs/P02-worker-report.md, docs/implementation/P02-compatibility.md and docs/implementation/P02-setup.md; inspect actual changes against P02 v1.1 and CONTRACTS v1.2; reproduce the focused regressions and check live Twitch/OBS evidence separately from fixtures. DeepSeek has stopped editing. Record acceptance, consolidated corrections or specific unmet gates. P03 remains held until accepted."
~~~

## Historical: P01-R01 correction — completed, do not redispatch

~~~text
Work in C:\Users\Dv\Desktop\Twitch Hackathon as the sole DeepSeek implementation worker for P01-R01.

Read AGENTS.md, docs/execution/STATUS.md, WORKFLOW.md and CONTRACTS.md; docs/execution/packages/P01-foundation.md; docs/execution/handoffs/P01-worker-report.md and P01-review.md; and docs/execution/corrections/P01-R01.md.

Confirm P01-R01 is assigned for correction and no other writer is active, then claim ownership in STATUS.

Implement all four required corrections:
1. Restore the private CSRF token after an authenticated browser reload.
2. Make production settings commit/cache/SSE publication ordering coherent.
3. Investigate the intermittent shutdown failure and test shutdown with an actually active SSE client.
4. Remove raw credential-backend error text from safe formatted errors and test sentinel-secret redaction.

Use the exact acceptance and regression requirements in P01-R01. Test the actual production App/hub and browser JS where relevant; don't rely solely on duplicate test implementations. Run the focused lifecycle test at least five times, then the complete required checks. Clearly separate real browser/binary Ctrl+C evidence from fixtures or forced termination.

Do not change specifications, add P02/P04 features, perform a branding sweep, or do any Git/GitHub operations. CueMender/public cuemender is finalized; Codex will coordinate naming and Git work separately.

Append the actual changes, commands/results and remaining limitations under P01-R01 in the existing worker report. Preserve prior evidence. Update operational STATUS fields, stop all processes you started, transfer ownership to Codex and stop editing. Do not start P02.

Return this handoff:
"P01-R01 corrections are ready for review. Read the correction section of docs/execution/handoffs/P01-worker-report.md, inspect actual changes against docs/execution/corrections/P01-R01.md, and reproduce the focused regressions. DeepSeek has stopped editing. Record acceptance, remaining corrections or specific blockers. P02 remains held until accepted."
~~~

## Original P01 fresh-start prompt — historical, do not redispatch now

~~~text
You are the implementation worker for LiveCheck in this shared workspace:

C:\Users\Dv\Desktop\Twitch Hackathon

Codex is the planner, contract owner, acceptance reviewer and Git/GitHub operator. Codex also directly implements P04's approval-and-repair engine and tests, and performs needed integration fixes/debugging after ownership transfer. DeepSeek implements P01/P02/P03/P05/P06 only. Your job is to implement, test, debug and report the assigned packet. Do not create other workers or run parallel editing chats.

Read these files before editing:
1. AGENTS.md
2. docs/execution/README.md
3. docs/execution/STATUS.md
4. docs/execution/WORKFLOW.md
5. docs/execution/PROJECT_SPEC.md
6. docs/execution/ARCHITECTURE.md
7. docs/execution/CONTRACTS.md
8. docs/execution/packages/P01-foundation.md
9. docs/execution/templates/WORKER_REPORT.md

Execute P01 v1.1 only. Check that STATUS still releases P01 and no other writer is active, then claim ownership as instructed. Implement the packet end to end; run its required checks, fix your failures, and provide evidence. Don't stop after proposing a plan.

The folder initially contains planning documents and no application code or Git repository. Preserve the existing documents. Go was not found on Codex's PATH; check your environment and installed locations first. P01 permits an official checksum-verified portable Go ZIP inside ignored .tools if your harness permits that download. No administrator/system-wide install or machine PATH change. If the toolchain still cannot be made available within those limits, report the exact blocker and do not produce a large uncompiled scaffold.

Keep the backend Go-only. Implement the small foundation, not Twitch/OBS integrations, diagnostics or repair actions. Do not change the authoritative specifications or add framework/service dependencies. Normal local implementation decisions are allowed. A needed contract/dependency-strategy change must be reported to Codex.

P04 is reserved for substantive Codex implementation; do not pre-implement it or generate its production code under P01. Record your actual contribution accurately. This division is a workflow choice, not a guarantee of hackathon eligibility, and no tool may claim authorship of work performed by the other.

Do not initialize Git, commit, push, switch/reset/clean/stash branches, publish or submit anything. Do not put credentials, bootstrap URLs or raw tokens into reports. No real credentials are needed for P01.

Write docs/implementation/P01-environment.md and docs/execution/handoffs/P01-worker-report.md, mapping P01-A through P01-I to actual evidence. Separate executed checks, failures, skipped checks and unverified browser/live behavior. Include exact tool versions, changed files, commands/results, dependency pins, environment changes and reproduction steps.

When finished or blocked, update only the operational fields of STATUS, transfer ownership to Codex, stop your application/test processes and stop editing. Do not start P02.

End with a short status summary and this exact handoff message for me to paste into Codex:

"P01 worker handoff is ready. Read docs/execution/handoffs/P01-worker-report.md and docs/implementation/P01-environment.md, inspect the actual files against P01 v1.1 and the shared contracts, and perform the focused acceptance review. DeepSeek has stopped editing. Record accepted, changes required, or blocked. If you directly fix integration issues, take sole edit ownership and record your changes/tests. Then provide the consolidated correction prompt or release the next packet."
~~~

## Resume interrupted P01 in the same DeepSeek chat

~~~text
Resume only the already dispatched P01 in C:\Users\Dv\Desktop\Twitch Hackathon.

Reread AGENTS.md, docs/execution/STATUS.md, docs/execution/WORKFLOW.md and docs/execution/packages/P01-foundation.md. Inspect actual files and your prior results before repeating work. Continue only if STATUS still assigns P01 implementation to you; if ownership has transferred to Codex, stop and report that.

Finish remaining in-scope tasks and required checks, fix failures, update the P01 worker report honestly, and hand back to Codex. Do not change specifications, perform Git mutations, or start P02.
~~~

## P01 completion notice — paste into this Codex chat

~~~text
P01 worker handoff is ready. Read docs/execution/handoffs/P01-worker-report.md and docs/implementation/P01-environment.md, inspect the actual files against P01 v1.1 and the shared contracts, and perform the focused acceptance review. DeepSeek has stopped editing. Record accepted, changes required, or blocked. If you directly fix integration issues, take sole edit ownership and record your changes/tests. Then provide the consolidated correction prompt or release the next packet.
~~~

## Phase prompt index — consult STATUS

### P02

P02 implementation/corrections are complete. Only deferred live Twitch verification remains; no worker correction is released.

### P03

Use the current P03-R02 correction prompt at the top. D30's live Twitch deferral remains unchanged; no new initial implementation dispatch.

### P04 — send to Codex in this main chat, not DeepSeek

~~~text
In C:\Users\Dv\Desktop\Twitch Hackathon, take sole Codex implementation ownership of P04. DeepSeek is stopped. Do not create another worker chat or delegate this implementation.

Read AGENTS.md; docs/execution/STATUS.md, WORKFLOW.md, PROJECT_SPEC.md, ARCHITECTURE.md, CONTRACTS.md, CONTRIBUTIONS.md; docs/execution/packages/P04-approved-repair.md; and docs/execution/handoffs/P03-review.md. Proceed only if P03 is accepted, P04 explicitly released and no other writer is active.

First complete P04.0: test the recorder, start capture of genuine Codex implementation/test moments, and keep secrets off-screen. At completion stop and verify saved footage before recording its path. Do not silently skip the recording checkpoint.

Directly implement the production proposal/approval logic, durable action intent, fresh preconditions, single OBS track update, read-back and timeout/restart reconciliation. Add and run the specified meaningful tests and debug the actual integration. No generalized OBS control endpoint or optional expansion. Live trial uses only the dedicated configured test setup. Publishing remains a separate authorized step.

Write docs/execution/handoffs/P04-codex-build.md with exact implemented files, commands/results, live-vs-fixture checks, recovery evidence and remaining blockers. Update CONTRIBUTIONS with actual work, not planned claims. Verify against the acceptance criteria and write P04-review.md, clearly labeling checks of your own code as self-verification. Then update STATUS and, if accepted, prepare the DeepSeek P05 prompt; do not launch DeepSeek yourself.
~~~

### P05

~~~text
In C:\Users\Dv\Desktop\Twitch Hackathon, act as the sole DeepSeek implementation worker for P05 only.

Read AGENTS.md; docs/execution/STATUS.md, WORKFLOW.md, PROJECT_SPEC.md, ARCHITECTURE.md, CONTRACTS.md; docs/execution/packages/P05-user-experience.md and its released visual brief; and docs/execution/handoffs/P04-review.md. Proceed only if P04 is accepted, P05 explicitly released with its design checkpoint, and no other writer is active.

Finish setup/control/overlay, viewer feedback and isolated rehearsal. Preserve the accepted backend boundaries. Use vanilla HTML/CSS/JS and the released visual brief. Do not claim a skill/tool was used if your harness lacks it. Test actual browser interactions and rehearsal's lack of live effects. No contract changes or Git/publishing operations.

Write docs/execution/handoffs/P05-worker-report.md, with screenshots when available and no secrets. Update operational STATUS fields, stop editing/processes, and give me a short Codex review prompt. Do not begin P06.
~~~

### P06

~~~text
In C:\Users\Dv\Desktop\Twitch Hackathon, act as the sole DeepSeek implementation worker for P06 only.

Read AGENTS.md; docs/execution/STATUS.md, WORKFLOW.md, PROJECT_SPEC.md, ARCHITECTURE.md, CONTRACTS.md, VERIFICATION.md, SUBMISSION.md; docs/execution/packages/P06-release-readiness.md; and docs/execution/handoffs/P05-review.md. Proceed only if P05 is accepted, P06 explicitly released and no other writer is active.

Finish required reliability checks, local Windows release ZIP, clean-folder trial and accurate setup/troubleshooting documentation. Report real Twitch/OBS behavior separately from rehearsal. Do not expand scope, change specifications, initialize/commit/push Git, upload artifacts, contact others or submit anything.

Write docs/execution/handoffs/P06-worker-report.md with artifact path/hash, exact build/test evidence and all remaining limitations. Update operational STATUS fields, stop editing/processes, and give me a short Codex acceptance prompt. Final external release/submission remains with Codex and the user.
~~~

## Correction prompt template

Codex fills in the exact packet, correction number and current revision when issuing it. Do not dispatch the unfilled template.

~~~text
In C:\Users\Dv\Desktop\Twitch Hackathon, resume implementation of [PACKET_ID] only.

Read AGENTS.md, STATUS, the current packet, the worker report, the Codex review and docs/execution/corrections/[PACKET_ID]-[REVISION_ID].md. Confirm STATUS assigns this correction to you and no other writer is active.

Implement all numbered correction requests within their stated scope, run the specified checks and fix regressions. Do not redesign contracts or add features. Preserve prior report evidence and append a correction section with actual changes/results. No Git operations.

Hand back to Codex, update operational STATUS fields and stop editing. Do not start the next packet.
~~~

## Later Codex acceptance prompt template

~~~text
[PACKET_ID] worker handoff is ready in the shared workspace. DeepSeek has stopped editing.

Read docs/execution/handoffs/[PACKET_ID]-worker-report.md, the assigned packet/current revision and shared contracts. Inspect actual changes and reproduce the focused acceptance behavior; don't treat the worker report as proof.

Record the result in docs/execution/handoffs/[PACKET_ID]-review.md and update STATUS. If worker changes are required, write one consolidated correction file and exact DeepSeek prompt. If you directly fix an integration issue, first take sole edit ownership and record actual changes/tests and self-verification. If accepted, checkpoint appropriately and release/refine the next packet with its exact prompt for the assigned implementer: P04 is Codex-owned. Preserve existing work and do not publish externally just because a package passed.
~~~
