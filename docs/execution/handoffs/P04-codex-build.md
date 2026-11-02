# P04 Codex implementation record

Status: local implementation complete, 2026-11-02; required live approval gate remains unmet. Starting baseline: P03 merge `01a6e3d68cd919b5e90e80bc96e5d61e8c0204e4`. Branch: feature/p04-approved-repair. User explicitly requested Codex takeover, actual screen recording first, and GPT-6.1 Sol high subagents. DeepSeek remains stopped. See [verification](P04-review.md) and STATUS. No full P04 acceptance or P05 release is claimed.

## Actual recording checkpoint

Used the computer-use skill to inspect existing windows without activation, then the existing FFmpeg installation as the background recorder. No OBS configuration, audio track, streaming state or system setting was changed. FFmpeg window GDI capture produced black frames and was rejected. Initial Windows Graphics Capture received no frames until the user restored the chat. The stopped failed tests are not build evidence.

After the user replied ready, Windows Graphics Capture produced readable actual Codex main-chat frames: H.264, 1920x1034, 15 fps, 7.734 seconds, silent. Inspected extracted frame visually. Test file: `.tools/p04-recording/test-wgc-ready.mkv`; frame: `.tools/p04-recording/test-wgc-ready.png`. Audio is deliberately absent; narration can be added later. Recordings are ignored and never uploaded automatically.

Actual build recording: `.tools/p04-recording/p04-build-01.mkv`, started before production edits, using `gfxcapture` against the observed Codex window handle. Maximum duration one hour, no microphone/desktop capture. Keep this chat restored/unminimized. Capture correctness is checked from saved frames, not inferred from process success. Stop and verify completed footage before claiming the final artifact. Raw footage includes actual tool/agent activity; do not reconstruct or label unrecorded work as original footage.

## Allocation and verified preflight

- Root: planner, recording operation, contract decisions, integration verification and Git/GitHub.
- GPT-6.1 Sol high p04_obs_audit: read-only SDK audit, then sole writer for the narrow OBS adapter and protocol tests.
- GPT-6.1 Sol high p04_contract_audit: read-only app/coordinator/store design during the first writer's work; implementation only after explicit transfer.
- SDK audit: existing nonnil map serializes explicit false values. OBS requires numeric string keys 1 through 6; domain keys track1 through track6 must be converted. No SDK patch is pre-authorized or indicated.
- Preserve active-monitoring repair gate. While Twitch is deferred, fixture implementation may proceed, but full live product approval may remain unavailable. Do not bypass readiness or mislabel the real gate.

Implementation changes, tests, contribution provenance, recording stop/verification and useful clip timestamps will be appended as completed. Codex-authored code receives self-verification and separately described peer inspection; neither is a claim of independent external acceptance.

## OBS adapter checkpoint — completed

Commit `44295ec`: Codex Sol6.1 high OBS agent authored internal/domain/track_repair.go, internal/obs/repair.go, internal/obs/repair_test.go and the narrow obs.go extension. Unified RepairTrack owns preflight/callback/write/readback; ReadTrackState is strict read-only recovery. The backend discovery interface remains read-only; the internal writer converts six domain keys to numeric wire keys and keeps false assignments. No SDK patch/dependency change.

The separate test-review agent identified readiness remaining stale when a repair closes its SDK transport. Writer corrected this through the existing transport-loss lifecycle and added caller-cancel plus natural response-timeout regressions. Peer source inspection is separately identified; tests run by the implementation agent are self-verification.

Executed with Go1.27.3 windows/amd64: `go test ./internal/obs ./internal/domain` passed (OBS10.830s), scoped `go vet` passed, changed-file `gofmt -l` empty. Initial restricted loopback tests failed with access denied; the same suite passed with authorized execution outside that socket restriction. Actual local pinned SDK websocket evidence covers all 192 possible single-track additions, six flag preservation, malformed/null/alias map rejection, no-op, callback rejection, mismatched readback, old generation, lost response, natural timeout, original-source read and operation admission. No live OBS calls or application approval acceptance claimed.

OBS writer stopped; root checkpointed this substep and transferred sole production ownership to the contract/integration agent. Full integrated suite/race/browser verification remains outstanding. Root is recording and inspecting/documenting; DeepSeek stays stopped.

## Integrated implementation and corrections — completed

The contract/integration agent then became the only production writer. Added pure proposal binding/validation (`internal/actions`), typed private proposal/action data, schema 3 and transactional repair records, coordinator reservation/dispatch/settlement/reconciliation, the strict private approval route, settings authority integration and minimal approval UI. Root resumed sole writer ownership after that agent stopped.

The proposal binds source name/UUID/context, all six assignments, selected confirmed track, settings revision, OBS and Twitch generations, active monitoring session and 60-second expiry. Concurrent/repeated approvals share a durable action; the adapter freshly checks identity/map, calls the durable admission boundary, writes at most once, reads back and never unmutes. Exact desired map is a no-op. An uncertain outcome remains auditable and can only reconcile by reading the saved source; restart invalidates approvals and never replays an intent.

Separate Sol high peer source inspections found and helped resolve: stale readiness after repair transport loss; immutable source comparison; expiry crossing the durable-intent callback; empty reconciliation releasing another action's ownership; retention/cache disagreement; and outcome-persistence failure followed by Stop/reload losing recovery ownership. Root's final production fixes preserve unknown outcomes across reload and distinguish temporary discovery-refresh gaps from real context invalidation. Both approval and dispatch still require resolved, current evidence; no readiness bypass was added.

Root extended the outcome-persistence test with Stop/reload, added discovery-refresh/context-change coverage, and used a test-only local protocol app harness for rendered-browser verification. The harness has synthetic OAuth/Helix/EventSub/OBS endpoints and no production authentication or readiness override. Keepalive metadata and a one-second P04 fixture poll cadence corrected fixture-induced disconnect/admission instability. Production protocol deadlines were not relaxed.

Race instrumentation found a test callback changed after a diagnostic worker started and an older OBS assertion racing a legitimate new-revision poll. The callback now comes from immutable constructor options; the old test owns poll admission while testing invalidation, preserving its assertions. Both race regressions subsequently passed ten repetitions. Final whole-suite evidence is in the review record.

### Changed areas

- `internal/actions/actions.go` and tests: complete map/source/binding validation and expiry.
- `internal/domain/actions.go`, `track_repair.go`, `projections.go`: typed private proposal/action and adapter contracts; public allowlist remains separate.
- `internal/store/actions.go`, `store.go`, `incidents.go`: migration 3, unique proposal action, atomic reservation/intent/outcome and retained audit/recovery.
- `internal/incidents/actions.go`, `coordinator.go`, `p04_test.go`: operation authority, settings invalidation, bounded action ownership, read-only recovery and fault/concurrency tests.
- `internal/obs/repair.go`, `repair_test.go`, `obs.go`: single narrow writer and original-source recovery; `p02_correction_test.go` only gains deterministic poll exclusion.
- `internal/app/app.go`, `diagnostics.go`, `integrations.go`, `version.go`: coherent snapshots, readiness context, common settings authority and version 0.4.0-p04.
- `internal/app/p04_protocol_test.go`, `p03_protocol_test.go`: full local protocol/private-HTTP fixture, valid keepalives, SQLite/SSE/public redaction evidence and optional headless-browser harness.
- `internal/httpui/server.go`, `web/control/app.js`, `index.html`, `testdata/browser/p04_approval.js`: strict private approval and minimal existing-card controls. No design overhaul, dependency or framework addition.
- `internal/diagnostics/diagnostics.go`: supported-action guidance updated; unchanged diagnostic scope. Existing store version assertions now expect schema 3.

## Recording stop and verification

Stopped the owned FFmpeg recorder gracefully with `q`. Final raw file: `.tools/p04-recording/p04-build-01.mkv` (22,373,355 bytes), H.264 1920x1034, 15 fps, **2249.867 seconds** (37:29.867), silent. `ffprobe` read its completed metadata. Full `ffmpeg -hide_banner -loglevel error -i <file> -f null NUL` decode exited 0 with no errors. Visually inspected actual saved frames at 01:00, 15:00 and 36:40; the main chat and genuine tool/agent progress are readable.

Useful raw-footage locations: 01:00 opening implementation work; 15:00 OBS checkpoint and approval/persistence work; 36:40 final integration debugging and recovery explanation. These are verified frame anchors, not a completed edit or a claim that the full test output is expanded on screen. Final browser/race completion happened after recording stopped. Do not represent those later checks as captured in this file. Select short excerpts and crop unrelated sidebar content during final video preparation. No recording was uploaded, no audio captured and no final submission video created. Genuine Codex session ID still needs to be collected for submission; no ID is invented here.

## Reproduction and environment

From the workspace, use Go 1.27.3 and `CGO_ENABLED=0`. `go test ./internal/app -run TestP04ActualAppPrivateApprovalProtocolFlow -count=1` exercises actual app/HTTP/SQLite and local protocol servers. It is not a live service test. To repeat the rendered check, set `CUEMENDER_P04_BROWSER_DIR` to a new ignored directory, run that test, load its synthetic storage state in a hidden browser, open its metadata baseURL plus `/control/`, approve before expiry, verify the result/reload, then create the directory's `stop` file. The harness requires exactly one verified action and one wire write; it exits after a bounded wait. Never publish storage-state files.

Go tooling used workspace-local `.tools/go1.27.3/go`, caches under `.tools`, `GOTOOLCHAIN=local`, `GOPROXY=off`; Linux race used the existing WSL Ubuntu/compiler and `.tools/linux-go1.27.3/go`. Node 24.14.1 ran actual-script fixtures; installed headless Edge via cached Playwright CLI ran the rendered checks. No system installation, machine PATH change, backend language, dependency pin or real OBS configuration changed. Browser and fixture processes were closed; recorder stopped. Exact command/results and local/live distinctions follow in P04-review.md.
