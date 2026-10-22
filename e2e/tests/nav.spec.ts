// TestE2ENav's 大 item, written again in Playwright (d-3c9b). It is the same
// walk over the same fixture: internal/app/e2e_nav_test.go's e2eNavCarry, test
// for test, assertion for assertion.
//
// The test's name is the list's ID: checks/nav.md's 大 item NAV-N6 is the walk
// the band carries one screen's changes to the other and back (N6〜N9, R2/R3).
import { test, expect } from '@playwright/test';
import { writeFileSync } from 'node:fs';
import path from 'node:path';
import {
  choose, go, makeFixture, number, press, readFile, removeRoot, setField,
  startServe, text, wait, waitPath, watch,
} from './harness';

// navPick is the assign row N6〜N9 work on: the core team's impl-a, whose
// profile the fixture has as codex-high.
const pick = '.team[data-g="core"] .arow[data-k="m-2"]';
const pickChanged = () =>
  `document.querySelector('.team[data-g="core"] .arow[data-k="m-2"]').classList.contains('changed')`;

test('NAV-N6 the changes one screen carries', async ({ page }) => {
  const f = makeFixture('nav');
  const serve = await startServe(f);
  try {
    const said = watch(page, true);

    // N6: make a pick in the assign tab and walk to the role screen with the
    // band. The band shows a mark on the profile screen.
    await go(page, `${serve.url}/profiles/assign`);
    await wait(page, `!!document.querySelector('.team[data-g="core"]')`);
    await choose(page, `${pick} select`, 'opus-deep');
    await wait(page, pickChanged());
    expect(await text(page, `String(!!document.querySelector('.scr[data-screen="profiles"] .dot'))`),
      'N6: the band marks the profile screen, want a dot').toBe('true');

    await press(page, 'a.scr[data-screen="roles"]');
    await waitPath(page, '/roles');
    await wait(page, `!!document.querySelector('#rows')`);
    expect(await text(page, `String(!!document.querySelector('a.scr[data-screen="profiles"] .dot'))`),
      'N6: the changed pick left no dot on the profile screen').toBe('true');
    expect(await number(page, `document.querySelectorAll('.teams .arow.changed').length`),
      'N6: the role screen shows changed picks, want none').toBe(0);

    // N7: walking back puts the pick where it was, marked as not saved yet.
    await press(page, 'a.scr[data-screen="profiles"]');
    await waitPath(page, '/profiles/assign');
    await wait(page, pickChanged());
    expect(await text(page, `((document.querySelector('.team[data-g="core"] .arow[data-k="m-2"] s.was')||{}).textContent||'')`),
      'N7: the carried pick does not show what it was').toBe('codex-high');
    expect(await text(page, `((document.querySelector('.team[data-g="core"] .tside .dirty')||{}).textContent||'')`),
      "N7: the team's head does not read 1 人").toBe('1 人');
    expect(await text(page, `String(!!document.querySelector('.scr.on .dot'))`),
      'N7: the screen the person is on shows no change mark').toBe('true');

    // N9: a reload puts nothing back — the band carried it, and it is spent.
    await page.reload();
    await wait(page, `!!document.querySelector('.team[data-g="core"]')`);
    expect(await number(page, `document.querySelectorAll('.arow.changed').length`),
      'N9: a reload put changed picks back, want none').toBe(0);

    // R2: a draft the band did not carry is spent, not put back later. Make a
    // pick, walk to the role screen with the band, then open the profile screen
    // by typing its URL — the draft is dropped there. Walking the band back and
    // forth afterwards must not bring it back.
    await choose(page, `${pick} select`, 'opus-deep');
    await wait(page, pickChanged());
    await press(page, 'a.scr[data-screen="roles"]');
    await waitPath(page, '/roles');
    await wait(page, `!!document.querySelector('#rows')`);
    await go(page, `${serve.url}/profiles/assign`);
    await wait(page, `!!document.querySelector('.team[data-g="core"]')`);
    expect(await number(page, `document.querySelectorAll('.arow.changed').length`),
      'R2: opening the profile screen by URL put changed picks back, want none').toBe(0);
    expect(await text(page, `String(!!document.querySelector('.screens .dot'))`),
      'R2: the band marks a screen after a typed URL dropped the draft').toBe('false');
    await press(page, 'a.scr[data-screen="roles"]');
    await waitPath(page, '/roles');
    await wait(page, `!!document.querySelector('#rows')`);
    expect(await text(page, `String(!!document.querySelector('a.scr[data-screen="profiles"] .dot'))`),
      'R2: the profile screen still shows a dot after its draft was dropped').toBe('false');
    await press(page, 'a.scr[data-screen="profiles"]');
    await waitPath(page, '/profiles/assign');
    await wait(page, `!!document.querySelector('.team[data-g="core"]')`);
    expect(await text(page, `document.querySelector('.team[data-g="core"] .arow[data-k="m-2"] select').value`),
      'R2: the pick was not put back after the dropped draft').toBe('codex-high');
    expect(await number(page, `document.querySelectorAll('.arow.changed').length`),
      'R2: a draft the band did not carry came back, want none').toBe(0);

    // N6 again, for N8: carry the pick over, change team.yaml outside, and walk
    // back — the draft is not put back, and the band's mark goes.
    await choose(page, `${pick} select`, 'opus-deep');
    await wait(page, pickChanged());
    await press(page, 'a.scr[data-screen="roles"]');
    await waitPath(page, '/roles');
    const team = path.join(f.proj, '.hergy', 'teams', 'core', 'team.yaml');
    writeFileSync(team, readFile(team).replace('注文 API を作り直す', '外で書き換えた使命'));
    await press(page, 'a.scr[data-screen="profiles"]');
    await waitPath(page, '/profiles/assign');
    await wait(page, `!!document.querySelector('.team[data-g="core"]')`);
    expect(await number(page, `document.querySelectorAll('.arow.changed').length`),
      'N8: a draft made before the file moved outside was put back, want none').toBe(0);
    expect(await text(page, `String(!!document.querySelector('.screens .dot'))`),
      'N8: the band still marks a screen after the draft was dropped').toBe('false');

    // R3: a field inside a role's row does not redraw the screen, but it is a
    // change all the same — the band's mark comes with it and goes when the
    // field is put back.
    await go(page, `${serve.url}/roles?open=builder`);
    await wait(page, `!!document.querySelector('.role.open .panel[data-l="project"] [data-f="title"]')`);
    const title = `document.querySelector('.role.open .panel[data-l="project"] [data-f="title"]')`;
    await setField(page, title, '設計と実装');
    await wait(page, `!!document.querySelector('.scr.on[data-screen="roles"] .dot')`);
    await setField(page, title, '設計実装');
    await wait(page, `!document.querySelector('.scr.on[data-screen="roles"] .dot')`);
    expect(await text(page, `String(!!document.querySelector('.screens .dot'))`),
      'R3: the band still marks a screen after the field was put back').toBe('false');

    expect(said, 'the band logged console errors').toEqual([]);
  } finally {
    await serve.stop();
    removeRoot(f);
  }
});
