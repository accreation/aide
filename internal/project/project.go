package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"aide/internal/fsutil"
)

// lockTimeout bounds how long Register waits for a concurrent aide
// process to finish its own read-modify-write of projects.json.
const lockTimeout = 5 * time.Second

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

	if err := fsutil.WriteFileAtomic(p, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", p, err)
	}
	return nil
}

// Register adds or updates a project entry in the registry.
func Register(name, path string) error {
	if err := ensureDir(); err != nil {
		return err
	}
	p, err := projectsPath()
	if err != nil {
		return err
	}
	unlock, err := fsutil.Lock(p, lockTimeout)
	if err != nil {
		return err
	}
	defer unlock()

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
