# Spec: Named Project Registration & Start

**Date:** 2026-08-05
**Status:** draft

## Goal

Allow users to register projects by name during `aide init` and later jump to and launch them from any directory using `aide start <name>`.

## Feature 1: `aide init --name`

Add an optional `--name` / `-n` flag to `aide init`.

**Behavior:**
1. Create `aide.yaml` as before (unchanged).
2. If `--name` is provided, register the project in `~/.aide/projects.json`.
3. If the name already exists in the registry, overwrite the path (last init wins).

**Registry format** (`~/.aide/projects.json`):
```json
{
  "myproject": "/home/user/dev/myproject",
  "other": "/home/user/dev/other"
}
```

A simple JSON map: name → absolute path.

## Feature 2: `aide start <name>`

New subcommand. Takes exactly one argument: the project name.

**Behavior:**
1. Read `~/.aide/projects.json`.
2. Look up the project name → get the absolute path.
3. Error with a helpful message if the name is not found.
4. `os.Chdir(path)` — jump to the project directory.
5. Call the same `runCheck` logic as the root command:
   - Find and parse `aide.yaml` (walking upward).
   - Check provider and tools.
   - If all OK, launch the provider.
   - If something is missing, print error and exit.

## New Package: `internal/project`

Encapsulates project registry operations:

```
internal/project/
  project.go       — Register, Get, List, path helpers
  project_test.go  — unit tests
```

**Functions:**
- `Register(name, path string) error` — adds or updates a project entry.
- `Get(name string) (string, error)` — returns the path for a name, or error if not found.
- `List() (map[string]string, error)` — returns all registered projects.
- `path() string` — returns `~/.aide/projects.json` (cross-platform: `os.UserHomeDir` + `.aide` + `projects.json`).

**Error handling:**
- Creates `~/.aide/` directory if it doesn't exist.
- Returns clear errors for missing projects ("project 'foo' not found. Registered projects: bar, baz").

## Files Changed

| File | Change |
|------|--------|
| `cmd/init.go` | Add `--name` flag; call `project.Register` after creating `aide.yaml` |
| `cmd/start.go` | **New file** — `start` subcommand |
| `internal/project/project.go` | **New file** — registry CRUD |
| `internal/project/project_test.go` | **New file** — unit tests |

## Non-Goals

- No `aide start` without arguments (listing projects could be a future enhancement).
- No deletion or renaming of projects (can be done manually in the JSON file for now).
- No validation that the path still exists at `start` time (fail fast with os.Chdir error).

## Edge Cases

- **Name collision**: overwrite — last `init` wins.
- **Project moved/deleted**: `os.Chdir` will fail with a clear OS error.
- **No `~/.aide/` directory**: created on first `Register` call.
- **Empty registry**: `Get` returns "project 'X' not found. No projects registered."
