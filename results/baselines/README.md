# M2Sim Baseline Data Versioning Protocol

## Overview
This directory implements the baseline data versioning protocol designed to prevent false regression alarms and ensure measurement accuracy consistency (Issue #432).

## Directory Structure
```
baselines/
├── current/                    # Active baseline measurements
│   ├── accuracy_baseline_YYYY_MM_DD.json
│   └── cpi_baseline_YYYY_MM_DD.json
├── archive/                    # Historical baselines
│   └── [year]/[month]/
└── validation/                 # Integrity checks and protocols
    └── baseline_integrity_checks.json
```

## Baseline File Format

### Metadata Requirements
All baseline files must include:
- **creation_date**: Measurement date
- **commit_hash**: Git commit when measured
- **measurement_source**: Data collection method
- **timing_model_version**: Architecture version
- **validation_status**: Approval status
- **measurement_environment**: Configuration details

### Example Structure
```json
{
  "baseline_metadata": {
    "creation_date": "2026-02-11",
    "commit_hash": "54f60c09...",
    "timing_model_version": "post-PR-429",
    "validation_status": "approved"
  },
  "benchmark_accuracy": [...],
  "aggregate_metrics": {...}
}
```

## Usage Protocol

### 1. Baseline Updates
**Trigger baseline refresh when:**
- Major timing model changes (memory model, pipeline architecture)
- Hardware recalibration work
- Monthly refresh (first Monday of month)
- Accuracy improvements >5% achieved

### 2. Comparison Requirements
**For any accuracy analysis:**
- Specify exact baseline file used
- Include commit hash reference
- Document measurement environment
- Cross-validate with multiple tools

### 3. QA Validation
**Before merge approval:**
- Verify baseline currency (<30 days for major comparisons)
- Confirm environment consistency
- Validate measurement methodology

## Preventing False Alarms

### Issue #430 Prevention
The Issue #430 false regression occurred due to stale baseline comparison. This protocol prevents recurrence by:

1. **Mandatory baseline version specification**
2. **Automated staleness detection** (planned CI integration)
3. **Cross-validation requirements** (TestCPIComparison + accuracy_report.py)
4. **Environment metadata tracking**

### Quality Gates
- **PR Reviews**: Accuracy claims require baseline version
- **CI Integration**: Baseline freshness as check condition
- **Team Standards**: Protocol compliance in QA checklist

## Implementation Status
- ✅ Directory structure created
- ✅ Baseline format specification
- ✅ Initial baseline with PR #429 improvements
- ✅ Validation framework design
- 🔄 CI integration (planned)
- 🔄 Automated staleness detection (planned)

## Team Responsibilities

### QA Specialist (Diana)
- Validate baseline currency before regression analysis
- Enforce protocol compliance in PR reviews
- Monitor measurement accuracy and consistency

### Development Team
- Update baselines after major timing model changes
- Follow comparison requirements for accuracy claims
- Maintain environment metadata accuracy

### Strategy Team
- Monitor baseline staleness patterns
- Coordinate major baseline refresh cycles
- Evaluate protocol effectiveness

---
*Created: 2026-02-11 by Diana as part of Issue #432 implementation*