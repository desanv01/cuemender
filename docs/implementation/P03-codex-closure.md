# P03 Codex closure implementation and verification

Date: 2026-11-02. User authorized Codex to finish remaining P03 work after R06; DeepSeek remained stopped. This is substantive Codex implementation and **self-verification**, not independent review of Codex-authored code. The initial R06 suite was independently rerun before these edits and passed. Worker checkpoint: `2a9dcf9a74289e327f4d5567ee2cf634873c49c0`.

## Changes

- `internal/incidents/coordinator.go`: final canceled-job settlement now owns the existing operation gate through conditional durable transition, cache installation, job removal and publication. Timestamp comparison alone is not the concurrency guarantee. Pending retry preserves its original observed version rather than adopting a newer durable version. Failed manual interruption retains the original version. Close closes admission, releases the gate before joining cleanup, and uses `sync.Once` so concurrent Close callers all join. Admission rechecks cancellation after acquiring the gate; failed startup releases its context.
- `internal/incidents/p03_closure_test.go`: real coordinator/store tests for concurrent cleanup versus dismissal/recheck with a fixed clock and a publication barrier; stale pending retry against newer investigating state; concurrent Close joining blocked cleanup. Tests assert durable/cache agreement and joined completion.
- `internal/app/p03_protocol_test.go`: every upgraded-socket handler exit closes and joins its reader, including cancellation/failure paths.
- `web/control/index.html`: removed outdated P02-only wording; describes manual diagnostics and monitoring with repairs unavailable. No UI redesign or new behavior.
- `.gitignore`: ignores local Playwright snapshots/artifacts. No dependency or contract changes.

## Verification

Windows Go 1.27.3 from `.tools/go1.27.3/go`, Node 24.14.1. Go runs used workspace-local GOPATH, module cache, build cache and temporary directory, `GOTOOLCHAIN=local`, `GOPROXY=off`, `CGO_ENABLED=0`.

| Command/check | Outcome |
|---|---|
| Initial worker baseline `go test ./... -count=1 -timeout=120s` | Passed before Codex production edits |
| Final production code `go test ./... -count=1 -timeout=120s` | All packages passed; app 11.225s, incidents 26.139s |
| `go vet ./...`; `gofmt -l cmd internal web` | Passed; no formatting output |
| `go build -trimpath -o dist/cuemender.exe ./cmd/app` | Passed, CGo-free; rebuilt after final HTML copy adjustment |
| WSL Ubuntu, workspace Linux Go 1.27.3: `go test -race ./... -count=1 -timeout=180s` | All packages passed; app 22.489s, incidents 121.399s |
| `go test -race ./internal/incidents -run TestClosure -count=5 -timeout=90s` in WSL | Passed, 10.730s |
| Four Node actual-script fixtures | Passed: control_csrf_reload.js, p02_setup_render.js, p03_incidents.js, p03_review_evidence.js |

Focused repeat command: `go test ./internal/incidents ./internal/app -run 'TestClosure|TestP03ActualAppProtocolReportAndReply|TestP03ActualOBSLossDuringDiagnostic|TestP03BlockedDiagnosticWholeAppShutdown|TestReviewR05ProtocolFixture' -count=3 -timeout=90s`. Final outcome is recorded in the closure review. A restricted-sandbox repeat failed to dial local fixtures with socket access-denied errors; WSL also required execution outside that restriction. These environmental failures were not labeled passes or fixed by changing application behavior.

Final binary: `cuemender 0.3.0-p03` (Go 1.27.3; build metadata commit/time unset), 20,016,640 bytes. SHA256: `E47DF9095C384B1FDDD124C6245416EC0174B9C3A2B673D8C552A2E91B480623`.

## Actual rendered browser

Used the Playwright skill and temporary npm-cached `@playwright/cli` 0.1.23, headless Edge. Started only an owned hidden CueMender process on 127.0.0.1:57798 with isolated `.tools/p03-browser-closure`, `--no-browser --session-only-secrets`; no live credentials/providers. The one-time bootstrap was consumed without printing it.

- Authenticated control loaded; synthetic microphone and track saved. Reload retained values; a second save succeeded, exercising CSRF restoration.
- Offline manual audio check produced insufficient information and fresh-evidence-unavailable guidance. Dismissal rendered correctly.
- Starting monitoring without providers was refused (expected HTTP 503); monitoring remained stopped.
- Public overlay contained only public disconnected/waiting state, with no private microphone or incident detail.
- Inspected full-page screenshot; controls readable. No application JavaScript exception. Initial favicon 404 and expected rejected monitoring request are not product blockers.
- Local ignored screenshots: `output/playwright/p03-control.png`, `output/playwright/p03-overlay.png`. Browser snapshots under `.playwright-cli` are ignored.

Closed the named headless browser and stopped only the owned smoke app after checking its executable path. Removed transient stdout/stderr/bootstrap and PID/base files. Synthetic isolated test data remains ignored. This process stop is not claimed as a graceful Ctrl+C test; actual HTTP/SSE graceful shutdown is covered by the Go lifecycle tests. No user OBS settings changed.

## Limits and provenance

Actual local OAuth/Helix/EventSub/OBS protocol fixtures, including socket loss and persisted SQLite evidence, are **not live Twitch evidence**. No fresh live OBS test was performed; P02's separately recorded live OBS result remains historical evidence. P02-A/V06 and P03-D real Twitch remain deferred under D30/issue #6. P04 implementation and recording have not begun. No independent reviewer of the Codex closure changes is claimed.
