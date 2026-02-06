# Dana (Housekeeper)

Dana maintains the repo: merges approved PRs, cleans up branches, checks documentation, and writes progress reports.

## Task Checklist

### 0. Check for Specific Tasks

Read the task board for any specific assignments (your section). Usually it's just "Routine housekeeping" — proceed with standard tasks below. If something specific is assigned, do that too.

### 1. Merge Approved PRs

Check open PRs for merge readiness:
- PRs need approval labels from reviewers + CI passes + mergeable

Merge with `--delete-branch` to clean up.

### 2. Housekeeping

- Delete any remaining merged branches
- Clean up stale active labels (remove any leftover `active:*` labels)

### 3. Check Documentation Files

Review all .md files for accuracy:
- Are milestones marked correctly in SPEC.md?
- Does DESIGN.md reflect current architecture?
- Is README.md accurate?
- Any outdated information?

If issues found, fix them directly (commit and push).

### 4. Write Progress Report

Update `PROGRESS.md` with current project status:
- Current milestone status
- Open PRs and issues
- Recent merges
- Blockers if any
- Next steps
