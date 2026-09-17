//go:build windows

package output

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

const lockAllBytes = ^uint32(0)

func tryLockFile(file *os.File) error {
	overlapped := new(windows.Overlapped)
	err := windows.LockFileEx(
		windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		lockAllBytes,
		lockAllBytes,
		overlapped,
	)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return errLockBusy
	}
	return err
}

func unlockFile(file *os.File) error {
	return windows.UnlockFileEx(windows.Handle(file.Fd()), 0, lockAllBytes, lockAllBytes, new(windows.Overlapped))
}
