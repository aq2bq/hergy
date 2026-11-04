import { test, expect } from './fixtures';
import { ready } from './helpers';
import { existsSync, readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { realpathSync } from 'node:fs';

test.use({ reads: 'teams' });

for (const place of ['project', 'global'] as const) {
  test(`rename a ${place} team definition without changing other bytes`, async ({ page, app }) => {
    const old = place === 'project' ? 'core' : 'draft';
    const dir = path.join(place === 'project' ? path.join(app.proj, '.hergy') : path.join(app.cfg, 'hergy'), 'teams');
    const file = path.join(dir, old, 'team.yaml');
    const before = `# keep this comment\nteam: '${old}' # keep this too\n${readFileSync(file, 'utf8')}`;
    writeFileSync(file, before);
    const agreeFile = path.join(dir, old, 'agreement.md');
    const agree = existsSync(agreeFile) ? readFileSync(agreeFile, 'utf8') : null;
    await page.goto(`${app.base}/teams?open=${old}`);
    await ready(page);
    const row = page.locator(`.tdef[data-name="${old}"]`);
    await row.getByRole('button', { name: '名前を変更', exact: true }).click();
    await row.getByLabel('新しいチーム定義名').fill('renamed');
    await expect(row.locator('.team-rename')).toContainText('起動の記録は変更前の名前で残ります');
    await row.getByRole('button', { name: '名前を保存', exact: true }).click();
    const renamed = page.locator('.tdef[data-name="renamed"]');
    await expect(renamed).toHaveClass(/open/);
    await expect(page).toHaveURL(/open=renamed/);
    expect(existsSync(path.join(dir, old))).toBe(false);
    expect(readFileSync(path.join(dir, 'renamed', 'team.yaml'), 'utf8')).toBe(before.replace(`team: '${old}'`, "team: 'renamed'"));
    if (agree !== null) expect(readFileSync(path.join(dir, 'renamed', 'agreement.md'), 'utf8')).toBe(agree);
    await expect(renamed.locator('.team-rename')).toContainText('起動の記録は変更前の名前で残ります');
    await page.goto(`${app.base}/teams?open=${old}`);
    await ready(page);
    await expect(page.getByRole('status')).toContainText('見つかりません');
  });
}

test('rename is blocked by unsaved edits and preserves a stale-file refusal', async ({ page, app }) => {
  await page.goto(`${app.base}/teams?open=core`);
  await ready(page);
  const row = page.locator('.tdef[data-name="core"]');
  const description = row.getByRole('textbox', { name: /の説明/ });
  const original = await description.inputValue();
  await description.fill('名前変更前の編集中');
  await expect(row.getByRole('button', { name: '名前を変更', exact: true })).toBeDisabled();
  await description.fill(original);
  await row.getByRole('button', { name: '名前を変更', exact: true }).click();
  await row.getByLabel('新しいチーム定義名').fill('renamed');
  const file = path.join(app.proj, '.hergy/teams/core/agreement.md');
  const changed = `${readFileSync(file, 'utf8')}\n# another editor\n`;
  writeFileSync(file, changed);
  await row.getByRole('button', { name: '名前を保存', exact: true }).click();
  await expect(row.getByRole('alert')).toContainText('画面を読み直して');
  await expect(row.getByLabel('新しいチーム定義名')).toHaveValue('renamed');
  expect(readFileSync(file, 'utf8')).toBe(changed);
  expect(existsSync(path.join(app.proj, '.hergy/teams/renamed'))).toBe(false);
});

test.describe('registered projects', () => {
  test.use({ reads: 'hq' });
  test('a rename keeps another project draft and opens its new scoped row', async ({ page, app }) => {
    await page.goto(`${app.base}/teams`); await ready(page);
    const alpha = realpathSync(app.proj), beta = realpathSync(path.join(app.home, 'src/beta'));
    const other = page.locator(`.tdef[data-name="beta-dev|${beta}"]`);
    await other.getByRole('button', { name: /を開閉する/ }).click();
    const draft = other.getByRole('textbox', { name: /の説明/ });
    await draft.fill('ほかのプロジェクトの下書き');
    const row = page.locator(`.tdef[data-name="alpha-dev|${alpha}"]`);
    await row.getByRole('button', { name: /を開閉する/ }).click();
    await row.getByRole('button', { name: '名前を変更', exact: true }).click();
    await row.getByLabel('新しいチーム定義名').fill('renamed');
    await row.getByRole('button', { name: '名前を保存', exact: true }).click();
    await expect(page.locator(`.tdef[data-name="renamed|${alpha}"]`)).toHaveClass(/open/);
    await expect(draft).toHaveValue('ほかのプロジェクトの下書き');
  });
});
