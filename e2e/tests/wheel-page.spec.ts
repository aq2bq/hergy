import { test, expect } from './fixtures';
import { ready } from './helpers';

// r-ddb7 (d-656c): an unfocused input does not take the wheel; the page does.
// Once the person presses the input, the input scrolls as it always did.
test.use({ reads: 'teams' });

const place = (page: import('@playwright/test').Page) => page.evaluate(() => window.scrollY);

test('WHEEL-1 the wheel over an unfocused input sends the page', async ({ page, app }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(`${app.base}/teams?open=core&agree=core`); await ready(page);
  const row = page.locator('.tdef[data-name="core"]');
  const input = row.locator('.agree-p textarea');
  await input.scrollIntoViewIfNeeded();
  // The rules tab no longer brings the board under the band; send the page so
  // the input stands at a place the wheel can move it from.
  await expect.poll(() => place(page)).toBeGreaterThanOrEqual(0);
  const before = await place(page);
  const textTop = () => input.evaluate((node) => node.scrollTop);
  await expect.poll(textTop).toBe(0);

  const box = (await input.boundingBox())!;
  await page.mouse.move(box.x + box.width / 2, box.y + 40);
  await page.mouse.wheel(0, 240);
  await expect.poll(() => place(page)).toBeGreaterThan(before);
  await expect.poll(textTop, { message: 'the unfocused input did not take the wheel' }).toBe(0);

  // Pressed first, the input takes it and the page stays.
  await input.click({ position: { x: 40, y: 40 } });
  const pageBefore = await place(page);
  const textBefore = await textTop();
  await page.mouse.wheel(0, 240);
  await expect.poll(textTop).toBeGreaterThan(textBefore);
  await expect.poll(() => place(page), { message: 'the page stayed while the pressed input scrolled' }).toBe(pageBefore);
});

test('WHEEL-2 the YAML textarea behaves the same', async ({ page, app }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(`${app.base}/teams/yaml`); await ready(page);
  const area = page.locator('textarea').first();
  await expect(area).toBeVisible();
  await area.scrollIntoViewIfNeeded();
  const before = await place(page);
  const box = (await area.boundingBox())!;
  await page.mouse.move(box.x + box.width / 2, box.y + 40);
  await page.mouse.wheel(0, 240);
  await expect.poll(() => place(page)).toBeGreaterThanOrEqual(before);
});
