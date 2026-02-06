#!/usr/bin/env node
/**
 * M2Sim Multi-Agent Orchestrator (Standalone)
 * Runs all agents sequentially in one cycle, then waits before next cycle.
 * Config is reloaded at the start of each cycle.
 */

import { spawn, execSync } from 'child_process';
import { existsSync, mkdirSync, appendFileSync, readFileSync } from 'fs';
import { dirname, join } from 'path';
import { fileURLToPath } from 'url';
import YAML from 'yaml';

const __dirname = dirname(fileURLToPath(import.meta.url));
const REPO_DIR = join(__dirname, '..');
const SKILL_PATH = join(__dirname, 'skills');
const LOGS_DIR = join(__dirname, 'logs');
const CONFIG_PATH = join(__dirname, 'config.yaml');

// Track currently running agent
let currentAgentProcess = null;
let currentAgentName = null;
let cycleCount = 0;

function log(message) {
  const timestamp = new Date().toISOString().replace('T', ' ').slice(0, 19);
  console.log(`[${timestamp}] ${message}`);
}

function loadConfig() {
  try {
    const raw = readFileSync(CONFIG_PATH, 'utf-8');
    const config = YAML.parse(raw);
    log(`Config loaded: interval=${config.cycleIntervalMs/1000}s, agents=${config.agents.join('→')}, model=${config.model}`);
    return config;
  } catch (e) {
    log(`Error loading config: ${e.message}, using defaults`);
    return {
      cycleIntervalMs: 600_000,
      agentTimeoutMs: 900_000,
      model: 'claude-opus-4-5',
      agents: ['alice', 'eric', 'bob', 'cathy', 'dana'],
      graceCycleInterval: 10,
      trackerIssue: 45
    };
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

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

function loadSkill(filename) {
  try {
    return readFileSync(join(SKILL_PATH, filename), 'utf-8');
  } catch (e) {
    log(`Warning: Could not load skill ${filename}: ${e.message}`);
    return '';
  }
}

async function runAgent(agent, config) {
  const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
  const logFile = join(LOGS_DIR, `${agent}-${timestamp}.log`);
  
  log(`Running agent: ${agent}`);
  
  // Pull latest before each agent
  exec('git pull --rebase --quiet');
  
  // Load skills fresh each time
  const everyoneSkill = loadSkill('everyone.md');
  const agentSkill = loadSkill(`${agent}.md`);
  
  const prompt = `You are [${agent}] working on the M2Sim project.

**Config:**
- GitHub Repo: sarchlab/m2sim  
- Local Path: ${REPO_DIR}
- Tracker Issue: #${config.trackerIssue}

**Shared Rules (everyone.md):**
${everyoneSkill}

**Your Role (${agent}.md):**
${agentSkill}

**Instructions:**
Execute your full cycle as described above. Work autonomously. Complete your tasks, then exit.`;

  return new Promise((resolve) => {
    const proc = spawn('claude', [
      '--model', config.model,
      '--dangerously-skip-permissions',
      '--print',
      prompt
    ], {
      cwd: REPO_DIR,
      stdio: ['ignore', 'pipe', 'pipe']
    });

    currentAgentProcess = proc;
    currentAgentName = agent;

    const timeout = setTimeout(() => {
      log(`Agent ${agent} timed out, killing...`);
      proc.kill('SIGTERM');
    }, config.agentTimeoutMs);

    proc.stdout.on('data', (data) => {
      const text = data.toString();
      process.stdout.write(text);
      appendFileSync(logFile, text);
    });

    proc.stderr.on('data', (data) => {
      const text = data.toString();
      process.stderr.write(text);
      appendFileSync(logFile, text);
    });

    proc.on('close', (code) => {
      clearTimeout(timeout);
      currentAgentProcess = null;
      currentAgentName = null;
      log(`Agent ${agent} finished with code ${code}`);
      resolve(code);
    });

    proc.on('error', (err) => {
      clearTimeout(timeout);
      currentAgentProcess = null;
      currentAgentName = null;
      log(`Agent ${agent} error: ${err.message}`);
      resolve(1);
    });
  });
}

async function runCycle() {
  cycleCount++;
  
  // Reload config at start of each cycle
  const config = loadConfig();
  
  log(`========== CYCLE ${cycleCount} START ==========`);
  
  // Run Grace at cycle 1, 11, 21, etc.
  if (cycleCount % config.graceCycleInterval === 1) {
    log('Running Grace (advisor)');
    await runAgent('grace', config);
  }
  
  // Run all agents sequentially
  for (const agent of config.agents) {
    await runAgent(agent, config);
  }
  
  log(`========== CYCLE ${cycleCount} END ==========`);
  
  return config;
}

async function main() {
  log('M2Sim Orchestrator started (Node.js, standalone)');
  log(`Config path: ${CONFIG_PATH}`);
  log(`Repo: ${REPO_DIR}`);
  
  // Create logs directory
  if (!existsSync(LOGS_DIR)) {
    mkdirSync(LOGS_DIR, { recursive: true });
  }
  
  // Main loop
  while (true) {
    const config = await runCycle();
    
    log(`Sleeping ${config.cycleIntervalMs / 1000}s until next cycle...`);
    await sleep(config.cycleIntervalMs);
  }
}

// Handle graceful shutdown
process.on('SIGINT', () => {
  log('Shutting down...');
  if (currentAgentProcess) {
    log(`Killing agent ${currentAgentName}...`);
    currentAgentProcess.kill('SIGTERM');
  }
  process.exit(0);
});

process.on('SIGTERM', () => {
  log('Shutting down...');
  if (currentAgentProcess) {
    log(`Killing agent ${currentAgentName}...`);
    currentAgentProcess.kill('SIGTERM');
  }
  process.exit(0);
});

main().catch(console.error);
