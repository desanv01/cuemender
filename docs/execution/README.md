# CueMender execution pack

Revision: 3.0 • Updated 2026-11-02 (Malaysia) • P05 accepted locally; live checks deferred.

## Immediate next action

D37/C12: Codex completed and accepted the [receive-only fallback](ANONYMOUS-CHAT.md), verified with real offline-channel Twitch messages and read-only local OBS. Original EventSub/repair gates remain deferred. [P06 v1.1](packages/P06-release-readiness.md) now packages this path alongside EventSub and rehearsal; check STATUS for release. No DeepSeek writer is active.

**Current: P06 v1.1 is ready for one DeepSeek worker after D37 acceptance.** P05 atomic scenario replacement, typed intake, simulated-provider guards and rendered controls remain verified in the [closure record](handoffs/P05-codex-closure.md). Read STATUS and the P06 launch prompt; no worker is active yet. The user's retained [live-verification plan](LIVE-VERIFICATION.md) stays pending until the 2FA hold is released. Original release summary below is historical context.

**Historical P05 dispatch: v1.1 was released to DeepSeek under D34/D35 and contracts v1.5/C11.** P04 is conditionally accepted for development and merged via PR #11; genuine build footage is saved. Read [STATUS](STATUS.md), [P05 packet](packages/P05-user-experience.md), [written visual brief](P05-VISUAL-BRIEF.md) and [PROMPTS](PROMPTS.md). P06 stays held. No P05 application code was written during dispatch preparation.

P03 local acceptance is documented in its [closure review](handoffs/P03-codex-closure-review.md); PR #9 merged and #7/#8 closed. P02-A/V06 and P03-D real Twitch remain deferred in [issue #6](https://github.com/desanv01/cuemender/issues/6). P04's active-monitoring approval gate is retained, so dependent full live approval verification may remain unmet. Track P04 in [issue #10](https://github.com/desanv01/cuemender/issues/10). No live Twitch login work or simulated-as-live claims.

## Reading order

1. [Shared instructions](../../AGENTS.md)
2. [Current ownership and readiness](STATUS.md)
3. [Workflow](WORKFLOW.md)
4. [Product specification](PROJECT_SPEC.md)
5. [Architecture](ARCHITECTURE.md)
6. [Contracts](CONTRACTS.md)
7. [Assigned packet](packages/P05-user-experience.md) and [visual brief](P05-VISUAL-BRIEF.md)

Consult [decisions](DECISIONS.md), [verification matrix](VERIFICATION.md), [initial workspace baseline](BASELINE.md) and [official sources](SOURCES.md) as needed.

## Execution sequence

| Packet | Outcome | Dispatch state |
|---|---|---|
| [P01 — Foundation](packages/P01-foundation.md) | Runnable local shell, persistent settings, credential boundary, private/public HTTP boundaries | Accepted after P01-R01 |
| [P02 — Live connections](packages/P02-live-connections.md) | Own-channel Twitch chat and OBS discovery/read evidence | Development baseline conditionally accepted under D30; Twitch deferred |
| [P03 — Diagnostics](packages/P03-diagnostics.md) | Three evidence-based checks and persistent incidents | Accepted local baseline under D30/D31 |
| [P04 — Approved repair](packages/P04-approved-repair.md) | Codex approval, correction, reconciliation and tests | Conditional development acceptance D34; required real approval pending |
| [P05 — User experience](packages/P05-user-experience.md) | Setup, controls, overlay, closure, isolated rehearsal | Accepted locally after Codex closure; dependent real journey deferred |
| [P06 — Release readiness](packages/P06-release-readiness.md) | Tested Windows ZIP and reproducible instructions | v1.1 ready under D37; check STATUS |
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
