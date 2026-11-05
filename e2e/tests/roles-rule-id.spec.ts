import { test, expect } from './fixtures';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { ready } from './helpers';
test.use({ reads: 'roles' });
test('prose saves exactly and an old rule link opens its document', async ({ page, app }) => {
  await page.goto(`${app.base}/roles?open=lead&rule=PAAF`); await ready(page);
  const panel = page.locator('.role.open .panel[data-l=project]');
  const text = '# 進行役\n\n空白を保つ。  \n\n## 次の仕事\nlead → builder は本文。\n';
  await panel.getByRole('textbox', { name: '規則の全文' }).fill(text);
  await panel.locator('[data-act=save]').click();
  await expect(panel.locator('.ok')).toBeVisible();
  expect(readFileSync(path.join(app.proj, '.hergy/roles/lead.md'), 'utf8')).toBe(text);
  await panel.getByRole('button', { name: '見出しだけ', exact: true }).click();
  await expect(panel.locator('.prose-outline li')).toHaveCount(2);
  await panel.getByRole('button', { name: '全文' }).click();
  await expect(panel.getByRole('textbox', { name: '規則の全文' })).toHaveValue(text);
});

test('a role URL reveals its editor without repeating its identity or covering its text', async ({ page, app }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(`${app.base}/roles?open=writer%7Cglobal`); await ready(page);
  const panel = page.locator('.role.open .panel[data-l="global"]');
  await expect(panel).toBeVisible();
  await expect(panel.locator('.prose-head h2, .prose-readers')).toHaveCount(0);
  await expect(panel.locator('.prose-head .path')).toHaveText('グローバル · roles/writer.md');
  const input = panel.getByRole('textbox', { name: 'writer の規則の全文', exact: true });
  await expect.poll(async () => (await input.boundingBox())!.y).toBeLessThan(500);
  await expect(panel.locator(':scope > .pf')).toHaveCSS('position', 'static');
  await input.fill('字'.repeat(1080));
  await expect(panel.locator('.meter')).not.toHaveClass(/near/);
  await input.fill('字'.repeat(1081));
  await expect(panel.locator('.meter')).toHaveClass(/near/);
  await input.fill('字'.repeat(1201));
  await expect(panel.locator('.meter')).toHaveClass(/over/);
});
