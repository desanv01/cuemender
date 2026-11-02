# Decisions and assumptions

Revision: 1.4

## D30 — Conditional P02 baseline and P03 release (2026-11-01)

User explicitly approved moving to P03 while live Twitch verification waits for their phone/2FA resolution. Codex accepts the completed P02 engineering/local/live-OBS work as the development baseline and releases P03 v1.2 under contracts v1.3. This is conditional progression, not full live Twitch acceptance; it supersedes D11/D26 and older P03 hold wording only for this transition.

- P02-A/V06 remains deferred/unmet. P03-D's real Twitch report-to-OBS-finding portion is also deferred, not replaced by fixtures. Track both in [issue #6](https://github.com/desanv01/cuemender/issues/6).
- Do not attempt Twitch login, registration or authorization until the user lifts the hold. Do not ask for credentials or invent a client ID. Local protocol fixtures, manual OBS checks and incident implementation proceed independently.
- Keep the live gates required before final release/submission readiness and before claiming the real Twitch end-to-end flow works. Codex can review P03's local implementation while explicitly carrying these gates; the worker cannot waive them or release P04.
- Merge the verified P02 development baseline through PR #5, retaining issue #6; dispatch P03 on feature/p03-diagnostics, tracked in [issue #7](https://github.com/desanv01/cuemender/issues/7). This is source integration, not a production release.
- P04 remains Codex-owned, held until a separate review/release decision, with genuine implementation recording required. DeepSeek is the sole P03 writer only after claiming STATUS.
- No application code changed for this decision. P03-specific API/intake details are clarified in C09; existing safety, persistence, provenance and OBS read-only constraints remain.

| ID | Decision | Rationale / status |
|---|---|---|
| D01 | Go only backend; mixed backend deferred | Explicit user choice |
| D02 | Windows 11 x64 local executable, browser UI | Accepted implementation direction; exact host version unverified |
| D03 | Vanilla JS/HTML/CSS, embedded assets, SSE | Small interface; no separate Node runtime |
| D04 | SQLite modernc driver and OS keyring | Local persistence without a server; explicit session-only fallback |
| D05 | goobs and focused Twitch adapter with coder/websocket | Selected adapters; exact versions require compile verification |
| D06 | Three checks; one track-add repair | Fixed MVP; mute/visibility remain manual |
| D07 | No required runtime AI or mixed worker services | Preserve a dependable baseline |
| D08 | Codex specifies, implements P04, verifies and handles integration fixes; DeepSeek implements/tests P01/P02/P03/P05/P06 | Revised shared-folder workflow; supersedes planning/review-only restriction |
| D09 | One writer, six packages, explicit handoff | Avoid shared-file conflicts and excessive relays |
| D10 | CueMender public name; public cuemender repository | User finalized name/visibility; Go module github.com/desanv01/cuemender, executable cuemender.exe; legacy storage identifiers retained |
| D11 | Later packet release follows previous acceptance | Instructions may be refined using real integration findings |
| D12 | Direct scene-item visibility only in MVP | Bounded implementation interpretation; nested groups need future work |
| D13 | Device grant with Twitch public client | Official flow for a distributed desktop client; verify real registration in P02 |
| D14 | No inferred truth/approval from chat volume | Community reports are evidence, not control authority |
| D15 | No guaranteed exclusive access to OBS | Other clients can race writes; detect disagreement, never claim atomic compare-and-set |
| D16 | No Git initialization during docs preparation | Existing folder preserved; Codex can establish checkpoint after implementation review |
| D17 | Reserve the approval/repair engine for Codex | Concrete product contribution rather than relying solely on planning or incidental bug fixes; no eligibility guarantee |
| D18 | Label self-verification accurately | Codex-authored work is not independently reviewed merely because Codex later checks it |
| D19 | Track actual contributions and footage, not quotas | No invented minimum percentage, generated session IDs, backdated commits or cosmetic code changes to imply authorship |
| D20 | state_revision is process-local; settings.revision persists | Clarifies SSE reconnect semantics without requiring a durable event journal |
| D21 | Codex authorized for Git/GitHub lifecycle | Public repo, meaningful commits, issues, PRs, merges and closure; accepted baseline follows P01 acceptance; see GIT_WORKFLOW |

## Change record procedure

### P02 closure decisions — 2026-11-01

- D26 (Codex, user-authorized takeover): Codex completes remaining P02 implementation and verification with GPT-6.1 Sol high subagents. DeepSeek remains stopped. One implementation writer at a time; other agents may inspect independently. P03 stays held. Codex-authored fixes receive self-verification and separately identified peer review, not a claim of independent DeepSeek acceptance.
- D27 (Codex): permit a minimal local patched copy of pinned goobs v1.11.1 under third_party/goobs, retaining its license and provenance with a local go.mod replacement. Inspection found constructor/authentication, raw EOF and request-wait ownership defects that public adapter hooks cannot fully repair while meeting prompt cancellation and joined cleanup. Limit upstream production edits to client.go and api/client.go and record the exact patch; no SDK substitution or version upgrade. Reverify real-protocol cancel/failure/shutdown and race behavior. Storage and supported OBS write scope do not change.
- D28 (user constraint): all computer access stays in the background, including OBS. Do not activate windows or take mouse/keyboard control. Guide the user through necessary foreground authentication/OBS settings steps.
- D29 (Codex): use existing WSL Ubuntu/GCC with an official checksum-verified workspace-local Linux Go toolchain for race validation; production Windows artifacts remain CGO_ENABLED=0. No administrator/system-wide installation.

New entry: date, author/approver, affected contracts/packets, decision, reason, migration or retest effect. Workers propose changes in their reports rather than editing this file.

Reasonable local function naming, CSS choices within a released design brief and small test helpers are not architectural decisions. Dependency substitution, state/API semantics, platform scope and automatic side effects are.
## 2026-11-01 — P02 release and publication decisions

- D22 (Codex): preserve LiveCheck data/keyring namespace, livecheck.db and livecheck_session for compatibility. Visible branding and module/imports use CueMender. No storage migration. Rename gets full build/test self-verification.
- D23 (Codex): C08 and P02 v1.1 define connect/disconnect bodies, atomic config ownership and asynchronous status against actual P01 interfaces. P02 may develop with fixtures while Public client registration is pending, but real Twitch/OBS acceptance gates remain required.
- D24 (Codex): P02 diagnostic receipt counts can be shown privately while idle as a setup aid. They never create incidents or claim monitoring; C04 monitoring-gated report processing starts in P03.
- D25 (Codex, 2026-11-01): approve explicit private OBS Connect after process restart for the MVP. A default endpoint alone is not intent to connect. Keep exact-endpoint saved-password reuse and established-loss automatic recovery; no new startup flag/config field. This resolves the worker's reported choice under C08's optional startup resume, without waiving cancellation/readiness gates.

## D31 — P03 Codex takeover and local acceptance (2026-11-02)

User authorized Codex to complete remaining P03 work after R06 with DeepSeek stopped. Codex finished settlement/cache/retry/Close corrections and missing concurrency/browser verification as sole writer. Own changes are self-verified, not independently reviewed. Accept the local P03 baseline and integrate via PR #9; close local P03 issues #7/#8 after merge. D30/#6 live Twitch gates remain deferred and required before final readiness. P04 remains held for separate explicit release and the actual implementation recording checkpoint. No contract, product scope or dependency change.

## D32 — P04 release, recording and agent allocation (2026-11-02)

User authorized P04 implementation and GPT-6.1 Sol high subagents after first setting up and testing actual main-chat recording. Capture test passed after restoring Codex: FFmpeg gfxcapture, 1920x1034, 15 fps, silent; actual build capture started before code edits. GDI black tests are rejected evidence. Root coordinates one production writer at a time and records genuine contributions. This supersedes the older no-subagent wording for this packet only. DeepSeek stays stopped, P05 held.

P04 v1.3 is released on feature/p04-approved-repair. D30/#6 Twitch hold remains. Active-monitoring repair authorization is not waived; dependent full-product live OBS approval acceptance stays pending if Twitch readiness prevents it. No new scope, runtime framework, cloud component or generic mutation API.

## D33 — P04 contract integration choices (2026-11-02)

C10/contracts v1.4 resolves approval JSON, proposal-only expiry/invalidation, exact-desired-map no-op precedence, shared dispatch authority and action-safe restart against current P03 source. This avoids making a valid diagnostic incident terminal merely because its approval expired. Current-session monitoring is mandatory; no manual/offline repair bypass. Final dispatch admission follows durable intent and is the serialization point against stop/settings changes; subsequent uncertainty remains auditable and never replayed. Six numeric wire assignments preserve all other flags. No generic mutation API or SDK replacement.

## D34 — P05 release with live acceptance deferred (2026-11-02)

The user explicitly requested moving to P05 and keeping live Twitch and the real OBS approval trial pending until their phone/2FA issue is resolved. Conditionally accept P04's locally verified implementation as a development baseline and integrate PR #11; this is source integration, not full live P04 acceptance. Keep issue #6 (P02-A/V06 and P03-D Twitch) and issue #10 (P04-A/V11 real approved OBS correction) open. The dependent real end-to-end portion of P05-A/V14 and final V18/V19 remain unmet. P05 may implement and verify with isolated local protocol fixtures and clearly labeled rehearsal. Production monitoring/approval authorization is unchanged. No login, registration, credentials, real chat or real OBS mutation while the hold remains.

Release P05 v1.1 to one DeepSeek worker after its packet, C11 and visual brief are prepared. Codex retains contracts, review and Git/GitHub; no P05 subagents or parallel writers are authorized by the earlier P04-only agent instruction. P06 stays held until explicit review/release.

Codex inspected the actual P04 rendered screen and applied the available 12ui skill. Automatic approval review initially rejected screenshot export; the user then explicitly authorized it. The authorized retry and generic brief attempt both returned sponsored-budget-exhausted; no generated design or selected candidate exists. Release a written layout/interaction brief preserving the current visual language, and record this tool limitation. No CSS/application implementation was performed in this dispatch. Rendered visual acceptance still occurs at P05 review; do not claim a generated design checkpoint passed.

## D35 — P05 contract resolution (2026-11-02)

C11/contracts v1.5 defines immutable constructor mode, rehearsal child data directory/DB ownership, memory-only rehearsal secrets with zero OS-keyring calls, isolated in-process adapters, fixed private scenario/intake routes and transactional broadcaster closure. Isolation is structural, not a UI boolean or readiness override. P04 remains live-only by default; trusted rehearsal may share rules only with matching provenance. Existing feedback is reused. Migration 4 is authorized for closure/mode ownership with backward preservation tests. Written visual brief scopes functional reorganization in the current language, not a new framework or generated-design claim.
