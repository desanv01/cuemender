# CueMender execution pack

Revision: 1.8 • Updated 2026-11-01 • P01 accepted; P02 local closure/live OBS verified; live Twitch deferred.

## Immediate next action

Codex completed the remaining P02 boundary fixes, local/full race tests and actual hidden-browser/live OBS verification. See [the closure review](handoffs/P02-codex-closure-review.md) and [exact evidence](../implementation/P02-codex-closure.md). **Only live Twitch P02-A/V06 remains unmet**, explicitly deferred by the user because of 2FA/phone setup. All writers and test processes stopped. Do not redispatch old corrections or start P03; follow [STATUS](STATUS.md).

DeepSeek remains the planned implementer for P03/P05/P06 after their release. Codex owns the current P02 closure and directly implements P04 and its tests. This division provides substantive Codex work but is not an organizer guarantee of eligibility.

## Reading order

1. [Shared instructions](../../AGENTS.md)
2. [Current ownership and readiness](STATUS.md)
3. [Workflow](WORKFLOW.md)
4. [Product specification](PROJECT_SPEC.md)
5. [Architecture](ARCHITECTURE.md)
6. [Contracts](CONTRACTS.md)
7. [Assigned packet](packages/P02-live-connections.md)

Consult [decisions](DECISIONS.md), [verification matrix](VERIFICATION.md), [initial workspace baseline](BASELINE.md) and [official sources](SOURCES.md) as needed.

## Execution sequence

| Packet | Outcome | Dispatch state |
|---|---|---|
| [P01 — Foundation](packages/P01-foundation.md) | Runnable local shell, persistent settings, credential boundary, private/public HTTP boundaries | Accepted after P01-R01 |
| [P02 — Live connections](packages/P02-live-connections.md) | Own-channel Twitch chat and OBS discovery/read evidence | Local/live OBS verified; Twitch deferred; P03 held |
| [P03 — Diagnostics](packages/P03-diagnostics.md) | Three evidence-based checks and persistent incidents | Planned |
| [P04 — Approved repair](packages/P04-approved-repair.md) | Codex implements approval, correction, reconciliation and tests | Planned; reserved for Codex |
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
