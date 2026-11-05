// r-56f0 / ac-2c11: real files in both places, with projects left unlisted.
import { test, expect } from './fixtures';
import { cpSync, realpathSync } from 'node:fs';
import path from 'node:path';
import { ready } from './helpers';

test.use({ reads: 'teams' });

test('qualified team URLs open one place and keep it across tabs, reload and back', async ({ page, app }) => {
  cpSync(path.join(app.proj, '.hergy', 'teams', 'core'), path.join(app.cfg, 'hergy', 'teams', 'core'), { recursive: true });
  const project = realpathSync(app.proj);
  for (const [group, place] of [['global', 'global'], [project, 'project']]) {
    const ref = `core|${group}`;
    await page.goto(`${app.base}/teams?${new URLSearchParams({ open: ref })}`);
    await ready(page);
    const opened = page.locator('.tdef.open');
    await expect(opened).toHaveCount(1);
    await expect(opened).toHaveAttribute('data-key', `core.${place}`);
    await expect(page.getByText(/チーム定義 .* が見つかりません/)).toHaveCount(0);
    await opened.getByRole('tab', { name: /^規則/ }).click();
    await expect(page).toHaveURL((url) => url.searchParams.get('open') === ref && url.searchParams.get('agree') === ref);
    await page.reload();
    await ready(page);
    await expect(opened).toHaveAttribute('data-key', `core.${place}`);
    await opened.getByRole('tab', { name: /^トポロジー/ }).click();
    await expect(page).toHaveURL((url) => url.searchParams.get('open') === ref && !url.searchParams.has('agree'));
    await page.goto(`${app.base}/profiles`);
    await page.goBack();
    await ready(page);
    await expect(opened).toHaveCount(1);
    await expect(opened).toHaveAttribute('data-key', `core.${place}`);
  }
  await page.goto(`${app.base}/teams?open=core`);
  await ready(page);
  await expect(page.locator('.tdef.open')).toHaveCount(2);
  await page.goto(`${app.base}/teams?${new URLSearchParams({ open: 'core|/other/project' })}`);
  await ready(page);
  await expect(page.locator('.tdef.open')).toHaveCount(0);
  await expect(page.getByText('チーム定義 core が見つかりません。')).toBeVisible();
});
