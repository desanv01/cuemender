# D37 receive-only fallback — Codex implementation and self-verification

Date: 2026-11-02. Tracking: [#15](https://github.com/desanv01/cuemender/issues/15). Contract: [C12](../ANONYMOUS-CHAT.md). **Accepted for the documented read-only scope.** This is Codex's own implementation/self-verification, not independent review. DeepSeek remained stopped; no subagents implemented this packet.

## Implemented

- Bounded Go stdlib TLS IRC receiver with tags/commands capability acknowledgment and joined channel/room binding. Exact existing command parser; numeric identity, message ID and timestamp checks; shared-chat/self/foreign-room rejection; dedup/cooldown; heartbeat, retry, generation and canceled/joined dial/socket lifecycle. Only protocol housekeeping is sent; no PRIVMSG/reply API exists. The anonymous placeholder in PASS is public protocol convention, not a credential.
- Immutable `--receive-only-channel LOGIN`, live only, isolated `receive-only/LOGIN` data. Explicit private session/CSRF channel confirmation. EventSub startup is not invoked; no authenticated Twitch manager, reply queue, repair callback or reconciliation callback is attached. App approval rejects this composition. Authenticated EventSub/rehearsal remain separate.
- Truthful private connectivity versus authorization, public safe receive-only label, setup confirmation and manual-correction wording. No channel/sender identifiers added to the public projection. Version `0.5.1-receive-only`.
- P06 v1.1 updated for three supported paths, MIT owner choice, exact binary/module naming, safe locking/history clear, packaging/notices and honest clean-extraction/live limits.

## Actual external evidence

1. At approximately 11:05–11:12 UTC, the user's second account sent two harmless probe messages to the user-confirmed main channel. Each arrived via certificate-verified anonymous Twitch IRC with sender ID, room ID, message ID and server timestamp. The second was sent after closing/reopening the socket. Both connections acknowledged capabilities and joined successfully; probe exited zero and closed sockets.
2. The actual new receiver independently reached real channel-bound readiness, including ROOMSTATE binding. No developer app, Twitch token or OAuth ownership was used.
3. The actual App composition ran locally at a temporary loopback port. It connected to real OBS 32.1.1 (previously verified websocket 5.7.3), selected the previously user-approved Mic/Aux and declared track 1, and used real discovery/fresh diagnostics. The existing saved credential did not authenticate; the authorized local OBS WebSocket password was consumed directly into session-only memory without displaying it or modifying OBS settings.
4. In the hidden private browser, an unchecked confirmation prevented connection; explicit confirmation connected chat, then Start monitoring succeeded. At 11:20:46 UTC, the user sent `!issueaudio` from the separate viewer account. CueMender admitted one viewer report and persisted one incident. The fresh result was `no_supported_finding`: microphone unmuted and present on track 1. This matches observed OBS mute=false and all six track assignments=true. Independent read-only SQLite query confirmed origin=viewer, report_count=1, the two check explanation codes and repair_available=false. Private state had twitch_authorized=false, chat_connected=true, receive_only=true, zero proposals and zero actions.
5. Real UI disconnect/reconnect succeeded; OBS remained connected and streaming=false. Monitoring was stopped afterward; the incident became interrupted under existing stop semantics. Later shutdown released the listener and the SQLite database reopened successfully. No broadcast, unmute, track assignment, source reveal or other OBS write occurred.

This is genuine **offline-channel Twitch -> local OBS diagnostic** evidence, not a claim of broadcast audio recovery, live induced fault, automated repair, successful viewer feedback or EventSub verification. No new demo recording was made; P04 build footage is preserved. Issues [#6](https://github.com/desanv01/cuemender/issues/6) and [#10](https://github.com/desanv01/cuemender/issues/10) remain open for the original gates.

## Local verification

Environment: Windows version 10.0.26300.0; Go 1.27.3 windows/amd64; Node v24.14.1; existing Ubuntu WSL Go 1.27.3/compiler environment for race instrumentation. No installed dependency or go.mod/go.sum change.

Windows commands used workspace `.tools/go1.27.3/go/bin/go.exe`, GOROOT there, GOPATH `.tools/gopath`, GOMODCACHE `.tools/gopath/pkg/mod`, GOCACHE `.tools/gocache`, GOTMPDIR `.tools/gotmp`, GOTOOLCHAIN=local, GOPROXY=off and CGO_ENABLED=0.

- `gofmt -w` on changed Go files; final format check clean.
- `go test ./... -count=1 -timeout 180s`: passed all packages with loopback access enabled. Initial sandboxed run failed because local socket connections were forbidden; that failed attempt is not product evidence.
- `go vet ./...`: passed, including after final HTTP mode-rejection changes.
- `go test ./internal/twitch ./internal/app -run 'TestAnonymous|TestReceiveOnly' -count=3 -timeout 90s`: passed, including final silent-handshake cancellation and mode-route rejection. Three repetitions of the unchanged actual EventSub App protocol journey and rehearsal HTTP flow also passed.
- All six `node testdata/browser/*.js` fixtures passed. Existing assertions preserved; added receive-only confirmation/CSRF/route/label assertions and overlay label check. These are actual JS in Node, not rendered-browser evidence.
- WSL `CGO_ENABLED=1 go test -race ./... -count=1 -timeout 240s`: passed all packages, no race reports. Final focused `go test -race ./internal/twitch -run TestAnonymous -count=3 -timeout 90s` and `go test -race ./internal/app -run TestReceiveOnly -count=3 -timeout 90s` passed. One attempted combined WSL regex command was misquoted by the shell and failed before providing valid evidence; the corrected separate commands are the final results.
- `go build -o dist/cuemender.exe ./cmd/app`: passed with CGo disabled. Final development artifact: 20,442,112 bytes; SHA256 `1E4D9136E0CDCC9B406817A186A2DB5E6F2B3B82708E1AA064DD02F4C70EDD30`. `--version` prints `0.5.1-receive-only`; commit/build metadata remain unknown until P06 packaging.

New Go tests cover malformed/missing/duplicate metadata, stale/future timestamps, channel/self/shared-chat rejection, supported feedback parsing, dedup/cooldown, replacement generation, manual disconnect, closed-manager rejection, cancellation during dial and silent handshake. Actual App + local IRC/OBS protocol fixture traverses protected HTTP confirmation, real readiness, fresh diagnostics and SQLite persistence of simultaneous mute+track findings. It asserts no proposal/action and no bot acknowledgment, and every observed OBS request is in the read-only allowlist. Fixtures are not real Twitch/OBS evidence.

Rendered hidden IAB checks: confirmation requirement, connect, monitoring, actual result, disconnect/reconnect, stop, final capability wording after restart and public label. A screenshot and DOM measurement showed no horizontal overflow at the actual 1265px content width. The attempted 390px viewport override did not change the observed viewport, so **new mobile acceptance is not claimed**. Prior P05 layout evidence remains historical; native zoom and screen-reader limits remain. No foreground activation, browser/account settings change or OBS window manipulation occurred. Hidden verification tabs closed and viewport override reset.

## Handoff and limits

All owned probe/test app processes stopped; port 18789 released, DB reopened, ephemeral private-state/bootstrap helper files removed. Ignored local test data retain the incident; no secrets/raw chat/launch links included in this report or Git. The actual OBS password was never written to report/source/config. Source changes are limited to the anonymous receiver, App composition/CLI/domain/HTTP/UI integration, focused tests and execution/release documentation.

P06 may proceed only on explicit STATUS release, using the revised packet and PROMPTS. No full release ZIP or clean-machine acceptance yet. Feedback is parsed with real metadata but retains existing eligible-stage rules; no successful real feedback trial is claimed. TLS port 6697 must be available. IRC availability is observed now, not a permanent platform guarantee. Organizer eligibility remains their decision; a limitation disclosure does not waive rules.
