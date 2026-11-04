import { test, expect } from './fixtures';
import { chromium, type Page } from '@playwright/test';
import { writeFileSync, mkdtempSync, rmSync } from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { ready } from './helpers';

test.use({ reads: 'teams' });
const screens = ['activity', 'teams', 'roles', 'profiles'];
const names = { light: 'ライト', dark: 'ダーク', system: 'OS に従う' };
async function choose(page: Page, mode: keyof typeof names) {
  await page.getByRole('button', { name: /^明暗を選ぶ/ }).click();
  await page.getByRole('menuitemradio', { name: names[mode], exact: true }).click();
}
async function color(page: Page, mode: 'light' | 'dark') {
  await expect(page.locator('html')).toHaveCSS('color-scheme', mode);
  await expect(page.locator('body')).toHaveCSS('background-color', mode === 'dark' ? 'rgb(15, 18, 24)' : 'rgb(236, 239, 243)');
}
for (const screen of screens) {
  test(`theme acceptance on ${screen}`, async ({ page, app }, info) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.emulateMedia({ colorScheme: 'light' });
    await page.goto(`${app.base}/${screen}`); await ready(page);
    await choose(page, 'dark'); await color(page, 'dark');
    await expect.poll(() => page.evaluate(() => localStorage.getItem('hergy.theme'))).toBe('dark');
    await page.reload(); await ready(page); await color(page, 'dark');
    for (const next of screens.filter(name => name !== screen)) {
      await page.locator(`.scr[data-screen="${next}"]`).click(); await ready(page); await color(page, 'dark');
    }
    await page.locator(`.scr[data-screen="${screen}"]`).click(); await ready(page);
    await page.getByRole('button', { name: /^明暗を選ぶ/ }).click();
    await expect(page.getByRole('menuitemradio', { name: 'ダーク', exact: true })).toHaveAttribute('aria-checked', 'true');
    await page.screenshot({ path: info.outputPath(`${screen}-dark-menu.png`) });
    await page.keyboard.press('Escape');
    await choose(page, 'system'); await color(page, 'light');
    await expect(page.locator('html')).not.toHaveAttribute('data-theme');
    await expect.poll(() => page.evaluate(() => localStorage.getItem('hergy.theme'))).toBe('system');
    await page.emulateMedia({ colorScheme: 'dark' }); await color(page, 'dark');
    await choose(page, 'light'); await color(page, 'light');
    await page.reload(); await ready(page); await color(page, 'light');
    await page.getByRole('button', { name: /^明暗を選ぶ/ }).click();
    await page.screenshot({ path: info.outputPath(`${screen}-light-menu.png`) });
    writeFileSync(info.outputPath('acceptance.json'), JSON.stringify({ screen, dark: true, reloadAndThreeOtherScreens: true, followLightOS: true, followDarkOS: true, explicitLightOnDarkOS: true }));
  });
}

test('theme changes and deletion propagate to another tab without a reload', async ({ page, context, app }) => {
  await page.emulateMedia({ colorScheme: 'light' });
  await page.goto(`${app.base}/teams`); await ready(page);
  const other = await context.newPage(); await other.emulateMedia({ colorScheme: 'light' });
  await other.goto(`${app.base}/roles`); await ready(other);
  await choose(page, 'dark'); await color(other, 'dark');
  await choose(other, 'light'); await color(page, 'light');
  await choose(page, 'dark'); await color(other, 'dark');
  await page.evaluate(() => localStorage.removeItem('hergy.theme')); await color(other, 'light');
  await choose(page, 'dark'); await color(other, 'dark');
  await page.evaluate(() => localStorage.clear()); await color(other, 'light');
  await other.close();
});

for (const operation of ['getItem', 'setItem'] as const) {
  test(`theme follows OS and recovers after ${operation} fails`, async ({ page, app }) => {
    await page.emulateMedia({ colorScheme: 'light' });
    await page.addInitScript(operation => {
      localStorage.setItem('hergy.theme', 'dark');
      const original = Storage.prototype[operation];
      const flags = window as typeof window & { blockTheme: boolean };
      flags.blockTheme = true;
      Storage.prototype[operation] = function(key: string, ...args: string[]) {
        if (key === 'hergy.theme' && flags.blockTheme) throw new Error('storage blocked');
        return original.apply(this, [key, ...args] as [string, string]);
      };
    }, operation);
    await page.goto(`${app.base}/teams`); await ready(page);
    if (operation === 'setItem') await choose(page, 'light');
    await color(page, 'light');
    await expect(page.getByRole('button', { name: '明暗を選ぶ（現在: OS に従う）', exact: true })).toBeEnabled();
    await expect(page.getByRole('status').filter({ hasText: '選択を保存できないため' })).toBeVisible();
    await page.evaluate(() => { (window as typeof window & { blockTheme: boolean }).blockTheme = false; });
    await choose(page, 'dark'); await color(page, 'dark');
    await expect(page.locator('.theme-notice')).toBeHidden();
  });
}

for (const mode of ['light', 'dark'] as const) {
  for (const screen of screens) {
    test(`first painted ${screen} follows saved ${mode} even with delayed initialization`, async ({ page, app }, info) => {
      await page.emulateMedia({ colorScheme: mode === 'dark' ? 'light' : 'dark' });
      await page.addInitScript(mode => {
        localStorage.setItem('hergy.theme', mode);
        const frames: { theme: string | null; background: string; time: number }[] = [];
        Object.assign(window, { themeFrames: frames });
        const sample = () => {
          if (document.body) {
            const background = getComputedStyle(document.body).backgroundColor;
            if (background !== 'rgba(0, 0, 0, 0)') frames.push({ theme: document.documentElement.getAttribute('data-theme'), background, time: performance.now() });
          }
          if (frames.length < 10) requestAnimationFrame(sample);
        };
        requestAnimationFrame(sample);
      }, mode);
      let release!: () => void, arrived!: () => void;
      const held = new Promise<void>(resolve => { release = resolve; });
      const requested = new Promise<void>(resolve => { arrived = resolve; });
      await page.route('**/static/lib/theme-init.js', async route => { arrived(); await held; await route.continue(); });
      const navigation = page.goto(`${app.base}/${screen}`);
      await requested;
      await expect.poll(() => page.evaluate(() => document.body === null)).toBe(true);
      release(); await navigation; await ready(page); await color(page, mode);
      const frames = () => page.evaluate(() => (window as typeof window & { themeFrames: { theme: string; background: string; time: number }[] }).themeFrames);
      await expect.poll(async () => (await frames()).length).toBeGreaterThan(0);
      await expect.poll(async () => (await frames()).every(frame => frame.theme === mode && frame.background === (mode === 'dark' ? 'rgb(15, 18, 24)' : 'rgb(236, 239, 243)'))).toBe(true);
      writeFileSync(info.outputPath('first-frames.json'), JSON.stringify({ frames: await frames(), paints: await page.evaluate(() => performance.getEntriesByType('paint').map(entry => ({ name: entry.name, time: entry.startTime }))) }, null, 2));
    });
  }
}

test('theme menu stays within each screen at narrow and wide widths and supports keyboard', async ({ page, app }) => {
  for (const width of [1440, 1000, 760, 375, 2560]) {
    await page.setViewportSize({ width, height: 900 });
    for (const screen of screens) {
      await page.goto(`${app.base}/${screen}`); await ready(page);
      const trigger = page.getByRole('button', { name: /^明暗を選ぶ/ });
      await trigger.focus(); await page.keyboard.press('ArrowDown');
      await expect(page.getByRole('menuitemradio', { name: 'OS に従う', exact: true })).toBeFocused();
      await page.keyboard.press('Home'); await expect(page.getByRole('menuitemradio', { name: 'ライト', exact: true })).toBeFocused();
      await page.keyboard.press('ArrowDown'); await page.keyboard.press('Enter'); await color(page, 'dark');
      await expect(trigger).toBeFocused();
      await choose(page, 'system');
      await trigger.click();
      await expect.poll(() => page.locator('.theme-panel').evaluate(panel => {
        const r = panel.getBoundingClientRect(); return r.left >= 0 && r.right <= innerWidth;
      })).toBe(true);
      await expect.poll(() => page.locator('.theme-trigger').evaluate(button => {
        const r = button.getBoundingClientRect();
        return r.width >= 34 && r.height >= 34 && [...document.querySelectorAll('.crumb > :not(.theme-control), .screens, .hq-search, .apl-bandbtn')].every(other => {
          const b = other.getBoundingClientRect(); return Math.min(r.right, b.right) <= Math.max(r.left, b.left) || Math.min(r.bottom, b.bottom) <= Math.max(r.top, b.top);
        });
      })).toBe(true);
      await page.keyboard.press('Escape'); await expect(trigger).toBeFocused();
      await trigger.click(); await page.locator('h1').click(); await expect(page.getByRole('menu', { name: '明暗' })).toBeHidden();
    }
  }
});

test('theme survives closing and reopening the browser profile', async ({ app }) => {
  const profile = mkdtempSync(path.join(os.tmpdir(), 'hergy-theme-'));
  let context = await chromium.launchPersistentContext(profile, { headless: true, colorScheme: 'light' });
  try {
    let page = context.pages()[0];
    await page.goto(`${app.base}/teams`); await ready(page); await choose(page, 'dark');
    await context.close();
    context = await chromium.launchPersistentContext(profile, { headless: true, colorScheme: 'light' });
    page = context.pages()[0];
    await page.goto(`${app.base}/roles`); await ready(page); await color(page, 'dark');
  } finally { await context.close(); rmSync(profile, { recursive: true, force: true }); }
});

test('invalid saved values follow OS and unavailable initialization leaves a disabled control', async ({ page, app }) => {
  await page.emulateMedia({ colorScheme: 'dark' });
  await page.addInitScript(() => localStorage.setItem('hergy.theme', 'invalid'));
  await page.goto(`${app.base}/teams`); await ready(page); await color(page, 'dark');
  await expect(page.getByRole('button', { name: '明暗を選ぶ（現在: OS に従う）', exact: true })).toBeEnabled();
  await expect(page.locator('.theme-notice')).toBeHidden();
  await page.route('**/static/lib/theme-init.js', route => route.fulfill({ contentType: 'text/javascript', body: '' }));
  await page.reload(); await ready(page); await color(page, 'dark');
  await expect(page.getByRole('button', { name: /^明暗を選ぶ/ })).toBeDisabled();
});


test('switching theme keeps the unsaved rules draft', async ({ page, app }) => {
  await page.goto(`${app.base}/teams?open=core&agree=core`); await ready(page);
  const input = page.locator('.tdef[data-name="core"] .agree-p textarea');
  await input.fill('# 下書き\n保存前の内容');
  await choose(page, 'dark'); await color(page, 'dark');
  await expect(input).toHaveValue('# 下書き\n保存前の内容');
  await choose(page, 'light'); await expect(input).toHaveValue('# 下書き\n保存前の内容');
});
