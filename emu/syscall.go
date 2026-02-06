// Package emu provides functional ARM64 emulation.
package emu

import "io"

// ARM64 Linux syscall numbers.
const (
	SyscallRead  uint64 = 63 // read(fd, buf, count)
	SyscallWrite uint64 = 64 // write(fd, buf, count)
	SyscallExit  uint64 = 93 // exit(status)
)

// Linux error codes.
const (
	EBADF  = 9  // Bad file descriptor
	ENOSYS = 38 // Function not implemented
	EIO    = 5  // I/O error
)

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
	stdin   io.Reader
	stdout  io.Writer
	stderr  io.Writer
}

// NewDefaultSyscallHandler creates a default syscall handler.
func NewDefaultSyscallHandler(regFile *RegFile, memory *Memory, stdout, stderr io.Writer) *DefaultSyscallHandler {
	return &DefaultSyscallHandler{
		regFile: regFile,
		memory:  memory,
		stdin:   nil,
		stdout:  stdout,
		stderr:  stderr,
	}
}

// SetStdin sets the stdin reader for the syscall handler.
func (h *DefaultSyscallHandler) SetStdin(stdin io.Reader) {
	h.stdin = stdin
}

// Handle executes the syscall indicated by the register file state.
func (h *DefaultSyscallHandler) Handle() SyscallResult {
	syscallNum := h.regFile.ReadReg(8)

	switch syscallNum {
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
