# Git and GitHub execution policy

User authorization recorded 2026-11-01.

## Repository and product

- Public product name: CueMender.
- Repository name: cuemender.
- Visibility: public.
- Verified owner: desanv01; public repository https://github.com/desanv01/cuemender.
- Existing workspace location remains C:\Users\Dv\Desktop\Twitch Hackathon.
- Module github.com/desanv01/cuemender and executable cuemender.exe use the product name. LiveCheck data/keyring namespace, livecheck.db and livecheck_session remain stable; no incidental migration.
- README, About and repository presentation must follow the user's ErrataScope reference, adapted to CueMender. See [REPOSITORY_PRESENTATION.md](REPOSITORY_PRESENTATION.md) for the inspected patterns and publication checkpoints.

## Authorized responsibility

Codex owns local Git and GitHub actions: repository creation/connection, meaningful commits, issue creation, branches, PRs, review/merge, issue closure and publishing accepted source history. The user has requested this workflow; do not repeatedly ask whether routine commits, PRs or merges are allowed.

DeepSeek remains a code/test worker and must not mutate repository state. No force pushes, destructive resets, unrelated repository edits or secret publication are implied by this authorization.

## Sequence

1. After P01 is accepted, inspect ignored/staged content for secrets, local data and tools, initialize local Git and record the accepted foundation checkpoint.
2. Verify the account and create/connect public cuemender. If a repository already exists, inspect and preserve it; never replace unrelated history.
3. Push the accepted baseline. Before a public push, verify the actual staged/pushed contents; do not blindly add ignored tools, runtime DBs, recordings or credentials.
4. Track meaningful bugs/features with issues. Use focused branches and commits, open PRs with behavior/validation evidence, verify, merge accepted changes and close associated issues.
5. If several corrections belong to one acceptance packet, use a cohesive PR with distinct meaningful commits where useful. No issue or PR is needed for each keystroke.
6. The initial bootstrap may use a direct baseline commit because no existing default branch exists. Subsequent changes normally use the issue/branch/PR/merge flow.
7. Record actual commit/issue/PR identifiers in status/review evidence. Do not fabricate public links before creation.
8. Before submission, inspect source access, setup instructions, release artifact and judge access.

## Current state

2026-11-02 D34 update: PR #11 merged as conditional P04 development baseline at 6c09f11178f8bed8230d000cfd2652b74ea68265. Keep #6 and #10 open for deferred real verification. P05 uses feature/p05-user-experience and issue #12; Codex checkpoints dispatch docs before worker editing. No full live acceptance or release artifact follows from this source merge.

P01-R01 baseline a07b3df3c33f0284c1da450a323fbba3e9ed1771 is published. P02 source checkpoint 1b068843e38bee905ff6ea80c44f612e88ed32d4 is integrated through PR #5 under D30's conditional development acceptance. Close completed implementation/correction issues #2/#4 with explicit links to the deferred live Twitch gate in issue #6; do not close #6 before actual verification. P03 is tracked in issue #7 on feature/p03-diagnostics, prepared by Codex before dispatch. Verify actual remote state for final PR/merge hashes rather than treating this planning note as merge evidence. Source integration does not imply a release artifact or full live acceptance.

This authorization covers Git/GitHub work. It does not itself authorize unrelated email, YouTube publication or Devpost submission.
