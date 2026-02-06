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
})
