// ROLES-P5 and ROLES-P14 — checks/roles.md's 大 items, walked in a browser. Only
// what the browser alone can see is compared here (d-3e43): the document-wide
// warning the screen's own script puts on the preview's first line (R4), the
// text a person types being the one line a save writes, and the role a person
// picks being the one line a save writes.
import { test, expect } from './fixtures';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { changedLines, choose, press, q, readTo, set, there, wait } from './helpers';

test.use({ reads: 'roles' });

// rule is one rule of the open role's panel, found by its id.
const rule = (id: string) =>
  `[...document.querySelectorAll('#rows .role.open .panel .rule')].find(function(r){return r.querySelector('.rid b').textContent.trim()===${JSON.stringify(id)}})`;

test('ROLES-P5 the rule a person edits', async ({ page, app }) => {
  const builder = path.join(app.proj, '.hergy', 'roles', 'builder.yaml');

  // R4: a sentence about the document as a whole marks the first line of the
  // preview alone, not every heading and blank line. The screen's own script
  // puts the mark on a line, so the browser is the only place it is seen.
  await page.goto(`${app.base}/roles?open=analyst`);
  await wait(page, there('#rows .role.open .panel'));
  await readTo<number>(
    page,
    `document.querySelectorAll('#rows .role.open .yprev .yl.warn').length`,
    "analyst's preview marks lines as a warning, want the first line alone",
  ).toBe(1);

  // P4・C4: the text a person types is what the save writes, and the save
  // changes the rule's text line alone.
  const before = readFileSync(builder, 'utf8');
  const want = 'E2E が書き換えた本文。';
  await page.goto(`${app.base}/roles?open=builder`);
  await wait(page, there('#rows .role.open .panel .rule'));
  await press(page, `${rule('BUILD-5')}.querySelector('.rsum')`);
  await wait(page, there('#rows .role.open .rule.open .redit [data-f=text]'));
  await set(page, q('#rows .role.open .rule.open .redit [data-f=text]'), want);
  await press(page, q('#rows .role.open .pf [data-act=save]'));
  await wait(page, there('#rows .role.open .ok'));
  const changed = changedLines(before, readFileSync(builder, 'utf8'));
  expect(
    changed.size,
    `the save changed lines, want the rule's text line alone: ${JSON.stringify([...changed])}`,
  ).toBe(1);
  for (const [line, text] of changed) {
    expect(text, `line ${line} became, want the rule's new text`).toBe(`        text: ${want}`);
  }
});

test('ROLES-P14 the person whose role is swapped', async ({ page, app }) => {
  // P14・P6: one person's role is picked in its card, and the save writes that
  // one line of team.yaml.
  const file = path.join(app.proj, '.hergy', 'teams', 'research', 'team.yaml');
  const before = readFileSync(file, 'utf8');

  await page.goto(`${app.base}/roles/assign`);
  await wait(page, there('.team[data-t=research]'));
  await choose(page, q('.team[data-t=research] .arow[data-m=scout-2] select'), 'writer');
  await wait(page, there('.team[data-t=research] [data-act=t-save]'));
  await press(page, q('.team[data-t=research] [data-act=t-save]'));
  await wait(page, there('.team[data-t=research] .ok'));

  const changed = changedLines(before, readFileSync(file, 'utf8'));
  expect(
    changed.size,
    `the assign save changed lines, want scout-2's role alone: ${JSON.stringify([...changed])}`,
  ).toBe(1);
  for (const [line, text] of changed) {
    expect(text, `line ${line} became, want scout-2's new role`).toBe('    role: writer');
  }
});
