// Package emu provides functional ARM64 emulation.
package emu

import (
	"io"
	"os"
)

// ARM64 Linux syscall numbers.
const (
	SyscallOpenat uint64 = 56  // openat(dirfd, pathname, flags, mode)
	SyscallClose  uint64 = 57  // close(fd)
	SyscallLseek  uint64 = 62  // lseek(fd, offset, whence)
	SyscallRead   uint64 = 63  // read(fd, buf, count)
	SyscallWrite  uint64 = 64  // write(fd, buf, count)
	SyscallFstat  uint64 = 80  // fstat(fd, statbuf)
	SyscallExit   uint64 = 93  // exit(status)
	SyscallBrk    uint64 = 214 // brk(addr)
	SyscallMmap   uint64 = 222 // mmap(addr, length, prot, flags, fd, offset)
)

// Linux error codes.
const (
	ENOENT = 2  // No such file or directory
	EIO    = 5  // I/O error
	EBADF  = 9  // Bad file descriptor
	ENOMEM = 12 // Out of memory
	EACCES = 13 // Permission denied
	EINVAL = 22 // Invalid argument
	ESPIPE = 29 // Illegal seek (on pipes/sockets)
	ENOSYS = 38 // Function not implemented
)

// Linux mmap protection flags.
const (
	PROT_NONE  = 0x0
	PROT_READ  = 0x1
	PROT_WRITE = 0x2
	PROT_EXEC  = 0x4
)

// Linux mmap flags.
const (
	MAP_SHARED    = 0x1
	MAP_PRIVATE   = 0x2
	MAP_FIXED     = 0x10
	MAP_ANONYMOUS = 0x20
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

// Lseek whence constants.
const (
	SEEK_SET = 0 // Seek from beginning of file
	SEEK_CUR = 1 // Seek from current position
	SEEK_END = 2 // Seek from end of file
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

// MmapRegion represents a mapped memory region.
type MmapRegion struct {
	Addr   uint64 // Start address
	Length uint64 // Length in bytes
	Prot   int    // Protection flags
	Flags  int    // Mapping flags
}

// DefaultSyscallHandler provides a basic syscall handler implementation.
type DefaultSyscallHandler struct {
	regFile      *RegFile
	memory       *Memory
	fdTable      *FDTable
	stdin        io.Reader
	stdout       io.Writer
	stderr       io.Writer
	programBreak uint64       // Current program break (heap end)
	nextMmapAddr uint64       // Next address for anonymous mmap
	mmapRegions  []MmapRegion // Tracked mmap regions
}

// DefaultProgramBreak is the initial program break address.
// This is set to a reasonable default for the heap start.
const DefaultProgramBreak uint64 = 0x10000000 // 256MB mark

// DefaultMmapBase is the starting address for anonymous mmap allocations.
// This is placed well above the heap to avoid collisions.
const DefaultMmapBase uint64 = 0x40000000 // 1GB mark

// NewDefaultSyscallHandler creates a default syscall handler.
func NewDefaultSyscallHandler(regFile *RegFile, memory *Memory, stdout, stderr io.Writer) *DefaultSyscallHandler {
	return &DefaultSyscallHandler{
		regFile:      regFile,
		memory:       memory,
		fdTable:      NewFDTable(),
		stdin:        nil,
		stdout:       stdout,
		stderr:       stderr,
		programBreak: DefaultProgramBreak,
		nextMmapAddr: DefaultMmapBase,
		mmapRegions:  make([]MmapRegion, 0),
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

// GetProgramBreak returns the current program break.
func (h *DefaultSyscallHandler) GetProgramBreak() uint64 {
	return h.programBreak
}

// SetProgramBreak sets the program break to a specific address.
func (h *DefaultSyscallHandler) SetProgramBreak(addr uint64) {
	h.programBreak = addr
}

// Handle executes the syscall indicated by the register file state.
func (h *DefaultSyscallHandler) Handle() SyscallResult {
	syscallNum := h.regFile.ReadReg(8)

	switch syscallNum {
	case SyscallOpenat:
		return h.handleOpenat()
	case SyscallClose:
		return h.handleClose()
	case SyscallLseek:
		return h.handleLseek()
	case SyscallRead:
		return h.handleRead()
	case SyscallWrite:
		return h.handleWrite()
	case SyscallFstat:
		return h.handleFstat()
	case SyscallExit:
		return h.handleExit()
	case SyscallBrk:
		return h.handleBrk()
	case SyscallMmap:
		return h.handleMmap()
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

	var n int
	var err error
	buf := make([]byte, count)

	if fd == 0 {
		// stdin: use configured stdin reader
		if h.stdin == nil {
			h.regFile.WriteReg(0, 0) // EOF
			return SyscallResult{}
		}
		n, err = h.stdin.Read(buf)
	} else {
		// File descriptor: use FDTable
		n, err = h.fdTable.Read(fd, buf)
	}

	if err != nil && n == 0 {
		if err == os.ErrInvalid {
			h.setError(EBADF)
		} else {
			// EOF or other error with no bytes read
			h.regFile.WriteReg(0, 0)
		}
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

	// Read buffer from memory
	buf := make([]byte, count)
	for i := uint64(0); i < count; i++ {
		buf[i] = h.memory.Read8(bufPtr + i)
	}

	var n int
	var err error

	switch fd {
	case 1:
		// stdout
		n, err = h.stdout.Write(buf)
	case 2:
		// stderr
		n, err = h.stderr.Write(buf)
	default:
		// File descriptor: use FDTable
		n, err = h.fdTable.Write(fd, buf)
	}

	if err != nil {
		if err == os.ErrInvalid {
			h.setError(EBADF)
		} else {
			h.setError(EIO)
		}
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

// handleBrk handles the brk syscall (214).
// brk manages the program break (end of heap).
// - addr == 0: query current program break
// - addr < current: no change, return current break
// - addr > current: extend heap, return new break
func (h *DefaultSyscallHandler) handleBrk() SyscallResult {
	addr := h.regFile.ReadReg(0)

	// If addr is 0 or less than current break, just return current break
	if addr == 0 || addr < h.programBreak {
		h.regFile.WriteReg(0, h.programBreak)
		return SyscallResult{}
	}

	// Extend the program break
	h.programBreak = addr
	h.regFile.WriteReg(0, h.programBreak)
	return SyscallResult{}
}

// handleMmap handles the mmap syscall (222).
// mmap maps memory regions. Currently only supports anonymous mappings.
// Arguments:
//   - X0: addr (hint address, or 0 for kernel to choose)
//   - X1: length (size of mapping)
//   - X2: prot (protection flags)
//   - X3: flags (mapping flags)
//   - X4: fd (file descriptor, -1 for anonymous)
//   - X5: offset (offset in file)
func (h *DefaultSyscallHandler) handleMmap() SyscallResult {
	addr := h.regFile.ReadReg(0)
	length := h.regFile.ReadReg(1)
	prot := int(h.regFile.ReadReg(2))
	flags := int(h.regFile.ReadReg(3))
	fd := int64(h.regFile.ReadReg(4))
	// offset := h.regFile.ReadReg(5) // Not used for anonymous mappings

	// Validate length
	if length == 0 {
		h.setError(EINVAL)
		return SyscallResult{}
	}

	// Check if anonymous mapping
	isAnonymous := (flags & MAP_ANONYMOUS) != 0

	// For now, only support anonymous mappings
	// fd should be -1 for anonymous mappings
	if !isAnonymous || (fd != -1 && !isAnonymous) {
		h.setError(ENOSYS) // File mappings not implemented
		return SyscallResult{}
	}

	// Page-align the length (4KB pages)
	const pageSize uint64 = 4096
	alignedLength := (length + pageSize - 1) & ^(pageSize - 1)

	var mappedAddr uint64

	// Handle MAP_FIXED
	if flags&MAP_FIXED != 0 {
		if addr == 0 {
			h.setError(EINVAL)
			return SyscallResult{}
		}
		// Use the requested address (page-aligned)
		mappedAddr = addr & ^(pageSize - 1)
	} else {
		// Allocate from next available mmap address
		mappedAddr = h.nextMmapAddr
		h.nextMmapAddr += alignedLength
	}

	// Track the mapping
	region := MmapRegion{
		Addr:   mappedAddr,
		Length: alignedLength,
		Prot:   prot,
		Flags:  flags,
	}
	h.mmapRegions = append(h.mmapRegions, region)

	// Return the mapped address
	h.regFile.WriteReg(0, mappedAddr)
	return SyscallResult{}
}

// GetMmapRegions returns the list of mmap'd regions.
func (h *DefaultSyscallHandler) GetMmapRegions() []MmapRegion {
	return h.mmapRegions
}

// handleLseek handles the lseek syscall (62).
// lseek repositions the file offset of an open file descriptor.
func (h *DefaultSyscallHandler) handleLseek() SyscallResult {
	fd := h.regFile.ReadReg(0)
	offset := int64(h.regFile.ReadReg(1))
	whence := int(h.regFile.ReadReg(2))

	// Check for standard streams (stdin/stdout/stderr) - they can't be seeked
	if fd <= 2 {
		h.setError(ESPIPE)
		return SyscallResult{}
	}

	// Validate whence
	if whence < SEEK_SET || whence > SEEK_END {
		h.setError(EINVAL)
		return SyscallResult{}
	}

	// Perform the seek
	newPos, err := h.fdTable.Seek(fd, offset, whence)
	if err != nil {
		h.setError(EBADF)
		return SyscallResult{}
	}

	// Return the new file position
	h.regFile.WriteReg(0, uint64(newPos))
	return SyscallResult{}
}

// ARM64 Linux stat structure offsets (128 bytes total).
const (
	statOffsetDev     = 0   // uint64
	statOffsetIno     = 8   // uint64
	statOffsetMode    = 16  // uint32
	statOffsetNlink   = 20  // uint32
	statOffsetUID     = 24  // uint32
	statOffsetGID     = 28  // uint32
	statOffsetRdev    = 32  // uint64
	statOffsetPad1    = 40  // uint64
	statOffsetSize    = 48  // int64
	statOffsetBlksize = 56  // int32
	statOffsetPad2    = 60  // int32
	statOffsetBlocks  = 64  // int64
	statOffsetAtime   = 72  // int64 (seconds)
	statOffsetAtimeNs = 80  // int64 (nanoseconds)
	statOffsetMtime   = 88  // int64 (seconds)
	statOffsetMtimeNs = 96  // int64 (nanoseconds)
	statOffsetCtime   = 104 // int64 (seconds)
	statOffsetCtimeNs = 112 // int64 (nanoseconds)
	statSize          = 128
)

// handleFstat handles the fstat syscall (80).
// fstat gets file status for an open file descriptor.
func (h *DefaultSyscallHandler) handleFstat() SyscallResult {
	fd := h.regFile.ReadReg(0)
	statbufPtr := h.regFile.ReadReg(1)

	// Get file info from FDTable
	info, err := h.fdTable.Stat(fd)
	if err != nil {
		h.setError(EBADF)
		return SyscallResult{}
	}

	// Write stat structure to memory
	h.writeStatToMemory(statbufPtr, info)

	// Return 0 on success
	h.regFile.WriteReg(0, 0)
	return SyscallResult{}
}

// writeStatToMemory writes a FileInfo to memory as an ARM64 stat structure.
func (h *DefaultSyscallHandler) writeStatToMemory(addr uint64, info os.FileInfo) {
	// Device ID (use 0 for simplicity)
	h.memory.Write64(addr+statOffsetDev, 0)

	// Inode (use 0 for simplicity)
	h.memory.Write64(addr+statOffsetIno, 0)

	// Mode - convert Go FileMode to Linux mode
	mode := h.fileInfoToLinuxMode(info)
	h.memory.Write32(addr+statOffsetMode, mode)

	// Number of hard links (use 1)
	h.memory.Write32(addr+statOffsetNlink, 1)

	// UID and GID (use 0)
	h.memory.Write32(addr+statOffsetUID, 0)
	h.memory.Write32(addr+statOffsetGID, 0)

	// Device ID for special files (use 0)
	h.memory.Write64(addr+statOffsetRdev, 0)

	// Padding
	h.memory.Write64(addr+statOffsetPad1, 0)

	// Size in bytes
	h.memory.Write64(addr+statOffsetSize, uint64(info.Size()))

	// Block size (use 4096)
	h.memory.Write32(addr+statOffsetBlksize, 4096)

	// Padding
	h.memory.Write32(addr+statOffsetPad2, 0)

	// Number of 512-byte blocks allocated
	blocks := (info.Size() + 511) / 512
	h.memory.Write64(addr+statOffsetBlocks, uint64(blocks))

	// Timestamps
	modTime := info.ModTime()
	h.memory.Write64(addr+statOffsetAtime, uint64(modTime.Unix()))
	h.memory.Write64(addr+statOffsetAtimeNs, uint64(modTime.Nanosecond()))
	h.memory.Write64(addr+statOffsetMtime, uint64(modTime.Unix()))
	h.memory.Write64(addr+statOffsetMtimeNs, uint64(modTime.Nanosecond()))
	h.memory.Write64(addr+statOffsetCtime, uint64(modTime.Unix()))
	h.memory.Write64(addr+statOffsetCtimeNs, uint64(modTime.Nanosecond()))
}

// Linux file mode constants.
const (
	S_IFMT   = 0170000 // File type mask
	S_IFREG  = 0100000 // Regular file
	S_IFDIR  = 0040000 // Directory
	S_IFCHR  = 0020000 // Character device
	S_IFIFO  = 0010000 // FIFO (named pipe)
	S_IFLNK  = 0120000 // Symbolic link
	S_IFSOCK = 0140000 // Socket
)

// fileInfoToLinuxMode converts Go FileMode to Linux mode_t.
func (h *DefaultSyscallHandler) fileInfoToLinuxMode(info os.FileInfo) uint32 {
	mode := uint32(info.Mode().Perm()) // Permission bits

	// File type
	switch {
	case info.IsDir():
		mode |= S_IFDIR
	case info.Mode()&os.ModeSymlink != 0:
		mode |= S_IFLNK
	case info.Mode()&os.ModeCharDevice != 0:
		mode |= S_IFCHR
	case info.Mode()&os.ModeNamedPipe != 0:
		mode |= S_IFIFO
	case info.Mode()&os.ModeSocket != 0:
		mode |= S_IFSOCK
	default:
		mode |= S_IFREG // Regular file
	}

	return mode
}
