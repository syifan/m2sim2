# Research: Long-Running Job Execution for Autonomous Agents

**Author:** Eric (Researcher)  
**Date:** 2026-02-06  
**Related Issue:** #224

## Executive Summary

This research addresses how agents can autonomously execute long-running jobs (SPEC benchmarks, validation suites) that exceed session timeouts. We evaluate four approaches and recommend a **hybrid solution** combining GitHub Actions for scheduled/heavy workloads with local background processes for quick iterations.

---

## Problem Statement

Current limitations:
- Agent sessions timeout after ~30 minutes of inactivity
- SPEC benchmarks can run for hours
- Cross-compilation builds take 10-30 minutes
- No mechanism for agents to "fire and forget" then check results later

---

## Approach 1: Local Background Processes

### Mechanisms

**nohup + output redirection:**
```bash
nohup ./run-benchmark.sh > results/job-$(date +%s).log 2>&1 &
echo $! > results/job.pid
```

**screen/tmux detached sessions:**
```bash
screen -dmS spec-run ./run-benchmark.sh
# Later: screen -r spec-run (to attach) or screen -ls (to list)
```

### Status Checking

```bash
# Check if process is still running
if kill -0 $(cat results/job.pid) 2>/dev/null; then
    echo "Still running"
else
    echo "Complete - check results/job-*.log"
fi
```

### Pros
- Simple, no external dependencies
- Works on any Unix system
- Low overhead

### Cons
- Requires persistent machine (not suitable for ephemeral environments)
- No built-in notification system
- Agent must poll for completion
- No automatic retries on failure

### Recommendation
**Good for:** Quick local iterations, development testing  
**Not for:** Production accuracy runs, overnight validation

---

## Approach 2: GitHub Actions

### Workflow Types

**On-demand (workflow_dispatch):**
```yaml
name: Run SPEC Benchmark
on:
  workflow_dispatch:
    inputs:
      benchmark:
        description: 'Benchmark to run'
        required: true
        default: 'all'
        type: choice
        options:
          - all
          - 500.perlbench_r
          - 502.gcc_r
          # ... etc

jobs:
  benchmark:
    runs-on: [self-hosted, m2-chip]  # Requires M2 runner
    timeout-minutes: 360  # 6 hours
    steps:
      - uses: actions/checkout@v4
      - name: Run benchmark
        run: ./scripts/run-spec.sh ${{ inputs.benchmark }}
      - uses: actions/upload-artifact@v4
        with:
          name: benchmark-results
          path: results/
          retention-days: 30
```

**Scheduled (cron):**
```yaml
on:
  schedule:
    - cron: '0 2 * * *'  # Daily at 2 AM
```

### Agent Triggering

```bash
# Start workflow
gh workflow run spec-benchmark.yml -f benchmark=500.perlbench_r

# Check status
gh run list --workflow=spec-benchmark.yml --limit=1

# Wait for completion and download artifacts
gh run download $(gh run list --workflow=spec-benchmark.yml -L1 --json databaseId -q '.[0].databaseId')
```

### Pros
- Managed infrastructure
- Automatic retries
- Artifact storage
- Status notifications
- Works across machine restarts

### Cons
- Requires self-hosted runner for M2 hardware
- Setup complexity
- Limited free minutes (2000/month for private repos)
- Cold start latency

### Self-Hosted Runner Setup

For M2 baseline capture, we need a **self-hosted runner** on M2 hardware:

```bash
# On M2 Mac:
mkdir actions-runner && cd actions-runner
curl -O -L https://github.com/actions/runner/releases/download/v2.xxx/actions-runner-osx-arm64-xxx.tar.gz
tar xzf ./actions-runner-osx-arm64-xxx.tar.gz
./config.sh --url https://github.com/sarchlab/m2sim --token <TOKEN>
./run.sh  # Or: ./svc.sh install && ./svc.sh start
```

### Recommendation
**Good for:** Production runs, scheduled validation, SPEC benchmarks  
**Not for:** Quick iteration during development

---

## Approach 3: Simple Job Queue (File-Based)

### Concept

Agents write jobs to a queue file; a cron job processes them.

**Queue file format (jobs/queue.jsonl):**
```json
{"id": "job-1707184800", "command": "./run-spec.sh 500.perlbench_r", "status": "pending", "created": "2026-02-06T06:00:00Z"}
```

**Cron processor (scripts/job-processor.sh):**
```bash
#!/bin/bash
QUEUE="jobs/queue.jsonl"
while read -r job; do
    id=$(echo "$job" | jq -r '.id')
    cmd=$(echo "$job" | jq -r '.command')
    
    # Mark as running
    sed -i "s/\"id\": \"$id\", \"status\": \"pending\"/\"id\": \"$id\", \"status\": \"running\"/" "$QUEUE"
    
    # Execute
    eval "$cmd" > "jobs/$id.log" 2>&1
    
    # Mark complete
    sed -i "s/\"id\": \"$id\", \"status\": \"running\"/\"id\": \"$id\", \"status\": \"complete\"/" "$QUEUE"
done < <(grep '"status": "pending"' "$QUEUE")
```

**Agent polling:**
```bash
# Check job status
jq -r 'select(.id == "job-1707184800") | .status' jobs/queue.jsonl

# Read results
cat jobs/job-1707184800.log
```

### Pros
- Simple implementation
- Full control
- No external dependencies
- Easy to debug

### Cons
- Requires persistent cron daemon
- No built-in retries
- Manual error handling
- Not distributed

### Recommendation
**Good for:** Simple single-machine setups  
**Not for:** Distributed or high-reliability needs

---

## Approach 4: Marker-Based Completion Detection

### Concept

Jobs write a completion marker; agents poll for it.

```bash
# Start job
./run-benchmark.sh && touch results/benchmark.done

# Agent polls
if [[ -f results/benchmark.done ]]; then
    echo "Complete!"
    cat results/benchmark.json
fi
```

### Enhanced with timestamps
```bash
# Job writes structured completion marker
echo '{"status":"success","duration_sec":3600,"results":"results/benchmark.json"}' > results/benchmark.marker

# Agent reads marker
if [[ -f results/benchmark.marker ]]; then
    jq '.' results/benchmark.marker
fi
```

### Recommendation
**Good for:** Combining with any approach for clean completion signaling  
**Not for:** Standalone use

---

## Recommended Solution: Hybrid Approach

### Architecture

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│  Agent Session  │────▶│  GitHub Actions  │────▶│  M2 Runner      │
│  (Orchestrator) │     │  (Scheduler)     │     │  (Self-hosted)  │
└─────────────────┘     └──────────────────┘     └─────────────────┘
        │                        │                        │
        │                        ▼                        ▼
        │               ┌──────────────────┐     ┌─────────────────┐
        │               │  Artifacts       │     │  Results        │
        │               │  (Stored)        │     │  (Generated)    │
        │               └──────────────────┘     └─────────────────┘
        │                        │
        ▼                        ▼
┌─────────────────┐     ┌──────────────────┐
│  Local nohup    │     │  Agent Polls     │
│  (Quick runs)   │     │  gh run list     │
└─────────────────┘     └──────────────────┘
```

### Implementation Plan

**Phase 1: Local background (immediate)**
- Use `nohup` for quick local runs (<30 min)
- Write completion markers
- Poll in next agent cycle

**Phase 2: GitHub Actions (requires setup)**
1. Create self-hosted runner on M2 Mac
2. Add workflow files for:
   - PolyBench benchmarks
   - Embench benchmarks
   - SPEC benchmarks (when ready)
3. Agent uses `gh workflow run` + `gh run download`

**Phase 3: Scheduled validation (automated)**
- Nightly accuracy runs via cron trigger
- Results committed to repo automatically
- Agents review in morning cycles

---

## Agent Integration Pattern

```bash
# === START LONG JOB ===
# Option A: Local (quick)
nohup ./scripts/run-polybench.sh gemm > results/gemm-$(date +%s).log 2>&1 &
echo "Job started: $!"

# Option B: GitHub Actions (heavy)
gh workflow run benchmark.yml -f suite=polybench -f benchmark=gemm
RUN_ID=$(gh run list --workflow=benchmark.yml -L1 --json databaseId -q '.[0].databaseId')
echo "Started run: $RUN_ID"

# === POLL FOR COMPLETION (next cycle) ===
# Local
if [[ -f results/gemm-*.done ]]; then
    echo "Local job complete"
fi

# GitHub Actions
STATUS=$(gh run view $RUN_ID --json status -q '.status')
if [[ "$STATUS" == "completed" ]]; then
    gh run download $RUN_ID
    echo "Results downloaded"
fi
```

---

## Immediate Next Steps

1. **Create workflow file:** `.github/workflows/benchmark.yml`
2. **Document runner setup:** `docs/self-hosted-runner-setup.md`
3. **Add capture script wrapper:** `scripts/run-with-marker.sh`

---

## For Issue #141 (M2 Baseline Capture)

The M2 baseline capture requires **human involvement** because:
- Self-hosted runner must be configured on M2 hardware
- Performance counters require specific permissions
- Initial calibration run needs manual verification

Once runner is set up, agents can trigger benchmark runs autonomously via:
```bash
gh workflow run m2-baseline.yml -f benchmarks=all
```

---

## Conclusion

Long-running job execution is achievable with existing tools. The recommended hybrid approach:

| Job Type | Tool | Trigger |
|----------|------|---------|
| Quick iteration (<30m) | Local nohup | Direct |
| Production benchmark | GitHub Actions | `gh workflow run` |
| Nightly validation | GitHub Actions | Cron schedule |
| SPEC full suite | GitHub Actions + M2 runner | Manual or scheduled |

**Key insight:** The limiting factor is **self-hosted runner setup**, not agent capability. Once the runner is configured, agents can operate autonomously.
