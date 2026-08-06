# Recipes Maintainer Agent Role — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create a `recipes-maintainer` agent role with `/rm` slash command for adding/removing/updating recipes in `recipes.yaml` and maintaining a human-readable `docs/tools-catalog.md`.

**Architecture:** Add an `agent-roles/recipes-maintainer.md` instruction file that AI agents read when `/rm` is invoked. The command dispatches from `copilot-instructions.md`. A new `docs/tools-catalog.md` serves as the human-readable reference for all supported tools.

**Tech Stack:** Markdown (agent instructions), YAML (recipes), no code changes.

## Global Constraints

- Follow existing project conventions: Go 1.22, module path `aide`
- Recipes format: `internal/installer/recipes.yaml` with `windows/macos/linux` per-OS blocks
- Package managers: winget, scoop, choco, brew, apt, dnf, curl, github, pip, pipx
- Validation after any change: `go vet ./...` && `go test -race ./internal/installer/...`

---

### Task 1: Creating agent role instructions

**Files:**
- Create: `.github/agent-roles/recipes-maintainer.md`

**Interfaces:**
- Produces: Agent instructions file referenced by `copilot-instructions.md` in Task 2
- No dependencies on other tasks

- [ ] **Step 1: Create the role file**

Create `.github/agent-roles/recipes-maintainer.md`:

````markdown
# Recipes Maintainer

You are the **recipes-maintainer** agent for the Aide project. Your responsibility is maintaining `internal/installer/recipes.yaml` — the embedded recipe database that maps tool names to platform-specific package manager install commands — and keeping `docs/tools-catalog.md` in sync as a human-readable reference.

## Core Rules

1. **Always validate after changes**: run `go vet ./...` and `go test -race ./internal/installer/...` after any modification to `recipes.yaml`.
2. **Alphabetical order**: insert new recipes in alphabetical order by tool name.
3. **Research before adding**: when adding a tool, find its official website, GitHub repository, and available package managers before writing the recipe.
4. **Never remove without confirmation**: if the user says `/rm remove pkg`, confirm before deleting.

## Recipe Format Reference

Recipes live in `internal/installer/recipes.yaml`. Format:

```yaml
tool-name:
  arch_map:              # optional — maps GOARCH to tool-specific arch names
    amd64: x86_64
    arm64: aarch64
  requires:              # optional — prerequisites
    - pipx
  windows:               # per-OS, tried in order, first available PM wins
    - winget: PackageId
    - scoop: pkg-name
    - choco: pkg-name
    - github: "owner/repo asset-pattern binary-name"
    - curl: "url | sh"
    - pip: pkg-name
    - pipx: pkg-name
  macos:
    - brew: pkg-name
  linux:
    - apt: pkg-name
    - dnf: pkg-name
```

### Package Manager Types

| PM | Format | Example |
|----|--------|---------|
| `winget` | `winget: PackageId` | `winget: GitHub.cli` |
| `scoop` | `scoop: pkg-name` | `scoop: gh` |
| `choco` | `choco: pkg-name` | `choco: gh` |
| `brew` | `brew: pkg-name` | `brew: gh` |
| `apt` | `apt: pkg-name` | `apt: gh` |
| `dnf` | `dnf: pkg-name` | `dnf: gh` |
| `github` | `github: "owner/repo asset-pattern binary-name"` | `github: "cli/cli gh_*_windows_amd64.zip gh.exe"` |
| `curl` | `curl: "url \| sh"` | `curl: "https://install.example.com \| bash"` |
| `pip` | `pip: pkg-name` | `pip: requests` |
| `pipx` | `pipx: pkg-name` | `pipx: black` |

### Template Variables

Available in `github` and `curl` values:

| Variable | Source | Example |
|----------|--------|---------|
| `${GOARCH}` | `runtime.GOARCH` | `amd64`, `arm64` |
| `${GOOS}` | `runtime.GOOS` | `windows`, `darwin`, `linux` |
| `${OS}` | Alias for `${GOOS}` | `windows`, `darwin`, `linux` |
| `${ARCH}` | `arch_map[GOARCH]`, fallback to `GOARCH` | `x86_64`, `aarch64` |

## Commands

### `/rm add <pkg-name>`

Add a new tool recipe.

**Algorithm:**

1. **Research the package.** Use web search to find:
   - Official website and GitHub repository URL
   - Short description (5-10 words, what the tool does)
   - Available package managers per platform:
     - **Windows**: check winget (`winget search pkg`), scoop (`scoop search pkg`), choco (`choco search pkg`)
     - **macOS**: check brew (`brew search pkg`)
     - **Linux**: check apt (`apt search pkg`), dnf (`dnf search pkg`)
   - If the tool distributes via GitHub Releases, identify the repo and asset naming pattern

2. **Write the recipe block.**
   - Use the format above
   - Include `arch_map` only if the tool uses different arch names (e.g., `amd64` → `x86_64`)
   - Include `requires` only if the tool needs prerequisites (e.g., `pipx` before a pipx package)
   - List PM entries in order: most common/reliable first
   - For `github` type: `"owner/repo asset-pattern binary-name"` — three space-separated fields
   - For `curl` type: `"url | sh"` — pipe to shell

3. **Insert into `recipes.yaml`.**
   - Open `internal/installer/recipes.yaml`
   - Find the correct alphabetical position
   - Insert the new block with a blank line before and after

4. **Update `docs/tools-catalog.md`.**
   - Add a new row to the table in alphabetical order
   - Format: `| pkg-name | Description | [repo](url) | winget, scoop | brew | apt, dnf |`
   - List package managers as comma-separated values per platform

5. **Validate.**
   ```bash
   go vet ./...
   go test -race ./internal/installer/...
   ```

### `/rm remove <pkg-name>`

Remove a tool recipe.

**Algorithm:**

1. **Confirm.** Ask: "Remove `pkg-name` from recipes.yaml and tools-catalog.md?"
2. **Remove from `recipes.yaml`.** Delete the entire YAML block for the tool (from the tool name line through the last PM entry line, including trailing blank line).
3. **Remove from `docs/tools-catalog.md`.** Delete the table row for the tool.
4. **Validate.**
   ```bash
   go vet ./...
   go test -race ./internal/installer/...
   ```

### `/rm update <pkg-name>`

Update an existing recipe or its catalog description.

**Algorithm:**

1. **Read the current recipe** in `recipes.yaml` to understand what exists.
2. **Ask the user** what they want to change: package manager entries? description? arch_map?
3. **Apply the changes** — edit the YAML block in `recipes.yaml` and/or the table row in `docs/tools-catalog.md`.
4. **Validate.**
   ```bash
   go vet ./...
   go test -race ./internal/installer/...
   ```

## `docs/tools-catalog.md` Format

The catalog is a Markdown table:

```markdown
# Aide — Supported Tools Catalog

> Auto-generated from `internal/installer/recipes.yaml`.
> Maintained by the recipes-maintainer agent (`/rm`).

| Tool | Description | Official | Windows | macOS | Linux |
|------|-------------|----------|---------|-------|-------|
| aws  | AWS CLI — manage AWS services from terminal | [aws.amazon.com/cli](https://aws.amazon.com/cli/) | winget, scoop, choco | brew | curl |
| ...  | ... | ... | ... | ... | ... |
```

**Rules for table rows:**
- **Tool**: exact name from `recipes.yaml` key
- **Description**: 5-10 words, what the tool does, in English
- **Official**: `[display](url)` — link to GitHub repo or official website
- **Platform columns**: comma-separated list of package manager names from the recipe (e.g., `winget, scoop, choco`)

## Validation

After every change to `recipes.yaml`, run:

```bash
go vet ./...
go test -race ./internal/installer/...
```

If validation fails, report the error to the user and do NOT commit.
````

- [ ] **Step 2: Commit**

```bash
git add .github/agent-roles/recipes-maintainer.md
git commit -m "feat: add recipes-maintainer agent role instructions"
```

---

### Task 2: Adding `/rm` dispatch to copilot-instructions

**Files:**
- Modify: `.github/copilot-instructions.md`

**Interfaces:**
- Consumes: `.github/agent-roles/recipes-maintainer.md` from Task 1
- Produces: Updated instructions that dispatch `/rm` to the role file

- [ ] **Step 1: Add agent roles section to copilot-instructions.md**

Read the current file and append after the "Key Conventions" section. Add this block at the end:

```markdown
## Agent Roles

- **`/rm {add|remove|update} <pkg>`** — Recipe maintainer. When invoked, read and strictly follow the instructions in `.github/agent-roles/recipes-maintainer.md`. This role handles adding, removing, and updating tool recipes in `internal/installer/recipes.yaml` and keeps `docs/tools-catalog.md` in sync.
```

Edit `.github/copilot-instructions.md` — append the above block after the last line.

- [ ] **Step 2: Commit**

```bash
git add .github/copilot-instructions.md
git commit -m "feat: add /rm agent role dispatch to copilot-instructions"
```

---

### Task 3: Creating the tools catalog page

**Files:**
- Create: `docs/tools-catalog.md`

**Interfaces:**
- Produces: Human-readable catalog maintained by the recipes-maintainer agent
- No code dependencies

- [ ] **Step 1: Create the catalog with all current tools**

Read `internal/installer/recipes.yaml` to get the full list of tools. Create `docs/tools-catalog.md` with a table row for every tool currently in the recipes file.

Create `docs/tools-catalog.md`:

```markdown
# Aide — Supported Tools Catalog

> Auto-generated from `internal/installer/recipes.yaml`.
> Maintained by the recipes-maintainer agent (`/rm`).

| Tool | Description | Official | Windows | macOS | Linux |
|------|-------------|----------|---------|-------|-------|
| aws | AWS CLI — manage AWS services from terminal | [aws.amazon.com/cli](https://aws.amazon.com/cli/) | winget, scoop, choco | brew | curl |
| az | Azure CLI — manage Azure resources from terminal | [github.com/Azure/azure-cli](https://github.com/Azure/azure-cli) | winget | brew | curl |
| bun | Bun — fast JavaScript runtime, bundler, and package manager | [bun.sh](https://bun.sh) | winget, scoop | brew | curl |
| deno | Deno — secure JavaScript/TypeScript runtime | [deno.com](https://deno.com) | winget, scoop, choco | brew | curl |
| docker | Docker — container platform for building and running applications | [docker.com](https://www.docker.com) | winget, scoop, choco | brew | apt, dnf |
| dotnet | .NET SDK — build cross-platform applications with C#/F# | [dotnet.microsoft.com](https://dotnet.microsoft.com) | winget, scoop, choco | brew | curl |
| fd | fd — fast, user-friendly alternative to `find` | [github.com/sharkdp/fd](https://github.com/sharkdp/fd) | winget, scoop, choco | brew | apt, dnf |
| fzf | fzf — fuzzy finder for the command line | [github.com/junegunn/fzf](https://github.com/junegunn/fzf) | winget, scoop, choco | brew | apt, dnf |
| gcloud | Google Cloud CLI — manage GCP resources from terminal | [cloud.google.com/sdk](https://cloud.google.com/sdk) | winget, scoop | brew | curl |
| gh | GitHub CLI — manage repos, PRs, issues from terminal | [github.com/cli/cli](https://github.com/cli/cli) | winget, scoop, choco | brew | apt, dnf |
| git | Git — distributed version control system | [git-scm.com](https://git-scm.com) | winget | brew | apt, dnf |
| glab | GitLab CLI — manage GitLab repos, MRs, issues from terminal | [gitlab.com/gitlab-org/cli](https://gitlab.com/gitlab-org/cli) | winget | brew | apt, dnf |
| go | Go — systems programming language and toolchain | [go.dev](https://go.dev) | winget, scoop, choco | brew | apt, dnf |
| graphify | Graphify — generate visual graphs from codebases | [github.com/rtk-ai/graphify](https://github.com/rtk-ai/graphify) | pipx, pip | pipx, pip | pipx, pip |
| helm | Helm — Kubernetes package manager | [helm.sh](https://helm.sh) | winget, scoop, choco | brew | apt, dnf |
| java | OpenJDK — Java development kit (Temurin 21) | [adoptium.net](https://adoptium.net) | winget, scoop, choco | brew | apt, dnf |
| jq | jq — lightweight JSON processor for the command line | [github.com/jqlang/jq](https://github.com/jqlang/jq) | winget, scoop, choco | brew | apt, dnf |
| kubectl | kubectl — Kubernetes command-line tool | [kubernetes.io](https://kubernetes.io/docs/reference/kubectl/) | winget, scoop, choco | brew | apt, dnf |
| nodejs | Node.js — JavaScript runtime (LTS) | [nodejs.org](https://nodejs.org) | winget, scoop, choco | brew | apt, dnf |
| pipx | pipx — install and run Python applications in isolated environments | [github.com/pypa/pipx](https://github.com/pypa/pipx) | scoop, pip | brew | apt, dnf, pip |
| pnpm | pnpm — fast, disk-space efficient package manager | [pnpm.io](https://pnpm.io) | winget, scoop, choco | brew | curl |
| python | Python — programming language runtime (3.12) | [python.org](https://www.python.org) | winget, scoop, choco | brew | apt, dnf |
| rg | ripgrep — ultra-fast text search tool | [github.com/BurntSushi/ripgrep](https://github.com/BurntSushi/ripgrep) | winget, scoop, choco | brew | apt, dnf |
| rtk | RTK — AI-powered terminal toolkit | [github.com/rtk-ai/rtk](https://github.com/rtk-ai/rtk) | github | brew | curl |
| rustup | Rustup — Rust toolchain installer | [rustup.rs](https://rustup.rs) | winget, scoop | curl | curl |
| terraform | Terraform — infrastructure as code tool by HashiCorp | [terraform.io](https://www.terraform.io) | winget, scoop, choco | brew | apt, dnf |
| yarn | Yarn — fast, reliable JavaScript package manager | [yarnpkg.com](https://yarnpkg.com) | winget, scoop, choco | brew | apt, dnf |
```

- [ ] **Step 2: Commit**

```bash
git add docs/tools-catalog.md
git commit -m "docs: add tools catalog with all current recipes"
```
