# P06 release readiness

Revision: P06 v1.1 local packaging implementation. This is a development-machine release candidate, not a published release. Codex owns final acceptance, Git/GitHub and external publishing.

## Codex acceptance corrections

The worker evidence below describes the original candidate. Codex's final review
is [P06-review.md](../execution/handoffs/P06-review.md). History clear now returns
the exact deleted IDs from its transaction and atomically removes those cache
entries after commit; no fallible reload can turn a committed deletion into a
reported failure. The packaging script resolves its own working directory, uses
a fresh staging directory without recursive deletion, labels dirty builds,
supports optional `-Offline` rather than requiring an existing cache, and embeds
BUILD-MANIFEST.json plus writes SHA256SUMS.txt. Go runtime and modernc bundled
third-party/SQLite notices were added. The corrected archive has 30 files.
Original hashes below are historical; use the final archive's manifest and
adjacent SHA256SUMS.txt, with the final review receipt, for the accepted artifact.

## Reliability and data protection

### Single-instance data-directory lock

Every effective data directory is locked before configuration, SQLite, listeners or providers are opened. The lock is an OS advisory exclusive lock on `<effective-data-dir>/.cuemender.lock`:

- Windows: `LockFileEx` with `LOCKFILE_EXCLUSIVE_LOCK|LOCKFILE_FAIL_IMMEDIATE`.
- Linux/other Unix: `flock(LOCK_EX|LOCK_NB)`.
- The lock is released on normal `App.Close` and by deferred cleanup after any failed startup.
- A stale lock file alone is not deleted and does not block a later process; only a live OS lock blocks.
- Live, `rehearsal/`, and `receive-only/LOGIN/` are separate effective directories and therefore independently locked.

Tests: `TestP06SameDataDirectoryRejectedAcrossPorts` (same directory/different ports rejected; separate directory succeeds; normal close releases), `TestP06FailedStartupReleasesLock` (occupied listener startup failure releases lock).

### History clear

`POST /api/v1/history/clear` is authenticated, CSRF-protected, strict empty JSON and follows C07. `Store.ClearHistory` is transactional and refuses the entire operation if any action is pending/uncertain or if diagnostics/pending settlement own work. It deletes only completed incidents (`dismissed`, `closed`, `expired`, `invalidated`), cascading their detail; active/unresolved incidents and action recovery evidence remain. Coordinator reloads and clears proposal/action caches after commit; failure leaves prior state. Tests: `TestP06ClearHistoryPreservesActiveAndPendingRecovery`, `TestP05ClosureAndRehearsalRouteBoundaries` (CSRF/route).

## Package

- Script: `scripts/build-release.ps1`.
- Toolchain: workspace Go 1.27.3, `CGO_ENABLED=0`, `GOOS=windows`, `GOARCH=amd64`, `GOTOOLCHAIN=local`, `GOPROXY=off`, `-trimpath -buildvcs=false`.
- Version/commit/build time injected into `internal/app` via ldflags.
- Output: `dist/cuemender.exe`, `dist/cuemender-windows-x64-1.0.0.zip`, `dist/p06-manifest.json`.
- ZIP contents: executable, README, quickstart, MIT LICENSE, third-party notices, 19 dependency license files, patched-goobs attribution.
- No databases, config, credentials, bootstrap URLs, `.tools`, tests or private reports are included.
- Script exits nonzero on build/archive failure. The script is unsigned; run it with `powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/build-release.ps1` when policy blocks unsigned scripts.

Recorded local artifact:

- Version `1.0.0`; commit `e583c740f04d`; build time `2026-11-02T12:01:49Z`.
- `cuemender.exe`: 13,822,976 bytes; SHA-256 `3F411D6974DF397BE5C6C95DBD485E13428EB2BAAF7323A15627F625E563CAE2`.
- ZIP: 5,858,204 bytes; SHA-256 `A49F4D349E3521C4E06E0323177183059C62EE9853215E005101F268FBA8EFCF`; 25 files.

`THIRD_PARTY_NOTICES.md` was generated from modules actually linked into `./cmd/app`; each entry points to a preserved upstream license file. The patched goobs source is Apache-2.0 with the original license and `CUEMENDER_PATCH.md` attribution included as `GOOBS_PATCH_ATTRIBUTION.md`.

## Clean extraction verification

Extracted the actual ZIP into `.tools/p06 clean extraction` (path contains spaces) and ran the shipped executable. Binaries/assets required no repository-relative files, Go or Node (`--version` succeeded with PATH limited to System32). Verified health, bootstrap, settings save/reload, public redaction, isolated rehearsal, competing-instance rejection (different port), receive-only unconfirmed/mismatched confirmation 400, clean shutdown/restart, and history clear (1 completed cleared, 0 incidents remaining). The final ZIP extraction was also run through the complete rehearsal report → finding → approval → one simulated read-back → feedback → closure flow; public projection had no private canary. This is a development-machine extraction, not a pristine clean-machine test.

## Receive-only and authenticated boundaries

Receive-only is preserved from D37/C12: immutable channel launch, separate storage, explicit private confirmation, strict intake, no OAuth ownership claim, no automatic replies, no repair/reconciliation executor, and public/private receive-only labeling. Tests remain in `internal/app/anonymous_test.go` and the control/overlay Node fixtures. Codex’s real offline-channel/read-only-OBS evidence remains separate from fixtures/rehearsal and does not establish broadcast recovery, authenticated EventSub or an approved OBS repair.

## Known limits

- No authenticated Twitch EventSub live journey or real OBS approved repair trial was performed; issues #6/#10 remain open.
- The extraction test is on the development machine, not a fresh VM/clean computer.
- The ZIP has no code signature; Windows SmartScreen may warn. Build time and archive timestamps mean byte-for-byte ZIP reproducibility is not claimed, though the source/build recipe is pinned.
- `THIRD_PARTY_NOTICES.md` is generated from the current linked module graph; dependencies not linked into the executable are intentionally excluded. No CI workflow was added; optional CI is left to Codex.
