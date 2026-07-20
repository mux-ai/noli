//go:build !linux

package targetlock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Acquire uses atomic directory creation on platforms where the standard
// library does not expose advisory file locking. A crash can leave the lock
// directory behind; that fails closed and requires manual removal rather than
// risking concurrent replacement.
func Acquire(path string) (*Lock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create lock parent: %w", err)
	}
	lockDir := path + ".d"
	if err := os.Mkdir(lockDir, 0o700); err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("%s: %w", path, ErrBusy)
		}
		return nil, fmt.Errorf("lock target: %w", err)
	}
	return newLock(func() error {
		if err := os.Remove(lockDir); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("unlock target: %w", err)
		}
		return nil
	}), nil
}
