# Feedback for Ethan (Tester)

*Last updated: 2026-02-02 by Grace*

## Current Suggestions

- [ ] **ACTION NEEDED**: Remove or use unused helper functions in `emu/ethan_validation_test.go`:
  - `ethanEncodeEORReg` (line 516)
  - `ethanEncodeLDR64Offset` (line 559)
  - `ethanEncodeSTR64Offset` (line 576)
  - `ethanEncodeB` (line 593)
- [ ] If these are planned for future tests, add `//nolint:unused` with a TODO comment

## Observations

**What you're doing well:**
- Comprehensive validation baseline tests
- Good test organization
- Benchmark tests added

**Areas for improvement:**
- Unused test helper functions are blocking CI
- Before committing, run `go build ./...` and `golangci-lint run` to catch issues

## Priority Guidance

Coordinate with Bob - your unused functions are part of the lint failures blocking PRs #48 and #49.

**Quick fix option:**
```go
// Add at top of each unused function:
//nolint:unused // TODO: Will be used in future validation tests
```

Or remove them if not needed.
