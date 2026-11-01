# Architecture

Revision: 1.0

## Selected stack

| Area | Choice |
|---|---|
| Runtime | One Go executable in the logged-in Windows user session |
| HTTP, CLI, HTTP clients, logging | net/http, flag, context, log/slog and standard library |
| Browser | Embedded HTML/CSS/vanilla JS; SSE snapshots plus POST actions |
| SQLite | database/sql + modernc.org/sqlite |
| Secret storage | github.com/zalando/go-keyring; explicit session-only mode |
| OBS | github.com/andreykaipov/goobs |
| Twitch | Focused OAuth/Helix/EventSub adapter; github.com/coder/websocket |
| Distribution | Windows x64 ZIP, assets compiled in, no Node installation |
| Verification | Go testing/httptest, fixtures, configured race run, real Twitch/OBS smoke |

Pin versions only after checking supported Go versions and compiling. P01 introduces SQLite and keyring; P02 introduces the OBS and WebSocket dependencies. Keep go.mod/go.sum exact and record compatibility. Do not add unused libraries merely to fill a stack table.

## Data flow

~~~mermaid
flowchart LR
 T[Twitch adapter] --> Q[Bounded events]
 O[OBS adapter] --> Q
 Q --> C[Incident coordinator]
 C --> D[Pure diagnostics]
 D --> C
 C --> S[(SQLite)]
 UI[Private UI] --> A[Approval validation]
 A --> C
 C --> X[Action executor]
 X --> O
 O --> C
 C --> P[Public projection]
 C --> V[Private projection]
 P --> OV[Overlay SSE]
 V --> UI
~~~

This is one process with small internal packages, not independently deployed services.

## Package boundaries

- cmd/app: flags, main, exit codes.
- internal/app: dependency wiring, lifecycle, monitoring state, shutdown.
- internal/domain: only shared types actually needed; no dependency on network/UI/SQLite libraries.
- internal/store: migrations, repositories, bounded history; hides SQL.
- internal/secrets: narrow Get/Set/Delete abstraction with OS and explicit memory implementations.
- internal/httpui: routing, session/CSRF, JSON validation, public/private projections, SSE transport.
- web: embed.go beside static files in web/control and web/overlay. Go embed cannot reach a parent directory; do not put an invalid ../../web directive inside internal/httpui.
- internal/twitch: OAuth tokens, EventSub lifecycle, command normalization, bounded outgoing replies.
- internal/obs: connection lifecycle, discovery, observations and the single supported write primitive.
- internal/diagnostics: pure rules against expectations and observations.
- internal/incidents: single logical owner of incident state, grouping and transitions.
- internal/actions: proposal/precondition verification, durable intent, write/read-back/reconciliation.
- internal/rehearsal: isolated fixtures implementing the required adapter seams.

Create directories when used. Do not generate empty framework packages or placeholder endpoints for later phases.

## State and concurrency

One coordinator owns incident mutations. It must not wait on network calls while holding a lock or processing the sole event loop. Bounded jobs do I/O and return results carrying request ID, monitoring session ID, configuration revision and appropriate connection generation. Obsolete results are ignored and observable as stale, never applied to new state.

Action serialization is stricter than report processing: at most one mutation in flight for this OBS instance. Duplicate approvals return the existing action status. Durable intent is committed before any OBS write. A failed database write prevents the external action.

Queues are bounded. Coalesce telemetry/latest UI snapshots; don't silently drop an approval or action result. Return a busy response when accepting more work would exceed limits. Keep slow SSE clients from blocking the coordinator.

Production clocks use UTC and monotonic durations where appropriate. Inject a small clock seam for expiry/cooldown tests; do not build a generic scheduling framework.

## Storage ownership

Bootstrap JSON owns startup preferences: loopback listen address, non-secret Twitch client ID, local OBS endpoint, preference flags. SQLite owns versioned expected sources/track and product records. The same setting must not have two competing persisted authorities.

Default data directory is the platform user-config directory joined with LiveCheck. Allow --data-dir for isolation/tests. Rehearsal uses a separate data directory and credential namespace. Never place real data or tokens inside the repository.

P01 needs schema_migrations and app_settings only. P03 adds incidents, reports, findings and delivery metadata. P04 adds durable action records. Migrations are numbered, transactional, forward-only and tested against the prior accepted schema.

## Local access boundary

Bind only explicit loopback addresses. Public pages contain only static UI and sanitized public state. Private APIs require an in-memory session established from a one-time launch capability. Use Host/Origin validation, CSRF for mutations, HttpOnly/SameSite cookies, no permissive CORS, safe DOM text rendering, and bounded request bodies.

The HTTP contract specifies the bootstrap and SSE behavior. Loopback HTTP is an intentional MVP deployment choice, not protection against malware or another privileged process on the same computer.

## Integration seam policy

Prefer small consumer-owned interfaces and typed results over a universal event bus. Keep vendor SDK types inside adapters. P01 does not implement imaginary versions of goobs interfaces; P02 records the actual pinned SDK methods and protocol fields. Contract changes come to Codex with evidence.

One adapter instance graph per run: live or rehearsal. Never switch a live writer into rehearsal by changing only a boolean in the UI.
