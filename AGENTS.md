# AGENTS.md

This file provides guidance to coding agents when working with code in this repository.

## What this is

A GitHub CLI extension (`gh dependabot`) written in Go to list, approve, merge and interactively review Dependabot pull requests on the repository in the current working directory.

## Commands

```bash
go build -o gh-dependabot .   # build
go vet ./...                  # static checks (no linter config in the repo)
gofmt -l .                    # formatting check
```

There is no test suite yet (`go test ./...` compiles but finds no tests).

To try a change, build then run the binary from inside a repo that has open Dependabot PRs: `/path/to/gh-dependabot/gh-dependabot [interactive|approve|merge]`. It relies on an authenticated `gh` and infers the repo from the cwd, so there is no need to `gh extension install .` between rebuilds.

Releases are cut by pushing a `v*` tag; `.github/workflows/release.yml` uses `cli/gh-extension-precompile` to build binaries with the Go version from `go.mod`.

## Architecture

Single package `cmd/` built on Cobra; `main.go` just calls `cmd.Execute()`. Each subcommand registers itself on `rootCmd` from its file's `init()`.

Key conventions:
- GitHub interaction goes two ways: reads use `go-gh` `api.DefaultGraphQLClient()` + `repository.Current()`, and mutations shell out with `gh.Exec("pr", ...)` (`pr review --approve`, `pr diff`, `pr merge`).
- A PR counts as Dependabot's if the author is `dependabot[bot]` / `app/dependabot` / `dependabot` **or** the head branch starts with `dependabot/`.
- GraphQL open PRs only. Check contexts mix `CheckRun.conclusion` and `StatusContext.state`; the state is normalised through `mapStateToConclusion`.
- Bubble Tea/Bubbles are **v2**, imported from `charm.land/...` (not `github.com/charmbracelet/...`): `View()` returns `tea.View` and keys arrive as `tea.KeyPressMsg`.
