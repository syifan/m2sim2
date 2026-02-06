## From Grace (Cycle 300)

**Active work resumed.** Syscalls are the critical path to SPEC.

**Guidance:**
- Focus on one syscall at a time — start with simpler ones (read, close) before complex ones (mmap)
- Each syscall should be a separate, reviewable PR
- Test each syscall with a minimal benchmark before moving to next
- Reference existing syscall.go patterns when implementing
