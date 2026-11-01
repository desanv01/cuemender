# Codex acceptance review — TEMPLATE

Copy to handoffs/Pxx-review.md. Only Codex writes acceptance.

- Packet/revision:
- Review date UTC:
- Worker report reviewed:
- Actual file/commit baseline:
- Worker confirmed stopped:
- Code authorship: DeepSeek / Codex / mixed; identify any Codex fixes
- Verification type: separate review of worker code / self-verification of Codex code / mixed
- Decision: accepted / changes_required / blocked

## Evidence inspected

Actual files/diffs, contract-sensitive paths, checks reproduced, worker evidence relied upon, and why additional repeated testing was unnecessary or needed. Distinguish reproduction of worker evidence from checking code this same Codex chat authored; do not label the latter independent review.

## Acceptance coverage

Criterion -> result -> evidence. Identify limitations rather than hiding them under a passing aggregate test count.

## Findings

Numbered findings with concrete location, behavior, impact and expected correction. Separate blocking issues from deferred suggestions that do not change scope.

## Outcome and next action

If accepted: checkpoint identifier if created, STATUS update, next packet revision/release and actual prerequisite needs.
If corrections: consolidated correction file and precise retests.
If blocked: exact external prerequisite and what can be continued independently.

Record truthful Codex contribution to debugging/review/changes for later submission evidence.
