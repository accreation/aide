<p align="center">
  <img src="https://raw.githubusercontent.com/accreation/aide/main/logo.png" alt="Aide" width="200" />
</p>

<h1 align="center">Aide</h1>

<p align="center">
  <strong>The <code>package.json</code> for your AI development environment.</strong><br/>
  One config file. One command. All your AI tools — installed, verified, ready.
</p>

<p align="center">
  <a href="https://github.com/accreation/aide/releases"><img src="https://img.shields.io/github/v/release/accreation/aide?color=blue" alt="Release" /></a>
  <a href="https://github.com/accreation/aide/actions"><img src="https://img.shields.io/github/actions/workflow/status/accreation/aide/ci.yml?branch=main" alt="CI" /></a>
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-AGPL--3.0-green" alt="License: AGPL-3.0" /></a>
  <a href="https://pkg.go.dev/github.com/accreation/aide"><img src="https://img.shields.io/badge/go-reference-blue?logo=go" alt="Go Reference" /></a>
</p>

---

## Table of Contents

- [The Problem](#the-problem)
- [How It Works](#how-it-works)
- [Quick Start](#quick-start)
- [Configuration Reference](#configuration-reference)
- [CLI Reference](#cli-reference)
- [Isolated Mode](#isolated-mode)
- [Multi-Account Switching](#multi-account-switching)
- [Supported Tools](#supported-tools)
- [Recipes System](#recipes-system)
- [Project Registry](#project-registry)
- [Architecture](#architecture)
- [Environment Variables](#environment-variables)
- [Building from Source](#building-from-source)
- [Package Manager Distribution](#package-manager-distribution)
- [The Philosophy](#the-philosophy)
- [License](#license)
- [Contributing](#contributing)

---

## The Problem

AI coding agents (Claude Code, GitHub Copilot, OpenAI Codex) need CLI tools installed on your machine to do real work — `gh` for GitHub, `glab` for GitLab, `az` for Azure, and so on. Today, every developer on a team has to:

- Remember which tools are needed
- Install them manually, one by one
- Keep versions in sync across the team
- Debug "works on my machine" when someone is missing a dependency

**Aide fixes this. Declare your AI environment once, and it just works — for everyone.**

## How It Works

```
aide.yaml   →   [Check]   →   [Install missing]   →   [Launch AI provider]
```

1. **Declare** your AI provider and required tools in `aide.yaml`
2. **Check** — `aide` verifies everything is installed with correct versions
3. **Install** — `aide install` fetches whatever is missing via native package managers
4. **Launch** — when all checks pass, your AI provider starts automatically

Aide discovers `aide.yaml` by walking up from the current working directory to the filesystem root — so commands work from any subdirectory of your project.

---

## Quick Start

### 1. Install Aide

#### macOS

```bash
brew install accreation/tap/aide
```

#### Linux

> ⚠️ **Name conflict:** Ubuntu/Debian have a system package called `aide` (Advanced Intrusion Detection Environment). Make sure to install **`aide-cli`** (not `aide`) to get this tool. The command you run after installation is still `aide`.

```bash
# APT (Debian / Ubuntu)
curl -fsSL https://accreation.github.io/aide-repo/gpg.key | sudo gpg --dearmor -o /etc/apt/trusted.gpg.d/aide.gpg
echo "deb https://accreation.github.io/aide-repo stable main" | sudo tee /etc/apt/sources.list.d/aide.list
sudo apt update && sudo apt install aide-cli

# DNF (Fedora / RHEL)
sudo dnf config-manager --add-repo https://accreation.github.io/aide-repo/aide.repo
sudo dnf install aide-cli

# Portable binary (any distro)
curl -fsSL https://github.com/accreation/aide/releases/latest/download/aide-linux-amd64.tar.gz | tar -xz
sudo mv aide-linux-amd64 /usr/local/bin/aide
```

#### Windows

```powershell
# Winget
winget install Accreation.Aide

# Chocolatey
choco install aide

# Scoop
scoop bucket add accreation https://github.com/accreation/scoop-bucket
scoop install aide

# One-liner installer (recommended if no package manager)
powershell -c "irm https://raw.githubusercontent.com/accreation/aide/main/install.ps1 | iex"
```

> 📦 **All releases**: [github.com/accreation/aide/releases](https://github.com/accreation/aide/releases)

#### Go Install

```bash
go install github.com/accreation/aide@latest
```

### 2. Initialize Your Project

```bash
cd your-project
aide init --provider claude --name myproject
```

This creates an `aide.yaml`:

```yaml
provider: claude

tools: []
```

Use `--name` to register your project in the global registry at `~/.aide/projects.json`. You can then jump back to it from any directory:

```bash
# From anywhere on your system
aide start myproject
```

> 💡 Already have an `aide.yaml`? Run `aide init --name myproject` to register an existing project without recreating the config.

### 3. Add Your Tools

```bash
aide add gh          # auto-detects installed version with >= constraint
aide add glab        # pins ">=1.0.0" if installed, or no constraint if missing
aide add az --version ">=2.60.0"
```

When `--version` is omitted, `aide add` tries `--version` then `version` subcommand to detect the installed version. If the tool is not found in PATH, it adds it without a version constraint.

Your `aide.yaml` now looks like:

```yaml
provider: claude

tools:
  - name: gh
    version: ">=2.65.0"
  - name: glab
  - name: az
    version: ">=2.60.0"
```

### 4. Check & Launch

```bash
aide                  # checks everything, launches provider if all OK
```

Output:

```
Aide — environment check
───────────────────────
  OK  provider: claude
  OK  gh v2.65.0 (>=2.65.0)
  OK  glab
  FAIL  az — not found

Some items are missing or outdated. Run 'aide install' to fix.
```

### 5. Install Missing Tools

```bash
aide install          # or: aide i
```

Aide resolves the best available native package manager on your system for each tool and installs it. After installation, it re-checks all tools and launches your AI provider if everything passes.

```bash
aide check            # check only — no install, no launch (for CI pipelines)
```

---

## Configuration Reference

### `aide.yaml`

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `provider` | Yes | `string` | AI provider binary name: `claude`, `copilot`, `codex`, `opencode` |
| `account` | No | `string` | Name of a registered account from `~/.aide/accounts.json` (see [Multi-Account Switching](#multi-account-switching)) |
| `mode` | No | `string` | `"isolated"` for project-local tool isolation (see [Isolated Mode](#isolated-mode)) |
| `args` | No | `string` | Extra CLI arguments passed to the provider on launch (space-separated) |
| `tools` | Yes | `[]Tool` | List of required CLI tools |
| `tools[].name` | Yes | `string` | Executable name (must match a name in [recipes](#recipes-system)) |
| `tools[].version` | No | `string` | [Semver constraint](https://github.com/Masterminds/semver): `>=`, `^`, `~`, `=`, or compound ranges (e.g., `>=1.0.0 <2.0.0`) |

**Full example with all fields:**

```yaml
provider: claude
account: work-account
mode: isolated
args: "--dangerously-skip-permissions"

tools:
  - name: gh
    version: ">=2.65.0"
  - name: git
    version: ">=2.40.0"
  - name: docker
  - name: python
    version: "^3.10.0"
  - name: nodejs
    version: ">=18.0.0"
  - name: terraform
    version: "~1.5.0"
```

### Mode: Isolated vs Global

| Mode | Description |
|------|-------------|
| *(omitted)* | **Global mode** — tools are installed system-wide via native package managers. PATH is unchanged. |
| `isolated` | **Isolated mode** — tools supporting isolation are installed to `.aide/store/` and exposed via shims in `.aide/shims/`. Only `.aide/shims/` is prepended to PATH. Other tools fall back to system install. |

### Version Constraint Formats

Aide uses [Masterminds/semver](https://github.com/Masterminds/semver) for version comparison. Supported operators:

| Operator | Example | Meaning |
|----------|---------|---------|
| `>=` | `>=1.5.0` | Version 1.5.0 or higher |
| `^` | `^1.5.0` | Compatible with 1.5.0 (< 2.0.0) |
| `~` | `~1.5.0` | Approximately 1.5.0 (>= 1.5.0, < 1.6.0) |
| `=` | `=1.5.0` | Exactly 1.5.0 |
| Compound | `>=1.0.0 <2.0.0` | Range between versions |
| Empty | *(no value)* | Any version (only checks existence in PATH) |

---

## CLI Reference

### Commands

| Command | Alias | Description |
|---------|-------|-------------|
| `aide` | — | Check environment + launch provider if all OK |
| `aide start <name>` | — | Jump to a registered project directory and run the full check + launch |
| `aide check` | — | Check only — no install, no launch |
| `aide install` | `aide i` | Check, install missing tools, then launch provider |
| `aide init` | — | Create `aide.yaml` in current directory |
| `aide add <tool>` | — | Add or update a tool entry in `aide.yaml` |
| `aide account add <name>` | — | Register a provider account (Copilot, Claude, Codex) |
| `aide account list` | — | List all registered accounts |
| `aide account remove <name>` | — | Remove a registered account |
| `aide cache clear` | — | Clear the remote recipes cache |
| `aide --version` | — | Print version and exit |
| `aide --help` | — | Show help |

### Flags

| Flag | Applies to | Description |
|------|-----------|-------------|
| `--provider`, `-p` | `init`, `account add` | AI provider name (default: `claude`) |
| `--name`, `-n` | `init` | Register project in global registry for `aide start` |
| `--isolated` | `init` | Enable project-local tool isolation (`mode: isolated`) |
| `--version`, `-v` | `add` | Pin a specific version constraint |
| `--user` | `account add` | GitHub username (for Copilot accounts) |
| `--api-key` | `account add` | API key (for Claude accounts) |
| `--codex-home` | `account add` | Codex home directory path (for Codex accounts) |
| `--command` | `account add` | Credential broker command; its stdout (trimmed) becomes the account's secret instead of `--token`/`--api-key` |
| `--recipes-url` | *(global)* | URL to fetch external recipes from (env: `AIDE_RECIPES_URL`) |
| `--account` | *(global)* | Account name to use, overriding `aide.yaml` (env: `AIDE_ACCOUNT`) — see [Account Binding Precedence](#account-binding-precedence) |

### Exit Codes

| Code | Meaning |
|------|---------|
| `0` | All checks passed, provider launched |
| `1` | Something missing, outdated, or errored |

---

## Isolated Mode

Isolated mode installs tools **per-project** — no global system changes, no version conflicts between projects, and fully reproducible environments.

### How It Works

```
project/
├── aide.yaml          # mode: isolated
├── .aide/
│   ├── store/         # local tool installations
│   │   ├── rtk/
│   │   │   └── 0.5.0/
│   │   │       └── bin/
│   │   │           └── rtk.exe
│   │   └── graphify/
│   │       └── 1.2.0/
│   │           └── bin/
│   │               └── graphify
│   └── shims/         # PATH entries that resolve to store binaries
│       ├── rtk → ../store/rtk/0.5.0/bin/rtk       (Unix: symlink)
│       └── rtk.cmd                                   (Windows: .cmd wrapper)
```

1. **Init with isolation**: `aide init --provider claude --isolated` creates `aide.yaml` with `mode: isolated`
2. **Install tools**: `aide install` downloads tools to `.aide/store/<tool>/<version>/bin/` and creates shims in `.aide/shims/`
3. **Launch**: Aide prepends `.aide/shims/` to `PATH` before launching the AI provider — the provider sees only the project's tool versions
4. **`.aide/` is gitignored**: Aide automatically adds `.aide/` to `.gitignore` (if not already present)

### Supported Isolation Types

| Recipe type | Behavior in isolated mode |
|-------------|---------------------------|
| `github` | ✅ Downloaded to `.aide/store/`, shim created |
| `pipx` | ✅ Installed to `.aide/store/`, shim created |
| `winget`, `brew`, `apt`, `dnf`, `choco`, `scoop` | ⚠️ Falls back to global install with a warning (system PMs install globally by nature) |
| `curl` | ⚠️ Falls back to global install |

### Example: Isolated Project

```bash
# Create an isolated project
aide init --provider claude --isolated --name my-isolated-project

# Add tools — they'll be installed locally
aide add rtk
aide add graphify

# Install (downloads to .aide/store/, creates shims)
aide install

# Your aide.yaml now looks like:
# provider: claude
# mode: isolated
# tools:
#   - name: rtk
#   - name: graphify
```

### Version Resolution

When a tool has a version constraint (e.g., `>=2.0.0`), isolated mode resolves the latest matching GitHub release. If no constraint is specified, the latest release is used. Each version gets its own directory in `.aide/store/`, allowing different projects to pin different versions of the same tool.

---

## Multi-Account Switching

Aide supports **named provider accounts** — configure multiple Claude, Codex, Copilot, or OpenCode accounts and switch between them per project.

Every provider account is a **credential profile**: a directory under `~/.aide/accounts/<name>/` that aide binds a launched process to via environment variables (`CLAUDE_CONFIG_DIR`, `CODEX_HOME`, `GH_CONFIG_DIR`/`COPILOT_HOME` for copilot, `XDG_DATA_HOME` for opencode). This isolates the whole credential and session state — including subscription/OAuth logins, not just API keys — so two projects on two different companies' subscriptions never share or overwrite each other's session.

### Account Management

```bash
# Add an account. With no legacy flags (--api-key / --codex-home / --user),
# this creates a credential profile instead of a per-provider field override.
aide account add acme-claude --provider claude
aide account add acme-codex --provider codex
aide account add acme-opencode --provider opencode

# Copilot profiles can be seeded with a PAT up front (fine-grained, with
# "Copilot Requests" permission, or a gh-issued gho_/ghu_ token — classic
# ghp_ tokens are rejected)...
aide account add acme-copilot --provider copilot --token github_pat_xxx
# ...or left blank and authenticated via 'aide account login', which runs
# `gh auth login --insecure-storage` scoped to this profile's GH_CONFIG_DIR.
aide account add acme-copilot --provider copilot

# Authenticate the profile (runs the provider's own login flow, scoped to
# this profile's directory — subscription/OAuth logins work here).
aide account login acme-claude

# Check whether a profile is actually logged in
aide account status acme-claude

# Adopt an existing directory as the profile root (e.g. a CODEX_HOME you
# already use) instead of creating a new one under ~/.aide/accounts/
aide account add my-codex --provider codex --dir /path/to/existing/codex-home

# Print an account's credential profile directory
aide account path acme-claude

# List registered accounts
aide account list

# Remove an account. If it has a credential profile on disk, pass one of:
#   --force            also delete the profile directory
#   --keep-credentials remove the entry but leave the profile on disk
aide account remove acme-claude --force
```

> Copilot's identity resolution silently falls through several tiers (its own token env var, `GH_TOKEN`, `GITHUB_TOKEN`, its own credential store, then `gh auth token`) — a bad or missing credential doesn't error, it just runs (and bills) as whatever `gh auth token` returns ambiently. The copilot adapter scrubs `GH_TOKEN`/`GITHUB_TOKEN` and pins `GH_CONFIG_DIR` so that fallback resolves inside the profile instead of your real `gh` session. This has only been verified on Linux with GNOME Keyring; on macOS/Windows copilot's credential store is process-global (and `gh` may be absent from `PATH` on Windows), so double-check isolation there before relying on it.

> **Provider auto-updaters run per profile with aide's own version check, not with the provider's.** `claude`'s auto-updater installs to a global location outside `CLAUDE_CONFIG_DIR` (`~/.local/share/claude/versions`, symlinked from `~/.local/bin/claude`) — left alone, two profiles launched concurrently would race over that one shared install, and the binary could drift out from under aide's `>=` constraint between runs. Credential-profile launches set `DISABLE_AUTOUPDATER=1` for `claude` (and `COPILOT_AUTO_UPDATE=false` for copilot, unchanged) so the version aide checked is the version that runs; bump it yourself by raising the constraint in `aide.yaml`. Accounts using the legacy (non-profile) fields are unaffected. `claude`'s background/`--bg` daemon separately refuses to start (exit 1) whenever `CLAUDE_CONFIG_DIR` is set — not a concern here since aide always execs the provider in the foreground.

> **macOS/Windows isolation is unverified beyond Linux.** `claude` stores credentials in the Keychain on macOS and Credential Manager (or a file) on Windows, keyed by a hash of `CLAUDE_CONFIG_DIR`'s absolute path — profiles should separate cleanly there by the same logic that separates them on Linux, but it's never been run on either OS. `opencode`'s reliance on `XDG_DATA_HOME` for isolation is unverified on every platform, Linux included. Until someone runs a real two-account login (and post-idle token refresh) test on macOS and Windows, treat multi-account isolation for `claude`, `copilot`, and `opencode` as Linux-confirmed only — see [`docs/credential-profile-verification.md`](./docs/credential-profile-verification.md) for the runbook to close this out.

Accounts are stored in `~/.aide/accounts.json` with permissions `0600`; `~/.aide` itself and the credential profile directories under `~/.aide/accounts/` are `0700` (owner-only — never world-readable). See [Known limitations](#known-limitations) for what these modes do and don't guarantee.

### Using Accounts in Projects

Reference an account in your `aide.yaml`:

```yaml
provider: claude
account: acme-claude

tools:
  - name: gh
```

`aide` verifies the account is actually logged in as part of its normal check, before launching the provider — a misconfigured or logged-out account fails the check (exit 1) rather than silently launching on the wrong identity:

```
Aide — environment check
───────────────────────
  OK    provider: claude
  OK    account: acme-claude — anar@acme.example
  OK    gh >= 2.65.0 (2.97.0)
```

This means you can have different projects use different accounts — no manual switching needed. Launch any project with `aide` (or `aide start`) and the right account is applied automatically.

> 💡 Combine isolated mode with account switching for fully self-contained project environments: `mode: isolated` + `account: acme-claude` gives you both tool isolation and account isolation.

### Account Binding Precedence

`aide.yaml` is committed and cloned from strangers, so its `account:` field is trusted as a *name only* — never as something that overrides your own identity on your own machine. If you need a different account than what a repo declares (or the repo declares none at all), resolve it from highest to lowest precedence:

```
--account <name>              (flag)
AIDE_ACCOUNT=<name>           (env)
~/.aide/config.yaml bindings  (user-owned, path-conditional)
aide.yaml  account: <name>    (repo-declared, lowest)
```

`~/.aide/config.yaml` is never written by `aide` and never committed — you maintain it by hand, the same way you'd maintain a git `includeIf` block:

```yaml
# ~/.aide/config.yaml
bindings:
  - path: ~/work/acme/**
    account: acme-corp
  - path: ~/work/dcb/**
    # Per-provider map instead of a bare account: — useful if this repo's
    # provider might change (claude today, copilot tomorrow) without
    # having to edit this file to keep the same identity.
    accounts:
      claude: dcb-claude
      copilot: dcb-gh
  - path: ~/personal/**
    account: personal
```

Longest-prefix match wins, so a more specific binding (`~/work/acme/frontend/**`) overrides a broader one (`~/work/acme/**`) for repos nested under it. `aide start <name>` and `aide install` resolve the same way `aide` does — the binding is keyed off `aide.yaml`'s directory, not your shell's cwd, so `cd` tricks can't accidentally dodge it.

```bash
# One-off override, e.g. testing a repo under an account it doesn't declare
aide --account acme-personal

# Session-wide override
export AIDE_ACCOUNT=acme-personal
```

#### Credential brokers

An account's secret doesn't have to sit in `~/.aide/accounts.json` in the clear. Set `--command` instead of `--token`/`--api-key` and its stdout (trimmed) is used as the secret instead — the same shape as AWS's `credential_process` or git's `credential.helper`: whatever prints the credential to stdout works, including `op read`, `security find-generic-password`, `secret-tool lookup`, or `pass show`.

```bash
aide account add acme-copilot --provider copilot --command "op read op://work/acme-github/token"
aide account add personal-claude --provider claude --command "security find-generic-password -s anthropic-key -w"
```

The command re-runs on every launch — nothing is cached to disk — and, when set, always takes precedence over a `--token`/`--api-key` also present on the same account.

> `aide.yaml` can never carry a `command:` of its own, an env-interpolated account name, or a raw secret — only a plain account *name*, resolved against your own `~/.aide/accounts.json` and `~/.aide/config.yaml`. This is deliberate: a committed, cloned config file must never be able to dereference your credentials or your ambient environment on its own.

### Legacy accounts

Accounts created before credential profiles existed (or added with `--api-key` / `--codex-home`) keep working exactly as before — they're per-provider field overrides, not isolated credential profiles, and have no login/status support:

| Provider | Account action |
|----------|---------------|
| `claude` | Sets `ANTHROPIC_API_KEY=<api-key>` in the provider's environment (overrides subscription billing) |
| `codex` | Sets `CODEX_HOME=<path>` in the provider's environment |

```bash
aide account add personal-claude --provider claude --api-key sk-ant-xxx
aide account add my-codex --provider codex --codex-home /path/to/codex/config
```

**`copilot` accounts added with `--user` (before credential profiles) no longer work.** That flag ran `gh auth switch --user <username>` — a global mutation of `~/.config/gh/hosts.yml` that persisted after `aide` exited and made two concurrent `aide` sessions on different copilot accounts interfere with each other. That mechanism has been removed outright, not deprecated: `aide` refuses to launch with a clear error instead of silently running as whatever `gh` user happens to be active. `user:` was a pointer into global `gh` state, not a credential, so there is nothing to migrate automatically — re-add the account without `--user` (optionally with `--token`, see above) to get an isolated profile, then run `aide account login <name>`.

### Known limitations

Account isolation binds the *provider process* to a credential profile. Three things it deliberately does not do:

**The resolved secret is visible in the child's environment.** Whether it comes from `--token`, `--api-key`, or a `--command` broker, the credential ends up in the launched process's environment, where any other process running as the same user can read it (`/proc/<pid>/environ` on Linux, `ps -E`/`procstat` elsewhere). A broker keeps the secret out of `~/.aide/accounts.json` on disk; it does not keep it out of the process environment. This is worth naming explicitly because the child here is an LLM agent, which can be prompted into printing its own environment. Isolation between *accounts* is the guarantee; secrecy from other processes under your own UID is not. The known upgrade path is `aws-vault --server`-style brokering (a local endpoint that hands out short-lived credentials instead of exporting a long-lived one) and is out of scope.

**Tool credentials are not isolated — only the provider's.** The agent's own shell still runs `gh`, `glab`, `az` and friends with your global identity, so an agent working in a repo bound to `acme-corp` can still push to GitHub as your personal account. The one exception is a side effect: the copilot adapter pins `GH_CONFIG_DIR` into the profile, so `gh` invoked *inside* a copilot session resolves against the profile's own `gh` state. Nothing equivalent exists for the other three providers. If you need per-project tool identity today, set it up outside aide (a project-local `GH_CONFIG_DIR` via direnv, separate SSH keys, `git config user.email` per directory).

**POSIX modes are advisory here, not part of the guarantee.** `~/.aide` and each profile directory are 0700 and the index files 0600, re-asserted on write — but those bits are no-ops on Windows, and aide will not refuse to launch because a mode could not be enforced. Isolation rests on the environment plan and the pre-launch identity check, not on the filesystem permissions.

---

## Supported Tools

Aide ships with built-in installation recipes for **27 tools** across Windows, macOS, and Linux. The full catalog is maintained in [`docs/tools-catalog.md`](./docs/tools-catalog.md).

| Tool | Description | Windows | macOS | Linux |
|------|-------------|---------|-------|-------|
| `gh` | GitHub CLI | winget, scoop, choco | brew | apt, dnf |
| `glab` | GitLab CLI | winget | brew | apt, dnf |
| `az` | Azure CLI | winget | brew | curl |
| `aws` | AWS CLI | winget, scoop, choco | brew | curl |
| `gcloud` | Google Cloud CLI | winget, scoop | brew | curl |
| `git` | Git | winget | brew | apt, dnf |
| `docker` | Docker | winget, scoop, choco | brew | apt, dnf |
| `kubectl` | Kubernetes CLI | winget, scoop, choco | brew | apt, dnf |
| `helm` | Helm (K8s package manager) | winget, scoop, choco | brew | apt, dnf |
| `terraform` | Terraform (IaC) | winget, scoop, choco | brew | apt, dnf |
| `python` | Python 3.12 | winget, scoop, choco | brew | apt, dnf |
| `nodejs` | Node.js LTS | winget, scoop, choco | brew | apt, dnf |
| `bun` | Bun runtime | winget, scoop | brew | curl |
| `deno` | Deno runtime | winget, scoop, choco | brew | curl |
| `go` | Go toolchain | winget, scoop, choco | brew | apt, dnf |
| `rustup` | Rust toolchain | winget, scoop | curl | curl |
| `dotnet` | .NET SDK 9 | winget, scoop, choco | brew | curl |
| `java` | OpenJDK 21 (Temurin) | winget, scoop, choco | brew | apt, dnf |
| `pnpm` | pnpm package manager | winget, scoop, choco | brew | curl |
| `yarn` | Yarn package manager | winget, scoop, choco | brew | apt, dnf |
| `jq` | JSON processor | winget, scoop, choco | brew | apt, dnf |
| `rg` | ripgrep | winget, scoop, choco | brew | apt, dnf |
| `fd` | fd (find alternative) | winget, scoop, choco | brew | apt, dnf |
| `fzf` | Fuzzy finder | winget, scoop, choco | brew | apt, dnf |
| `rtk` | RTK AI toolkit | github releases | brew | curl |
| `graphify` | Code graph visualizer | pipx, pip | pipx, pip | pipx, pip |
| `pipx` | Python app runner | scoop, pip | brew | apt, dnf, pip |

---

## Recipes System

### How Recipes Work

Recipes define **how** to install each tool. They are embedded in the binary (`internal/installer/recipes.yaml`) via Go's `//go:embed`. At runtime, Aide uses them to resolve the best package manager available on the current OS.

### Recipe Priority

Recipes are loaded with this priority (highest wins):

1. **Remote URL** — `--recipes-url` flag or `AIDE_RECIPES_URL` env var (cached for 1 hour)
2. **External file** — a `recipes.yaml` file passed at load time
3. **Embedded** — the built-in recipes in the binary

This allows teams to add custom tools without modifying the binary — just host a `recipes.yaml` on your internal server.

### Recipe Format

```yaml
<tool-name>:
  arch_map:              # optional — maps GOARCH to tool-specific arch names
    amd64: x86_64
    arm64: aarch64
  requires:              # optional — prerequisites auto-installed before this tool
    - pipx               # example: graphify requires pipx to be installed first
  windows:               # per-OS install methods (tried in order)
    - winget: package-id
    - scoop: package-name
    - github: "owner/repo asset-pattern binary-name"
  macos:
    - brew: formula-name
  linux:
    - apt: package-name
    - dnf: package-name
    - curl: "https://install.sh | bash"
```

### Supported Package Managers

| PM | Used for | Notes |
|----|----------|-------|
| `winget` | Windows | `winget install --accept-source-agreements --accept-package-agreements` |
| `scoop` | Windows | `scoop install` |
| `choco` | Windows | `choco install -y` |
| `brew` | macOS | `brew install` |
| `apt` | Debian/Ubuntu | `apt install -y` |
| `dnf` | Fedora/RHEL | `dnf install -y` |
| `curl` | Any | Pipe to `bash` for shell-based installers |
| `github` | Any | Download release asset from GitHub Releases (`.zip` / `.tar.gz`), extract to `~/.local/bin` |
| `pip` | Any | Python package via pip |
| `pipx` | Any | Python app via pipx (isolated environment) |

### GitHub Release Installation

Tools with `github` recipes are downloaded from the latest GitHub release:

```
github: "owner/repo asset-pattern binary-name"
```

Aide finds the asset matching the pattern in the latest release, downloads it, extracts the binary, and places it in `~/.local/bin`. Template variables `${GOARCH}`, `${GOOS}`, `${ARCH}` (from `arch_map`) are supported in asset patterns.

### External Recipes

To override or extend recipes without rebuilding:

```bash
aide install --recipes-url https://your-org.example.com/recipes.yaml
```

Or set it globally:

```bash
export AIDE_RECIPES_URL="https://your-org.example.com/recipes.yaml"
```

Cached recipes expire after 1 hour. Use `aide cache clear` to force a fresh download.

---

## Project Registry

Aide maintains a global project registry at `~/.aide/projects.json` — a JSON map of project names to absolute paths.

**Register a project:**

```bash
aide init --name myproject
```

**Jump to a project from anywhere:**

```bash
aide start myproject
```

This changes directory to the project path and runs the full check + launch flow. Useful for quickly switching between projects or for shell aliases.

**Under the hood:**

- `~/.aide/` — configuration directory (cache, project registry, accounts)
- `~/.aide/projects.json` — project name → path registry
- `~/.aide/accounts.json` — provider accounts (permissions `0600`, owner read/write only)
- `~/.aide/accounts/<name>/` — credential profile directories for claude/codex accounts (permissions `0700`)
- `~/.local/bin/` — user-local binary directory for GitHub release installations

---

## Architecture

### Package Layout

```
cmd/            — CLI surface (cobra commands): root, init, add, install, start, account, cache
internal/
  account/      — provider account credentials (~/.aide/accounts.json) and claude/codex credential-profile adapters, used by checker and launcher
  config/       — aide.yaml parsing (find upwards from cwd, parse YAML)
  checker/      — verifies binaries exist in PATH (or .aide/shims for isolated mode) and satisfy semver constraints; verifies configured accounts are logged in
  installer/    — resolves tool → package-manager mapping via embedded recipes.yaml, executes install; handles project-local isolation (.aide/store/ + shims)
  launcher/     — execs the provider binary inheriting stdin/stdout/stderr; applies account switching and isolated PATH
  display/      — formats check/install results for terminal output
  project/      — named project registry (~/.aide/projects.json), used by `aide start`
  semver/       — extracts versions from --version output and checks constraints
  userconfig/   — parses ~/.aide/config.yaml (user-owned account bindings), longest-prefix path match against aide.yaml's directory
```

### Key Design Decisions

- **Recipes are embedded** (`internal/installer/recipes.yaml` via `//go:embed`) and optionally overridden by a remote URL. Remote → external file → embedded priority.
- **Config discovery walks up**: `config.FindAndParse` searches from cwd up to the filesystem root for `aide.yaml`, so commands work from any subdirectory.
- **Package manager resolution** is OS-specific: the `Recipe` struct has `windows`, `macos`, and `linux` keys, each a list of `PMEntry` maps tried in order. The first available PM on the system wins.
- **Version detection** tries `--version` first, then `-v`, then the `version` subcommand. Uses regex to extract the first semver from the output.
- **`aide start`** uses a JSON registry at `~/.aide/projects.json` (name → absolute path), chdir's into the project, then runs the full check + launch flow.
- **Isolated mode** installs tools to `.aide/store/<tool>/<version>/bin/` and creates shims in `.aide/shims/` — prepended to `PATH` before launching the provider. Only `github` and `pipx` recipes support full isolation; system PMs fall back to global install.
- **Account switching**: `claude`/`codex`/`copilot`/`opencode` accounts with a credential profile (`~/.aide/accounts/<name>/`, `0700`) bind a launched process to it via `internal/account`'s per-provider `Adapter` (`CLAUDE_CONFIG_DIR`, `CODEX_HOME`, `GH_CONFIG_DIR`/`COPILOT_HOME`, `XDG_DATA_HOME`), verified with a cheap identity check in `Checker.CheckAccount()` before launch. Accounts without a profile fall back to the pre-profile legacy fields in `~/.aide/accounts.json` (`0600`): `ANTHROPIC_API_KEY` for Claude, `CODEX_HOME` for Codex. Either fallback (and copilot's profile-based `Token`) can instead be sourced fresh on every launch from an `Account.Command` credential broker (`internal/account.ResolveToken`/`ResolveAPIKey`) rather than a stored secret.
- **Account binding precedence**: which account name is actually used is resolved once, in `cmd.resolveAccountName`, as `--account` flag > `AIDE_ACCOUNT` env > `~/.aide/config.yaml` path bindings (`internal/userconfig`, longest-prefix match against `aide.yaml`'s directory and provider) > `aide.yaml`'s own `account:` field — never the reverse, since `aide.yaml` is committed/cloned and must not get the deciding vote over the machine's actual owner.
- **PATH management** for GitHub release installs: Aide automatically adds `~/.local/bin` to your user PATH (via `~/.bashrc`, `~/.zshrc` on Unix, or `[Environment]::SetEnvironmentVariable` on Windows).

### Dependencies

Aide has exactly **3 external dependencies**:

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/goccy/go-yaml` | YAML parsing |
| `github.com/Masterminds/semver/v3` | Version constraint checking |

**Zero runtime dependencies** beyond Go's standard library for core logic.

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `AIDE_RECIPES_URL` | URL to fetch external recipes from (same as `--recipes-url` flag) |
| `AIDE_ACCOUNT` | Account name to use, overriding `aide.yaml` (same as `--account` flag) — see [Account Binding Precedence](#account-binding-precedence) |

Build-time variables (set via `-ldflags`):

| Variable | Default | Description |
|----------|---------|-------------|
| `cmd.Version` | `dev` | Binary version displayed by `aide --version` |
| `cmd.DefaultRecipesURL` | *(empty)* | URL for automatic remote recipe fetching (when set, no `--recipes-url` flag needed) |

---

## Building from Source

### Prerequisites

- Go 1.22 or later

### Build

```bash
# Simple build
make build

# Or directly:
go build -o aide.exe .

# With version and default recipes URL
make build VERSION=1.0.0 RECIPES_URL=https://example.com/recipes.yaml
```

The underlying command:

```bash
go build -ldflags "-X aide/cmd.Version=1.0.0 -X aide/cmd.DefaultRecipesURL=https://example.com/recipes.yaml" -o aide.exe .
```

### Cross-Platform Build

```bash
make build-all
```

Produces binaries in `dist/`:
- `dist/aide-windows-amd64.exe`
- `dist/aide-darwin-amd64`
- `dist/aide-darwin-arm64`
- `dist/aide-linux-amd64`

### Run Tests

```bash
# All tests with race detection
make test-race
# → go test -race ./internal/...

# Quick tests (no race detection)
make test
# → go test ./internal/...

# Run a single test
go test -race -run TestName ./internal/package/...
```

### Lint

```bash
make lint      # → go vet ./internal/...
make fmt       # → go fmt ./internal/... ./cmd/...
```

---

## Package Manager Distribution

Aide is distributed through **8 package managers** via automated CI/CD pipelines.

### Available Channels

| Platform | Package Manager | Repository |
|----------|----------------|------------|
| macOS | Homebrew | `accreation/homebrew-tap` |
| Windows | Winget | `microsoft/winget-pkgs` |
| Windows | Chocolatey | Community Repository |
| Windows | Scoop | `accreation/scoop-bucket` |
| Windows | PowerShell | One-liner via `install.ps1` |
| Linux | APT | `accreation.github.io/aide-repo` |
| Linux | DNF | `accreation.github.io/aide-repo` |
| Any | Go | `go install` |
| Any | GitHub Releases | Direct download |

### Publishing Infrastructure

See [`docs/package-manager-infrastructure.md`](./docs/package-manager-infrastructure.md) for details on:
- Repository setup (`homebrew-tap`, `scoop-bucket`, `aide-repo`)
- GPG signing key for APT/DNF
- GitHub Secrets configuration
- CI/CD workflow triggers

See [`docs/package-manager-tokens-guide.md`](./docs/package-manager-tokens-guide.md) for step-by-step token setup instructions.

### Release Process

1. Push a tag: `git tag v0.5.0 && git push origin v0.5.0`
2. CI builds cross-platform binaries and publishes to all package managers
3. Homebrew and Scoop repos are updated automatically
4. APT/DNF repos are updated with GPG-signed packages
5. Winget and Chocolatey publish with optional manual moderation for first release

---

## The Philosophy

Aide is to AI development environments what `package.json` + `npm install` is to Node.js projects:

- **Declarative** — the config is the source of truth
- **Reproducible** — `aide install` produces the same result on any machine
- **Portable** — check `aide.yaml` into git; every teammate gets the same environment
- **Non-invasive** — uses your system's native package managers, no custom registry
- **Extensible** — custom recipes via URL or local file, no fork needed

---

## License

Aide is licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)**.

**In plain terms:**

- ✅ **Free for personal and non-commercial use** — fork, modify, share freely
- ✅ **Free for internal company use** — run Aide inside your organization without restrictions
- ⚠️ **If you redistribute Aide or offer it as a service**, you must release your modifications under the same AGPL-3.0 license
- 💼 **Commercial licensing** — if you want to embed, resell, or distribute Aide without open-sourcing your changes, [contact us](mailto:licensing@accreation.com) for a commercial license

See [LICENSE](./LICENSE) for the full legal text.

---

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `go test -race ./...`
5. Run lint: `go vet ./...`
6. Format code: `go fmt ./internal/... ./cmd/...`
7. Open a pull request

### Adding New Tool Recipes

To add a new tool that Aide can install:

1. Edit [`internal/installer/recipes.yaml`](./internal/installer/recipes.yaml)
2. Follow the [recipe format](#recipe-format) described above
3. Update [`docs/tools-catalog.md`](./docs/tools-catalog.md) to keep the catalog in sync
4. Run tests: `go test -race ./internal/installer/...`

---

<p align="center">
  <sub>Built with Go · Cross-platform · AGPL-3.0</sub>
</p>
