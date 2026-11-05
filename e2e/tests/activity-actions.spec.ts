// The action row on every tile opens the existing apply and stop sheets.
import { expect, test } from './fixtures';
import type { Page } from '@playwright/test';
import { appendFileSync, mkdirSync, readFileSync, realpathSync, renameSync, rmSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { ready } from './helpers';

test.use({ reads: 'activity' });

async function routeSheets(page: Page, project: string): Promise<void> {
  await page.route('**/api/**', async (route) => {
    const pathname = new URL(route.request().url()).pathname;
    if (pathname === '/api/hq/running') {
      await route.fulfill({ json: {
        herdr: { ok: true, text: '' },
        teams: [{
          team: 'core', definition: 'core', place: 'project', project, project_name: 'fixture',
          workspace: { id: 'w1', label: 'main' }, tab: { id: 't1', label: 'core' },
          members: [{ name: 'lead', herdr: 'core-lead', live: true, state: 'idle' }],
          workers: [], waiting: [], starting: false,
        }],
      } });
      return;
    }
    if (pathname === '/api/apply/count') {
      await route.fulfill({ json: { count: 1 } });
      return;
    }
    if (pathname === '/api/apply') {
      await route.fulfill({ json: {
        herdr: { ok: true, text: '' }, count: 1,
        teams: [{
          team: 'core', definition: 'core', project, project_name: 'fixture',
          members: [{
            name: 'core-lead', state: 'idle', how: 'resend', can_apply: true,
            changes: [{ kind: 'prompt', text: 'prompt を更新しました' }],
          }],
        }],
      } });
      return;
    }
    await route.continue();
  });
}

test('the tile action opens the apply sheet for that team', async ({ page, app }) => {
  await routeSheets(page, app.proj);
  await page.goto(`${app.base}/activity`);
  await ready(page);
  const tile = page.locator('.hq-tile[data-team="core"]');
  const apply = tile.locator('[data-hq="apply"]');
  await expect(apply).toBeVisible();
  await apply.click();
  const sheet = page.locator('.apl-sheet');
  await expect(sheet).toBeVisible();
  await expect(sheet.locator('.apl-h b')).toHaveText('反映: core');
  await expect(sheet.locator('[data-apl-name="core-lead"]')).toBeVisible();
});

test('the tile action opens the stop sheet for that team', async ({ page, app }) => {
  await routeSheets(page, app.proj);
  await page.goto(`${app.base}/activity`);
  await ready(page);
  const tile = page.locator('.hq-tile[data-team="core"]');
  await tile.locator('[data-hq="stop"]').click();
  const sheet = page.locator('.hq-sheet');
  await expect(sheet).toBeVisible();
  await expect(sheet.locator('.hq-h b')).toHaveText('終了する');
  await expect(sheet.locator('.hq-h code')).toHaveText('core');
  await expect(sheet.locator('.hq-acts [data-hq="stopgo"]')).toHaveText('終了する');
});

function globalLaunch(app: { proj: string; cfg: string; state: string }): string {
  const dir = path.join(app.cfg, 'hergy', 'teams', 'blueprint');
  mkdirSync(path.dirname(dir), { recursive: true });
  renameSync(path.join(app.proj, '.hergy', 'teams', 'core'), dir);
  appendFileSync(path.join(app.state, 'hergy', 'launches', 'log.jsonl'), JSON.stringify({
    time: '2026-10-21T00:29:00+09:00', command: 'up', project: realpathSync(app.proj),
    layer: 'global', definition: 'blueprint', team: 'core-hotfix',
    team_yaml: path.join(dir, 'team.yaml'), members: [],
  }) + '\n');
  return dir;
}

for (const theme of ['light', 'dark'] as const) {
  test(`activity opens the launch definition in its place in ${theme}`, async ({ page, app }) => {
    globalLaunch(app);
    await page.emulateMedia({ colorScheme: theme });
    for (const [team, definition, place] of [['core-hotfix', 'blueprint', 'global'], ['migrate', 'migrate', 'project']]) {
      await page.goto(`${app.base}/activity`);
      await ready(page);
      await page.locator(`.hq-tile[data-team="${team}"]`).getByRole('link', { name: 'チーム定義を開く' }).click();
      await ready(page);
      const opened = page.locator(`.tdef.open[data-key$=".${place}"]`);
      await expect(opened).toHaveCount(1);
      await expect(opened.locator('.c-name .nm')).toHaveText(definition);
      if (place === 'global') await expect(opened.locator('.c-place .pill.on')).toHaveText('グローバル');
      await expect(page.getByText(/チーム定義 .* が見つかりません/)).toHaveCount(0);
    }
  });
}

for (const listed of [true, false]) {
  test(`a global launch keeps its figure and link when a same-named project definition appears (${listed ? 'listed' : 'unlisted'})`, async ({ page, app }) => {
    globalLaunch(app);
    if (listed) writeFileSync(path.join(app.cfg, 'hergy', 'config.yaml'), `version: 1\nlang: ja\nprojects:\n  - ${app.proj}\n`);
    const local = path.join(app.proj, '.hergy', 'teams', 'blueprint');
    mkdirSync(local, { recursive: true });
    const text = readFileSync(path.join(app.proj, '.hergy', 'teams', 'migrate', 'team.yaml'), 'utf8');
    writeFileSync(path.join(local, 'team.yaml'), text.replace('  - human -> lead\n', ''));
    await page.goto(`${app.base}/activity`);
    await ready(page);
    const tile = page.locator('.hq-tile[data-team="core-hotfix"]');
    await expect(tile.locator('.hq-e.turn[data-from="human"][data-to="lead"], .hq-e.turn[data-from="lead"][data-to="human"]')).toHaveCount(1);
    await expect(tile.locator('.hq-e.turn[data-from="human"][data-to="tester"], .hq-e.turn[data-from="tester"][data-to="human"]')).toHaveCount(0);
    await tile.getByRole('link', { name: 'チーム定義を開く' }).click();
    await ready(page);
    await expect(page.locator('.tdef.open')).toHaveCount(1);
    await expect(page.locator('.tdef.open')).toHaveAttribute('data-key', `${listed ? 'blueprint|global' : 'blueprint'}.global`);
    await expect(page.locator('.tdef.open .c-name .nm')).toHaveText('blueprint');
    await expect(page.getByText(/チーム定義 .* が見つかりません/)).toHaveCount(0);
  });
}

test('a removed launch definition keeps the existing not-found wording', async ({ page, app }) => {
  const dir = globalLaunch(app);
  rmSync(dir, { recursive: true });
  await page.goto(`${app.base}/activity`);
  await ready(page);
  await page.locator('.hq-tile[data-team="core-hotfix"]').getByRole('link', { name: 'チーム定義を開く' }).click();
  await ready(page);
  await expect(page.getByText('チーム定義 blueprint が見つかりません。')).toBeVisible();
});
