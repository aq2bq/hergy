import { test, expect } from './fixtures';
import { ready } from './helpers';

test.use({ reads: 'teams' });

for (const theme of ['light', 'dark'] as const) {
  for (const width of [2000, 390]) {
    test(`the ${theme} logo is legible and the band stays the same height at ${width}px`, async ({ page, app }) => {
      await page.emulateMedia({ colorScheme: theme });
      await page.setViewportSize({ width, height: 1100 });
      for (const screen of ['activity', 'teams', 'roles', 'profiles']) {
        await page.goto(`${app.base}/${screen}`); await ready(page);
        await page.evaluate(() => document.fonts.ready);
        const logo = page.getByRole('link', { name: 'hergy', exact: true });
        await expect(logo).toHaveAttribute('href', '/');
        await expect(logo.locator('[style]')).toHaveCount(0);
        // Before the SVG replaced the 13px word, the same fixture measured
        // 62px on desktop and 170px at 390px, on each of these four screens.
        await expect.poll(async () => (await page.locator('.bar').boundingBox())!.height).toBe(width === 390 ? 170 : 62);
        await expect.poll(async () => (await logo.locator('.logo-word').boundingBox())!.height).toBeGreaterThanOrEqual(13 * 1.4);
        await expect.poll(() => page.locator('.bar').evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
        await expect.poll(async () => (await logo.locator('svg').boundingBox())!.height).toBe(36);
        await expect.poll(() => logo.locator('path').evaluateAll(paths => paths.every(el =>
          getComputedStyle(el).strokeWidth === '1px' && getComputedStyle(el).vectorEffect === 'non-scaling-stroke',
        ))).toBe(true);
        await expect.poll(() => logo.locator('.logo-robot').evaluate(el => {
          const matrix = (el as SVGGraphicsElement).getCTM()!;
          return matrix.b === 0 && matrix.c === 0;
        })).toBe(true);
        await logo.focus(); await expect(logo).toBeFocused();
        await expect.poll(() => logo.evaluate(el => getComputedStyle(el).outlineStyle)).toBe('solid');
        await expect.poll(() => logo.locator('svg').evaluate(el => ({
          ink: getComputedStyle(el.querySelector('.logo-word')!).stroke,
          robot: getComputedStyle(el.querySelector('.logo-robot')!).stroke,
          antenna: getComputedStyle(el.querySelector('.logo-antenna')!).stroke,
        }))).toEqual({
          ink: theme === 'dark' ? 'rgb(227, 231, 238)' : 'rgb(20, 25, 34)',
          robot: theme === 'dark' ? 'rgb(227, 231, 238)' : 'rgb(20, 25, 34)',
          antenna: theme === 'dark' ? 'rgb(243, 141, 112)' : 'rgb(195, 72, 43)',
        });
      }
      await page.getByRole('link', { name: 'hergy', exact: true }).click();
      await expect(page).toHaveURL(/\/(activity|teams)$/);
    });
  }
}
