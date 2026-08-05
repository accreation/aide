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
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/accreation/aide?logo=go" alt="Go Version" /></a>
  <a href="https://goreportcard.com/report/github.com/accreation/aide"><img src="https://goreportcard.com/badge/github.com/accreation/aide" alt="Go Report Card" /></a>
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-AGPL--3.0-green" alt="License: AGPL-3.0" /></a>
  <a href="https://pkg.go.dev/github.com/accreation/aide"><img src="https://img.shields.io/badge/go-reference-blue?logo=go" alt="Go Reference" /></a>
</p>

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

## Quick Start

### 1. Install Aide

#### macOS (Homebrew)
```bash
brew install accreation/tap/aide
```

#### Linux
```bash
# Debian / Ubuntu
curl -fsSL https://github.com/accreation/aide/releases/latest/download/aide_amd64.deb -o aide.deb
sudo dpkg -i aide.deb

# Fedora / RHEL
curl -fsSL https://github.com/accreation/aide/releases/latest/download/aide-1.amd64.rpm -o aide.rpm
sudo rpm -i aide.rpm

# Portable binary (any distro)
curl -fsSL https://github.com/accreation/aide/releases/latest/download/aide-linux-amd64.tar.gz | tar -xz
sudo mv aide-linux-amd64 /usr/local/bin/aide
```

#### Windows
```powershell
# Download the portable executable
Invoke-WebRequest -Uri "https://github.com/accreation/aide/releases/latest/download/aide-windows-amd64.exe.zip" -OutFile aide.zip
Expand-Archive aide.zip -DestinationPath .
# Add aide.exe to your PATH, or run directly
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

Use `--name` to register your project so you can jump back to it from any directory:

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
✓ provider: claude
✓ gh v2.65.0 (>=2.65.0)
✓ glab
✗ az — not found

Some items are missing or outdated. Run 'aide install' to fix.
```

### 5. Install Missing Tools

```bash
aide install          # or just: aide i
```

Aide uses your system's native package managers (`brew`, `apt`, `winget`, `dnf`, `scoop`, `choco`) to install each tool, then launches your AI provider.

---

## Configuration Reference

### `aide.yaml`

| Field | Required | Description |
|-------|----------|-------------|
| `provider` | Yes | AI provider binary name: `claude`, `copilot`, `codex`, `opencode` |
| `args` | No | Extra CLI arguments passed to the provider on launch |
| `tools` | Yes | List of required CLI tools |
| `tools[].name` | Yes | Executable name |
| `tools[].version` | No | [Semver constraint](https://github.com/Masterminds/semver) (`>=`, `^`, `~`, `=`, or ranges) |

### Supported Tools (built-in recipes)

| Tool | Description | Windows | macOS | Linux |
|------|-------------|---------|-------|-------|
| `gh` | GitHub CLI | winget / scoop / choco | brew | apt / dnf |
| `glab` | GitLab CLI | winget | brew | apt / dnf |
| `az` | Azure CLI | winget | brew | curl |
| `git` | Git | winget | brew | apt / dnf |
| `rtk` | RTK AI tool | GitHub releases | brew | curl |

> 💡 Recipes are embedded in the binary. [Add more tools](https://github.com/accreation/aide/blob/main/internal/installer/recipes.yaml) or override with an external `recipes.yaml`.

---

## CLI Reference

```bash
aide                  # Check environment + launch provider if OK
aide start <name>     # Jump to a registered project and launch aide
aide check            # Check only (no launch)
aide install          # Check, install missing, launch
aide i                # Alias for install
aide init             # Create aide.yaml in current directory
  --provider, -p      #   AI provider name (claude, copilot, codex, opencode)
  --name, -n          #   Register project with a name for 'aide start'
aide add <tool>       # Add a tool to aide.yaml
  --version, -v       #   Pin a specific version constraint
aide cache clear      # Clear the remote recipes cache
aide --version        # Print version
aide --help           # Show help
```

### Exit Codes

| Code | Meaning |
|------|---------|
| `0` | All checks passed, provider launched |
| `1` | Something missing, outdated, or errored |

---

## The Philosophy

Aide is to AI development environments what `package.json` + `npm install` is to Node.js projects:

- **Declarative** — the config is the source of truth
- **Reproducible** — `aide install` produces the same result on any machine
- **Portable** — check `aide.yaml` into git; every teammate gets the same environment
- **Non-invasive** — uses your system's native package managers, no custom registry

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
5. Open a pull request

---

<p align="center">
  <sub>Built with Go · Cross-platform · AGPL-3.0</sub>
</p>
