// Package emu provides functional ARM64 emulation.
package emu_test

import (
	"bytes"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sarchlab/m2sim/emu"
)

var _ = Describe("Syscall Handler", func() {
	var (
		regFile *emu.RegFile
		memory  *emu.Memory
		stdout  *bytes.Buffer
		stderr  *bytes.Buffer
		handler *emu.DefaultSyscallHandler
	)

	BeforeEach(func() {
		regFile = &emu.RegFile{}
		memory = emu.NewMemory()
		stdout = new(bytes.Buffer)
		stderr = new(bytes.Buffer)
		handler = emu.NewDefaultSyscallHandler(regFile, memory, stdout, stderr)
	})

	Describe("Unknown syscall", func() {
		It("should return ENOSYS for unknown syscall numbers", func() {
			// Set X8 to an unknown syscall number (e.g., 999)
			regFile.WriteReg(8, 999)

			result := handler.Handle()

			// Should not exit
			Expect(result.Exited).To(BeFalse())

			// X0 should contain -ENOSYS (38) as two's complement
			x0 := regFile.ReadReg(0)
			var enosys int64 = 38
			expectedError := uint64(-enosys) // -ENOSYS
			Expect(x0).To(Equal(expectedError))
		})

		It("should handle syscall 0 as unknown", func() {
			// Set X8 to syscall 0 (not implemented)
			regFile.WriteReg(8, 0)

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			// X0 should be -ENOSYS
			x0 := regFile.ReadReg(0)
			var enosys int64 = 38
			expectedError := uint64(-enosys)
			Expect(x0).To(Equal(expectedError))
		})
	})

	Describe("Write syscall with bad fd", func() {
		It("should return EBADF for invalid file descriptor", func() {
			// Set up write syscall with invalid fd
			regFile.WriteReg(8, 64) // SyscallWrite
			regFile.WriteReg(0, 42) // Invalid fd (not 1 or 2)
			regFile.WriteReg(1, 0)  // buf pointer
			regFile.WriteReg(2, 5)  // count

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			// X0 should contain -EBADF (9)
			x0 := regFile.ReadReg(0)
			var ebadf int64 = 9
			expectedError := uint64(-ebadf) // -EBADF
			Expect(x0).To(Equal(expectedError))
		})
	})

	Describe("Exit syscall", func() {
		It("should exit with specified code", func() {
			regFile.WriteReg(8, 93) // SyscallExit
			regFile.WriteReg(0, 42) // Exit code

			result := handler.Handle()

			Expect(result.Exited).To(BeTrue())
			Expect(result.ExitCode).To(Equal(int64(42)))
		})

		It("should handle zero exit code", func() {
			regFile.WriteReg(8, 93) // SyscallExit
			regFile.WriteReg(0, 0)  // Exit code 0

			result := handler.Handle()

			Expect(result.Exited).To(BeTrue())
			Expect(result.ExitCode).To(Equal(int64(0)))
		})
	})

	Describe("Write syscall to stdout", func() {
		It("should write buffer to stdout", func() {
			// Store "hello" in memory
			memory.Write8(0x1000, 'h')
			memory.Write8(0x1001, 'e')
			memory.Write8(0x1002, 'l')
			memory.Write8(0x1003, 'l')
			memory.Write8(0x1004, 'o')

			// Set up write syscall
			regFile.WriteReg(8, 64)     // SyscallWrite
			regFile.WriteReg(0, 1)      // stdout
			regFile.WriteReg(1, 0x1000) // buf pointer
			regFile.WriteReg(2, 5)      // count

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			Expect(stdout.String()).To(Equal("hello"))
			// X0 should contain bytes written
			Expect(regFile.ReadReg(0)).To(Equal(uint64(5)))
		})
	})

	Describe("Write syscall to stderr", func() {
		It("should write buffer to stderr", func() {
			// Store "err" in memory
			memory.Write8(0x2000, 'e')
			memory.Write8(0x2001, 'r')
			memory.Write8(0x2002, 'r')

			// Set up write syscall
			regFile.WriteReg(8, 64)     // SyscallWrite
			regFile.WriteReg(0, 2)      // stderr
			regFile.WriteReg(1, 0x2000) // buf pointer
			regFile.WriteReg(2, 3)      // count

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			Expect(stderr.String()).To(Equal("err"))
			Expect(regFile.ReadReg(0)).To(Equal(uint64(3)))
		})
	})

	Describe("Close syscall", func() {
		It("should close stdin successfully", func() {
			regFile.WriteReg(8, 57) // SyscallClose
			regFile.WriteReg(0, 0)  // stdin

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			// X0 should be 0 (success)
			Expect(regFile.ReadReg(0)).To(Equal(uint64(0)))
		})

		It("should close stdout successfully", func() {
			regFile.WriteReg(8, 57) // SyscallClose
			regFile.WriteReg(0, 1)  // stdout

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			Expect(regFile.ReadReg(0)).To(Equal(uint64(0)))
		})

		It("should close stderr successfully", func() {
			regFile.WriteReg(8, 57) // SyscallClose
			regFile.WriteReg(0, 2)  // stderr

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			Expect(regFile.ReadReg(0)).To(Equal(uint64(0)))
		})

		It("should return EBADF for invalid fd", func() {
			regFile.WriteReg(8, 57)  // SyscallClose
			regFile.WriteReg(0, 999) // Invalid fd

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			// X0 should contain -EBADF (9)
			x0 := regFile.ReadReg(0)
			var ebadf int64 = 9
			expectedError := uint64(-ebadf)
			Expect(x0).To(Equal(expectedError))
		})

		It("should return EBADF when closing already closed fd", func() {
			// First close stdin
			regFile.WriteReg(8, 57) // SyscallClose
			regFile.WriteReg(0, 0)  // stdin

			handler.Handle()

			// Try to close again
			regFile.WriteReg(8, 57)
			regFile.WriteReg(0, 0)

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			// X0 should contain -EBADF
			x0 := regFile.ReadReg(0)
			var ebadf int64 = 9
			expectedError := uint64(-ebadf)
			Expect(x0).To(Equal(expectedError))
		})
	})

	Describe("Openat syscall", func() {
		var tempDir string

		BeforeEach(func() {
			var err error
			tempDir, err = os.MkdirTemp("", "openat_test")
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			_ = os.RemoveAll(tempDir)
		})

		writePathToMemory := func(path string, addr uint64) {
			for i, c := range []byte(path) {
				memory.Write8(addr+uint64(i), c)
			}
			memory.Write8(addr+uint64(len(path)), 0) // null terminator
		}

		It("should open existing file for reading", func() {
			// Create a test file
			testFile := filepath.Join(tempDir, "test.txt")
			err := os.WriteFile(testFile, []byte("hello"), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Write path to memory
			writePathToMemory(testFile, 0x1000)

			// Set up openat syscall
			regFile.WriteReg(8, 56)               // SyscallOpenat
			regFile.WriteReg(0, emu.AT_FDCWD_U64) // AT_FDCWD
			regFile.WriteReg(1, 0x1000)           // pathname pointer
			regFile.WriteReg(2, 0)                // O_RDONLY
			regFile.WriteReg(3, 0)                // mode (ignored for O_RDONLY)

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			// X0 should be a valid fd >= 3
			fd := regFile.ReadReg(0)
			Expect(fd).To(BeNumerically(">=", 3))

			// Clean up: close the fd
			regFile.WriteReg(8, 57) // SyscallClose
			regFile.WriteReg(0, fd)
			handler.Handle()
		})

		It("should return ENOENT for non-existent file", func() {
			// Write non-existent path to memory
			writePathToMemory("/nonexistent/file.txt", 0x1000)

			// Set up openat syscall
			regFile.WriteReg(8, 56)               // SyscallOpenat
			regFile.WriteReg(0, emu.AT_FDCWD_U64) // AT_FDCWD
			regFile.WriteReg(1, 0x1000)           // pathname pointer
			regFile.WriteReg(2, 0)                // O_RDONLY
			regFile.WriteReg(3, 0)                // mode

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			// X0 should contain -ENOENT (2)
			x0 := regFile.ReadReg(0)
			var enoent int64 = 2
			expectedError := uint64(-enoent)
			Expect(x0).To(Equal(expectedError))
		})

		It("should create new file with O_CREAT", func() {
			newFile := filepath.Join(tempDir, "newfile.txt")
			writePathToMemory(newFile, 0x1000)

			// Set up openat syscall with O_WRONLY | O_CREAT
			regFile.WriteReg(8, 56)               // SyscallOpenat
			regFile.WriteReg(0, emu.AT_FDCWD_U64) // AT_FDCWD
			regFile.WriteReg(1, 0x1000)           // pathname pointer
			regFile.WriteReg(2, 1|0x40)           // O_WRONLY | O_CREAT
			regFile.WriteReg(3, 0644)             // mode

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			// X0 should be a valid fd >= 3
			fd := regFile.ReadReg(0)
			Expect(fd).To(BeNumerically(">=", 3))

			// Verify file was created
			_, err := os.Stat(newFile)
			Expect(err).ToNot(HaveOccurred())

			// Clean up: close the fd
			regFile.WriteReg(8, 57) // SyscallClose
			regFile.WriteReg(0, fd)
			handler.Handle()
		})

		It("should return EBADF for invalid dirfd", func() {
			testFile := filepath.Join(tempDir, "test.txt")
			err := os.WriteFile(testFile, []byte("hello"), 0644)
			Expect(err).ToNot(HaveOccurred())
			writePathToMemory(testFile, 0x1000)

			// Set up openat syscall with invalid dirfd (not AT_FDCWD)
			regFile.WriteReg(8, 56)     // SyscallOpenat
			regFile.WriteReg(0, 42)     // Invalid dirfd (not AT_FDCWD)
			regFile.WriteReg(1, 0x1000) // pathname pointer
			regFile.WriteReg(2, 0)      // O_RDONLY
			regFile.WriteReg(3, 0)      // mode

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			// X0 should contain -EBADF (9)
			x0 := regFile.ReadReg(0)
			var ebadf int64 = 9
			expectedError := uint64(-ebadf)
			Expect(x0).To(Equal(expectedError))
		})

		It("should allocate sequential file descriptors", func() {
			// Create test files
			testFile1 := filepath.Join(tempDir, "test1.txt")
			testFile2 := filepath.Join(tempDir, "test2.txt")
			err := os.WriteFile(testFile1, []byte("1"), 0644)
			Expect(err).ToNot(HaveOccurred())
			err = os.WriteFile(testFile2, []byte("2"), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Open first file
			writePathToMemory(testFile1, 0x1000)
			regFile.WriteReg(8, 56)
			regFile.WriteReg(0, emu.AT_FDCWD_U64)
			regFile.WriteReg(1, 0x1000)
			regFile.WriteReg(2, 0)
			regFile.WriteReg(3, 0)
			handler.Handle()
			fd1 := regFile.ReadReg(0)

			// Open second file
			writePathToMemory(testFile2, 0x2000)
			regFile.WriteReg(8, 56)
			regFile.WriteReg(0, emu.AT_FDCWD_U64)
			regFile.WriteReg(1, 0x2000)
			regFile.WriteReg(2, 0)
			regFile.WriteReg(3, 0)
			handler.Handle()
			fd2 := regFile.ReadReg(0)

			Expect(fd2).To(Equal(fd1 + 1))

			// Clean up
			regFile.WriteReg(8, 57)
			regFile.WriteReg(0, fd1)
			handler.Handle()
			regFile.WriteReg(8, 57)
			regFile.WriteReg(0, fd2)
			handler.Handle()
		})
	})

	Describe("Brk syscall", func() {
		It("should return default program break when addr is 0", func() {
			regFile.WriteReg(8, 214) // SyscallBrk
			regFile.WriteReg(0, 0)   // Query current break

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			// X0 should contain the default program break
			Expect(regFile.ReadReg(0)).To(Equal(emu.DefaultProgramBreak))
		})

		It("should extend program break when addr is larger", func() {
			newBreak := emu.DefaultProgramBreak + 0x10000 // Add 64KB

			regFile.WriteReg(8, 214)      // SyscallBrk
			regFile.WriteReg(0, newBreak) // Request larger break

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			// X0 should contain the new program break
			Expect(regFile.ReadReg(0)).To(Equal(newBreak))

			// Verify break was actually extended
			Expect(handler.GetProgramBreak()).To(Equal(newBreak))
		})

		It("should not shrink program break when addr is smaller", func() {
			// First, extend the break
			newBreak := emu.DefaultProgramBreak + 0x10000
			regFile.WriteReg(8, 214)
			regFile.WriteReg(0, newBreak)
			handler.Handle()

			// Try to shrink it
			smallerAddr := emu.DefaultProgramBreak - 0x1000
			regFile.WriteReg(8, 214)
			regFile.WriteReg(0, smallerAddr)

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			// X0 should still contain the current (larger) break
			Expect(regFile.ReadReg(0)).To(Equal(newBreak))
			Expect(handler.GetProgramBreak()).To(Equal(newBreak))
		})

		It("should handle multiple sequential brk calls", func() {
			// Query initial break
			regFile.WriteReg(8, 214)
			regFile.WriteReg(0, 0)
			handler.Handle()
			initialBreak := regFile.ReadReg(0)

			// Extend break
			firstExtend := initialBreak + 0x1000
			regFile.WriteReg(8, 214)
			regFile.WriteReg(0, firstExtend)
			handler.Handle()
			Expect(regFile.ReadReg(0)).To(Equal(firstExtend))

			// Extend again
			secondExtend := firstExtend + 0x2000
			regFile.WriteReg(8, 214)
			regFile.WriteReg(0, secondExtend)
			handler.Handle()
			Expect(regFile.ReadReg(0)).To(Equal(secondExtend))

			// Query again (addr=0)
			regFile.WriteReg(8, 214)
			regFile.WriteReg(0, 0)
			handler.Handle()
			Expect(regFile.ReadReg(0)).To(Equal(secondExtend))
		})

		It("should handle setting custom initial program break", func() {
			customBreak := uint64(0x20000000)
			handler.SetProgramBreak(customBreak)

			regFile.WriteReg(8, 214)
			regFile.WriteReg(0, 0)

			handler.Handle()

			Expect(regFile.ReadReg(0)).To(Equal(customBreak))
		})
	})

	Describe("Mmap syscall", func() {
		It("should allocate anonymous memory", func() {
			// mmap(NULL, 4096, PROT_READ|PROT_WRITE, MAP_PRIVATE|MAP_ANONYMOUS, -1, 0)
			regFile.WriteReg(8, 222)                               // SyscallMmap
			regFile.WriteReg(0, 0)                                 // addr (NULL = kernel chooses)
			regFile.WriteReg(1, 4096)                              // length
			regFile.WriteReg(2, emu.PROT_READ|emu.PROT_WRITE)      // prot
			regFile.WriteReg(3, emu.MAP_PRIVATE|emu.MAP_ANONYMOUS) // flags
			regFile.WriteReg(4, ^uint64(0))                        // fd = -1
			regFile.WriteReg(5, 0)                                 // offset

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			// Should return an address >= DefaultMmapBase
			addr := regFile.ReadReg(0)
			Expect(addr).To(BeNumerically(">=", emu.DefaultMmapBase))

			// Verify region was tracked
			regions := handler.GetMmapRegions()
			Expect(regions).To(HaveLen(1))
			Expect(regions[0].Addr).To(Equal(addr))
			Expect(regions[0].Length).To(Equal(uint64(4096)))
		})

		It("should page-align allocation length", func() {
			// Request 100 bytes, should be aligned to 4096
			regFile.WriteReg(8, 222)
			regFile.WriteReg(0, 0)
			regFile.WriteReg(1, 100) // Non-aligned length
			regFile.WriteReg(2, emu.PROT_READ)
			regFile.WriteReg(3, emu.MAP_PRIVATE|emu.MAP_ANONYMOUS)
			regFile.WriteReg(4, ^uint64(0))
			regFile.WriteReg(5, 0)

			handler.Handle()

			regions := handler.GetMmapRegions()
			Expect(regions).To(HaveLen(1))
			Expect(regions[0].Length).To(Equal(uint64(4096))) // Aligned to page
		})

		It("should allocate sequential regions", func() {
			// First allocation
			regFile.WriteReg(8, 222)
			regFile.WriteReg(0, 0)
			regFile.WriteReg(1, 4096)
			regFile.WriteReg(2, emu.PROT_READ|emu.PROT_WRITE)
			regFile.WriteReg(3, emu.MAP_PRIVATE|emu.MAP_ANONYMOUS)
			regFile.WriteReg(4, ^uint64(0))
			regFile.WriteReg(5, 0)
			handler.Handle()
			addr1 := regFile.ReadReg(0)

			// Second allocation
			regFile.WriteReg(8, 222)
			regFile.WriteReg(0, 0)
			regFile.WriteReg(1, 8192)
			regFile.WriteReg(2, emu.PROT_READ)
			regFile.WriteReg(3, emu.MAP_PRIVATE|emu.MAP_ANONYMOUS)
			regFile.WriteReg(4, ^uint64(0))
			regFile.WriteReg(5, 0)
			handler.Handle()
			addr2 := regFile.ReadReg(0)

			// Second allocation should start after first
			Expect(addr2).To(Equal(addr1 + 4096))

			regions := handler.GetMmapRegions()
			Expect(regions).To(HaveLen(2))
		})

		It("should handle MAP_FIXED", func() {
			fixedAddr := uint64(0x50000000) // Some page-aligned address

			regFile.WriteReg(8, 222)
			regFile.WriteReg(0, fixedAddr)
			regFile.WriteReg(1, 4096)
			regFile.WriteReg(2, emu.PROT_READ|emu.PROT_WRITE)
			regFile.WriteReg(3, emu.MAP_PRIVATE|emu.MAP_ANONYMOUS|emu.MAP_FIXED)
			regFile.WriteReg(4, ^uint64(0))
			regFile.WriteReg(5, 0)

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			Expect(regFile.ReadReg(0)).To(Equal(fixedAddr))
		})

		It("should return EINVAL for zero length", func() {
			regFile.WriteReg(8, 222)
			regFile.WriteReg(0, 0)
			regFile.WriteReg(1, 0) // Invalid: zero length
			regFile.WriteReg(2, emu.PROT_READ)
			regFile.WriteReg(3, emu.MAP_PRIVATE|emu.MAP_ANONYMOUS)
			regFile.WriteReg(4, ^uint64(0))
			regFile.WriteReg(5, 0)

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			// X0 should contain -EINVAL (22)
			x0 := regFile.ReadReg(0)
			var einval int64 = 22
			expectedError := uint64(-einval)
			Expect(x0).To(Equal(expectedError))
		})

		It("should return EINVAL for MAP_FIXED with NULL address", func() {
			regFile.WriteReg(8, 222)
			regFile.WriteReg(0, 0) // NULL with MAP_FIXED is invalid
			regFile.WriteReg(1, 4096)
			regFile.WriteReg(2, emu.PROT_READ)
			regFile.WriteReg(3, emu.MAP_PRIVATE|emu.MAP_ANONYMOUS|emu.MAP_FIXED)
			regFile.WriteReg(4, ^uint64(0))
			regFile.WriteReg(5, 0)

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			x0 := regFile.ReadReg(0)
			var einval int64 = 22
			expectedError := uint64(-einval)
			Expect(x0).To(Equal(expectedError))
		})

		It("should return ENOSYS for file mappings", func() {
			regFile.WriteReg(8, 222)
			regFile.WriteReg(0, 0)
			regFile.WriteReg(1, 4096)
			regFile.WriteReg(2, emu.PROT_READ)
			regFile.WriteReg(3, emu.MAP_PRIVATE) // No MAP_ANONYMOUS
			regFile.WriteReg(4, 5)               // Valid fd (file mapping)
			regFile.WriteReg(5, 0)

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			// X0 should contain -ENOSYS (38)
			x0 := regFile.ReadReg(0)
			var enosys int64 = 38
			expectedError := uint64(-enosys)
			Expect(x0).To(Equal(expectedError))
		})
	})

	Describe("Fstat syscall", func() {
		var tempDir string

		BeforeEach(func() {
			var err error
			tempDir, err = os.MkdirTemp("", "fstat_test")
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			_ = os.RemoveAll(tempDir)
		})

		writePathToMemory := func(path string, addr uint64) {
			for i, c := range []byte(path) {
				memory.Write8(addr+uint64(i), c)
			}
			memory.Write8(addr+uint64(len(path)), 0) // null terminator
		}

		It("should return file size and mode for regular file", func() {
			// Create a test file with known content
			testFile := filepath.Join(tempDir, "test.txt")
			content := []byte("hello world") // 11 bytes
			err := os.WriteFile(testFile, content, 0644)
			Expect(err).ToNot(HaveOccurred())

			// Open the file
			writePathToMemory(testFile, 0x1000)
			regFile.WriteReg(8, 56)               // SyscallOpenat
			regFile.WriteReg(0, emu.AT_FDCWD_U64) // AT_FDCWD
			regFile.WriteReg(1, 0x1000)           // pathname pointer
			regFile.WriteReg(2, 0)                // O_RDONLY
			regFile.WriteReg(3, 0)                // mode
			handler.Handle()
			fd := regFile.ReadReg(0)
			Expect(fd).To(BeNumerically(">=", 3))

			// Call fstat
			statbufAddr := uint64(0x2000)
			regFile.WriteReg(8, 80)          // SyscallFstat
			regFile.WriteReg(0, fd)          // fd
			regFile.WriteReg(1, statbufAddr) // statbuf

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			// X0 should be 0 (success)
			Expect(regFile.ReadReg(0)).To(Equal(uint64(0)))

			// Check file size at offset 48
			size := memory.Read64(statbufAddr + 48)
			Expect(size).To(Equal(uint64(11)))

			// Check mode at offset 16 - should be S_IFREG | permissions
			mode := memory.Read32(statbufAddr + 16)
			Expect(mode & 0170000).To(Equal(uint32(0100000))) // S_IFREG

			// Clean up
			regFile.WriteReg(8, 57) // SyscallClose
			regFile.WriteReg(0, fd)
			handler.Handle()
		})

		It("should return EBADF for invalid fd", func() {
			statbufAddr := uint64(0x2000)
			regFile.WriteReg(8, 80)          // SyscallFstat
			regFile.WriteReg(0, 999)         // Invalid fd
			regFile.WriteReg(1, statbufAddr) // statbuf

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			// X0 should contain -EBADF (9)
			x0 := regFile.ReadReg(0)
			var ebadf int64 = 9
			expectedError := uint64(-ebadf)
			Expect(x0).To(Equal(expectedError))
		})

		It("should handle fstat on stdout", func() {
			statbufAddr := uint64(0x2000)
			regFile.WriteReg(8, 80)          // SyscallFstat
			regFile.WriteReg(0, 1)           // stdout
			regFile.WriteReg(1, statbufAddr) // statbuf

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			// X0 should be 0 (success)
			Expect(regFile.ReadReg(0)).To(Equal(uint64(0)))

			// Check mode - should be character device
			mode := memory.Read32(statbufAddr + 16)
			Expect(mode & 0170000).To(Equal(uint32(0020000))) // S_IFCHR
		})

		It("should return correct block count", func() {
			// Create a file with known size (5000 bytes)
			testFile := filepath.Join(tempDir, "bigfile.txt")
			content := make([]byte, 5000)
			err := os.WriteFile(testFile, content, 0644)
			Expect(err).ToNot(HaveOccurred())

			// Open the file
			writePathToMemory(testFile, 0x1000)
			regFile.WriteReg(8, 56)               // SyscallOpenat
			regFile.WriteReg(0, emu.AT_FDCWD_U64) // AT_FDCWD
			regFile.WriteReg(1, 0x1000)           // pathname pointer
			regFile.WriteReg(2, 0)                // O_RDONLY
			regFile.WriteReg(3, 0)                // mode
			handler.Handle()
			fd := regFile.ReadReg(0)

			// Call fstat
			statbufAddr := uint64(0x2000)
			regFile.WriteReg(8, 80)          // SyscallFstat
			regFile.WriteReg(0, fd)          // fd
			regFile.WriteReg(1, statbufAddr) // statbuf
			handler.Handle()

			// Check blocks at offset 64 - should be ceil(5000/512) = 10
			blocks := memory.Read64(statbufAddr + 64)
			Expect(blocks).To(Equal(uint64(10)))

			// Clean up
			regFile.WriteReg(8, 57) // SyscallClose
			regFile.WriteReg(0, fd)
			handler.Handle()
		})
	})

	Describe("File I/O via read/write syscalls", func() {
		var tempDir string

		BeforeEach(func() {
			var err error
			tempDir, err = os.MkdirTemp("", "fileio_test")
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			_ = os.RemoveAll(tempDir)
		})

		writePathToMemory := func(path string, addr uint64) {
			for i, c := range []byte(path) {
				memory.Write8(addr+uint64(i), c)
			}
			memory.Write8(addr+uint64(len(path)), 0) // null terminator
		}

		It("should read from opened file", func() {
			// Create a test file with known content
			testFile := filepath.Join(tempDir, "test.txt")
			content := []byte("hello file")
			err := os.WriteFile(testFile, content, 0644)
			Expect(err).ToNot(HaveOccurred())

			// Open the file
			writePathToMemory(testFile, 0x1000)
			regFile.WriteReg(8, 56)               // SyscallOpenat
			regFile.WriteReg(0, emu.AT_FDCWD_U64) // AT_FDCWD
			regFile.WriteReg(1, 0x1000)           // pathname pointer
			regFile.WriteReg(2, 0)                // O_RDONLY
			regFile.WriteReg(3, 0)                // mode
			handler.Handle()
			fd := regFile.ReadReg(0)
			Expect(fd).To(BeNumerically(">=", 3))

			// Read from the file
			bufAddr := uint64(0x2000)
			regFile.WriteReg(8, 63)      // SyscallRead
			regFile.WriteReg(0, fd)      // fd
			regFile.WriteReg(1, bufAddr) // buf pointer
			regFile.WriteReg(2, 10)      // count

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			// X0 should be bytes read
			Expect(regFile.ReadReg(0)).To(Equal(uint64(10)))

			// Verify content in memory
			var readBuf []byte
			for i := uint64(0); i < 10; i++ {
				readBuf = append(readBuf, memory.Read8(bufAddr+i))
			}
			Expect(string(readBuf)).To(Equal("hello file"))

			// Clean up
			regFile.WriteReg(8, 57) // SyscallClose
			regFile.WriteReg(0, fd)
			handler.Handle()
		})

		It("should write to opened file", func() {
			// Open new file for writing
			testFile := filepath.Join(tempDir, "output.txt")
			writePathToMemory(testFile, 0x1000)

			regFile.WriteReg(8, 56)               // SyscallOpenat
			regFile.WriteReg(0, emu.AT_FDCWD_U64) // AT_FDCWD
			regFile.WriteReg(1, 0x1000)           // pathname pointer
			regFile.WriteReg(2, 1|0x40)           // O_WRONLY | O_CREAT
			regFile.WriteReg(3, 0644)             // mode
			handler.Handle()
			fd := regFile.ReadReg(0)
			Expect(fd).To(BeNumerically(">=", 3))

			// Write "test data" to memory
			content := []byte("test data")
			bufAddr := uint64(0x2000)
			for i, b := range content {
				memory.Write8(bufAddr+uint64(i), b)
			}

			// Write to the file
			regFile.WriteReg(8, 64)                   // SyscallWrite
			regFile.WriteReg(0, fd)                   // fd
			regFile.WriteReg(1, bufAddr)              // buf pointer
			regFile.WriteReg(2, uint64(len(content))) // count

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			// X0 should be bytes written
			Expect(regFile.ReadReg(0)).To(Equal(uint64(len(content))))

			// Close the file
			regFile.WriteReg(8, 57) // SyscallClose
			regFile.WriteReg(0, fd)
			handler.Handle()

			// Verify file contents on disk
			fileContent, err := os.ReadFile(testFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(fileContent)).To(Equal("test data"))
		})

		It("should return EBADF for read on invalid fd", func() {
			bufAddr := uint64(0x2000)
			regFile.WriteReg(8, 63)      // SyscallRead
			regFile.WriteReg(0, 999)     // Invalid fd
			regFile.WriteReg(1, bufAddr) // buf pointer
			regFile.WriteReg(2, 10)      // count

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			// X0 should contain -EBADF (9)
			x0 := regFile.ReadReg(0)
			var ebadf int64 = 9
			expectedError := uint64(-ebadf)
			Expect(x0).To(Equal(expectedError))
		})

		It("should return EBADF for write on invalid fd", func() {
			// Write some data to memory
			bufAddr := uint64(0x2000)
			memory.Write8(bufAddr, 'x')

			regFile.WriteReg(8, 64)      // SyscallWrite
			regFile.WriteReg(0, 999)     // Invalid fd
			regFile.WriteReg(1, bufAddr) // buf pointer
			regFile.WriteReg(2, 1)       // count

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			// X0 should contain -EBADF (9)
			x0 := regFile.ReadReg(0)
			var ebadf int64 = 9
			expectedError := uint64(-ebadf)
			Expect(x0).To(Equal(expectedError))
		})

		It("should still write to stdout correctly", func() {
			// Store "hi" in memory
			memory.Write8(0x1000, 'h')
			memory.Write8(0x1001, 'i')

			// Write to stdout
			regFile.WriteReg(8, 64)     // SyscallWrite
			regFile.WriteReg(0, 1)      // stdout
			regFile.WriteReg(1, 0x1000) // buf pointer
			regFile.WriteReg(2, 2)      // count

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			Expect(stdout.String()).To(Equal("hi"))
			Expect(regFile.ReadReg(0)).To(Equal(uint64(2)))
		})
	})

	Describe("Lseek syscall", func() {
		var tempDir string

		BeforeEach(func() {
			var err error
			tempDir, err = os.MkdirTemp("", "lseek_test")
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			_ = os.RemoveAll(tempDir)
		})

		writePathToMemory := func(path string, addr uint64) {
			for i, c := range []byte(path) {
				memory.Write8(addr+uint64(i), c)
			}
			memory.Write8(addr+uint64(len(path)), 0) // null terminator
		}

		It("should seek to beginning of file with SEEK_SET", func() {
			// Create a test file
			testFile := filepath.Join(tempDir, "test.txt")
			err := os.WriteFile(testFile, []byte("hello world"), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Open the file
			writePathToMemory(testFile, 0x1000)
			regFile.WriteReg(8, 56)               // SyscallOpenat
			regFile.WriteReg(0, emu.AT_FDCWD_U64) // AT_FDCWD
			regFile.WriteReg(1, 0x1000)           // pathname pointer
			regFile.WriteReg(2, 0)                // O_RDONLY
			regFile.WriteReg(3, 0)                // mode
			handler.Handle()
			fd := regFile.ReadReg(0)
			Expect(fd).To(BeNumerically(">=", 3))

			// Seek to position 6 with SEEK_SET
			regFile.WriteReg(8, 62) // SyscallLseek
			regFile.WriteReg(0, fd) // fd
			regFile.WriteReg(1, 6)  // offset
			regFile.WriteReg(2, 0)  // SEEK_SET

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			Expect(regFile.ReadReg(0)).To(Equal(uint64(6)))

			// Clean up: close the fd
			regFile.WriteReg(8, 57) // SyscallClose
			regFile.WriteReg(0, fd)
			handler.Handle()
		})

		It("should seek from current position with SEEK_CUR", func() {
			// Create a test file
			testFile := filepath.Join(tempDir, "test.txt")
			err := os.WriteFile(testFile, []byte("hello world"), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Open the file
			writePathToMemory(testFile, 0x1000)
			regFile.WriteReg(8, 56)               // SyscallOpenat
			regFile.WriteReg(0, emu.AT_FDCWD_U64) // AT_FDCWD
			regFile.WriteReg(1, 0x1000)           // pathname pointer
			regFile.WriteReg(2, 0)                // O_RDONLY
			regFile.WriteReg(3, 0)                // mode
			handler.Handle()
			fd := regFile.ReadReg(0)

			// First seek to position 3
			regFile.WriteReg(8, 62)
			regFile.WriteReg(0, fd)
			regFile.WriteReg(1, 3)
			regFile.WriteReg(2, 0) // SEEK_SET
			handler.Handle()

			// Now seek +4 from current position
			regFile.WriteReg(8, 62)
			regFile.WriteReg(0, fd)
			regFile.WriteReg(1, 4)
			regFile.WriteReg(2, 1) // SEEK_CUR

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			Expect(regFile.ReadReg(0)).To(Equal(uint64(7)))

			// Clean up
			regFile.WriteReg(8, 57)
			regFile.WriteReg(0, fd)
			handler.Handle()
		})

		It("should seek from end of file with SEEK_END", func() {
			// Create a test file
			testFile := filepath.Join(tempDir, "test.txt")
			err := os.WriteFile(testFile, []byte("hello world"), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Open the file
			writePathToMemory(testFile, 0x1000)
			regFile.WriteReg(8, 56)
			regFile.WriteReg(0, emu.AT_FDCWD_U64)
			regFile.WriteReg(1, 0x1000)
			regFile.WriteReg(2, 0)
			regFile.WriteReg(3, 0)
			handler.Handle()
			fd := regFile.ReadReg(0)

			// Seek to -5 from end (should be at position 6 in "hello world")
			regFile.WriteReg(8, 62)
			regFile.WriteReg(0, fd)
			var negOffset int64 = -5
			regFile.WriteReg(1, uint64(negOffset)) // -5 as two's complement
			regFile.WriteReg(2, 2)                 // SEEK_END

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			// "hello world" is 11 bytes, 11 - 5 = 6
			Expect(regFile.ReadReg(0)).To(Equal(uint64(6)))

			// Clean up
			regFile.WriteReg(8, 57)
			regFile.WriteReg(0, fd)
			handler.Handle()
		})

		It("should return ESPIPE for stdin", func() {
			regFile.WriteReg(8, 62) // SyscallLseek
			regFile.WriteReg(0, 0)  // stdin
			regFile.WriteReg(1, 0)  // offset
			regFile.WriteReg(2, 0)  // SEEK_SET

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			// X0 should contain -ESPIPE (29)
			x0 := regFile.ReadReg(0)
			var espipe int64 = 29
			expectedError := uint64(-espipe)
			Expect(x0).To(Equal(expectedError))
		})

		It("should return ESPIPE for stdout", func() {
			regFile.WriteReg(8, 62) // SyscallLseek
			regFile.WriteReg(0, 1)  // stdout
			regFile.WriteReg(1, 0)  // offset
			regFile.WriteReg(2, 0)  // SEEK_SET

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			// X0 should contain -ESPIPE (29)
			x0 := regFile.ReadReg(0)
			var espipe int64 = 29
			expectedError := uint64(-espipe)
			Expect(x0).To(Equal(expectedError))
		})

		It("should return ESPIPE for stderr", func() {
			regFile.WriteReg(8, 62) // SyscallLseek
			regFile.WriteReg(0, 2)  // stderr
			regFile.WriteReg(1, 0)  // offset
			regFile.WriteReg(2, 0)  // SEEK_SET

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			// X0 should contain -ESPIPE (29)
			x0 := regFile.ReadReg(0)
			var espipe int64 = 29
			expectedError := uint64(-espipe)
			Expect(x0).To(Equal(expectedError))
		})

		It("should return EBADF for invalid fd", func() {
			regFile.WriteReg(8, 62) // SyscallLseek
			regFile.WriteReg(0, 99) // invalid fd
			regFile.WriteReg(1, 0)  // offset
			regFile.WriteReg(2, 0)  // SEEK_SET

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			// X0 should contain -EBADF (9)
			x0 := regFile.ReadReg(0)
			var ebadf int64 = 9
			expectedError := uint64(-ebadf)
			Expect(x0).To(Equal(expectedError))
		})

		It("should return EINVAL for invalid whence", func() {
			// Create a test file
			testFile := filepath.Join(tempDir, "test.txt")
			err := os.WriteFile(testFile, []byte("hello world"), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Open the file
			writePathToMemory(testFile, 0x1000)
			regFile.WriteReg(8, 56)               // SyscallOpenat
			regFile.WriteReg(0, emu.AT_FDCWD_U64) // AT_FDCWD
			regFile.WriteReg(1, 0x1000)           // pathname pointer
			regFile.WriteReg(2, 0)                // O_RDONLY
			regFile.WriteReg(3, 0)                // mode
			handler.Handle()
			fd := regFile.ReadReg(0)
			Expect(fd).To(BeNumerically(">=", 3))

			// Try to seek with invalid whence (5 is invalid)
			regFile.WriteReg(8, 62) // SyscallLseek
			regFile.WriteReg(0, fd) // fd
			regFile.WriteReg(1, 0)  // offset
			regFile.WriteReg(2, 5)  // invalid whence

			result := handler.Handle()

			Expect(result.Exited).To(BeFalse())
			// X0 should contain -EINVAL (22) per lseek(2) manual
			x0 := regFile.ReadReg(0)
			var einval int64 = 22
			expectedError := uint64(-einval)
			Expect(x0).To(Equal(expectedError))

			// Clean up
			regFile.WriteReg(8, 57) // SyscallClose
			regFile.WriteReg(0, fd)
			handler.Handle()
		})
	})
})
