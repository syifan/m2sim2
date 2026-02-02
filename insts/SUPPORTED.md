# ARM64 Instruction Support Status

This document tracks the ARM64 instructions supported by M2Sim's decoder.

## Decoder Support

### Data Processing (Immediate)

| Instruction | Description | Decoder | Emulator |
|-------------|-------------|---------|----------|
| ADD (imm)   | Add with immediate | ✅ | ✅ |
| ADDS (imm)  | Add with immediate, set flags | ✅ | ✅ |
| SUB (imm)   | Subtract with immediate | ✅ | ✅ |
| SUBS (imm)  | Subtract with immediate, set flags | ✅ | ✅ |

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
| B           | Unconditional branch | ✅ | ❌ |
| BL          | Branch with link | ✅ | ❌ |
| B.cond      | Conditional branch | ✅ | ❌ |
| BR          | Branch to register | ✅ | ❌ |
| BLR         | Branch with link to register | ✅ | ❌ |
| RET         | Return from subroutine | ✅ | ❌ |

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

---

*Last updated: Issue #2 implementation*
