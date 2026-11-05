import { test, expect } from './fixtures';
import { ready } from './helpers';
import { existsSync, realpathSync } from 'node:fs';
import path from 'node:path';

// r-d480: what cannot be taken back is asked for in one shared dialog, away from
// the button that asked. The same place pressed again and Enter pressed twice
// never run it; Esc and やめる close it with nothing done.
test.use({ reads: 'teams' });

test('DEL-PLACE-1 the delete asks in a dialog, away from the button', async ({ page, app }, info) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  const file = path.join(app.proj, '.hergy/teams/core/team.yaml');
  await page.goto(`${app.base}/teams?open=core`); await ready(page);
  const row = page.locator('.tdef[data-name="core"]');
  const del = row.locator('[data-act="del"]');
  const ask = page.locator('.confirm');
  await expect(del).toBeVisible();

  // The focus starts on やめる, so two Enters keep the first one and close it.
  await del.click();
  await expect(ask).toBeVisible();
  await expect(page.getByRole('button', { name: 'やめる', exact: true })).toBeFocused();
  await page.keyboard.press('Enter');
  await page.keyboard.press('Enter');
  await expect(ask).toHaveCount(0);
  expect(existsSync(file), 'the file is still there after two Enters').toBe(true);

  // The second press at the same place does not run it either.
  const box = (await del.boundingBox())!;
  await del.click();
  await expect(ask).toBeVisible();
  await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
  await expect(ask).toBeVisible();
  expect(existsSync(file), 'the file is still there after the second press').toBe(true);
  await page.keyboard.press('Escape');
  await expect(ask).toHaveCount(0);

  // Esc closes it with nothing done.
  await del.click();
  await expect(ask).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(ask).toHaveCount(0);
  expect(existsSync(file), 'the file is still there after Esc').toBe(true);

  // What it says: the files it removes, and that it cannot be taken back.
  for (const scheme of ['light', 'dark'] as const) {
    await page.emulateMedia({ colorScheme: scheme });
    await del.click();
    await expect.poll(() => page.evaluate(() => (document.querySelector('.confirm') as HTMLDialogElement | null)?.open === true)).toBe(true);
    await page.screenshot({ path: info.outputPath(`confirm-${scheme}.png`) });
    await page.keyboard.press('Escape');
    await expect(ask).toHaveCount(0);
  }
  await del.click();
  await expect(ask.locator('h2')).toHaveText('core を削除しますか');
  await expect(ask).toContainText('team.yaml と agreement.md');
  await expect(ask).toContainText('取り消せません');
  await ask.getByRole('button', { name: 'やめる', exact: true }).click();
  await expect(ask).toHaveCount(0);

  // Confirming runs it.
  await del.click();
  await ask.locator('.btn.danger.solid').click();
  await expect(row).toHaveCount(0);
  expect(existsSync(file), 'the file went with the confirmation').toBe(false);
});

test('DEL-PLACE-2 the headquarters stop asks in the same dialog', async ({ page, app }, info) => {
  await page.route('**/api/hq/running', (route) => route.fulfill({ json: {
    herdr: { ok: true, text: '' },
    teams: [{
      team: 'core', definition: 'core', place: 'project', project: realpathSync(app.proj), project_name: 'fixture',
      workspace: { id: 'w1', label: 'main' }, tab: { id: 't1', label: 'core' },
      members: [{ name: 'lead', herdr: 'core-lead', live: true, state: 'idle' }],
      workers: [], waiting: [], starting: false,
    }],
  } }));
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(`${app.base}/teams`); await ready(page);
  await page.locator('.tdef .hq-runc [data-hq="stop"]').click();
  const ask = page.locator('.confirm');
  await expect(ask.locator('h2')).toHaveText('チーム core を終了しますか');
  await expect(page.getByRole('button', { name: 'やめる', exact: true })).toBeFocused();
  await page.keyboard.press('Escape');
  await expect(ask).toHaveCount(0);
  await page.locator('.tdef .hq-runc [data-hq="stop"]').click();
  await expect(ask).toBeVisible();
  for (const scheme of ['light', 'dark'] as const) {
    await page.emulateMedia({ colorScheme: scheme });
    await page.screenshot({ path: info.outputPath(`stop-confirm-${scheme}.png`) });
  }
});
