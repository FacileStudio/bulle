# bulle — Development

Local setup, the quality gate, and how releases are cut.

## Prerequisites

- **Go 1.26.4** (`go.mod`) — Bubble Tea v2 declares that floor, and this repository is
  nothing but that floor.
- **mise**, for the task runner. Optional — every task is a one-line shell command runnable
  directly.
- **golangci-lint v2**, optional locally. CI pins `v2.12.2`. The local gate skips the lint
  pass and says so when the binary is missing or fails to run.

## Tasks

```sh
mise run check      # gofmt + build + vet + test -race + golangci-lint
mise run test       # go test ./...
mise run format     # rewrite Go sources in place
```

## Git hooks

Run `mise install` once per clone. It installs the pinned toolchain and then runs
`lefthook install` through its `postinstall` hook. `pre-push` calls `scripts/check.sh`
directly, so a bad push is caught before it leaves your machine.

## The quality gate

`scripts/check.sh` reports and never rewrites, except under `--format`. One module since
this repository left the SDK tree, so there is no workspace to reconcile and no nested-module
machinery — gofmt, build, vet, `test -race`, lint, in that order, every stage running even
after an earlier one fails.

## CI

Two workflows under `.github/workflows/`:

- **`ci.yml`** — one job on Go 1.26.4 with `GOTOOLCHAIN=local`: gofmt, build, vet,
  `test -race`, `golangci-lint-action@v7` at `v2.12.2`.
- **`filet.yml`** — runs `filet check`, `fail-on: info`.

## Versioning

Semver tags with a leading `v`, one tag per release, following the suite's versioning
convention: while on `v0`, a breaking change bumps the **minor**. Every change is recorded in
[CHANGELOG.md](../CHANGELOG.md) in Keep a Changelog format, with the reason it exists. Add an
`Unreleased` entry as part of the change, not after.

The version lives in exactly one place: the git tag. goreleaser stamps it into `main.version`
at build time, and a source build reports `dev`. Do not introduce a second copy of the number
— a literal that tooling does not read is a copy that drifts.

### Cutting a release

Check first with the suite-wide flow:

```sh
mycelium flow run release-preflight
```

Then it is one commit, one tag:

```sh
sh scripts/check.sh
git commit && git push && git tag vX.Y.Z && git push origin vX.Y.Z
```

The tag push triggers goreleaser, which publishes the release assets and updates
`Casks/bulle.rb` in [FacileStudio/homebrew-tap](https://github.com/FacileStudio/homebrew-tap)
using the `HOMEBREW_TAP_GITHUB_TOKEN` secret. That token must be a fine-grained PAT scoped to
only the tap repository with Contents read/write; a missing or under-scoped token fails the
formula push after the binaries are already published, which leaves the release half-done —
check the run before assuming a green upload means a shipped release.
