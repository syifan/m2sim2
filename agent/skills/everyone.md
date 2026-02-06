# Everyone — Shared Rules for All Agents

Read this file before executing any task.

---

## 1. Safety Rules

**Before ANY action**, verify you are in the correct repository. Must be `{{GITHUB_REPO}}`.

**If repo doesn't match, ABORT immediately.**

Do not:
- Run commands in wrong directories
- Push to wrong remotes
- Modify wrong tracker issues

When in doubt, **STOP and report the discrepancy**.

---

## 2. Context to Read

Before starting work, gather context from:

- **Messages for you** — check `messages/{your_name}.md` for guidance
- **Open issues and their comments** — look for messages directed at you (`→YourName:`)
- **Open PRs** — check for PRs awaiting review or action
- **Recent CI runs** — check for failures that need attention

---

## 3. GitHub Conventions

**All GitHub activity must be prefixed with your agent name in brackets.**

| Type | Format |
|------|--------|
| Issue title | `[AgentName] Description` |
| PR title | `[AgentName] Description` |
| Comments | `# [AgentName]` header |
| Commits | `[AgentName] Message` |
| Branch names | `agentname/description` |

**Addressing agents:**
- **FROM you:** `# [YourName]` (header style)
- **TO another agent:** `→AgentName:` (arrow prefix)

**Do NOT use @mentions** — they may notify real GitHub users.

---

## 4. Active Label

- **Start of cycle:** Add `active:{yourname}` label to tracker issue
- **End of cycle:** Remove `active:{yourname}` label

This signals to other agents and the orchestrator that you are working.

---

## 5. End of Cycle

When finishing your cycle:

1. **Comment on tracker** using this template:
```
# [AgentName]

**Input:** (issues, PRs, CI failures, messages you saw)

**Actions:** (what you did)
```

2. **Remove your active label** (see above)

---

## 6. Tips

- **Complete all assigned tasks** in a single cycle. Don't stop after just one.
- **Engage on issues** — share opinions, ask for clarification, respond to questions.
- **Be concise** — avoid verbose explanations. Get things done.
- **Pull before working** — always pull before making changes.
- **Small commits** — commit frequently with clear messages.
- **See something, say something** — if you find a problem, raise an issue.
