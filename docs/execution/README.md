# CueMender execution pack

Revision: 2.6 • Updated 2026-11-02 (Malaysia) • P03 accepted locally; live Twitch deferred.

## Immediate next action

**P03 local implementation is accepted.** Read the [closure review](handoffs/P03-codex-closure-review.md) and [implementation/verification record](../implementation/P03-codex-closure.md). R06 plus Codex takeover completed the remaining ordering, retry, socket cleanup and concurrent test work. Actual headless-browser checks passed. No DeepSeek dispatch is active.

P02-A/V06 and P03-D real Twitch remain deferred under D30/D31 in [issue #6](https://github.com/desanv01/cuemender/issues/6), required before final readiness. Do not restart Twitch account work. P04 remains held for explicit release and its genuine implementation recording checkpoint. DeepSeek remains stopped; Codex owns GitHub integration through PR #9. This is local source acceptance, not an eligibility guarantee or release/submission readiness.

## Reading order

1. [Shared instructions](../../AGENTS.md)
2. [Current ownership and readiness](STATUS.md)
3. [Workflow](WORKFLOW.md)
4. [Product specification](PROJECT_SPEC.md)
5. [Architecture](ARCHITECTURE.md)
6. [Contracts](CONTRACTS.md)
7. [Assigned packet](packages/P03-diagnostics.md)

Consult [decisions](DECISIONS.md), [verification matrix](VERIFICATION.md), [initial workspace baseline](BASELINE.md) and [official sources](SOURCES.md) as needed.

## Execution sequence

| Packet | Outcome | Dispatch state |
|---|---|---|
| [P01 — Foundation](packages/P01-foundation.md) | Runnable local shell, persistent settings, credential boundary, private/public HTTP boundaries | Accepted after P01-R01 |
| [P02 — Live connections](packages/P02-live-connections.md) | Own-channel Twitch chat and OBS discovery/read evidence | Development baseline conditionally accepted under D30; Twitch deferred |
| [P03 — Diagnostics](packages/P03-diagnostics.md) | Three evidence-based checks and persistent incidents | Accepted local baseline under D30/D31 |
| [P04 — Approved repair](packages/P04-approved-repair.md) | Codex implements approval, correction, reconciliation and tests | Held for explicit release and recording; reserved for Codex |
| [P05 — User experience](packages/P05-user-experience.md) | Finished setup, controls, overlay, confirmation, rehearsal | Planned |
| [P06 — Release readiness](packages/P06-release-readiness.md) | Tested Windows ZIP and reproducible instructions | Planned |
| [Final integration/submission](SUBMISSION.md) | Codex review, actual demonstration, accurate entry | Codex/user stage |

Later packets contain tasks, failure cases and acceptance gates now. Their exact file impact and verified SDK details are finalized before dispatch. This avoids treating untested API assumptions as facts.

## Authority and changes

The user's current instruction governs. Within this pack, DECISIONS records approved deviations; PROJECT_SPEC owns behavior, CONTRACTS owns shared semantics, ARCHITECTURE owns component boundaries, and each packet owns its implementation scope. A contradiction is a blocker for the affected task, not permission for a worker to pick whichever version is easiest.

Workers do not edit authoritative specifications. Record a proposed deviation in the report. Codex updates affected documents and revision numbers before redispatch.

## Handoff files

Use the templates in [templates](templates/WORKER_REPORT.md). Actual reports and review records go in handoffs; consolidated corrections go in corrections. Placeholder acceptance reports have deliberately not been created.

Prompts for launch, resume, correction and acceptance review are in [PROMPTS.md](PROMPTS.md). Each later launch is conditional on STATUS marking that packet released.

Record actual implementation and verification provenance in [CONTRIBUTIONS.md](CONTRIBUTIONS.md). Planned ownership is not proof that work has already happened.

The user selected CueMender / public cuemender. [GIT_WORKFLOW.md](GIT_WORKFLOW.md) records authorized Git/GitHub actions and their timing.

Use the inspected [ErrataScope presentation reference](REPOSITORY_PRESENTATION.md) for CueMender's README, About, screenshots and public documentation layout.

## Time discipline

Prioritize a real Twitch event and correct OBS reads immediately after P01, then the complete approved-repair flow. Preserve time for a real demonstration, packaging and submission. No full redesign or optional features until required acceptance gates pass. A short foundation is useful; a generic framework is not.
