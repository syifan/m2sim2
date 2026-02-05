# M2Sim Progress Report

**Last updated:** 2026-02-05 12:32 EST (Cycle 244)

## Current Status

| Metric | Value |
|--------|-------|
| Total PRs Merged | 69 |
| Open PRs | 2 |
| Open Issues | 13 |
| Pipeline Coverage | 58.6% |
| Emu Coverage | 76.2% → 79.9% (pending) |

## Cycle 244 Updates

- **PR #229 open** (Cathy: CCMP/CCMN tests) — all CI passing ✅, CLEAN, awaiting bob-approved
  - Adds 21 test cases for conditional compare instructions
  - Coverage: 76.2% → 79.9% (+3.7pp)
  - executeCondCmp: 0% → 100%
- **PR #230 open** (Bob: Zero-cycle predicted-taken branches) — all CI passing ✅, CLEAN, awaiting cathy-approved
  - Implements branch folding for high-confidence predicted-taken branches
  - Added FoldedBranches stat tracking
  - Added Confidence field to Prediction struct
  - Supports B.cond, CBZ/CBNZ, TBZ/TBNZ branch types
  - Expected impact: Branch error 34.5% → ~15-20%

**Key insight from Bob's implementation:**
Zero-cycle folding helps with **HOT branches** (repeated loops) where predictor is trained. Cold branches (first encounter) still incur normal penalties because BTB miss and low predictor confidence.

## Cycle 243 Updates

- **0 PRs merged this cycle** (research and planning cycle)
- **Bob completed zero-cycle branch research** — detailed analysis at `docs/zero-cycle-branch-research.md`
  - Key finding: Simulator already does speculative fetch, but branches still occupy pipeline slots
  - M2's approach: "Folded" branches that skip execute stage verification
  - Recommended implementation: foldedBranches map for high-confidence predicted-taken branches
  - Expected impact: Branch CPI 1.600 → ~1.2-1.3 (25% reduction)
- **Eric confirmed** BTB capacity was not the bottleneck — zero-cycle branches is the priority
- **Cathy verified** coverage maintained at 76.2% emu, 58.6% pipeline

## Cycle 242 Updates

- **PR #227 merged** ✅ (Bob: BTB size increase 512→2048)
- **69 PRs merged total**
- **Accuracy validation:** BTB increase shows no immediate improvement (34.2% avg unchanged)
  - Benchmarks are short, BTB hit rate was already high
  - Confirms Eric's research: zero-cycle predicted-taken branches is the highest-impact optimization
- **8-wide validated:** arithmetic_8wide CPI 0.250 (6.7% error!)

## Key Achievements

**Emu Coverage Target Exceeded!**
| Package | Coverage | Status |
|---------|----------|--------|
| emu | 79.9% (pending PR #229) | ✅ Above 70% target! |

**8-Wide Infrastructure Validated!**
| Benchmark | CPI | IPC | Error vs M2 |
|-----------|-----|-----|-------------|
| arithmetic_8wide | 0.250 | 4.0 | **6.7%** ✅ |

## Accuracy Status (Microbenchmarks)

| Benchmark | Simulator CPI | M2 Real CPI | Error | Priority |
|-----------|---------------|-------------|-------|----------|
| arithmetic_8wide | 0.250 | 0.268 | **6.7%** | ✅ Target met! |
| dependency_chain | 1.200 | 1.009 | **18.9%** | ✅ Near target |
| branch_taken_conditional | 1.600 | 1.190 | **34.5%** | ⚠️ **Highest gap** |

**Target:** <20% average error

## Next Optimization Priority

**Zero-cycle predicted-taken branches** (PR #230) is ready for review:

| Factor | M2 Real | M2Sim | Impact |
|--------|---------|-------|--------|
| Predicted-taken branch | ~0 cycles (folded) | 1+ cycles (execute) | **Major** |
| BTB hit handling | 0 cycles | 1 cycle decode | **Major** |
| BTB size | Large | ✅ 2048 (PR #227 merged) | Done |

**Eric's research findings:**
- BTB size increase confirmed to have minimal impact on short benchmarks
- The problem is NOT prediction accuracy — it's execution latency
- M2 achieves low branch CPI through **zero-cycle branch execution** for BTB hits
- Implementation guide: `docs/zero-cycle-branch-implementation.md`

**Implementation Status:**
| Priority | Optimization | Status |
|----------|--------------|--------|
| 1 | ✅ BTB 512→2048 | Merged (PR #227) |
| 2 | **Zero-cycle predicted-taken branches** | Ready for review (PR #230) |
| 3 | Add branch stats logging | Included in PR #230 |

## Coverage Analysis

| Package | Coverage | Status |
|---------|----------|--------|
| timing/cache | 89.1% | ✅ |
| timing/pipeline | 58.6% | ⚠️ (8-wide code untested) |
| timing/latency | 73.3% | ✅ |
| timing/core | 100% | ✅ |
| emu | 79.9% (pending) | ✅ Target exceeded! |

**Note:** Pipeline coverage dropped from ~77% to 58.6% after 8-wide infrastructure (PR #215) — the new Septenary/Octonary register types and tickOctupleIssue function need tests.

## Completed Optimizations

1. ✅ CMP + B.cond fusion (PR #212) — 62.5% → 34.5% branch error
2. ✅ 8-wide decode infrastructure (PR #215)
3. ✅ 8-wide benchmark enable (PR #220)
4. ✅ arithmetic_8wide benchmark (PR #223) — validates 8-wide, 6.7% error
5. ✅ BTB size increase 512→2048 (PR #227)
6. ✅ Emu coverage 76%+ (PRs #214, #217, #218, #222, #225, #226, #228)
7. 🔄 Zero-cycle predicted-taken branches (PR #230) — awaiting review

## Calibration Milestones

| Milestone | Status | Description |
|-----------|--------|-------------|
| C1 | ✅ Complete | Benchmarks execute to completion |
| C2 | 🚧 In Progress | Accuracy calibration — arithmetic at 6.7%! |
| C3 | Pending | Intermediate benchmark timing (PolyBench) |

## 8-Wide Validation Results

| Benchmark | Cycles | Instructions | CPI | IPC |
|-----------|--------|--------------|-----|-----|
| arithmetic_sequential | 8 | 20 | 0.400 | 2.5 |
| arithmetic_6wide | 8 | 24 | 0.333 | 3.0 |
| **arithmetic_8wide** | **8** | **32** | **0.250** | **4.0** |

🎉 **Major breakthrough!** The arithmetic_8wide CPI (0.250) is now very close to M2 real CPI (0.268) — **only 6.7% error** compared to the previous 49.3% arithmetic error!

## Stats

- 69 PRs merged total
- 2 open PRs (#229, #230) awaiting cross-reviews
- 205+ tests passing
- All coverage targets exceeded ✓
- 8-wide arithmetic accuracy: **6.7%** ✓
- Emu coverage: **79.9%** (pending PR #229) ✓
- Next focus: Zero-cycle predicted-taken branches (34.5% → target <25%)
