import { test, expect } from './fixtures';
import { ready } from './helpers';
import { copyFileSync, mkdirSync } from 'node:fs';
import path from 'node:path';

// r-ea17: a name both layers hold is two rows, one 効いている and one
// 隠れている, on all three screens; a name only one layer holds carries no
// marker.
const marks = async (rows, name: string) => {
  const two = rows.filter({ has: rows.page().locator(`text="${name}"`) });
  return two;
};

test.describe('one project', () => {
  test.use({ reads: 'teams' });

  test('teams: a name both places hold', async ({ page, app }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(`${app.base}/teams`); await ready(page);
    const rows = page.locator('#view .tdef');
    const both = rows.filter({ has: page.locator('.c-name .nm', { hasText: /^docs$/ }) });
    await expect(both).toHaveCount(2);
    await expect(both.filter({ hasText: '効いている' })).toHaveCount(1);
    await expect(both.filter({ hasText: '隠れている' })).toHaveCount(1);
    const one = rows.filter({ has: page.locator('.c-name .nm', { hasText: /^research$/ }) });
    await expect(one).not.toContainText('効いている');
    await expect(one).not.toContainText('隠れている');
  });
});

test.describe('roles', () => {
  test.use({ reads: 'roles' });

  test('roles: a name both layers hold', async ({ page, app }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(`${app.base}/roles`); await ready(page);
    const rows = page.locator('#rows .role');
    const both = rows.filter({ has: page.locator('.c-name .nm', { hasText: /^lead$/ }) });
    await expect(both).toHaveCount(2);
    await expect(both.filter({ hasText: '効いている' })).toHaveCount(1);
    await expect(both.filter({ hasText: '隠れている' })).toHaveCount(1);
    const one = rows.filter({ has: page.locator('.c-name .nm', { hasText: /^analyst$/ }) });
    await expect(one).not.toContainText('効いている');
  });
});

test.describe('profiles', () => {
  test.use({ reads: 'profiles' });

  test('profiles: a name both files hold', async ({ page, app }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(`${app.base}/profiles`); await ready(page);
    const rows = page.locator('#rows .row');
    const both = rows.filter({ has: page.locator('.c-name .nm', { hasText: /^sonnet-fast$/ }) });
    await expect(both).toHaveCount(2);
    await expect(both.filter({ hasText: '効いている' })).toHaveCount(1);
    await expect(both.filter({ hasText: '隠れている' })).toHaveCount(1);
  });
});

test.describe('the headquarters list', () => {
  test.use({ reads: 'hq' });

  test('roles: a name both the global layer and a listed project hold', async ({ page, app }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    const alpha = path.join(app.home, 'src', 'alpha', '.hergy', 'roles');
    mkdirSync(alpha, { recursive: true });
    copyFileSync(path.join(app.cfg, 'hergy', 'roles', 'lead.md'), path.join(alpha, 'lead.md'));
    await page.goto(`${app.base}/roles`); await ready(page);
    const rows = page.locator('#rows .role');
    const both = rows.filter({ has: page.locator('.c-name .nm', { hasText: /^lead$/ }) });
    await expect(both.filter({ hasText: '効いている' })).not.toHaveCount(0);
    await expect(both.filter({ hasText: '隠れている' })).not.toHaveCount(0);
  });
});
