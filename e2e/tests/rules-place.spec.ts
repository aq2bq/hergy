import { test, expect } from './fixtures';
import { ready } from './helpers';

// r-e9a2: the rules tab changes what stands in the board, never where the page
// is. The tab body keeps a floor of the tallest it has been, so a shorter tab
// cannot pull the page up (ac-4ba8), and picking documents does not send the
// page either (ac-c66a). Opening the tab shows every document of the definition
// from the start (ac-d93b).
test.use({ reads: 'teams' });

const place = (page: import('@playwright/test').Page) => page.evaluate(() => window.scrollY);

test('RULES-PLACE-1 switching tabs keeps the page place', async ({ page, app }, info) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(`${app.base}/teams?open=core&agree=core`); await ready(page);
  const row = page.locator('.tdef[data-name="core"]');
  const shape = row.getByRole('tab', { name: /^トポロジー/ });
  const agree = row.getByRole('tab', { name: /^規則/ });
  await shape.click();
  await expect(shape).toHaveAttribute('aria-selected', 'true');

  // A person presses the tabs where they stand: just under the sticky band and
  // summary, with the page sent down to bring them there.
  const spot = await page.evaluate(() => {
    const bar = document.querySelector<HTMLElement>('.bar');
    const sum = document.querySelector<HTMLElement>('.tdef[data-name="core"] > .sum');
    const tabs = document.querySelector<HTMLElement>('.tdef[data-name="core"] .dtabs')!;
    const top = (bar?.offsetHeight || 0) + (sum?.offsetHeight || 0) + 16;
    return Math.max(0, tabs.getBoundingClientRect().top + scrollY - top);
  });
  expect(spot).toBeGreaterThan(0);

  for (const [from, to] of [[shape, agree], [agree, shape]] as const) {
    await from.click();
    await page.evaluate((top) => window.scrollTo(0, top), spot);
    await expect.poll(() => place(page)).toBeGreaterThan(spot - 2);
    const before = await place(page);
    const height = await page.evaluate(() => document.documentElement.scrollHeight);
    await expect(to).toBeInViewport();
    await to.click();
    await expect(to).toHaveAttribute('aria-selected', 'true');
    await expect.poll(() => place(page)).toBe(before);
    // The page never has a shorter document to clamp the place into.
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollHeight)).toBeGreaterThanOrEqual(height);
  }

  for (const scheme of ['light', 'dark'] as const) {
    await page.emulateMedia({ colorScheme: scheme });
    await agree.click();
    await page.evaluate(() => window.scrollTo(0, 0));
    await expect.poll(() => place(page)).toBe(0);
    await page.screenshot({ path: info.outputPath(`rules-all-${scheme}.png`) });
    await shape.click();
    await page.screenshot({ path: info.outputPath(`topology-${scheme}.png`) });
    await agree.click();
  }
});

test('RULES-PLACE-2 picking documents keeps the page place', async ({ page, app }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(`${app.base}/teams?open=core&agree=core`); await ready(page);
  const row = page.locator('.tdef[data-name="core"]');
  await page.evaluate(() => window.scrollTo(0, 240));
  await expect.poll(() => place(page)).toBe(240);
  const before = await place(page);
  expect(before).toBeGreaterThan(0);

  const documents = row.locator('.prose-documents');
  const boxes = documents.getByRole('checkbox');
  await boxes.nth(1).uncheck();
  await expect.poll(() => place(page)).toBe(before);
  await boxes.nth(1).check();
  await expect.poll(() => place(page)).toBe(before);

  // A document read alone, then all of them back.
  await documents.getByRole('button', { name: /^ロール builder / }).click();
  await expect(row.locator('.prose-column:visible')).toHaveCount(1);
  await expect.poll(() => place(page)).toBe(before);
  const count = await documents.locator('.prose-choice').count();
  await documents.getByRole('button', { name: 'すべて表示', exact: true }).click();
  await expect(row.locator('.prose-column:visible')).toHaveCount(count);
  await expect.poll(() => place(page)).toBe(before);
});

test('RULES-PLACE-3 the rules tab opens with every document checked', async ({ page, app }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(`${app.base}/teams?open=core&agree=core`); await ready(page);
  const row = page.locator('.tdef[data-name="core"]');
  const boxes = row.locator('.prose-documents').getByRole('checkbox');
  const count = await boxes.count();
  expect(count).toBeGreaterThan(1);
  for (let i = 0; i < count; i += 1) await expect(boxes.nth(i)).toBeChecked();
  await expect(row.locator('.prose-column:visible')).toHaveCount(count);
});
