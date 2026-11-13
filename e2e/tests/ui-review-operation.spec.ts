import { test, expect } from './fixtures';
import { ready } from './helpers';
import { rmSync } from 'node:fs';
import path from 'node:path';

test.use({ reads: 'activity' });

test('moving to a pane announces progress while waiting and says it was done', async ({ page, app }) => {
  let release: () => void = () => {};
  const gate = new Promise<void>((resolve) => { release = resolve; });
  await page.route('**/api/activity/focus', async (route) => {
    await gate;
    await route.fulfill({ json: { ok: true } });
  });
  await page.goto(app.base + '/activity'); await ready(page);
  const button = page.locator('.hq-at[data-pick="migrate"] .hq-btn.primary');
  await expect(button).toHaveText('herdr でペインへ移動');
  await button.click();
  await expect(button).toHaveText('移動中…');
  await expect(button).toBeDisabled();
  await expect(button).toHaveAttribute('aria-busy', 'true');
  release();
  await expect(button).toHaveText('herdr で移動しました');
  await expect(button).toBeEnabled();
});

test('missing herdr executable gives an actionable error without exec details', async ({ page, app }) => {
  await page.goto(app.base + '/activity'); await ready(page);
  rmSync(path.join(app.work, 'bin', 'herdr'));
  await page.locator('.hq-at[data-pick="migrate"] .hq-btn.primary').click();
  await expect(page.getByRole('alert')).toContainText('herdrのインストール');
  await expect(page.getByRole('alert')).not.toContainText('fork/exec');
});

test('390px activity controls fit the viewport', async ({ page, app }, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(app.base + '/activity'); await ready(page);
  await expect.poll(() => page.locator('button, input, select, a').evaluateAll((elements) => elements.flatMap((el) => {
    const rect = el.getBoundingClientRect();
    return rect.width > 0 && (rect.right > innerWidth + 1 || rect.left < -1) ? [(el.textContent || el.getAttribute('aria-label') || el.tagName).trim()] : [];
  }))).toEqual([]);
  await page.screenshot({ path: testInfo.outputPath('activity-390.png'), fullPage: true });
});

for (const theme of ['light', 'dark'] as const) {
  test(`${theme} column headings and paths have readable contrast`, async ({ page, app }, testInfo) => {
    await page.emulateMedia({ colorScheme: theme });
    for (const screen of ['teams', 'roles', 'profiles']) {
      await page.goto(`${app.base}/${screen}`); await ready(page);
      await expect.poll(() => page.locator('.head, .hq-gh .hq-gp').evaluateAll((elements) => {
        const luminance = (rgb: number[]) => rgb.map((v) => { const c = v / 255; return c <= .04045 ? c / 12.92 : ((c + .055) / 1.055) ** 2.4; }).reduce((sum, c, i) => sum + c * [.2126, .7152, .0722][i], 0);
        const root = getComputedStyle(document.documentElement);
        const hex = root.getPropertyValue('--bg').trim().replace('#', '');
        const background = luminance([0, 2, 4].map((i) => parseInt(hex.slice(i, i + 2), 16)));
        return Math.min(...elements.map((el) => {
          const color = luminance((getComputedStyle(el).color.match(/[\d.]+/g) || []).slice(0, 3).map(Number));
          return (Math.max(color, background) + .05) / (Math.min(color, background) + .05);
        }));
      })).toBeGreaterThanOrEqual(4.5);
    }
    await page.screenshot({ path: testInfo.outputPath(`contrast-${theme}.png`), fullPage: true });
  });
}
