import { test, expect } from './fixtures';
import { ready } from './helpers';
import { chmodSync } from 'node:fs';
import path from 'node:path';

// r-ea17, d-17ab: a profiles.yaml that cannot be read is one row of the list,
// named after the file, and the band counts it on every screen.
test.use({ reads: 'profiles' });

test('the band counts the profiles file it cannot read, on every screen', async ({ page, app }, info) => {
  const file = path.join(app.proj, '.hergy/profiles.yaml');
  chmodSync(file, 0o000);

  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(`${app.base}/profiles`); await ready(page);
  const rows = page.locator('#rows .row');
  const count = await rows.count();
  const named = (name: string) => rows.filter({ has: page.locator('.c-name .nm', { hasText: new RegExp(`^${name}$`) }) });
  await expect(named('profiles.yaml')).toHaveCount(1);
  const band = () => page.locator('.scr[data-screen="profiles"] .n').textContent();
  await expect.poll(band).toBe(String(count));
  for (const screen of ['teams', 'roles', 'activity']) {
    await page.goto(`${app.base}/${screen}`); await ready(page);
    await expect.poll(band).toBe(String(count));
  }
  await page.goto(`${app.base}/profiles`); await ready(page);
  // The reason is readable in the row.
  await named('profiles.yaml').locator('.sum').click();
  await expect(named('profiles.yaml').locator('[data-problem]')).toContainText('読めません');
  for (const scheme of ['light', 'dark'] as const) {
    await page.emulateMedia({ colorScheme: scheme });
    await page.screenshot({ path: info.outputPath(`unreadable-profiles-${scheme}.png`) });
  }
  chmodSync(file, 0o644);
});
