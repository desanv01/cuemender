# Copy-paste prompts

Revision: 1.2

Current dispatch: P01-R01 correction below. P01 is implemented but not accepted. Do not use the original fresh-start prompt or begin P02. Each later packet is conditional on Codex acceptance/release in STATUS.

This replaces the v1.0 launch instructions: Codex is also a production implementer. P04 is assigned to Codex in the main chat; its prompt below must not be sent to DeepSeek.

## Current: P01-R01 correction — paste into DeepSeek

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

## Later packet dispatch prompts — held until released

### P02

~~~text
In C:\Users\Dv\Desktop\Twitch Hackathon, act as the sole DeepSeek implementation worker for P02 only.

Read AGENTS.md; docs/execution/STATUS.md, WORKFLOW.md, PROJECT_SPEC.md, ARCHITECTURE.md, CONTRACTS.md; docs/execution/packages/P02-live-connections.md; and docs/execution/handoffs/P01-review.md. Use the current packet revision. Proceed only if Codex accepted P01 and explicitly released P02 and no other writer is active.

Implement the released live-connection packet, test/debug it, and report actual Twitch/OBS evidence separately from fixtures. Don't change specifications, add new scope or perform Git/GitHub/publishing operations. No real credentials in chat, source, logs or reports. Missing live prerequisites must be reported, not simulated as success.

Write docs/execution/handoffs/P02-worker-report.md using the worker template and docs/implementation/P02-compatibility.md. Update operational STATUS fields, stop editing/processes, and give me a short Codex review prompt pointing to those files. Do not begin P03.
~~~

### P03

~~~text
In C:\Users\Dv\Desktop\Twitch Hackathon, act as the sole DeepSeek implementation worker for P03 only.

Read AGENTS.md; docs/execution/STATUS.md, WORKFLOW.md, PROJECT_SPEC.md, ARCHITECTURE.md, CONTRACTS.md; docs/execution/packages/P03-diagnostics.md; and docs/execution/handoffs/P02-review.md. Proceed only if P02 is accepted, P03 is explicitly released and no other writer is active.

Implement, test and debug the three diagnostics and incident lifecycle as specified. No OBS mutations, scope/contract changes or Git operations. Preserve unknown/stale evidence semantics and distinguish live from fixture results.

Write docs/execution/handoffs/P03-worker-report.md using the worker template. Update operational STATUS fields, stop editing/processes, and give me a short Codex review prompt pointing to that report. Do not begin P04.
~~~

### P04 — send to Codex in this main chat, not DeepSeek

~~~text
In C:\Users\Dv\Desktop\Twitch Hackathon, take sole Codex implementation ownership of P04. DeepSeek is stopped. Do not create another worker chat or delegate this implementation.

Read AGENTS.md; docs/execution/STATUS.md, WORKFLOW.md, PROJECT_SPEC.md, ARCHITECTURE.md, CONTRACTS.md, CONTRIBUTIONS.md; docs/execution/packages/P04-approved-repair.md; and docs/execution/handoffs/P03-review.md. Proceed only if P03 is accepted, P04 explicitly released and no other writer is active.

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
