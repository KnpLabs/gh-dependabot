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
make test    # go test ./...
make check   # lint + test + build, run before committing
make tidy    # go mod tidy
make clean   # remove the built binary
```

There is no test suite yet (`make test` compiles but finds no tests).

To try a change, `make build` then run the binary from inside a repo that has open Dependabot PRs: `/path/to/gh-dependabot/gh-dependabot [interactive|approve|merge]`. It relies on an authenticated `gh` and infers the repo from the cwd, so there is no need to `gh extension install .` between rebuilds.

Releases are cut by pushing a `v*` tag; `.github/workflows/release.yml` uses `cli/gh-extension-precompile` to build binaries with the Go version from `go.mod`.

## Architecture

Single package `cmd/` built on Cobra; `main.go` just calls `cmd.Execute()`. Each subcommand registers itself on `rootCmd` from its file's `init()`.

Key conventions:
- GitHub interaction goes two ways: reads use `go-gh` `api.DefaultGraphQLClient()` + `repository.Current()`, and mutations shell out with `gh.Exec("pr", ...)` (`pr review --approve`, `pr diff`, `pr merge`).
- A PR counts as Dependabot's if the author is `dependabot[bot]` / `app/dependabot` / `dependabot` **or** the head branch starts with `dependabot/`.
- GraphQL open PRs only. Check contexts mix `CheckRun.conclusion` and `StatusContext.state`; the state is normalised through `mapStateToConclusion`.
- Bubble Tea/Bubbles are **v2**, imported from `charm.land/...` (not `github.com/charmbracelet/...`): `View()` returns `tea.View` and keys arrive as `tea.KeyPressMsg`.
