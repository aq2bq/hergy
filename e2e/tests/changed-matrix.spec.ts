import { test, expect } from './fixtures';
import { ready } from './helpers';
import { chmodSync } from 'node:fs';
import path from 'node:path';

// r-1e9a: one owner decides 「変えたか」. The band's dot, the row's 未保存/N か所,
// 元に戻す and the discard's question agree, for every editable thing, way of
// making it, and operation (untouched, one change, change back).
const dot = (page, screen: string) => page.locator(`.scr[data-screen="${screen}"] .dot`);

test.describe('teams', () => {
  test.use({ reads: 'teams' });
  test('a saved description: change, back, revert and discard agree', async ({ page, app }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(`${app.base}/teams?open=core`); await ready(page);
    const row = page.locator('.tdef[data-name="core"]');
    const field = row.locator('.in-description');
    const before = await field.inputValue();
    await expect(dot(page, 'teams')).toHaveCount(0);
    await field.fill(before + 'X');
    await expect(dot(page, 'teams')).toHaveCount(1);
    await expect(row.locator('.dirty')).toContainText('か所');
    await expect(row.locator('[data-act="t-revert"]')).toBeVisible();
    await field.fill(before);
    await expect(dot(page, 'teams')).toHaveCount(0);
    await expect(row.locator('[data-act="t-revert"]')).toBeHidden();

    // A new row that nobody touched goes at once; one change asks first.
    await page.locator('.list .hq-gh[data-place="project"] [data-act="new"]').first().click();
    const draft = page.locator('.tdef[data-name^="~new"]');
    await expect(draft).toHaveCount(1);
    await expect(dot(page, 'teams')).toHaveCount(1);
    await draft.locator('.in-name').fill('matrix-new');
    await draft.locator('[data-act="discard"]').click();
    await expect(page.locator('.confirm')).toBeVisible();
    await page.locator('.confirm .btn').first().click(); // やめる
    await expect(draft).toHaveCount(1);
  });
});

test.describe('roles', () => {
  test.use({ reads: 'roles' });
  test('a role document: change, back, and the carried edit keeps the dot', async ({ page, app }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(`${app.base}/roles?open=builder`); await ready(page);
    const row = page.locator('#rows .role.open');
    const field = row.locator('[data-f="name"]').first();
    const before = await field.inputValue();
    await expect(dot(page, 'roles')).toHaveCount(0);
    await field.fill('builder-x');
    await expect(dot(page, 'roles')).toHaveCount(1);
    // The band carries the edit to another screen and back: the dot stays.
    await page.locator('a.scr[data-screen="profiles"]').click(); await ready(page);
    await expect(dot(page, 'roles')).toHaveCount(1);
    await page.locator('a.scr[data-screen="roles"]').click(); await ready(page);
    await expect(dot(page, 'roles')).toHaveCount(1);
    await page.locator('#rows .role.open [data-f="name"]').first().fill(before);
    await expect(dot(page, 'roles')).toHaveCount(0);
  });
});

test.describe('profiles', () => {
  test.use({ reads: 'profiles' });
  test('a profile panel: change, back, and the new row asks before it goes', async ({ page, app }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(`${app.base}/profiles?open=codex-mini`); await ready(page);
    const row = page.locator('#rows .row[data-name="codex-mini"]');
    const field = row.locator('.in-name').first();
    const before = await field.inputValue();
    await expect(dot(page, 'profiles')).toHaveCount(0);
    await field.fill('codex-x');
    await expect(dot(page, 'profiles')).toHaveCount(1);
    await field.fill(before);
    await expect(dot(page, 'profiles')).toHaveCount(0);
  });
});

test.describe('unreadable rows', () => {
  test.use({ reads: 'teams' });
  test('a file that cannot be read carries no mark and no dot', async ({ page, app }) => {
    const files = [
      path.join(app.proj, '.hergy/teams/core/team.yaml'),
      path.join(app.proj, '.hergy/roles/lead.md'),
      path.join(app.proj, '.hergy/profiles.yaml'),
    ];
    for (const f of files) chmodSync(f, 0o000);
    await page.setViewportSize({ width: 1440, height: 900 });
    for (const screen of ['teams', 'roles', 'profiles']) {
      await page.goto(`${app.base}/${screen}`); await ready(page);
      await expect(dot(page, screen)).toHaveCount(0);
    }
    for (const f of files) chmodSync(f, 0o644);
  });
});

test.describe('the revert and its undo, in the field', () => {
  test.use({ reads: 'teams' });
  test('teams figure: 元に戻す puts the draft back and 取り消す brings it', async ({ page, app }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(`${app.base}/teams?open=core`); await ready(page);
    const row = page.locator('.tdef[data-name="core"]');
    const field = row.locator('.in-description');
    const before = await field.inputValue();
    await field.fill(before + 'Y');
    await row.locator('[data-act="t-revert"]').click();
    await expect(field).toHaveValue(before);
    await expect(row.locator('.undo')).toContainText('取り消す');
    await row.locator('.undo button').click();
    await expect(field).toHaveValue(before + 'Y');
    await expect(dot(page, 'teams')).toHaveCount(1);
  });
});
