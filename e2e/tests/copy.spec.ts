// COPY-P1 .. COPY-P6 — r-f781 (ac-5a1e): copying a role, a profile or a team
// definition between a project and the global layer draws the copy on the side
// it was copied to, leaves the side it came from as it was, and writes only the
// destination's file. Each walk reads the two sides' counts before the copy,
// presses the copy where the source stands, saves it, and reads the counts and
// the files again after opening the page once more.
//
// The walks stand on the headquarters fixture (testdata/hq): alpha holds one
// role, one profile and one definition; the global layer holds two roles, two
// profiles and one definition.
import { test, expect, type Page } from './fixtures';
import { existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { press, read, readTo, run, set, there, viewport, wait, ready } from './helpers';

test.use({ reads: 'hq' });

const ROLES = '#rows .role';
const PROFILES = '#rows .row';
// The teams list has no #rows: its rows and heads stand directly under .list.
const TEAMS = '#view .list .tdef';
const TEAMS_LIST = '#view .list';
const ROWS = '#rows';
const ALPHA = 'alpha';
const GLOBAL = 'グローバル';

// rowsIn is the rows one side's head stands over: the list draws a head and the
// rows under it until the next head.
const rowsIn = (root: string, sel: string, group: string) => `(() => {
  const out = [];
  let on = null;
  const list = document.querySelector(${JSON.stringify(root)});
  if (!list) return out;
  for (const el of list.children) {
    if (el.classList.contains('hq-gh')) {
      const n = el.querySelector('.hq-gn');
      on = n ? n.textContent.trim() : '';
      continue;
    }
    if (on === ${JSON.stringify(group)} && el.matches(${JSON.stringify(sel)})) out.push(el);
  }
  return out;
})()`;

const rootOf = (sel: string) => (sel === TEAMS ? TEAMS_LIST : ROWS);
const countIn = (sel: string, group: string) => `${rowsIn(rootOf(sel), sel, group)}.length`;

// rowIn is one row under one place's head, by the name it carries: a saved row
// shows the name, and a row a copy made shows it in the name field it opens
// with (r-edbb).
const rowIn = (sel: string, group: string, name: string) => `(${rowsIn(rootOf(sel), sel, group)}.find((el) => {
  const n = el.querySelector('.c-name .nm');
  if (n && n.textContent.trim() === ${JSON.stringify(name)}) return true;
  const f = el.querySelector('.c-name .in-name');
  return !!f && f.value === ${JSON.stringify(name)};
}) || null)`;

// headIn is one place's head, by the name it stands under.
const headIn = (sel: string, group: string) => `([...document.querySelectorAll(${JSON.stringify(rootOf(sel))} + ' .hq-gh')].find((el) => {
  const n = el.querySelector('.hq-gn');
  return !!n && n.textContent.trim() === ${JSON.stringify(group)};
}) || null)`;

// headOffScreen is whether one head stands outside the window: the place a copy
// goes to is one the person cannot see when they press the copy (ac-fe3e).
const headOffScreen = (sel: string, group: string) => `(() => {
  const h = ${headIn(sel, group)};
  const r = h.getBoundingClientRect();
  return r.bottom <= 0 || r.top >= innerHeight;
})()`;

// nameChosen is whether one row's name field holds the cursor with all of the
// name chosen: the row a copy made is ready to be typed over (d-022a).
const nameChosen = (row: string) => `(() => {
  const one = ${row};
  const f = one.querySelector('.in-name[data-f=name]') || one.querySelector('.in-name');
  return !!f && document.activeElement === f && f.selectionStart === 0 && f.selectionEnd === f.value.length && f.value.length > 0;
})()`;

// open presses one row's summary, which is where its panels and their コピー /
// 保存 buttons stand.
async function open(page: Page, sel: string, group: string, name: string): Promise<void> {
  const row = rowIn(sel, group, name);
  await press(page, `${row}.querySelector('.sum')`);
  await wait(page, `!!${row} && ${row}.classList.contains('open')`);
}

// copyAndSave presses one panel's copy, waits for the copy on the destination
// side, and presses its save.
async function copyAndSave(
  page: Page,
  sel: string,
  src: string,
  dst: string,
  name: string,
  copy: string,
  save: string,
): Promise<void> {
  await open(page, sel, src, name);
  await press(page, copy);
  const copyRow = rowIn(sel, dst, name);
  await wait(page, `!!${copyRow}`);
  await press(page, save);
  await wait(page, `!!${copyRow} && !!${copyRow}.querySelector('.ok')`);
}

// sameAfterReload opens the page again and reads the two sides' counts: the
// copy is a file, so the reading after the page is opened again is the same.
async function countsAfterReload(
  page: Page,
  sel: string,
  src: string,
  dst: string,
  srcCount: number,
  dstCount: number,
): Promise<void> {
  await page.reload();
  await wait(page, there(sel));
  await readTo<number>(page, countIn(sel, dst), 'the copy stands on its side after the page is opened again').toBe(dstCount);
  await readTo<number>(page, countIn(sel, src), 'the side the copy came from holds its own rows after the page is opened again').toBe(srcCount);
}

test('COPY-P1 a role copied from the project to the global layer', async ({ page, app }) => {
  const srcFile = path.join(app.proj, '.hergy', 'roles', 'builder.md');
  const dstFile = path.join(app.cfg, 'hergy', 'roles', 'builder.md');
  const before = readFileSync(srcFile, 'utf8');

  await page.goto(`${app.base}/roles`);
  await wait(page, there(ROLES));
  const srcCount = await read<number>(page, countIn(ROLES, ALPHA));
  const dstCount = await read<number>(page, countIn(ROLES, GLOBAL));
  expect(existsSync(dstFile), 'the global layer holds no builder before the copy').toBe(false);

  await copyAndSave(
    page, ROLES, ALPHA, GLOBAL, 'builder',
    `${rowIn(ROLES, ALPHA, 'builder')}.querySelector('.panel[data-l=project] [data-act=copy]')`,
    `${rowIn(ROLES, GLOBAL, 'builder')}.querySelector('.panel[data-l=global] [data-act=save]')`,
  );

  await readTo<number>(page, countIn(ROLES, GLOBAL), 'the copy stands under the global layer').toBe(dstCount + 1);
  await readTo<number>(page, countIn(ROLES, ALPHA), 'the project holds the roles it had').toBe(srcCount);
  expect(readFileSync(dstFile, 'utf8'), 'the copy wrote the source document on the global side').toBe(before);
  expect(readFileSync(srcFile, 'utf8'), 'copying left the project file alone').toBe(before);
  await countsAfterReload(page, ROLES, ALPHA, GLOBAL, srcCount, dstCount + 1);
  expect(readFileSync(dstFile, 'utf8'), 'the global file is still the copy after opening the page again').toBe(before);
});

test('COPY-P2 a role copied from the global layer to the project', async ({ page, app }) => {
  const srcFile = path.join(app.cfg, 'hergy', 'roles', 'lead.md');
  const dstFile = path.join(app.proj, '.hergy', 'roles', 'lead.md');
  const before = readFileSync(srcFile, 'utf8');

  await page.goto(`${app.base}/roles`);
  await wait(page, there(ROLES));
  const srcCount = await read<number>(page, countIn(ROLES, GLOBAL));
  const dstCount = await read<number>(page, countIn(ROLES, ALPHA));
  expect(existsSync(dstFile), 'the project holds no lead before the copy').toBe(false);

  await copyAndSave(
    page, ROLES, GLOBAL, ALPHA, 'lead',
    `${rowIn(ROLES, GLOBAL, 'lead')}.querySelector('.panel[data-l=global] [data-act=copy]')`,
    `${rowIn(ROLES, ALPHA, 'lead')}.querySelector('.panel[data-l=project] [data-act=save]')`,
  );

  await readTo<number>(page, countIn(ROLES, ALPHA), 'the copy stands under the project').toBe(dstCount + 1);
  await readTo<number>(page, countIn(ROLES, GLOBAL), 'the global layer holds the roles it had').toBe(srcCount);
  expect(readFileSync(dstFile, 'utf8'), 'the copy wrote the source document on the project side').toBe(before);
  expect(readFileSync(srcFile, 'utf8'), 'copying left the global file alone').toBe(before);
  await countsAfterReload(page, ROLES, GLOBAL, ALPHA, srcCount, dstCount + 1);
  expect(readFileSync(dstFile, 'utf8'), 'the project file is still the copy after opening the page again').toBe(before);
});

test('COPY-P3 a profile copied from the project to the global layer', async ({ page, app }) => {
  const srcFile = path.join(app.proj, '.hergy', 'profiles.yaml');
  const dstFile = path.join(app.cfg, 'hergy', 'profiles.yaml');
  const before = readFileSync(srcFile, 'utf8');

  await page.goto(`${app.base}/profiles`);
  await wait(page, there(PROFILES));
  const srcCount = await read<number>(page, countIn(PROFILES, ALPHA));
  const dstCount = await read<number>(page, countIn(PROFILES, GLOBAL));
  expect(readFileSync(dstFile, 'utf8'), 'the global layer holds no alpha-fast before the copy').not.toContain('alpha-fast');

  await copyAndSave(
    page, PROFILES, ALPHA, GLOBAL, 'alpha-fast',
    `${rowIn(PROFILES, ALPHA, 'alpha-fast')}.querySelector('.panel[data-file=project] [data-act=copy]')`,
    `${rowIn(PROFILES, GLOBAL, 'alpha-fast')}.querySelector('.panel[data-file=global] [data-act=save]')`,
  );

  await readTo<number>(page, countIn(PROFILES, GLOBAL), 'the copy stands under the global layer').toBe(dstCount + 1);
  await readTo<number>(page, countIn(PROFILES, ALPHA), 'the project holds the profiles it had').toBe(srcCount);
  expect(readFileSync(dstFile, 'utf8'), 'the global file holds the copied profile').toContain('alpha-fast');
  expect(readFileSync(srcFile, 'utf8'), 'copying left the project file alone').toBe(before);
  await countsAfterReload(page, PROFILES, ALPHA, GLOBAL, srcCount, dstCount + 1);
  expect(readFileSync(dstFile, 'utf8'), 'the global file still holds the copy after opening the page again').toContain('alpha-fast');
});

test('COPY-P4 a profile copied from the global layer to the project', async ({ page, app }) => {
  const srcFile = path.join(app.cfg, 'hergy', 'profiles.yaml');
  const dstFile = path.join(app.proj, '.hergy', 'profiles.yaml');
  const before = readFileSync(srcFile, 'utf8');

  await page.goto(`${app.base}/profiles`);
  await wait(page, there(PROFILES));
  const srcCount = await read<number>(page, countIn(PROFILES, GLOBAL));
  const dstCount = await read<number>(page, countIn(PROFILES, ALPHA));
  expect(readFileSync(dstFile, 'utf8'), 'the project holds no planner before the copy').not.toContain('planner');

  await copyAndSave(
    page, PROFILES, GLOBAL, ALPHA, 'planner',
    `${rowIn(PROFILES, GLOBAL, 'planner')}.querySelector('.panel[data-file=global] [data-act=copy]')`,
    `${rowIn(PROFILES, ALPHA, 'planner')}.querySelector('.panel[data-file=project] [data-act=save]')`,
  );

  await readTo<number>(page, countIn(PROFILES, ALPHA), 'the copy stands under the project').toBe(dstCount + 1);
  await readTo<number>(page, countIn(PROFILES, GLOBAL), 'the global layer holds the profiles it had').toBe(srcCount);
  expect(readFileSync(dstFile, 'utf8'), 'the project file holds the copied profile').toContain('planner');
  expect(readFileSync(srcFile, 'utf8'), 'copying left the global file alone').toBe(before);
  await countsAfterReload(page, PROFILES, GLOBAL, ALPHA, srcCount, dstCount + 1);
  expect(readFileSync(dstFile, 'utf8'), 'the project file still holds the copy after opening the page again').toContain('planner');
});

test('COPY-P5 a team definition copied from the project to the global layer', async ({ page, app }) => {
  const srcFile = path.join(app.proj, '.hergy', 'teams', 'alpha-dev', 'team.yaml');
  const dstFile = path.join(app.cfg, 'hergy', 'teams', 'alpha-dev', 'team.yaml');
  const before = readFileSync(srcFile, 'utf8');

  await page.goto(`${app.base}/teams`);
  await wait(page, there(TEAMS));
  const srcCount = await read<number>(page, countIn(TEAMS, ALPHA));
  const dstCount = await read<number>(page, countIn(TEAMS, GLOBAL));
  expect(existsSync(dstFile), 'the global layer holds no alpha-dev before the copy').toBe(false);

  await copyAndSave(
    page, TEAMS, ALPHA, GLOBAL, 'alpha-dev',
    `${rowIn(TEAMS, ALPHA, 'alpha-dev')}.querySelector('[data-act=copy]')`,
    `${rowIn(TEAMS, GLOBAL, 'alpha-dev')}.querySelector('.detail [data-act=t-save]')`,
  );

  await readTo<number>(page, countIn(TEAMS, GLOBAL), 'the copy stands under the global layer').toBe(dstCount + 1);
  await readTo<number>(page, countIn(TEAMS, ALPHA), 'the project holds the definitions it had').toBe(srcCount);
  expect(existsSync(dstFile), 'the copy made the definition on the global side').toBe(true);
  expect(readFileSync(dstFile, 'utf8'), 'the global team.yaml holds the copied description').toContain('注文の流れを作り直す');
  expect(readFileSync(srcFile, 'utf8'), 'copying left the project file alone').toBe(before);
  await countsAfterReload(page, TEAMS, ALPHA, GLOBAL, srcCount, dstCount + 1);
  expect(readFileSync(dstFile, 'utf8'), 'the global file is still the copy after opening the page again').toContain('注文の流れを作り直す');
});

test('COPY-P6 a team definition copied from the global layer to the project', async ({ page, app }) => {
  const srcFile = path.join(app.cfg, 'hergy', 'teams', 'shared', 'team.yaml');
  const dstFile = path.join(app.proj, '.hergy', 'teams', 'shared', 'team.yaml');
  const before = readFileSync(srcFile, 'utf8');

  await page.goto(`${app.base}/teams`);
  await wait(page, there(TEAMS));
  const srcCount = await read<number>(page, countIn(TEAMS, GLOBAL));
  const dstCount = await read<number>(page, countIn(TEAMS, ALPHA));
  expect(existsSync(dstFile), 'the project holds no shared before the copy').toBe(false);

  await copyAndSave(
    page, TEAMS, GLOBAL, ALPHA, 'shared',
    `${rowIn(TEAMS, GLOBAL, 'shared')}.querySelector('[data-act=copy]')`,
    `${rowIn(TEAMS, ALPHA, 'shared')}.querySelector('.detail [data-act=t-save]')`,
  );

  await readTo<number>(page, countIn(TEAMS, ALPHA), 'the copy stands under the project').toBe(dstCount + 1);
  await readTo<number>(page, countIn(TEAMS, GLOBAL), 'the global layer holds the definitions it had').toBe(srcCount);
  expect(existsSync(dstFile), 'the copy made the definition on the project side').toBe(true);
  expect(readFileSync(dstFile, 'utf8'), 'the project team.yaml holds the copied description').toContain('困りごとの相談');
  expect(readFileSync(srcFile, 'utf8'), 'copying left the global file alone').toBe(before);
  await countsAfterReload(page, TEAMS, GLOBAL, ALPHA, srcCount, dstCount + 1);
  expect(readFileSync(dstFile, 'utf8'), 'the project file is still the copy after opening the page again').toContain('困りごとの相談');
});

// ---- COPY-V1 .. COPY-V6 — r-edbb (ac-fe3e) ------------------------------
//
// Copying a role, a profile or a definition between the places closes the row
// the person pressed, opens the copy under the other place's head, brings the
// copy into the window and puts the cursor in its name with all of it chosen.
//
// Each walk stands on the headquarters sample with rows filled into the
// project's own place, and reads the screen at a window shorter than the rows a
// copy has to pass: the place a copy goes to stands outside the window when the
// copy is pressed (ac-fe3e's 測り方). The direction the copy goes decides how:
// copied to the global layer, the global head is below the window; copied to
// the project, the person has scrolled down to the source row and the project's
// head is above it.

// fillRows writes <n> more documents into the project's own place, so the head
// below it stands outside a short window.
function fillRows(app: { proj: string }, screen: 'roles' | 'profiles' | 'teams', n: number): void {
  const hergy = path.join(app.proj, '.hergy');
  if (screen === 'roles') {
    mkdirSync(path.join(hergy, 'roles'), { recursive: true });
    for (let i = 1; i <= n; i++) {
      writeFileSync(
        path.join(hergy, 'roles', `filler-${i}.md`),
        `# 埋め草 ${i}\n\n## 進め方\n${i} 番目の行を書く。\n`,
      );
    }
    return;
  }
  if (screen === 'profiles') {
    const file = path.join(hergy, 'profiles.yaml');
    let text = readFileSync(file, 'utf8');
    if (!text.endsWith('\n')) text += '\n';
    for (let i = 1; i <= n; i++) text += `  filler-${i}:\n    kind: claude\n    args: ["--model", "claude-haiku-5-5"]\n`;
    writeFileSync(file, text);
    return;
  }
  const one = readFileSync(path.join(hergy, 'teams', 'alpha-dev', 'team.yaml'), 'utf8');
  for (let i = 1; i <= n; i++) {
    const dir = path.join(hergy, 'teams', `filler-${i}`);
    mkdirSync(dir, { recursive: true });
    writeFileSync(path.join(dir, 'team.yaml'), one);
  }
}

// copyButton is the copy one open row offers, named the way each screen draws
// it.
const copyButton = (sel: string, group: string, name: string, from: string): string => {
  const row = rowIn(sel, group, name);
  if (sel === TEAMS) return `${row}.querySelector('[data-act=copy]')`;
  if (sel === PROFILES) return `${row}.querySelector('.panel[data-file=${from}] [data-act=copy]')`;
  return `${row}.querySelector('.panel[data-l=${from}] [data-act=copy]')`;
};

// copyShows is what every walk reads after the press: the copy is open under
// the head of the place it was copied to, its top stands inside the window, its
// name field holds the cursor with the whole name chosen, and the row the
// person pressed is closed.
async function copyShows(
  page: Page,
  sel: string,
  src: string,
  dst: string,
  name: string,
): Promise<void> {
  const copy = rowIn(sel, dst, name);
  const was = rowIn(sel, src, name);
  await readTo<boolean>(page, `${copy}.classList.contains('open')`, 'the copy stands open under the head it was copied to').toBe(true);
  await readTo<boolean>(page, `${copy}.getBoundingClientRect().top >= 0 && ${copy}.getBoundingClientRect().top < innerHeight`, 'the top of the copy stands inside the window').toBe(true);
  await readTo<boolean>(page, nameChosen(copy), 'the name field of the copy holds the cursor with the whole name chosen').toBe(true);
  await readTo<boolean>(page, `!${was}.classList.contains('open')`, 'the row the person pressed is closed').toBe(true);
}

test('COPY-V1 a role copied from the project to the global layer comes into the window', async ({ page, app }) => {
  await viewport(page, 2000, 480);
  fillRows(app, 'roles', 24);
  await page.goto(`${app.base}/roles`);
  await wait(page, there(ROLES));
  await wait(page, headOffScreen(ROLES, GLOBAL));
  await open(page, ROLES, ALPHA, 'builder');
  await press(page, copyButton(ROLES, ALPHA, 'builder', 'project'));
  await copyShows(page, ROLES, ALPHA, GLOBAL, 'builder');
});

test('COPY-V2 a role copied from the global layer to the project comes into the window', async ({ page, app }) => {
  await viewport(page, 2000, 480);
  fillRows(app, 'roles', 24);
  await page.goto(`${app.base}/roles`);
  await wait(page, there(ROLES));
  // The person scrolls down to the global layer's row, so the project's head is
  // above the window when the copy is pressed.
  await run(page, `const one = ${rowIn(ROLES, GLOBAL, 'lead')}; if (one) one.scrollIntoView({ block: 'center' });`);
  await wait(page, headOffScreen(ROLES, ALPHA));
  await open(page, ROLES, GLOBAL, 'lead');
  await press(page, copyButton(ROLES, GLOBAL, 'lead', 'global'));
  await copyShows(page, ROLES, GLOBAL, ALPHA, 'lead');
});

test('COPY-V3 a profile copied from the project to the global layer comes into the window', async ({ page, app }) => {
  await viewport(page, 2000, 480);
  fillRows(app, 'profiles', 24);
  await page.goto(`${app.base}/profiles`);
  await wait(page, there(PROFILES));
  await wait(page, headOffScreen(PROFILES, GLOBAL));
  await open(page, PROFILES, ALPHA, 'alpha-fast');
  await press(page, copyButton(PROFILES, ALPHA, 'alpha-fast', 'project'));
  await copyShows(page, PROFILES, ALPHA, GLOBAL, 'alpha-fast');
});

test('COPY-V4 a profile copied from the global layer to the project comes into the window', async ({ page, app }) => {
  await viewport(page, 2000, 480);
  fillRows(app, 'profiles', 24);
  await page.goto(`${app.base}/profiles`);
  await wait(page, there(PROFILES));
  await run(page, `const one = ${rowIn(PROFILES, GLOBAL, 'planner')}; if (one) one.scrollIntoView({ block: 'center' });`);
  await wait(page, headOffScreen(PROFILES, ALPHA));
  await open(page, PROFILES, GLOBAL, 'planner');
  await press(page, copyButton(PROFILES, GLOBAL, 'planner', 'global'));
  await copyShows(page, PROFILES, GLOBAL, ALPHA, 'planner');
});

test('COPY-V5 a team definition copied from the project to the global layer comes into the window', async ({ page, app }) => {
  await viewport(page, 2000, 480);
  fillRows(app, 'teams', 24);
  await page.goto(`${app.base}/teams`);
  await wait(page, there(TEAMS));
  await wait(page, headOffScreen(TEAMS, GLOBAL));
  await open(page, TEAMS, ALPHA, 'alpha-dev');
  await press(page, copyButton(TEAMS, ALPHA, 'alpha-dev', 'project'));
  await copyShows(page, TEAMS, ALPHA, GLOBAL, 'alpha-dev');
});

test('COPY-V6 a team definition copied from the global layer to the project comes into the window', async ({ page, app }) => {
  await viewport(page, 2000, 480);
  fillRows(app, 'teams', 24);
  await page.goto(`${app.base}/teams`);
  await wait(page, there(TEAMS));
  await run(page, `const one = ${rowIn(TEAMS, GLOBAL, 'shared')}; if (one) one.scrollIntoView({ block: 'center' });`);
  await wait(page, headOffScreen(TEAMS, ALPHA));
  await open(page, TEAMS, GLOBAL, 'shared');
  await press(page, copyButton(TEAMS, GLOBAL, 'shared', 'global'));
  await copyShows(page, TEAMS, GLOBAL, ALPHA, 'shared');
});

// ---- COPY-S1 .. COPY-S7 — r-a028 (ac-f07b, ac-456e) ----------------------
//
// A definition that stands under a listed project's head is that project's own
// file. Copying one of those rows into the global layer used to read the file of
// the project serve was started in: the copy had no base, so its 保存 was shut
// for good and the screen said nothing about it. Each walk below presses the copy
// under beta's head — a project serve was not started in — and reads what the
// global layer got.
//
// A copy the base of which cannot be read at all is the one state left; there the
// save stays shut and the foot must say what happened and what to do (ac-456e).
const BETA = 'beta';

// betaFile is one document under the listed project's head.
const betaFile = (app: { home: string }, rel: string) => path.join(app.home, 'src', BETA, '.hergy', rel);

// alphaFile is the same under the project serve was started in.
const alphaFile = (app: { proj: string }, rel: string) => path.join(app.proj, '.hergy', rel);

// saveButton is the 保存 of the copy that stands under the global head.
const saveOfCopy = (name: string) => `${rowIn(TEAMS, GLOBAL, name)}.querySelector('.detail [data-act=t-save]')`;

// whyOfCopy is what the copy's foot says about a save it does not offer.
const whyOfCopy = (name: string) => `(${rowIn(TEAMS, GLOBAL, name)}.querySelector('.detail .pf .refusal') || {}).textContent || ''`;

// ---- the reason a save is shut, read at the windows a person reads it in ----
//
// The foot says why the save is shut, and the sentence carries the way to fix
// it: it must be readable to its last character at a wide window and at a narrow
// one, and the page must not scroll sideways to show it (r-a028, ac-456e).

// whyIn is the sentence one row's foot says about the save it does not offer.
const whyIn = (row: string) => `(${row}.querySelector('.pf .refusal') || null)`;

// across is how far the page scrolls sideways; 0 is a page a person reads
// without pushing it left and right.
const across = () => '(() => { const d = document.documentElement; return Math.max(d.scrollWidth, document.body.scrollWidth) - d.clientWidth; })()';

// lastText is the last run of text one sentence holds: the sentence may stand in
// a span inside the element, and the last character of the way to fix it is the
// end of that run.
const lastText = (el: string) => `(() => {
  const w = ${el};
  if (!w) return null;
  const walk = document.createTreeWalker(w, NodeFilter.SHOW_TEXT);
  let node = null;
  while (walk.nextNode()) { if (walk.currentNode.length) node = walk.currentNode; }
  return node; })()`;

// whyWhole is whether the whole sentence stands inside the window.
const whyWhole = (el: string) => `(() => { const w = ${el}; if (!w) return false; w.scrollIntoView({ block: 'center' });
  const r = w.getBoundingClientRect();
  return r.left >= -0.5 && r.right <= innerWidth + 0.5 && r.top >= -0.5 && r.bottom <= innerHeight + 0.5; })()`;

// whyLast is whether the sentence's last character stands inside the window: the
// way to fix it is the end of the sentence.
const whyLast = (el: string) => `(() => { const w = ${el}; if (!w) return false; w.scrollIntoView({ block: 'center' });
  const n = ${lastText(el)};
  if (!n) return false;
  const g = document.createRange(); g.setStart(n, n.length - 1); g.setEnd(n, n.length);
  const r = g.getBoundingClientRect();
  return r.left >= -0.5 && r.right <= innerWidth + 0.5 && r.top >= -0.5 && r.bottom <= innerHeight + 0.5; })()`;

// whyCut is whether the sentence is cut inside its own box.
const whyCut = (el: string) => `(() => { const w = ${el}; return !w || w.scrollWidth > w.clientWidth + 1; })()`;

// sentenceRead reads one sentence a screen shows about a save it does not offer,
// at 1520 and at 500: the whole sentence and its last character inside the
// window, nothing cut, no sideways scroll, and the way to fix it still in the
// sentence. el is an expression for the sentence's element.
async function sentenceRead(page: Page, el: string, tail: string, what: string): Promise<void> {
  for (const width of [1520, 500]) {
    await viewport(page, width, 900);
    await readTo<number>(page, across(), `the page of ${what} does not scroll sideways at ${width}`).toBeLessThanOrEqual(0);
    await readTo<boolean>(page, whyWhole(el), `the whole reason of ${what} stands inside the window at ${width}`).toBe(true);
    await readTo<boolean>(page, whyLast(el), `the last character of the reason of ${what} stands inside the window at ${width}`).toBe(true);
    await readTo<boolean>(page, whyCut(el), `the reason of ${what} is not cut at ${width}`).toBe(false);
    await readTo<string>(page, `${el}.textContent`, `the reason of ${what} still says how to fix it at ${width}`).toContain(tail);
  }
}

// reasonRead is sentenceRead for one row's foot.
const reasonRead = (page: Page, row: string, tail: string, what: string) => sentenceRead(page, whyIn(row), tail, what);

test('COPY-S1 a team definition copied from another listed project to the global layer saves', async ({ page, app }) => {
  const srcFile = betaFile(app, path.join('teams', 'beta-dev', 'team.yaml'));
  const dstFile = path.join(app.cfg, 'hergy', 'teams', 'beta-dev', 'team.yaml');
  const before = readFileSync(srcFile, 'utf8');

  await page.goto(`${app.base}/teams`);
  await wait(page, there(TEAMS));
  await open(page, TEAMS, BETA, 'beta-dev');
  await press(page, copyButton(TEAMS, BETA, 'beta-dev', 'project'));
  await wait(page, `!!${rowIn(TEAMS, GLOBAL, 'beta-dev')}`);

  // The copy's save is offered: the row stands on beta's file.
  await readTo<boolean>(page, `${saveOfCopy('beta-dev')}.disabled`, 'the save of the copy is not shut').toBe(false);
  await readTo<string>(page, whyOfCopy('beta-dev'), 'the foot of a copy that can be saved says no reason').toBe('');
  await press(page, saveOfCopy('beta-dev'));
  await wait(page, `!!${rowIn(TEAMS, GLOBAL, 'beta-dev')}.querySelector('.ok')`);

  expect(existsSync(dstFile), 'the copy made the definition in the global layer').toBe(true);
  expect(readFileSync(dstFile, 'utf8'), 'the global team.yaml holds the copied description').toContain('見積もりの計算をそろえる');
  expect(readFileSync(srcFile, 'utf8'), 'copying left the listed project file alone').toBe(before);

  // The copy is a file: opening the page again shows it under the global head,
  // and beta still holds its own.
  await page.reload();
  await wait(page, there(TEAMS));
  await readTo<number>(page, countIn(TEAMS, GLOBAL), 'the copy stands under the global head after the page is opened again').toBe(2);
  expect(readFileSync(dstFile, 'utf8'), 'the global file is still the copy after opening the page again').toContain('見積もりの計算をそろえる');
});

test('COPY-S2 a team definition whose source is broken is copied and saved as it is', async ({ page, app }) => {
  // The 0.9.5 shape: team.yaml still writes mission:, and no description.
  const dir = betaFile(app, path.join('teams', 'old-dev'));
  mkdirSync(dir, { recursive: true });
  writeFileSync(path.join(dir, 'team.yaml'), [
    'version: 1',
    'mission: "古い言い方のまま"',
    'members:',
    '  - name: lead',
    '    role: lead',
    '    profile: beta-fast',
    'turns:',
    '  - human -> lead',
    '  - lead -> human',
    '',
  ].join('\n'));
  const dstFile = path.join(app.cfg, 'hergy', 'teams', 'old-dev', 'team.yaml');
  const before = readFileSync(path.join(dir, 'team.yaml'), 'utf8');

  await page.goto(`${app.base}/teams`);
  await wait(page, there(TEAMS));
  await open(page, TEAMS, BETA, 'old-dev');

  // The check says what is wrong with the source, and the save is still offered:
  // the error was there before the copy stood (d-9abb, spec-teams X3).
  await press(page, copyButton(TEAMS, BETA, 'old-dev', 'project'));
  await wait(page, `!!${rowIn(TEAMS, GLOBAL, 'old-dev')}`);
  await readTo<string>(page, `${rowIn(TEAMS, GLOBAL, 'old-dev')}.querySelector('.vlist').textContent`, 'the copy says what is wrong with the source').toContain('mission:');
  await readTo<boolean>(page, `${saveOfCopy('old-dev')}.disabled`, 'the save of a copy whose source is broken is not shut').toBe(false);
  await press(page, saveOfCopy('old-dev'));
  // The save writes the folder as it stands. The row then reads as the saved
  // definition it wrote, and a saved definition that does not read draws the
  // check column alone (spec-teams 1.9).
  await expect.poll(() => existsSync(dstFile), { timeout: 8_000, message: 'the copy wrote the global folder' }).toBe(true);
  await wait(page, `${rowIn(TEAMS, GLOBAL, 'old-dev')}.classList.contains('open')`);
  expect(readFileSync(dstFile, 'utf8'), 'the copy kept the source text as it is').toBe(before);
  await readTo<string>(page, `${rowIn(TEAMS, GLOBAL, 'old-dev')}.querySelector('.vlist').textContent`, 'the saved copy still says what is wrong with it').toContain('mission:');
});

// draftRowIn is the row under one head that is not there yet: it is the one with
// a name field, and it is how a walk keeps hold of a copy once its name changes.
const draftRowIn = (group: string) => `(${rowsIn(rootOf(TEAMS), TEAMS, group)}.find((el) => el.querySelector('.in-name')) || null)`;

test('COPY-S3 a copy whose name the global layer already holds says why its save is shut', async ({ page, app }) => {
  await page.goto(`${app.base}/teams`);
  await wait(page, there(TEAMS));
  await open(page, TEAMS, BETA, 'beta-dev');
  await press(page, copyButton(TEAMS, BETA, 'beta-dev', 'project'));
  await wait(page, `!!${rowIn(TEAMS, GLOBAL, 'beta-dev')}`);

  // The person types the name of a definition the global layer already holds.
  const copy = draftRowIn(GLOBAL);
  await set(page, `${copy}.querySelector('.in-name[data-f=name]')`, 'shared');
  await readTo<string>(page, `${copy}.querySelector('.in-name').value`, 'the name field holds the name the person typed').toBe('shared');
  await readTo<boolean>(page, `${copy}.querySelector('.detail [data-act=t-save]').disabled`, 'the save is shut while the name is taken').toBe(true);
  // The reason is read whole at a wide window and at a narrow one (ac-456e).
  await reasonRead(page, copy, '別のチーム定義名を指定してください', 'a copy whose name is taken');
});

test('COPY-S4 a copy whose source cannot be read says why its save is shut', async ({ page, app }) => {
  const srcFile = betaFile(app, path.join('teams', 'beta-dev', 'team.yaml'));
  const kept = readFileSync(srcFile, 'utf8');

  await page.goto(`${app.base}/teams`);
  await wait(page, there(TEAMS));
  await open(page, TEAMS, BETA, 'beta-dev');

  // The source goes away between the list and the copy: the base cannot be read.
  rmSync(srcFile);
  await press(page, copyButton(TEAMS, BETA, 'beta-dev', 'project'));
  const copy = rowIn(TEAMS, GLOBAL, 'beta-dev');
  await wait(page, `!!${copy}`);

  await readTo<boolean>(page, `${saveOfCopy('beta-dev')}.disabled`, 'the save of a copy with no base is shut').toBe(true);
  // The reason names what could not be read and what to do, and it is read whole
  // at a wide window and at a narrow one (ac-456e).
  await reasonRead(page, copy, 'もう一度 複製 か コピー をしてください', 'a copy whose base cannot be read');
  expect(existsSync(path.join(app.cfg, 'hergy', 'teams', 'beta-dev', 'team.yaml')), 'a copy whose base was not read wrote nothing').toBe(false);
  writeFileSync(srcFile, kept);
});

test('COPY-S5 a role copied from another listed project stands on that project’s document', async ({ page, app }) => {
  const srcFile = betaFile(app, path.join('roles', 'builder.md'));
  const dstFile = path.join(app.cfg, 'hergy', 'roles', 'builder.md');
  // A line only this project's document carries: the copy keeps it only when the
  // base it stands on is this project's file.
  writeFileSync(srcFile, '# beta だけが書いているメモ\n' + readFileSync(srcFile, 'utf8'));
  const before = readFileSync(srcFile, 'utf8');
  expect(existsSync(dstFile), 'the global layer holds no builder before the copy').toBe(false);

  await page.goto(`${app.base}/roles`);
  await wait(page, there(ROLES));
  await open(page, ROLES, BETA, 'builder');
  await press(page, copyButton(ROLES, BETA, 'builder', 'project'));
  await wait(page, `!!${rowIn(ROLES, GLOBAL, 'builder')}`);
  await press(page, `${rowIn(ROLES, GLOBAL, 'builder')}.querySelector('.panel[data-l=global] [data-act=save]')`);
  await wait(page, `!!${rowIn(ROLES, GLOBAL, 'builder')}.querySelector('.panel[data-l=global] .ok')`);

  expect(readFileSync(dstFile, 'utf8'), 'the global role document is the copy of beta’s document').toBe(before);
});

test('COPY-S6 a copy that grows an error says why its save is shut, and the reason is read whole', async ({ page, app }) => {
  await page.goto(`${app.base}/teams`);
  await wait(page, there(TEAMS));
  await open(page, TEAMS, BETA, 'beta-dev');
  await press(page, copyButton(TEAMS, BETA, 'beta-dev', 'project'));
  const copy = rowIn(TEAMS, GLOBAL, 'beta-dev');
  await wait(page, `!!${copy}`);
  await readTo<boolean>(page, `${saveOfCopy('beta-dev')}.disabled`, 'the save of a sound copy is offered').toBe(false);

  // The person clears the description the copy stands on: that error was not in
  // the definition it stands on, so the save shuts (spec-teams X3, d-9abb).
  await set(page, `${copy}.querySelector('.in-description')`, '');
  await readTo<boolean>(page, `${saveOfCopy('beta-dev')}.disabled`, 'the save is shut by the error the copy grew').toBe(true);
  await reasonRead(page, copy, 'description: を 1 行書いてください', 'a copy that grew an error');
});

test('COPY-S7 a team definition with its own rules is copied with the rules of its project', async ({ page, app }) => {
  // The same name stands in both projects, with rules and a description of its
  // own: the copy into the global layer must carry beta's two files, not the
  // ones of the project serve was started in (ac-f07b).
  // The ids are hergy's own shape: an id in an older shape is written again on
  // the next save (d-4aab), and this walk reads what the copy carries.
  const both = [
    { file: alphaFile(app, path.join('teams', 'both-dev')), description: 'alpha の見積もり', id: 'ABAA', rule: 'alpha の相談は 1 文で書く。' },
    { file: betaFile(app, path.join('teams', 'both-dev')), description: 'beta の見積もり', id: 'BAAA', rule: 'beta の相談は 1 文で書く。' },
  ];
  for (const one of both) {
    mkdirSync(one.file, { recursive: true });
    writeFileSync(path.join(one.file, 'team.yaml'), [
      'version: 1',
      `description: "${one.description}"`,
      'members:',
      '  - name: lead',
      '    role: lead',
      '    profile: alpha-fast',
      'turns:',
      '  - human -> lead',
      '  - lead -> human',
      '',
    ].join('\n'));
    writeFileSync(path.join(one.file, 'agreement.md'), one.rule + '\n');
  }
  const srcTeam = readFileSync(path.join(both[1].file, 'team.yaml'), 'utf8');
  const srcAgree = readFileSync(path.join(both[1].file, 'agreement.md'), 'utf8');
  expect(srcAgree, 'the two projects’ rules differ before the copy').not.toBe(readFileSync(path.join(both[0].file, 'agreement.md'), 'utf8'));

  await page.goto(`${app.base}/teams`);
  await wait(page, there(TEAMS));
  await open(page, TEAMS, BETA, 'both-dev');
  await press(page, copyButton(TEAMS, BETA, 'both-dev', 'project'));
  await wait(page, `!!${rowIn(TEAMS, GLOBAL, 'both-dev')}`);
  await readTo<boolean>(page, `${saveOfCopy('both-dev')}.disabled`, 'the save of the copy is not shut').toBe(false);
  await press(page, saveOfCopy('both-dev'));
  await wait(page, `!!${rowIn(TEAMS, GLOBAL, 'both-dev')}.querySelector('.ok')`);

  const dstTeam = path.join(app.cfg, 'hergy', 'teams', 'both-dev', 'team.yaml');
  const dstAgree = path.join(app.cfg, 'hergy', 'teams', 'both-dev', 'agreement.md');
  expect(existsSync(dstAgree), 'the copy made the rules in the global layer').toBe(true);
  expect(readFileSync(dstAgree, 'utf8'), 'the global agreement.md is beta’s document, byte for byte').toBe(srcAgree);
  expect(readFileSync(dstTeam, 'utf8'), 'the global team.yaml is beta’s document').toBe(srcTeam);
  expect(readFileSync(dstAgree, 'utf8'), 'the other project’s rule is not mixed in').not.toContain('alpha の相談');
});

// ---- COPY-C1 .. COPY-C4 — r-7e4a (ac-8ae7, ac-2a35, d-dc81) --------------
//
// A copy of a project's definition into the global layer writes the definition's
// two files alone. The role documents and profiles the definition's members use,
// which the global layer does not hold, are the person's to choose, and only what
// is chosen is written with it. Each walk stands on beta-dev, whose members use
// the profile beta-fast and the role builder: both stand in beta alone.
const CARRY_ROLE = (name: string) => `${rowIn(TEAMS, GLOBAL, 'beta-dev')}.querySelector('.detail [data-act=carry-role][data-name=${name}]')`;
const CARRY_PROFILE = (name: string) => `${rowIn(TEAMS, GLOBAL, 'beta-dev')}.querySelector('.detail [data-act=carry-profile][data-name=${name}]')`;
const CARRY_NOTE = (name: string) => `${rowIn(TEAMS, GLOBAL, 'beta-dev')}.querySelector('.detail .carry')`;
const CARRY_PROBLEMS = (name: string) => `${rowIn(TEAMS, GLOBAL, name)}.querySelector('.detail .vlist').textContent`;
// writesOfCopy is the files the copy's save would write, as the foot names them.
const writesOfCopy = (name: string) => `[...${rowIn(TEAMS, GLOBAL, name)}.querySelectorAll('.detail .pf .also code')].map((el) => el.textContent).join(' ')`;


test('COPY-C1 the copy says it brings the definition alone, and names what the global layer does not hold', async ({ page, app }) => {
  await page.goto(`${app.base}/teams`);
  await wait(page, there(TEAMS));
  await open(page, TEAMS, BETA, 'beta-dev');
  await press(page, copyButton(TEAMS, BETA, 'beta-dev', 'project'));
  await wait(page, `!!${rowIn(TEAMS, GLOBAL, 'beta-dev')}`);

  // The row says what the copy is: the definition's two files alone, and the
  // names it can bring, listed one by one (ac-8ae7, d-dc81).
  await readTo<string>(page, `${CARRY_NOTE('beta-dev')}.textContent`, 'the copy says the definition is what is written').toContain('グローバルに書くのは、このチーム定義だけです');
  await readTo<string>(page, `${CARRY_NOTE('beta-dev')}.textContent`, 'the copy names the role the global layer does not hold').toContain('ロール builder');
  await readTo<string>(page, `${CARRY_NOTE('beta-dev')}.textContent`, 'the copy names the profile the global layer does not hold').toContain('プロファイル beta-fast');
  await readTo<string>(page, writesOfCopy('beta-dev'), 'before anything is chosen the save writes the definition alone').toBe('team.yaml');

  // The problems say where the name is, rather than that there is none: the team
  // can be started from beta, and not from a project that has no beta-fast.
  await readTo<string>(page, CARRY_PROBLEMS('beta-dev'), 'the profile problem names the place it is missing from').toContain('グローバルにありません');
  await readTo<string>(page, CARRY_PROBLEMS('beta-dev'), 'the profile problem names the place it stands in').toContain('~/src/beta/.hergy にあります');
  await readTo<string>(page, CARRY_PROBLEMS('beta-dev'), 'the role problem names the place it is missing from').toContain('ロール builder の文書はグローバルにありません');
});

test('COPY-C2 a copy with nothing chosen writes the definition alone', async ({ page, app }) => {
  const profiles = path.join(app.cfg, 'hergy', 'profiles.yaml');
  const role = path.join(app.cfg, 'hergy', 'roles', 'builder.md');
  const beforeProfiles = readFileSync(profiles, 'utf8');
  expect(existsSync(role), 'the global layer holds no builder before the copy').toBe(false);

  await page.goto(`${app.base}/teams`);
  await wait(page, there(TEAMS));
  await open(page, TEAMS, BETA, 'beta-dev');
  await press(page, copyButton(TEAMS, BETA, 'beta-dev', 'project'));
  await wait(page, `!!${rowIn(TEAMS, GLOBAL, 'beta-dev')}`);
  await press(page, saveOfCopy('beta-dev'));
  await wait(page, `!!${rowIn(TEAMS, GLOBAL, 'beta-dev')}.querySelector('.ok')`);

  // The definition is there, and the global layer's own profiles and roles are
  // as they were, byte for byte (ac-2a35).
  expect(existsSync(path.join(app.cfg, 'hergy', 'teams', 'beta-dev', 'team.yaml')), 'the copy wrote the definition').toBe(true);
  expect(readFileSync(profiles, 'utf8'), 'nothing chosen left the global profiles.yaml as it was').toBe(beforeProfiles);
  expect(existsSync(role), 'nothing chosen wrote no role document into the global layer').toBe(false);
});

test('COPY-C3 the names a person chooses are written with the copy, as the project holds them', async ({ page, app }) => {
  const profiles = path.join(app.cfg, 'hergy', 'profiles.yaml');
  const srcRole = betaFile(app, path.join('roles', 'builder.md'));
  const dstRole = path.join(app.cfg, 'hergy', 'roles', 'builder.md');
  const beforeProfiles = readFileSync(profiles, 'utf8');

  await page.goto(`${app.base}/teams`);
  await wait(page, there(TEAMS));
  await open(page, TEAMS, BETA, 'beta-dev');
  await press(page, copyButton(TEAMS, BETA, 'beta-dev', 'project'));
  await wait(page, `!!${rowIn(TEAMS, GLOBAL, 'beta-dev')}`);

  // The two names are chosen: the foot says they go with the definition, and the
  // sentences about them go (ac-2a35).
  await press(page, CARRY_ROLE('builder'));
  await press(page, CARRY_PROFILE('beta-fast'));
  await readTo<string>(page, writesOfCopy('beta-dev'), 'the chosen names go with the definition').toContain('builder.md');
  await readTo<string>(page, writesOfCopy('beta-dev'), 'the chosen profile goes with the definition').toContain('profiles.yaml');
  await readTo<string>(page, CARRY_PROBLEMS('beta-dev'), 'the sentence about the role is gone once it is chosen').not.toContain('グローバルにありません');

  await press(page, saveOfCopy('beta-dev'));
  await wait(page, `!!${rowIn(TEAMS, GLOBAL, 'beta-dev')}.querySelector('.ok')`);

  // The role document is the project's own, byte for byte, and the profile is
  // the project's entry with the profiles the global layer already held.
  expect(readFileSync(dstRole, 'utf8'), 'the copied role document is beta’s own').toBe(readFileSync(srcRole, 'utf8'));
  const written = readFileSync(profiles, 'utf8');
  // The entry is the project's own: the same kind and the same arguments, in the
  // file the global layer already had. The args stand one to a line here, which
  // is the form the profiles screen writes; what the copy keeps is the content.
  expect(written, 'the copied profile is the project’s own').toContain('beta-fast:');
  expect(written, 'the copied profile keeps the project’s kind').toMatch(/beta-fast:\n\s+kind: claude/);
  expect(written, 'the copied profile keeps the project’s args').toContain('- claude-haiku-5-5');
  expect(written, 'the profiles the global layer held are still there').toContain('coder:');
  expect(written, 'choosing the two wrote nothing else').not.toContain('alpha-fast');
});

test('COPY-C4 a name the global layer holds when the save comes is not written over', async ({ page, app }) => {
  const dstRole = path.join(app.cfg, 'hergy', 'roles', 'builder.md');
  const mine = 'version: 1\ntitle: グローバルの builder\nsections:\n  - heading: 作り方\n    rules:\n      - id: DAAA\n        text: グローバルが持っている文書。\n';

  await page.goto(`${app.base}/teams`);
  await wait(page, there(TEAMS));
  await open(page, TEAMS, BETA, 'beta-dev');
  await press(page, copyButton(TEAMS, BETA, 'beta-dev', 'project'));
  await wait(page, `!!${rowIn(TEAMS, GLOBAL, 'beta-dev')}`);
  await press(page, CARRY_ROLE('builder'));

  // The global layer gets a builder of its own after the person chose the name:
  // the save leaves it as it is (ac-2a35).
  mkdirSync(path.dirname(dstRole), { recursive: true });
  writeFileSync(dstRole, mine);
  await press(page, saveOfCopy('beta-dev'));
  await wait(page, `!!${rowIn(TEAMS, GLOBAL, 'beta-dev')}.querySelector('.ok')`);
  expect(readFileSync(dstRole, 'utf8'), 'the copy wrote over the global layer’s own role document').toBe(mine);
});

// ---- COPY-R1 .. R3 and COPY-F1 .. F3 — r-a028, ac-456e, the role and profile
// copies -------------------------------------------------------------------
//
// A copy of a role or a profile that cannot be saved must say why and what to
// fix, the way a team definition's copy does. The sentence is read the same way:
// the whole of it, its last character too, inside the window at 1520 and at 500,
// with nothing cut and the page standing still sideways.
// copyRowIn is the row under one head that is not there yet: the one whose panel
// carries a name field. A walk keeps hold of a copy by it once its name changes,
// because the name it is found by is the name being typed.
const copyRowIn = (sel: string, group: string) => `(${rowsIn(rootOf(sel), sel, group)}.find((el) => el.querySelector('.panel .in-name')) || null)`;
const rolePanel = () => `${copyRowIn(ROLES, GLOBAL)}.querySelector('.panel[data-l=global]')`;
const profilePanel = () => `${copyRowIn(PROFILES, GLOBAL)}.querySelector('.panel[data-file=global]')`;
const footOf = (panel: string) => `${panel}.querySelector('.pf .refusal')`;
const saveOf = (panel: string) => `${panel}.querySelector('.pf [data-act=save]')`;

test('COPY-R1 a copied role whose name is not a role name says why its save is shut', async ({ page, app }) => {
  await page.goto(`${app.base}/roles`);
  await wait(page, there(ROLES));
  await open(page, ROLES, ALPHA, 'builder');
  await press(page, copyButton(ROLES, ALPHA, 'builder', 'project'));
  await wait(page, `!!${rowIn(ROLES, GLOBAL, 'builder')}`);

  // The name a person types is not one a role goes by.
  await set(page, `${rolePanel()}.querySelector('.in-name')`, 'Builder');
  await readTo<boolean>(page, `${saveOf(rolePanel())}.disabled`, 'the save of the copied role is shut').toBe(true);
  await sentenceRead(page, footOf(rolePanel()), 'ロール名は英小文字で始め', 'a copied role whose name is not a role name');
});

test('COPY-R2 a copied role can save empty prose', async ({ page, app }) => {
  await page.goto(`${app.base}/roles?open=builder`); await ready(page);
  const source = page.locator('.role.open .panel[data-l=project]');
  await source.locator('[data-act=copy]').click();
  const target = page.locator('.role.open .panel[data-l=global]').filter({ has: page.locator('[data-f=name]') });
  // Playwright writes into a textarea by selecting it and then sending the text
  // as keys; an empty text is the Delete key. The copy puts its cursor in the
  // name when the check answers, so a key sent before that landed in the name
  // and emptied it (the save then wrote roles/.md, #7). The cursor stands in the
  // name once the copy is ready for a person to type there.
  await expect(target.locator('[data-f=name]'), 'the copy puts the cursor in its name').toBeFocused();
  await target.getByRole('textbox', { name: '規則の全文' }).fill('');
  await target.locator('[data-act=save]').click(); await expect(target.locator('.ok')).toBeVisible();
  expect(readFileSync(path.join(app.cfg, 'hergy/roles/builder.md'), 'utf8')).toBe('');
});

test('COPY-R3 a copied role saved on a name the global layer holds says why it was not written', async ({ page, app }) => {
  await page.goto(`${app.base}/roles`);
  await wait(page, there(ROLES));
  await open(page, ROLES, ALPHA, 'builder');
  await press(page, copyButton(ROLES, ALPHA, 'builder', 'project'));
  await wait(page, `!!${rowIn(ROLES, GLOBAL, 'builder')}`);

  // The name the global layer holds: the save is refused, and the sentence says
  // what happened and what to do (the check itself has nothing to say about a
  // name another place holds).
  await set(page, `${rolePanel()}.querySelector('.in-name')`, 'lead');
  await press(page, saveOf(rolePanel()));
  await readTo<string>(page, `${footOf(rolePanel())}.textContent`, 'the foot says the name is taken, and what to do').toContain('別のロール名を指定してください');
  await sentenceRead(page, footOf(rolePanel()), '別のロール名を指定してください', 'a copied role saved on a name the global layer holds');
});

test('COPY-F1 a copied profile whose name is not a profile name says why its save is shut', async ({ page, app }) => {
  await page.goto(`${app.base}/profiles`);
  await wait(page, there(PROFILES));
  await open(page, PROFILES, ALPHA, 'alpha-fast');
  await press(page, copyButton(PROFILES, ALPHA, 'alpha-fast', 'project'));
  await wait(page, `!!${rowIn(PROFILES, GLOBAL, 'alpha-fast')}`);

  await set(page, `${profilePanel()}.querySelector('.in-name')`, 'Alpha Fast');
  await readTo<boolean>(page, `${saveOf(profilePanel())}.disabled`, 'the save of the copied profile is shut').toBe(true);
  await sentenceRead(page, footOf(profilePanel()), 'の形にしてください', 'a copied profile whose name is not a profile name');
});

test('COPY-F2 a copied profile whose name the global layer holds says why its save is shut', async ({ page, app }) => {
  await page.goto(`${app.base}/profiles`);
  await wait(page, there(PROFILES));
  await open(page, PROFILES, ALPHA, 'alpha-fast');
  await press(page, copyButton(PROFILES, ALPHA, 'alpha-fast', 'project'));
  await wait(page, `!!${rowIn(PROFILES, GLOBAL, 'alpha-fast')}`);

  await set(page, `${profilePanel()}.querySelector('.in-name')`, 'coder');
  await readTo<boolean>(page, `${saveOf(profilePanel())}.disabled`, 'the save of the copied profile is shut by the name').toBe(true);
  // The sentence says the way to fix it: the name is one the file already holds,
  // so another profile name is what the person picks (ac-456e, r-a028).
  await sentenceRead(page, footOf(profilePanel()), '別のプロファイル名を指定してください', 'a copied profile whose name the global layer holds');
});

test('COPY-F3 a copied profile whose variable name is not a name says why its save is shut', async ({ page, app }) => {
  await page.goto(`${app.base}/profiles`);
  await wait(page, there(PROFILES));
  await open(page, PROFILES, ALPHA, 'alpha-fast');
  await press(page, copyButton(PROFILES, ALPHA, 'alpha-fast', 'project'));
  await wait(page, `!!${rowIn(PROFILES, GLOBAL, 'alpha-fast')}`);

  // The copy gains a variable whose name hergy cannot pass on: the check names
  // it, and the save shuts.
  await press(page, `${profilePanel()}.querySelector('[data-act=env-add]')`);
  await wait(page, `!!${profilePanel()}.querySelector('.envrow [data-f=ek]')`);
  await set(page, `${profilePanel()}.querySelector('.envrow [data-f=ek]')`, '1BAD');
  await readTo<boolean>(page, `${saveOf(profilePanel())}.disabled`, 'the save of the copied profile is shut by its variable').toBe(true);
  await sentenceRead(page, footOf(profilePanel()), '環境変数の名前として使えません', 'a copied profile whose variable name is not a name');
});
