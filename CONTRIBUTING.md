# Contributing to omasword

## Workflow

1. Branch from `main` and open a pull request; `main` only changes through
   merged PRs.
2. Run `gofmt -w`, `go vet ./...` and `go test ./...` before pushing.
3. Stage files explicitly (`git add <paths>`), not `git add -A`, so local
   files such as `.claude/settings.local.json` never end up in a commit.

## Commit messages

Use [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` new feature
- `fix:` bug fix
- `refactor:` restructuring without behavior change
- `docs:`, `chore:`, `test:`, `perf:` as usual

## Versioning

omasword follows [Semantic Versioning](https://semver.org/). The version
lives in `internal/version/version.go`; bump it in the PR that prepares a
release, then tag `vX.Y.Z` on `main`. Pushing the tag runs
`.github/workflows/release.yml`, which tests, builds Linux amd64/arm64
binaries and publishes a GitHub release.
