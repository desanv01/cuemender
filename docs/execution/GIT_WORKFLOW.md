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

P01-R01 is accepted; baseline a07b3df3c33f0284c1da450a323fbba3e9ed1771 is published. Presentation/P02 preparation is tracked in issue #1 through chore/cuemender-presentation-p02. P02 implementation is tracked in issue #2 on feature/p02-live-connections; Codex prepares that branch before dispatch. Subsequent accepted work uses the PR lifecycle. Verify actual remote state for final PR/merge hashes rather than treating this planning note as merge evidence.

This authorization covers Git/GitHub work. It does not itself authorize unrelated email, YouTube publication or Devpost submission.
