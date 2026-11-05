import { test, expect } from './fixtures';
import { ready } from './helpers';
import path from 'node:path';

// r-1e9a (d-79e3 の A): 元に戻す goes at once and leaves 取り消す in the band's
// one shared notice; 取り消す puts the draft and the band's dot back.
const typed = '打った文';

async function yamlUndo(page, url: string, out: string) {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(url); await ready(page);
  const area = page.locator('.ytext').first();
  const before = await area.inputValue();
  await area.fill(before + typed);
  await page.locator('[data-act="yaml-revert"]').first().click();
  await expect(page.locator('.undo')).toBeVisible();
  await expect(area).toHaveValue(before);
  for (const scheme of ['light', 'dark'] as const) {
    await page.emulateMedia({ colorScheme: scheme });
    await page.screenshot({ path: path.join(out, `undo-${scheme}.png`) });
  }
  await page.locator('.undo button').click();
  await expect(page.locator('.undo')).toHaveCount(0);
  await expect(area).toHaveValue(before + typed);
}

test.describe('teams', () => {
  test.use({ reads: 'teams' });
  test('YAML: 元に戻す → 取り消す', async ({ page, app }, info) => {
    await yamlUndo(page, `${app.base}/teams/yaml`, path.dirname(info.outputPath('x.png')));
  });
});

test.describe('roles', () => {
  test.use({ reads: 'roles' });
  test('YAML: 元に戻す → 取り消す', async ({ page, app }, info) => {
    await yamlUndo(page, `${app.base}/roles/yaml`, path.dirname(info.outputPath('x.png')));
  });
});

test.describe('profiles', () => {
  test.use({ reads: 'profiles' });
  test('YAML: 元に戻す → 取り消す', async ({ page, app }, info) => {
    await yamlUndo(page, `${app.base}/profiles/yaml`, path.dirname(info.outputPath('x.png')));
  });
});
