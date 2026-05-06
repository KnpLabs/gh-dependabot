# gh-dependabot

A [GitHub CLI](https://cli.github.com/) extension to interact with pull requests opened by Dependabot on GitHub.

## Requirements

- [GitHub CLI](https://cli.github.com/) (`gh`) version 2.0 or higher
- [Go](https://golang.org/) version 1.26 or higher (only required for building from source)

## Installation

```bash
gh extension install clementvtrd/gh-dependabot
```

## Usage

Once installed, the extension is available as a `gh` subcommand:

```bash
gh dependabot
```

Use the `--help` flag to see available commands and options:

```bash
gh dependabot --help
```

You can approve Dependabot's PR:

```bash
gh dependabot approve [PR's number...]
```

You can merge Dependabot's PR:

```bash
gh dependabot merge [PR's number...]
```

Wants more feature? Open an issue, I'll take a look as soon as I can!

## Building from source

1. Clone the repository:

   ```bash
   git clone https://github.com/clementvtrd/gh-dependabot.git
   cd gh-dependabot
   ```

2. Build the binary:

   ```bash
   go build -o gh-dependabot
   ```

3. Install it locally as a GitHub CLI extension:

   ```bash
   gh extension install .
   ```
