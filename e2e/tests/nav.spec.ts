// NAV-N6 the changes one screen carries — checks/nav.md's 大 item, walked in a
// browser. It is internal/app e2eNavCarry, test for test and assertion for
// assertion, over the same fixture (testdata/profiles-v2 + testdata/roles-v2 +
// testdata/nav): the pick a person makes and does not save is carried to the
// other screen and back, is dropped when the file moved outside, is not put back
// by a reload, and a draft the band did not carry is spent for good (N6〜N9,
// R2, R3).
import { test, expect } from './fixtures';
import { readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { choose, press, q, read, set, there, wait, waitPath } from './helpers';

// pick is the assign row N6〜N9 work on: the core team's impl-a, whose profile
// the fixture has as codex-high.
const pick = '.team[data-g="core"] .arow[data-k="m-2"]';

// The navigation band's fixture: no team of this project is running.
test.use({ reads: 'nav' });

test('NAV-N6 the changes one screen carries', async ({ page, app }) => {
  // N6: make a pick in the assign tab and walk to the role screen with the
  // band. The band shows a mark on the profile screen.
  await page.goto(`${app.base}/profiles/assign`);
  await wait(page, there('.team[data-g="core"]'));
  await choose(page, q(`${pick} select`), 'opus-deep');
  await wait(page, `${q(pick)}.classList.contains('changed')`);
  expect(
    await read<string>(page, `String(!!${q('.scr[data-screen="profiles"] .dot')})`),
    'N6: the band marks the profile screen, want a dot',
  ).toBe('true');

  await press(page, q('a.scr[data-screen="roles"]'));
  await waitPath(page, '/roles');
  await wait(page, there('#rows'));
  expect(
    await read<string>(page, `String(!!${q('a.scr[data-screen="profiles"] .dot')})`),
    'N6: the changed pick left no dot on the profile screen',
  ).toBe('true');
  expect(
    await read<number>(page, `document.querySelectorAll('.teams .arow.changed').length`),
    'N6: the role screen shows changed picks, want none',
  ).toBe(0);

  // N7: walking back puts the pick where it was, marked as not saved yet.
  await press(page, q('a.scr[data-screen="profiles"]'));
  await waitPath(page, '/profiles/assign');
  await wait(page, `${q(pick)}.classList.contains('changed')`);
  expect(
    await read<string>(page, `${q(`${pick} s.was`)}.textContent || ''`),
    'N7: the carried pick does not show what it was',
  ).toBe('codex-high');
  expect(
    await read<string>(page, `${q('.team[data-g="core"] .tside .dirty')}.textContent || ''`),
    'N7: the team head does not read 1 人',
  ).toBe('1 人');
  expect(
    await read<string>(page, `String(!!${q('.scr.on .dot')})`),
    'N7: the screen the person is on shows no change mark',
  ).toBe('true');

  // N9: a reload puts nothing back — the band carried it, and it is spent.
  await page.reload();
  await wait(page, there('.team[data-g="core"]'));
  expect(
    await read<number>(page, `document.querySelectorAll('.arow.changed').length`),
    'N9: a reload put changed picks back, want none',
  ).toBe(0);

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
  expect(
    await read<number>(page, `document.querySelectorAll('.arow.changed').length`),
    'R2: opening the profile screen by URL put changed picks back, want none',
  ).toBe(0);
  expect(
    await read<string>(page, `String(!!${q('.screens .dot')})`),
    'R2: the band marks a screen after a typed URL dropped the draft',
  ).toBe('false');
  await press(page, q('a.scr[data-screen="roles"]'));
  await waitPath(page, '/roles');
  await wait(page, there('#rows'));
  expect(
    await read<string>(page, `String(!!${q('a.scr[data-screen="profiles"] .dot')})`),
    'R2: the profile screen still shows a dot after its draft was dropped',
  ).toBe('false');
  await press(page, q('a.scr[data-screen="profiles"]'));
  await waitPath(page, '/profiles/assign');
  await wait(page, there('.team[data-g="core"]'));
  expect(
    await read<string>(page, `${q(`${pick} select`)}.value`),
    'R2: the pick was not put back after the dropped draft',
  ).toBe('codex-high');
  expect(
    await read<number>(page, `document.querySelectorAll('.arow.changed').length`),
    'R2: a draft the band did not carry came back, want none',
  ).toBe(0);

  // N6 again, for N8: carry the pick over, change team.yaml outside, and walk
  // back — the draft is not put back, and the band's mark goes.
  await choose(page, q(`${pick} select`), 'opus-deep');
  await wait(page, `${q(pick)}.classList.contains('changed')`);
  await press(page, q('a.scr[data-screen="roles"]'));
  await waitPath(page, '/roles');
  const team = path.join(app.proj, '.hergy', 'teams', 'core', 'team.yaml');
  writeFileSync(team, readFileSync(team, 'utf8').replace('注文 API を作り直す', '外で書き換えた使命'));
  await press(page, q('a.scr[data-screen="profiles"]'));
  await waitPath(page, '/profiles/assign');
  await wait(page, there('.team[data-g="core"]'));
  expect(
    await read<number>(page, `document.querySelectorAll('.arow.changed').length`),
    'N8: a draft made before the file moved outside was put back, want none',
  ).toBe(0);
  expect(
    await read<string>(page, `String(!!${q('.screens .dot')})`),
    'N8: the band still marks a screen after the draft was dropped',
  ).toBe('false');

  // R3: a field inside a role's row does not redraw the screen, but it is a
  // change all the same — the band's mark comes with it and goes when the
  // field is put back.
  await page.goto(`${app.base}/roles?open=builder`);
  await wait(page, there('.role.open .panel[data-l="project"] [data-f="title"]'));
  const title = '.role.open .panel[data-l="project"] [data-f="title"]';
  await set(page, q(title), '設計と実装');
  await wait(page, `!!${q('.scr.on[data-screen="roles"] .dot')}`);
  await set(page, q(title), '設計実装');
  await wait(page, `!${q('.scr.on[data-screen="roles"] .dot')}`);
  expect(
    await read<string>(page, `String(!!${q('.screens .dot')})`),
    'R3: the band still marks a screen after the field was put back',
  ).toBe('false');
});
