// PROFILES-P1 and PROFILES-P11 — walked in a
// browser. Only what the browser alone can see is compared here (d-3e43): the
// row a person presses opens inside the view (d-d475), the box that rewrites the
// people using a renamed profile is ticked from the start, and a person's chip
// puts the cursor on the person's row of the assign tab.
import { test } from './fixtures';
import { press, q, readTo, set, there, wait } from './helpers';

test.use({ reads: 'profiles' });

// row is the list row one profile name stands on.
const row = (name: string) => `#rows .row[data-name="${name}"]`;

test('PROFILES-P1 rows open in place', async ({ page, app }) => {
  // P1: the row opens under itself, and the detail it opens stands in the view
  // (d-d475: a press changed something off screen).
  await page.goto(`${app.base}/profiles`);
  await wait(page, there('#rows .row'));
  await press(page, `${q(`${row('sonnet-fast')} .sum`)}`);
  await wait(page, `!${q(`${row('sonnet-fast')} .detail`)}.hidden`);
  const visible = `(function(){var d=${q(`${row('sonnet-fast')} .detail`)};if(!d){return;}var r=d.getBoundingClientRect();return r.top < window.innerHeight && r.bottom > 0;})()`;
  await readTo<boolean>(page, visible, 'the opened detail is not in the view').toBe(true);

  // R1・P1: a name both files hold still counts the readers of the panel's own
  // file, and the box that rewrites them is ticked at first — the write the
  // screen starts with, which no 中 test can see.
  await page.goto(`${app.base}/profiles?open=sonnet-fast`);
  await wait(page, there(row('sonnet-fast')));
  await set(page, q(`${row('sonnet-fast')} .panel[data-file=project] [data-f=name]`), 'sonnet-5.5');
  await wait(page, `!${q(`${row('sonnet-fast')} .panel[data-file=project] .rename`)}.hidden`);
  await readTo<boolean>(
    page,
    `${q(`${row('sonnet-fast')} .panel[data-file=project] .rename input`)}.checked`,
    'the rename box is not ticked at first',
  ).toBe(true);
});

test('PROFILES-P11 usage jumps', async ({ page, app }) => {
  // P11: a person's chip moves to the assign tab, centers the person's row and
  // lights it.
  await page.goto(`${app.base}/profiles?open=opus-plan`);
  await wait(page, there('.uchip'));
  await press(page, q(`${row('opus-plan')} .uchip`));
  await wait(page, there('.team[data-g=core] .arow[data-k="m-0"].flash'));
  await readTo<string>(
    page,
    `document.activeElement.closest('.arow').dataset.k`,
    'the focus is on, want m-0',
  ).toBe('m-0');
});
