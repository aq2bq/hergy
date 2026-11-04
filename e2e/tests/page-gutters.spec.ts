import { test, expect } from './fixtures';
import { ready } from './helpers';
import { writeFileSync } from 'node:fs';
import path from 'node:path';

test.use({ reads: 'teams' });
for (const width of [2000, 1440]) for (const theme of ['light', 'dark'] as const) {
 test(`shared band and page scroll gutters at ${width}px in ${theme}`, async ({ page, app }, info) => {
  writeFileSync(path.join(app.proj, '.hergy/roles/lead.md'), '# Lead\n' + 'Long rules for measuring the page scroll gutters.\n'.repeat(120));
  await page.setViewportSize({ width, height: 900 });await page.emulateMedia({ colorScheme: theme });
  const measureBand = () => page.locator('.bar').evaluate(bar => [...bar.querySelectorAll('.mark, .brand-logo, .crumb > .sep, .hq-pbtn, .hq-pbtn b, .hq-caret, .screens, .scr, .scr .n, .scr .wait, .hq-search, .hq-search > *, .theme-trigger')].map(n => {
   const b = n.getBoundingClientRect();return { x: b.x, width: b.width };
  }));
  let band: Awaited<ReturnType<typeof measureBand>> | undefined;
  const measurements = [];
  for (const screen of ['activity', 'teams', 'profiles', 'roles']) {
   const suffix = screen === 'teams' ? '?open=core&agree=core' : screen === 'roles' ? '?open=lead|project' : '';
   await page.goto(`${app.base}/${screen}${suffix}`);await ready(page);await page.evaluate(() => document.fonts.ready);
   if (band) await expect.poll(async () => {
    const now = await measureBand();
    return now.length === band!.length && now.every((b, i) => Math.abs(b.x - band![i].x) < 1 && Math.abs(b.width - band![i].width) < 1);
   }).toBe(true);
   else band = await measureBand();
   await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
   await expect.poll(() => page.locator('.wrap').evaluate(n => {
    const b = n.getBoundingClientRect(), s = getComputedStyle(n);
    return b.left >= 0 && b.right <= document.documentElement.clientWidth && parseFloat(s.paddingLeft) >= 40 && parseFloat(s.paddingRight) >= 40;
   })).toBe(true);
   if (width === 2000) await expect.poll(() => page.locator('.wrap').evaluate(n => n.getBoundingClientRect().width)).toBeGreaterThan(1680);
   measurements.push({ screen, band: await measureBand() });
   await page.screenshot({ path: info.outputPath(`${screen}.png`) });
   if (screen !== 'teams' && screen !== 'roles') continue;
   const inputs = page.locator(screen === 'teams' ? '.tdef[data-name="core"] .prose-column textarea' : '.role.open textarea');
   if (screen === 'teams') await expect(inputs).toHaveCount(5);
   await expect.poll(() => inputs.evaluateAll(ns => {
    const b = ns.map(n => n.getBoundingClientRect());
    return Math.min(...b.map(r => r.left)) >= 40 && document.documentElement.clientWidth - Math.max(...b.map(r => r.right)) >= 40;
   })).toBe(true);
   if (screen === 'teams') {
    const last = page.locator('.tdef[data-name="core"] .prose-column').last();
    await last.getByRole('button', { name: '広く', exact: true }).click();
    await last.scrollIntoViewIfNeeded();
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
   }
   for (const side of ['left', 'right']) {
    await page.evaluate(() => window.scrollTo(0, 0));await expect.poll(() => page.evaluate(() => scrollY)).toBe(0);
    const x = side === 'left' ? 20 : await page.evaluate(() => document.documentElement.clientWidth - 20);
    await expect.poll(() => page.evaluate(x => !document.elementFromPoint(x, 450)?.closest('textarea'), x)).toBe(true);
    const before = await page.locator('textarea').evaluateAll(ns => ns.map(n => n.scrollTop));
    await page.mouse.move(x, 450);await page.mouse.wheel(0, 240);
    await expect.poll(() => page.evaluate(() => scrollY)).toBeGreaterThan(0);
    await expect.poll(() => page.locator('textarea').evaluateAll(ns => ns.map(n => n.scrollTop))).toEqual(before);
   }
  }
  writeFileSync(info.outputPath('band.json'), JSON.stringify(measurements, null, 2));
 });
}
