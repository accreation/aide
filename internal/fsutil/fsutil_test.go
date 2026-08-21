package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestWriteFileAtomicCreatesFileWithPerm(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "data.json")

	if err := WriteFileAtomic(p, []byte(`{"a":1}`), 0600); err != nil {
		t.Fatalf("WriteFileAtomic failed: %v", err)
	}

	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if string(data) != `{"a":1}` {
		t.Errorf("unexpected contents: %q", data)
	}

	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected mode 0600, got %o", info.Mode().Perm())
	}
}

func TestWriteFileAtomicReassertsPermOnOverwrite(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "data.json")

	if err := os.WriteFile(p, []byte("old"), 0644); err != nil {
		t.Fatalf("seeding file: %v", err)
	}

	if err := WriteFileAtomic(p, []byte("new"), 0600); err != nil {
		t.Fatalf("WriteFileAtomic failed: %v", err)
	}

	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected perm to be reasserted to 0600, got %o", info.Mode().Perm())
	}

	data, _ := os.ReadFile(p)
	if string(data) != "new" {
		t.Errorf("expected contents 'new', got %q", data)
	}
}

func TestWriteFileAtomicLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "data.json")

	if err := WriteFileAtomic(p, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFileAtomic failed: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 file in dir, got %d: %v", len(entries), entries)
	}
}

func TestLockExcludesConcurrentCallers(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "data.json")

	unlock, err := Lock(p, time.Second)
	if err != nil {
		t.Fatalf("first Lock failed: %v", err)
	}

	// A second Lock call should not succeed until the first is released.
	done := make(chan struct{})
	go func() {
		unlock2, err := Lock(p, 200*time.Millisecond)
		if err == nil {
			t.Errorf("expected second Lock to time out while first is held")
			unlock2()
		}
		close(done)
	}()
	<-done

	unlock()

	unlock3, err := Lock(p, time.Second)
	if err != nil {
		t.Fatalf("Lock after release failed: %v", err)
	}
	unlock3()
}

func TestLockRemovesStaleLockFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "data.json")
	lockPath := p + ".lock"

	if err := os.WriteFile(lockPath, nil, 0600); err != nil {
		t.Fatalf("seeding stale lock: %v", err)
	}
	stale := time.Now().Add(-2 * staleAfter)
	if err := os.Chtimes(lockPath, stale, stale); err != nil {
		t.Fatalf("backdating lock mtime: %v", err)
	}

	unlock, err := Lock(p, time.Second)
	if err != nil {
		t.Fatalf("expected stale lock to be reclaimed, got error: %v", err)
	}
	unlock()
}

func TestLockSerializesConcurrentIncrement(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "counter")
	if err := os.WriteFile(p, []byte("0"), 0644); err != nil {
		t.Fatalf("seeding counter: %v", err)
	}

	const n = 20
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock, err := Lock(p, 5*time.Second)
			if err != nil {
				t.Errorf("Lock failed: %v", err)
				return
			}
			defer unlock()

			data, err := os.ReadFile(p)
			if err != nil {
				t.Errorf("read failed: %v", err)
				return
			}
			var v int
			for _, c := range data {
				v = v*10 + int(c-'0')
			}
			v++
			if err := WriteFileAtomic(p, []byte(itoa(v)), 0644); err != nil {
				t.Errorf("write failed: %v", err)
			}
		}()
	}
	wg.Wait()

	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("final read failed: %v", err)
	}
	var got int
	for _, c := range data {
		got = got*10 + int(c-'0')
	}
	if got != n {
		t.Errorf("expected counter %d after %d locked increments, got %d (lost updates)", n, n, got)
	}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var digits []byte
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	return string(digits)
}

func setHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

func TestAideDirCreates0700(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not meaningful on Windows")
	}
	home := t.TempDir()
	setHome(t, home)

	dir, err := AideDir()
	if err != nil {
		t.Fatalf("AideDir failed: %v", err)
	}
	if want := filepath.Join(home, ".aide"); dir != want {
		t.Errorf("expected %s, got %s", want, dir)
	}
	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0700 {
		t.Errorf("expected mode 0700 on a freshly created ~/.aide, got %04o", perm)
	}
}

// An ~/.aide created by an older aide (or by any earlier umask) is 0755;
// AideDir must re-tighten it, since os.MkdirAll leaves existing dirs alone.
func TestAideDirRetightensExistingDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not meaningful on Windows")
	}
	home := t.TempDir()
	setHome(t, home)

	loose := filepath.Join(home, ".aide")
	if err := os.MkdirAll(loose, 0755); err != nil {
		t.Fatalf("seeding a 0755 dir failed: %v", err)
	}
	if err := os.Chmod(loose, 0755); err != nil {
		t.Fatalf("chmod failed: %v", err)
	}

	if _, err := AideDir(); err != nil {
		t.Fatalf("AideDir failed: %v", err)
	}
	fi, err := os.Stat(loose)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0700 {
		t.Errorf("expected an existing 0755 ~/.aide to be re-tightened to 0700, got %04o", perm)
	}
}
