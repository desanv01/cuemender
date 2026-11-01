# P01 acceptance review

- Packet: P01 v1.1
- Review: 2026-11-01, approximately 06:37 UTC / 14:37 MYT
- Reviewer: Codex, current main chat
- Worker report: P01-worker-report.md; environment note: ../../implementation/P01-environment.md
- Baseline: no Git repository; actual application files inspected directly
- Worker confirmed stopped: yes
- Code authorship: DeepSeek; Codex made no production-code changes in this review
- Verification type: separate review of worker implementation
- **Decision: changes_required**
- Update: P01-R01 has since been accepted; see [the correction acceptance record](P01-R01-review.md). This document preserves the original findings.
- Correction: [P01-R01](../corrections/P01-R01.md)
- P02 remains held. No accepted Git checkpoint created.

## Actual evidence

Inspected the entry point, configuration, application lifecycle/state, production SSE hub, HTTP routing/middleware/session/JSON boundary, SQL migrations/settings, secrets, browser scripts/markup and relevant tests. Root planning SHA-256 hashes still match BASELINE.

Used the existing portable Go executable .tools/go1.27.3/go/bin/go.exe, with repository-local GOPATH/GOMODCACHE/GOCACHE/GOTMPDIR as documented in README. CGO_ENABLED=0; GOTOOLCHAIN=local and GOPROXY=off for the unsandboxed reproduction. No downloads or installations were needed.

| Check | Observed result |
|---|---|
| go version | go1.27.3 windows/amd64 |
| go test ./... -count=1 inside sandbox | Loopback connects denied by sandbox; not evidence of a product networking defect |
| Same full suite outside sandbox | httpui, secrets and store passed; app failed TestRunServesAndShutsDownWithActiveSSE after approximately 5.39 seconds |
| go test ./internal/app -run TestRunServesAndShutsDownWithActiveSSE -count=3 -v outside sandbox | One failure after approximately 5.35 seconds, then two passes around 0.02 seconds: intermittent failure reproduced |
| go vet ./... | Exit 0 |
| gofmt -l ./cmd ./internal ./web | No unformatted files reported |
| go build -trimpath -o .\dist\livecheck-review.exe .\cmd\app | Produced reviewer artifact; Go build metadata confirms windows/amd64 and CGO_ENABLED=0 |
| Actual web/control/app.js evaluated with a small DOM/fetch fixture using Node | Reload-with-existing-session path sends empty X-CSRF-Token; save displays missing-CSRF error |
| Browser visual rendering | Not exercised in this review; JS fixture is not a real browser test |
| Compiled binary Ctrl+C | Still unverified; worker's failed/inconclusive probe is not accepted as clean shutdown evidence |
| OS keyring real round trip | Worker evidence retained; reviewer did not repeat credential-store writes |
| Race instrumentation | Not rerun; worker recorded missing gcc. Optional P01 check remains skipped |

The full-suite run was retried outside the sandbox after permission was granted because loopback connections were blocked, not to bypass a product failure. The outside-sandbox shutdown failure is a separate result.

## Findings requiring correction

### R01 — Control-page reload loses CSRF token

web/control/app.js:133 loads private state but never copies privateState.csrf_token into state.csrf. Only bootstrapFromFragment sets it at line 162. After the browser clears the fragment and the page is reloaded, its cookie remains valid but state.csrf resets to an empty string. Saving then sends an empty token at line 213 and fails.

This violates C03's explicit authenticated-reload behavior. Reproduced using the actual script with a fake authenticated /api/v1/state response, not merely inferred from a string search.

### R02 — Settings commit, cached state and SSE publication can reorder

internal/app/app.go:272–299 performs the SQL save before acquiring stateMu, then releases stateMu before publishing. An older successful writer can be paused after SQL commit, a newer writer can commit/install its result, then the older writer can overwrite the cached settings. Separately, even when cached state updates in order, an older snapshot can publish after a newer snapshot or after a subscriber receives a newer initial state.

The hub's separate subscription-copy/send phases do not impose revision order. The result can be a cached/public-private event sequence inconsistent with persisted revision or the monotonic state contract. This finding is based on the concrete permitted interleavings in production code; no timing-stress run is being represented as proof.

Existing HTTP tests use a separate testBackend/testHub implementation, so their passing update tests do not verify production App/hub ordering.

### R03 — Shutdown verification fails intermittently and does not exercise its stated case

The existing test failed in the unrestricted full suite and one of three targeted repetitions. app_test.go:142 cancels the SSE request before canceling the app, so it also does not reliably demonstrate server shutdown with an active client. Its five-second test timeout equals the production five-second Shutdown allowance, leaving no scheduling/cleanup margin.

The exact cause of the observed delay has not been established; do not label this a proven deadlock. Correct the test scenario/budget and investigate production behavior as necessary. Keep binary Ctrl+C verification explicit.

### R04 — Credential error text forwards raw backend errors

internal/secrets/secrets.go:36 formats e.Err verbatim, while Get/Set/Delete attach backend errors. Therefore the boundary does not enforce the documented guarantee that backend failures cannot expose credential contents. The current test supplies "set exploded" while checking for an unrelated secret string, so it cannot detect this behavior.

No actual credential leak was observed. This is a code-inspection finding about the required redaction boundary before P02 begins using real tokens/passwords. Use a synthetic backend error containing a sentinel secret in a regression test.

## Acceptance coverage

| Criteria | Review result |
|---|---|
| P01-A | Build/toolchain confirmed; dependency provenance/license details remain worker evidence |
| P01-B / V01 / V05 | Embedded source/build present; shutdown acceptance incomplete |
| P01-C / V02 | Store reopen/revision tests pass; production commit/cache/publication ordering needs R02 |
| P01-D / V03 | HTTP mutation boundary tests pass; real reload workflow broken by R01 |
| P01-E | Explicit public projection and HTTP leak checks are sound for P01 |
| P01-F / V04 | Missing/unavailable/memory behavior tested; raw error redaction requires R04 |
| P01-G | HTTP fixtures pass, but production ordering and active-SSE shutdown need corrections |
| P01-H | Required suite currently fails; no blanket pass |
| P01-I | Original documents preserved and worker report is candid about limitations |

## Nonblocking contract clarification

state_revision may reset when the process restarts. It is monotonic within a running instance; reconnect gets a full current snapshot, not a persisted event journal. settings.revision remains persistent. This interpretation is recorded in CONTRACTS/DECISIONS; no new durable event store is required.

## Next action

DeepSeek implements only consolidated P01-R01, fixes and verifies the four issues, appends exact results to its existing report, and stops. Codex reviews the resulting changes before releasing P02. No production fix or independent browser success has been claimed by Codex.

## JS reproduction used

From the workspace root, the following PowerShell command evaluates the actual browser asset with an existing-session fixture. Node is a review tool only, not a product runtime dependency:

~~~powershell
@'
const fs = require('node:fs');
const vm = require('node:vm');
const nodes = new Map();
const node = id => {
  if (!nodes.has(id)) nodes.set(id, {
    value:'', hidden:false, disabled:false, checked:false, textContent:'',
    addEventListener(type, fn) { this[type] = fn; }
  });
  return nodes.get(id);
};
let postedCSRF;
const snapshot = {
  mode:'live', state_revision:2, csrf_token:'REVIEW_DUMMY_CSRF',
  settings:{revision:1,microphone:{name:'Review mic'},stream_track:1,visual:null}
};
const sandbox = {
  document:{getElementById:node},
  location:{hash:'',pathname:'/control/',search:''},
  history:{replaceState(){}}, URLSearchParams,
  EventSource:class { addEventListener(){} },
  fetch:async(path,opts)=>{
    if (path === '/api/v1/settings') {
      postedCSRF=opts.headers['X-CSRF-Token'];
      const ok=postedCSRF===snapshot.csrf_token;
      return {ok,status:ok?200:403,json:async()=>ok
        ? {data:{...snapshot.settings,revision:2}}
        : {error:{message:'Request is missing a valid CSRF token.'}}};
    }
    return {ok:true,json:async()=>({data:snapshot})};
  }
};
vm.runInNewContext(fs.readFileSync('web/control/app.js','utf8'),sandbox);
setImmediate(async()=>{
  await node('settings-form').submit({preventDefault(){}});
  console.log(JSON.stringify({
    csrfRestored:postedCSRF===snapshot.csrf_token,
    sentEmptyCSRF:postedCSRF==='',
    notice:node('notice').textContent
  }));
});
'@ | node
~~~

Before correction: csrfRestored=false, sentEmptyCSRF=true, missing-CSRF notice.
