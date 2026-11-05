import { test, expect } from './fixtures';
import { ready } from './helpers';
import { readFileSync } from 'node:fs';
import path from 'node:path';

test.use({ reads: 'activity' });
for (const width of [1440, 1000, 640]) {
 test(`selected prose stays readable and keeps drafts at ${width}px`, async ({ page, app }) => {
  await page.setViewportSize({ width, height: 1000 });
  await page.goto(app.base + '/teams?open=core&agree=core'); await ready(page);
  const row = page.locator('.tdef[data-name="core"]');
  const team = row.locator('.agree-p textarea');
  const role = row.locator('.role-column').filter({ has: page.getByRole('heading', { name: /builder/ }) });
  await expect(row.locator('.prose-column:visible')).toHaveCount(1);
  await expect(row.locator('.hq-runc')).toBeVisible();
  await expect(row.locator('.sum .c-size')).not.toContainText('目安');
  await expect(team).toHaveCSS('font-size', '16px');
  const box = (await team.boundingBox())!;
  expect(box.width).toBeLessThanOrEqual(45 * 16);
  if (width >= 1000) expect(box.width).toBeGreaterThanOrEqual(35 * 16);
  const originalTeam = readFileSync(path.join(app.proj, '.hergy/teams/core/agreement.md'), 'utf8');
  await team.fill(originalTeam + '\n# 残す下書き\n');
  const builder = row.locator('.prose-documents').getByRole('button', { name: /^ロール builder / });
  await builder.focus(); await builder.press('Enter');
  await expect(role).toBeVisible(); await expect(row.locator('.prose-column:visible')).toHaveCount(1);
  const originalRole = readFileSync(path.join(app.proj, '.hergy/roles/builder.md'), 'utf8');
  await role.getByRole('textbox').fill(originalRole + '\n# ロールの編集\n');
  await row.locator('.prose-documents').getByRole('button', { name: /^チームの規則 core / }).click();
  await expect(team).toHaveValue(originalTeam + '\n# 残す下書き\n');
  await expect(builder).toContainText('未保存');
  await builder.click(); await role.getByRole('button', { name: '保存', exact: true }).click();
  await expect(role.locator('.prose-status')).toContainText('保存しました');
  expect(readFileSync(path.join(app.proj, '.hergy/teams/core/agreement.md'), 'utf8')).toBe(originalTeam);
  expect(readFileSync(path.join(app.proj, '.hergy/roles/builder.md'), 'utf8')).toBe(originalRole + '\n# ロールの編集\n');
  await row.locator('.prose-documents').getByRole('checkbox', { name: 'チームの規則 core を並べる' }).check();
  await expect(row.locator('.prose-column:visible')).toHaveCount(2);
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
 });
}

test('all screens keep compact, unclipped bands, including pending changes and narrow search', async ({ page, app }, info) => {
 for (const width of [420, 640, 700, 760, 1000, 1100, 1101, 1440]) {
  await page.setViewportSize({ width, height: 1000 });
  const bands: unknown[] = [];
  for (const screen of ['activity', 'teams', 'roles', 'profiles']) {
   await page.goto(app.base + '/' + screen); await ready(page);
   await page.evaluate(() => document.fonts.ready);
   await expect(page.locator('.apl-bandbtn')).toBeVisible();
   await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
   await expect.poll(() => page.locator('.scr').evaluateAll(tabs => tabs.every(tab => {
    const box = tab.getBoundingClientRect(), range = document.createRange(); range.selectNodeContents(tab);
    return [...range.getClientRects()].every(rect => rect.left >= box.left && rect.right <= box.right);
   }))).toBe(true);
   if (width === 1440) await expect.poll(() => page.locator('.bar').evaluate(el => el.getBoundingClientRect().height)).toBeLessThanOrEqual(110);
   if (width === 1000 || width === 1100) await expect.poll(() => page.locator('.search-toggle').evaluate(el => el.getBoundingClientRect().bottom <= document.querySelector('.screens')!.getBoundingClientRect().top)).toBe(true);
   if (width > 420) await expect.poll(() => page.locator('.theme-trigger').evaluate(el => { const a = el.getBoundingClientRect(), b = document.querySelector('.search-toggle')!.getBoundingClientRect(); return Math.abs((a.top + a.bottom - b.top - b.bottom) / 2); })).toBeLessThanOrEqual(2);
   if (screen === 'activity') await page.screenshot({ path: info.outputPath(`band-${width}.png`) });
   bands.push(await page.locator('.bar').evaluate(el => {
    const b = el.getBoundingClientRect(), n = el.querySelector('.screens')!.getBoundingClientRect();
    return { height: b.height, x: n.x, y: n.y, width: n.width };
   }));
  }
  for (const band of bands) expect(band).toEqual(bands[0]);
  if (width <= 1600) {
   const search = page.getByRole('searchbox', { name: '並べたプロジェクトを横断して探す' });
   await expect(search).toBeHidden(); await page.getByRole('button', { name: '探す', exact: true }).click();
   await expect(search).toBeFocused(); await search.fill('core');
   await expect(page.locator('.hq-sr')).toBeVisible();
  }
 }
});

test('definition description has one editing field and narrow selection leads to it', async ({ page, app }) => {
 await page.setViewportSize({ width: 640, height: 1000 });
 await page.goto(app.base + '/teams?open=core'); await ready(page);
 const row = page.locator('.tdef[data-name="core"]');
 await expect(row.locator('input[data-cv-f="description"]')).toHaveCount(1);
 await expect(row.locator('.sum .in-description')).toHaveCount(0);
 await row.locator('[data-cv-n="impl-a"]').click();
 await row.getByRole('button', { name: '選択した属性を編集' }).click();
 await expect(row.locator('.cv-side')).toBeInViewport();
 await expect(row.locator('[data-cv-f="m-name"]')).toBeFocused();
 await expect(row.locator('[data-cv-f="m-name"]')).toBeInViewport();
});

test('both themes keep counts and YAML line numbers readable', async ({ page, app }) => {
 for (const theme of ['light', 'dark']) for (const screen of ['activity', 'teams', 'roles', 'profiles/yaml']) {
  await page.emulateMedia({ colorScheme: theme });
  await page.goto(app.base + '/' + screen); await ready(page);
  const ratios = await page.locator('.scr:not(.on) .n, .gutter').evaluateAll(nodes => {
   const rgb = (value: string) => (value.match(/[\d.]+/g) || []).slice(0, 3).map(Number);
   const luminance = (color: number[]) => color.map(v => { v /= 255; return v <= .04045 ? v / 12.92 : ((v + .055) / 1.055) ** 2.4; }).reduce((n, v, i) => n + v * [.2126, .7152, .0722][i], 0);
   const tokens = getComputedStyle(document.documentElement);
   const level = (token: string) => luminance(tokens.getPropertyValue(token).trim().slice(1).match(/../g)!.map(hex => parseInt(hex, 16)));
   if (!(Math.abs(level('--ink') - level('--muted')) > 0 && Math.abs(level('--ink') - level('--faint')) > Math.abs(level('--ink') - level('--muted')))) throw new Error('supplementary text hierarchy is reversed');
   return nodes.filter(node => node.getClientRects().length).map(node => {
    let parent: Element | null = node;
    while (parent && getComputedStyle(parent).backgroundColor === 'rgba(0, 0, 0, 0)') parent = parent.parentElement;
    const foreground = luminance(rgb(getComputedStyle(node).color));
    const background = luminance(rgb(getComputedStyle(parent!).backgroundColor));
    return (Math.max(foreground, background) + .05) / (Math.min(foreground, background) + .05);
   });
  });
  expect(ratios.length).toBeGreaterThanOrEqual(3);
  for (const ratio of ratios) expect(ratio).toBeGreaterThanOrEqual(4.5);
 }
});

test('narrow team descriptions use the row width and names never overlap badges', async ({ page, app }, info) => {
 for (const width of [375, 768]) for (const theme of ['light', 'dark'] as const) {
  await page.setViewportSize({ width, height: 1100 }); await page.emulateMedia({ colorScheme: theme });
  await page.goto(app.base + '/teams'); await ready(page); await page.evaluate(() => document.fonts.ready);
  const rows = page.locator('.tdef > .sum');
  await expect.poll(() => rows.count()).toBeGreaterThanOrEqual(4);
  for (const row of await rows.all()) {
   const box = (await row.boundingBox())!, description = (await row.locator('.c-description').boundingBox())!;
   expect(description.width).toBeGreaterThanOrEqual(box.width / 2);
   await expect.poll(() => row.locator('.c-name .nm, .c-name .sev, .c-place .pill').evaluateAll(nodes => {
    const boxes = nodes.map(node => node.getBoundingClientRect()).filter(box => box.width && box.height);
    return boxes.some((a, i) => boxes.some((b, j) => j > i && Math.min(a.right, b.right) > Math.max(a.left, b.left) && Math.min(a.bottom, b.bottom) > Math.max(a.top, b.top)));
   })).toBe(false);
  }
  await page.screenshot({ path: info.outputPath(`team-list-${width}-${theme}.png`), fullPage: true });
 }
});

test('activity cards align their actions and explain unknown ownership once', async ({ page, app }, info) => {
 await page.setViewportSize({ width: 1440, height: 1100 });
 await page.goto(app.base + '/activity'); await ready(page);
 await expect(page.locator('.hq-worker-guide')).toHaveCount(1);
 await expect.poll(() => page.locator('.hq-tactions').evaluateAll(nodes => nodes.every(el => {
  const box = el.getBoundingClientRect(), stop = el.querySelector('.danger')!.getBoundingClientRect();
  return stop.width < box.width / 2 && box.right - stop.right <= 15 && [...el.children].filter(child => !child.matches('.danger')).every(child => child.getBoundingClientRect().bottom < stop.top);
 }))).toBe(true);
 await expect.poll(() => page.locator('.hq-tile .hq-tactions').evaluateAll(nodes => {
  const tops = nodes.slice(0, 3).map(node => node.getBoundingClientRect().top);
  return Math.max(...tops) - Math.min(...tops);
 })).toBeLessThan(1);
 await page.locator('.hq-tactions').first().scrollIntoViewIfNeeded();
 await page.screenshot({ path: info.outputPath('activity-actions.png') });
});


test('profile state stays immediately after its path', async ({ page, app }, info) => {
 await page.setViewportSize({ width: 1440, height: 1100 }); await page.goto(app.base + '/profiles'); await ready(page);
 await page.locator('.row[data-name="sonnet-fast"] .sum').first().click();
 const header = page.locator('.panel .ph').filter({ has: page.locator('.state.on') }).first();
 await expect(header.locator('.state.on')).toBeVisible();
 await expect.poll(() => header.evaluate(el => el.querySelector('.state')!.getBoundingClientRect().left - el.querySelector('.path')!.getBoundingClientRect().right)).toBeLessThanOrEqual(12);
 await page.screenshot({ path: info.outputPath('profile-state.png') });
});
