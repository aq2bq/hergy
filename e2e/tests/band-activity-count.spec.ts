import { test, expect } from './fixtures';
import { ready } from './helpers';
import { readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';

// r-ea17, ac-9b66: the band's アクティビティ and 待ち are the activity screen's
// tiles and waiting rows, on all four screens, and they follow a team that ends
// without the page being read again (the headquarters' own ten-second reading).
test.use({ reads: 'activity' });

// endTeam takes one team out of the fake herdr's tabs and agents: the team the
// fixture names by this tab is no longer running.
function endTeam(cfg: string, tab: string, label: string): void {
  const tabs = path.join(cfg, 'activity', 'herdr', 'tabs.json');
  const agents = path.join(cfg, 'activity', 'herdr', 'agents.json');
  const t = JSON.parse(readFileSync(tabs, 'utf8')) as { result: { tabs: { tab_id: string; label: string }[] } };
  t.result.tabs = t.result.tabs.filter((one) => one.tab_id !== tab);
  writeFileSync(tabs, JSON.stringify(t));
  const a = JSON.parse(readFileSync(agents, 'utf8')) as { result: { agents: { tab_id: string }[] } };
  a.result.agents = a.result.agents.filter((one) => one.tab_id !== tab);
  writeFileSync(agents, JSON.stringify(a));
  if (!label) throw new Error('no label');
}

test('the band is the activity screen, and it follows a team that ends', async ({ page, app }) => {
  const band = {
    running: () => page.locator('.screens .scr[data-screen="activity"] .n').textContent(),
    waiting: () => page.locator('.screens .scr[data-screen="activity"] .wait').textContent(),
  };
  const tiles = () => page.locator('.hq-tile[data-team]');
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(`${app.base}/activity`); await ready(page);
  const running = await tiles().count();
  // The waiting rows are the attention list's own number: 待ち N reads the same.
  const waitingText = await page.locator('.hq-att .hq-ah .n').first().textContent();
  const waiting = Number(waitingText || 0);
  await expect.poll(band.running).toBe(String(running));
  await expect.poll(band.waiting).toBe(`待ち ${waiting}`);

  // The same two numbers on the other three screens.
  for (const screen of ['teams', 'roles', 'profiles']) {
    await page.goto(`${app.base}/${screen}`); await ready(page);
    await expect.poll(band.running).toBe(String(running));
    await expect.poll(band.waiting).toBe(`待ち ${waiting}`);
  }

  // One team ends behind the screen. The page is not read again: the
  // headquarters' own re-reading carries the number in within ten seconds.
  await page.goto(`${app.base}/activity`); await ready(page);
  endTeam(app.cfg, 't4', 'research');
  await expect.poll(band.running, { timeout: 15_000 }).toBe(String(running - 1));
  await expect.poll(() => tiles().count(), { timeout: 15_000 }).toBe(running - 1);
  for (const screen of ['teams', 'roles', 'profiles']) {
    await page.goto(`${app.base}/${screen}`); await ready(page);
    await expect.poll(band.running).toBe(String(running - 1));
  }
});
