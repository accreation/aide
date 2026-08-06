## Build, Test, and Lint Commands

- Build: `go build -o aide.exe .` (or `make build`)
- Run all tests with race detection: `go test -race ./...`
- Run a single test: `go test -race -run TestName ./internal/package/...`
- Lint (vet): `go vet ./...`
- Format: `go fmt ./internal/... ./cmd/...`
- Cross-platform build: `make build-all`

## Architecture

**Aide** is "the `package.json` for AI development environments" — it reads an `aide.yaml` config, verifies that AI provider CLI tools and declared tool dependencies are installed with correct versions, optionally installs missing ones via native package managers, and launches the AI provider.

### Package Layout

```
cmd/         — CLI surface (cobra commands): root/check, init, add, install, start, cache
internal/
  config/    — aide.yaml parsing (find upwards from cwd, parse YAML)
  checker/   — verifies binaries exist in PATH and satisfy semver constraints
  installer/ — resolves tool → package-manager mapping via embedded recipes.yaml, executes install
  launcher/  — execs the provider binary inheriting stdin/stdout/stderr
  display/   — formats check/install results for terminal output
  project/   — named project registry (~/.aide/projects.json), used by `aide start`
  semver/    — extracts versions from --version output and checks constraints
```

### Key Design Decisions

- **Recipes are embedded** (`internal/installer/recipes.yaml` via `//go:embed`) and optionally overridden by a remote URL (`--recipes-url` flag or `AIDE_RECIPES_URL` env var). Remote → external file → embedded priority.
- **Config discovery walks up**: `config.FindAndParse` searches from cwd up to the filesystem root for `aide.yaml`, so commands work from any subdirectory.
- **Package manager resolution** is OS-specific: the `Recipes` struct has `windows`, `macos`, and `linux` keys, each a list of `PMEntry` maps tried in order. The first available PM on the system wins.
- **Version detection** in `cmd/add.go` tries `--version` first, then the `version` subcommand. The `checker` also tries `--version`, `-v`, then `version` subcommand.
- **`aide start`** uses a JSON registry at `~/.aide/projects.json` (name → absolute path), chdir's into the project, then runs the full check+launch flow.

## Key Conventions

- **Go 1.22**, module path `aide`. Only three external dependencies: cobra (CLI), go-yaml (YAML parsing), Masterminds/semver (version constraints).
- **Build-time variables** set via `-ldflags`: `cmd.Version` and `cmd.DefaultRecipesURL`.
- **Tests live alongside source** in `_test.go` files within each internal package. Use table-driven tests.
- **New tools are added** by editing `internal/installer/recipes.yaml`. The `github` PM type expects the format `owner/repo asset-pattern binary-name` and uses `installFromGithub` (downloads release asset by pattern matching). The `curl` PM type pipes to bash.
- **Add new CLI commands** by creating a new file in `cmd/`, registering it in its `init()` with `rootCmd.AddCommand(...)`.

## Agent Roles

- **`/rm {add|remove|update} <pkg>`** — Recipe maintainer. When invoked, read and strictly follow the instructions in `.github/agent-roles/recipes-maintainer.md`. This role handles adding, removing, and updating tool recipes in `internal/installer/recipes.yaml` and keeps `docs/tools-catalog.md` in sync.
