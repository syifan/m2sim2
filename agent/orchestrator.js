#!/usr/bin/env node
/**
 * ML Performance Survey - Multi-Agent Orchestrator
 * Manager agents (Athena, Apollo, Hermes) run periodically.
 * Worker agents are discovered from agent/workers/ folder.
 */

import { spawn, execSync } from 'child_process';
import { existsSync, readFileSync, writeFileSync, readdirSync, watch } from 'fs';
import { dirname, join, basename } from 'path';
import { fileURLToPath } from 'url';
import { createServer } from 'http';
import YAML from 'yaml';

const __dirname = dirname(fileURLToPath(import.meta.url));
const REPO_DIR = join(__dirname, '..');
const MANAGERS_PATH = join(__dirname, 'managers');
const WORKERS_PATH = join(__dirname, 'workers');
const SKILLS_PATH = join(__dirname, 'skills');
const CONFIG_PATH = join(__dirname, 'config.yaml');
const ORCHESTRATOR_PATH = join(__dirname, 'orchestrator.js');
const STATE_PATH = join(__dirname, 'state.json');
const CONTROL_PORT = 3002;

let currentAgentProcess = null;
let currentAgentName = null;
let currentAgentStartTime = null;
let cycleCount = 0;
let currentAgentIndex = 0;
let managersRun = []; // Track which managers have run this cycle
let pendingReload = false;
let isPaused = false;
let wakeNow = false;
let startTime = Date.now();
let sleepUntil = null; // Timestamp when sleep ends

function loadState() {
  try {
    const raw = readFileSync(STATE_PATH, 'utf-8');
    const state = JSON.parse(raw);
    cycleCount = state.cycleCount || 0;
    currentAgentIndex = state.currentAgentIndex || 0;
    managersRun = state.managersRun || [];
    isPaused = state.isPaused || false;
    log(`State loaded: cycle=${cycleCount}, agentIndex=${currentAgentIndex}, managersRun=[${managersRun.join(',')}], paused=${isPaused}`);
    return state;
  } catch (e) {
    log('No saved state, starting fresh');
    return { cycleCount: 0, currentAgentIndex: 0, managersRun: [], isPaused: false };
  }
}

function saveState() {
  writeFileSync(STATE_PATH, JSON.stringify({ cycleCount, currentAgentIndex, managersRun, isPaused }, null, 2));
}

function log(message) {
  const timestamp = new Date().toISOString().replace('T', ' ').slice(0, 19);
  console.log(`[${timestamp}] ${message}`);
}

function loadConfig(silent = false) {
  try {
    const raw = readFileSync(CONFIG_PATH, 'utf-8');
    const config = YAML.parse(raw);
    if (!silent) log(`Config: interval=${config.cycleIntervalMs/1000}s, model=${config.model}`);
    return config;
  } catch (e) {
    log(`Error loading config: ${e.message}, using defaults`);
    return {
      cycleIntervalMs: 600_000,
      agentTimeoutMs: 900_000,
      model: 'claude-opus-4-5',
      athenaCycleInterval: 10,
      apolloCycleInterval: 10,
      trackerIssue: 1
    };
  }
}

function discoverWorkers() {
  try {
    if (!existsSync(WORKERS_PATH)) return [];
    return readdirSync(WORKERS_PATH)
      .filter(f => f.endsWith('.md') && f !== 'everyone.md')
      .map(f => basename(f, '.md'));
  } catch (e) {
    log(`Error discovering workers: ${e.message}`);
    return [];
  }
}

function exec(cmd, options = {}) {
  try {
    return execSync(cmd, { 
      cwd: REPO_DIR, 
      encoding: 'utf-8',
      stdio: ['pipe', 'pipe', 'pipe'],
      ...options 
    }).trim();
  } catch (e) {
    return e.stdout?.trim() || '';
  }
}

function postComment(config, body) {
  try {
    const escapedBody = body.replace(/'/g, "'\\''");
    exec(`gh issue comment ${config.trackerIssue} --body '${escapedBody}'`);
    log('Posted timeout comment to tracker');
  } catch (e) {
    log(`Failed to post comment: ${e.message}`);
  }
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

function loadSkill(path) {
  try {
    return readFileSync(path, 'utf-8');
  } catch (e) {
    log(`Warning: Could not load skill ${path}: ${e.message}`);
    return '';
  }
}

function extractModel(skillContent) {
  // Parse YAML frontmatter for model field
  const match = skillContent.match(/^---\n([\s\S]*?)\n---/);
  if (match) {
    const frontmatter = match[1];
    const modelMatch = frontmatter.match(/^model:\s*(.+)$/m);
    if (modelMatch) {
      return modelMatch[1].trim();
    }
  }
  return null;
}

function extractFastMode(skillContent) {
  // Parse YAML frontmatter for fast field
  const match = skillContent.match(/^---\n([\s\S]*?)\n---/);
  if (match) {
    const frontmatter = match[1];
    const fastMatch = frontmatter.match(/^fast:\s*(true|false)$/m);
    if (fastMatch) {
      return fastMatch[1] === 'true';
    }
  }
  return false;
}

async function runAgent(agent, config, isManager = false) {
  log(`Running: ${agent}${isManager ? ' (manager)' : ''}`);
  
  exec('git pull --rebase --quiet');
  
  const skillPath = isManager ? join(MANAGERS_PATH, `${agent}.md`) : join(WORKERS_PATH, `${agent}.md`);
  const everyoneSkill = loadSkill(join(__dirname, 'everyone.md'));
  const agentSkill = loadSkill(skillPath);
  const agentModel = extractModel(agentSkill) || config.model;
  const useFastMode = extractFastMode(agentSkill);
  
  const prompt = `You are ${agent} working on the ML Performance Survey project.

**Config:**
- GitHub Repo: syifan/ml-perf-survey
- Local Path: ${REPO_DIR}
- Tracker Issue: #${config.trackerIssue}

**Shared Rules:**
${everyoneSkill}

**Your Role:**
${agentSkill}

**Instructions:**
Execute your full cycle as described above. Work autonomously. Complete your tasks, then exit.`;

  log(`Using model: ${agentModel}${useFastMode ? ' (fast)' : ''}`);
  
  return new Promise((resolve) => {
    const args = [
      '--model', agentModel,
      '--dangerously-skip-permissions',
      '--output-format', 'json'
    ];
    if (useFastMode) args.push('--fast');
    args.push(prompt);
    
    const proc = spawn('claude', args, {
      cwd: REPO_DIR,
      stdio: ['ignore', 'pipe', 'ignore']
    });

    currentAgentProcess = proc;
    currentAgentName = agent;
    currentAgentStartTime = Date.now();
    let timedOut = false;
    let timeout = null;
    let outputBuffer = '';

    // Collect stdout for JSON parsing
    proc.stdout.on('data', (data) => {
      outputBuffer += data.toString();
    });

    // Only set timeout if agentTimeoutMs > 0 (0 = never timeout)
    if (config.agentTimeoutMs > 0) {
      timeout = setTimeout(() => {
        timedOut = true;
        log(`${agent} timed out, killing...`);
        proc.kill('SIGTERM');
      }, config.agentTimeoutMs);
    }

    proc.on('close', (code) => {
      if (timeout) clearTimeout(timeout);
      currentAgentProcess = null;
      currentAgentStartTime = null;
      
      // Post timeout comment to tracker
      if (timedOut) {
        const timeoutMin = Math.round(config.agentTimeoutMs / 60000);
        postComment(config, `# [Orchestrator]

⏱️ **Agent Timeout**

**Agent:** ${agent}
**Cycle:** ${cycleCount}
**Timeout:** ${timeoutMin} minutes
**Status:** Killed (SIGTERM)

The agent exceeded the maximum allowed runtime and was terminated.`);
      }
      currentAgentName = null;
      
      // Parse JSON output for usage info
      try {
        // Find the last complete JSON object (result line)
        const lines = outputBuffer.trim().split('\n');
        for (let i = lines.length - 1; i >= 0; i--) {
          try {
            const json = JSON.parse(lines[i]);
            if (json.type === 'result' && json.usage) {
              const u = json.usage;
              const cost = json.total_cost_usd ? `$${json.total_cost_usd.toFixed(4)}` : 'N/A';
              log(`${agent} done (code ${code}) | tokens: in=${u.input_tokens || 0} out=${u.output_tokens || 0} cache_read=${u.cache_read_input_tokens || 0} | cost: ${cost}`);
              resolve(code);
              return;
            }
          } catch { /* not valid JSON, continue */ }
        }
      } catch { /* ignore parse errors */ }
      
      log(`${agent} done (code ${code})`);
      resolve(code);
    });

    proc.on('error', (err) => {
      if (timeout) clearTimeout(timeout);
      currentAgentProcess = null;
      currentAgentName = null;
      log(`${agent} error: ${err.message}`);
      resolve(1);
    });
  });
}

async function runCycle() {
  // Check for STOP file
  const stopFile = join(__dirname, 'STOP');
  if (existsSync(stopFile)) {
    log('🛑 STOP file found - halting orchestrator');
    const reason = readFileSync(stopFile, 'utf-8');
    log(reason);
    process.exit(0);
  }
  
  const config = loadConfig();
  const workers = discoverWorkers();
  
  // Check if starting fresh cycle (no managers run yet and no workers done)
  const isNewCycle = managersRun.length === 0 && currentAgentIndex === 0;
  if (isNewCycle) {
    cycleCount++;
    log(`===== CYCLE ${cycleCount} (${workers.length} workers) =====`);
  } else {
    log(`===== CYCLE ${cycleCount} (resuming: managers=[${managersRun.join(',')}], workers=${currentAgentIndex}/${workers.length}) =====`);
  }
  
  // Athena (strategist) - runs on cycle 1, 1+interval, 1+2*interval, etc.
  const athenaShould = (cycleCount - 1) % (config.athenaCycleInterval || 10) === 0;
  if (athenaShould && !managersRun.includes('athena')) {
    await runAgent('athena', config, true);
    managersRun.push('athena');
    saveState();
    if (pendingReload) return config;
  }
  
  // Apollo (HR) - runs on cycle 1, 1+interval, 1+2*interval, etc.
  const apolloShould = (cycleCount - 1) % (config.apolloCycleInterval || 10) === 0;
  if (apolloShould && !managersRun.includes('apollo')) {
    await runAgent('apollo', config, true);
    managersRun.push('apollo');
    saveState();
    if (pendingReload) return config;
  }
  
  // Hermes (PM) every cycle
  if (!managersRun.includes('hermes')) {
    await runAgent('hermes', config, true);
    managersRun.push('hermes');
    saveState();
    if (pendingReload) return config;
  }
  
  // Run worker agents
  while (currentAgentIndex < workers.length) {
    const agent = workers[currentAgentIndex];
    await runAgent(agent, config, false);
    currentAgentIndex++;
    saveState();
    if (pendingReload) return config;
  }
  
  // Cycle complete - reset for next cycle
  currentAgentIndex = 0;
  managersRun = [];
  saveState();
  
  return config;
}

// ============ Control API Server ============

function startControlServer() {
  const server = createServer((req, res) => {
    // CORS headers
    res.setHeader('Access-Control-Allow-Origin', '*');
    res.setHeader('Access-Control-Allow-Methods', 'GET, POST, OPTIONS');
    res.setHeader('Access-Control-Allow-Headers', 'Content-Type');
    
    if (req.method === 'OPTIONS') {
      res.writeHead(200);
      res.end();
      return;
    }
    
    const url = new URL(req.url, `http://localhost:${CONTROL_PORT}`);
    const path = url.pathname;
    
    // GET /status
    if (req.method === 'GET' && path === '/status') {
      const config = loadConfig(true);  // silent
      const workers = discoverWorkers();
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({
        running: !isPaused,
        paused: isPaused,
        currentAgent: currentAgentName,
        currentAgentStartTime,
        currentAgentRuntime: currentAgentStartTime ? Math.floor((Date.now() - currentAgentStartTime) / 1000) : null,
        currentAgentIndex,
        cycleCount,
        managersRun,
        totalWorkers: workers.length,
        pid: process.pid,
        uptime: Math.floor((Date.now() - startTime) / 1000),
        cycleIntervalMs: config.cycleIntervalMs,
        pendingReload,
        sleeping: sleepUntil !== null,
        sleepUntil
      }));
      return;
    }
    
    // GET /queue
    if (req.method === 'GET' && path === '/queue') {
      const workers = discoverWorkers();
      const config = loadConfig(true);  // silent
      const queue = [];
      
      // Add remaining workers in current cycle
      for (let i = currentAgentIndex; i < workers.length; i++) {
        queue.push({ name: workers[i], type: 'worker' });
      }
      
      // Add managers for next cycle
      const nextCycle = cycleCount + 1;
      if ((nextCycle - 1) % (config.athenaCycleInterval || 10) === 0) {
        queue.push({ name: 'athena', type: 'manager', nextCycle: true });
      }
      if ((nextCycle - 1) % (config.apolloCycleInterval || 10) === 0) {
        queue.push({ name: 'apollo', type: 'manager', nextCycle: true });
      }
      queue.push({ name: 'hermes', type: 'manager', nextCycle: true });
      
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ queue, workers }));
      return;
    }
    
    // POST /pause
    if (req.method === 'POST' && path === '/pause') {
      isPaused = true;
      saveState();
      log('⏸️  Paused via API');
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ success: true, paused: true }));
      return;
    }
    
    // POST /resume
    if (req.method === 'POST' && path === '/resume') {
      isPaused = false;
      wakeNow = true;
      saveState();
      log('▶️  Resumed via API');
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ success: true, paused: false }));
      return;
    }
    
    // POST /skip
    if (req.method === 'POST' && path === '/skip') {
      if (currentAgentProcess) {
        log(`⏭️  Skipping ${currentAgentName} via API`);
        currentAgentProcess.kill('SIGTERM');
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ success: true, skipped: currentAgentName }));
      } else {
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ success: false, message: 'No agent running' }));
      }
      return;
    }
    
    // POST /reload
    if (req.method === 'POST' && path === '/reload') {
      log('🔄 Reload requested via API');
      pendingReload = true;
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ success: true, pendingReload: true }));
      return;
    }
    
    // POST /stop
    if (req.method === 'POST' && path === '/stop') {
      log('🛑 Stop requested via API');
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ success: true, stopping: true }));
      if (currentAgentProcess) currentAgentProcess.kill('SIGTERM');
      setTimeout(() => process.exit(0), 500);
      return;
    }
    
    // 404
    res.writeHead(404, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ error: 'Not found' }));
  });
  
  server.listen(CONTROL_PORT, () => {
    log(`Control API listening on http://localhost:${CONTROL_PORT}`);
  });
  
  return server;
}

async function main() {
  log('Orchestrator started');
  loadState();
  startControlServer();
  
  let debounce = null;
  watch(ORCHESTRATOR_PATH, (eventType) => {
    if (eventType === 'change') {
      clearTimeout(debounce);
      debounce = setTimeout(() => {
        log('⚡ Code changed — reloading after current agent');
        pendingReload = true;
      }, 500);
    }
  });
  
  while (true) {
    // Check pause state
    while (isPaused) {
      await sleep(1000);
    }
    
    await runCycle();
    
    if (pendingReload) {
      log('⚡ Reloading...');
      process.exit(75);
    }
    
    // Reload config right before sleep to get latest interval
    const sleepConfig = loadConfig();
    log(`Sleeping ${sleepConfig.cycleIntervalMs / 1000}s...`);
    
    // Interruptible sleep - check every 5s for wake signal
    let sleptMs = 0;
    wakeNow = false;
    sleepUntil = Date.now() + sleepConfig.cycleIntervalMs;
    while (sleptMs < sleepConfig.cycleIntervalMs && !wakeNow) {
      await sleep(5000);
      sleptMs += 5000;
      // If paused during sleep, wait here
      while (isPaused && !wakeNow) {
        await sleep(1000);
      }
    }
    sleepUntil = null;
    if (wakeNow) {
      log('⏰ Woke early via API');
      wakeNow = false;
    }
  }
}

process.on('SIGINT', () => {
  log('Shutting down...');
  if (currentAgentProcess) currentAgentProcess.kill('SIGTERM');
  process.exit(0);
});

process.on('SIGTERM', () => {
  log('Shutting down...');
  if (currentAgentProcess) currentAgentProcess.kill('SIGTERM');
  process.exit(0);
});

process.on('SIGHUP', () => {
  log('⚡ SIGHUP — reloading after current agent');
  pendingReload = true;
});

main().catch(console.error);
