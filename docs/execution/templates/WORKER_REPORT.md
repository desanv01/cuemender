# Worker report — TEMPLATE

Copy to handoffs/Pxx-worker-report.md. Replace all placeholders; remove inapplicable template instructions. This is not an acceptance record.

## Identity and ownership

- Packet / revision:
- Worker/harness:
- Started / finished UTC:
- Starting checkpoint or file baseline:
- Result: ready_for_review / blocked
- Editing stopped: yes/no
- Processes stopped: yes/no; remaining PID/purpose if any
- STATUS updated:

## What changed

| Path | Added/modified/deleted | Purpose |
|---|---|---|
| Fill actual paths | | |

No blanket "everything done." List deviations from allowed scope separately.

## Acceptance mapping

| Criterion | Evidence / reproduction | Result |
|---|---|---|
| Packet-specific ID | Actual test/run | pass / fail / blocked / not run |

## Exact checks

| Working directory | Exact command/action | Exit code/result | Relevant evidence |
|---|---|---|---|
| Fill | Fill | Fill | Summary, not invented output |

Distinguish formatting/build/static checks, unit/integration tests, browser checks and live Twitch/OBS checks.

## Environment and dependencies

Toolchain executable/version, OS/architecture, pinned module versions, downloaded tools + official checksum verification, temporary/runtime data paths, keyring test cleanup and environment changes. No secrets or bootstrap URLs.

## Behavior demonstrated

Central success path, meaningful failure paths, persisted/restart behavior and integration prerequisites. List live evidence separately from simulated fixtures.

## Deviations / unresolved issues

Contract questions, incomplete criteria, skipped checks and exact reasons. State the smallest next action or missing prerequisite. No claims that a blocked mandatory gate passed.

## Reviewer reproduction

Ordered steps, commands and expected visible outcomes. Include isolated fixtures/data paths. Describe how to stop any deliberately running process.

## Contribution and handoff

What this worker actually implemented/tested. Note Codex decisions/review artifacts referenced without claiming authorship of work Codex did not do.

Editing is stopped. Next owner: Codex. Provide the short completion prompt in PROMPTS.md.

## Correction history

Append each correction ID, addressed items, changed paths and new evidence here; retain previous failures so the review history is intelligible.
