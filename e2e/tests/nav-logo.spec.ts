import { test, expect } from './fixtures';
import { ready } from './helpers';
import type { Page } from '@playwright/test';

// Read the same path rectangles as the acceptance criteria, including the
// rotated lettering. Parent SVG bounds do not describe the painted logo.
const logoLayout = (page: Page) => page.locator('.bar').evaluate(bar => {
  const paths = [...bar.querySelectorAll('.brand-logo path')].map(el => el.getBoundingClientRect());
  const line = { left: Math.min(...paths.map(p => p.left)), top: Math.min(...paths.map(p => p.top)),
    right: Math.max(...paths.map(p => p.right)), bottom: Math.max(...paths.map(p => p.bottom)) };
  const band = bar.getBoundingClientRect(), mark = bar.querySelector('.mark')!.getBoundingClientRect();
  const robot = bar.querySelector('.logo-robot')!.getBoundingClientRect();
  const faceCenter = (robot.top + robot.bottom) / 2;
  const others = [...bar.querySelectorAll('.crumb > :not(.mark), .screens, .hq-search, .apl-bandbtn, .theme-trigger')].filter(el => el.getClientRects().length > 0).map(el => el.getBoundingClientRect());
  return {
    height: line.bottom - line.top, ratio: (line.right - line.left) / (line.bottom - line.top), bandHeight: band.height,
    inset: Math.min(line.left - band.left, line.top - band.top, band.right - line.right, band.bottom - line.bottom),
    gap: bar.querySelector('.sep')!.getBoundingClientRect().left - line.right,
    contained: mark.left <= line.left && mark.top <= line.top && mark.right >= line.right && mark.bottom >= line.bottom,
    centerDelta: Math.max(0, ...others.filter(other => Math.min(line.bottom, other.bottom) > Math.max(line.top, other.top))
      .map(other => Math.abs((other.top + other.bottom) / 2 - faceCenter))),
    partsContained: [mark, ...others].every(part => part.left >= band.left && part.right <= band.right),
    overlaps: others.some(other => Math.min(line.right, other.right) > Math.max(line.left, other.left)
      && Math.min(line.bottom, other.bottom) > Math.max(line.top, other.top)),
  };
});

// Wide bands use one control row; narrow bands keep the logo at its approved size.
const bandMaximum = { 1440: 110, 1000: 200, 760: 250, 375: 250 };
for (const reads of ['teams', 'hq'] as const) {
  test.describe(`${reads} header`, () => {
    test.use({ reads });

    for (const theme of ['light', 'dark'] as const) {
      for (const width of [1440, 1000, 760, 375] as const) {
        test(`the ${theme} enlarged logo fits its band and focus box at ${width}px`, async ({ page, app }) => {
          await page.emulateMedia({ colorScheme: theme });
          await page.setViewportSize({ width, height: 1100 });
          for (const screen of ['', 'activity', 'teams', 'roles', 'profiles']) {
            await page.goto(`${app.base}/${screen}`); await ready(page);
            await page.evaluate(() => document.fonts.ready);
            const logo = page.getByRole('link', { name: 'hergy', exact: true });
            await expect(logo).toHaveAttribute('href', '/');
            await expect(logo.locator('[style]')).toHaveCount(0);
            await page.keyboard.press('Tab'); await expect(logo).toBeFocused();
            // Below 420px the 16px gutters restore the original project action
            // wrapping; the theme button still reserves its 46px.
            await expect.poll(async () => {
              const layout = await logoLayout(page);
              return {
                size: layout.height >= 72 && layout.height <= 83,
                ratio: layout.ratio >= (48.89 / 34.43) * .95 && layout.ratio <= (48.89 / 34.43) * 1.05,
                band: layout.bandHeight >= 83 && layout.bandHeight <= bandMaximum[width],
                inset: layout.inset >= 2, gap: layout.gap >= 8 && layout.gap <= 16,
                focus: layout.contained, clear: !layout.overlaps, parts: layout.partsContained, aligned: layout.centerDelta <= 2,
              };
            }).toEqual({ size: true, ratio: true, band: true, inset: true, gap: true, focus: true, clear: true, parts: true, aligned: true });
            await expect.poll(() => page.locator('.bar').evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
            await expect.poll(() => logo.locator('path').evaluateAll(paths => paths.every(el =>
              getComputedStyle(el).strokeWidth === '1px' && getComputedStyle(el).vectorEffect === 'non-scaling-stroke',
            ))).toBe(true);
            await expect.poll(() => logo.locator('.logo-robot').evaluate(el => {
              const matrix = (el as SVGGraphicsElement).getCTM()!;
              return matrix.b === 0 && matrix.c === 0;
            })).toBe(true);
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
  });
}
