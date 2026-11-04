import { test, expect } from './fixtures';
import { ready } from './helpers';
import path from 'node:path';
import { readFileSync, realpathSync, rmSync } from 'node:fs';

test.describe('UI review: project-scoped editors', () => {
  test.use({ reads: 'hq' });
  for (const screen of ['/teams/yaml', '/roles/assign', '/profiles/assign', '/roles/yaml', '/profiles/yaml']) {
    test(`${screen} identifies and switches the editing project`, async ({ page, app }) => {
      await page.goto(app.base + screen);
      await ready(page);
      const project = page.getByRole('combobox', { name: '表示するプロジェクト' });
      const beta = realpathSync(path.join(app.home, 'src', 'beta'));
      await project.selectOption(beta);
      await ready(page);
      await expect(project).toHaveValue(beta);
      await expect(page).toHaveURL(/project=.*beta/);
      await expect(page.getByText('このプロジェクトとグローバルの設定を表示しています。')).toBeVisible();
      if (screen === '/profiles/yaml') {
        const editor = page.locator('.yed').first();
        const field = editor.locator('.ytext');
        const alphaFile = path.join(app.home, 'src', 'alpha', '.hergy', 'profiles.yaml');
        const originalAlpha = readFileSync(alphaFile, 'utf8');
        await field.fill((await field.inputValue()) + '\n# UI review beta save\n');
        await editor.locator('[data-act=yaml-save]').click();
        await expect(editor.locator('.ok')).toHaveText('保存しました');
        expect(readFileSync(path.join(beta, '.hergy', 'profiles.yaml'), 'utf8')).toContain('# UI review beta save');
        expect(readFileSync(alphaFile, 'utf8')).toBe(originalAlpha);
        await page.getByRole('tab', { name: '割り当て' }).click();
        await expect(project).toHaveValue(beta);
      }
      if (screen === '/roles/assign') {
        await expect(page.locator('.rolebar')).toContainText('beta');
        await expect(page.locator('.rolebar')).not.toContainText('alpha');
      }
    });
  }
});

test.describe('UI review: waiting and editing', () => {
  test.use({ reads: 'activity' });
  test('permission shows the command and offers a guarded focus action', async ({ page, app }, testInfo) => {
    await page.goto(app.base + '/activity'); await ready(page);
    const waiting = page.locator('.hq-at[data-pick="migrate"]');
    await expect(waiting).toContainText('0042_split_ledger.sql');
    const denied = await page.request.post(app.base + '/api/activity/focus', { data: { team: 'migrate', member: 'migrate-tester' } });
    expect(denied.status()).toBe(403);
    const request = page.waitForRequest((r) => r.url().endsWith('/api/activity/focus') && r.method() === 'POST');
    await waiting.getByRole('button', { name: '端末で返答する' }).click();
    const focused = await request;
    expect(focused.postDataJSON()).toEqual({ team: 'migrate', member: 'migrate-tester' });
    await page.screenshot({ path: testInfo.outputPath('activity.png'), fullPage: true });
  });
  test('768px editors have headings, separate controls and no page overflow', async ({ page, app }, testInfo) => {
    await page.setViewportSize({ width: 768, height: 1000 });
    for (const screen of ['teams', 'roles', 'profiles']) {
      await page.goto(app.base + '/' + screen); await ready(page);
      await expect(page.getByRole('heading', { level: 1 })).toHaveCount(1);
      await expect(page.locator('[role="button"] button, button button')).toHaveCount(0);
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await page.screenshot({ path: testInfo.outputPath(`${screen}-768.png`), fullPage: true });
    }
  });
  test('an optional empty agreement offers full-text editing without an error', async ({ page, app }) => {
    rmSync(path.join(app.proj, '.hergy', 'teams', 'core', 'agreement.md'), { force: true });
    await page.goto(app.base + '/teams?open=core'); await ready(page);
    const row = page.locator('.tdef[data-name="core"]');
    await row.locator('[data-act="dtab"][data-t="agree"]').click();
    await expect(row.getByPlaceholder('チームの規則は任意です。本文を入力して保存できます。')).toBeVisible();
    await expect(row.locator('.agree-p .vm')).not.toContainText('問題');
    await expect(row.locator('.agree-p').getByRole('button', { name: '元に戻す' })).not.toBeVisible();
    await expect(row.locator('.agree-p').getByRole('textbox', { name: '規則の全文' })).toBeVisible();
  });
  test('end sheet stays by its trigger and explains the irreversible result', async ({ page, app }, testInfo) => {
    await page.goto(app.base + '/activity'); await ready(page);
    const trigger = page.locator('.hq-tile[data-team="core"] [data-hq="stop"]');
    await trigger.click();
    const sheet = page.getByRole('dialog');
    await expect(sheet.getByText('会話は残りません。この操作は取り消せません。')).toBeVisible();
    const buttonRect = (await trigger.boundingBox())!;
    const sheetRect = (await sheet.boundingBox())!;
    expect(Math.abs(buttonRect.x - sheetRect.x)).toBeLessThan(610);
    expect(sheetRect.y).toBeGreaterThan(100);
    await page.screenshot({ path: testInfo.outputPath('stop.png'), fullPage: true });
    await sheet.getByRole('button', { name: 'やめる', exact: true }).click();
    await expect(sheet).toHaveCount(0);
  });
  test('empty profile explains disabled save, then saves normally', async ({ page, app }) => {
    await page.goto(app.base + '/profiles'); await ready(page);
    await page.getByRole('button', { name: '＋ proj にプロファイルを追加', exact: true }).click();
    await expect(page.getByText('プロファイル名を入力してください。')).toBeVisible();
    await page.getByRole('textbox', { name: 'プロファイル名', exact: true }).fill('ui-review');
    await page.locator('.panel [data-f="arg"]').last().fill('review-model');
    await page.getByRole('button', { name: '保存', exact: true }).click();
    await expect(page.getByText('保存しました', { exact: true })).toBeVisible();
    expect(readFileSync(path.join(app.proj, '.hergy', 'profiles.yaml'), 'utf8')).toContain('ui-review:');
  });
});
