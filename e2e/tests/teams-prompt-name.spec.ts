import { test, expect } from './fixtures';
import { ready } from './helpers';
import { execFileSync } from 'node:child_process';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import path from 'node:path';
import { hergyBin } from '../scripts/fixture.mjs';

// r-0b69: a team booted under another name (up --as) gives the prompt column and
// the whole-text panel the same member names, and the column says which name it
// built with.
test.use({ reads: 'teams' });

test('a team running under another name builds both texts with that name', async ({ page, app }, info) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  const yaml = path.join(app.proj, '.hergy/teams/core/team.yaml');
  const plan = JSON.parse(execFileSync(hergyBin, ['up', 'core', '--dry', '--json', '--lang', 'ja'], { cwd: app.proj, env: { ...process.env, ...app.env }, encoding: 'utf8' }));
  const alias = 'core-hotfix';
  // The launch record and herdr answer of the aliased team: the same definition,
  // booted as core-hotfix, with its members named core-hotfix-*.
  const dir = path.join(app.state, 'hergy/launches');
  mkdirSync(path.join(dir, 'blobs'), { recursive: true });
  const members = plan.members.map((m: { name: string; prompt: string }) => {
    const name = alias + m.name.slice('core'.length);
    const hash = createHash('sha256').update(m.prompt).digest('hex');
    writeFileSync(path.join(dir, 'blobs', hash), m.prompt);
    return { name, short: m.name.slice('core-'.length), prompt: hash };
  });
  writeFileSync(path.join(dir, 'log.jsonl'), JSON.stringify({
    time: new Date().toISOString(), command: 'up', project: app.proj, layer: 'local',
    definition: 'core', team: alias, team_yaml: yaml, members,
  }) + '\n');
  const herdr = path.join(app.work, 'herdr');
  mkdirSync(herdr, { recursive: true });
  writeFileSync(path.join(herdr, 'tabs.json'), JSON.stringify({ result: { tabs: [{ tab_id: 't1', label: alias, workspace_id: 'w1' }] } }));
  writeFileSync(path.join(herdr, 'agents.json'), JSON.stringify({ result: { agents: members.map((m: { name: string }, i: number) => ({
    name: m.name, agent: 'claude', agent_status: 'idle', pane_id: `p${i + 1}`, tab_id: 't1', workspace_id: 'w1',
  })) } }));

  await page.goto(`${app.base}/teams?open=core`); await ready(page);
  await page.locator('.cv-n.member[data-cv-n="lead"]').click();
  await page.locator('.cv-side [data-cv-side="prompt"]').click();
  const panel = page.locator('.tdef[data-name="core"] .cv-topology-prompt');
  await expect(panel.locator('.cv-note')).toContainText(`実行中のチーム ${alias} の名前で作っています。`);
  await expect(panel.locator('.cv-topology-section[data-kind=roster] pre')).toContainText(`${alias}-lead`);

  for (const scheme of ['light', 'dark'] as const) {
    await page.emulateMedia({ colorScheme: scheme });
    await page.screenshot({ path: info.outputPath(`alias-${scheme}.png`) });
  }

  // The whole-text panel builds with the same name.
  await panel.getByRole('button', { name: '全文と差分を読む', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'プロンプト · core / lead', exact: true });
  await dialog.getByRole('button', { name: '今の文', exact: true }).click();
  await expect.poll(() => dialog.locator('.prompt-line-text').allTextContents().then((lines) => lines.join(''))).toContain(`${alias}-lead（`);

  // The two readings name the member the same way.
  const columnName = await panel.locator('.cv-topology-section[data-kind=roster] pre').textContent();
  const current = await dialog.locator('.prompt-line-text').allTextContents().then((lines) => lines.join(''));
  expect(current).toContain((columnName || '').trim().split('\n')[0]);

});
