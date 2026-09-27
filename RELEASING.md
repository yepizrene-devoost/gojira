# Release GoJira

Releases are operator-approved promotions from `develop` to `main`. A `vX.Y.Z`
tag on a commit contained in `main` triggers GitHub Actions to build and publish
the release with GoReleaser.

## Quick path

1. Update `CHANGELOG.md`, replacing the target version placeholder with the
   release date and final user-visible changes.
2. From a clean `develop`, create the release branch:

   ```bash
   dflow start release vX.Y.Z
   ```

3. Make any release-only adjustments, then preview the dflow plan:

   ```bash
   dflow finish --dry-run
   ```

4. Run the local checks in [Dry run](#dry-run), commit the release preparation,
   and obtain explicit approval before pushing.
5. Push the release branch and open a pull request to `main`. `main` is a
   manual dflow target; do not use `dflow finish` to merge it.
6. After the pull request is approved and merged, verify the intended commit is
   on `main`. Obtain separate explicit approval before creating or pushing the
   tag:

   ```bash
   git tag vX.Y.Z <main-commit>
   git push origin vX.Y.Z
   ```

7. Confirm the GitHub Actions release job succeeds, then inspect the GitHub
   release, archives, version output, and checksum file.
8. Follow the dflow plan for the release branch's `develop` target separately.
   Direct merges and pushes still require explicit approval.

## Dry run

Run validation before requesting promotion:

```bash
go test ./...
go vet ./...
git diff --check
go build ./...
goreleaser check
goreleaser release --snapshot --clean
```

The snapshot command does not publish. Inspect `dist/` for six platform builds:
Linux, macOS, and Windows on `amd64` and `arm64`. Linux and macOS use `.tar.gz`;
Windows uses `.zip`. Every archive must contain the binary, `README.md`,
`LICENSE`, and `CHANGELOG.md`. Confirm `checksums.txt` is present.

Check the version embedded in a snapshot binary as well. Snapshot versions use
the GoReleaser next-patch `-next` identifier; tagged releases preserve the full
`vX.Y.Z` tag:

```bash
./dist/gojira_darwin_arm64/gojira version --json
```

Adjust the path for the host platform and the directory emitted by the local
GoReleaser version.

## Release checks

Before pushing a tag, confirm all of the following:

- The release pull request is merged and the target commit is contained in
  `main`.
- `CHANGELOG.md` has a dated entry for the version being tagged.
- The tag uses the exact `vX.Y.Z` form and points to the reviewed `main` commit.
- The local validation and snapshot build pass.
- A maintainer explicitly approved the tag push.

The release workflow runs only for `v*` tags. It checks that the tag resolves to
the workflow commit and that the commit is in `origin/main`; a tag on an
unmerged branch fails before publishing. The job grants only repository-content
write permission and passes GitHub's ephemeral `GITHUB_TOKEN` to GoReleaser.

After publishing, download one archive for each operating system, verify it
against `checksums.txt`, and run `gojira version --json`. Do not move or reuse a
published tag. If a release is wrong, stop and prepare a new patch release.
