// Package targetlock serializes filesystem replacements that target the same
// path. It is internal to the public pkg tree so generator and retrieval can
// share the locking contract without exposing it as SDK surface.
package targetlock

import (
	"errors"
	"sync"
)

// ErrBusy reports that another process currently owns the target lock.
var ErrBusy = errors.New("target is locked by another process")

// Lock is an acquired interprocess lock. Release is idempotent.
type Lock struct {
	once    sync.Once
	release func() error
	err     error
}

func newLock(release func() error) *Lock {
	return &Lock{release: release}
}

// Release relinquishes the lock. The lock file itself may remain so another
// process can safely lock the same inode without an unlink/recreate race.
func (l *Lock) Release() error {
	if l == nil {
		return nil
	}
	l.once.Do(func() {
		if l.release != nil {
			l.err = l.release()
		}
	})
	return l.err
}
