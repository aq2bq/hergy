// r-df61 / d-b023 — the 動き column's start button says what it does, and the
// sheet it opens takes the team's name (empty for the definition's name). The
// name is the server's own check (hergy up --as), and its refusal stands beside
// the name's field.
//
// The headquarters fixture (testdata/hq) holds alpha's own definition
// (alpha-dev) and the global one (shared); the fake herdr gives the start a
// workspace and a tab, so a start is a real one: the launch record and the row's
// team name are read back (fixture.mjs).
import { test, expect } from './fixtures';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { press, q, readTo, set, there, wait } from './helpers';

test.use({ reads: 'hq' });

// rowOf is one definition's row, by the name the list shows.
const rowOf = (name: string) =>
  `[...document.querySelectorAll('#view .tdef')].find(function(r){var n=r.querySelector('.c-name .nm');return !!n&&n.textContent.trim()===${JSON.stringify(name)}})`;

// startOf is one row's start button in the 動き column.
const startOf = (name: string) => `(${rowOf(name)}).querySelector('.hq-runc [data-hq=start]')`;

const SHEET = q('.hq-sheet');
const title = `${SHEET}.querySelector('.hq-h b').textContent`;
const nameField = `${SHEET}.querySelector('#hq-as')`;

// chooseNewWorkspace picks 新しく作る, the one workspace a start can make when
// the fake herdr lists none for the sheet.
const chooseNewWorkspace = (page: Parameters<typeof press>[0]) =>
  press(page, `${SHEET}.querySelector('input[name=hq-ws][value="__new"]')`);

// recordText is the launch record this test's serve wrote.
const recordText = (state: string): string => {
  try {
    return readFileSync(path.join(state, 'hergy', 'launches', 'log.jsonl'), 'utf8');
  } catch {
    return '';
  }
};

test('the start button names its act, and the sheet always offers the team name', async ({ page, app }) => {
  await page.goto(`${app.base}/teams`);
  await wait(page, there('#view .tdef'));
  await readTo<string>(page, `${startOf('alpha-dev')}.textContent`, 'the project definition starts this project').toBe('このプロジェクトで起動する');
  await readTo<string>(page, `${startOf('shared')}.textContent`, 'the global definition asks for the project').toBe('プロジェクトを選んで起動する');

  await press(page, startOf('alpha-dev'));
  await wait(page, there('#hq-as'));
  await readTo<string>(page, title, 'the project start names the project').toBe('このプロジェクトで起動する');
  // The sheet is where a person picks the definition to start, so it carries
  // the definition's description (d-5f55).
  await readTo<string>(page, `${SHEET}.querySelector('[data-role=description]').textContent`, 'the start sheet does not carry the description').toBe('注文の流れを作り直す');
  await readTo<string>(page, `${nameField}.value`, 'the team name starts empty').toBe('');
  await readTo<string>(page, `${SHEET}.querySelector('[data-role=same]').textContent`, 'the sentence is the hergy up it stands for').toContain('hergy up alpha-dev');
  await press(page, `${SHEET}.querySelector('[data-hqx=close]')`);

  await press(page, startOf('shared'));
  await wait(page, there('#hq-as'));
  await readTo<string>(page, title, 'the global start asks for the project').toBe('プロジェクトを選んで起動する');
  await readTo<string>(page, `${nameField}.value`, 'the global team name starts empty').toBe('');
});

test('a written team name starts the team under that name, in the record and the 動き column', async ({ page, app }) => {
  await page.goto(`${app.base}/teams`);
  await wait(page, there('#view .tdef'));
  await press(page, startOf('alpha-dev'));
  await wait(page, there('#hq-as'));
  await set(page, nameField, 'dev2');
  await readTo<string>(page, `${SHEET}.querySelector('[data-role=same]').textContent`, 'the sentence carries --as').toContain('hergy up alpha-dev --as dev2');
  await chooseNewWorkspace(page);
  await press(page, `${SHEET}.querySelector('[data-hq=go]')`);
  await readTo<string>(page, title, 'the start ends').toBe('起動しました');

  await expect.poll(() => recordText(app.state), { timeout: 8_000, message: 'the launch record names the team' }).toContain('"team":"dev2"');

  await press(page, `${SHEET}.querySelector('[data-hqx=close]')`);
  await readTo<string>(page, `(${rowOf('alpha-dev')}).querySelector('[data-role=team-name]').textContent`, 'the row shows the team name it was started under').toBe('dev2');
});

test('an empty team name starts the team under the definition name', async ({ page, app }) => {
  await page.goto(`${app.base}/teams`);
  await wait(page, there('#view .tdef'));
  await press(page, startOf('alpha-dev'));
  await wait(page, there('#hq-as'));
  await readTo<string>(page, `${nameField}.value`, 'the team name is empty').toBe('');
  await chooseNewWorkspace(page);
  await press(page, `${SHEET}.querySelector('[data-hq=go]')`);
  await readTo<string>(page, title, 'the start ends').toBe('起動しました');

  await expect.poll(() => recordText(app.state), { timeout: 8_000, message: 'the launch record names the definition' }).toContain('"team":"alpha-dev"');
});

test('a name the server refuses stays beside the name field, and nothing starts', async ({ page, app }) => {
  // A live agent named taken-lead is a running team named taken, so the
  // server's own hergy up --as check refuses that name too.
  const herdr = path.join(app.work, 'herdr');
  mkdirSync(herdr, { recursive: true });
  writeFileSync(path.join(herdr, 'agents.json'), JSON.stringify({
    result: { agents: [{ name: 'taken-lead', pane_id: 'p1', tab_id: 't1' }] },
  }));

  await page.goto(`${app.base}/teams`);
  await wait(page, there('#view .tdef'));
  await press(page, startOf('alpha-dev'));
  await wait(page, there('#hq-as'));
  await chooseNewWorkspace(page);

  await set(page, nameField, 'Dev 2');
  await press(page, `${SHEET}.querySelector('[data-hq=go]')`);
  await readTo<string>(page, `${SHEET}.querySelector('[data-role=name-error]').textContent`, 'the unreadable name says why beside the field').toContain('チームの名前は');
  await readTo<string>(page, `${SHEET}.querySelector('[data-role=error]').textContent`, 'the sheet keeps no other refusal').toBe('');

  await set(page, nameField, 'taken');
  await press(page, `${SHEET}.querySelector('[data-hq=go]')`);
  await readTo<string>(page, `${SHEET}.querySelector('[data-role=name-error]').textContent`, 'the running name says why beside the field').toContain('taken');
  await readTo<string>(page, title, 'the sheet is still the start').toBe('このプロジェクトで起動する');
});
