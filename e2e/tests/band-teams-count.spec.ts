import { test, expect } from './fixtures';
import { ready } from './helpers';
import { chmodSync } from 'node:fs';
import path from 'node:path';

// r-ea17, d-17ab: a team.yaml that cannot be read is one row of the list, with
// the reason, and the band counts it on every screen — the same look the role
// and profile rows have.
test.use({ reads: 'teams' });

test('the band counts the team.yaml it cannot read, on every screen', async ({ page, app }, info) => {
  const file = path.join(app.proj, '.hergy/teams/core/team.yaml');
  chmodSync(file, 0o000);

  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(`${app.base}/teams`); await ready(page);
  const rows = page.locator('.tdef');
  const count = await rows.count();
  const named = (name: string) => rows.filter({ has: page.locator('.c-name .nm', { hasText: new RegExp(`^${name}$`) }) });
  // The team.yaml that cannot be read is still a row (d-17ab).
  const broken = named('core');
  await expect(broken).toHaveCount(1);
  await expect(broken).toContainText('問題 1');
  const band = () => page.locator('.scr[data-screen="teams"] .n').textContent();
  await expect.poll(band).toBe(String(count));
  for (const screen of ['activity', 'roles', 'profiles']) {
    await page.goto(`${app.base}/${screen}`); await ready(page);
    await expect.poll(band).toBe(String(count));
  }
  await page.goto(`${app.base}/teams`); await ready(page);
  // The reason is readable in the row, where the other kinds show theirs.
  await broken.locator('.sum').click();
  await expect(broken.locator('[data-problem="team"]')).toContainText('読めません');
  for (const scheme of ['light', 'dark'] as const) {
    await page.emulateMedia({ colorScheme: scheme });
    await page.screenshot({ path: info.outputPath(`unreadable-team-${scheme}.png`) });
  }
  chmodSync(file, 0o644);
});
