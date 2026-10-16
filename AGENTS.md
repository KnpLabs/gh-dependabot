# AGENTS.md

This file provides guidance to coding agents when working with code in this repository.

## What this is

A GitHub CLI extension (`gh dependabot`) written in Go to list, approve, merge and interactively review Dependabot pull requests on the repository in the current working directory.

## Commands

Prefer the `Makefile` targets over calling `go` directly (run `make` to list them):

```bash
make build   # build the gh-dependabot binary
make fmt     # format the code (gofmt -w)
make lint    # gofmt check + go vet (no linter config in the repo)
make test    # go test -race -cover ./...
make check   # lint + test + build, run before committing
make tidy    # go mod tidy
make clean   # remove the built binary
```

Tests are table-driven, stdlib `testing` only, next to the code (`cmd/*_test.go`, `internal/dependabot/*_test.go`), with fixtures in the sibling `testdata/` directories. Commands are tested end to end through `newFakeGitHub` (`cmd/harness_test.go`), which swaps the `newGraphQLClient` and `ghExec` package variables from `root.go`:
- GraphQL responses are served from `cmd/testdata/graphql/*.json`, keyed by operation name; the requests sent are snapshotted in `cmd/testdata/golden/*.graphql`.
- `gh` invocations are recorded and asserted with `assertExecs`.
- After an intended query change, regenerate the snapshots with `go test ./cmd -update` and review the diff.

Always go through `ghExec` / `newGraphQLClient`, never `gh.Exec` / `api.DefaultGraphQLClient` directly, or the call escapes the fake. The TUI's `interactive` flow itself isn't covered.

To try a change, `make build` then run the binary from inside a repo that has open Dependabot PRs: `/path/to/gh-dependabot/gh-dependabot [interactive|approve|merge]`. It relies on an authenticated `gh` and infers the repo from the cwd, so there is no need to `gh extension install .` between rebuilds.

Releases are cut by pushing a `v*` tag; `.github/workflows/release.yml` uses `cli/gh-extension-precompile` to build binaries with the Go version from `go.mod`.

## Architecture

`cmd/` holds the Cobra wiring, GitHub I/O and the TUI; `main.go` just calls `cmd.Execute()`. Each subcommand registers itself on `rootCmd` from its file's `init()`.

`internal/dependabot/` is the pure domain (no I/O): `PullRequest`, `CheckState`, `Mergeability`, eligibility rules and `MergeMethod` parsing/validation. Business rules live there and must never depend on display strings; `checksStatus` / `mergeableStatus` in `cmd/list.go` only render them.

Key conventions:
- GitHub interaction goes two ways: reads use `go-gh` `api.DefaultGraphQLClient()` + `repository.Current()`, and mutations shell out with `gh.Exec("pr", ...)` (`pr review --approve`, `pr diff`, `pr merge`).
- PRs are fetched by a single query, `fetchDependabotPullRequests` (`cmd/pullrequests.go`), shared by every command.
- A PR counts as Dependabot's if the author is `dependabot[bot]` / `app/dependabot` / `dependabot` **or** the head branch starts with `dependabot/`.
- GraphQL open PRs only. Check contexts mix `CheckRun.conclusion` and `StatusContext.state`; the state is normalised through `mapStateToConclusion`.
- Bubble Tea/Bubbles are **v2**, imported from `charm.land/...` (not `github.com/charmbracelet/...`): `View()` returns `tea.View` and keys arrive as `tea.KeyPressMsg`.

## Code style

Don't add comments to the code (doc comments included) unless the user explicitly asks for them. Rely on clear names instead. Leave existing comments alone.
