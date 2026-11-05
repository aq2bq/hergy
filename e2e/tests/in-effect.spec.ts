import { test, expect } from './fixtures';
import { ready } from './helpers';
import { appendFileSync, copyFileSync, cpSync, mkdirSync, writeFileSync, readFileSync } from 'node:fs';
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
    // The place column carries the same answer: the project's row crosses out
    // the global place, and the global row names the project that overrides it.
    const projectRow = both.filter({ has: page.locator('.c-place .pill.shadow') });
    await expect(projectRow).toHaveCount(1);
    await expect(both.filter({ has: page.locator('.c-place .pill.hq-ov') })).toHaveCount(1);
    // The YAML tab's badge is the file row's mark, not the file's layer.
    await page.goto(`${app.base}/teams/yaml`); await ready(page);
    const yaml = page.locator('.yed .ph .state');
    await expect(yaml.filter({ hasText: '効いている' })).toHaveCount(1);
    await expect(yaml.filter({ hasText: '隠れている' })).toHaveCount(1);
  });
});

test.describe('roles', () => {
  test.use({ reads: 'roles' });

  test('roles: a name both layers hold', async ({ page, app }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(`${app.base}/roles`); await ready(page);
    const rows = page.locator('#rows .role');
    const both = rows.filter({ has: page.locator('.c-name .nm', { hasText: /^scout$/ }) });
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

  test('teams: a name both the global layer and a listed project hold', async ({ page, app }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    const alpha = path.join(app.home, 'src', 'alpha', '.hergy', 'teams');
    mkdirSync(alpha, { recursive: true });
    cpSync(path.join(app.cfg, 'hergy', 'teams', 'shared'), path.join(alpha, 'shared'), { recursive: true });
    await page.goto(`${app.base}/teams`); await ready(page);
    const rows = page.locator('#rows .tdef, #view .tdef');
    const both = rows.filter({ has: page.locator('.c-name .nm', { hasText: /^shared$/ }) });
    // The listed display does not call one project the hiding side: the global
    // row carries no 隠れている, and the project's row is in effect (r-ea17).
    await expect(both.filter({ hasText: '効いている' })).not.toHaveCount(0);
    await expect(both.filter({ hasText: '隠れている' })).toHaveCount(0);
  });

  test('profiles: a name both the global layer and a listed project hold', async ({ page, app }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    const file = path.join(app.home, 'src', 'alpha', '.hergy', 'profiles.yaml');
    appendFileSync(file, '  coder:\n    kind: claude\n    args: ["--model", "x"]\n');
    await page.goto(`${app.base}/profiles`); await ready(page);
    const rows = page.locator('#rows .row');
    const both = rows.filter({ has: page.locator('.c-name .nm', { hasText: /^coder$/ }) });
    await expect(both.filter({ hasText: '効いている' })).not.toHaveCount(0);
    await expect(both.filter({ hasText: '隠れている' })).toHaveCount(0);
  });

  test('roles: a name both the global layer and a listed project hold', async ({ page, app }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    const alpha = path.join(app.home, 'src', 'alpha', '.hergy', 'roles');
    mkdirSync(alpha, { recursive: true });
    copyFileSync(path.join(app.cfg, 'hergy', 'roles', 'lead.md'), path.join(alpha, 'lead.md'));
    await page.goto(`${app.base}/roles`); await ready(page);
    const rows = page.locator('#rows .role');
    const both = rows.filter({ has: page.locator('.c-name .nm', { hasText: /^lead$/ }) });
    await expect(both.filter({ hasText: '効いている' })).not.toHaveCount(0);
    await expect(both.filter({ hasText: '隠れている' })).toHaveCount(0);
  });
});
