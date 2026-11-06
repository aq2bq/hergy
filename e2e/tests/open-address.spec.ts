// ⑥-2 / d-a143 — the address of the row a person opens. A name both layers hold
// is two rows: pressing one opens that row alone, the address names its place,
// and opening the address again opens the same row. The old address, a name
// with no place, opens both rows of that name.
import { test, expect } from './fixtures';
import { cpSync } from 'node:fs';
import path from 'node:path';
import { ready } from './helpers';

test.describe('teams', () => {
  test.use({ reads: 'teams' });

  test('the row pressed opens alone, and the old address opens both', async ({ page, app }) => {
    cpSync(path.join(app.proj, '.hergy', 'teams', 'core'), path.join(app.cfg, 'hergy', 'teams', 'core'), { recursive: true });
    await page.goto(`${app.base}/teams`); await ready(page);
    const opened = page.locator('.tdef.open');
    await page.locator('.tdef[data-key="core.project"] .sum').click();
    await expect(page).toHaveURL((u) => u.searchParams.get('open') === 'core|project');
    await expect(opened).toHaveCount(1);
    await expect(opened).toHaveAttribute('data-key', 'core.project');
    // Opening the address again opens the same row.
    await page.reload(); await ready(page);
    await expect(opened).toHaveCount(1);
    await expect(opened).toHaveAttribute('data-key', 'core.project');
    // The other row is its own address.
    await opened.locator('.sum').click();
    await expect(page).toHaveURL((u) => !u.searchParams.has('open'));
    await page.locator('.tdef[data-key="core.global"] .sum').click();
    await expect(page).toHaveURL((u) => u.searchParams.get('open') === 'core|global');
    await expect(opened).toHaveCount(1);
    await expect(opened).toHaveAttribute('data-key', 'core.global');
    // The old address names no place: both rows stand open.
    await page.goto(`${app.base}/teams?open=core`); await ready(page);
    await expect(opened).toHaveCount(2);
  });
});

test.describe('roles', () => {
  test.use({ reads: 'roles' });

  test('a named place opens its row alone, and the old name opens both', async ({ page, app }) => {
    await page.goto(`${app.base}/roles?open=scout|global`); await ready(page);
    const opened = page.locator('.role.open');
    await expect(opened).toHaveCount(1);
    await expect(opened.locator('.panel[data-l=global]')).toHaveCount(1);
    await page.reload(); await ready(page);
    await expect(opened).toHaveCount(1);
    await expect(opened.locator('.panel[data-l=global]')).toHaveCount(1);
    await page.goto(`${app.base}/roles?open=scout`); await ready(page);
    await expect(opened).toHaveCount(2);
  });
});

test.describe('profiles', () => {
  test.use({ reads: 'profiles' });

  test('a named place opens its row alone, and the old name opens both', async ({ page, app }) => {
    await page.goto(`${app.base}/profiles?open=sonnet-fast|global`); await ready(page);
    const opened = page.locator('.row.open');
    await expect(opened).toHaveCount(1);
    await expect(opened).toHaveAttribute('data-group', 'global');
    await page.reload(); await ready(page);
    await expect(opened).toHaveCount(1);
    await expect(opened).toHaveAttribute('data-group', 'global');
    await page.goto(`${app.base}/profiles?open=sonnet-fast`); await ready(page);
    await expect(opened).toHaveCount(2);
  });
});
