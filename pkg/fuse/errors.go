package fuse

import (
	"errors"
	"fmt"
)

type OldVersionError struct {
	Kernel     Protocol
	LibraryMin Protocol
}

func (e *OldVersionError) Error() string {
	return fmt.Sprintf("kernel FUSE version is too old: %v < %v", e.Kernel, e.LibraryMin)
}

var (
	ErrClosedWithoutInit = errors.New("fuse connection closed without init")
)

type bugKernelWriteError struct {
	Error string
	Stack string
}

func (b bugKernelWriteError) String() string {
	return fmt.Sprintf("kernel write error: error=%q stack=\n%s", b.Error, b.Stack)
}

// safe to call even with nil error
func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

type notCachedError struct{}

func (notCachedError) Error() string {
	return "node not cached"
}

var _ ErrorNumber = notCachedError{}

func (notCachedError) Errno() Errno {
	// Behave just like if the original syscall.ENOENT had been passed
	// straight through.
	return ENOENT
}

var (
	ErrNotCached = notCachedError{}
)
