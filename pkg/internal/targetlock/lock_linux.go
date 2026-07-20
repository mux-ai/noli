//go:build linux

package targetlock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// Acquire takes a non-blocking advisory lock for path. The open lock file is
// deliberately retained after Release: removing a flock file can let a third
// process lock a new inode while a waiter still references the old one.
func Acquire(path string) (*Lock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create lock parent: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open target lock: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, fmt.Errorf("%s: %w", path, ErrBusy)
		}
		return nil, fmt.Errorf("lock target: %w", err)
	}
	return newLock(func() error {
		unlockErr := syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		closeErr := file.Close()
		if unlockErr != nil {
			return fmt.Errorf("unlock target: %w", unlockErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close target lock: %w", closeErr)
		}
		return nil
	}), nil
}
