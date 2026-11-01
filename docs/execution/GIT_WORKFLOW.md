# Git and GitHub execution policy

User authorization recorded 2026-11-01.

## Repository and product

- Public product name: CueMender.
- Repository name: cuemender.
- Visibility: public.
- Intended owner: user's GitHub account; previously supplied profile is desanv01. Verify the authenticated account/target before creating or connecting a repository.
- Existing workspace location remains C:\Users\Dv\Desktop\Twitch Hackathon.
- Existing livecheck module/executable/data names are temporary implementation identifiers. Codex coordinates a consistent rename without overwriting existing runtime data.
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

P01 has changes required. No accepted baseline or remote repository has been created by Codex yet. Prepare/correct/review P01 first. GitHub actions will follow the accepted checkpoint sequence above.

This authorization covers Git/GitHub work. It does not itself authorize unrelated email, YouTube publication or Devpost submission.
