# CueMender lifecycle patch

This directory contains the source of `github.com/andreykaipov/goobs` v1.11.1,
retaining its original LICENSE and generated request/type surface.

- Upstream: https://github.com/andreykaipov/goobs
- Tag commit: `d7b017046d9a50c3a4ee16680ac9396134ae8bd2`
- Go module zip checksum: `h1:73qAE5wK7Dx2G7oT6z2s0rXcAZUy7cDS3VFeaRaSI/I=`
- Original zip SHA256: `e64a03499839ed668aad4f13b811188fb36d568d75520723b2d77cd206ff69f6`

Only `client.go` and `api/client.go` contain production modifications. Local
regression coverage lives in `cuemender_lifecycle_test.go`. The parent module
selects this source through a local `replace`, retaining the v1.11.1 requirement.

The patch adds a lifecycle context and worker join, cancels dialing and OBS
authentication, interrupts pending requests on disconnect, and closes/joins on
constructor failure. Raw EOF invalidates the client. Opcode and response sends
select disconnection; shared producer channels are never closed underneath
senders. Authentication completion is buffered and Identify is written by the
existing opcode worker, eliminating its former detached self-send goroutine.
Request deadlines cover both opcode admission and response waiting. A timeout
or mismatched response invalidates the single-flight pipeline and closes its
raw socket, preventing late responses from blocking subsequent requests.
Generated requests, response fields, UUID mapping, authentication algorithm,
subscriptions, dependency versions and external OBS semantics are unchanged.

The upstream live-OBS tests are preserved and still require their original
external prerequisites. Run only the offline local patch tests with
`go test . -run TestCueMender -count=1` in this directory. Parent adapter/app
fixtures provide additional local protocol and shutdown coverage; these are
not evidence of a live OBS integration.
