# P01-R01 acceptance review

- Date: 2026-11-01
- Reviewer: Codex, current main chat
- Scope: P01 v1.1 plus P01-R01 v1.0
- Code author: DeepSeek; Codex reviewed the source and reran checks
- Worker stopped: yes
- Decision: **ACCEPTED for P01 foundation**
- Supersedes the changes-required decision in P01-review.md; retain that earlier record.

## Corrections verified

| Item | Code/evidence inspected | Result |
|---|---|---|
| R01 reload CSRF | Actual control script restores private-state CSRF and clears on 401; actual-script fixture rerun | Passed |
| R02 ordering | saveMu serializes SQL commit/cache/publication; stateMu covers installation and both bounded hub publications plus subscriber snapshots | Correct ordering invariant; production App tests passed |
| R03 shutdown | Listener/new-idle connection cleanup, active SSE cancellation, bounded shutdown; test keeps stream active, checks timeout warning, rebinds port and reopens DB | Full suite and five targeted repetitions passed |
| R04 redaction | Unexported cause, safe Error/Format, typed errors; sentinel tests for Get/Set/Delete and formatting/JSON/slog | Passed |

The production state tests cover competing expected revisions and sequential subscriber interleavings. The ordering conclusion also depends on source inspection of the shared save/state critical sections; the tests are not described as an exhaustive concurrency proof.

## Independently executed checks

Existing portable Go 1.27.3, CGO_ENABLED=0, GOTOOLCHAIN=local, GOPROXY=off; repository-local Go caches. Test commands ran outside the filesystem/network sandbox because loopback sockets are blocked inside it.

- go test ./... -count=1: all packages passed.
- go test ./internal/app -run TestRunServesAndShutsDownWithActiveSSE -count=5 -v: 5/5 passed, approximately 0.34s first and 0.04s each subsequent run.
- go vet ./...: passed.
- gofmt -l ./cmd ./internal ./web: clean.
- node testdata/browser/control_csrf_reload.js: bootstrapCSRF=true, reloadCSRF=true, reloadSaveStatus=200, lockedReload=true, failures=[].

## Evidence retained and limits

- Worker reports a successful real console CTRL_C_EVENT against its final binary, with clean exit and released port. Codex inspected the signal/lifecycle implementation and reran in-process shutdown; it did not independently repeat the removed console helper.
- Actual browser script tested in a Node VM; rendered-browser visual testing remains pending. This is recorded follow-up for P02's real control-flow check and P05, not a claim that a browser was tested.
- Race instrumentation remains skipped due to absent gcc; optional for P01, tracked for a configured release/CI runner.
- Twitch/OBS are deliberately unconnected at this milestone.
- No production fixes authored by Codex during this correction review; later branding/documentation changes must be recorded separately.

## Next action

Create the authorized accepted Git baseline; establish public desanv01/cuemender and its truthful foundation presentation. Release P02 with concrete integration seams and a live-registration prerequisite. The user confirmed no Twitch Public client app is registered yet.
