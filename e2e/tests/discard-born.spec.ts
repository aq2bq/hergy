import { test, expect } from './fixtures';
import { ready } from './helpers';

// r-d480: 取り消し compares the row with the state it was made in — a new row, a
// duplicate or a copy. Untouched, it goes at once; one change asks first.
test.use({ reads: 'teams' });

const openAsk = (page: import('@playwright/test').Page) => expect.poll(() => page.evaluate(() => (document.querySelector('.confirm') as HTMLDialogElement | null)?.open === true)).toBe(true);

for (const screen of ['teams', 'roles', 'profiles'] as const) {
  test(`DISCARD-${screen}: a row nobody touched goes, one change asks`, async ({ page, app }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    const ask = page.locator('.confirm');
    const gone = async (row: import('@playwright/test').Locator) => {
      await expect(row).toHaveCount(0);
      await expect(ask).toHaveCount(0);
    };
    const askedThenKeep = async (row: import('@playwright/test').Locator) => {
      await openAsk(page);
      await ask.getByRole('button', { name: 'やめる', exact: true }).click();
      await expect(ask).toHaveCount(0);
      await expect(row).toHaveCount(1);
    };
    const discard = async (row: import('@playwright/test').Locator) => { await row.locator('[data-act="discard"]').first().click(); };

    // One creation way: make the row, return it, and a way to change it.
    const ways: { name: string; make: () => Promise<import('@playwright/test').Locator>; change: (row: import('@playwright/test').Locator) => Promise<void> }[] = [];
    if (screen === 'teams') {
      ways.push({ name: 'new', make: async () => { await page.locator('.list .hq-gh [data-act="new"]').first().click(); return page.locator('.tdef[data-name^="~new"]'); }, change: async (row) => { await row.locator('.in-name').fill('touched-name'); } });
      ways.push({ name: 'dup', make: async () => {
        await page.goto(`${app.base}/teams?open=core`); await ready(page);
        await page.locator('.tdef[data-name="core"] [data-act="dup"]').click();
        return page.locator('.tdef[data-name^="~new"]');
      }, change: async (row) => { await row.locator('.in-name').fill('touched-name'); } });
      ways.push({ name: 'copy', make: async () => {
        await page.goto(`${app.base}/teams?open=core`); await ready(page);
        await page.locator('.tdef[data-name="core"] [data-act="copy"]').click();
        return page.locator('.tdef[data-name^="~new"]');
      }, change: async (row) => { await row.locator('.in-name').fill('touched-name'); } });
    }
    if (screen === 'roles') {
      ways.push({ name: 'new', make: async () => { await page.locator('.list .hq-gh [data-act="new"]').first().click(); return page.locator('.role').filter({ has: page.locator('[data-act="discard"]') }); }, change: async (row) => { await row.locator('[data-f="name"]').fill('touched-name'); } });
      ways.push({ name: 'dup', make: async () => {
        await page.goto(`${app.base}/roles?open=builder`); await ready(page);
        await page.locator('.role .panel [data-act="dup"]').first().click();
        return page.locator('.role').filter({ has: page.locator('[data-act="discard"]') });
      }, change: async (row) => { await row.locator('[data-f="name"]').fill('touched-name'); } });
      ways.push({ name: 'copy', make: async () => {
        await page.goto(`${app.base}/roles?open=builder`); await ready(page);
        await page.locator('.role .panel [data-act="copy"]').first().click();
        return page.locator('.role').filter({ has: page.locator('[data-act="discard"]') });
      }, change: async (row) => { await row.locator('[data-f="name"]').fill('touched-name'); } });
    }
    if (screen === 'profiles') {
      ways.push({ name: 'new', make: async () => { await page.locator('.list .hq-gh [data-act="new"]').first().click(); return page.locator('.row').filter({ has: page.locator('[data-act="discard"]') }); }, change: async (row) => { await row.locator('.in-name').fill('touched-name'); } });
      ways.push({ name: 'dup', make: async () => {
        await page.goto(`${app.base}/profiles?open=codex-mini`); await ready(page);
        await page.locator('.row[data-name="codex-mini"] [data-act="dup"]').first().click();
        return page.locator('.row').filter({ has: page.locator('[data-act="discard"]') });
      }, change: async (row) => { await row.locator('.in-name').fill('touched-name'); } });
      ways.push({ name: 'copy', make: async () => {
        await page.goto(`${app.base}/profiles?open=codex-mini`); await ready(page);
        await page.locator('.row[data-name="codex-mini"] [data-act="copy"]').first().click();
        return page.locator('.row').filter({ has: page.locator('[data-act="discard"]') });
      }, change: async (row) => { await row.locator('.in-name').fill('touched-name'); } });
    }

    for (const way of ways) {
      await page.goto(`${app.base}/${screen}`); await ready(page);
      // Untouched: gone at once, no dialog.
      let row = await way.make();
      await expect(row).toHaveCount(1);
      await discard(row);
      await gone(row);

      // One change: the same dialog asks, and やめる keeps it.
      row = await way.make();
      await expect(row).toHaveCount(1);
      await way.change(row);
      await discard(row);
      await askedThenKeep(row);
    }
  });
}
