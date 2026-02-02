# AGENTS.md - Multi-Agent Development Coordination

This project is developed by a team of AI agents coordinated through GitHub.

## Project Goal

Build a cycle-accurate Apple M2 CPU simulator using the Akita simulation framework. The simulator should:

1. **Functional emulation**: Execute ARM64 user-space programs correctly
2. **Timing model**: Predict execution time with <2% average error
3. **Modular design**: Separate functional and timing simulation
4. **Incremental**: Add features gradually, starting with basic instructions

### Target Specifications (Apple M2)
- ARM64 ISA (user space only)
- Single-core MVP, multi-core later
- Pipeline stages, branch prediction, cache hierarchy (L1/L2/SLC)
- Benchmarks: μs to ms range, shorter programs first

### Reference Projects
- Akita: https://github.com/sarchlab/akita (simulation engine)
- MGPUSim: https://github.com/sarchlab/mgpusim (architecture reference)

## Team Roles

### Alice (Project Manager)
- Breaks down requirements into milestones
- Creates issues for each feature
- Assigns work by labeling issues `ready-for-bob`
- Reviews PRs and merges if approved by Cathy and Dylan
- Declares project complete when MVP is done

### Bob (Coder)
- Picks up issues labeled `ready-for-bob`
- Writes tests first, then implementation
- Creates PRs linking to issues
- Addresses review feedback

### Cathy (Code Quality Reviewer)
- Reviews PRs for code style, naming, structure
- Ensures Go best practices and Akita patterns
- Approves or requests changes
- Adds label `cathy-approved` when satisfied

### Dylan (Logic Reviewer)
- Reviews PRs for correctness, edge cases, test coverage
- Validates ARM64 instruction semantics
- Checks timing model accuracy
- Adds label `dylan-approved` when satisfied

### Ethan (User Tester)
- Runs the simulator with test programs
- Compares results with real M2 execution
- Files issues for bugs (`bug` label)
- Reports accuracy metrics

### Frank (Documentation Writer)
- Writes README and architecture docs
- Documents instruction support status
- Creates usage examples

### Grace (Advisor)
- Reviews process every 30 actions
- Checks for duplicate work
- Recommends improvements or completion
- Can update this file with project-specific rules

## Naming Convention

All GitHub issues, PRs, and comments must be prefixed with the agent name:
- **Issue titles:** `[Alice] Implement basic ALU instructions`
- **PR titles:** `[Bob] Add ADD/SUB instruction support`
- **Comments:** Start with `**[AgentName]:** `

## Labels

- `ready-for-bob` — Issue ready for implementation
- `ready-for-review` — PR ready for Cathy & Dylan
- `cathy-approved` — Cathy approved the PR
- `dylan-approved` — Dylan approved the PR
- `needs-changes` — Reviewers requested changes
- `bug` — Bug found by Ethan
- `enhancement` — Enhancement suggested
- `documentation` — Doc task for Frank

## Directory Structure

```
m2sim/
├── emu/           # Functional ARM64 emulation
│   ├── alu.go     # ALU operations
│   ├── branch.go  # Branch instructions
│   ├── load_store.go
│   └── ...
├── timing/        # Cycle-accurate CPU model
│   ├── core/      # Pipeline, scheduling
│   ├── cache/     # Cache hierarchy
│   └── mem/       # Memory controller
├── insts/         # ARM64 instruction definitions
│   ├── decoder.go
│   ├── format.go
│   └── ...
├── driver/        # OS service emulation
│   └── syscall.go
├── benchmarks/    # Test programs (C source + binaries)
└── samples/       # Runnable simulation examples
```

## Milestones

### M1: Foundation (MVP)
- [ ] Project setup (Go module, Akita dependency)
- [ ] ARM64 instruction decoder (basic formats)
- [ ] Basic ALU instructions (ADD, SUB, AND, OR, etc.)
- [ ] Register file (X0-X30, SP, PC)
- [ ] Simple test program execution

### M2: Memory & Control Flow
- [ ] Load/Store instructions
- [ ] Branch instructions (B, BL, BR, conditional)
- [ ] Basic syscall emulation (exit, write)
- [ ] Memory model (simple, no cache)

### M3: Timing Model
- [ ] Pipeline stages (Fetch, Decode, Execute, Memory, Writeback)
- [ ] Simple timing for each instruction
- [ ] Execution time prediction

### M4: Cache Hierarchy
- [ ] L1 instruction cache
- [ ] L1 data cache
- [ ] L2 unified cache
- [ ] Cache timing model

### M5: Advanced Features
- [ ] Branch prediction
- [ ] Out-of-order execution (if needed for accuracy)
- [ ] More ARM64 instructions (SIMD basics)

### M6: Validation & Benchmarks
- [ ] Run standard benchmarks
- [ ] Compare with real M2 timing
- [ ] Achieve <2% average error

## Workflow

```
Alice creates milestone → Alice creates issues → 
Bob codes & submits PR → Cathy & Dylan review → 
Alice merges → Ethan tests → (repeat until done)
Frank writes docs throughout
Grace reviews every 30 actions
```

## Activity Logging (Grace Rule #1)

**Every orchestrator action MUST be logged to ACTIVITY_LOG.md** with:
- Action number
- Timestamp
- Agent name
- Action taken
- Result (SUCCESS/BLOCKED/FAILED)
- Relevant PR/Issue numbers

This ensures audit trail continuity for process reviews. Missing entries break historical analysis.

## Completion Criteria

Project is complete when:
1. All M1-M4 milestone issues are closed (MVP)
2. Ethan confirms <2% average timing error on target benchmarks
3. Documentation is complete
4. Alice creates `[Alice] Project Complete` issue
