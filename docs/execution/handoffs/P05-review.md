# P05 v1.1 acceptance review

2026-11-02 Malaysia. **Changes required; P06 held.** DeepSeek stopped; Codex inspected worker checkpoint `3ebf6be` and added tests/docs only. No Codex production fixes. Live Twitch/real OBS approval remain deferred under D34/#6/#10. The retained live runbook is [LIVE-VERIFICATION.md](../LIVE-VERIFICATION.md); it is not permission to resume live work.

## Independently reproduced

- Windows Go 1.27.3, CGO_ENABLED=0, local cached modules: `go test ./... -count=1 -timeout=300s` passed, exit 0 (app 12.866s, incidents 26.744s, OBS 12.608s). `go vet ./...` passed; `gofmt -l cmd internal web` empty.
- All six actual-script Node fixtures passed independently. Worker binary hash matched `19F611CC65BF8A88C26AFF601F46F0DC219C0C479CE6F1F447A9D1D70393A6BB`. Worker-reported full WSL race remains worker evidence; not independently rerun because concrete acceptance failures already require corrections.
- Independent CGo-free `go build -trimpath -o .tools/p05-review.exe ./cmd/app` passed, version 0.5.0-p05/go1.27.3; SHA256 `6B6A80B2CE9D2C7DA8C06F705543A2F5DBBAADECEAD75A30AE14E12717C077D3`. Built after source checkpoint, so embedded Go VCS metadata differs from the worker's uncommitted build; no byte-identical claim.
- Added eight failing top-level `TestReviewP05*` regressions in internal/app/p05_review_test.go and internal/store/p05_review_test.go, under reviewrepro. Command `go test -tags reviewrepro ./internal/app ./internal/store -run TestReviewP05 -count=1 -timeout=60s` exits 1. Preserve assertions and promote after correction.
- Applied Playwright skill; existing cached CLI/headless Edge against isolated actual rehearsal binary on loopback. No live network/provider setup. At actual viewport 390x844, idle Monitor measured viewport/scrollWidth 390, no right-edge element overflow; screenshot inspected. This narrow state passes, unlike the worker's uncertain capture. Other states and 200% zoom are not thereby accepted.
- Real UI: select wrong_scene, Apply, Start monitoring, Simulate viewer report => audio/no_supported_finding and 'No supported conflict' instead of the visual limitation. Then select track_missing, Apply/Start/report/approve => locally verified. Simulate !fixed targets the older history entry, shows 'vote recorded', but current feedback stays 0/0. Both closure selects have zero associated labels. Screenshots: ignored output/playwright/p05-review-mobile.png, p05-review-wrong-scene.png, p05-review-approved.png.
- Private UI source rebuilds every incident card on every state render, losing select/focus state. Initial unconfigured tab is Monitor, not Setup; compact persistent provider/readiness summary, advanced disclosures and closure-outcome rendering from the brief are incomplete. Overlay ignores local_verification for awaiting_feedback, omits applying state, and hides disconnected/no-incident status. These are source findings, not claims that every path was browser-tested.

## Reproduced defects

1. Legacy schema 1 is accepted and relabeled rehearsal because OpenMode detects only an incidents table. Schema 3 is rejected only after migration to 4. Both violate pre-migration ownership/preservation.
2. CloseIncident updates proposals.status but not proposal_json in its transaction; LoadRepairs returns proposed after closure. A second coordinator invalidation transaction masks this on a happy path, but cannot make the first transaction atomic.
3. Rehearsal Repair uses caller ExpectedTracks as alleged preflight, always mutates track2, ignores cancellation, and returns verified without Observed/readback. Tests reproduce selected-track3 unchanged, changed-map admission and canceled write.
4. Failed scenario persistence already mutates provider state. Scenario changes also lack the specified guard against owned diagnostics/actions/recovery; exact interleaving test remains required.
5. RehearsalReport returns snapshot list[0], which can be another category/history entry. Test proves a video report returns audio.
6. stillbroken is passed unnormalized while coordinator expects still_broken, producing a positive vote. Human rejection strings are treated as success; closed feedback returns nil error. Test proves both. UI's first-history feedback target compounds this.

See [P05-R01](../corrections/P05-R01.md) for one consolidated correction packet. Existing passing suite is not acceptance of the missing scenarios/failure boundaries. No dependency or architectural expansion is requested.

## Evidence limits and cleanup

No complete real or local-production-protocol separate-viewer feedback/closure flow, all-scenario rendered completion, measured contrast/full keyboard/zoom acceptance or new live OBS evidence. No new live account/chat/OBS actions or recording. Owned headless browser closed; isolated review binary stopped by its recorded PID/command line; synthetic auth state and stdout capability file removed. Local screenshot/test artifacts remain ignored. The review does not label fixtures/rehearsal as live evidence.
