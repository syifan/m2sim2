// Package emu provides functional ARM64 emulation.
package emu

import (
	"os"
	"sync"
	"time"
)

// FileDescriptor represents an open file descriptor.
type FileDescriptor struct {
	HostFile *os.File // Host file handle (nil for closed or special FDs)
	Path     string   // Original path (empty for stdin/stdout/stderr)
	Flags    int      // Open flags
	IsOpen   bool     // Whether the FD is currently open
}

// FDTable manages file descriptors for syscall emulation.
type FDTable struct {
	fds    map[uint64]*FileDescriptor
	nextFD uint64
	mu     sync.Mutex
}

// NewFDTable creates a new file descriptor table with standard streams initialized.
func NewFDTable() *FDTable {
	t := &FDTable{
		fds:    make(map[uint64]*FileDescriptor),
		nextFD: 3, // Start allocating at FD 3
	}

	// Initialize standard streams (FDs 0, 1, 2)
	// These are special - they don't have host files but are marked as open
	t.fds[0] = &FileDescriptor{Path: "stdin", IsOpen: true}
	t.fds[1] = &FileDescriptor{Path: "stdout", IsOpen: true}
	t.fds[2] = &FileDescriptor{Path: "stderr", IsOpen: true}

	return t
}

// Open opens a file and returns a new file descriptor.
func (t *FDTable) Open(path string, flags int, mode os.FileMode) (uint64, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Open the file on the host
	hostFile, err := os.OpenFile(path, flags, mode)
	if err != nil {
		return 0, err
	}

	// Allocate new FD
	fd := t.nextFD
	t.nextFD++

	t.fds[fd] = &FileDescriptor{
		HostFile: hostFile,
		Path:     path,
		Flags:    flags,
		IsOpen:   true,
	}

	return fd, nil
}

// Close closes a file descriptor.
func (t *FDTable) Close(fd uint64) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	entry, exists := t.fds[fd]
	if !exists || !entry.IsOpen {
		return os.ErrInvalid
	}

	// Special handling for stdin/stdout/stderr
	if fd <= 2 {
		// Mark as closed but don't actually close anything
		entry.IsOpen = false
		return nil
	}

	// Close the host file
	if entry.HostFile != nil {
		err := entry.HostFile.Close()
		if err != nil {
			return err
		}
	}

	entry.HostFile = nil
	entry.IsOpen = false

	return nil
}

// Get returns the file descriptor entry if it exists and is open.
func (t *FDTable) Get(fd uint64) (*FileDescriptor, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	entry, exists := t.fds[fd]
	if !exists || !entry.IsOpen {
		return nil, false
	}

	return entry, true
}

// IsOpen checks if a file descriptor is open.
func (t *FDTable) IsOpen(fd uint64) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	entry, exists := t.fds[fd]
	return exists && entry.IsOpen
}

// Read reads from a file descriptor into a buffer.
func (t *FDTable) Read(fd uint64, buf []byte) (int, error) {
	t.mu.Lock()
	entry, exists := t.fds[fd]
	if !exists || !entry.IsOpen {
		t.mu.Unlock()
		return 0, os.ErrInvalid
	}

	hostFile := entry.HostFile
	t.mu.Unlock()

	// stdin is handled separately by the syscall handler
	if fd == 0 {
		return 0, os.ErrInvalid
	}

	if hostFile == nil {
		return 0, os.ErrInvalid
	}

	return hostFile.Read(buf)
}

// Write writes a buffer to a file descriptor.
func (t *FDTable) Write(fd uint64, buf []byte) (int, error) {
	t.mu.Lock()
	entry, exists := t.fds[fd]
	if !exists || !entry.IsOpen {
		t.mu.Unlock()
		return 0, os.ErrInvalid
	}

	hostFile := entry.HostFile
	t.mu.Unlock()

	// stdout/stderr are handled separately by the syscall handler
	if fd <= 2 {
		return 0, os.ErrInvalid
	}

	if hostFile == nil {
		return 0, os.ErrInvalid
	}

	return hostFile.Write(buf)
}

// Stat returns file information for a file descriptor.
func (t *FDTable) Stat(fd uint64) (os.FileInfo, error) {
	t.mu.Lock()
	entry, exists := t.fds[fd]
	if !exists || !entry.IsOpen {
		t.mu.Unlock()
		return nil, os.ErrInvalid
	}

	hostFile := entry.HostFile
	t.mu.Unlock()

	// stdin/stdout/stderr return a stub FileInfo
	if fd <= 2 {
		return &stdioFileInfo{name: entry.Path, isCharDevice: true}, nil
	}

	if hostFile == nil {
		return nil, os.ErrInvalid
	}

	return hostFile.Stat()
}

// Seek sets the file position for the given file descriptor.
func (t *FDTable) Seek(fd uint64, offset int64, whence int) (int64, error) {
	t.mu.Lock()
	entry, exists := t.fds[fd]
	if !exists || !entry.IsOpen {
		t.mu.Unlock()
		return 0, os.ErrInvalid
	}

	hostFile := entry.HostFile
	t.mu.Unlock()

	// stdin/stdout/stderr can't be seeked
	if fd <= 2 {
		return 0, os.ErrInvalid
	}

	if hostFile == nil {
		return 0, os.ErrInvalid
	}

	return hostFile.Seek(offset, whence)
}

// stdioFileInfo is a stub FileInfo for stdin/stdout/stderr.
type stdioFileInfo struct {
	name         string
	isCharDevice bool
}

func (f *stdioFileInfo) Name() string       { return f.name }
func (f *stdioFileInfo) Size() int64        { return 0 }
func (f *stdioFileInfo) Mode() os.FileMode  { return os.ModeCharDevice | 0666 }
func (f *stdioFileInfo) ModTime() time.Time { return time.Time{} }
func (f *stdioFileInfo) IsDir() bool        { return false }
func (f *stdioFileInfo) Sys() interface{}   { return nil }
