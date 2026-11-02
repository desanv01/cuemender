# P06 v1.1 — Codex acceptance review

Date: 2026-11-02. Decision: **accepted for local Windows release scope under D37**.
This is not authenticated Twitch/real OBS approval acceptance or final submission readiness.
DeepSeek stopped before review. Codex was the only writer for the corrections below;
their checks are self-verification, separate from review of the worker implementation.

## Changes made during acceptance

1. History clearing originally committed deletion, then performed fallible cache
   reloads (including redundant repair reload). A post-commit read failure could
   return an error after deletion and leave publication incomplete. The store now
   returns exact deleted IDs from `DELETE ... RETURNING` only after successful
   commit. The coordinator's operation gate guards a single cache removal across
   incidents/order/proposals/actions, followed by publication, with no post-commit
   database reads. Pending diagnostics, settlements and unknown actions still block.
2. Added ordinary regressions for durable/cache/publication agreement, preservation
   of unresolved entries, canceled/busy admission and a failed SQL delete followed
   by successful retry. Existing action-recovery preservation tests remain passing.
3. Packaging now establishes its own working directory, verifies Go 1.27.3, supports
   fresh-clone dependency resolution (optional `-Offline` for cached builds), marks
   dirty source, and uses a fresh staging directory without recursive deletion.
   It embeds a source/build/per-file checksum manifest and writes SHA256SUMS.txt.
4. Added omitted Go runtime and upstream modernc libc/SQLite bundled notices,
   including musl notices and SQLite dedication. Corrected stale README/license/
   development claims and documented three distinct execution paths.

## Executed verification

Working directory: project root unless stated. Portable Go 1.27.3 on Windows,
Node v24.14.1, existing Linux Go 1.27.3/WSL compiler environment. No dependency
version changes, live service calls, OBS edits, account work or recording.

- `go test ./... -count=1 -timeout=360s`: PASS, all packages, with local socket access.
- `go vet ./...`: PASS. `gofmt -l cmd internal web`: clean.
- All six `testdata/browser/*.js` via Node: PASS (actual-script fixtures, not rendered UI).
- WSL `CGO_ENABLED=1 go test -race ./... -count=1 -timeout=480s`: PASS, no race reports.
  Used existing `.tools/linux-go1.27.3/go`, Linux build cache and shared populated
  `.tools/gopath/pkg/mod`; production builds remain CGo-free.
- Packaging invoked from `.tools`, outside the repository root: PASS. The review
  archive correctly identifies `e583c740f04d-dirty`; this is a review candidate,
  not the final source identity. Final distribution must be rebuilt after the
  reviewed source is committed, then its manifest/hashes checked again.
- Actual ZIP extraction into a newly created path containing spaces: PASS.
  All 29 payload files match the embedded manifest; 30 archive files including
  that manifest. Checked for unexpected paths/runtime databases/key files/config.
- Shipped binary ran with PATH restricted to System32: version, health and
  full private HTTP rehearsal scenario/report/proposal/approval/readback/feedback/
  closure all passed. This is development-machine extraction, not a fresh VM.
- History route: unauthenticated 401, missing-CSRF 403, unknown field 400;
  successful clear removed incident/proposal/action projections and stayed cleared
  after process restart, retaining settings revision. Public projection excluded
  private microphone/config/CSRF fields.
- Real separate processes: same effective directory/different ports rejected;
  a distinct directory ran concurrently; forced termination released the OS lock
  and the same directory reopened. Graceful shutdown/failed-start cleanup and
  active HTTP/SSE/blocked-provider lifecycle are covered by the passing Go suite.
- Receive-only extracted binary: unconfirmed/mismatched channel 400; OAuth,
  approval and rehearsal-scenario routes rejected 409. Separate channel database,
  `anonymous_irc` capability state and public receive-only label preserved.
  Invalid rehearsal+receive-only launch rejected. No real connection attempted.

## Failed attempts and provenance

- Initial sandboxed Windows suite could not dial its own loopback fixtures
  (`connectex ... forbidden by its access permissions`). Approved rerun with local
  socket access passed; those environment failures were not patched out of tests.
- First WSL invocation selected an empty module cache while offline and failed
  before the affected tests ran. Corrected invocation used the existing populated
  cache and the full suite passed.
- Initial artifact smoke expected approval HTTP 200; the existing contract returns
  asynchronous 202. Corrected the test harness expectation, not product behavior;
  full rerun passed. Owned subprocesses were stopped even on failed attempts.

## Gate disposition

P06-A/V16 clean extraction, P06-B/V17 locking, P06-C required local release
checks, P06-D artifact notices/content, P06-E truthful limitations: accepted locally.
Final committed-build identity/checksum verification belongs to the release
checkpoint immediately after this review; final hash receipts travel with the
artifact, rather than recursively embedding an artifact's hash into its source.

Still pending: authenticated EventSub #6, approved real OBS repair #10 and dependent
real V14/V18/V19. D37 offline-channel/local-OBS evidence is distinct from broadcast
recovery. No fresh rendered-browser or live OBS run in this review. No pristine
machine/VM test, code signing, byte-identical ZIP claim or configured CI workflow.
Video editing, public YouTube and Devpost submission remain separate tasks.

Next: Git checkpoint/PR/merge, clean-source package rebuild plus artifact recheck,
then the saved post-P06 recording/submission gameplan. No new worker packet.
