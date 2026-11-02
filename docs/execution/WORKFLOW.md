# Worker and reviewer protocol

Revision: 1.2

Current exception: D30 explicitly permits P03 development on the conditionally accepted P02 baseline while live Twitch gates remain deferred/unmet in issue #6. This changes phase sequencing only, not live-evidence requirements or final readiness. Earlier P02/P03 hold wording in historical reports is superseded by this recorded decision; consult STATUS for the current release. No automatic P04 release follows.

## Ownership sequence

ready -> in_progress -> ready_for_review -> accepted
                         \-> changes_required -> in_progress
Any stage may record blocked with a specific prerequisite.

Only Codex releases packets and accepts them. Later packets are planned, not authorized. The user dispatches the exact prompt. Before editing, DeepSeek checks STATUS and claims the released packet. If another worker is recorded as active, stop and report the conflict. STATUS is advisory; the user must stop the other harness as well.

After handoff, DeepSeek stops all edits. Codex reviews actual files and required behavior, records one result, and produces a consolidated correction if needed. Do not create a new chat for each individual subtask.

## Implementation allocation

| Packet | Primary implementer | Completion gate |
|---|---|---|
| P01 foundation | DeepSeek | Codex reviews actual implementation |
| P02 live connections | DeepSeek | Codex reviews actual implementation and live evidence |
| P03 diagnostics | DeepSeek | Codex reviews findings and incident behavior |
| P04 approved repair | Codex in this main chat | Codex documents its implementation, tests and real integration verification |
| P05 user experience | DeepSeek | Codex reviews working UI and rehearsal isolation |
| P06 release readiness | DeepSeek | Codex verifies artifact and complete workflow |

Codex owns meaningful production implementation in P04: proposal/approval binding, fresh preconditions, durable action intent, narrow OBS write, read-back, timeout/restart reconciliation and their tests. This is planned work, not an already completed contribution.

After any DeepSeek handoff, Codex may directly fix a scoped integration defect or debug code when that avoids another relay. First take sole edit ownership in STATUS; record affected files, actual changes and verification afterward. Do not rewrite functioning code merely to manufacture Codex authorship. Larger worker corrections still receive one consolidated packet.

P04 runs under this main chat without launching DeepSeek. D32 records the user's explicit authorization for GPT-6.1 Sol high subagents, with exactly one production writer at a time; root retains planning, acceptance and Git ownership. Write handoffs/P04-codex-build.md with exact changes/tests and handoffs/P04-review.md with the acceptance evidence and limits. Call checks of Codex's own code self-verification; a role label does not create an independent reviewer. An additional review may be requested if unresolved risk warrants it, but is not fabricated or automatically delegated.

## Worker permissions

May inspect relevant files and public official documentation, implement assigned scope, choose small local code organization details, fetch pinned module dependencies through normal tooling, run tests and debug failures. May create test fixtures and temporary data in an isolated location.

May not alter product/contract/specification documents, accept its own work, add backend languages/services, perform Git mutations, publish/upload, contact third parties, change unrelated user files, or modify a live OBS scene/track outside the authorized tested product workflow. Missing tooling does not authorize machine-wide installs or elevated work.

A current installed Go executable outside PATH may be used by full path and reported. P01 also permits an official checksum-verified portable toolchain confined to .tools when the harness permits the download. Otherwise stop dependent tasks and report the exact prerequisite; never imply tests ran.

## Baseline and changes

Initial workspace has no .git and no application code. BASELINE records preserved root documents. P01 may proceed without Git; report every added/modified/deleted file. Worker must not initialize Git. Codex can establish a repository/checkpoint during acceptance. If Git appears, inspect status before changes and preserve unrelated edits.

For later packets, the accepted commit or file baseline and previous report identify the starting point. A worker does not sweep unrelated modifications into its completion claim.

## Reports

Use templates/WORKER_REPORT.md. Write handoffs/Pxx-worker-report.md, preserving prior report versions if a correction adds new evidence. Include UTC timestamp, packet revision, exact commands/results, acceptance coverage, failures/skips, tool versions, changed paths, deviations, live vs simulated results, processes stopped and remaining prerequisites.

No credentials, bootstrap links, device codes, private chat logs or tokens. Redact authorization headers and sensitive URLs. Report only whether keyring operations worked, not their stored values.

The final pasted message points Codex at the report and states that editing stopped. It is not a huge replacement for the report.

## Codex acceptance

1. Confirm the worker stopped and take ownership.
2. Inspect the actual changed files, scope and contract-sensitive logic.
3. Reproduce the central acceptance behavior and highest-risk cases.
4. Use prior passing evidence for unchanged low-risk areas; expand only for new changes or concerns.
5. Record accepted, changes_required, or blocked with exact reasons in handoffs/Pxx-review.md.
6. If accepted, checkpoint as appropriate, update STATUS and release the next refined packet.
7. If worker corrections are needed, write one corrections/Pxx-Rnn.md with numbered requests and retest expectations; give one prompt. If Codex instead performs a scoped fix, record the ownership transfer, actual code changes and retests before accepting.

Tests are evidence, not automatic acceptance. No requirement to rerun the entire matrix after a prose-only change. No acceptance of an external-action path based solely on a mocked happy path.

## Blockers and time

When a prerequisite fails, record it promptly and complete independent work in the assigned scope. Stop before speculative redesign. Typical blockers: missing Go, incompatible SDK requirement, unavailable OBS instance, absent registered Twitch public client, or inability to perform a required live check.

Keep progress updates concise. Batch corrections. Spend the first cycle proving the foundation rather than polishing it. Prioritize real integrations in P02 and one full repair loop in P04. Cut optional enhancements first; required-scope changes need an explicit user/Codex decision.

## Contribution provenance

Codex plans, directly implements P04 and needed integration fixes, and verifies the resulting product. DeepSeek implements/tests the other assigned packets. Record actual contributions in CONTRIBUTIONS.md and the associated reports. Never claim Codex generated implementation it did not generate. Preserve genuine build/debugging/test footage and actual session identification; do not stage cosmetic edits as evidence of substantive authorship.

The supplied rules permit AI coding assistants but still require a project built with Codex. They do not define a minimum code percentage or explicitly approve this exact split. This workflow creates a concrete Codex implementation role; it cannot promise organizer acceptance or no judging impact.
