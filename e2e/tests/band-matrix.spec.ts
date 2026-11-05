import { test, expect } from './fixtures';
import { ready } from './helpers';
import { chmodSync } from 'node:fs';
import path from 'node:path';

// r-ea17, ac-89ea: the band's numbers are the rows the lists draw, in both
// displays (one project and the headquarters' list), on all four screens, for
// all three kinds. Each display breaks one file of each kind before it reads
// them, so the rows that cannot be read are on the axis too (d-17ab); the kinds
// of row (a legacy role, a name with no document, a name in both layers) are
// band-roles-count.spec.ts's.
const KINDS = [
  { screen: 'teams', rows: '.tdef', band: 'teams' },
  { screen: 'roles', rows: '#rows .role', band: 'roles' },
  { screen: 'profiles', rows: '#rows .row', band: 'profiles' },
] as const;

type Counts = Record<(typeof KINDS)[number]['band'], number>;

// countsOf reads each kind's list and counts the rows it draws.
async function countsOf(page, base: string): Promise<Counts> {
  const out = { teams: 0, roles: 0, profiles: 0 };
  for (const kind of KINDS) {
    await page.goto(`${base}/${kind.screen}`);
    await ready(page);
    out[kind.band] = await page.locator(kind.rows).count();
  }
  return out;
}

// everyScreen holds each band's number to the rows everywhere: the screen a
// kind belongs to draws its list, and the other three draw no list of it.
async function everyScreen(page, base: string, want: Counts): Promise<void> {
  for (const screen of ['activity', 'teams', 'roles', 'profiles']) {
    await page.goto(`${base}/${screen}`);
    await ready(page);
    for (const kind of KINDS) {
      const band = page.locator(`.scr[data-screen="${kind.band}"] .n`);
      await expect.poll(() => band.textContent()).toBe(String(want[kind.band]));
    }
  }
}

// unreadable takes the read permission from one file of each kind, and gives it
// back after the walk.
function unreadable(files: string[]): () => void {
  for (const file of files) chmodSync(file, 0o000);
  return () => { for (const file of files) chmodSync(file, 0o644); };
}

test.describe('one project', () => {
  test.use({ reads: 'teams' });

  test('the band is the rows every list draws, on every screen', async ({ page, app }) => {
    // One team.yaml, one role document and one profiles.yaml this project has:
    // each stands as a row that cannot be read (d-17ab).
    const restore = unreadable([
      path.join(app.proj, '.hergy/teams/core/team.yaml'),
      path.join(app.proj, '.hergy/roles/lead.md'),
      path.join(app.proj, '.hergy/profiles.yaml'),
    ]);
    await page.setViewportSize({ width: 1440, height: 900 });
    await everyScreen(page, app.base, await countsOf(page, app.base));
    restore();
  });
});

test.describe('the headquarters list', () => {
  test.use({ reads: 'hq' });

  test('the band is the rows every list draws, on every screen', async ({ page, app }) => {
    // The global layer's role and profiles, and this serve's project's team:
    // the rows of a display that cannot be read (d-17ab).
    const restore = unreadable([
      path.join(app.cfg, 'hergy/roles/lead.md'),
      path.join(app.cfg, 'hergy/profiles.yaml'),
      path.join(app.proj, '.hergy/teams/alpha-dev/team.yaml'),
    ]);
    await page.setViewportSize({ width: 1440, height: 900 });
    await everyScreen(page, app.base, await countsOf(page, app.base));
    restore();
  });
});
