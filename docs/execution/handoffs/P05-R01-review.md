# P05-R01 independent review

2026-11-02. Outcome: **changes required**. DeepSeek worker checkpoint `8275fca`; Codex authored review tests/documents only. P06 held. D34/#6/#10 live holds unchanged.

## Confirmed improvements

All eight previous top-level review regressions now pass in ordinary coverage. Legacy schema 1/3 rejection, serialized proposal invalidation, selected-track simulated writes, canceled admission, failed-save provider preservation, category selection in the App API and negative/terminal feedback regression cases are improved. Full ordinary Go suite and six Node fixtures pass independently. This does not establish the broader R01 acceptance requirements.

## Reproduced remaining defects

1. **Feedback falsely reports acceptance.** `TestReviewP05R01FeedbackRejectsUnacceptedVotes` fails both awaiting-approval and prior-session cases: nil error with zero votes. The adapter still uses nonempty human reply as success. Hidden Edge independently reproduced HTTP 202 for an interrupted old-session incident, while the UI said “Simulated vote recorded” and old/new incident counts both stayed 0/0. Report intake likewise still infers identity by scanning history instead of returning the accepted result from the authority; source finding, not a separate reproduced report race.
2. **Scenario publication is incoherent and generation remains constant.** `TestReviewP05R01ScenarioPublishesMatchingProvider` fails: selecting mic_muted publishes settings revision 1 with the previous unmuted/missing-track observation; the later provider mutation emits no corrected event. OBS generation stays 1. Source inspection shows separate ActiveJobs/RepairSnapshot/Stop/Invalidate/SaveSettings/setScenario operations, rather than one coordinator-owned transition. No deterministic blocked transition test was supplied.
3. **Reconciliation fabricates provenance.** `TestReviewP05R01ReconciliationRejectsForeignSource` fails: ReadTrack returns the caller's Foreign Mic/Other identity and generation/revision 999 with the fixture's map. It ignores cancellation and actual fixture provenance. Repair similarly echoes requested generation/revision; source inspection confirms these are not owned fixture values.
4. **Required UI paths remain broken.** Hidden Edge actual binary: wrong_scene → start → simulate report produces audio/no_supported_finding, because handler still hardcodes audio. With subsequent mic_muted history, Simulate !fixed still picks `state.incidents[0]`, producing the false success above. The closure select retains its chosen value and now has a label, but focus moves to BODY after state refresh: renderIncidents calls replaceChildren before reading activeElement. Stable keyed controls were not implemented. User section choice is only in memory, so reload persistence is not established.
5. **Required local acceptance evidence remains missing.** Full production OAuth/Helix/EventSub/OBS separate-viewer feedback→closure extension; deterministic scenario versus diagnosis/approval/reconciliation; full private HTTP accepted/rejected flow; complete keyboard/zoom/contrast/scenario browser matrix. These were already required by R01. No new feature scope is introduced by completing them.

Additional source follow-ups within R01: OpenMode discards metadata-query errors instead of distinguishing absent legacy metadata from read failures; simulated discovery lacks direct visual-item identity; overlay error callback updates text but does not unhide an idle-hidden panel. Verify/fix these within the existing requirements, with focused cases rather than broad redesign.

## Independent commands and evidence

- Portable Go 1.27.3, Windows, CGO_ENABLED=0, GOTOOLCHAIN=local, GOPROXY=off, existing workspace caches.
- `go test ./... -count=1 -timeout=300s`: exit 0, all packages. Log `.tools/p05-r01-review-suite.log`.
- `go vet ./...`: exit 0. `gofmt -l cmd internal web`: no output. `go build -trimpath -o .tools/p05-r01-review.exe ./cmd/app`: exit 0.
- All six `testdata/browser/*.js` Node fixtures: exit 0. Node is not rendered evidence.
- `go test -tags reviewrepro ./internal/app -run TestReviewP05R01 -count=1 -timeout=60s`: exit 1, all three new top-level review tests fail as above. Log `.tools/p05-r01-new-regressions.log`. First attempt had a review-only fixture compilation mistake (`Subscription.Close` instead of Cancel); corrected before behavioral execution. No production code changed.
- Worker binary SHA256 verified: `93AFACB1FD2F449BC0263D673382C67841CA3A6B1AB706F1C615A117FB7825CA`.
- Headless Edge via installed Playwright CLI, actual binary, isolated rehearsal directory/port. Setup default, wrong-scene report, scenario history/feedback and focus reproduced. Actual 390x844 viewport measured document scrollWidth=390; tested incident screen fits. Screenshot `output/playwright/p05-r01-wrong-scene-focus-mobile.png` inspected. This does not establish every mobile state or 200% zoom/contrast acceptance.
- First focus script used incorrect `#incidents` locator and timed out after focus action; rerun with actual `#incident-list` succeeded. A rapid scenario/start sequence initially hit 409 before the asynchronous transition finished; subsequent run waited for readiness and reproduced feedback failure. These are not claimed as separate defects.
- WSL race reported passing by worker; not independently rerun this cycle because deterministic acceptance failures already require corrections. No fresh live OBS, Twitch or approved OBS writes attempted.
- Owned rehearsal process and hidden browser stopped; temporary bootstrap output/auth storage removed. Existing footage/live data preserved. No new dependencies or system changes.

## Disposition

P05-R02 consolidates unfinished R01 work. Preserve all passing regressions and promote the three new tests after fixes. Issue #12 and draft PR #13 remain open; do not merge, close live issues or start P06 based on this review.
