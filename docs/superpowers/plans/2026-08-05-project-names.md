# Named Project Registration & Start — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `aide init --name <name>` to register projects and `aide start <name>` to jump to and launch them from any directory.

**Architecture:** New `internal/project` package manages a JSON registry at `~/.aide/projects.json`. `cmd/init.go` gets a `--name` flag that calls `project.Register`. New `cmd/start.go` looks up the project, `os.Chdir`s to it, then delegates to the existing `runCheck` logic.

**Tech Stack:** Go 1.22, `encoding/json`, `os`, cobra

## Global Constraints

- Use `os.UserHomeDir()` for cross-platform home directory resolution.
- Registry file: `~/.aide/projects.json` as a flat `map[string]string`.
- Name collision on `init --name`: overwrite (last wins).
- No new external dependencies — stdlib only.

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/project/project.go` | Create | `Register`, `Get`, `List` — JSON registry CRUD |
| `internal/project/project_test.go` | Create | Unit tests for all three functions |
| `cmd/init.go` | Modify | Add `--name` flag, call `project.Register` |
| `cmd/start.go` | Create | `start` subcommand — lookup, chdir, runCheck |

---

### Task 1: Creating the project registry package

**Files:**
- Create: `internal/project/project.go`
- Create: `internal/project/project_test.go`

**Interfaces:**
- Produces: `func Register(name, path string) error`, `func Get(name string) (string, error)`, `func List() (map[string]string, error)`

- [ ] **Step 1: Write the test file**

```go
package project

import (
	"os"
	"path/filepath"
	"testing"
)

func setHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

func TestRegisterAndGet(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	p := "/home/user/dev/myproject"
	if err := Register("myproject", p); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got, err := Get("myproject")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got != p {
		t.Errorf("expected %q, got %q", p, got)
	}
}

func TestRegisterCreatesAideDir(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	// Ensure .aide dir does not exist yet
	os.RemoveAll(filepath.Join(tmp, ".aide"))

	if err := Register("test", "/tmp/test"); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmp, ".aide")); os.IsNotExist(err) {
		t.Error(".aide directory was not created")
	}
}

func TestRegisterOverwrite(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	if err := Register("proj", "/old/path"); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := Register("proj", "/new/path"); err != nil {
		t.Fatalf("Register overwrite failed: %v", err)
	}

	got, err := Get("proj")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got != "/new/path" {
		t.Errorf("expected /new/path, got %q", got)
	}
}

func TestGetNotFound(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	_, err := Get("nosuchproject")
	if err == nil {
		t.Fatal("expected error for missing project")
	}
}

func TestGetNotFoundEmptyRegistry(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	_, err := Get("anything")
	if err == nil {
		t.Fatal("expected error when no projects registered")
	}
}

func TestList(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	Register("a", "/path/a")
	Register("b", "/path/b")

	all, err := List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 projects, got %d", len(all))
	}
	if all["a"] != "/path/a" {
		t.Errorf("expected /path/a, got %q", all["a"])
	}
	if all["b"] != "/path/b" {
		t.Errorf("expected /path/b, got %q", all["b"])
	}
}

func TestListEmpty(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	all, err := List()
	if err != nil {
		t.Fatalf("List on empty registry failed: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("expected 0 projects, got %d", len(all))
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd D:\work\aion && go test ./internal/project/... -v
```

Expected: build fails — package `project` not found.

- [ ] **Step 3: Write the implementation**

```go
package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func projectsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".aide", "projects.json"), nil
}

func ensureDir() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}
	dir := filepath.Join(home, ".aide")
	return os.MkdirAll(dir, 0755)
}

func readProjects() (map[string]string, error) {
	p, err := projectsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("reading %s: %w", p, err)
	}

	var projects map[string]string
	if err := json.Unmarshal(data, &projects); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", p, err)
	}
	if projects == nil {
		projects = map[string]string{}
	}
	return projects, nil
}

func writeProjects(projects map[string]string) error {
	if err := ensureDir(); err != nil {
		return err
	}

	p, err := projectsPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(projects, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling projects: %w", err)
	}

	if err := os.WriteFile(p, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", p, err)
	}
	return nil
}

// Register adds or updates a project entry in the registry.
func Register(name, path string) error {
	projects, err := readProjects()
	if err != nil {
		return err
	}
	projects[name] = path
	return writeProjects(projects)
}

// Get returns the path for a registered project name.
func Get(name string) (string, error) {
	projects, err := readProjects()
	if err != nil {
		return "", err
	}

	p, ok := projects[name]
	if !ok {
		if len(projects) == 0 {
			return "", fmt.Errorf("project %q not found — no projects registered. Run 'aide init --name %s' first", name, name)
		}
		names := make([]string, 0, len(projects))
		for n := range projects {
			names = append(names, n)
		}
		sort.Strings(names)
		return "", fmt.Errorf("project %q not found. Registered projects: %s", name, strings.Join(names, ", "))
	}
	return p, nil
}

// List returns all registered projects.
func List() (map[string]string, error) {
	return readProjects()
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
cd D:\work\aion && go test ./internal/project/... -v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/project/project.go internal/project/project_test.go
git commit -m "feat: add project registry package for named projects"
```

---

### Task 2: Adding --name flag to aide init

**Files:**
- Modify: `cmd/init.go`

**Interfaces:**
- Consumes: `project.Register(name, path string) error`
- Produces: `--name` / `-n` flag on `init` command

- [ ] **Step 1: Add the --name flag and registration call**

Edit `cmd/init.go`:

```go
package cmd

import (
	"fmt"
	"os"

	"aide/internal/config"
	"aide/internal/project"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

var initProvider string
var initName string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create aide.yaml in the current directory",
	Long:  "Generates a default aide.yaml with the specified provider. Use --name to register the project for 'aide start'.",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().StringVarP(&initProvider, "provider", "p", "claude", "AI provider name (claude, copilot, codex, opencode)")
	initCmd.Flags().StringVarP(&initName, "name", "n", "", "Register project with a name for 'aide start'")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	path := "aide.yaml"
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", path)
	}

	cfg := config.GenerateDefault(initProvider)
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	fmt.Printf("Created %s with provider: %s\n", path, initProvider)
	fmt.Println("Add your tools to the 'tools' list, then run 'aide check'.")

	if initName != "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}
		if err := project.Register(initName, cwd); err != nil {
			return fmt.Errorf("registering project: %w", err)
		}
		fmt.Printf("Registered project %q — use 'aide start %s' to jump here from anywhere.\n", initName, initName)
	}

	return nil
}
```

- [ ] **Step 2: Build and verify the flag works**

```bash
cd D:\work\aion && go build -o aide.exe .
```

Then in a temp directory:

```bash
mkdir C:\Temp\test-aion-init && cd C:\Temp\test-aion-init && D:\work\aion\aide.exe init --name testproj --provider claude
```

Expected: creates `aide.yaml` and prints registration message.

- [ ] **Step 3: Commit**

```bash
git add cmd/init.go
git commit -m "feat: add --name flag to aide init for project registration"
```

---

### Task 3: Creating the aide start command

**Files:**
- Create: `cmd/start.go`

**Interfaces:**
- Consumes: `project.Get(name string) (string, error)`, `runCheck(cmd, args)` (existing in `cmd/root.go`)
- Produces: `start` subcommand

- [ ] **Step 1: Write the start command**

Create `cmd/start.go`:

```go
package cmd

import (
	"fmt"
	"os"

	"aide/internal/project"

	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start <name>",
	Short: "Jump to a registered project and launch aide",
	Long:  "Looks up a project registered with 'aide init --name', changes to its directory, and runs the full aide check + launch.",
	Args:  cobra.ExactArgs(1),
	RunE:  runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
	name := args[0]

	projectPath, err := project.Get(name)
	if err != nil {
		return err
	}

	if err := os.Chdir(projectPath); err != nil {
		return fmt.Errorf("changing to project directory %q: %w", projectPath, err)
	}

	// Run the standard aide check + launch flow (from root.go)
	return runCheck(cmd, args)
}
```

- [ ] **Step 2: Build and verify**

```bash
cd D:\work\aion && go build -o aide.exe .
```

Test start with the previously registered project:

```bash
D:\work\aion\aide.exe start testproj
```

Expected: changes to the project directory, runs check + launches provider.

Test with unknown name:

```bash
D:\work\aion\aide.exe start nonexistent
```

Expected: error message listing registered projects.

- [ ] **Step 3: Run full test suite**

```bash
cd D:\work\aion && go test ./... -v
```

Expected: all tests PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/start.go
git commit -m "feat: add aide start command to jump to registered projects"
```

---

### Task 4: Final verification

- [ ] **Step 1: End-to-end test**

```bash
# Create temp project
mkdir C:\Temp\aion-e2e && cd C:\Temp\aion-e2e
D:\work\aion\aide.exe init --name e2etest --provider copilot

# From another directory, start it
cd C:\Temp
D:\work\aion\aide.exe start e2etest
```

Expected: switches to `C:\Temp\aion-e2e`, runs check, launches provider.

- [ ] **Step 2: Clean up temp directories**

```bash
Remove-Item -Recurse -Force C:\Temp\test-aion-init -ErrorAction SilentlyContinue
Remove-Item -Recurse -Force C:\Temp\aion-e2e -ErrorAction SilentlyContinue
```

- [ ] **Step 3: Final commit if any cleanup**

No changes expected.
