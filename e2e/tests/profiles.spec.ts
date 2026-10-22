// PROFILES-P1 and PROFILES-P11 — checks/profiles.md's 大 items, walked in a
// browser. They are internal/app e2eProfilesOpen and e2eProfilesUsage, test for
// test and assertion for assertion, over the same real-scale fixture
// (testdata/profiles-v2: 11 profiles, 4 definitions, 15 members, 3 workers).
import { test, expect } from './fixtures';
import { press, q, read, set, there, wait } from './helpers';

test.use({ reads: 'profiles' });

// row is the list row one profile name stands on.
const row = (name: string) => `#rows .row[data-name="${name}"]`;

test('PROFILES-P1 rows open in place', async ({ page, app }) => {
  // P1・S3・S4: the row opens under itself, the detail holds the people then the
  // two panels, and every panel is a head, name, kind, args, env and foot.
  await page.goto(`${app.base}/profiles`);
  await wait(page, there('#rows .row'));
  await press(page, `${q(`${row('sonnet-fast')} .sum`)}`);
  await wait(page, `!${q(`${row('sonnet-fast')} .detail`)}.hidden`);
  expect(
    await read<string>(page, `window.location.search`),
    'opening a row did not write the URL',
  ).toContain('open=sonnet-fast');
  expect(
    await read<number>(page, `document.querySelectorAll('${row('sonnet-fast')} .detail .panel').length`),
    'sonnet-fast shows panels, want the project and the shared one',
  ).toBe(2);
  expect(
    await read<string>(
      page,
      `[...document.querySelectorAll('${row('sonnet-fast')} .detail .panel .ln')].map(e=>e.textContent).join(',')`,
    ),
    'the panels read',
  ).toBe('プロジェクト,共通');
  expect(
    await read<string>(
      page,
      `[...document.querySelectorAll('${row('sonnet-fast')} .panel .state')].map(e=>e.textContent).join(',')`,
    ),
    'the states read',
  ).toBe('効いている,隠れている');
  expect(
    await read<number>(page, `document.querySelectorAll('${row('sonnet-fast')} .panel .field').length`),
    'the two panels hold fields, want 4 each',
  ).toBe(8);
  for (const name of ['project', 'global']) {
    expect(
      await read<string>(
        page,
        `document.querySelector('${row('sonnet-fast')} .panel[data-file=${name}] .path').textContent`,
      ),
      `the ${name} panel's head reads, want the file's path`,
    ).toContain('profiles.yaml');
  }
  expect(
    await read<string>(page, `getComputedStyle(${q('.uchip')}).color`),
    "the name on a person's chip is, want the body's ink",
  ).toBe(await read<string>(page, `getComputedStyle(document.body).color`));
  expect(
    await read<number>(page, `document.querySelectorAll('.uchip .g').length`),
    "a member's role on a chip is not the faint mono word",
  ).toBeGreaterThan(0);
  const visible = `(function(){var d=${q(`${row('sonnet-fast')} .detail`)};if(!d){return;}var r=d.getBoundingClientRect();return r.top < window.innerHeight && r.bottom > 0;})()`;
  expect(await read<boolean>(page, visible), 'the opened detail is not in the view').toBe(true);

  // P1 again: the same row closes.
  await press(page, `${q(`${row('sonnet-fast')} .sum`)}`);
  await wait(page, `${q(`${row('sonnet-fast')} .detail`)}.hidden`);
  expect(
    await read<string>(page, `window.location.search`),
    'closing the row left it in the URL',
  ).not.toContain('open=');

  // R1: a name both files hold still counts the readers of the panel's own file.
  await page.goto(`${app.base}/profiles?open=sonnet-fast`);
  await wait(page, there(row('sonnet-fast')));
  await set(page, q(`${row('sonnet-fast')} .panel[data-file=project] [data-f=name]`), 'sonnet-5.5');
  await wait(page, `!${q(`${row('sonnet-fast')} .panel[data-file=project] .rename`)}.hidden`);
  expect(
    await read<string>(page, `${q(`${row('sonnet-fast')} .panel[data-file=project] .rename span`)}.textContent`),
    'the rename hint reads',
  ).toContain('使っている 3 人の指定も sonnet-5.5 に書き換える');
  expect(
    await read<boolean>(page, `${q(`${row('sonnet-fast')} .panel[data-file=project] .rename input`)}.checked`),
    'the rename box is not ticked at first',
  ).toBe(true);
  await page.goto(`${app.base}/profiles`);
  await wait(page, there('#rows .row'));
});

test('PROFILES-P11 usage jumps', async ({ page, app }) => {
  // P11: a person's chip moves to the assign tab, centers the person's row and
  // lights it.
  await page.goto(`${app.base}/profiles?open=opus-plan`);
  await wait(page, there('.uchip'));
  await press(page, q(`${row('opus-plan')} .uchip`));
  await wait(page, there('.team[data-g=core] .arow[data-k="m-0"].flash'));
  expect(
    await read<string>(page, `document.activeElement.tagName`),
    "the person's row does not have the focus",
  ).toBe('SELECT');
  expect(
    await read<string>(page, `document.activeElement.closest('.arow').dataset.k`),
    'the focus is on, want m-0',
  ).toBe('m-0');
});
