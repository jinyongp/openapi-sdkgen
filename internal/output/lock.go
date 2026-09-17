package output

import (
	"errors"
	"fmt"
	"os"
)

var errLockBusy = errors.New("advisory lock is already held")

type outputLock struct {
	file *os.File
}

func acquireOutputLock(output string) (*outputLock, error) {
	lockPath := output + ".openapi-sdkgen.lock"
	file, err := openOutputLockFile(lockPath, true)
	if err != nil {
		return nil, publicationFailure(fmt.Errorf("open incremental output lock %s: %w", lockPath, err))
	}
	if err := tryLockFile(file); err != nil {
		_ = file.Close()
		if errors.Is(err, errLockBusy) {
			return nil, publicationFailure(fmt.Errorf("incremental output %s is locked by another generation", output))
		}
		return nil, publicationFailure(fmt.Errorf("lock incremental output %s: %w", output, err))
	}
	return &outputLock{file: file}, nil
}

func checkExistingOutputLock(output, label string) error {
	lockPath := output + ".openapi-sdkgen.lock"
	file, err := openOutputLockFile(lockPath, false)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return publicationFailure(fmt.Errorf("inspect %s lock %s: %w", label, lockPath, err))
	}
	if err := tryLockFile(file); err != nil {
		_ = file.Close()
		if errors.Is(err, errLockBusy) {
			return publicationFailure(fmt.Errorf("%s %s is locked by another generation", label, output))
		}
		return publicationFailure(fmt.Errorf("inspect %s lock %s: %w", label, lockPath, err))
	}
	_ = unlockFile(file)
	if err := file.Close(); err != nil {
		return publicationFailure(fmt.Errorf("close %s lock %s: %w", label, lockPath, err))
	}
	return nil
}

func openOutputLockFile(path string, create bool) (*os.File, error) {
	flags := os.O_RDWR
	if create {
		flags |= os.O_CREATE
	}
	file, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		return nil, err
	}
	opened, statErr := file.Stat()
	pathInfo, lstatErr := os.Lstat(path)
	if statErr != nil || lstatErr != nil || !opened.Mode().IsRegular() || !pathInfo.Mode().IsRegular() || !os.SameFile(opened, pathInfo) {
		_ = file.Close()
		if statErr != nil {
			return nil, statErr
		}
		if lstatErr != nil {
			return nil, lstatErr
		}
		return nil, errors.New("lock path must be one regular file")
	}
	return file, nil
}

func (lock *outputLock) release() {
	if lock == nil || lock.file == nil {
		return
	}
	_ = unlockFile(lock.file)
	_ = lock.file.Close()
	lock.file = nil
}
