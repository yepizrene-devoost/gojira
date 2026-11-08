package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ErrWriteLocked reports that another process holds the configuration write
// lock and did not release it within lockWaitTimeout. Callers can match it
// with errors.Is to tell a concurrent write apart from other failures.
var ErrWriteLocked = errors.New("configuration is being written by another process")

// lockWaitTimeout is how long Update waits for the configuration lock before
// giving up with an error wrapping ErrWriteLocked.
var lockWaitTimeout = 5 * time.Second

// lockStaleAfter is the age after which an existing lock file is considered
// abandoned by a crashed writer and is reclaimed instead of waited on.
var lockStaleAfter = 30 * time.Second

// lockRetryInterval is the delay between attempts to acquire the lock.
const lockRetryInterval = 10 * time.Millisecond

// lockFileName is the lock file that serializes configuration writers inside
// the configuration directory.
const lockFileName = "config.yaml.lock"

// Update is the single serialized entry point for every configuration writer.
// It creates the configuration directory if needed, takes an exclusive lock,
// loads the current configuration from disk, applies mutate to that freshly
// loaded state, and saves only when mutate returns nil; the lock is always
// released. A concurrent caller waits for the lock, so its mutation is applied
// to the state written by the previous one and no write is lost.
func Update(mutate func(*Config) error) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	unlock, err := acquireLock(dir)
	if err != nil {
		return err
	}
	defer unlock()

	cfg, err := Load()
	if err != nil {
		return err
	}
	if err := mutate(cfg); err != nil {
		return err
	}
	return cfg.Save()
}

// lockFilePath returns the lock file path inside the given configuration
// directory. The path is built from the directory only, so Update never
// follows or creates a symlinked config directory entry by accident.
func lockFilePath(dir string) string {
	return filepath.Join(dir, lockFileName)
}

// acquireLock creates the lock file exclusively, retrying every
// lockRetryInterval until lockWaitTimeout elapses. A lock file whose
// modification time is older than lockStaleAfter is removed and the attempt
// retried, so a crashed writer cannot block configuration forever. On success
// it returns a function that releases the lock by removing the file.
func acquireLock(dir string) (func(), error) {
	path := lockFilePath(dir)
	start := time.Now()

	for {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			if closeErr := file.Close(); closeErr != nil {
				_ = os.Remove(path)
				return nil, closeErr
			}
			return func() { _ = os.Remove(path) }, nil
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("creating lock file %s: %w", path, err)
		}

		if time.Since(start) >= lockWaitTimeout {
			return nil, fmt.Errorf("%w (remove %s if no other writer is running)", ErrWriteLocked, path)
		}
		// Lstat (not Stat) so an existing symlink is inspected, never
		// followed; Remove deletes the link itself, not its target.
		if info, statErr := os.Lstat(path); statErr == nil && time.Since(info.ModTime()) > lockStaleAfter {
			if removeErr := os.Remove(path); removeErr == nil {
				continue
			}
		}
		time.Sleep(lockRetryInterval)
	}
}
