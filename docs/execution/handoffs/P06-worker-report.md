# P06 worker report

Worker: DeepSeek. Branch: `feature/p06-release-readiness`. Starting HEAD: `e583c740f04dbd6091825cb13482fce12855e3e9`. P06 v1.1 was released in STATUS operational fields; the packet table still said `planned/held` and was stale at dispatch.

Outcome: local P06 v1.1 release candidate, packaging and development-machine extraction verification are ready for Codex review. This is not a published release, not Codex acceptance, and not a clean-machine test. Live authenticated Twitch, real OBS approval and final V18/V19 remain unmet under issues #6/#10.

## P06-A — Reliability and data protection

- Added `internal/datalock`: OS advisory exclusive lock on the effective data directory. Windows uses `LockFileEx`; Unix uses `flock`. Lock is acquired before config/SQLite/listener/provider construction, held for the process lifetime, released on normal Close and on every failed-start cleanup path. No PID-only stale deletion.
- Effective live, `rehearsal/` and `receive-only/LOGIN/` directories are distinct lock owners.
- Tests: `TestP06SameDataDirectoryRejectedAcrossPorts` (same dir/different ports rejected, separate dir succeeds, normal close releases), `TestP06FailedStartupReleasesLock` (failed listener startup releases lock).
- Added transactional `Store.ClearHistory` and authenticated/CSRF `POST /api/v1/history/clear`. It rejects while diagnostics/pending settlement or pending/uncertain actions own recovery, preserves active incidents, deletes only completed incidents and cascades their detail. Coordinator reloads caches after commit and publishes only committed state.
- Tests: `TestP06ClearHistoryPreservesActiveAndPendingRecovery` (pending action blocks and leaves all rows; after resolution 2 completed cleared, active retained), `TestP05ClosureAndRehearsalRouteBoundaries` (clear success/CSRF).
- Existing lifecycle/retention/recovery/shutdown suite preserved.

## P06-B — Reproducible Windows package

- Added `scripts/build-release.ps1` with explicit failure handling, pinned workspace Go, CGo disabled, `GOOS=windows GOARCH=amd64`, `-trimpath -buildvcs=false`, version/commit/UTC ldflags, staging, ZIP and manifest.
- Version is `1.0.0`; commit `e583c740f04d`; build time `2026-11-02T12:01:49Z`.
- Added MIT `LICENSE` (Copyright 2026 Desan Vasu), release quickstart, `THIRD_PARTY_NOTICES.md`, 19 linked-module license files, and patched-goobs Apache license/attribution.
- ZIP manifest `dist/p06-manifest.json`: 25 files, no credentials/databases/config/bootstrap URLs/tests/.tools/private reports.
- Artifact:
  - `dist/cuemender.exe`: 13,822,976 bytes; SHA-256 `3F411D6974DF397BE5C6C95DBD485E13428EB2BAAF7323A15627F625E563CAE2`.
  - `dist/cuemender-windows-x64-1.0.0.zip`: 5,858,204 bytes; SHA-256 `A49F4D349E3521C4E06E0323177183059C62EE9853215E005101F268FBA8EFCF`.
- `go version`: `go1.27.3 windows/amd64`; Node `v24.14.1`; Windows `Microsoft Windows NT 10.0.26300.0`.

## P06-C — Clean-extraction verification

- Extracted ZIP to `.tools/p06 clean extraction` (spaces in path). Shipped executable `--version` succeeded with PATH limited to System32, demonstrating no Go/Node dependency.
- Verified health, bootstrap/session, settings save/reload, public redaction, conflicting-instance rejection on a different port, successful restart after shutdown, receive-only unconfirmed/mismatched confirmation rejection (`400` each), and history clear (`cleared=1`, then `0` incidents).
- Final shipped binary complete rehearsal flow: scenario → report `audio` → proposal → approval → one simulated read-back `verified` → feedback `1 positive/0 negative` → close `local_only`; public projection had no private canaries.
- This is a development-machine extraction, not a fresh-VM/clean-computer test. No real account, OBS write, streaming or recording.
- Runtime data was isolated under `.tools/p06 ...`; owned processes stopped.

## P06-D — Receive-only boundaries

Preserved accepted D37/C12 implementation unchanged: immutable channel CLI, separate storage, explicit confirmation, strict intake, no OAuth ownership claim, no replies/repairs, no repair/reconciliation callbacks, and receive-only labels. Verified private confirmation rejection in the extracted binary (`confirmed=false` and mismatched channel both 400), public `chat_mode=receive_only`, provider `anonymous_irc`, `receive_only=true`, `twitch_authorized=false`. Existing `anonymous_test.go` and Node receive-only assertions remain passing.

## P06-E — Documentation

- Updated README with release installation, SHA statement, receive-only quickstart, authenticated/rehearsal paths, data locations, reset/privacy and troubleshooting; no fabricated release links/badges.
- Updated DEVELOPMENT flags/routes for receive-only, lock, history clear and rehearsal.
- Added `docs/implementation/P06-release.md`.
- Package quickstart distinguishes receive-only, rehearsal and authenticated paths and explicitly states no replies/ownership/repairs.

## Verification

| Check | Result |
|---|---|
| `gofmt -l ./cmd ./internal ./web` | clean |
| `go test ./... -count=1 -timeout=360s` | pass all packages, including new lock/history tests |
| `go vet ./...` | pass |
| `scripts/build-release.ps1 -Version 1.0.0` via process-scoped ExecutionPolicy Bypass | pass; hashes above |
| Six Node fixtures | pass |
| WSL Linux `go test -race ./... -count=1 -timeout=480s` | pass all packages |
| ZIP extraction with spaces + full rehearsal flow | pass, development machine |
| Competing instance same dir/different port | rejected with clear data-directory-in-use error |
| Receive-only unconfirmed + mismatched channel | 400/400 |

## Failures and fixes during this work

- `pwsh` is not installed in this environment; used `powershell.exe`.
- Unsigned `.ps1` was blocked by execution policy; ran with process-scoped `-ExecutionPolicy Bypass`, no machine policy change.
- `Invoke-WebRequest` prompt blocked in NonInteractive; added `-UseBasicParsing`.
- History test initially tried to create an intent action directly; corrected to reserve an action first.
- First package build and extraction preceded the final README; package was rebuilt and re-smoked.

## Remaining limits / unmet gates

- Real authenticated Twitch login/EventSub verification and real OBS approved repair trial remain deferred under #6/#10. D37 actual offline-channel/read-only-OBS evidence is Codex evidence, not broadcast recovery or approved repair.
- Clean-machine testing was not possible; only isolated extraction on the development machine.
- ZIP is not code-signed; SmartScreen may warn. Byte-identical ZIP reproducibility is not claimed because build time/archive metadata vary.
- No CI workflow was added (optional packet item); no external publishing, uploads, Git mutations or recording.
- The packet/STATUS table discrepancy noted at dispatch was not edited beyond operational P06 fields.

## Reproduction

`gofmt -l ./cmd ./internal ./web`; `go test ./...`; `go vet ./...`; `powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/build-release.ps1 -Version 1.0.0 -OutputDir dist`; extract the ZIP to a new space-containing directory; run `cuemender.exe --mode rehearsal --no-browser`; use the printed private link and a control session to run the rehearsal scenario/report/approval/feedback/closure flow. Use an isolated `--data-dir`; stop the process afterward.

P06 v1.1 handoff is ready. Read docs/execution/handoffs/P06-worker-report.md and docs/implementation/P06-release.md; inspect actual changes against P06 v1.1 and CONTRACTS v1.6/C12, verify locking/history-clear, the actual ZIP and clean extraction, dependency notices and receive-only/rehearsal/authenticated boundaries. DeepSeek has stopped editing. Record acceptance or consolidated corrections. Keep #6/#10 and final live/submission gaps explicit; fixtures and offline-channel diagnostics are not broadcast recovery or approved OBS repair evidence.
