# P06 — Release readiness

Revision: 1.0 • PLANNED / HELD
Dispatch requires: P05 accepted; no known unresolved unsafe mutation behavior; explicit STATUS release.

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
- Package livecheck.exe, README quickstart, LICENSE only after owner selection, dependency notices and approved demo assets into a Windows ZIP.
- ZIP excludes source secrets, runtime DB/config, .tools and private reports with sensitive environment details.
- Produce SHA-256 for local release artifact and record build command/tool versions.
- CI may include Windows build/test and a supported race-test job (for example Linux with a C compiler), with test design avoiding real keyring/network credentials. Worker does not create external repo secrets or trigger paid services.

### P06.3 — Clean run and real end-to-end trial

- Extract into a new directory with spaces; run without repository-relative files or a frontend/dev server.
- Verify runtime uses no Go/Node binaries. A clean-machine test is stronger; state accurately if only a dev-machine isolated run was performed.
- Use real registered Twitch app and dedicated OBS test setup.
- Walk report -> finding -> approval -> read-back -> separate-viewer feedback -> explicit closure.
- Exercise one disconnect and one stale approval using safe fixtures/dedicated setup.
- Record OBS version, Windows version and tested constraints, not unsupported platform claims.

### P06.4 — Documentation and handoff

- Follow [REPOSITORY_PRESENTATION.md](../REPOSITORY_PRESENTATION.md): the user's ErrataScope README/About layout is the reference, adapted to CueMender's actual features, screenshots and Go distribution.
- Explain registration/client ID, private authorization, OBS WebSocket setup, track declaration, overlay setup, monitoring, privacy/data location and uninstall/data cleanup.
- Troubleshooting: missing permissions, token expiry, OBS password failure, unknown sources, wrong declared track, port conflict, keyring unavailable and unknown action outcome.
- Provide labeled rehearsal with no account requirements plus actual live setup instructions.
- Draft a factual demo script and contribution ledger update for Codex/user review; no fabricated session ID.
- Summarize all M01–M14 / V01–V18 coverage and remaining gaps.

## Acceptance

- P06-A / V16: Shipped ZIP runs from clean extraction and docs match behavior.
- P06-B / V17: One data directory cannot be used by competing writers; shutdown/restart correct.
- P06-C / V18: Real complete loop and appropriate race/build checks recorded.
- P06-D: No secrets or inappropriate assets in artifact; licenses/source requirements identified.
- P06-E: Known limitations and skipped checks visible; no false complete claim.

Codex reviews actual artifact and submission readiness. Worker stops. Uploading YouTube, publishing GitHub or submitting Devpost is outside this packet.
