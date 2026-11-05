import { test, expect } from './fixtures';
import { ready } from './helpers';
import { chmodSync, mkdirSync, writeFileSync } from 'node:fs';
import path from 'node:path';

// r-929e: the band's ロール number is the number of rows the roles screen draws.
// The rows that made the two counts differ are here: a legacy .yaml, a document
// that cannot be read, a name only a member names, and one name in both layers.
test.use({ reads: 'teams' });

test('the band counts every kind of role row, on every screen', async ({ page, app }, info) => {
  const project = path.join(app.proj, '.hergy/roles');
  const global = path.join(app.cfg, 'hergy/roles');
  mkdirSync(project, { recursive: true });
  mkdirSync(global, { recursive: true });
  // A legacy document: the list draws it, and it has no .md.
  writeFileSync(path.join(project, 'old.yaml'), 'version: 1\nprompt: 旧形式のロール\n');
  // One name in both layers: two rows, one on each side.
  writeFileSync(path.join(project, 'shared.md'), '# shared\n');
  writeFileSync(path.join(global, 'shared.md'), '# shared\n');
  // A document that cannot be read: the row stands with the reason (d-17ab).
  const broken = path.join(project, 'broken.md');
  writeFileSync(broken, '# broken\n');
  chmodSync(broken, 0o000);
  // A document on the global side that cannot be read is left out as well.
  const brokenGlobal = path.join(global, 'broken-global.md');
  writeFileSync(brokenGlobal, '# broken-global\n');
  chmodSync(brokenGlobal, 0o000);
  // A name only a member names, with no document anywhere.
  mkdirSync(path.join(app.proj, '.hergy/teams/ghost-team'), { recursive: true });
  writeFileSync(path.join(app.proj, '.hergy/teams/ghost-team/team.yaml'),
    'version: 1\ndescription: 文書の無いロールを名指す\nmembers:\n  - name: someone\n    role: ghost\n    profile: default\n');

  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(`${app.base}/roles`); await ready(page);
  const rows = page.locator('#rows .role');
  const count = await rows.count();
  // exact, so "broken" does not also find "broken-global".
  const named = (name: string) => rows.filter({ has: page.locator('.c-name .nm', { hasText: new RegExp(`^${name}$`) }) });
  await expect(named('old')).toHaveCount(1);
  // A document that cannot be read is a row and is counted (A, d-17ab).
  await expect(named('broken')).toHaveCount(1);
  await expect(named('broken-global')).toHaveCount(1);
  await expect(named('ghost')).toHaveCount(1);
  await expect(named('shared')).toHaveCount(2);
  const band = () => page.locator('.scr[data-screen="roles"] .n').textContent();
  await expect.poll(band).toBe(String(count));
  for (const screen of ['teams', 'profiles', 'activity']) {
    await page.goto(`${app.base}/${screen}`); await ready(page);
    await expect.poll(band).toBe(String(count));
  }
  await page.goto(`${app.base}/roles`); await ready(page);
  await expect.poll(band).toBe(String(count));
  // The reason is readable in the row (ac-ad92).
  await named('broken').locator('.sum').click();
  await expect(named('broken').locator('[data-problem]')).toContainText('読めません');
  for (const scheme of ['light', 'dark'] as const) {
    await page.emulateMedia({ colorScheme: scheme });
    await page.screenshot({ path: info.outputPath(`unreadable-${scheme}.png`) });
  }
  chmodSync(broken, 0o644);
  chmodSync(brokenGlobal, 0o644);
});
