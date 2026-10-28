// The action row on every tile opens the existing apply and stop sheets.
import { expect, test } from './fixtures';
import type { Page } from '@playwright/test';

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
  const tile = page.locator('.hq-tile[data-team="core"]');
  await tile.locator('[data-hq="stop"]').click();
  const sheet = page.locator('.hq-sheet');
  await expect(sheet).toBeVisible();
  await expect(sheet.locator('.hq-h b')).toHaveText('止める');
  await expect(sheet.locator('.hq-h code')).toHaveText('core');
  await expect(sheet.locator('.hq-acts [data-hq="stopgo"]')).toBeVisible();
});
