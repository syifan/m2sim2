// Package emu provides functional ARM64 emulation.
package emu

import (
	"io"
	"os"
)

// ARM64 Linux syscall numbers.
const (
	SyscallOpenat uint64 = 56 // openat(dirfd, pathname, flags, mode)
	SyscallClose  uint64 = 57 // close(fd)
	SyscallRead   uint64 = 63 // read(fd, buf, count)
	SyscallWrite  uint64 = 64 // write(fd, buf, count)
	SyscallExit   uint64 = 93 // exit(status)
)

// Linux error codes.
const (
	ENOENT = 2  // No such file or directory
	EIO    = 5  // I/O error
	EBADF  = 9  // Bad file descriptor
	EACCES = 13 // Permission denied
	ENOSYS = 38 // Function not implemented
)

// Linux open flags.
const (
	O_RDONLY = 0
	O_WRONLY = 1
	O_RDWR   = 2
	O_CREAT  = 0x40
	O_TRUNC  = 0x200
	O_APPEND = 0x400
)

// AT_FDCWD indicates relative to current working directory.
const AT_FDCWD int64 = -100

// AT_FDCWD_U64 is AT_FDCWD as unsigned 64-bit (for register comparison).
// This is the two's complement representation of -100 in uint64.
const AT_FDCWD_U64 uint64 = 0xFFFFFFFFFFFFFF9C

// SyscallResult represents the result of a syscall execution.
type SyscallResult struct {
	// Exited is true if the syscall caused program termination.
	Exited bool

	// ExitCode is the exit status if Exited is true.
	ExitCode int64
}

// SyscallHandler is the interface for handling ARM64 syscalls.
type SyscallHandler interface {
	// Handle executes the syscall indicated by the register file state.
	// ARM64 Linux syscall convention:
	//   - Syscall number in X8
	//   - Arguments in X0-X5
	//   - Return value in X0
	Handle() SyscallResult
}

// DefaultSyscallHandler provides a basic syscall handler implementation.
type DefaultSyscallHandler struct {
	regFile *RegFile
	memory  *Memory
	fdTable *FDTable
	stdin   io.Reader
	stdout  io.Writer
	stderr  io.Writer
}

// NewDefaultSyscallHandler creates a default syscall handler.
func NewDefaultSyscallHandler(regFile *RegFile, memory *Memory, stdout, stderr io.Writer) *DefaultSyscallHandler {
	return &DefaultSyscallHandler{
		regFile: regFile,
		memory:  memory,
		fdTable: NewFDTable(),
		stdin:   nil,
		stdout:  stdout,
		stderr:  stderr,
	}
}

// SetFDTable sets a custom file descriptor table for the syscall handler.
func (h *DefaultSyscallHandler) SetFDTable(fdTable *FDTable) {
	h.fdTable = fdTable
}

// GetFDTable returns the file descriptor table used by the syscall handler.
func (h *DefaultSyscallHandler) GetFDTable() *FDTable {
	return h.fdTable
}

// SetStdin sets the stdin reader for the syscall handler.
func (h *DefaultSyscallHandler) SetStdin(stdin io.Reader) {
	h.stdin = stdin
}

// Handle executes the syscall indicated by the register file state.
func (h *DefaultSyscallHandler) Handle() SyscallResult {
	syscallNum := h.regFile.ReadReg(8)

	switch syscallNum {
	case SyscallOpenat:
		return h.handleOpenat()
	case SyscallClose:
		return h.handleClose()
	case SyscallRead:
		return h.handleRead()
	case SyscallWrite:
		return h.handleWrite()
	case SyscallExit:
		return h.handleExit()
	default:
		return h.handleUnknown()
	}
}

// handleExit handles the exit syscall (93).
func (h *DefaultSyscallHandler) handleExit() SyscallResult {
	exitCode := int64(h.regFile.ReadReg(0))
	return SyscallResult{
		Exited:   true,
		ExitCode: exitCode,
	}
}

// handleRead handles the read syscall (63).
func (h *DefaultSyscallHandler) handleRead() SyscallResult {
	fd := h.regFile.ReadReg(0)
	bufPtr := h.regFile.ReadReg(1)
	count := h.regFile.ReadReg(2)

	// Only stdin (fd=0) is supported for now
	if fd != 0 {
		h.setError(EBADF)
		return SyscallResult{}
	}

	// If no stdin is configured, return EOF
	if h.stdin == nil {
		h.regFile.WriteReg(0, 0)
		return SyscallResult{}
	}

	// Read from stdin
	buf := make([]byte, count)
	n, err := h.stdin.Read(buf)
	if err != nil && n == 0 {
		// EOF or error with no bytes read
		h.regFile.WriteReg(0, 0)
		return SyscallResult{}
	}

	// Write to memory
	for i := 0; i < n; i++ {
		h.memory.Write8(bufPtr+uint64(i), buf[i])
	}

	// Return bytes read
	h.regFile.WriteReg(0, uint64(n))
	return SyscallResult{}
}

// handleWrite handles the write syscall (64).
func (h *DefaultSyscallHandler) handleWrite() SyscallResult {
	fd := h.regFile.ReadReg(0)
	bufPtr := h.regFile.ReadReg(1)
	count := h.regFile.ReadReg(2)

	// Select output based on file descriptor
	var writer io.Writer
	switch fd {
	case 1:
		writer = h.stdout
	case 2:
		writer = h.stderr
	default:
		h.setError(EBADF)
		return SyscallResult{}
	}

	// Read buffer from memory
	buf := make([]byte, count)
	for i := uint64(0); i < count; i++ {
		buf[i] = h.memory.Read8(bufPtr + i)
	}

	// Write to output
	n, err := writer.Write(buf)
	if err != nil {
		h.setError(EIO)
		return SyscallResult{}
	}

	// Return bytes written
	h.regFile.WriteReg(0, uint64(n))
	return SyscallResult{}
}

// handleUnknown handles unrecognized syscalls.
func (h *DefaultSyscallHandler) handleUnknown() SyscallResult {
	h.setError(ENOSYS)
	return SyscallResult{}
}

// setError sets X0 to -errno (as two's complement).
func (h *DefaultSyscallHandler) setError(errno int) {
	h.regFile.WriteReg(0, uint64(-int64(errno)))
}

// handleClose handles the close syscall (57).
func (h *DefaultSyscallHandler) handleClose() SyscallResult {
	fd := h.regFile.ReadReg(0)

	err := h.fdTable.Close(fd)
	if err != nil {
		h.setError(EBADF)
		return SyscallResult{}
	}

	// Return 0 on success
	h.regFile.WriteReg(0, 0)
	return SyscallResult{}
}

// handleOpenat handles the openat syscall (56).
func (h *DefaultSyscallHandler) handleOpenat() SyscallResult {
	dirfd := int64(h.regFile.ReadReg(0))
	pathnamePtr := h.regFile.ReadReg(1)
	flags := int(h.regFile.ReadReg(2))
	mode := os.FileMode(h.regFile.ReadReg(3))

	// Read pathname from memory (null-terminated)
	pathname := h.readString(pathnamePtr)

	// Only support AT_FDCWD for now (relative to current directory)
	if dirfd != AT_FDCWD {
		h.setError(EBADF)
		return SyscallResult{}
	}

	// Convert Linux flags to Go flags
	goFlags := h.linuxToGoFlags(flags)

	// Open the file
	fd, err := h.fdTable.Open(pathname, goFlags, mode)
	if err != nil {
		if os.IsNotExist(err) {
			h.setError(ENOENT)
		} else if os.IsPermission(err) {
			h.setError(EACCES)
		} else {
			h.setError(EIO)
		}
		return SyscallResult{}
	}

	// Return the new file descriptor
	h.regFile.WriteReg(0, fd)
	return SyscallResult{}
}

// readString reads a null-terminated string from memory.
func (h *DefaultSyscallHandler) readString(addr uint64) string {
	var buf []byte
	for {
		b := h.memory.Read8(addr)
		if b == 0 {
			break
		}
		buf = append(buf, b)
		addr++
	}
	return string(buf)
}

// linuxToGoFlags converts Linux open flags to Go os.OpenFile flags.
func (h *DefaultSyscallHandler) linuxToGoFlags(linuxFlags int) int {
	var goFlags int

	// Access mode (lower 2 bits)
	switch linuxFlags & 3 {
	case O_RDONLY:
		goFlags = os.O_RDONLY
	case O_WRONLY:
		goFlags = os.O_WRONLY
	case O_RDWR:
		goFlags = os.O_RDWR
	}

	// Additional flags
	if linuxFlags&O_CREAT != 0 {
		goFlags |= os.O_CREATE
	}
	if linuxFlags&O_TRUNC != 0 {
		goFlags |= os.O_TRUNC
	}
	if linuxFlags&O_APPEND != 0 {
		goFlags |= os.O_APPEND
	}

	return goFlags
}
