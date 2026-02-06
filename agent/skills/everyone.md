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

- **Messages from Grace** — check `messages/{your_name}.md` for guidance
- **Open issues and their comments** — look for messages directed at you (`→YourName:`)

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

**Do NOT use @mentions** — they may notify real GitHub users.

---

## 4. End of Cycle

When finishing your cycle:

1. **Comment on tracker** with what you noticed and what you did
2. **Remove your active label**

---

## 5. Tips

- **Complete all assigned tasks** in a single cycle. Don't stop after just one.
- **Engage on issues** — share opinions, ask for clarification, respond to questions.
- **Be concise** — avoid verbose explanations. Get things done.
- **Pull before working** — always pull before making changes.
- **Small commits** — commit frequently with clear messages.
