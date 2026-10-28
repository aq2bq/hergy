// ACTIVITY-FIGURE checks the packed labels in a real browser at both requested
// viewport widths, and saves each view for visual comparison (r-8b04).
import { test, expect } from './fixtures';

const members = [
  { name: 'lead', herdr: 'large-lead', state: 'active', at: '' },
  { name: 'webui-design', herdr: 'large-webui-design', state: 'idle', at: '' },
  { name: 'long-reviewer', herdr: 'large-long-reviewer', state: 'blocked', at: '' },
  { name: 'qa', herdr: 'large-qa', state: 'absent', at: '' },
];
const workers = [
  { name: 'impl-b', from: 'lead', state: 'active' },
  { name: 'impl-a', from: 'lead', state: 'active' },
  { name: 'inv-b', from: 'webui-design', state: 'active' },
  { name: 'template-image', from: 'webui-design', state: 'idle' },
  { name: 'html-mail', from: 'long-reviewer', state: 'active' },
  { name: 'listing', from: 'long-reviewer', state: 'active' },
  { name: 'rules-audit', from: 'qa', state: 'active' },
];

test.use({ reads: 'activity' });

test('the activity figure labels do not overlap at 1440 and 760 pixels', async ({ page, app }, testInfo) => {
  const team = {
    name: 'large', project: app.proj, project_name: 'fixture',
    summary: { definition: 'large', state: 'working', last: '' },
  };
  await page.route('**/api/**', async (route) => {
    const pathname = new URL(route.request().url()).pathname;
    let body: unknown = {};
    if (pathname === '/api/activity') body = { items: [team], herdr: { ok: true, text: '' }, running: 1, waiting: 0 };
    else if (pathname === '/api/activity/large') body = { doc: { nodes: members, hired: workers, tail: { lines: [] }, herdr: '' } };
    else if (pathname === '/api/hq/projects') body = { mode: 'listed', current: { path: app.proj, name: 'fixture' }, projects: [{ path: app.proj, name: 'fixture', listed: true, definitions: 1 }], global: { definitions: 0 } };
    else if (pathname === '/api/hq/running') body = { herdr: { ok: true, text: '' }, teams: [] };
    else if (pathname === '/api/hq/workspaces') body = { workspaces: [] };
    else if (['/api/hq/teams', '/api/hq/roles', '/api/hq/profiles'].includes(pathname)) body = { groups: [] };
    else if (pathname === '/api/apply/count') body = { count: 0 };
    else if (pathname === '/api/apply') body = { herdr: { ok: true, text: '' }, count: 0, teams: [] };
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
  });

  await page.goto(`${app.base}/activity`);
  await expect(page.locator('.hq-detail .hq-dleft svg.packed')).toBeVisible();
  await page.evaluate(() => document.fonts.ready.then(() => true));

  for (const width of [1440, 760]) {
    await page.setViewportSize({ width, height: 1000 });
    const tile = page.locator('.hq-tile svg.packed');
    const detail = page.locator('.hq-detail .hq-dleft svg.packed');
    await expect(tile).toBeVisible();
    await expect(detail).toBeVisible();
    const sizes = await page.evaluate(() => {
      const boxes = [...document.querySelectorAll('.hq-tile svg.packed, .hq-detail .hq-dleft svg.packed')]
        .map((svg) => svg.getBoundingClientRect().width);
      return { tile: boxes[0], detail: boxes[1] };
    });
    expect(sizes.detail).toBeGreaterThan(sizes.tile);

    const overlaps = await detail.evaluate((svg) => {
      const rects = [...svg.querySelectorAll('text[data-label-width]')].map((label) => {
        const rect = label.getBBox();
        return { text: label.textContent, row: label.getAttribute('data-label-row'), x: rect.x, y: rect.y, right: rect.x + rect.width, bottom: rect.y + rect.height };
      });
      const found: string[] = [];
      for (let i = 0; i < rects.length; i++) for (let j = i + 1; j < rects.length; j++) {
        const a = rects[i], b = rects[j];
        if (a.x < b.right && a.right > b.x && a.y < b.bottom && a.bottom > b.y) found.push(`${a.text} / ${b.text}`);
      }
      return found;
    });
    expect(overlaps).toEqual([]);
    await page.screenshot({ path: testInfo.outputPath(`activity-figure-${width}.png`), fullPage: true });
  }
});
