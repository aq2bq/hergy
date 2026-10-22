// NAV-N6 the changes one screen carries — walked in a
// browser. Only what the browser alone can see is compared here (d-3e43): the
// pick a person makes and does not save is carried to the other screen and back
// (N7), a draft the band did not carry is spent for good (R2), a draft made
// before the file moved outside is not put back (N8), and a field inside a row
// marks the band and unmarks it (R3).
import { test } from './fixtures';
import { readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { choose, press, q, readTo, set, there, wait, waitPath } from './helpers';

// pick is the assign row N6〜N9 work on: the core team's impl-a, whose profile
// the fixture has as codex-high.
const pick = '.team[data-g="core"] .arow[data-k="m-2"]';

// The navigation band's fixture: no team of this project is running.
test.use({ reads: 'nav' });

test('NAV-N6 the changes one screen carries', async ({ page, app }) => {
  // N6・N7: walking the band to the role screen and back puts the pick where it
  // was, marked as not saved yet.
  await page.goto(`${app.base}/profiles/assign`);
  await wait(page, there('.team[data-g="core"]'));
  await choose(page, q(`${pick} select`), 'opus-deep');
  await wait(page, `${q(pick)}.classList.contains('changed')`);
  await press(page, q('a.scr[data-screen="roles"]'));
  await waitPath(page, '/roles');
  await wait(page, there('#rows'));
  await press(page, q('a.scr[data-screen="profiles"]'));
  await waitPath(page, '/profiles/assign');
  await wait(page, `${q(pick)}.classList.contains('changed')`);
  await readTo<string>(
    page,
    `${q(`${pick} s.was`)}.textContent || ''`,
    'N7: the carried pick does not show what it was',
  ).toBe('codex-high');

  // R2: a draft the band did not carry is spent, not put back later. Make a
  // pick, walk to the role screen with the band, then open the profile screen
  // by typing its URL — the draft is dropped there. Walking the band back and
  // forth afterwards must not bring it back.
  await choose(page, q(`${pick} select`), 'opus-deep');
  await wait(page, `${q(pick)}.classList.contains('changed')`);
  await press(page, q('a.scr[data-screen="roles"]'));
  await waitPath(page, '/roles');
  await wait(page, there('#rows'));
  await page.goto(`${app.base}/profiles/assign`);
  await wait(page, there('.team[data-g="core"]'));
  await press(page, q('a.scr[data-screen="roles"]'));
  await waitPath(page, '/roles');
  await wait(page, there('#rows'));
  await press(page, q('a.scr[data-screen="profiles"]'));
  await waitPath(page, '/profiles/assign');
  await wait(page, there('.team[data-g="core"]'));
  await readTo<number>(
    page,
    `document.querySelectorAll('.arow.changed').length`,
    'R2: a draft the band did not carry came back, want none',
  ).toBe(0);

  // N8: carry the pick over, change team.yaml outside, and walk back — the
  // draft is not put back.
  await choose(page, q(`${pick} select`), 'opus-deep');
  await wait(page, `${q(pick)}.classList.contains('changed')`);
  await press(page, q('a.scr[data-screen="roles"]'));
  await waitPath(page, '/roles');
  const team = path.join(app.proj, '.hergy', 'teams', 'core', 'team.yaml');
  writeFileSync(team, readFileSync(team, 'utf8').replace('注文 API を作り直す', '外で書き換えた使命'));
  await press(page, q('a.scr[data-screen="profiles"]'));
  await waitPath(page, '/profiles/assign');
  await wait(page, there('.team[data-g="core"]'));
  await readTo<number>(
    page,
    `document.querySelectorAll('.arow.changed').length`,
    'N8: a draft made before the file moved outside was put back, want none',
  ).toBe(0);

  // R3: a field inside a role's row does not redraw the screen, but it is a
  // change all the same — the band's mark comes with it and goes when the
  // field is put back.
  await page.goto(`${app.base}/roles?open=builder`);
  await wait(page, there('.role.open .panel[data-l="project"] [data-f="title"]'));
  const title = '.role.open .panel[data-l="project"] [data-f="title"]';
  await set(page, q(title), '設計と実装');
  await readTo<string>(
    page,
    `String(!!${q('.scr.on[data-screen="roles"] .dot')})`,
    'R3: the band shows no mark while a field inside the row is changed',
  ).toBe('true');
  await set(page, q(title), '設計実装');
  await readTo<string>(
    page,
    `String(!!${q('.screens .dot')})`,
    'R3: the band still marks a screen after the field was put back',
  ).toBe('false');
});
