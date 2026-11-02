# P06 — Release readiness

Revision: 1.1 • PREPARED / dispatch requires explicit STATUS release under D37
Dispatch requires: P05 accepted locally, D37/C12 fallback accepted by Codex, no active writer and explicit STATUS release. Read ANONYMOUS-CHAT.md and its closure record before work. The fallback is implemented by Codex; do not rebuild it or broaden its permissions.

## Outcome and ownership

A reproducible Windows x64 release ZIP, honest run instructions, complete required verification and documented limitations. Worker prepares local artifacts only. Codex owns Git/GitHub and external publishing.

Own release packaging script, README/setup/troubleshooting, original test fixtures, targeted reliability corrections, license inventory and CI configuration. No unrelated architecture rewrite.

## Tasks

### P06.1 — Close required reliability gaps

- Review VERIFICATION rows and prior skipped checks; finish actual prerequisites or list remaining blockers.
- Implement/check same-data-directory single-instance protection even when listen ports differ. Stale lock handling must not delete a live process's lock; use an OS-supported locking approach compatible with the selected stack.
- Exercise shutdown with sockets/SSE/action reconciliation, SQLite contention, rejected settings saves, lost network and restart.
- Apply retention/history-clear policy without deleting unresolved action recovery evidence.
- Inspect credentials/logs/public projections and committed fixtures for leaks.
- Reuse existing tests; add only tests for newly addressed failure modes.

### P06.2 — Artifact and build repeatability

- Build with pinned Go/modules, CGo disabled for distribution, embedded assets and version metadata.
- Package cuemender.exe, README quickstart, the user-selected MIT LICENSE (copyright 2026 Desan Vasu), dependency notices and only approved demo assets into a Windows ZIP. Include the patched goobs license and patch/source attribution; inventory all redistributed dependencies and preserve their required notices. Do not guess licenses.
- ZIP excludes source secrets, runtime DB/config, .tools and private reports with sensitive environment details.
- Produce SHA-256 for local release artifact and record build command/tool versions.
- CI may include Windows build/test and a supported race-test job (for example Linux with a C compiler), with test design avoiding real keyring/network credentials. Worker does not create external repo secrets or trigger paid services.

### P06.3 — Clean run and real end-to-end trial

- Extract into a new directory with spaces; run without repository-relative files or a frontend/dev server.
- Verify runtime uses no Go/Node binaries. A clean-machine test is stronger; state accurately if only a dev-machine isolated run was performed.
- Preserve three documented paths: authenticated EventSub with real approval (live gates deferred), receive-only IRC with real local diagnostics and no replies/repairs, and clearly labeled isolated rehearsal.
- Verify the final binary parses --receive-only-channel, isolates its data, refuses invalid/rehearsal combinations, protects the confirmation route and never enables real approval. Use local fixtures; do not log in, send real chat or touch OBS during worker execution. Codex owns any later live check.
- Exercise report -> finding -> approval -> read-back -> feedback -> closure using labeled rehearsal and existing protocol fixtures. Real authenticated end-to-end V18/V19 remains unmet under #6/#10; do not claim this local sequence closes it.
- Exercise one disconnect and one stale approval using safe fixtures/dedicated setup.
- Record OBS version, Windows version and tested constraints, not unsupported platform claims.

### P06.4 — Documentation and handoff

- Follow [REPOSITORY_PRESENTATION.md](../REPOSITORY_PRESENTATION.md): the user's ErrataScope README/About layout is the reference, adapted to CueMender's actual features, screenshots and Go distribution.
- Explain the receive-only quickstart first as the available no-developer-app path: cuemender.exe --receive-only-channel YOUR_LOGIN; confirm the channel in private Setup, connect OBS, select/confirm expectations, start monitoring, and send !issueaudio from a separate viewer. No automatic replies, ownership proof or OBS repairs. Data is below receive-only/LOGIN; never ask users to transplant authenticated databases.
- Also explain authenticated registration/client ID, private authorization, OBS setup, track declaration, overlay setup, monitoring, privacy/data location and uninstall/data cleanup. A channel being offline does not itself prevent chat intake; viewers still need account/chat permission. TLS IRC uses port 6697.
- Troubleshooting: missing permissions, token expiry, OBS password failure, unknown sources, wrong declared track, port conflict, keyring unavailable and unknown action outcome.
- Provide labeled rehearsal with no account requirements plus actual live setup instructions.
- Draft a factual demo script and contribution ledger update for Codex/user review; no fabricated session ID.
- Summarize all M01–M14 / V01–V18 coverage and remaining gaps.

## Acceptance

- P06-A / V16: Shipped ZIP runs from clean extraction and docs match behavior.
- P06-B / V17: One data directory cannot be used by competing writers; shutdown/restart correct.
- P06-C / V18: Required local release checks recorded, with real authenticated complete loop explicitly deferred/unmet under #6/#10. Keep D37 actual offline-channel/read-only evidence distinct from protocol fixtures, rehearsal, broadcasting and repair.
- P06-D: No secrets or inappropriate assets in artifact; licenses/source requirements identified.
- P06-E: Known limitations and skipped checks visible; no false complete claim.

Codex reviews actual artifact and submission readiness. Worker stops. Uploading YouTube, publishing GitHub or submitting Devpost is outside this packet.

## Execution details and evidence

1. Preflight: inspect Git status read-only, STATUS ownership, D37 closure and current toolchain. Do not overwrite Codex work. Claim P06 only when released.
2. Reliability: add same-effective-data-directory process exclusion (even on different HTTP ports), using OS advisory locks via existing x/sys or standard facilities; test same-directory rejection, separate-directory success and release after normal/failed startup. Lock the actual selected live/rehearsal/receive-only directory. No PID-only stale-lock deletion or CGo requirement.
3. History: implement the reserved authenticated/CSRF history-clear route if still absent, following C07 exactly. Keep active/unresolved incidents, sessions and any pending/unknown action recovery evidence. Transactional clear and coherent cache publication; failed clear must preserve state. Avoid sweeping all application data or removing credentials.
4. Packaging: use PowerShell-safe build/ZIP script in scripts/, exact module graph/local patched SDK, version/commit/UTC build-time ldflags for internal/app, CGo disabled, explicit error exits. Include a dependency license inventory and build instructions, no fabricated release links/badges. Do not bump module dependencies.
5. Clean extraction: new path containing spaces, no repo-relative assets. Exercise health, bootstrap, settings save/reload, public redaction, receive-only confirmation rejection, isolated rehearsal full loop and clean shutdown using the shipped binary. Record whether this is the development machine; do not call it a pristine-machine test.
6. Verification: gofmt, full Go suite, vet, production build, all six Node fixtures (including added receive-only assertions), configured WSL race suite. Reuse prior low-risk evidence but test new locking/history/packaging behavior directly. No real credentials in fixtures.
7. Report: docs/implementation/P06-release.md and docs/execution/handoffs/P06-worker-report.md; exact versions, commands/exits, ZIP and executable hashes, archive manifest, clean-run evidence, dependency/license inventory and every deferred gate. Stop owned processes; transfer to Codex ready_for_review.

External publishing, release uploads, PR/merge/issue mutations, video/YouTube/Devpost and live-account actions remain Codex/user responsibilities. No new framework, UI redesign, transport, feature expansion, permission bypass or approval-policy change.
