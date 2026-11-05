import { test, expect, type Page } from './fixtures';
import { ready } from './helpers';
import { readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
test.use({ reads: 'teams' });
async function open(page: Page, base: string) {
  await page.goto(`${base}/teams?open=core`); await ready(page);
  const row = page.locator('.tdef[data-name="core"]');
  await row.locator('[data-cv-n="impl-a"]').click();
  await row.getByRole('button', { name: 'ロールを確認・編集' }).first().click();
 await row.getByRole('button', { name: 'すべて表示', exact: true }).click();
  const column = row.locator('.role-column').filter({ has: page.getByRole('heading', { name: /builder/ }) });
  await expect(column.getByRole('textbox', { name: '規則の全文' })).toBeEnabled();
  return { row, column };
}
test('the topology action opens a single shared role column and keeps both drafts', async ({ page, app }) => {
  const { row, column } = await open(page, app.base);
  await expect(row.locator('.role-column')).toHaveCount(4);
  const input = column.getByRole('textbox', { name: '規則の全文' });
  await input.fill('# 編集中\n残す本文\n');
  await row.getByRole('tab', { name: 'トポロジー', exact: true }).click();
  await row.getByRole('button', { name: 'チームの説明を編集' }).click();
  await row.getByRole('textbox', { name: '説明', exact: true }).fill('編集中の説明');
  await row.getByRole('tab', { name: /^規則/ }).click();
  await expect(input).toHaveValue('# 編集中\n残す本文\n');
  await expect(row.locator('.sum .c-description')).toHaveText('編集中の説明');
});
test('saving a shared role writes only its actual file and another team reads it', async ({ page, app }) => {
  const file = path.join(app.proj, '.hergy/roles/builder.md');
  const team = path.join(app.proj, '.hergy/teams/core/team.yaml');
  const agree = path.join(app.proj, '.hergy/teams/core/agreement.md');
  const before = [readFileSync(team, 'utf8'), readFileSync(agree, 'utf8')];
  const { column } = await open(page, app.base);
  await expect(column.locator('details')).toHaveCount(0);
  await expect(page.getByText('確認できた範囲', { exact: true })).toHaveCount(1);
  await expect(page.getByText('確認できた範囲', { exact: true })).toBeVisible();
  await expect(column.locator('.prose-other-teams').filter({ hasText: 'migrate' })).toBeVisible();
  await expect(column.locator('.prose-other-teams')).toHaveAttribute('title', /migrate/);
  const text = '# 共有の設計担当\n全文を更新する。\n';
  await column.getByRole('textbox', { name: '規則の全文' }).fill(text);
  await column.getByRole('button', { name: '保存', exact: true }).click();
  await expect(column.locator('.prose-status > span')).toHaveText('保存しました');
  expect(readFileSync(file, 'utf8')).toBe(text);
  expect([readFileSync(team, 'utf8'), readFileSync(agree, 'utf8')]).toEqual(before);
  await page.goto(`${app.base}/teams?open=migrate&agree=migrate`); await ready(page);
 await page.getByRole('button', { name: 'すべて表示', exact: true }).click();
  await expect(page.locator('.role-column').filter({ has: page.getByRole('heading', { name: /builder/ }) }).getByRole('textbox', { name: '規則の全文' })).toHaveValue(text);
});
test('a shared role refuses a concurrent change and keeps the draft', async ({ page, app }) => {
  const { column } = await open(page, app.base);
  const file = path.join(app.proj, '.hergy/roles/builder.md');
  const text = '# 手元の本文\n';
  await column.getByRole('textbox', { name: '規則の全文' }).fill(text);
  const external = '# ほかの編集\n'; writeFileSync(file, external);
  await column.getByRole('button', { name: '保存', exact: true }).click();
  await expect(column.locator('.conflict')).toBeVisible();
  await expect(column.getByRole('textbox', { name: '規則の全文' })).toHaveValue(text);
  expect(readFileSync(file, 'utf8')).toBe(external);
});
test('a failed usage read holds saving and keeps entered prose', async ({ page, app }) => {
  const { column } = await open(page, app.base);
  await column.getByRole('textbox', { name: '規則の全文' }).fill('# 残す編集\n');
  await page.route('**/api/role/*/context?**', route => route.fulfill({ status: 503, json: { text: '使用先を確認できません' } }));
  page.once('dialog', question => question.accept());
  await column.getByRole('button', { name: 'ファイルからリロード' }).click();
  await expect(column.getByRole('alert').filter({ hasText: '使用先を確認できません' })).toBeVisible();
  await expect(column.getByRole('button', { name: '保存', exact: true })).toBeDisabled();
  await expect(column.getByRole('textbox', { name: '規則の全文' })).toHaveValue('# 残す編集\n');
});

test('the team budget warns on prose plus template while roles count their own text', async ({ page, app }) => {
  const file = path.join(app.proj, '.hergy/teams/core/team.yaml');
  writeFileSync(file, readFileSync(file, 'utf8') + '\nprompt: "' + '字'.repeat(100) + '%{agreement}%{roster}%{contact}%{turns}"\n');
  await page.goto(`${app.base}/teams?open=core&agree=core`); await ready(page);
 await page.getByRole('button', { name: 'すべて表示', exact: true }).click();
  const team = page.locator('.tdef[data-name="core"] .agree-p');
  await team.getByRole('textbox', { name: '規則の全文' }).fill('文'.repeat(1150));
  await expect(team.locator('.prose-budget .v')).toHaveText('1,250 字 · 目安 1,200 字');
  await expect(team.locator('.prose-budget .amt')).toHaveClass(/over/);
  await expect(team.locator('.prose-budget')).toHaveAttribute('title', '内訳: 規則 1,150 字・ひな形 100 字');
  await expect(team.locator('.prose-budget .v')).toHaveCount(1);
  await team.getByRole('textbox', { name: '規則の全文' }).fill('文'.repeat(1100));
  await expect(team.locator('.prose-budget .amt')).not.toHaveClass(/over/);
  const role = page.locator('.role-column').first();
  await role.getByRole('textbox', { name: '規則の全文' }).fill('文'.repeat(1201));
  await expect(role.locator('.prose-budget .v')).toHaveText('1,201 字 · 目安 1,200 字');
  await expect(role.locator('.prose-budget .amt')).toHaveClass(/over/);
});

test('caret movement updates the block count, blur clears it, and outline keeps the editor frame', async ({ page, app }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  const { column } = await open(page, app.base);
  const input = column.getByRole('textbox', { name: 'builder の規則の全文', exact: true });
  await input.fill('# One\nabc\n## Two\nxyz');
  await input.press('ArrowUp'); await input.press('ArrowUp'); await input.press('ArrowUp');
  await expect(column.locator('.prose-current')).toHaveText('入力位置: One（10 字）');
  await input.press('ArrowDown'); await input.press('ArrowDown'); await input.press('ArrowDown');
  await expect(column.locator('.prose-current')).toHaveText('入力位置: Two（10 字）');
  const frame = await input.boundingBox();
  await column.getByRole('button', { name: '見出しだけ', exact: true }).click();
  await expect(column.locator('.prose-current')).toHaveCount(0);
  await expect(column.getByRole('button', { name: '見出しだけ', exact: true })).toHaveAttribute('aria-pressed', 'true');
  const outline = column.locator('.prose-outline');
  await expect.poll(() => outline.boundingBox()).toEqual(frame);
  await expect(outline.locator('li').nth(1)).toHaveCSS('padding-left', '14px');
  const save = await column.getByRole('button', { name: '保存', exact: true }).boundingBox();
  expect(save!.y + save!.height).toBeLessThanOrEqual(900);
});

test('a missing role explains the next action outside collapsed details', async ({ page, app }) => {
  await page.route('**/api/role/builder/context?**', async route => {
    const response = await route.fetch();
    const data = await response.json();
    await route.fulfill({ json: { ...data, state: 'missing', doc: null, problem: 'builder の文書がありません。', errors: [], scanned: [] } });
  });
  await page.goto(`${app.base}/teams?open=core&agree=core`); await ready(page);
 await page.getByRole('button', { name: 'すべて表示', exact: true }).click();
  const column = page.locator('.role-column').filter({ has: page.getByRole('heading', { name: /builder/ }) });
  await expect(column.getByRole('alert')).toHaveText('builder の文書がありません。');
  await expect(column.getByRole('link', { name: 'ロールの画面で作る' })).toBeVisible();
  await expect(column.locator('.prose-budget, footer, details')).toHaveCount(0);
  await expect(column.getByRole('link')).toHaveCount(1);
  await expect(column.locator('.prose-kind')).toHaveText('ロール');
  const readable = page.locator('.role-column').filter({ has: page.locator('textarea') }).first();
  await expect.poll(async () => {
    const missing = (await column.locator('.prose-unavailable').boundingBox())!;
    const editor = (await readable.locator('textarea').boundingBox())!;
    return { top: missing.y - editor.y, height: missing.height - editor.height };
  }).toEqual({ top: 0, height: 0 });
});

test('opening prose allocates a stable viewport below the real band and keeps save visible', async ({ page, app }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(`${app.base}/teams?open=core&agree=core`); await ready(page);
 await page.getByRole('button', { name: 'すべて表示', exact: true }).click();
  const board = page.locator('.prose-board');
  const input = board.locator('.agree-p textarea');
  await expect.poll(async () => (await input.boundingBox())!.height).toBeGreaterThan(320);
  await expect.poll(async () => page.locator('.prose-tools').evaluate(n => {
    const band = document.querySelector('.bar')!.getBoundingClientRect();
    const summary = n.closest('.tdef')!.querySelector(':scope > .sum')!.getBoundingClientRect();
    return n.getBoundingClientRect().top >= Math.max(band.bottom, summary.bottom);
  })).toBe(true);
  await expect(page.locator('.prose-drag-hint')).toBeHidden();
  const before = (await board.boundingBox())!;
  const height = (await input.boundingBox())!.height;
  expect(before.y).toBeGreaterThan(80);
  expect(before.y + before.height).toBeLessThanOrEqual(900);
  await input.hover();
  await page.mouse.wheel(0, 700);
  await expect.poll(async () => (await input.boundingBox())!.height).toBe(height);
  await expect.poll(async () => (await board.boundingBox())!.y).toBe(before.y);
  await expect(board.locator('.role-dialog-body').first()).toHaveCSS('overflow', 'hidden');
  const save = board.locator('.agree-p').getByRole('button', { name: '保存', exact: true });
  await expect.poll(async () => { const b = (await save.boundingBox())!; return b.y + b.height; }).toBeLessThanOrEqual(900);
});

test('file reload asks to discard drafts and reloads both text and usage', async ({ page, app }) => {
  const { column } = await open(page, app.base);
  const input = column.getByRole('textbox', { name: '規則の全文' });
  const reload = column.getByRole('button', { name: 'ファイルからリロード' });
  await input.fill('# 保存していない編集\n');
  page.once('dialog', async question => {
    expect(question.message()).toBe('保存していない変更を捨てて、ファイルからリロードしますか？');
    await question.dismiss();
  });
  await reload.click();
  await expect(input).toHaveValue('# 保存していない編集\n');
  const file = path.join(app.proj, '.hergy/roles/builder.md');
  writeFileSync(file, '# ファイルの新しい内容\n');
  page.once('dialog', question => question.accept());
  const context = page.waitForResponse(response => response.url().includes('/api/role/builder/context?') && response.request().method() === 'GET');
  await reload.click();
  await context;
  await expect(input).toHaveValue('# ファイルの新しい内容\n');
  await expect(column.getByRole('button', { name: '保存', exact: true })).toBeDisabled();
  await expect(column.locator('.prose-other-teams').filter({ hasText: 'migrate' })).toBeVisible();
});


test('all columns align both editor edges and save buttons', async ({ page, app }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(`${app.base}/teams?open=core&agree=core`); await ready(page);
 await page.getByRole('button', { name: 'すべて表示', exact: true }).click();
  const board = page.locator('.prose-board');
  await expect(board.locator('textarea')).toHaveCount(5);
  await expect.poll(async () => board.locator('.prose-column').evaluateAll(nodes => {
    const boxes = nodes.map(n => ({ input: n.querySelector('textarea')!.getBoundingClientRect(), save: n.querySelector('footer .primary')!.getBoundingClientRect() }));
    return ['top', 'bottom', 'save'].map(key => {
      const values = boxes.map(b => key === 'save' ? b.save.y : b.input[key]);
      return Math.max(...values) - Math.min(...values);
    });
  })).toEqual([0, 0, 0]);
});
