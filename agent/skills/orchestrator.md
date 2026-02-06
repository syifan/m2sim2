# Multi-Agent Dev Orchestrator (Dispatcher Model)

Lightweight dispatcher that runs every 1 minute. Each agent runs in its own isolated session with fresh context.

## How It Works

1. **Check if any agent is active** (via `*-active` labels)
2. **If active → exit silently** (let current agent finish)
3. **If idle → read `next:X` label** to determine next agent
4. **Spawn that agent** in isolated session
5. **Exit** (agent handles the rest)

## Execution

```bash
cd {{LOCAL_PATH}}
git pull --rebase

# Step 1: Check for active agents
ACTIVE=$(gh issue view {{TRACKER_ISSUE}} --json labels -q '.labels[].name' | grep -E '.*-active$' | head -1)

if [ -n "$ACTIVE" ]; then
  echo "Agent running: $ACTIVE — exiting silently"
  exit 0
fi

# Step 2: Get next agent from label
NEXT=$(gh issue view {{TRACKER_ISSUE}} --json labels -q '.labels[].name' | grep -E '^next:' | sed 's/next://')

if [ -z "$NEXT" ]; then
  NEXT="alice"  # Default to Alice if no next label
fi

echo "Dispatching: $NEXT"
```

## Dispatch Agent

Spawn the next agent in an **isolated session** using `sessions_spawn`:

```
sessions_spawn(
  task="Run as [AgentName]. Config: GitHub={{GITHUB_REPO}}, Local={{LOCAL_PATH}}, Tracker=#{{TRACKER_ISSUE}}, Skill={{SKILL_PATH}}. Read everyone.md and {agent}.md, then execute your full cycle. When done, update the next label.",
  model="claude-opus-4-5",
  runTimeoutSeconds=900
)
```

## Agent Sequence

```
Grace → Alice → Eric → Bob → Cathy → Dana → (loop to Grace)
```

Each agent updates the `next:X` label when completing their cycle:
- Grace → sets `next:alice`
- Alice → sets `next:eric`
- Eric → sets `next:bob`
- Bob → sets `next:cathy`
- Cathy → sets `next:dana`
- Dana → sets `next:grace`

## Prompt Template

```
You are the Multi-Agent Dispatcher.

**Config:**
- GitHub Repo: {{GITHUB_REPO}}
- Local Path: {{LOCAL_PATH}}
- Tracker Issue: #{{TRACKER_ISSUE}}
- Skill Path: {{SKILL_PATH}}

**Steps:**

1. cd {{LOCAL_PATH}} && git pull --rebase

2. Check active labels:
   gh issue view {{TRACKER_ISSUE}} --json labels -q '.labels[].name' | grep -E '.*-active$'
   
   If any agent is active → reply "Agent active, exiting" and STOP.

3. Get next agent:
   gh issue view {{TRACKER_ISSUE}} --json labels -q '.labels[].name' | grep -E '^next:' | sed 's/next://'
   
   Default to "alice" if no next label found.

4. Spawn that agent using sessions_spawn with isolated session:
   - Task: "Run as [AgentName]. Config: GitHub={{GITHUB_REPO}}, Local={{LOCAL_PATH}}, Tracker=#{{TRACKER_ISSUE}}, Skill={{SKILL_PATH}}. First read {{SKILL_PATH}}/references/agents/everyone.md, then read {{SKILL_PATH}}/references/agents/{agent}.md. Execute your full cycle. When done, update the next:X label."
   - Model: claude-opus-4-5
   - Timeout: 900 seconds

5. Exit after spawning.
```
