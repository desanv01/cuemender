# CueMender foundation publication

Date: 2026-11-01. Author and verifier: Codex, current main chat.
These changes are Codex-authored and self-verified, separately from the independent P01-R01 review of DeepSeek's code.

## Accepted source checkpoint

- Accepted P01 / P01-R01 source and evidence: a07b3df3c33f0284c1da450a323fbba3e9ed1771.
- Public repository: https://github.com/desanv01/cuemender.
- Presentation/dispatch tracking: https://github.com/desanv01/cuemender/issues/1.
- P02 implementation tracking: https://github.com/desanv01/cuemender/issues/2.
- Initial source was checked for accidental toolchains, databases, logs, recordings and credential patterns before staging/publication. Git identity was configured only in this repository.

## Changes

1. Changed module/imports to github.com/desanv01/cuemender, CLI/version branding and visible embedded page copy to CueMender. No production algorithm changed.
2. Kept LiveCheck data/keyring namespace, livecheck.db and livecheck_session for compatibility. Did not move or delete existing runtime data.
3. Adapted the user's ErrataScope README layout: centered title/tagline, truthful badges/navigation, feature grid, demo instructions, stack, scope and documentation. Kept build/test details in docs/DEVELOPMENT.md. No invented screenshots, CI, release or license claims.
4. Refined P02 v1.1 / CONTRACTS v1.2 against actual app/domain/httpui interfaces; documented unregistered Public-client prerequisite, fixture/live separation and single-worker dispatch. No P02 integration code written.
5. Retained the P04 Codex recording checkpoint in STATUS and the P04 prompt. No footage recorded in this work.

## Executed self-verification after rename

Portable Go 1.27.3, repository-local caches, CGO_ENABLED=0, GOTOOLCHAIN=local, GOPROXY=off. Loopback-dependent checks ran with the sandbox exception approved by the execution tool.

- go test ./... -count=1: passed all packages.
- go vet ./...: passed.
- gofmt -l cmd internal web: no output, clean.
- go build -trimpath -o dist/cuemender.exe ./cmd/app: passed.
- dist/cuemender.exe --version: cuemender 0.1.0-p01; Go 1.27.3. Commit/build date remain unknown in this development build; no release artifact claimed.
- node testdata/browser/control_csrf_reload.js: bootstrapCSRF=true, reloadCSRF=true, reloadSaveStatus=200, lockedReload=true, failures=[].
- git diff --check: clean before publication.

Independent acceptance earlier in this turn also reran the lifecycle test five times. It is recorded in P01-R01-review.md; those five repetitions preceded the naming-only changes.

## Limits and next owner

No rendered-browser visual check, new real-console Ctrl+C run, race-instrumented test or live Twitch/OBS test is claimed here. Existing accepted limitations remain explicit. P02 development is released to one DeepSeek worker; its real integration gates remain pending. Codex retains Git/GitHub and acceptance ownership; substantive P04 implementation remains planned.
