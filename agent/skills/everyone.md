# Everyone — Shared Rules for All Agents

Read this file before executing any task.

## 📬 Read Messages First

Before taking any action, read your messages file:

```bash
cat messages/{your_name}.md
```

Grace (Advisor) leaves high-level suggestions there. Consider her guidance when working.

## 📋 Tracker Issue Comments

When reading the agent tracker (issue #{{TRACKER_ISSUE}}), only read the **last 100 comments** at most:

```bash
gh issue view {{TRACKER_ISSUE}} --comments --json comments -q '.comments[-100:]'
```

Older comments are history — don't waste tokens reading them.

## ⚠️ SAFETY — Verify Repo First

**Before ANY action**, verify you are in the correct repository:

```bash
cd {{LOCAL_PATH}}
gh repo view --json nameWithOwner -q '.nameWithOwner'
# Must return: {{GITHUB_REPO}}
```

**If repo doesn't match `{{GITHUB_REPO}}`, ABORT immediately.**

Do not:
- Run commands in wrong directories
- Push to wrong remotes
- Modify wrong tracker issues

When in doubt, **STOP and report the discrepancy**.

## 📝 Naming Convention

**All GitHub activity must be prefixed with your agent name in brackets.**

| Type | Format | Example |
|------|--------|---------|
| Issue title | `[AgentName] Description` | `[Alice] Add caching feature` |
| PR title | `[AgentName] Description` | `[Bob] Implement caching` |
| Comments | `# [AgentName]` header | `# [Alice]\n## PM Cycle Complete` |
| Commits | `[AgentName] Message` | `[Bob] Fix memory leak` |
| Branch names | `agentname/description` | `bob/issue-42-caching` |

**Addressing agents (TO vs FROM):**
- **FROM agent:** `# [Eric]` (header style)
- **TO agent:** `→Eric:` (arrow prefix)

Example:
```
→Eric: please research this topic.
→Alice: prioritize benchmark creation.
```

**Do NOT use @mentions** — names like @Alice may notify real GitHub users.

This makes it clear who did what in the project history.

## 🔢 Action Count

**Only Alice increments the action count.** One action = one orchestrator round (all 6 phases).

Other agents should NOT modify the action count — read it, reference it, but leave updates to Alice.

## 🏷️ Active & Next Labels

**Start of cycle** — set active AND update next immediately:
```bash
# Remove your next label, add active label
gh issue edit {{TRACKER_ISSUE}} --remove-label "next:{you}" --add-label "active:{you}"

# Set next agent label immediately
gh issue edit {{TRACKER_ISSUE}} --add-label "next:{next-agent}"
```

During your run: `active:bob` + `next:cathy` (both exist)

**End of cycle** — comment and remove active:
```bash
gh issue comment {{TRACKER_ISSUE}} --body "# [AgentName]
## Cycle Complete

**Inputs noticed:**
- (list what you saw: new issues, comments, Grace messages, etc.)

**Actions:**
- (what you did)
..."

# Remove your active label (next label already set)
gh issue edit {{TRACKER_ISSUE}} --remove-label "active:{you}"
```

**Agent sequence:** Alice → Eric → Bob → Cathy → Dana → Grace → (loop)

| You | Set next: |
|-----|-----------|
| Alice | `next:eric` |
| Eric | `next:bob` |
| Bob | `next:cathy` |
| Cathy | `next:dana` |
| Dana | `next:grace` |
| Grace | `next:alice` |

**Always include "Inputs noticed"** — briefly report what you saw that helped you make decisions (new issues, comments, Grace's guidance, etc.). This adds transparency.

Replace `{agent}` with your lowercase name: `alice`, `bob`, `cathy`, `dana`, `eric`, `grace`.

## ✅ Complete All Assigned Tasks

If you have multiple tasks assigned, complete **all of them** in a single cycle (unless there are dependencies between them). Don't stop after just one task.

## 💬 Engage on Issues

You can (and should) comment on issues:
- Share your opinions or perspective
- Ask for clarification if something is unclear
- Respond to questions directed at you

**Read open issues AND their comments** each cycle:
```bash
# List open issues (excluding tracker #45)
gh issue list --state open --json number,title

# Read comments on relevant issues
gh issue view $ISSUE_NUM --comments
```

Check for:
- Messages directed at you (`→YourName:`)
- Questions relevant to your role
- Context that affects your work

If you see something relevant, respond or act on it.
