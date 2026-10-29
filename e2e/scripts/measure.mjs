// Measure the screens' reads over testdata/perf/sample.json.
//
//   bun e2e/scripts/measure.mjs
//
// Each run owns its HOME, XDG directories, projects, fake Herdr and serve. The
// command fake takes 120ms per call; the optional socket fake answers in 2ms,
// matching the measured costs of the real process and socket paths.
import { chromium } from '@playwright/test';
import { chmodSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { execFileSync, spawn } from 'node:child_process';
import { createServer } from 'node:net';
import { setTimeout as delay } from 'node:timers/promises';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repo = path.resolve(here, '..', '..');
const samplePath = path.join(repo, 'testdata', 'perf', 'sample.json');
const sample = JSON.parse(readFileSync(samplePath, 'utf8'));
const projects = sample.projects;
const rolesByProject = sample.rolesByProject;
const teams = sample.teams;

function usage() {
  return '使い方: bun e2e/scripts/measure.mjs\n';
}

if (process.argv.length > 2) {
  if (process.argv[2] === '-h' || process.argv[2] === '--help') {
    process.stdout.write(usage());
    process.exit(0);
  }
  process.stderr.write(usage());
  process.exit(2);
}

const root = mkdtempSync(path.join(os.tmpdir(), 'hergy-perf-'));
const buildDir = path.join(repo, 'e2e', '.bin');
const hergyBin = path.join(buildDir, 'hergy-measure');
let serve;
let browser;
let socketServer;

function yamlString(value) {
  return JSON.stringify(value);
}

function ruleID(index) {
  return `A${String(index).padStart(3, '0')}`;
}

let nextRule = 0;

function rulesYAML(count, topic) {
  const rules = Array.from({ length: count }, (_, index) => {
    const id = ruleID(nextRule++);
    return `      - id: ${id}\n        text: ${yamlString(`${topic} の手順 ${String(index + 1).padStart(2, '0')} を確認する。`)}`;
  });
  const half = Math.ceil(rules.length / 2);
  const first = rules.slice(0, half).join('\n');
  const second = rules.slice(half).join('\n');
  return `sections:\n  - heading: ${yamlString(`${topic} の進め方`)}\n    rules:\n${first}\n  - heading: ${yamlString(`${topic} の確認`)}\n    rules:\n${second}\n`;
}

function roleYAML(name) {
  const title = `${name} の役割`;
  return `version: 1\ntitle: ${yamlString(title)}\n${rulesYAML(sample.rulesPerDocument, title)}`;
}

function agreementYAML(name) {
  const title = `${name} チームの約束`;
  return `version: 1\ntitle: ${yamlString(title)}\nrevision: 1\n${rulesYAML(sample.rulesPerDocument, title)}`;
}

function membersFor(team) {
  const roles = rolesByProject[team.project];
  return Array.from({ length: team.members }, (_, index) => ({
    name: index === 0 ? 'lead' : `member-${String(index + 1).padStart(2, '0')}`,
    role: index === 0 ? roles[0] : roles[1 + ((index - 1) % (roles.length - 1))],
    profile: index % 3 === 0 ? 'standard' : index % 3 === 1 ? 'fast' : 'review',
  }));
}

function workersFor(team) {
  return Array.from({ length: team.workers }, (_, index) => ({
    name: `worker-${String(index + 1).padStart(2, '0')}`,
    from: index % 2 === 0 ? 'lead' : 'member-02',
    purpose: `確認作業 ${String(index + 1).padStart(2, '0')}`,
    profile: index % 2 === 0 ? 'fast' : 'review',
  }));
}

function teamYAML(team) {
  const members = membersFor(team);
  const workers = workersFor(team);
  const lines = [
    'version: 1',
    `mission: ${yamlString(`${team.name} の作業を進める`)}`,
    'members:',
    ...members.flatMap((member) => [
      `  - name: ${member.name}`,
      `    role: ${member.role}`,
      `    profile: ${member.profile}`,
    ]),
    'workers:',
    ...workers.flatMap((worker) => [
      `  - from: ${worker.from}`,
      `    purpose: ${yamlString(worker.purpose)}`,
      `    profile: ${worker.profile}`,
    ]),
    'turns:',
    '  - human -> lead',
    ...members.slice(1).flatMap((member) => [`  - lead -> ${member.name}`, `  - ${member.name} -> lead`]),
    '  - lead -> human',
  ];
  return `${lines.join('\n')}\n`;
}

function profilesYAML() {
  return [
    'version: 1',
    'profiles:',
    '  standard:',
    '    kind: claude',
    '    args: ["--model", "claude-haiku-5-5"]',
    '  fast:',
    '    kind: codex',
    '    args: ["--model", "gpt-5.6-luna"]',
    '  review:',
    '    kind: claude',
    '    args: ["--model", "claude-sonnet-4-5"]',
    '',
  ].join('\n');
}

function writeDocument(file, text) {
  mkdirSync(path.dirname(file), { recursive: true });
  writeFileSync(file, text);
}

function createFixture() {
  const home = path.join(root, 'home');
  const config = path.join(root, 'config');
  const state = path.join(root, 'state');
  const configHergy = path.join(config, 'hergy');
  const projectPaths = Object.fromEntries(projects.map((name) => [name, path.join(home, 'src', name)]));
  for (const project of projects) {
    const projectRoot = projectPaths[project];
    const hergy = path.join(projectRoot, '.hergy');
    mkdirSync(hergy, { recursive: true });
    writeDocument(path.join(hergy, 'profiles.yaml'), profilesYAML());
    for (const role of rolesByProject[project]) {
      writeDocument(path.join(hergy, 'roles', `${role}.yaml`), roleYAML(role));
    }
    for (const team of teams.filter((one) => one.project === project)) {
      writeDocument(path.join(hergy, 'teams', team.name, 'team.yaml'), teamYAML(team));
      writeDocument(path.join(hergy, 'teams', team.name, 'agreement.yaml'), agreementYAML(team.name));
    }
    execFileSync('git', ['init', '-q'], {
      cwd: projectRoot,
      env: { ...process.env, GIT_CONFIG_GLOBAL: '/dev/null', GIT_CONFIG_NOSYSTEM: '1' },
      stdio: 'ignore',
    });
  }

  mkdirSync(configHergy, { recursive: true });
  mkdirSync(path.join(state, 'hergy', 'launches'), { recursive: true });
  writeDocument(
    path.join(configHergy, 'config.yaml'),
    `version: 1\nlang: ja\nprojects:\n${projects.map((name) => `  - ~/src/${name}`).join('\n')}\n`,
  );
  writeDocument(path.join(configHergy, 'profiles.yaml'), profilesYAML());
  writeDocument(path.join(state, 'hergy', 'launches', 'log.jsonl'), '');

  const herdrDir = path.join(root, 'herdr');
  mkdirSync(herdrDir, { recursive: true });
  const active = teams.filter((team) => team.active);
  const tabs = active.map((team, index) => ({
    tab_id: `t${index + 1}`,
    label: team.name,
    workspace_id: `w${index + 1}`,
  }));
  const agents = [];
  const panes = [];
  let sequence = 1;
  for (const [index, team] of active.entries()) {
    const tabID = `t${index + 1}`;
    const workspaceID = `w${index + 1}`;
    const projectRoot = projectPaths[team.project];
    const names = [
      ...membersFor(team).map((member) => member.name),
      ...workersFor(team).map((worker) => worker.name),
    ];
    for (const name of names) {
      const paneID = `p-${team.name}-${name}`;
      const agentName = `${team.name}-${name}`;
      const status = name === 'lead' ? 'working' : 'idle';
      agents.push({
        name: agentName,
        agent: 'claude',
        agent_status: status,
        pane_id: paneID,
        tab_id: tabID,
        workspace_id: workspaceID,
        state_change_seq: sequence++,
        terminal_title_stripped: '',
      });
      panes.push({
        pane_id: paneID,
        tab_id: tabID,
        workspace_id: workspaceID,
        label: agentName,
        cwd: projectRoot,
      });
    }
  }
  const workspaces = active.map((team, index) => ({
    workspace_id: `w${index + 1}`,
    label: `workspace-${index + 1}`,
    focused: index === 0,
    active_tab_id: `t${index + 1}`,
    number: index + 1,
    pane_count: agents.filter((agent) => agent.workspace_id === `w${index + 1}`).length,
    tab_count: 1,
    agent_status: 'working',
  }));
  const snapshot = {
    version: 'perf-fixture',
    protocol: 22,
    workspaces,
    tabs: tabs.map((tab, index) => ({
      ...tab,
      number: index + 1,
      focused: index === 0,
      pane_count: panes.filter((pane) => pane.tab_id === tab.tab_id).length,
      agent_status: 'working',
    })),
    panes: panes.map((pane) => ({
      ...pane,
      terminal_id: `term-${pane.pane_id}`,
      focused: false,
      agent_status: agents.find((agent) => agent.pane_id === pane.pane_id)?.agent_status ?? 'idle',
      revision: 1,
    })),
    layouts: [],
    agents: agents.map((agent) => ({
      ...agent,
      terminal_id: `term-${agent.pane_id}`,
      focused: false,
      revision: 1,
      interactive_ready: true,
      launch_pending: false,
      screen_detection_skipped: false,
    })),
  };

  for (const [name, value] of Object.entries({ tabs, panes, agents, workspaces })) {
    writeDocument(path.join(herdrDir, `${name}.json`), JSON.stringify({ result: { [name]: value } }));
  }
  writeDocument(path.join(herdrDir, 'current.json'), JSON.stringify({ result: { pane: { workspace_id: 'w1' } } }));
  writeDocument(path.join(herdrDir, 'first-agent.json'), JSON.stringify({ result: agents[0] }));
  writeDocument(path.join(herdrDir, 'process-info.json'), JSON.stringify({ result: { process_info: { pane_id: panes[0].pane_id, foreground_process_group_id: 10, shell_pid: 10 } } }));
  writeDocument(path.join(herdrDir, 'read.txt'), '');

  const commandDelay = (sample.herdrDelayMs.command / 1000).toFixed(3);
  const fakeHerdr = `#!/bin/sh
group="${'${1:-}'}"
sub="${'${2:-}'}"
here="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
if [ -z "$sub" ]; then
  printf 'herdr %s\\n' "$group"
  printf '  get\\n  prompt\\n  start\\n  list\\n  read\\n  focus\\n  send-keys\\n  current\\n  split\\n  rename\\n  close\\n  create\\n  process-info\\n'
  exit 0
fi
sleep ${commandDelay}
case "$group $sub" in
  "tab list") cat "$here/tabs.json" ;;
  "pane list") cat "$here/panes.json" ;;
  "agent list") cat "$here/agents.json" ;;
  "workspace list") cat "$here/workspaces.json" ;;
  "pane current") cat "$here/current.json" ;;
  "agent get") cat "$here/first-agent.json" ;;
  "agent read") cat "$here/read.txt" ;;
  "pane process-info") cat "$here/process-info.json" ;;
  *) printf '{"result":{}}\\n' ;;
esac
`;
  const herdrBin = path.join(herdrDir, 'herdr');
  writeDocument(herdrBin, fakeHerdr);
  chmodSync(herdrBin, 0o755);

  return {
    home,
    config,
    state,
    projectPaths,
    herdrBin,
    socketPath: path.join(root, 'herdr.sock'),
    snapshot,
  };
}

function socketAnswer(method, params, snapshot) {
  const firstAgent = snapshot.agents[0];
  const firstPane = snapshot.panes[0];
  if (method === 'session.snapshot') return { type: 'session_snapshot', snapshot };
  if (method === 'tab.list') return { type: 'tab_list', tabs: snapshot.tabs };
  if (method === 'pane.list') return { type: 'pane_list', panes: snapshot.panes };
  if (method === 'agent.list') return { type: 'agent_list', agents: snapshot.agents };
  if (method === 'workspace.list') return { type: 'workspace_list', workspaces: snapshot.workspaces };
  if (method === 'pane.current') return { type: 'pane_current', pane: firstPane };
  if (method === 'agent.get') return { type: 'agent_info', agent: firstAgent };
  if (method === 'agent.read' || method === 'pane.read') {
    return {
      type: 'pane_read',
      read: {
        pane_id: firstPane.pane_id,
        workspace_id: firstPane.workspace_id,
        tab_id: firstPane.tab_id,
        source: params?.source ?? 'detection',
        format: params?.format ?? 'text',
        text: '',
        revision: 1,
        truncated: false,
      },
    };
  }
  if (method === 'pane.process_info') {
    return { type: 'pane_process_info', process_info: { pane_id: firstPane.pane_id, foreground_process_group_id: 10, shell_pid: 10 } };
  }
  return { type: 'ok' };
}

async function listenSocket(socketPath, snapshot) {
  const server = createServer((connection) => {
    let pending = '';
    connection.setEncoding('utf8');
    connection.on('data', (chunk) => {
      pending += chunk;
      for (;;) {
        const newline = pending.indexOf('\n');
        if (newline < 0) break;
        const line = pending.slice(0, newline);
        pending = pending.slice(newline + 1);
        if (!line) continue;
        const request = JSON.parse(line);
        void (async () => {
          await delay(sample.herdrDelayMs.socket);
          const response = {
            id: request.id,
            result: socketAnswer(request.method, request.params ?? {}, snapshot),
          };
          connection.write(`${JSON.stringify(response)}\n`);
        })();
      }
    });
    connection.on('error', () => {});
  });
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(socketPath, () => {
      server.removeListener('error', reject);
      resolve();
    });
  });
  return server;
}

async function startServe(fixture) {
  serve = spawn(hergyBin, ['serve', '--no-open', '--port', '0'], {
    cwd: fixture.projectPaths.alpha,
    env: {
      ...process.env,
      HOME: fixture.home,
      XDG_CONFIG_HOME: fixture.config,
      XDG_STATE_HOME: fixture.state,
      HERDR_ENV: '',
      HERDR_PANE_ID: '',
      HERGY_GY: path.join(root, 'no-gy'),
      HERGY_HERDR: fixture.herdrBin,
      HERDR_SOCKET_PATH: fixture.socketPath,
      HERGY_E2E_NOW: '2026-10-29T12:00:00+09:00',
      TZ: 'Asia/Tokyo',
      GIT_CONFIG_GLOBAL: '/dev/null',
      GIT_CONFIG_NOSYSTEM: '1',
    },
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  let said = '';
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error(`serve printed no URL: ${said}`)), 20_000);
    const look = (chunk) => {
      said += chunk.toString();
      const found = said.match(/http:\/\/127\.0\.0\.1:[0-9]+/);
      if (!found) return;
      clearTimeout(timer);
      resolve(found[0]);
    };
    serve.stdout.on('data', look);
    serve.stderr.on('data', look);
    serve.once('error', (error) => {
      clearTimeout(timer);
      reject(error);
    });
    serve.once('exit', (code) => {
      clearTimeout(timer);
      reject(new Error(`serve stopped before it answered (${code}): ${said}`));
    });
  });
}

async function stopServe() {
  if (!serve || serve.exitCode !== null) return;
  const exited = new Promise((resolve) => serve.once('exit', resolve));
  serve.kill('SIGTERM');
  await Promise.race([exited, delay(10_000)]);
  if (serve.exitCode === null) {
    const killed = new Promise((resolve) => serve.once('exit', resolve));
    serve.kill('SIGKILL');
    await Promise.race([killed, delay(1_000)]);
  }
}

async function timedGet(base, target) {
  const started = performance.now();
  const response = await fetch(new URL(target, base));
  await response.arrayBuffer();
  const elapsed = performance.now() - started;
  if (!response.ok) throw new Error(`GET ${target} answered ${response.status}`);
  return elapsed;
}

async function screenRead(page, base, target) {
  const started = performance.now();
  const response = await page.goto(new URL(target, base).href, { waitUntil: 'load', timeout: 30_000 });
  if (!response?.ok()) throw new Error(`GET ${target} did not load: ${response?.status() ?? 'no response'}`);
  await page.waitForFunction(
    () => document.readyState === 'complete' && !(window.hergyPage && window.hergyPage.settled === false),
    { timeout: 30_000 },
  );
  return performance.now() - started;
}

function printTable(title, rows) {
  process.stdout.write(`\n${title}\n`);
  process.stdout.write('対象\t2回目 (ms)\t定義を1つ変更した直後 (ms)\n');
  for (const row of rows) {
    process.stdout.write(`${row.target}\t${row.second.toFixed(1)}\t${row.changed.toFixed(1)}\n`);
  }
}

async function main() {
  const fixture = createFixture();
  mkdirSync(buildDir, { recursive: true });
  execFileSync('go', ['build', '-tags', 'e2e', '-o', hergyBin, '.'], { cwd: repo, stdio: 'inherit' });
  socketServer = await listenSocket(fixture.socketPath, fixture.snapshot);
  const base = await startServe(fixture);
  browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();
  const pageErrors = [];
  page.on('pageerror', (error) => pageErrors.push(error.message));
  page.on('console', (message) => {
    if (message.type() === 'error') pageErrors.push(message.text());
  });

  const alpha = fixture.projectPaths.alpha;
  const apiPaths = [
    '/api/activity',
    '/api/activity/core',
    '/api/apply',
    '/api/apply/count',
    `/api/hq/launch/options?project=${encodeURIComponent(alpha)}&definition=core&place=project`,
    '/api/hq/projects',
    '/api/hq/profiles',
    '/api/hq/roles',
    '/api/hq/running',
    '/api/hq/teams',
    '/api/hq/workspaces',
    '/api/profile',
    '/api/profile/standard?place=project',
    '/api/role',
    '/api/role/lead?place=project',
    '/api/team',
    '/api/team/core?place=project',
  ];
  const screens = ['/activity', '/teams', '/roles', '/profiles'];
  const rows = [];

  for (const target of apiPaths) {
    await timedGet(base, target);
    const second = await timedGet(base, target);
    const display = target.startsWith('/api/hq/launch/options?') ? target.slice(0, target.indexOf('?')) : target;
    rows.push({ target: `GET ${display}`, route: target, second, changed: 0 });
  }
  const screenRows = [];
  for (const target of screens) {
    await screenRead(page, base, target);
    const second = await screenRead(page, base, target);
    screenRows.push({ target: `GET ${target}`, route: target, second, changed: 0 });
  }

  const changedDefinition = path.join(alpha, '.hergy', 'roles', 'lead.yaml');
  const before = readFileSync(changedDefinition, 'utf8');
  writeFileSync(changedDefinition, `${before}\n# one definition changed for this measurement\n`);
  for (const row of rows) row.changed = await timedGet(base, row.route);
  for (const row of screenRows) row.changed = await screenRead(page, base, row.route);

  if (pageErrors.length > 0) throw new Error(`screen errors: ${pageErrors.join(' | ')}`);
  printTable('API', rows);
  printTable('画面', screenRows);
  process.stdout.write(
    `\n見本: ${projects.length} プロジェクト、${teams.length} チーム、` +
      `${teams[0].members}+${teams[0].workers}=13 の大きなチームと他 3 チーム、` +
      `${Object.values(rolesByProject).flat().length} ロール、各文書 ${sample.rulesPerDocument} 規則。\n` +
      `Herdr 模擬待ち時間: コマンド ${sample.herdrDelayMs.command}ms、socket ${sample.herdrDelayMs.socket}ms。\n`,
  );
}

try {
  await main();
} finally {
  if (browser) await browser.close();
  await stopServe();
  if (socketServer) await new Promise((resolve) => socketServer.close(resolve));
  rmSync(root, { recursive: true, force: true });
}
