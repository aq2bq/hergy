import { test, expect } from './fixtures';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { ready } from './helpers';
test.use({ reads: 'roles' });
test('prose saves exactly and an old rule link opens its document', async ({ page, app }) => {
  await page.goto(`${app.base}/roles?open=lead&rule=PAAF`); await ready(page);
  const panel = page.locator('.role.open .panel[data-l=project]');
  const text = '# 進行役\n\n空白を保つ。  \n\n## 次の仕事\nlead → builder は本文。\n';
  await panel.getByRole('textbox', { name: '散文の全文' }).fill(text);
  await panel.locator('[data-act=save]').click();
  await expect(panel.locator('.ok')).toBeVisible();
  expect(readFileSync(path.join(app.proj, '.hergy/roles/lead.md'), 'utf8')).toBe(text);
  await panel.getByRole('button', { name: '見出しだけを表示する' }).click();
  await expect(panel.locator('.prose-outline li')).toHaveCount(2);
  await panel.getByRole('button', { name: '全文を編集する' }).click();
  await expect(panel.getByRole('textbox', { name: '散文の全文' })).toHaveValue(text);
});
