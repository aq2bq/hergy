// ROLES-P5 and ROLES-P14 — walked in a browser. Only
// what the browser alone can see is compared here (d-3e43): the document-wide
// warning the screen's own script puts on the preview's first line (R4), the
// text a person types being the one line a save writes, and the role a person
// picks being the one line a save writes.
import { test, expect } from './fixtures';
import { readFileSync, writeFileSync } from 'node:fs';
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
  // puts the mark on a line, so the browser is the only place it is seen. The
  // document-wide sentence is the size warning now (d-00a9): a role document
  // over the provisional budget gets one, on the document as a whole.
  const analyst = path.join(app.proj, '.hergy', 'roles', 'analyst.yaml');
  writeFileSync(
    analyst,
    readFileSync(analyst, 'utf8') +
      '\n  - heading: 長さ\n    rules:\n      - id: LONG-1\n        text: ' +
      'あ'.repeat(3100) +
      '\n',
  );
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

test('ROLES-R1 the switch, the day a rule went off, and the memo', async ({ page, app }) => {
  // d-85dc: the row's switch turns a rule off and on without opening it; hergy
  // writes the day it went off on the save and drops it when the rule comes
  // back. The opened rule holds ID, text and the memo alone.
  const file = path.join(app.proj, '.hergy', 'roles', 'lead.yaml');
  await page.goto(`${app.base}/roles?open=lead`);
  await wait(page, there('#rows .role.open .panel .rule'));
  const head = await page.$eval('.list .head', (el) => el.textContent || '');
  expect(head, 'the list column names the switch').toContain('オン / オフ');

  // The opened rule: ID, 本文 and メモ only.
  await press(page, `${rule('LEAD-1')}.querySelector('.rsum')`);
  await wait(page, there('#rows .role.open .rule.open .redit'));
  const labels = await page.$$eval('#rows .role.open .rule.open .redit .field > label, #rows .role.open .rule.open .redit .field > .fl', (els) => els.map((e) => (e.textContent || '').trim()));
  expect(labels, 'the opened rule holds ID, 本文 and メモ alone').toEqual(['ID', '本文', 'メモ']);
  const noteTag = await page.$eval('#rows .role.open .rule.open .redit [data-f=note]', (el) => el.tagName);
  expect(noteTag, 'the memo is a textarea that grows like the body').toBe('TEXTAREA');

  // The memo is written as it is.
  const memo = 'E2E のメモ。';
  await set(page, q('#rows .role.open .rule.open .redit [data-f=note]'), memo);

  // The switch goes off without opening the row; the day is not the screen's
  // to make before the save.
  await press(page, `${rule('LEAD-1')}.querySelector('.onsw')`);
  await wait(page, `${rule('LEAD-1')}.querySelector('.onsw').getAttribute('aria-checked')==='false'`);
  await readTo<string>(page, `${rule('LEAD-1')}.querySelector('.since').textContent`, 'the off day before the save').toBe('保存すると今日から');

  // The filter of the list keeps the roles that have an off rule.
  await press(page, q('[data-act=filter][data-f=off]'));
  await wait(page, `!document.querySelector('#rows .role').hidden`);
  await readTo<string>(page, `document.querySelector('#rows .role .c-name .nm').textContent`, 'the off filter keeps lead').toBe('lead');
  await press(page, q('[data-act=filter][data-f=""]'));

  await press(page, q('#rows .role.open .pf [data-act=save]'));
  await wait(page, there('#rows .role.open .ok'));
  let text = readFileSync(file, 'utf8');
  const block = text.slice(text.indexOf('id: LEAD-1'), text.indexOf('id: LEAD-2'));
  expect(block, 'the save wrote in_prompt: false').toContain('in_prompt: false');
  expect(block, 'the save wrote the day it went off').toMatch(/off_since: "\d{4}-\d{2}-\d{2}"/);
  expect(block, 'the save wrote the memo').toContain(`note: ${memo}`);
  const ruleCount = await page.$eval('#rows .role.open .c-count', (el) => el.textContent || '');
  expect(ruleCount, 'the rules column does not repeat the off count').not.toContain('オフ');
  const day = /off_since: "(\d{4})-(\d{2})-(\d{2})"/.exec(block);
  await readTo<string>(page, `${rule('LEAD-1')}.querySelector('.since').textContent`, 'the off day after the save').toBe(`${Number(day![2])}/${Number(day![3])} から`);

  // Back on: the save drops the day.
  await press(page, `${rule('LEAD-1')}.querySelector('.onsw')`);
  await wait(page, `${rule('LEAD-1')}.querySelector('.onsw').getAttribute('aria-checked')==='true'`);
  await press(page, q('#rows .role.open .pf [data-act=save]'));
  await wait(page, there('#rows .role.open .ok'));
  text = readFileSync(file, 'utf8');
  const after = text.slice(text.indexOf('id: LEAD-1'), text.indexOf('id: LEAD-2'));
  expect(after, 'the on save dropped the day').not.toContain('off_since');
});
