// TestE2EProfiles' two 大 items, written again in Playwright (d-3c9b):
// internal/app/e2e_profiles_test.go's e2eProfilesOpen (PROFILES-P1) and
// e2eProfilesUsage (PROFILES-P11), assertion for assertion.
import { test, expect } from '@playwright/test';
import {
  go, makeFixture, number, press, removeRoot, setField, startServe, text,
  truth, wait, waitFor, watch,
} from './harness';

test('PROFILES-P1 rows open in place', async ({ page }) => {
  const f = makeFixture('profiles');
  const serve = await startServe(f);
  try {
    const said = watch(page, false);
    await go(page, `${serve.url}/profiles`);
    await waitFor(page, '#rows .row');
    await press(page, `document.querySelector('#rows .row[data-name="sonnet-fast"] .sum')`);
    await wait(page, `!document.querySelector('#rows .row[data-name="sonnet-fast"] .detail').hidden`);
    expect(await text(page, 'window.location.search'),
      'opening a row did not write the URL').toContain('open=sonnet-fast');
    expect(await number(page, `document.querySelectorAll('#rows .row[data-name="sonnet-fast"] .detail .panel').length`),
      'sonnet-fast panels').toBe(2);
    expect(await text(page,
      `[...document.querySelectorAll('#rows .row[data-name="sonnet-fast"] .detail .panel .ln')].map(e=>e.textContent).join(',')`),
      'the panels').toBe('プロジェクト,共通');
    expect(await text(page,
      `[...document.querySelectorAll('#rows .row[data-name="sonnet-fast"] .panel .state')].map(e=>e.textContent).join(',')`),
      'the states').toBe('効いている,隠れている');
    expect(await number(page, `document.querySelectorAll('#rows .row[data-name="sonnet-fast"] .panel .field').length`),
      'the two panels fields').toBe(8);
    for (const name of ['project', 'global']) {
      expect(await text(page,
        `document.querySelector('#rows .row[data-name="sonnet-fast"] .panel[data-file=${name}] .path').textContent`),
        `the ${name} panel's head`).toMatch(/profiles\.yaml$/);
    }
    expect(await text(page, `getComputedStyle(document.querySelector('.uchip')).color`),
      "the name on a person's chip").toBe(await text(page, `getComputedStyle(document.body).color`));
    expect(await number(page, `document.querySelectorAll('.uchip .g').length`),
      "a member's role on a chip is the faint mono word").toBeGreaterThan(0);
    const visible = `(function(){var d=document.querySelector('#rows .row[data-name="sonnet-fast"] .detail');if(!d){return;}var r=d.getBoundingClientRect();return r.top < window.innerHeight && r.bottom > 0;})()`;
    expect(await truth(page, visible), 'the opened detail is not in the view').toBe(true);

    // P1 again: the same row closes.
    await press(page, `document.querySelector('#rows .row[data-name="sonnet-fast"] .sum')`);
    await wait(page, `document.querySelector('#rows .row[data-name="sonnet-fast"] .detail').hidden`);
    expect(await text(page, 'window.location.search'),
      'closing the row left open= in the URL').not.toContain('open=');
    // R1: a name both files hold still counts the readers of the panel's own file.
    await go(page, `${serve.url}/profiles?open=sonnet-fast`);
    await waitFor(page, `#rows .row[data-name=sonnet-fast]`);
    await setField(page,
      `document.querySelector('#rows .row[data-name="sonnet-fast"] .panel[data-file=project] [data-f=name]')`, 'sonnet-5.5');
    await wait(page, `!document.querySelector('#rows .row[data-name="sonnet-fast"] .panel[data-file=project] .rename').hidden`);
    expect(await text(page,
      `document.querySelector('#rows .row[data-name="sonnet-fast"] .panel[data-file=project] .rename span').textContent`),
      'the rename hint').toContain('使っている 3 人の指定も sonnet-5.5 に書き換える');
    expect(await truth(page,
      `document.querySelector('#rows .row[data-name="sonnet-fast"] .panel[data-file=project] .rename input').checked`),
      'the rename box is not ticked at first').toBe(true);
    await go(page, `${serve.url}/profiles`);
    await waitFor(page, '#rows .row');

    expect(said, 'the profile screen logged console errors').toEqual([]);
  } finally {
    await serve.stop();
    removeRoot(f);
  }
});

test('PROFILES-P11 usage jumps', async ({ page }) => {
  const f = makeFixture('profiles');
  const serve = await startServe(f);
  try {
    const said = watch(page, false);
    // P11: a person's chip moves to the assign tab, centers the person's row and
    // lights it, with the cursor in the select.
    await go(page, `${serve.url}/profiles?open=opus-plan`);
    await waitFor(page, '.uchip');
    await press(page, `document.querySelector('#rows .row[data-name="opus-plan"] .uchip')`);
    await wait(page, `!!document.querySelector('.team[data-g=core] .arow[data-k="m-0"].flash')`);
    expect(await text(page, `document.activeElement.tagName`),
      "the person's row does not have the focus").toBe('SELECT');
    expect(await text(page, `document.activeElement.closest('.arow').dataset.k`),
      'the focus').toBe('m-0');
    expect(said, 'the profile screen logged console errors').toEqual([]);
  } finally {
    await serve.stop();
    removeRoot(f);
  }
});
