# Publish a GoJira release locally

GoJira releases are prepared through dflow and published explicitly from a local
`main` checkout. CI validates release configuration but never publishes on a tag.
Every merge, tag, push, and publication remains a separate maintainer decision.

## Quick path

1. Prepare and review a `release/vX.Y.Z` branch from `develop`.
2. Promote that branch to `main` through a pull request.
3. On an up-to-date, clean `main`, create the local `vX.Y.Z` tag.
4. Extract and inspect the curated notes with `make release-notes`.
5. Push the tag only after separately confirming that external action.
6. Publish with `make release` only after a separate explicit approval.

## Prerequisites

Install Go, git-cliff, GoReleaser v2, and dflow. Add a fine-grained GitHub token
to the local `.env` file:

```dotenv
GITHUB_TOKEN=your-token
```

The token needs **Contents: write** permission for this repository. Never commit
`.env`, generated `RELEASE_NOTES.md`, or a real token.

## 1. Prepare the release branch

Start from a clean, current `develop` branch:

```bash
dflow start release vX.Y.Z
```

Curate the newest `CHANGELOG.md` section using this exact heading shape:

```markdown
## 📦 vX.Y.Z – Short release title
```

The section must be first among package sections and contain reviewed release
content. `make changelog` writes a heading-free, grouped history draft to
`CHANGELOG.draft.md`; use it as source material, not as a replacement for the
curated changelog. Update `HISTORY.md` when the release needs longer narrative
context.

Validate the preparation before review:

```bash
make changelog
make release-notes VERSION=vX.Y.Z
goreleaser check
go test ./...
git diff --check
```

Read `CHANGELOG.draft.md` for omitted or misclassified commits, then inspect
`RELEASE_NOTES.md`. The notes file must contain exactly the curated body beneath
the newest package heading. Both generated files are local artifacts.

Preview dflow's targets:

```bash
dflow finish --dry-run
```

`main` is a manual target. Push the release branch and open a pull request to
`main`; do not use `dflow finish` to merge that target. Handle the separate
`develop` target according to the reported dflow plan.

## 2. Tag the reviewed commit on main

After the pull request is merged, update local `main` and verify the exact commit.
The worktree must be clean and `HEAD` must be the reviewed release commit:

```bash
git branch --show-current
git status --short
git log -1 --oneline
git tag vX.Y.Z
```

Do not tag a release branch. The Makefile intentionally checks for an exact tag
but does not prove that the commit belongs to `main`; that operator check happens
here before publication.

Generate the notes again from the tagged tree and inspect them:

```bash
make release-notes VERSION=vX.Y.Z
```

Pushing the tag is an external action and requires its own approval:

```bash
git push origin vX.Y.Z
```

There is no tag-triggered publisher in GitHub Actions. After the tag is visible
on the expected GitHub commit, obtain separate approval for publication and run:

```bash
make release
```

`make release` loads `GITHUB_TOKEN` from `.env`, derives the version from the
exact tag, regenerates `RELEASE_NOTES.md`, and runs GoReleaser. This command is the
publishing action: it creates or replaces the GitHub release and its assets.

## 3. Verify the published release

- Confirm the GitHub release title, curated notes, and target commit.
- Confirm archives exist for Linux, macOS, and Windows on amd64 and arm64.
- Verify downloaded archives against `checksums.txt`.
- Run `gojira version --json` from an archive and confirm the release version.

## Safe reruns and tag corrections

GoReleaser uses replacement mode, so rerunning `make release` for the same tag
replaces the release and uploaded assets. Use that only when the tag still points
to the same reviewed commit and the curated notes are correct.

If the local tag is wrong **and neither the tag nor release was published**, fix
it locally before retrying:

```bash
git tag -d vX.Y.Z
git tag vX.Y.Z <reviewed-main-commit>
make release-notes VERSION=vX.Y.Z
make release
```

If the tag or GitHub release is already public, do not move or force-push it.
Stop, document the problem, and prepare a new patch release. Never delete or
retarget a public release tag as part of a rerun.
