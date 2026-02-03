// Package loader provides ELF binary loading for ARM64 executables.
package loader

import (
	"debug/elf"
	"fmt"
	"io"
)

// SegmentFlags represents memory protection flags for a segment.
type SegmentFlags uint32

const (
	// SegmentFlagExecute indicates the segment is executable.
	SegmentFlagExecute SegmentFlags = 1 << iota
	// SegmentFlagWrite indicates the segment is writable.
	SegmentFlagWrite
	// SegmentFlagRead indicates the segment is readable.
	SegmentFlagRead
)

// DefaultStackTop is the default stack top address for ARM64 Linux user space.
// This is a conventional high address in the user space address range.
const DefaultStackTop = 0x7ffffffff000

// DefaultStackSize is the default stack size (8MB).
const DefaultStackSize = 8 * 1024 * 1024

// Segment represents a loadable segment from an ELF binary.
type Segment struct {
	// VirtAddr is the virtual address where this segment should be loaded.
	VirtAddr uint64
	// Data contains the segment contents from the file.
	Data []byte
	// MemSize is the size in memory (may be larger than len(Data) for BSS).
	MemSize uint64
	// Flags contains the segment protection flags.
	Flags SegmentFlags
}

// Program represents a loaded ELF program ready for execution.
type Program struct {
	// EntryPoint is the virtual address where execution should begin.
	EntryPoint uint64
	// Segments contains all loadable segments from the ELF file.
	Segments []Segment
	// InitialSP is the initial stack pointer value.
	InitialSP uint64
}

// Load parses an ARM64 ELF binary and returns a Program struct ready for
// loading into the emulator's memory.
func Load(path string) (*Program, error) {
	// Open the ELF file
	f, err := elf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open ELF file: %w", err)
	}
	defer func() { _ = f.Close() }()

	// Validate ELF class (must be 64-bit)
	if f.Class != elf.ELFCLASS64 {
		return nil, fmt.Errorf("not a 64-bit ELF file")
	}

	// Validate machine type (must be ARM64/AArch64)
	if f.Machine != elf.EM_AARCH64 {
		return nil, fmt.Errorf("not an ARM64 ELF file (machine type: %v)", f.Machine)
	}

	// Create the program structure
	prog := &Program{
		EntryPoint: f.Entry,
		InitialSP:  DefaultStackTop,
	}

	// Load all PT_LOAD segments
	for _, phdr := range f.Progs {
		if phdr.Type != elf.PT_LOAD {
			continue
		}

		// Read segment data
		data := make([]byte, phdr.Filesz)
		if phdr.Filesz > 0 {
			n, err := phdr.ReadAt(data, 0)
			if err != nil && err != io.EOF {
				return nil, fmt.Errorf("failed to read segment at 0x%x: %w", phdr.Vaddr, err)
			}
			if uint64(n) != phdr.Filesz {
				return nil, fmt.Errorf("short read for segment at 0x%x: got %d bytes, expected %d",
					phdr.Vaddr, n, phdr.Filesz)
			}
		}

		// Convert ELF flags to our segment flags
		var flags SegmentFlags
		if phdr.Flags&elf.PF_X != 0 {
			flags |= SegmentFlagExecute
		}
		if phdr.Flags&elf.PF_W != 0 {
			flags |= SegmentFlagWrite
		}
		if phdr.Flags&elf.PF_R != 0 {
			flags |= SegmentFlagRead
		}

		seg := Segment{
			VirtAddr: phdr.Vaddr,
			Data:     data,
			MemSize:  phdr.Memsz,
			Flags:    flags,
		}

		prog.Segments = append(prog.Segments, seg)
	}

	return prog, nil
}
