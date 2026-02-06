# Everyone — Shared Rules for All Agents

Read this file before executing any task.

---

## 1. Safety Rules

**Before ANY action**, verify you are in the correct repository:

```bash
cd {{LOCAL_PATH}}
gh repo view --json nameWithOwner -q '.nameWithOwner'
# Must return: {{GITHUB_REPO}}
```

**If repo doesn't match, ABORT immediately.**

Do not:
- Run commands in wrong directories
- Push to wrong remotes
- Modify wrong tracker issues

When in doubt, **STOP and report the discrepancy**.

---

## 2. Context to Read

Before starting work, gather context from these sources:

### Messages from Grace
```bash
cat messages/{your_name}.md
```
Grace (Advisor) leaves high-level suggestions here. Consider her guidance.

### Open Issues and Their Comments
```bash
# List open issues (excluding tracker)
gh issue list --state open --json number,title

# Read comments on relevant issues
gh issue view $ISSUE_NUM --comments
```

Check for:
- Messages directed at you (`→YourName:`)
- Questions relevant to your role
- Context that affects your work

---

## 3. GitHub Conventions

**All GitHub activity must be prefixed with your agent name in brackets.**

| Type | Format | Example |
|------|--------|---------|
| Issue title | `[AgentName] Description` | `[Alice] Add caching feature` |
| PR title | `[AgentName] Description` | `[Bob] Implement caching` |
| Comments | `# [AgentName]` header | `# [Alice]\n## PM Cycle` |
| Commits | `[AgentName] Message` | `[Bob] Fix memory leak` |
| Branch names | `agentname/description` | `bob/issue-42-caching` |

**Addressing agents:**
- **FROM you:** `# [YourName]` (header style)
- **TO another agent:** `→AgentName:` (arrow prefix)

Example:
```
→Eric: please research this topic.
→Alice: prioritize benchmark creation.
```

**Do NOT use @mentions** — they may notify real GitHub users.

---

## 4. End of Cycle

When finishing your cycle:

**1. Comment on tracker:**
```bash
gh issue comment {{TRACKER_ISSUE}} --body "# [AgentName]
## Cycle Complete

**Inputs noticed:**
- (what you saw: issues, comments, Grace messages)

**Actions:**
- (what you did)
"
```

**2. Remove your active label:**
```bash
gh issue edit {{TRACKER_ISSUE}} --remove-label "active:{you}"
```

---

## 5. Tips

- **Complete all assigned tasks** in a single cycle (unless there are dependencies). Don't stop after just one.
- **Engage on issues** — share opinions, ask for clarification, respond to questions directed at you.
- **Be concise** — avoid verbose explanations. Get things done.
- **Pull before working** — always `git pull --rebase` before making changes.
- **Small commits** — commit frequently with clear messages.
