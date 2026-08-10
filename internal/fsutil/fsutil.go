// Package fsutil provides small helpers for safely reading and writing
// shared state files (e.g. ~/.aide/accounts.json, ~/.aide/projects.json)
// that may be touched by concurrent aide processes.
package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// staleAfter is how long a lock file may exist before it is considered
// abandoned (e.g. left behind by a killed process) and safe to remove.
const staleAfter = 10 * time.Second

// WriteFileAtomic writes data to path by first writing to a temp file in
// the same directory and then renaming it into place, so readers never
// observe a partially written or truncated file. The temp file is created
// with perm, so perm is applied to the final file on every call regardless
// of the target's previous permissions.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once the rename below succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return fmt.Errorf("setting permissions on temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming temp file into place: %w", err)
	}
	return nil
}

// Lock acquires an exclusive, cross-process advisory lock for path by
// creating a "<path>.lock" sidecar file, retrying with backoff until
// timeout elapses. The returned func releases the lock and must always be
// called (typically via defer).
//
// A lock file older than staleAfter is treated as abandoned (left behind by
// a killed process) and removed so it doesn't block callers forever.
func Lock(path string, timeout time.Duration) (func(), error) {
	lockPath := path + ".lock"
	deadline := time.Now().Add(timeout)

	for {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			f.Close()
			return func() { os.Remove(lockPath) }, nil
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("acquiring lock %s: %w", lockPath, err)
		}

		if info, statErr := os.Stat(lockPath); statErr == nil && time.Since(info.ModTime()) > staleAfter {
			os.Remove(lockPath)
			continue
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for lock %s", lockPath)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
