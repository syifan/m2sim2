package pipeline

import (
	"testing"
)

// Test isConditionalBranch (B.cond instruction detection)
func TestIsConditionalBranch(t *testing.T) {
	tests := []struct {
		name       string
		word       uint32
		pc         uint64
		wantMatch  bool
		wantTarget uint64
	}{
		{
			name:       "B.EQ forward offset",
			word:       0x54000040, // B.EQ +8 (imm19=2, cond=0)
			pc:         0x1000,
			wantMatch:  true,
			wantTarget: 0x1008,
		},
		{
			name:       "B.NE forward offset",
			word:       0x54000061, // B.NE +12 (imm19=3, cond=1)
			pc:         0x2000,
			wantMatch:  true,
			wantTarget: 0x200C,
		},
		{
			name:       "B.cond backward offset",
			word:       0x54FFFFC0, // B.EQ -8 (imm19=-2 sign extended)
			pc:         0x1000,
			wantMatch:  true,
			wantTarget: 0x0FF8,
		},
		{
			name:       "B.AL (always) condition",
			word:       0x5400002E, // B.AL +4 (imm19=1, cond=14)
			pc:         0x1000,
			wantMatch:  true,
			wantTarget: 0x1004,
		},
		{
			name:      "Not B.cond - wrong opcode",
			word:      0x14000001, // B (unconditional)
			pc:        0x1000,
			wantMatch: false,
		},
		{
			name:      "Not B.cond - bit 4 set",
			word:      0x54000050, // Would be B.cond but bit 4 is set
			pc:        0x1000,
			wantMatch: false,
		},
		{
			name:      "ADD instruction",
			word:      0x8B000000, // ADD X0, X0, X0
			pc:        0x1000,
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMatch, gotTarget := isConditionalBranch(tt.word, tt.pc)
			if gotMatch != tt.wantMatch {
				t.Errorf("isConditionalBranch() match = %v, want %v", gotMatch, tt.wantMatch)
			}
			if gotMatch && gotTarget != tt.wantTarget {
				t.Errorf("isConditionalBranch() target = 0x%X, want 0x%X", gotTarget, tt.wantTarget)
			}
		})
	}
}

// Test isCompareAndBranch (CBZ/CBNZ instruction detection)
func TestIsCompareAndBranch(t *testing.T) {
	tests := []struct {
		name       string
		word       uint32
		pc         uint64
		wantMatch  bool
		wantTarget uint64
	}{
		{
			name:       "CBZ X0 forward",
			word:       0xB4000040, // CBZ X0, +8 (64-bit, imm19=2)
			pc:         0x1000,
			wantMatch:  true,
			wantTarget: 0x1008,
		},
		{
			name:       "CBNZ X0 forward",
			word:       0xB5000060, // CBNZ X0, +12 (64-bit, imm19=3)
			pc:         0x2000,
			wantMatch:  true,
			wantTarget: 0x200C,
		},
		{
			name:       "CBZ W0 (32-bit)",
			word:       0x34000040, // CBZ W0, +8 (32-bit, imm19=2)
			pc:         0x1000,
			wantMatch:  true,
			wantTarget: 0x1008,
		},
		{
			name:       "CBNZ backward",
			word:       0xB5FFFFC0, // CBNZ X0, -8 (64-bit, imm19=-2)
			pc:         0x1000,
			wantMatch:  true,
			wantTarget: 0x0FF8,
		},
		{
			name:      "Not CBZ/CBNZ - wrong opcode",
			word:      0x54000040, // B.cond
			pc:        0x1000,
			wantMatch: false,
		},
		{
			name:      "B instruction",
			word:      0x14000001, // B (unconditional)
			pc:        0x1000,
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMatch, gotTarget := isCompareAndBranch(tt.word, tt.pc)
			if gotMatch != tt.wantMatch {
				t.Errorf("isCompareAndBranch() match = %v, want %v", gotMatch, tt.wantMatch)
			}
			if gotMatch && gotTarget != tt.wantTarget {
				t.Errorf("isCompareAndBranch() target = 0x%X, want 0x%X", gotTarget, tt.wantTarget)
			}
		})
	}
}

// Test isTestAndBranch (TBZ/TBNZ instruction detection)
func TestIsTestAndBranch(t *testing.T) {
	tests := []struct {
		name       string
		word       uint32
		pc         uint64
		wantMatch  bool
		wantTarget uint64
	}{
		{
			name:       "TBZ forward",
			word:       0x36000040, // TBZ X0, #0, +8 (imm14=2)
			pc:         0x1000,
			wantMatch:  true,
			wantTarget: 0x1008,
		},
		{
			name:       "TBNZ forward",
			word:       0x37000060, // TBNZ X0, #0, +12 (imm14=3)
			pc:         0x2000,
			wantMatch:  true,
			wantTarget: 0x200C,
		},
		{
			name:       "TBZ backward",
			word:       0x3607FFC0, // TBZ X0, #0, -8 (imm14=-2)
			pc:         0x1000,
			wantMatch:  true,
			wantTarget: 0x0FF8,
		},
		{
			name:       "TBZ with higher bit test",
			word:       0xB6100040, // TBZ X0, #32, +8 (b5=1, b40=2)
			pc:         0x1000,
			wantMatch:  true,
			wantTarget: 0x1008,
		},
		{
			name:      "Not TBZ/TBNZ - wrong opcode",
			word:      0x54000040, // B.cond
			pc:        0x1000,
			wantMatch: false,
		},
		{
			name:      "CBZ instruction",
			word:      0xB4000040, // CBZ
			pc:        0x1000,
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMatch, gotTarget := isTestAndBranch(tt.word, tt.pc)
			if gotMatch != tt.wantMatch {
				t.Errorf("isTestAndBranch() match = %v, want %v", gotMatch, tt.wantMatch)
			}
			if gotMatch && gotTarget != tt.wantTarget {
				t.Errorf("isTestAndBranch() target = 0x%X, want 0x%X", gotTarget, tt.wantTarget)
			}
		})
	}
}

// Test isFoldableConditionalBranch (combines all three checks)
func TestIsFoldableConditionalBranch(t *testing.T) {
	tests := []struct {
		name       string
		word       uint32
		pc         uint64
		wantMatch  bool
		wantTarget uint64
	}{
		{
			name:       "B.cond is foldable",
			word:       0x54000040, // B.EQ +8
			pc:         0x1000,
			wantMatch:  true,
			wantTarget: 0x1008,
		},
		{
			name:       "CBZ is foldable",
			word:       0xB4000040, // CBZ X0, +8
			pc:         0x1000,
			wantMatch:  true,
			wantTarget: 0x1008,
		},
		{
			name:       "CBNZ is foldable",
			word:       0xB5000040, // CBNZ X0, +8
			pc:         0x1000,
			wantMatch:  true,
			wantTarget: 0x1008,
		},
		{
			name:       "TBZ is foldable",
			word:       0x36000040, // TBZ X0, #0, +8
			pc:         0x1000,
			wantMatch:  true,
			wantTarget: 0x1008,
		},
		{
			name:       "TBNZ is foldable",
			word:       0x37000040, // TBNZ X0, #0, +8
			pc:         0x1000,
			wantMatch:  true,
			wantTarget: 0x1008,
		},
		{
			name:      "Unconditional B is not foldable",
			word:      0x14000001, // B +4
			pc:        0x1000,
			wantMatch: false,
		},
		{
			name:      "BL is not foldable",
			word:      0x94000001, // BL +4
			pc:        0x1000,
			wantMatch: false,
		},
		{
			name:      "BR is not foldable",
			word:      0xD61F0000, // BR X0
			pc:        0x1000,
			wantMatch: false,
		},
		{
			name:      "ADD is not foldable",
			word:      0x8B000000, // ADD X0, X0, X0
			pc:        0x1000,
			wantMatch: false,
		},
		{
			name:      "RET is not foldable",
			word:      0xD65F03C0, // RET
			pc:        0x1000,
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMatch, gotTarget := isFoldableConditionalBranch(tt.word, tt.pc)
			if gotMatch != tt.wantMatch {
				t.Errorf("isFoldableConditionalBranch() match = %v, want %v", gotMatch, tt.wantMatch)
			}
			if gotMatch && gotTarget != tt.wantTarget {
				t.Errorf("isFoldableConditionalBranch() target = 0x%X, want 0x%X", gotTarget, tt.wantTarget)
			}
		})
	}
}
