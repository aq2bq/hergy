import { test, expect } from './fixtures';
import { readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { ready } from './helpers';
test.use({ reads: 'teams' });

async function open(page, app) {
 await page.setViewportSize({ width: 1440, height: 900 });
 await page.goto(`${app.base}/teams?open=core&agree=core`); await ready(page);
 const row = page.locator('.tdef[data-name="core"]');
 const role = row.locator('.role-column').filter({ has: page.getByRole('heading', { name: /builder/ }) });
 await expect(role.getByRole('textbox', { name: '規則の全文' })).toBeEnabled();
 return { row, role };
}
async function menu(page, column, heading, destination, move = false) {
 await column.getByRole('button', { name: '見出しだけ', exact: true }).click();
 await column.getByRole('button', { name: heading + ' を写す・移す', exact: true }).click();
 const dialog = page.getByRole('dialog', { name: '見出しを写す・移す' });
 await dialog.getByLabel('宛先', { exact: true }).selectOption({ label: destination });
 if (move) await dialog.getByLabel('操作', { exact: true }).selectOption('move');
 await expect(dialog.getByRole('button', { name: '下書きに反映' })).toBeEnabled();
 return dialog;
}
test('copy a heading to another role and team, preserving bytes and undoing drafts', async ({ page, app }) => {
 const a = path.join(app.proj, '.hergy/roles/builder.md');
 const b = path.join(app.proj, '.hergy/roles/lead.md');
 const x = path.join(app.proj, '.hergy/teams/core/agreement.md');
 const y = path.join(app.proj, '.hergy/teams/migrate/agreement.md');
 const source = '# 作る\n本文  \n\n## 子\n残す\n', targetBefore = readFileSync(b, 'utf8');
 writeFileSync(a, source); writeFileSync(x, '# 合意\n\t約束  \n');
 const beforeY = readFileSync(y, 'utf8');
 const { row, role } = await open(page, app);
 let dialog = await menu(page, role, '作る', 'ロール lead（proj）');
 await dialog.getByLabel('子見出しを含める').check();
 await dialog.getByRole('button', { name: '下書きに反映' }).click();
 await dialog.getByRole('button', { name: '元に戻す', exact: true }).click();
 expect(readFileSync(b, 'utf8')).toBe(targetBefore);
 await dialog.getByRole('button', { name: '下書きに反映' }).click();
 await dialog.getByRole('button', { name: '写した先を保存', exact: true }).click();
 await expect(dialog.getByRole('status')).toHaveText('写した先を保存しました。');
 expect(readFileSync(b, 'utf8')).toBe(targetBefore + source);
 expect(readFileSync(a, 'utf8')).toBe(source);
 await dialog.getByRole('button', { name: '閉じる', exact: true }).click();
 dialog = await menu(page, row.locator('.agree-p'), '合意', 'チーム定義 migrate（proj）');
 await dialog.getByLabel('挿入位置').selectOption('0');
 await dialog.getByRole('button', { name: '下書きに反映' }).click();
 await dialog.getByRole('button', { name: '写した先を保存', exact: true }).click();
 await expect(dialog.getByRole('status')).toBeVisible();
 expect(readFileSync(y, 'utf8')).toBe('# 合意\n\t約束  \n' + beforeY);
});

test('a move saves the destination first and explains a conflicting source without losing text', async ({ page, app }) => {
 const a = path.join(app.proj, '.hergy/roles/builder.md');
 const b = path.join(app.proj, '.hergy/roles/lead.md');
 const source = '# 移動\n本文  \n'; writeFileSync(a, source);
 const beforeB = readFileSync(b, 'utf8');
 const { role } = await open(page, app);
 const dialog = await menu(page, role, '移動', 'ロール lead（proj）', true);
 await dialog.getByRole('button', { name: '下書きに反映' }).click();
 writeFileSync(a, source + '\n別の変更\n');
 await dialog.getByRole('button', { name: '宛先から順に保存' }).click();
 await expect(dialog.getByRole('status')).toContainText('両方のファイルに残っています');
 expect(readFileSync(b, 'utf8')).toBe(beforeB + source);
 expect(readFileSync(a, 'utf8')).toBe(source + '\n別の変更\n');
});

test('column width changes and equal reset keep the full text', async ({ page, app }) => {
 const { row, role } = await open(page, app);
 const before = readFileSync(path.join(app.proj, '.hergy/roles/builder.md'), 'utf8');
 await role.getByRole('button', { name: '狭く', exact: true }).click();
 await expect(role.locator('.prose-outline')).toBeVisible();
 await expect(role).toHaveCSS('width', '200px');
 await expect(role.locator('.prose-width .btn').first()).toHaveCSS('padding-left', '2px');
 await expect.poll(async () => { const reset = await role.getByRole('button', { name: '均等', exact: true }).boundingBox(); const box = await role.boundingBox(); return reset!.x + reset!.width <= box!.x + box!.width; }).toBe(true);
 await expect(role.locator('.prose-heading-link').first()).toHaveCSS('white-space', 'nowrap');
 await expect.poll(async () => (await role.locator('.prose-budget').boundingBox())!.height).toBe(24);
 await role.getByRole('button', { name: '広く', exact: true }).click();
 await expect(role).toHaveCSS('width', '580px');
 await role.getByRole('button', { name: '均等', exact: true }).click();
 await expect(role.getByRole('textbox', { name: '規則の全文' })).toHaveValue(before);
 await expect.poll(async () => {
  const widths = await row.locator('.prose-column').evaluateAll((nodes) => nodes.map((n) => n.getBoundingClientRect().width));
  return Math.max(...widths) - Math.min(...widths);
 }).toBeLessThan(1);
});

test('dragging stages a move and a failed destination never deletes the source', async ({ page, app }) => {
 const a = path.join(app.proj, '.hergy/roles/builder.md');
 const b = path.join(app.proj, '.hergy/roles/lead.md');
 const section = '# 移動\n本文  \n', remainder = '# 残す\n次の本文\n';
 writeFileSync(a, section + remainder); const beforeB = readFileSync(b, 'utf8');
 const { row, role } = await open(page, app);
 const lead = row.locator('.role-column').filter({ has: page.getByRole('heading', { name: /lead/ }) });
 await role.getByRole('button', { name: '見出しだけ', exact: true }).click();
 await role.locator('.prose-grip').first().dragTo(lead.getByRole('textbox', { name: '規則の全文' }));
 const dialog = page.getByRole('dialog', { name: '見出しを写す・移す' });
 await expect(dialog.getByLabel('操作', { exact: true })).toHaveValue('move');
 await dialog.getByRole('button', { name: '下書きに反映' }).click();
 await page.route('**/api/role/lead', route => route.request().method() === 'PUT' ? route.fulfill({ status: 503, json: { text: '保存先に接続できません' } }) : route.continue(), { times: 1 });
 await dialog.getByRole('button', { name: '宛先から順に保存' }).click();
 await expect(dialog.getByRole('alert')).toContainText('元のファイルは削除していません');
 expect(readFileSync(a, 'utf8')).toBe(section + remainder);
 expect(readFileSync(b, 'utf8')).toBe(beforeB);
 await dialog.getByRole('button', { name: '宛先から順に保存' }).click();
 await expect(dialog.getByRole('status')).toHaveText('宛先、元の順に保存しました。');
 expect(readFileSync(a, 'utf8')).toBe(remainder);
 expect(readFileSync(b, 'utf8')).toBe(beforeB + section);
});
