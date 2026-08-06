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

```bash
# APT (Debian / Ubuntu)
curl -fsSL https://accreation.github.io/aide-repo/gpg.key | sudo gpg --dearmor -o /etc/apt/trusted.gpg.d/aide.gpg
echo "deb https://accreation.github.io/aide-repo stable main" | sudo tee /etc/apt/sources.list.d/aide.list
sudo apt update && sudo apt install aide

# DNF (Fedora / RHEL)
sudo dnf config-manager --add-repo https://accreation.github.io/aide-repo/aide.repo
sudo dnf install aide

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
| `args` | No | `string` | Extra CLI arguments passed to the provider on launch (space-separated) |
| `tools` | Yes | `[]Tool` | List of required CLI tools |
| `tools[].name` | Yes | `string` | Executable name (must match a name in [recipes](#recipes-system)) |
| `tools[].version` | No | `string` | [Semver constraint](https://github.com/Masterminds/semver): `>=`, `^`, `~`, `=`, or compound ranges (e.g., `>=1.0.0 <2.0.0`) |

**Full example with all fields:**

```yaml
provider: claude
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
| `aide cache clear` | — | Clear the remote recipes cache |
| `aide --version` | — | Print version and exit |
| `aide --help` | — | Show help |

### Flags

| Flag | Applies to | Description |
|------|-----------|-------------|
| `--provider`, `-p` | `init` | AI provider name (default: `claude`) |
| `--name`, `-n` | `init` | Register project in global registry for `aide start` |
| `--version`, `-v` | `add` | Pin a specific version constraint |
| `--recipes-url` | *(global)* | URL to fetch external recipes from (env: `AIDE_RECIPES_URL`) |

### Exit Codes

| Code | Meaning |
|------|---------|
| `0` | All checks passed, provider launched |
| `1` | Something missing, outdated, or errored |

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
  requires:              # optional — prerequisites installed first if missing
    - pipx
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

- `~/.aide/` — configuration directory (cache, project registry)
- `~/.local/bin/` — user-local binary directory for GitHub release installations

---

## Architecture

### Package Layout

```
cmd/            — CLI surface (cobra commands): root, init, add, install, start, cache
internal/
  config/       — aide.yaml parsing (find upwards from cwd, parse YAML)
  checker/      — verifies binaries exist in PATH and satisfy semver constraints
  installer/    — resolves tool → package-manager mapping via embedded recipes.yaml, executes install
  launcher/     — execs the provider binary inheriting stdin/stdout/stderr
  display/      — formats check/install results for terminal output
  project/      — named project registry (~/.aide/projects.json), used by `aide start`
  semver/       — extracts versions from --version output and checks constraints
```

### Key Design Decisions

- **Recipes are embedded** (`internal/installer/recipes.yaml` via `//go:embed`) and optionally overridden by a remote URL. Remote → external file → embedded priority.
- **Config discovery walks up**: `config.FindAndParse` searches from cwd up to the filesystem root for `aide.yaml`, so commands work from any subdirectory.
- **Package manager resolution** is OS-specific: the `Recipe` struct has `windows`, `macos`, and `linux` keys, each a list of `PMEntry` maps tried in order. The first available PM on the system wins.
- **Version detection** tries `--version` first, then `-v`, then the `version` subcommand. Uses regex to extract the first semver from the output.
- **`aide start`** uses a JSON registry at `~/.aide/projects.json` (name → absolute path), chdir's into the project, then runs the full check + launch flow.
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
