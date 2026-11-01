# Reference sources and verification policy

Revision: 1.0

Use official documentation for API behavior and library documentation for the version actually pinned. A link is not proof that a local installation supports every method.

| Subject | Primary reference |
|---|---|
| Hackathon requirements | https://twitchcon-codex.devpost.com/rules |
| Hackathon updates | https://twitchcon-codex.devpost.com/updates |
| Twitch app registration | https://dev.twitch.tv/docs/authentication/register-app/ |
| Device grant / token acquisition | https://dev.twitch.tv/docs/authentication/getting-tokens-oauth/ |
| Token validation | https://dev.twitch.tv/docs/authentication/validate-tokens/ |
| Token refresh | https://dev.twitch.tv/docs/authentication/refresh-tokens/ |
| Chat authentication | https://dev.twitch.tv/docs/chat/authenticating/ |
| EventSub WebSockets | https://dev.twitch.tv/docs/eventsub/handling-websocket-events/ |
| EventSub subscription types | https://dev.twitch.tv/docs/eventsub/eventsub-subscription-types/ |
| Twitch API reference | https://dev.twitch.tv/docs/api/reference/ |
| OBS protocol | https://github.com/obsproject/obs-websocket/blob/master/docs/generated/protocol.md |
| Go OBS adapter | https://github.com/andreykaipov/goobs |
| WebSocket transport | https://github.com/coder/websocket |
| CGo-free SQLite | https://pkg.go.dev/modernc.org/sqlite |
| Credential storage | https://github.com/zalando/go-keyring |
| Go toolchain | https://go.dev/doc/install |
| Embedded assets | https://pkg.go.dev/embed |

Rules, device grant and EventSub pages were rechecked during document preparation on 2026-11-01. Dependency versions and host compatibility remain unverified.

P02 must check SDK signatures, required OBS requests and actual auth scope requirements against these sources. Do not copy an old blog example as authority when it conflicts with current protocol docs.

The rules require a Codex-built project, a build-session ID, description of Codex use, and footage showing that use. They permit standard development tools including AI coding assistants; that is not a blanket assurance that every proposed division of work meets judging expectations. Maintain accurate evidence of both tools' contributions; do not invent Codex authorship.
