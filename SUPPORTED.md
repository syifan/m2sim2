# ARM64 Instruction Support Status

This document tracks ARM64 instructions and syscalls supported by M2Sim.

## Supported Instructions

### Data Processing (Immediate)

| Instruction | Description | Decoder | Emulator |
|-------------|-------------|---------|----------|
| ADD (imm)   | Add with immediate | ✅ | ✅ |
| ADDS (imm)  | Add with immediate, set flags | ✅ | ✅ |
| SUB (imm)   | Subtract with immediate | ✅ | ✅ |
| SUBS (imm)  | Subtract with immediate, set flags | ✅ | ✅ |

### Logical (Immediate)

| Instruction | Description | Decoder | Emulator |
|-------------|-------------|---------|----------|
| AND (imm)   | Bitwise AND with bitmask immediate | ✅ | ✅ |
| ANDS (imm)  | Bitwise AND with bitmask immediate, set flags | ✅ | ✅ |
| ORR (imm)   | Bitwise OR with bitmask immediate | ✅ | ✅ |
| EOR (imm)   | Bitwise XOR with bitmask immediate | ✅ | ✅ |

### Data Processing (Register)

| Instruction | Description | Decoder | Emulator |
|-------------|-------------|---------|----------|
| ADD (reg)   | Add registers | ✅ | ✅ |
| ADDS (reg)  | Add registers, set flags | ✅ | ✅ |
| SUB (reg)   | Subtract registers | ✅ | ✅ |
| SUBS (reg)  | Subtract registers, set flags | ✅ | ✅ |
| AND (reg)   | Bitwise AND | ✅ | ✅ |
| ANDS (reg)  | Bitwise AND, set flags | ✅ | ✅ |
| ORR (reg)   | Bitwise OR | ✅ | ✅ |
| EOR (reg)   | Bitwise XOR | ✅ | ✅ |

### Branch Instructions

| Instruction | Description | Decoder | Emulator |
|-------------|-------------|---------|----------|
| B           | Unconditional branch | ✅ | ✅ |
| BL          | Branch with link | ✅ | ✅ |
| B.cond      | Conditional branch | ✅ | ✅ |
| BR          | Branch to register | ✅ | ✅ |
| BLR         | Branch with link to register | ✅ | ✅ |
| RET         | Return from subroutine | ✅ | ✅ |

### Exception Generation

| Instruction | Description | Decoder | Emulator |
|-------------|-------------|---------|----------|
| SVC         | Supervisor call (syscall trigger) | ✅ | ✅ |

### Load/Store Instructions

| Instruction | Description | Decoder | Emulator |
|-------------|-------------|---------|----------|
| LDR (imm, 64-bit) | Load 64-bit register | ✅ | ✅ |
| LDR (imm, 32-bit) | Load 32-bit register (zero-extend) | ✅ | ✅ |
| STR (imm, 64-bit) | Store 64-bit register | ✅ | ✅ |
| STR (imm, 32-bit) | Store 32-bit register | ✅ | ✅ |

### SIMD Integer Instructions

| Instruction | Description | Decoder | Emulator |
|-------------|-------------|---------|----------|
| VADD        | Vector add | ✅ | ✅ |
| VSUB        | Vector subtract | ✅ | ✅ |
| VMUL        | Vector multiply | ✅ | ✅ |

### SIMD Floating-Point Instructions

| Instruction | Description | Decoder | Emulator |
|-------------|-------------|---------|----------|
| VFADD       | Vector FP add | ✅ | ✅ |
| VFSUB       | Vector FP subtract | ✅ | ✅ |
| VFMUL       | Vector FP multiply | ✅ | ✅ |

### SIMD Load/Store Instructions

| Instruction | Description | Decoder | Emulator |
|-------------|-------------|---------|----------|
| LDR Q       | Load 128-bit vector register | ✅ | ✅ |
| STR Q       | Store 128-bit vector register | ✅ | ✅ |

### SIMD Copy Instructions

| Instruction | Description | Decoder | Emulator |
|-------------|-------------|---------|----------|
| DUP         | Duplicate scalar to vector | ✅ | ✅ |

### System Instructions

| Instruction | Description | Decoder | Emulator |
|-------------|-------------|---------|----------|
| MRS         | Move from system register | ✅ | ✅ |

**Supported System Registers:**
- **DCZID_EL0**: Data Cache Zero ID register - Returns cache line size information (64-byte cache lines)

## Supported Syscalls

The driver package emulates ARM64 Linux syscalls:

| Syscall | Number | Description |
|---------|--------|-------------|
| exit    | 93     | Terminate program with exit code |
| exit_group | 94  | Terminate program with exit code (all threads) |
| write   | 64     | Write buffer to file descriptor |

### Syscall Convention (ARM64 Linux)
- Syscall number in X8
- Arguments in X0-X5
- Return value in X0

## Condition Codes Supported

| Code | Meaning | Condition |
|------|---------|-----------|
| EQ | Equal | Z == 1 |
| NE | Not equal | Z == 0 |
| CS/HS | Carry set / Unsigned higher or same | C == 1 |
| CC/LO | Carry clear / Unsigned lower | C == 0 |
| MI | Minus / Negative | N == 1 |
| PL | Plus / Positive or zero | N == 0 |
| VS | Overflow | V == 1 |
| VC | No overflow | V == 0 |
| HI | Unsigned higher | C == 1 && Z == 0 |
| LS | Unsigned lower or same | C == 0 || Z == 1 |
| GE | Signed greater than or equal | N == V |
| LT | Signed less than | N != V |
| GT | Signed greater than | Z == 0 && N == V |
| LE | Signed less than or equal | Z == 1 || N != V |
| AL | Always | (unconditional) |

## Instruction Formats Supported

- **FormatDPImm**: Data Processing with Immediate
- **FormatDPReg**: Data Processing with Register
- **FormatBranch**: Unconditional Branch (Immediate)
- **FormatBranchCond**: Conditional Branch
- **FormatBranchReg**: Branch to Register
- **FormatLoadStore**: Load/Store with Immediate Offset
- **FormatSIMDReg**: SIMD Data Processing (Register)
- **FormatSIMDLoadStore**: SIMD Load/Store
- **FormatSIMDCopy**: SIMD Copy (DUP, MOV, etc.)
- **FormatSystemReg**: System Register Operations (MRS, MSR)

## Known Limitations

### Logical Register Operations - N-bit Not Handled

**Issue:** The N-bit (bit 21) in logical register instructions is not currently
decoded. This bit controls whether the second operand (Rm) is inverted before
the operation.

**Affected Instructions:**
- **BIC** (Bitwise Bit Clear) - decoded incorrectly as AND
- **ORN** (Bitwise OR NOT) - decoded incorrectly as ORR  
- **EON** (Bitwise Exclusive OR NOT) - decoded incorrectly as EOR

**Encoding Reference:**
```
Logical (shifted register): sf | opc | 01010 | shift | N | Rm | imm6 | Rn | Rd
                                                       ^
                                                   bit 21 (N)
```

When N=1, the Rm value should be bitwise inverted (~Rm) before the logical
operation is applied. The current decoder ignores this bit.

**Status:** Not implemented. Tracked for future work.

### Missing Test Coverage

The following areas lack test coverage:

1. **Shifted register operands** - No tests verify correct decoding of shift
   type (LSL, LSR, ASR, ROR) and shift amount for register operands in
   ADD/SUB/logical instructions.

---

*Last updated: 2026-02-07*
*Consolidated from root SUPPORTED.md and insts/SUPPORTED.md*
