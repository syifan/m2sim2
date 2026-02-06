// Package benchmarks contains acceptance tests for file I/O syscalls.
package benchmarks

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/sarchlab/m2sim/emu"
)

// TestFileIOAcceptance tests complete file I/O workflows through the emulator.
// These tests verify that file operations work correctly at the syscall level.
func TestFileIOAcceptance(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fileio_acceptance")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	t.Run("open_close_workflow", func(t *testing.T) {
		// Test opening and closing a file
		testFile := filepath.Join(tempDir, "open_close_test.txt")
		err := os.WriteFile(testFile, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		e := emu.NewEmulator(
			emu.WithMaxInstructions(100000),
		)

		// Write file path to memory at 0x3000
		for i, c := range []byte(testFile) {
			e.Memory().Write8(0x3000+uint64(i), c)
		}
		e.Memory().Write8(0x3000+uint64(len(testFile)), 0) // null terminator

		// Program: open file, save fd, close file, exit with 0 if success
		program := buildProgram(
			// openat(AT_FDCWD, path, O_RDONLY, 0)
			encodeADDImm(8, 31, 56, false), // X8 = 56 (openat)
			encodeMOVN(0, 99, 0),           // X0 = AT_FDCWD (-100)
			encodeMOVZ(1, 0x3000, 0),       // X1 = 0x3000 (path pointer)
			encodeMOVZ(2, 0, 0),            // X2 = 0 (O_RDONLY)
			encodeMOVZ(3, 0, 0),            // X3 = 0 (mode)
			encodeSVC(0),                   // syscall
			encodeADDReg(19, 0, 31, false), // X19 = fd (save for later)

			// close(fd)
			encodeADDImm(8, 31, 57, false), // X8 = 57 (close)
			encodeADDReg(0, 19, 31, false), // X0 = fd
			encodeSVC(0),                   // syscall
			encodeADDReg(20, 0, 31, false), // X20 = close result

			// exit(close_result) - should be 0 on success
			encodeADDImm(8, 31, 93, false), // X8 = 93 (exit)
			encodeADDReg(0, 20, 31, false), // X0 = close result
			encodeSVC(0),                   // syscall
		)

		e.LoadProgram(0x1000, program)
		exitCode := e.Run()

		// Close returns 0 on success
		if exitCode != 0 {
			t.Errorf("expected exit code 0 (close success), got %d", exitCode)
		}

		t.Logf("✓ open_close_workflow: opened and closed file successfully")
	})

	t.Run("lseek_on_opened_file", func(t *testing.T) {
		// Skip until lseek syscall is merged (PR #282)
		t.Skip("lseek syscall not yet merged - awaiting PR #282")
	})

	t.Run("lseek_espipe_for_stdin", func(t *testing.T) {
		// Skip until lseek syscall is merged (PR #282)
		t.Skip("lseek syscall not yet merged - awaiting PR #282")
	})

	t.Run("lseek_espipe_for_stdout", func(t *testing.T) {
		// Skip until lseek syscall is merged (PR #282)
		t.Skip("lseek syscall not yet merged - awaiting PR #282")
	})

	t.Run("lseek_espipe_for_stderr", func(t *testing.T) {
		// Skip until lseek syscall is merged (PR #282)
		t.Skip("lseek syscall not yet merged - awaiting PR #282")
	})

	t.Run("lseek_ebadf_for_invalid_fd", func(t *testing.T) {
		// Skip until lseek syscall is merged (PR #282)
		t.Skip("lseek syscall not yet merged - awaiting PR #282")
	})

	t.Run("open_nonexistent_file", func(t *testing.T) {
		e := emu.NewEmulator(
			emu.WithMaxInstructions(1000),
		)

		// Write non-existent path to memory
		nonExistentPath := "/tmp/definitely_does_not_exist_12345.txt"
		for i, c := range []byte(nonExistentPath) {
			e.Memory().Write8(0x3000+uint64(i), c)
		}
		e.Memory().Write8(0x3000+uint64(len(nonExistentPath)), 0)

		program := buildProgram(
			// openat(AT_FDCWD, path, O_RDONLY, 0)
			encodeADDImm(8, 31, 56, false),
			encodeMOVN(0, 99, 0),
			encodeMOVZ(1, 0x3000, 0),
			encodeMOVZ(2, 0, 0),
			encodeMOVZ(3, 0, 0),
			encodeSVC(0),

			encodeSUBReg(0, 31, 0, false),
			encodeADDImm(8, 31, 93, false),
			encodeSVC(0),
		)

		e.LoadProgram(0x1000, program)
		exitCode := e.Run()

		// Should return ENOENT (2)
		if exitCode != 2 {
			t.Errorf("expected ENOENT (2), got %d", exitCode)
		}

		t.Logf("✓ open_nonexistent_file: correctly returned ENOENT")
	})

	t.Run("lseek_seek_end", func(t *testing.T) {
		// Skip until lseek syscall is merged (PR #282)
		t.Skip("lseek syscall not yet merged - awaiting PR #282")
	})

	t.Run("lseek_seek_cur", func(t *testing.T) {
		// Skip until lseek syscall is merged (PR #282)
		t.Skip("lseek syscall not yet merged - awaiting PR #282")
	})

	t.Run("close_invalid_fd", func(t *testing.T) {
		e := emu.NewEmulator(
			emu.WithMaxInstructions(1000),
		)

		program := buildProgram(
			// close(99) - invalid fd
			encodeADDImm(8, 31, 57, false),
			encodeMOVZ(0, 99, 0),
			encodeSVC(0),

			encodeSUBReg(0, 31, 0, false),
			encodeADDImm(8, 31, 93, false),
			encodeSVC(0),
		)

		e.LoadProgram(0x1000, program)
		exitCode := e.Run()

		// Should return EBADF (9)
		if exitCode != 9 {
			t.Errorf("expected EBADF (9), got %d", exitCode)
		}

		t.Logf("✓ close_invalid_fd: correctly returned EBADF")
	})

	t.Run("multiple_files", func(t *testing.T) {
		// Test opening multiple files and verifying sequential FD allocation
		testFile1 := filepath.Join(tempDir, "multi1.txt")
		testFile2 := filepath.Join(tempDir, "multi2.txt")
		err := os.WriteFile(testFile1, []byte("file1"), 0644)
		if err != nil {
			t.Fatalf("failed to create test file1: %v", err)
		}
		err = os.WriteFile(testFile2, []byte("file2"), 0644)
		if err != nil {
			t.Fatalf("failed to create test file2: %v", err)
		}

		e := emu.NewEmulator(
			emu.WithMaxInstructions(100000),
		)

		// Write paths to memory
		for i, c := range []byte(testFile1) {
			e.Memory().Write8(0x3000+uint64(i), c)
		}
		e.Memory().Write8(0x3000+uint64(len(testFile1)), 0)
		for i, c := range []byte(testFile2) {
			e.Memory().Write8(0x4000+uint64(i), c)
		}
		e.Memory().Write8(0x4000+uint64(len(testFile2)), 0)

		// Program: open file1, save fd1, open file2, calculate fd2-fd1, exit with difference
		program := buildProgram(
			// openat file1
			encodeADDImm(8, 31, 56, false),
			encodeMOVN(0, 99, 0),
			encodeMOVZ(1, 0x3000, 0),
			encodeMOVZ(2, 0, 0),
			encodeMOVZ(3, 0, 0),
			encodeSVC(0),
			encodeADDReg(19, 0, 31, false), // X19 = fd1

			// openat file2
			encodeADDImm(8, 31, 56, false),
			encodeMOVN(0, 99, 0),
			encodeMOVZ(1, 0x4000, 0),
			encodeMOVZ(2, 0, 0),
			encodeMOVZ(3, 0, 0),
			encodeSVC(0),
			encodeADDReg(20, 0, 31, false), // X20 = fd2

			// X21 = fd2 - fd1 (should be 1)
			encodeSUBReg(21, 20, 19, false),

			// close both
			encodeADDImm(8, 31, 57, false),
			encodeADDReg(0, 19, 31, false),
			encodeSVC(0),
			encodeADDImm(8, 31, 57, false),
			encodeADDReg(0, 20, 31, false),
			encodeSVC(0),

			// exit with fd difference
			encodeADDImm(8, 31, 93, false),
			encodeADDReg(0, 21, 31, false),
			encodeSVC(0),
		)

		e.LoadProgram(0x1000, program)
		exitCode := e.Run()

		// FDs should be sequential (difference = 1)
		if exitCode != 1 {
			t.Errorf("expected fd difference 1, got %d", exitCode)
		}

		t.Logf("✓ multiple_files: FDs allocated sequentially (difference=%d)", exitCode)
	})

	t.Run("write_to_stdout", func(t *testing.T) {
		// Test write syscall to stdout still works
		stdoutBuf := &bytes.Buffer{}
		e := emu.NewEmulator(
			emu.WithMaxInstructions(10000),
			emu.WithStdout(stdoutBuf),
		)

		// Write "OK\n" to memory
		e.Memory().Write8(0x3000, 'O')
		e.Memory().Write8(0x3001, 'K')
		e.Memory().Write8(0x3002, '\n')

		program := buildProgram(
			// write(1, buf, 3)
			encodeADDImm(8, 31, 64, false), // X8 = 64 (write)
			encodeMOVZ(0, 1, 0),            // X0 = 1 (stdout)
			encodeMOVZ(1, 0x3000, 0),       // X1 = buffer
			encodeMOVZ(2, 3, 0),            // X2 = count
			encodeSVC(0),
			encodeADDReg(19, 0, 31, false), // X19 = bytes written

			// exit with bytes written
			encodeADDImm(8, 31, 93, false),
			encodeADDReg(0, 19, 31, false),
			encodeSVC(0),
		)

		e.LoadProgram(0x1000, program)
		exitCode := e.Run()

		if exitCode != 3 {
			t.Errorf("expected 3 bytes written, got %d", exitCode)
		}

		if stdoutBuf.String() != "OK\n" {
			t.Errorf("expected stdout 'OK\\n', got %q", stdoutBuf.String())
		}

		t.Logf("✓ write_to_stdout: wrote %d bytes correctly", exitCode)
	})
}

// encodeMOVN encodes MOVN (move wide with NOT) for 64-bit.
func encodeMOVN(rd uint8, imm16 uint16, hw uint8) uint32 {
	var inst uint32 = 0
	inst |= 1 << 31        // sf = 1 (64-bit)
	inst |= 0b00 << 29     // opc = 00 (MOVN)
	inst |= 0b100101 << 23 // fixed bits
	inst |= uint32(hw&3) << 21
	inst |= uint32(imm16) << 5
	inst |= uint32(rd & 0x1F)
	return inst
}

// encodeSUBReg encodes SUB (register) for 64-bit.
func encodeSUBReg(rd, rn, rm uint8, setFlags bool) uint32 {
	var inst uint32 = 0
	inst |= 1 << 31 // sf = 1 (64-bit)
	inst |= 1 << 30 // op = 1 (SUB)
	if setFlags {
		inst |= 1 << 29 // S = 1 (set flags)
	}
	inst |= 0b01011 << 24
	inst |= 0 << 22 // shift = LSL
	inst |= 0 << 21 // N = 0
	inst |= uint32(rm&0x1F) << 16
	inst |= 0 << 10 // imm6 = 0
	inst |= uint32(rn&0x1F) << 5
	inst |= uint32(rd & 0x1F)
	return inst
}
