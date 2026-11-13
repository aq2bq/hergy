import { registerRunningTeam, receiveRunningPrompts } from './running-state';
// ⑤ r-ba9b / ac-c7ee / ac-1f12: the prompt's table. What a member or a worker is
// handed is read by one owner, follows the definition's unsaved edits and the
// running teams (no run, one, one alias, two), and matches what the same hergy
// builds at that moment (`up`/`hire --dry --json`). Nothing is pressed to make
// it fresh: the screens' own ten-second reading brings it.
import { test, expect } from './fixtures';
import type { App } from './fixtures';
import { ready } from './helpers';
import { execFileSync } from 'node:child_process';
import { mkdirSync, readFileSync, realpathSync, writeFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import path from 'node:path';
import { hergyBin } from '../scripts/fixture.mjs';

test.use({ reads: 'teams' });

type Plan = { members: { name: string; prompt: string }[] };

const run = (app: App, argv: string[]): string => execFileSync(hergyBin, argv, {
  cwd: app.proj, env: { ...process.env, ...app.env }, encoding: 'utf8',
});
const planFor = (app: App, team: string): Plan => JSON.parse(run(app, [
  'up', 'core', '--dry', '--json', '--lang', 'ja', ...(team === 'core' ? [] : ['--as', team]),
])) as Plan;
const memberPrompt = (app: App, team: string): string => planFor(app, team).members.find((m) => m.name === `${team}-lead`)!.prompt;
const hirePrompt = (app: App, team: string): string => (JSON.parse(run(app, [
  'hire', 'w1', '--purpose', '依存の更新', '--task', '読み取る', '--from', 'lead', '--team', team, '--dry', '--json',
])) as Plan).members[0].prompt;

// setLive writes the launch record and herdr's answer for the teams named: the
// screen reads them at its next ten-second reading, with no press.
function setLive(app: App, teams: string[]): void {
  const dir = path.join(app.state, 'hergy/launches');
  mkdirSync(path.join(dir, 'blobs'), { recursive: true });
  const lines: string[] = [];
  const tabs: { tab_id: string; label: string; workspace_id: string }[] = [];
  const agents: { name: string; agent: string; agent_status: string; pane_id: string; tab_id: string; workspace_id: string }[] = [];
  teams.forEach((team, ti) => {
 registerRunningTeam(app,team,'core');
    const plan = planFor(app, team);
    receiveRunningPrompts(app, team, plan.members);
    const members = plan.members.map((m) => {
      const name = m.name;
      const hash = createHash('sha256').update(m.prompt).digest('hex');
      writeFileSync(path.join(dir, 'blobs', hash), m.prompt);
      return { name, short: m.name.slice(team.length + 1), prompt: hash };
    });
    lines.push(JSON.stringify({
      time: new Date().toISOString(), command: 'up', project: realpathSync(app.proj), layer: 'local',
      definition: 'core', team, team_yaml: realpathSync(path.join(app.proj, '.hergy/teams/core/team.yaml')), members,
    }));
    const tab = `t${ti + 1}`;
    tabs.push({ tab_id: tab, label: team, workspace_id: 'w1' });
    members.forEach((m, mi) => agents.push({ name: m.name, agent: 'claude', agent_status: 'idle', pane_id: `p${ti}-${mi}`, tab_id: tab, workspace_id: 'w1' }));
  });
  writeFileSync(path.join(dir, 'log.jsonl'), lines.join('\n') + (lines.length ? '\n' : ''));
  const herdr = path.join(app.work, 'herdr');
  mkdirSync(herdr, { recursive: true });
  writeFileSync(path.join(herdr, 'tabs.json'), JSON.stringify({ result: { tabs } }));
  writeFileSync(path.join(herdr, 'agents.json'), JSON.stringify({ result: { agents } }));
}

const panel = (page: import('@playwright/test').Page) => page.locator('.tdef[data-name="core"] .cv-topology-prompt');
const note = (page: import('@playwright/test').Page) => page.locator('.tdef[data-name="core"] [data-role="prompt-note"]');
const dialog = (page: import('@playwright/test').Page) => page.getByRole('dialog', { name: 'プロンプト · core / lead', exact: true });
const currentText = (page: import('@playwright/test').Page): Promise<string> =>
  dialog(page).locator('.prompt-line-text').allTextContents().then((lines) => lines.join(''));

async function openMember(page: import('@playwright/test').Page): Promise<void> {
  await page.locator('.cv-n.member[data-cv-n="lead"]').click();
  await page.locator('.cv-side [data-cv-side="prompt"]').click();
}
async function showCurrent(page: import('@playwright/test').Page): Promise<string> {
  await panel(page).getByRole('button', { name: '全文と差分を読む', exact: true }).click();
  await dialog(page).getByRole('button', { name: '今の文', exact: true }).click();
  const text = await currentText(page);
  await dialog(page).getByRole('button', { name: '閉じる', exact: true }).click();
  return text;
}

const workerPanel = (page: import('@playwright/test').Page) => page.locator('.tdef[data-name="core"] details.cv-prompt');
async function workerText(page: import('@playwright/test').Page, team: string): Promise<string> {
  return workerPanel(page).locator('.yl[data-ln] .yt').evaluateAll((els, arg) => els.map((el) => {
    const copy = el.cloneNode(true) as HTMLElement;
    copy.querySelectorAll<HTMLElement>('.cv-slot').forEach((slot) => { slot.textContent = slot.dataset.v || ''; });
    return (copy.textContent || '')
      .split(`${arg.team}-zz-worker-name-0`).join(`${arg.team}-w1`)
      .split('zzTaskPlaceholder0').join(arg.task);
  }).join('\n'), { team, task: '読み取る' });
}
async function openWorker(page: import('@playwright/test').Page): Promise<void> {
  await page.locator('.cv-n.worker[data-cv-w="0"]').click();
  await page.locator('.cv-side [data-cv-side="prompt"]').click();
}
const trimEnd = (text: string): string => text.replace(/\n+$/, '');

test('a member: the text and the note follow no run, one run and its alias', async ({ page, app }) => {
  setLive(app, []);
  await page.goto(`${app.base}/teams?open=core`); await ready(page); await openMember(page);
  await expect(note(page)).toContainText('このチーム定義から動いているチームが無いため、チーム定義名 core で作っています。', { timeout: 15_000 });
  await expect.poll(async () => trimEnd(await showCurrent(page)), { timeout: 15_000 }).toBe(trimEnd(memberPrompt(app, 'core')));

  // One team starts, under another name: the screen reads again by itself.
  setLive(app, ['core-hotfix']);
  await expect(note(page)).toContainText('実行中のチーム core-hotfix の名前で作っています。', { timeout: 15_000 });
  await expect.poll(async () => trimEnd(await showCurrent(page)), { timeout: 15_000 }).toBe(trimEnd(memberPrompt(app, 'core-hotfix')));

  // It stops and the definition's own team starts: the name changes again.
  setLive(app, ['core']);
  await expect(note(page)).toContainText('実行中のチーム core の名前で作っています。', { timeout: 15_000 });
  await expect.poll(async () => trimEnd(await showCurrent(page)), { timeout: 15_000 }).toBe(trimEnd(memberPrompt(app, 'core')));

  // Nothing runs: the definition's name is back.
  setLive(app, []);
  await expect(note(page)).toContainText('チーム定義名 core で作っています。', { timeout: 15_000 });
  await expect.poll(async () => trimEnd(await showCurrent(page)), { timeout: 15_000 }).toBe(trimEnd(memberPrompt(app, 'core')));
});

test('a member: two runs ask for a pick, and the pick is the one both panels use', async ({ page, app }) => {
  setLive(app, ['core', 'core-hotfix']);
  await page.goto(`${app.base}/teams?open=core`); await ready(page); await openMember(page);
  await expect(note(page)).toContainText('動いているチームが 2 つあるため、チーム定義名 core で作っています。', { timeout: 15_000 });
  await expect.poll(async () => trimEnd(await showCurrent(page)), { timeout: 15_000 }).toBe(trimEnd(memberPrompt(app, 'core')));

  await panel(page).getByRole('button', { name: '全文と差分を読む', exact: true }).click();
  await dialog(page).locator('.prompt-controls select').selectOption('core-hotfix');
  await expect(note(page)).toContainText('実行中のチーム core-hotfix の名前で作っています。', { timeout: 15_000 });
  await dialog(page).getByRole('button', { name: '今の文', exact: true }).click();
  await expect.poll(() => currentText(page).then(trimEnd), { timeout: 15_000 }).toBe(trimEnd(memberPrompt(app, 'core-hotfix')));
  await dialog(page).getByRole('button', { name: '閉じる', exact: true }).click();

  // One of the two stops: the pick stood for the one that is still running.
  setLive(app, ['core-hotfix']);
  await expect(note(page)).toContainText('実行中のチーム core-hotfix の名前で作っています。', { timeout: 15_000 });

  // The picked team stops and the other remains: the screen lets the pick go
  // and takes the one team still running.
  setLive(app, ['core']);
  await expect(note(page)).toContainText('実行中のチーム core の名前で作っています。', { timeout: 15_000 });
  await expect.poll(async () => trimEnd(await showCurrent(page)), { timeout: 15_000 }).toBe(trimEnd(memberPrompt(app, 'core')));
});

test('a member: an unsaved role edit is in the current text', async ({ page, app }) => {
  setLive(app, ['core-hotfix']);
  await page.goto(`${app.base}/teams?open=core`); await ready(page); await openMember(page);
  await expect(note(page)).toContainText('実行中のチーム core-hotfix の名前で作っています。', { timeout: 15_000 });
  // The attribute panel's role pick changes the draft, not the file.
  await page.locator('.cv-side [data-cv-side="attr"]').click();
  await page.locator('.cv-side select[data-cv-f="m-role"]').selectOption('builder');
  await page.locator('.cv-side [data-cv-side="prompt"]').click();
  const builder = readFileSync(path.join(app.proj, '.hergy/roles/builder.md'), 'utf8')
    .split('\n').map((line) => line.trim()).find((line) => line && !line.startsWith('#'))!;
  await expect.poll(async () => {
    await panel(page).getByRole('button', { name: '全文と差分を読む', exact: true }).click();
    await dialog(page).getByRole('button', { name: '今の文', exact: true }).click();
    const text = await currentText(page);
    await dialog(page).getByRole('button', { name: '閉じる', exact: true }).click();
    return text;
  }, { timeout: 15_000 }).toContain(builder);
});

test('a worker: the preview follows no run, one run and its alias', async ({ page, app }) => {
  setLive(app, []);
  await page.goto(`${app.base}/teams?open=core`); await ready(page); await openWorker(page);
  await expect.poll(async () => trimEnd(await workerText(page, 'core')), { timeout: 15_000 }).toBe(trimEnd(hirePrompt(app, 'core')));

  setLive(app, ['core-hotfix']);
  await expect.poll(async () => trimEnd(await workerText(page, 'core-hotfix')), { timeout: 15_000 }).toBe(trimEnd(hirePrompt(app, 'core-hotfix')));

  setLive(app, []);
  await expect.poll(async () => trimEnd(await workerText(page, 'core')), { timeout: 15_000 }).toBe(trimEnd(hirePrompt(app, 'core')));
});

test('a worker: two runs use the pick, and an unsaved purpose is in the preview', async ({ page, app }) => {
  setLive(app, ['core', 'core-hotfix']);
  await page.goto(`${app.base}/teams?open=core`); await ready(page);
  // The pick is made where both panels read it: the member's full panel.
  await openMember(page);
  await expect(note(page)).toContainText('動いているチームが 2 つあるため、チーム定義名 core で作っています。', { timeout: 15_000 });
  await panel(page).getByRole('button', { name: '全文と差分を読む', exact: true }).click();
  await dialog(page).locator('.prompt-controls select').selectOption('core-hotfix');
  await expect(note(page)).toContainText('実行中のチーム core-hotfix の名前で作っています。', { timeout: 15_000 });
  await dialog(page).getByRole('button', { name: '閉じる', exact: true }).click();

  await openWorker(page);
  await expect.poll(async () => trimEnd(await workerText(page, 'core-hotfix')), { timeout: 15_000 }).toBe(trimEnd(hirePrompt(app, 'core-hotfix')));

  // An unsaved purpose changes the preview: the same worker, renamed.
  await page.locator('.cv-side [data-cv-side="attr"]').click();
  await page.locator('.cv-side input[data-cv-f="w-purpose"]').fill('新しい目的');
  await page.locator('.cv-side [data-cv-side="prompt"]').click();
  await expect.poll(async () => (await workerPanel(page).textContent()) || '', { timeout: 15_000 }).toContain('新しい目的');
});
