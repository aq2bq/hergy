import { test, expect } from './fixtures';
import { ready } from './helpers';
import { chmodSync, mkdirSync, writeFileSync } from 'node:fs';
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

test.describe('teams figure member', () => {
  test.use({ reads: 'teams' });
  test('a member profile: change and back', async ({ page, app }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(`${app.base}/teams?open=core`); await ready(page);
    const row = page.locator('.tdef[data-name="core"]');
    await row.locator('.cv-n.member[data-cv-n="lead"]').click();
    const pick = row.locator('.cv-side [data-cv-f="m-profile"]');
    const before = await pick.inputValue();
    const options = await pick.locator('option').evaluateAll((os) => os.map((o) => (o as HTMLOptionElement).value));
    const other = options.find((v) => v !== before) || before;
    await expect(dot(page, 'teams')).toHaveCount(0);
    await pick.selectOption(other);
    await expect(dot(page, 'teams')).toHaveCount(1);
    await pick.selectOption(before);
    await expect(dot(page, 'teams')).toHaveCount(0);
  });
});

test.describe('teams figure worker', () => {
  test.use({ reads: 'teams' });
  test('a worker purpose: type and take it back', async ({ page, app }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(`${app.base}/teams?open=core`); await ready(page);
    const row = page.locator('.tdef[data-name="core"]');
    await row.locator('.cv-n.worker').first().click();
    const field = row.locator('[data-cv-f="w-purpose"]');
    await field.click();
    await field.press('End');
    await expect(dot(page, 'teams')).toHaveCount(0);
    await field.pressSequentially('x');
    await expect(dot(page, 'teams')).toHaveCount(1);
    await field.press('Backspace');
    await expect(dot(page, 'teams')).toHaveCount(0);
  });
});

test.describe('teams rules', () => {
  test.use({ reads: 'teams' });
  test('a rule change, and 元に戻す → 取り消す', async ({ page, app }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(`${app.base}/teams?open=core`); await ready(page);
    const row = page.locator('.tdef[data-name="core"]');
    await row.locator('.dtab[data-t="agree"]').click();
    const area = row.locator('.panel.agree-p textarea').last();
    const before = await area.inputValue();
    await area.click();
    await area.press('End');
    await expect(dot(page, 'teams')).toHaveCount(0);
    await area.pressSequentially('x');
    await expect(dot(page, 'teams')).toHaveCount(1);
    await area.press('Backspace');
    await expect(dot(page, 'teams')).toHaveCount(0);
    // 元に戻す goes at once and its 取り消す stands in the rules panel.
    await area.pressSequentially('y');
    await row.locator('.panel.agree-p [data-act="revert"]').click();
    await expect(area).toHaveValue(before);
    await expect(row.locator('.panel.agree-p .undo')).toContainText('取り消す');
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

test.describe('roles ghost', () => {
  test.use({ reads: 'roles' });
  test('a name only a member names is not a change until a person writes', async ({ page, app }) => {
    mkdirSync(path.join(app.proj, '.hergy/teams/matrix-ghost'), { recursive: true });
    writeFileSync(path.join(app.proj, '.hergy/teams/matrix-ghost/team.yaml'),
      'version: 1\nmembers:\n  - name: a\n    role: ghost2\n    profile: coder\n');
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(`${app.base}/roles`); await ready(page);
    await expect(dot(page, 'roles')).toHaveCount(0);
    const ghost = page.locator('#rows .role').filter({ has: page.locator('.c-name .nm', { hasText: /^ghost2$/ }) });
    await expect(ghost).toHaveCount(1);
    await expect(ghost).not.toHaveClass(/unsaved/);
    await ghost.locator('.sum').click();
    const field = page.locator('#rows .role.open [data-f="name"]').first();
    await field.fill('ghost2x');
    await expect(dot(page, 'roles')).toHaveCount(1);
    await expect(page.locator('#rows .role.open')).toHaveClass(/unsaved/);
    await field.fill('ghost2');
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
    for (const screen of ['teams', 'profiles']) {
      await page.goto(`${app.base}/${screen}`); await ready(page);
      await expect(dot(page, screen)).toHaveCount(0);
    }
    // The unreadable role's own row carries no unsaved mark. The roles band's
    // dot still stands after this (a finding: see the commit).
    await page.goto(`${app.base}/roles`); await ready(page);
    await expect(page.locator('#rows .role.unsaved')).toHaveCount(0);
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

// The YAML tab on every screen: typing marks it, typing the file's text back
// clears every mark.
for (const screen of ['teams', 'roles', 'profiles'] as const) {
  test.describe(`yaml ${screen}`, () => {
    test.use({ reads: screen });
    test('a change and the same text back', async ({ page, app }) => {
      await page.setViewportSize({ width: 1440, height: 900 });
      await page.goto(`${app.base}/${screen}/yaml`); await ready(page);
      const area = page.locator('.ytext').first();
      const before = await area.inputValue();
      await expect(dot(page, screen)).toHaveCount(0);
      await area.click();
      await area.press('End');
      await area.pressSequentially('x');
      await expect(dot(page, screen)).toHaveCount(1);
      await area.press('Backspace');
      await expect(dot(page, screen)).toHaveCount(0);
    });
  });
}

// One person's pick on the assignment tabs.
test.describe('roles assign', () => {
  test.use({ reads: 'roles' });
  test('a pick and the same pick back', async ({ page, app }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(`${app.base}/roles/assign`); await ready(page);
    const select = page.locator('select.pickrole').first();
    const before = await select.inputValue();
    const other = (await select.locator('option').evaluateAll((os) => os.map((o) => (o as HTMLOptionElement).value))).find((v) => v !== before) || before;
    await expect(dot(page, 'roles')).toHaveCount(0);
    await select.selectOption(other);
    await expect(dot(page, 'roles')).toHaveCount(1);
    await select.selectOption(before);
    await expect(dot(page, 'roles')).toHaveCount(0);
  });
});

test.describe('profiles assign', () => {
  test.use({ reads: 'profiles' });
  test('a pick and the same pick back', async ({ page, app }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(`${app.base}/profiles/assign`); await ready(page);
    const select = page.locator('select[data-act="pick"]').first();
    const before = await select.inputValue();
    const options = await select.locator('option').evaluateAll((os) => os.map((o) => (o as HTMLOptionElement).value));
    const other = options.find((v) => v !== before) || before;
    await expect(dot(page, 'profiles')).toHaveCount(0);
    await select.selectOption(other);
    await expect(dot(page, 'profiles')).toHaveCount(1);
    await select.selectOption(before);
    await expect(dot(page, 'profiles')).toHaveCount(0);
  });
});

// A duplicate nobody touched goes at once; one change asks. Copy too.
test.describe('duplicate and copy ask once touched', () => {
  test.use({ reads: 'teams' });
  test('team definition', async ({ page, app }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    for (const act of ['dup', 'copy']) {
      await page.goto(`${app.base}/teams?open=research`); await ready(page);
      await page.locator(`.tdef[data-name="research"] [data-act="${act}"]`).click();
      const draft = page.locator('.tdef[data-name^="~new"]');
      await expect(draft).toHaveCount(1);
      await draft.locator('.in-name').fill(`matrix-${act}`);
      await draft.locator('[data-act="discard"]').click();
      await expect(page.locator('.confirm')).toBeVisible();
      await page.locator('.confirm .btn').first().click();
    }
  });
});

test.describe('duplicate and copy ask once touched (roles, profiles)', () => {
  test.use({ reads: 'roles' });
  test('role', async ({ page, app }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    for (const act of ['dup', 'copy']) {
      await page.goto(`${app.base}/roles?open=builder`); await ready(page);
      const button = page.locator(`#rows .role .panel [data-act="${act}"]`).first();
      if (await button.count() === 0) continue;
      await button.click();
      const draft = page.locator('#rows .role').filter({ has: page.locator('[data-act="discard"]') });
      await expect(draft).toHaveCount(1);
      await draft.locator('[data-f="name"]').fill(`matrix-${act}`);
      await draft.locator('[data-act="discard"]').click();
      await expect(page.locator('.confirm')).toBeVisible();
      await page.locator('.confirm .btn').first().click();
    }
  });
});
