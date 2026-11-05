import { test, expect } from './fixtures';
import { ready } from './helpers';

// r-929e: the band's ロール number is the number of rows the roles screen draws,
// and every screen shows that one number (it is written by the headquarters
// reading alone).
test.use({ reads: 'teams' });

test('the band counts one role for every row, on every screen', async ({ page, app }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(`${app.base}/roles`); await ready(page);
  const rows = await page.locator('#rows .role').count();
  expect(rows).toBeGreaterThan(0);
  const band = () => page.locator('.scr[data-screen="roles"] .n').textContent();
  await expect.poll(band).toBe(String(rows));
  for (const screen of ['teams', 'profiles', 'activity']) {
    await page.goto(`${app.base}/${screen}`); await ready(page);
    await expect.poll(band).toBe(String(rows));
  }
  // Reading the roles screen again gives the same number.
  await page.goto(`${app.base}/roles`); await ready(page);
  await expect.poll(band).toBe(String(rows));
});
