# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Aide is a Go CLI (`aide`) that reads `aide.yaml`, checks whether required CLI tools (and an AI provider like `claude`, `copilot`, `codex`, `opencode`) are installed with correct versions, installs whatever is missing via native package managers, and launches the provider. Think "`package.json` for your AI dev environment."

## Commands

```bash
make build              # go build -ldflags "..." -o aide[.exe] .
make build VERSION=1.0.0 RECIPES_URL=https://example.com/recipes.yaml
make build-all           # cross-compile to dist/ (windows/amd64, darwin/amd64+arm64, linux/amd64)

make test                # go test ./internal/...
make test-race           # go test -race ./internal/...  (preferred before committing)
go test -race -run TestName ./internal/package/...   # single test

make lint                # go vet ./internal/...
make fmt                 # go fmt ./internal/... ./cmd/...
```

Tests live under `internal/<package>/*_test.go` — there are no tests under `cmd/`. `internal/installer` has an `integration_test.go` in addition to unit tests; check it when touching install flows.

## Architecture

```
cmd/            CLI surface (cobra commands): root (check+launch), init, add, install, start, account, cache
internal/
  config/       aide.yaml parsing — config.FindAndParse walks up from cwd to find the file
  checker/      verifies provider/tool binaries exist in PATH (or .aide/shims/ if isolated) and satisfy semver constraints
  installer/    tool -> package-manager resolution via embedded recipes.yaml; executes installs; handles isolated mode (.aide/store/ + shims)
  launcher/     execs the provider binary (inherits stdio); applies account env vars and isolated PATH before exec
  account/      provider account credentials, stored at ~/.aide/accounts.json (mode 0600)
  userconfig/   parses ~/.aide/config.yaml (user-owned account bindings), never written by aide
  project/      named project registry at ~/.aide/projects.json, used by `aide start`
  display/      formats check/install results for terminal output
  semver/       extracts a version from `--version`/`-v`/`version` subcommand output, checks constraints (Masterminds/semver)
```

Data flow for the default (no-subcommand) run, in `cmd/root.go`: find `aide.yaml` upward from cwd → build a `checker.Checker` (isolated or not, based on `cfg.IsIsolated()`) → `CheckProvider()` + `CheckTools()` → print via `display` → if all OK, build a `launcher.Launcher` and exec the provider, applying isolated PATH env if needed.

### Recipes (`internal/installer/recipes.yaml`)

- Embedded into the binary via `//go:embed`. Priority when resolving recipes at runtime: **remote URL** (`--recipes-url` flag / `AIDE_RECIPES_URL` env, cached 1h) → **external file** → **embedded**.
- Format: top-level key is the tool name; `windows`/`macos`/`linux` each hold an ordered list of `{pm: package}` entries, tried in order until one PM is available on the system. Optional `arch_map` (GOARCH → tool-specific arch string) and `requires` (prerequisites to install first, e.g. `graphify` requires `pipx`).
- Supported package managers: `winget`, `scoop`, `choco`, `brew`, `apt`, `dnf`, `curl` (piped to `bash`/`sh`), `github` (downloads a release asset, extracts to `~/.local/bin`), `pip`, `pipx`.
- When adding a new tool: edit `recipes.yaml`, update `docs/tools-catalog.md` to match, and run `go test -race ./internal/installer/...`.

### Isolated mode

When `aide.yaml` sets `mode: isolated`, tools with `github`/`pipx` recipes install into `.aide/store/<tool>/<version>/bin/` with shims created in `.aide/shims/`; that directory (not the system PATH) is prepended before launching the provider. System-PM recipes (`winget`/`brew`/`apt`/etc.) can't be isolated and fall back to a global install with a warning. `.aide/` is auto-added to `.gitignore`.

### Accounts

`aide.yaml` can reference a named account (`account: <name>`) resolved from `~/.aide/accounts.json`. All four providers (`claude`, `codex`, `copilot`, `opencode`) support credential-profile isolation via `internal/account.Adapters`: a profile directory under `~/.aide/accounts/<name>/` gets bound to the launched process via env vars (`CLAUDE_CONFIG_DIR`, `CODEX_HOME`, `GH_CONFIG_DIR`/`COPILOT_HOME`, `XDG_DATA_HOME`). Legacy per-provider field overrides (`--api-key` for claude, `--codex-home` for codex) still work for accounts created before profiles existed. Copilot's old legacy path (`gh auth switch --user <username>`) has been removed — an account with `User` set and no on-disk profile now fails the check/launch with a message to re-add without `--user`.

**Which account name is actually used** is resolved once, in `cmd.resolveAccountName` (called from `runCheck` and `runInstall` right after `projectDir` is known, then written back onto `cfg.Account` so everything downstream — `Checker.CheckAccount()`, `launchProvider`, `aide account status` — sees the resolved value transparently): `--account` flag > `AIDE_ACCOUNT` env > `~/.aide/config.yaml` path bindings (`internal/userconfig`, longest-prefix match against `projectDir` and `cfg.Provider`; supports a per-provider `accounts: {provider: name}` map alongside a bare `account:`) > `aide.yaml`'s own `account:` field. This ordering matters: `aide.yaml` is committed/cloned, so it must never get the deciding vote over the machine's actual owner — `~/.aide/config.yaml` is never written by `aide` and is the one layer meant to be hand-edited.

**Credential brokers**: `account.Account.Command`, set via `aide account add --command`, is an AWS `credential_process`/git `credential.helper`-shaped escape hatch — its stdout (trimmed) is used instead of a stored `Token`/`APIKey`, resolved fresh on every launch via `account.ResolveToken`/`ResolveAPIKey` (consumed by the copilot adapter's `COPILOT_GITHUB_TOKEN` and the legacy claude `ANTHROPIC_API_KEY` path respectively). This exists specifically so a real secret never has to sit in `accounts.json` — only a command that fetches it from a keyring.

### Dependencies

Only three external Go deps: `spf13/cobra` (CLI), `goccy/go-yaml` (YAML), `Masterminds/semver/v3` (version constraints). Keep it that way unless there's a strong reason not to.

## Docs

- `README.md` is the primary user-facing reference (config schema, CLI flags, isolated mode, accounts, recipe format) — check it before re-explaining behavior here.
- `docs/tools-catalog.md` — full table of built-in tool recipes, must stay in sync with `internal/installer/recipes.yaml`.
- `docs/package-manager-infrastructure.md` / `docs/package-manager-tokens-guide.md` — distribution/publishing setup (Homebrew tap, Scoop bucket, APT/DNF repo, tokens), not relevant to day-to-day code changes.
- `packaging/` holds the publish scripts and per-manager manifests (`apt`, `dnf`, `homebrew`, `chocolatey`, `scoop`) referenced by those docs.
