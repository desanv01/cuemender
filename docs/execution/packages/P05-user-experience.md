# P05 — Complete interactive experience

Revision: 1.1 • RELEASED to one DeepSeek worker under D34/D35

Starting source: P04 merge `6c09f11178f8bed8230d000cfd2652b74ea68265`; branch `feature/p05-user-experience`. Read STATUS for the documentation checkpoint/ownership. P04 is conditionally accepted for development only. Live Twitch (#6), real OBS approval (#10), dependent real P05 journey and final readiness remain pending. No live setup/mutations. P06 held.

## Outcome and scope

Usable setup, monitoring/evidence/approval/closure controls, quiet sanitized overlay and structurally isolated rehearsal. Go backend and embedded HTML/CSS/vanilla JS only. No new dependencies, SDK patches, framework, backend language, runtime AI, cloud or desktop wrapper.

Read AGENTS; execution README/STATUS/WORKFLOW/PROJECT_SPEC/ARCHITECTURE; CONTRACTS v1.5 C09–C11; D34/D35; P04-review; P05-VISUAL-BRIEF; REPOSITORY_PRESENTATION; worker report template. Existing feedback and repair are implemented: extend their seams, do not recreate them. Needed contract changes go to Codex before dependent work; finish independent work.

Own cmd/app mode flags; app composition/projections; internal/rehearsal; narrow coordinator/store/domain mode/closure extensions; HTTP routes; web/control/overlay; tests and user docs. No Git mutations, parallel workers, real services, recording, foreground computer control or original-planning-file edits.

## P05-A — Preflight and visual implementation

1. Check branch/HEAD/diff and released packet; claim sole ownership in STATUS. Worker cannot switch branches. Record Go/Node/WSL/browser availability; use installed portable tooling/caches.
2. Implement the written visual brief. Codex inspected the actual screen. 12ui generation returned budget exhausted after authorized retry; no generated kit exists. Do not claim a missing skill/tool ran. Preserve visual language and implement functional hierarchy, not speculative redesign.
3. Preserve existing setup handlers/IDs or update all fixtures coherently. Keep behavioral assertions when adapting markup. Focus, edits, selectors and active section survive SSE/reload/errors.

## P05-B — Setup and readiness

1. Separate account, OBS, broadcast, monitoring and credential-storage states. List missing prerequisites and link to the right setup section. Streaming is not required to configure/read.
2. Preserve Public-client device authorization, validation/refresh, endpoint/password reuse, explicit OBS Connect after restart, stable mic/scene discovery and explicit track confirmation. Secrets/private launch links never enter reports/logs/public UI.
3. Public overlay URL/copy with selectable fallback; accessible labels/errors; retain nonsecret edits on recoverable failures. Clear submitted secret inputs and never persist them in browser storage.
4. Real-mode monitoring remains blocked by missing Twitch readiness; manual read-only checks stay available. No canned readiness, bypass flag, automatic monitoring or mode-switch API.

## P05-C — Incidents, approval, feedback and closure

1. Show expectation vs observation, timestamp, origin and unknown/stale/partial limits. Keep remaining findings, especially mute, after track verification. Local read-back is not playback recovery.
2. Use existing approval route/engine. Show TTL/busy/conflict/expired, target and preserved assignments. Prevent repeated-click confusion while keeping backend idempotency authoritative. On lost response, reload state; do not assume success or create another action.
3. Preserve existing normalized !fixed/!stillbroken intake, latest-vote replacement, bounds, generation/session and bounded replies. Display positive/negative separately. Rehearsal feedback is visibly simulated.
4. Implement C11 close route/service/store transaction with explicit outcome, atomic feedback eligibility, pending job/retry/unknown action rejection, unused-proposal invalidation and preserved audit. Same outcome is idempotent; DB failure changes no cache/UI. Add typed closure data/migration 4 as needed, preserving previous DB versions.
5. Test feedback/close concurrency, repeat closure, stop/restart/old session, pending repair and persistence failure. Use existing coordinator gate, not another authority.

## P05-D — Public overlay

1. Replace count-only display with fixed stage/category copy and public code: checking, manual attention, awaiting private approval, local configuration verified, unknown/unresolved and viewer reports.
2. Explicit allowlist only; no names/UUIDs/maps/actor IDs/arbitrary errors/private URLs. Canary-test new fields via GET and SSE in both modes.
3. Compact transparent idle/active presentation; disconnected never means healthy. Rehearsal watermark persists even idle. Accessibility/contrast/reduced motion follow the brief.

## P05-E — Isolated rehearsal

1. Implement C11 CLI/mode/effective data directory and DB ownership before provider construction. Verify live default/legacy DB compatibility, invalid mode rejection, conflicting mode failure without user-data changes. Rehearsal child path never reads base live config/DB/keyring.
2. Compose in-process fixtures before any live manager/secret store. Narrow interfaces are allowed where concrete fields require separation; preserve live adapter behavior. Fake providers are bounded, cancelable and generation-aware. Zero live sender/dialer/keyring construction or calls.
3. Propagate trusted mode through coordinator/store/domain/proposals. Reuse actual diagnostics, persistence, approval and read-only recovery. Generalized validation compares constructor-owned mode with provenance; preserve existing live-only default/wrapper. Forged mode cannot obtain rehearsal privileges in live.
4. Fixed private scenario/report/feedback routes follow C11 security/schema. Stop/invalidate previous simulation before scenario selection; reject owned/in-flight/unknown work rather than resetting underneath it. No automatic monitoring/approval. One bounded synthetic viewer illustrates replacement votes.
5. Cover track_missing → explicit approval → one fake write/readback; mic_muted manual-only; wrong_scene limitation; unknown_write → read-only reconciliation without a repeat write. UI/overlay/reports always say REHEARSAL. Live credential/setup routes reject in rehearsal.
6. Isolation tests exercise actual App construction with live factory/keyring/dial/send counters or fail-fast sentinels, all zero; include live-looking config/forged IDs, cross-mode directory, CLI restart, fake-generation replacement and bounded shutdown with active SSE/blocked reads. Mock labels alone are insufficient.

## P05-F — Verification and user documentation

- Meaningful Go cases: closure predicates/transaction/concurrency; feedback preservation; mode ownership and migration from accepted schema 3 (plus existing old migrations); CLI validation; actual composition isolation; strict private routes; forged/cross-mode IDs; scenario ownership; single write/no replay; settings/provider/stop/shutdown boundaries.
- Preserve all five P01–P04 Node actual-script fixtures; add tab/error/focus, closure, overlay redaction/copy, rehearsal and SSE tests. Adapt DOM fixtures with equivalent/stronger assertions and explain changes; never delete regressions because markup changed.
- Run `go test ./... -count=1`, `go vet ./...`, `gofmt -l cmd internal web`, and CGo-free `go build -trimpath -o dist/cuemender.exe ./cmd/app`. Set development version 0.5.0-p05; record hash/size/version. Run full configured WSL race with a practical recorded timeout. No system installation to remove a skipped label.
- Extend actual-App local OAuth/Helix/EventSub/OBS protocol fixtures for separate-viewer feedback and closure, not direct readiness/callback-only shortcuts. Rehearsal is separate evidence, not a replacement for production-composition fixtures.
- Headless/local browser if available: setup/errors/reload, protocol report→approve→feedback→close, rehearsal scenarios, keyboard/focus/mobile/200% zoom, overlay idle/active/reconnect/watermark. Use fictional data and ignored auth state; close processes/remove owned auth files. If unavailable give precise reproduction and keep visual gate unverified. Node is not visual evidence.
- Update README and docs/DEVELOPMENT.md with actual behavior and limits; create docs/implementation/P05-user-experience.md for setup/modes/directories/scenarios/testing. Follow existing ErrataScope-inspired structure. No fake screenshots, release/CI badges, license selection or external publication. Label rehearsal/protocol imagery honestly.

## Acceptance and handoff

| Gate | Evidence |
|---|---|
| P05-A / V14 local | Rendered setup→report→evidence→approve→local result→separate feedback→closure against local protocols; real journey remains deferred |
| P05-B | Rendered keyboard/contrast/error/reconnect/mobile checks; quiet overlay and no lost edits/focus |
| P05-C / V15 | Actual rehearsal composition has zero live adapter/keyring use; isolated data/provenance/restart tested |
| P05-D | Local vs viewer vs simulated copy is clear; public GET/SSE allowlist checks |
| P05-E | Prior regressions preserved; full Go/vet/build/Node/race; no expanded mutation |

Write docs/execution/handoffs/P05-worker-report.md using the template, mapping P05-A through F and acceptance gates to files/commands/results. Distinguish self-tests/local protocols/rendered browser/rehearsal/unmet live. Include changed paths, versions, migration, failures/fixes, limitations and stopped processes. No credentials/raw chat/launch links. Update only operational STATUS to ready_for_review or a specific blocker, owner Codex, writer none. Stop editing; P06 held. Return the exact handoff in PROMPTS.md. Codex inspects and accepts or issues one consolidated correction.
