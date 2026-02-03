# Feedback for Bob (Coder)

*Last updated: 2026-02-02 by Grace*

## Current Suggestions

- [ ] **URGENT**: Fix lint errors blocking PRs #48 and #49 (see details below)
- [ ] After lint is fixed, both PRs are approved and ready to merge
- [ ] Consider adding a `.golangci.yml` config to tune linter rules if too strict

## Specific Lint Errors to Fix

### errcheck violations (unchecked error returns)
```
loader/elf.go:59 - defer f.Close()
loader/elf_test.go - multiple f.Close(), file.Write(), os.RemoveAll()
timing/latency/latency_test.go:281 - os.RemoveAll(tempDir)
```
**Fix**: Use `defer func() { _ = f.Close() }()` or check and log errors

### unused functions (emu/ethan_validation_test.go)
```
ethanEncodeEORReg (line 516)
ethanEncodeLDR64Offset (line 559)
ethanEncodeSTR64Offset (line 576)
ethanEncodeB (line 593)
```
**Fix**: Remove or use these functions, or add `//nolint:unused` if planned for future

### goimports violation (loader/elf_test.go:9)
**Fix**: Run `goimports -w loader/elf_test.go`

## Observations

**What you're doing well:**
- Quality code in PRs - both got reviewer approval
- Good commit messages and PR descriptions
- Proactive CI fixes (golangci-lint version issue)

**Areas for improvement:**
- Run `golangci-lint run` locally before pushing
- The lint errors are pre-existing but now blocking your PRs

## Priority Guidance

1. Fix lint errors (can be done on main branch or in PR #48)
2. Get PRs merged
3. Then #23 (Integration test enhancements) is in your backlog
