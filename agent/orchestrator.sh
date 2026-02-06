#!/usr/bin/env bash
# M2Sim Multi-Agent Orchestrator (Standalone)
# Runs agents in sequence using Claude CLI directly.
# No OpenClaw dependency.

set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
SKILL_PATH="$REPO_DIR/agent/skills"
TRACKER_ISSUE=45
INTERVAL_SECONDS=180  # 3 minutes
MODEL="claude-opus-4-5"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

get_action_count() {
    cd "$REPO_DIR"
    gh issue view "$TRACKER_ISSUE" --json body -q '.body' | grep -oP 'Action Count:\s*\K\d+' || echo "0"
}

get_next_agent() {
    cd "$REPO_DIR"
    local next_label
    next_label=$(gh issue view "$TRACKER_ISSUE" --json labels -q '.labels[].name' | grep -E '^next:' | sed 's/next://' | head -1)
    echo "${next_label:-alice}"
}

is_agent_active() {
    cd "$REPO_DIR"
    gh issue view "$TRACKER_ISSUE" --json labels -q '.labels[].name' | grep -qE '^active:'
}

get_active_agent() {
    cd "$REPO_DIR"
    gh issue view "$TRACKER_ISSUE" --json labels -q '.labels[].name' | grep -E '^active:' | sed 's/active://' | head -1
}

run_agent() {
    local agent="$1"
    local agent_file="$SKILL_PATH/${agent}.md"
    local everyone_file="$SKILL_PATH/everyone.md"
    
    log "Running agent: $agent"
    
    # Build the prompt
    local prompt="You are [$agent] working on the M2Sim project.

**Config:**
- GitHub Repo: sarchlab/m2sim  
- Local Path: $REPO_DIR
- Tracker Issue: #$TRACKER_ISSUE

**Instructions:**
1. First, read the shared rules from: $everyone_file
2. Then read your specific role from: $agent_file
3. Execute your full cycle as described in your role file.
4. At START of your work: remove label next:$agent, add label active:$agent, add next:{your-next-agent}
5. At END of your work: remove label active:$agent
6. All GitHub activity (commits, PRs, comments) must start with [$agent]

Work autonomously. Complete your cycle, then exit."

    # Run Claude CLI
    cd "$REPO_DIR"
    timeout 900 claude --model "$MODEL" --dangerously-skip-permissions "$prompt" 2>&1 | tee -a "$REPO_DIR/agent/logs/${agent}-$(date +%Y%m%d-%H%M%S).log" || {
        log "Agent $agent finished or timed out"
    }
}

clear_stale_label() {
    local agent="$1"
    log "Clearing stale active label for: $agent"
    cd "$REPO_DIR"
    gh issue edit "$TRACKER_ISSUE" --remove-label "active:$agent" 2>/dev/null || true
}

should_run_grace() {
    local count
    count=$(get_action_count)
    (( count % 10 == 0 && count > 0 ))
}

main() {
    log "M2Sim Orchestrator started (standalone mode)"
    log "Interval: ${INTERVAL_SECONDS}s, Repo: $REPO_DIR, Model: $MODEL"
    
    # Create logs directory
    mkdir -p "$REPO_DIR/agent/logs"
    
    while true; do
        log "--- Checking cycle ---"
        
        # Pull latest
        cd "$REPO_DIR"
        git pull --rebase --quiet 2>/dev/null || true
        
        # Check if an agent is active
        if is_agent_active; then
            local active_agent
            active_agent=$(get_active_agent)
            log "Agent '$active_agent' has active label, waiting..."
            # Note: Can't easily check if Claude CLI is running externally
            # Trust the label system; agents should clean up after themselves
        else
            # No active agent, spawn next one
            local next_agent
            next_agent=$(get_next_agent)
            
            # Check if Grace should run (every 10th cycle)
            if should_run_grace && [[ "$next_agent" == "alice" ]]; then
                log "Grace cycle (action count divisible by 10)"
                run_agent "grace"
            else
                run_agent "$next_agent"
            fi
        fi
        
        log "Sleeping ${INTERVAL_SECONDS}s..."
        sleep "$INTERVAL_SECONDS"
    done
}

# Handle signals for graceful shutdown
trap 'log "Shutting down..."; exit 0' SIGINT SIGTERM

main "$@"
