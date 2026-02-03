package loader_test

import (
	"encoding/binary"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sarchlab/m2sim/loader"
)

var _ = Describe("ELF Loader", func() {
	var tempDir string

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "elf-loader-test")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		_ = os.RemoveAll(tempDir)
	})

	Describe("Load", func() {
		Context("with a valid ARM64 ELF binary", func() {
			var elfPath string

			BeforeEach(func() {
				elfPath = filepath.Join(tempDir, "test.elf")
				createMinimalARM64ELF(elfPath, 0x400000, 0x400080, []byte{
					// Simple ARM64 code: mov x0, #42; ret
					0x40, 0x05, 0x80, 0xd2, // mov x0, #42
					0xc0, 0x03, 0x5f, 0xd6, // ret
				})
			})

			It("should load without error", func() {
				prog, err := loader.Load(elfPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(prog).NotTo(BeNil())
			})

			It("should extract the correct entry point", func() {
				prog, err := loader.Load(elfPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(prog.EntryPoint).To(Equal(uint64(0x400080)))
			})

			It("should load segments into memory", func() {
				prog, err := loader.Load(elfPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(prog.Segments)).To(BeNumerically(">", 0))
			})

			It("should set up initial stack pointer", func() {
				prog, err := loader.Load(elfPath)
				Expect(err).NotTo(HaveOccurred())
				// Stack should be set to a reasonable high address
				Expect(prog.InitialSP).To(BeNumerically(">", 0x7f0000000000))
			})
		})

		Context("with segment data", func() {
			It("should correctly load segment contents", func() {
				elfPath := filepath.Join(tempDir, "code.elf")
				codeData := []byte{
					0x40, 0x05, 0x80, 0xd2, // mov x0, #42
					0xc0, 0x03, 0x5f, 0xd6, // ret
				}
				createMinimalARM64ELF(elfPath, 0x400000, 0x400000, codeData)

				prog, err := loader.Load(elfPath)
				Expect(err).NotTo(HaveOccurred())

				// Find the segment containing our code
				var foundSegment *loader.Segment
				for i := range prog.Segments {
					if prog.Segments[i].VirtAddr == 0x400000 {
						foundSegment = &prog.Segments[i]
						break
					}
				}
				Expect(foundSegment).NotTo(BeNil())
				Expect(foundSegment.Data).To(HaveLen(len(codeData)))
			})
		})

		Context("with an invalid file", func() {
			It("should return error for non-existent file", func() {
				_, err := loader.Load("/nonexistent/path/to/file.elf")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to open"))
			})

			It("should return error for non-ELF file", func() {
				notElfPath := filepath.Join(tempDir, "not-elf.bin")
				err := os.WriteFile(notElfPath, []byte("not an elf file"), 0644)
				Expect(err).NotTo(HaveOccurred())

				_, err = loader.Load(notElfPath)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("ELF"))
			})

			It("should return error for empty file", func() {
				emptyPath := filepath.Join(tempDir, "empty.elf")
				err := os.WriteFile(emptyPath, []byte{}, 0644)
				Expect(err).NotTo(HaveOccurred())

				_, err = loader.Load(emptyPath)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("with non-ARM64 ELF", func() {
			It("should return error for x86-64 ELF", func() {
				elfPath := filepath.Join(tempDir, "x86.elf")
				createMinimalx86ELF(elfPath)

				_, err := loader.Load(elfPath)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not an ARM64"))
			})
		})

		Context("with 32-bit ELF", func() {
			It("should return error for 32-bit ELF", func() {
				elfPath := filepath.Join(tempDir, "elf32.elf")
				createMinimal32BitELF(elfPath)

				_, err := loader.Load(elfPath)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not a 64-bit"))
			})
		})
	})

	Describe("Program", func() {
		It("should provide LoadIntoMemory helper", func() {
			elfPath := filepath.Join(tempDir, "test.elf")
			codeData := []byte{0x40, 0x05, 0x80, 0xd2, 0xc0, 0x03, 0x5f, 0xd6}
			createMinimalARM64ELF(elfPath, 0x400000, 0x400000, codeData)

			prog, err := loader.Load(elfPath)
			Expect(err).NotTo(HaveOccurred())

			// Verify segments can be iterated for loading
			totalBytes := uint64(0)
			for _, seg := range prog.Segments {
				totalBytes += seg.MemSize
			}
			Expect(totalBytes).To(BeNumerically(">", 0))
		})
	})

	Describe("Segment", func() {
		It("should have correct virtual address", func() {
			elfPath := filepath.Join(tempDir, "test.elf")
			createMinimalARM64ELF(elfPath, 0x500000, 0x500000, []byte{0x00})

			prog, err := loader.Load(elfPath)
			Expect(err).NotTo(HaveOccurred())

			found := false
			for _, seg := range prog.Segments {
				if seg.VirtAddr == 0x500000 {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue())
		})

		It("should correctly report permissions", func() {
			elfPath := filepath.Join(tempDir, "test.elf")
			createMinimalARM64ELF(elfPath, 0x400000, 0x400000, []byte{0x00})

			prog, err := loader.Load(elfPath)
			Expect(err).NotTo(HaveOccurred())

			// At least one segment should be executable (code)
			hasExecutable := false
			for _, seg := range prog.Segments {
				if seg.Flags&loader.SegmentFlagExecute != 0 {
					hasExecutable = true
					break
				}
			}
			Expect(hasExecutable).To(BeTrue())
		})
	})

	Describe("Multi-segment ELFs", func() {
		It("should load multiple PT_LOAD segments", func() {
			elfPath := filepath.Join(tempDir, "multi-segment.elf")
			codeData := []byte{0x40, 0x05, 0x80, 0xd2, 0xc0, 0x03, 0x5f, 0xd6}
			dataData := []byte{0x01, 0x02, 0x03, 0x04}
			createMultiSegmentARM64ELF(elfPath, 0x400000, 0x400000, codeData, 0x600000, dataData)

			prog, err := loader.Load(elfPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(prog.Segments).To(HaveLen(2))

			// Find code segment
			var codeSeg, dataSeg *loader.Segment
			for i := range prog.Segments {
				if prog.Segments[i].VirtAddr == 0x400000 {
					codeSeg = &prog.Segments[i]
				}
				if prog.Segments[i].VirtAddr == 0x600000 {
					dataSeg = &prog.Segments[i]
				}
			}

			Expect(codeSeg).NotTo(BeNil())
			Expect(codeSeg.Data).To(Equal(codeData))
			Expect(codeSeg.Flags & loader.SegmentFlagExecute).NotTo(BeZero())

			Expect(dataSeg).NotTo(BeNil())
			Expect(dataSeg.Data).To(Equal(dataData))
			Expect(dataSeg.Flags & loader.SegmentFlagWrite).NotTo(BeZero())
		})
	})

	Describe("BSS segments", func() {
		It("should handle BSS segments where Memsz > Filesz", func() {
			elfPath := filepath.Join(tempDir, "bss.elf")
			initialData := []byte{0x01, 0x02, 0x03, 0x04}
			memSize := uint64(1024) // Much larger than file data
			createBSSSegmentELF(elfPath, 0x600000, 0x400000, initialData, memSize)

			prog, err := loader.Load(elfPath)
			Expect(err).NotTo(HaveOccurred())

			// Find the BSS segment
			var bssSeg *loader.Segment
			for i := range prog.Segments {
				if prog.Segments[i].VirtAddr == 0x600000 {
					bssSeg = &prog.Segments[i]
					break
				}
			}

			Expect(bssSeg).NotTo(BeNil())
			Expect(bssSeg.Data).To(Equal(initialData))
			Expect(bssSeg.MemSize).To(Equal(memSize))
			Expect(bssSeg.MemSize).To(BeNumerically(">", uint64(len(bssSeg.Data))))
		})
	})

	Describe("Zero Filesz segments", func() {
		It("should handle segments with zero file size", func() {
			elfPath := filepath.Join(tempDir, "zero-filesz.elf")
			memSize := uint64(4096)
			createZeroFileszELF(elfPath, 0x700000, 0x400000, memSize)

			prog, err := loader.Load(elfPath)
			Expect(err).NotTo(HaveOccurred())

			// Find the zero-filesz segment
			var zeroSeg *loader.Segment
			for i := range prog.Segments {
				if prog.Segments[i].VirtAddr == 0x700000 {
					zeroSeg = &prog.Segments[i]
					break
				}
			}

			Expect(zeroSeg).NotTo(BeNil())
			Expect(zeroSeg.Data).To(HaveLen(0))
			Expect(zeroSeg.MemSize).To(Equal(memSize))
		})
	})

	Describe("ELFs with no loadable segments", func() {
		It("should return empty segments list for ELF with no PT_LOAD", func() {
			elfPath := filepath.Join(tempDir, "no-load.elf")
			createNoLoadableSegmentsELF(elfPath, 0x400000)

			prog, err := loader.Load(elfPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(prog.Segments).To(BeEmpty())
			Expect(prog.EntryPoint).To(Equal(uint64(0x400000)))
		})
	})
})

// createMinimalARM64ELF creates a minimal valid ARM64 ELF64 binary.
func createMinimalARM64ELF(path string, loadAddr, entryPoint uint64, code []byte) {
	// ELF Header (64 bytes)
	elfHeader := make([]byte, 64)

	// Magic number
	copy(elfHeader[0:4], []byte{0x7f, 'E', 'L', 'F'})
	// Class: 64-bit
	elfHeader[4] = 2
	// Data: little endian
	elfHeader[5] = 1
	// Version
	elfHeader[6] = 1
	// OS/ABI
	elfHeader[7] = 0
	// Type: executable
	binary.LittleEndian.PutUint16(elfHeader[16:18], 2)
	// Machine: AArch64
	binary.LittleEndian.PutUint16(elfHeader[18:20], 183)
	// Version
	binary.LittleEndian.PutUint32(elfHeader[20:24], 1)
	// Entry point
	binary.LittleEndian.PutUint64(elfHeader[24:32], entryPoint)
	// Program header offset (right after ELF header)
	binary.LittleEndian.PutUint64(elfHeader[32:40], 64)
	// Section header offset (none)
	binary.LittleEndian.PutUint64(elfHeader[40:48], 0)
	// Flags
	binary.LittleEndian.PutUint32(elfHeader[48:52], 0)
	// ELF header size
	binary.LittleEndian.PutUint16(elfHeader[52:54], 64)
	// Program header entry size
	binary.LittleEndian.PutUint16(elfHeader[54:56], 56)
	// Number of program headers
	binary.LittleEndian.PutUint16(elfHeader[56:58], 1)
	// Section header entry size
	binary.LittleEndian.PutUint16(elfHeader[58:60], 64)
	// Number of section headers
	binary.LittleEndian.PutUint16(elfHeader[60:62], 0)
	// Section name string table index
	binary.LittleEndian.PutUint16(elfHeader[62:64], 0)

	// Program Header (56 bytes) - PT_LOAD
	progHeader := make([]byte, 56)
	// Type: PT_LOAD
	binary.LittleEndian.PutUint32(progHeader[0:4], 1)
	// Flags: PF_X | PF_R (readable + executable)
	binary.LittleEndian.PutUint32(progHeader[4:8], 0x5)
	// Offset in file (after headers)
	binary.LittleEndian.PutUint64(progHeader[8:16], 120)
	// Virtual address
	binary.LittleEndian.PutUint64(progHeader[16:24], loadAddr)
	// Physical address
	binary.LittleEndian.PutUint64(progHeader[24:32], loadAddr)
	// File size
	binary.LittleEndian.PutUint64(progHeader[32:40], uint64(len(code)))
	// Memory size
	binary.LittleEndian.PutUint64(progHeader[40:48], uint64(len(code)))
	// Alignment
	binary.LittleEndian.PutUint64(progHeader[48:56], 0x1000)

	// Write the ELF file
	file, _ := os.Create(path)
	defer func() { _ = file.Close() }()

	_, _ = file.Write(elfHeader)
	_, _ = file.Write(progHeader)
	_, _ = file.Write(code)
}

// createMinimalx86ELF creates a minimal x86-64 ELF to test rejection.
func createMinimalx86ELF(path string) {
	elfHeader := make([]byte, 64)

	copy(elfHeader[0:4], []byte{0x7f, 'E', 'L', 'F'})
	elfHeader[4] = 2                                    // 64-bit
	elfHeader[5] = 1                                    // little endian
	elfHeader[6] = 1                                    // version
	binary.LittleEndian.PutUint16(elfHeader[16:18], 2)  // executable
	binary.LittleEndian.PutUint16(elfHeader[18:20], 62) // x86-64
	binary.LittleEndian.PutUint32(elfHeader[20:24], 1)  // version
	binary.LittleEndian.PutUint64(elfHeader[24:32], 0)  // entry
	binary.LittleEndian.PutUint64(elfHeader[32:40], 64) // phoff
	binary.LittleEndian.PutUint16(elfHeader[52:54], 64) // ehsize
	binary.LittleEndian.PutUint16(elfHeader[54:56], 56) // phentsize
	binary.LittleEndian.PutUint16(elfHeader[56:58], 0)  // phnum

	file, _ := os.Create(path)
	defer func() { _ = file.Close() }()
	_, _ = file.Write(elfHeader)
}

// createMinimal32BitELF creates a minimal 32-bit ELF to test rejection.
func createMinimal32BitELF(path string) {
	elfHeader := make([]byte, 52)

	copy(elfHeader[0:4], []byte{0x7f, 'E', 'L', 'F'})
	elfHeader[4] = 1                                     // 32-bit (ELFCLASS32)
	elfHeader[5] = 1                                     // little endian
	elfHeader[6] = 1                                     // version
	binary.LittleEndian.PutUint16(elfHeader[16:18], 2)   // executable
	binary.LittleEndian.PutUint16(elfHeader[18:20], 183) // ARM64 (won't matter)
	binary.LittleEndian.PutUint32(elfHeader[20:24], 1)   // version

	file, _ := os.Create(path)
	defer func() { _ = file.Close() }()
	_, _ = file.Write(elfHeader)
}

// createMultiSegmentARM64ELF creates an ARM64 ELF with two PT_LOAD segments:
// a code segment (RX) and a data segment (RW).
func createMultiSegmentARM64ELF(path string, codeAddr, entryPoint uint64, code []byte, dataAddr uint64, data []byte) {
	// ELF Header (64 bytes)
	elfHeader := make([]byte, 64)

	copy(elfHeader[0:4], []byte{0x7f, 'E', 'L', 'F'})
	elfHeader[4] = 2                                     // 64-bit
	elfHeader[5] = 1                                     // little endian
	elfHeader[6] = 1                                     // version
	binary.LittleEndian.PutUint16(elfHeader[16:18], 2)   // executable
	binary.LittleEndian.PutUint16(elfHeader[18:20], 183) // AArch64
	binary.LittleEndian.PutUint32(elfHeader[20:24], 1)   // version
	binary.LittleEndian.PutUint64(elfHeader[24:32], entryPoint)
	binary.LittleEndian.PutUint64(elfHeader[32:40], 64) // phoff
	binary.LittleEndian.PutUint16(elfHeader[52:54], 64) // ehsize
	binary.LittleEndian.PutUint16(elfHeader[54:56], 56) // phentsize
	binary.LittleEndian.PutUint16(elfHeader[56:58], 2)  // phnum (2 segments)

	// Program Header 1: Code segment (RX)
	progHeader1 := make([]byte, 56)
	binary.LittleEndian.PutUint32(progHeader1[0:4], 1)                   // PT_LOAD
	binary.LittleEndian.PutUint32(progHeader1[4:8], 0x5)                 // PF_R | PF_X
	binary.LittleEndian.PutUint64(progHeader1[8:16], 64+56*2)            // offset
	binary.LittleEndian.PutUint64(progHeader1[16:24], codeAddr)          // vaddr
	binary.LittleEndian.PutUint64(progHeader1[24:32], codeAddr)          // paddr
	binary.LittleEndian.PutUint64(progHeader1[32:40], uint64(len(code))) // filesz
	binary.LittleEndian.PutUint64(progHeader1[40:48], uint64(len(code))) // memsz
	binary.LittleEndian.PutUint64(progHeader1[48:56], 0x1000)            // align

	// Program Header 2: Data segment (RW)
	progHeader2 := make([]byte, 56)
	binary.LittleEndian.PutUint32(progHeader2[0:4], 1)                          // PT_LOAD
	binary.LittleEndian.PutUint32(progHeader2[4:8], 0x6)                        // PF_R | PF_W
	binary.LittleEndian.PutUint64(progHeader2[8:16], 64+56*2+uint64(len(code))) // offset
	binary.LittleEndian.PutUint64(progHeader2[16:24], dataAddr)                 // vaddr
	binary.LittleEndian.PutUint64(progHeader2[24:32], dataAddr)                 // paddr
	binary.LittleEndian.PutUint64(progHeader2[32:40], uint64(len(data)))        // filesz
	binary.LittleEndian.PutUint64(progHeader2[40:48], uint64(len(data)))        // memsz
	binary.LittleEndian.PutUint64(progHeader2[48:56], 0x1000)                   // align

	file, _ := os.Create(path)
	defer func() { _ = file.Close() }()
	_, _ = file.Write(elfHeader)
	_, _ = file.Write(progHeader1)
	_, _ = file.Write(progHeader2)
	_, _ = file.Write(code)
	_, _ = file.Write(data)
}

// createBSSSegmentELF creates an ARM64 ELF with a BSS-like segment where Memsz > Filesz.
func createBSSSegmentELF(path string, segAddr, entryPoint uint64, data []byte, memSize uint64) {
	elfHeader := make([]byte, 64)

	copy(elfHeader[0:4], []byte{0x7f, 'E', 'L', 'F'})
	elfHeader[4] = 2                                     // 64-bit
	elfHeader[5] = 1                                     // little endian
	elfHeader[6] = 1                                     // version
	binary.LittleEndian.PutUint16(elfHeader[16:18], 2)   // executable
	binary.LittleEndian.PutUint16(elfHeader[18:20], 183) // AArch64
	binary.LittleEndian.PutUint32(elfHeader[20:24], 1)   // version
	binary.LittleEndian.PutUint64(elfHeader[24:32], entryPoint)
	binary.LittleEndian.PutUint64(elfHeader[32:40], 64) // phoff
	binary.LittleEndian.PutUint16(elfHeader[52:54], 64) // ehsize
	binary.LittleEndian.PutUint16(elfHeader[54:56], 56) // phentsize
	binary.LittleEndian.PutUint16(elfHeader[56:58], 1)  // phnum

	progHeader := make([]byte, 56)
	binary.LittleEndian.PutUint32(progHeader[0:4], 1)                   // PT_LOAD
	binary.LittleEndian.PutUint32(progHeader[4:8], 0x6)                 // PF_R | PF_W
	binary.LittleEndian.PutUint64(progHeader[8:16], 120)                // offset
	binary.LittleEndian.PutUint64(progHeader[16:24], segAddr)           // vaddr
	binary.LittleEndian.PutUint64(progHeader[24:32], segAddr)           // paddr
	binary.LittleEndian.PutUint64(progHeader[32:40], uint64(len(data))) // filesz
	binary.LittleEndian.PutUint64(progHeader[40:48], memSize)           // memsz > filesz
	binary.LittleEndian.PutUint64(progHeader[48:56], 0x1000)            // align

	file, _ := os.Create(path)
	defer func() { _ = file.Close() }()
	_, _ = file.Write(elfHeader)
	_, _ = file.Write(progHeader)
	_, _ = file.Write(data)
}

// createZeroFileszELF creates an ARM64 ELF with a segment that has zero Filesz but non-zero Memsz.
func createZeroFileszELF(path string, segAddr, entryPoint uint64, memSize uint64) {
	elfHeader := make([]byte, 64)

	copy(elfHeader[0:4], []byte{0x7f, 'E', 'L', 'F'})
	elfHeader[4] = 2                                     // 64-bit
	elfHeader[5] = 1                                     // little endian
	elfHeader[6] = 1                                     // version
	binary.LittleEndian.PutUint16(elfHeader[16:18], 2)   // executable
	binary.LittleEndian.PutUint16(elfHeader[18:20], 183) // AArch64
	binary.LittleEndian.PutUint32(elfHeader[20:24], 1)   // version
	binary.LittleEndian.PutUint64(elfHeader[24:32], entryPoint)
	binary.LittleEndian.PutUint64(elfHeader[32:40], 64) // phoff
	binary.LittleEndian.PutUint16(elfHeader[52:54], 64) // ehsize
	binary.LittleEndian.PutUint16(elfHeader[54:56], 56) // phentsize
	binary.LittleEndian.PutUint16(elfHeader[56:58], 1)  // phnum

	progHeader := make([]byte, 56)
	binary.LittleEndian.PutUint32(progHeader[0:4], 1)         // PT_LOAD
	binary.LittleEndian.PutUint32(progHeader[4:8], 0x6)       // PF_R | PF_W
	binary.LittleEndian.PutUint64(progHeader[8:16], 120)      // offset
	binary.LittleEndian.PutUint64(progHeader[16:24], segAddr) // vaddr
	binary.LittleEndian.PutUint64(progHeader[24:32], segAddr) // paddr
	binary.LittleEndian.PutUint64(progHeader[32:40], 0)       // filesz = 0
	binary.LittleEndian.PutUint64(progHeader[40:48], memSize) // memsz > 0
	binary.LittleEndian.PutUint64(progHeader[48:56], 0x1000)  // align

	file, _ := os.Create(path)
	defer func() { _ = file.Close() }()
	_, _ = file.Write(elfHeader)
	_, _ = file.Write(progHeader)
}

// createNoLoadableSegmentsELF creates an ARM64 ELF with no PT_LOAD segments (only PT_NOTE).
func createNoLoadableSegmentsELF(path string, entryPoint uint64) {
	elfHeader := make([]byte, 64)

	copy(elfHeader[0:4], []byte{0x7f, 'E', 'L', 'F'})
	elfHeader[4] = 2                                     // 64-bit
	elfHeader[5] = 1                                     // little endian
	elfHeader[6] = 1                                     // version
	binary.LittleEndian.PutUint16(elfHeader[16:18], 2)   // executable
	binary.LittleEndian.PutUint16(elfHeader[18:20], 183) // AArch64
	binary.LittleEndian.PutUint32(elfHeader[20:24], 1)   // version
	binary.LittleEndian.PutUint64(elfHeader[24:32], entryPoint)
	binary.LittleEndian.PutUint64(elfHeader[32:40], 64) // phoff
	binary.LittleEndian.PutUint16(elfHeader[52:54], 64) // ehsize
	binary.LittleEndian.PutUint16(elfHeader[54:56], 56) // phentsize
	binary.LittleEndian.PutUint16(elfHeader[56:58], 1)  // phnum (1 non-load segment)

	// PT_NOTE segment (type = 4), not PT_LOAD
	progHeader := make([]byte, 56)
	binary.LittleEndian.PutUint32(progHeader[0:4], 4)    // PT_NOTE (not PT_LOAD)
	binary.LittleEndian.PutUint32(progHeader[4:8], 0x4)  // PF_R
	binary.LittleEndian.PutUint64(progHeader[8:16], 120) // offset
	binary.LittleEndian.PutUint64(progHeader[16:24], 0)  // vaddr
	binary.LittleEndian.PutUint64(progHeader[24:32], 0)  // paddr
	binary.LittleEndian.PutUint64(progHeader[32:40], 0)  // filesz
	binary.LittleEndian.PutUint64(progHeader[40:48], 0)  // memsz
	binary.LittleEndian.PutUint64(progHeader[48:56], 4)  // align

	file, _ := os.Create(path)
	defer func() { _ = file.Close() }()
	_, _ = file.Write(elfHeader)
	_, _ = file.Write(progHeader)
}
