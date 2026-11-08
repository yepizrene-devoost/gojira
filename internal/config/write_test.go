package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestUpdateSerializesConcurrentWriters (rule a): while goroutine A is inside
// its mutation, goroutine B's Update must not complete, and after both run,
// both mutations must be present in the loaded config.
func TestUpdateSerializesConcurrentWriters(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	insideA := make(chan struct{})
	releaseA := make(chan struct{})
	doneA := make(chan error, 1)
	doneB := make(chan error, 1)

	go func() {
		doneA <- Update(func(cfg *Config) error {
			cfg.Email = "a@example.com"
			close(insideA)
			<-releaseA
			return nil
		})
	}()

	select {
	case <-insideA:
	case err := <-doneA:
		t.Fatalf("Update A returned before entering mutate: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("Update A never entered its mutate function")
	}

	go func() {
		doneB <- Update(func(cfg *Config) error {
			cfg.Domain = "b.example"
			return nil
		})
	}()

	select {
	case err := <-doneB:
		t.Fatalf("Update B completed while A still held the mutation (lost write): %v", err)
	case <-time.After(150 * time.Millisecond):
		// B is blocked as required.
	}

	close(releaseA)

	select {
	case err := <-doneA:
		if err != nil {
			t.Fatalf("Update A returned unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Update A did not finish after release")
	}
	select {
	case err := <-doneB:
		if err != nil {
			t.Fatalf("Update B returned unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Update B did not finish after A released the lock")
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}
	if loaded.Email != "a@example.com" {
		t.Fatalf("Email = %q, want a@example.com (A's write lost)", loaded.Email)
	}
	if loaded.Domain != "b.example" {
		t.Fatalf("Domain = %q, want b.example (B's write lost)", loaded.Domain)
	}
}

// TestUpdateFailsWithErrWriteLockedWhenLockIsHeld (rule b): a held lock file
// makes Update return an error wrapping ErrWriteLocked and leaves the config
// file unchanged.
func TestUpdateFailsWithErrWriteLockedWhenLockIsHeld(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	previous := lockWaitTimeout
	lockWaitTimeout = 50 * time.Millisecond
	t.Cleanup(func() { lockWaitTimeout = previous })

	dir, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir returned unexpected error: %v", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned unexpected error: %v", err)
	}

	existing := &Config{Domain: "original.example", Email: "keep@example.com"}
	if err := existing.Save(); err != nil {
		t.Fatalf("Save returned unexpected error: %v", err)
	}
	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath returned unexpected error: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned unexpected error: %v", err)
	}

	lockPath := filepath.Join(dir, "config.yaml.lock")
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("creating held lock: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(lockPath) })

	err = Update(func(cfg *Config) error {
		cfg.Domain = "changed.example"
		return nil
	})
	_ = lockFile.Close()
	if err == nil {
		t.Fatal("Update succeeded while the lock was held, want ErrWriteLocked")
	}
	if !errors.Is(err, ErrWriteLocked) {
		t.Fatalf("Update error = %v, want it to wrap ErrWriteLocked", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned unexpected error: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("config file changed under a held lock:\nbefore: %s\nafter:  %s", before, after)
	}
}

// TestUpdateReclaimsStaleLock (rule c): a lock file whose modification time is
// older than lockStaleAfter is reclaimed and Update succeeds.
func TestUpdateReclaimsStaleLock(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir returned unexpected error: %v", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned unexpected error: %v", err)
	}

	lockPath := filepath.Join(dir, "config.yaml.lock")
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("creating stale lock: %v", err)
	}
	if err := lockFile.Close(); err != nil {
		t.Fatalf("closing stale lock: %v", err)
	}
	stale := time.Now().Add(-lockStaleAfter - time.Minute)
	if err := os.Chtimes(lockPath, stale, stale); err != nil {
		t.Fatalf("aging the lock file: %v", err)
	}

	if err := Update(func(cfg *Config) error {
		cfg.Board = "RECOVERED"
		return nil
	}); err != nil {
		t.Fatalf("Update with a stale lock returned unexpected error: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}
	if loaded.Board != "RECOVERED" {
		t.Fatalf("Board = %q, want RECOVERED", loaded.Board)
	}
	if _, err := os.Lstat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("lock file still present after Update: Lstat error = %v", err)
	}
}

// TestUpdatePropagatesMutateErrorWithoutWriting (rule d): a mutate error is
// returned to the caller and the config file is not written.
func TestUpdatePropagatesMutateErrorWithoutWriting(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	existing := &Config{Domain: "original.example", Email: "keep@example.com"}
	if err := existing.Save(); err != nil {
		t.Fatalf("Save returned unexpected error: %v", err)
	}
	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath returned unexpected error: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned unexpected error: %v", err)
	}

	sentinel := errors.New("mutation rejected")
	err = Update(func(cfg *Config) error {
		cfg.Domain = "never-written.example"
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("Update error = %v, want it to wrap the mutate error", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned unexpected error: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("config file was written after a failed mutate:\nbefore: %s\nafter:  %s", before, after)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}
	if loaded.Domain != "original.example" {
		t.Fatalf("Domain = %q, want original.example unchanged", loaded.Domain)
	}
}
