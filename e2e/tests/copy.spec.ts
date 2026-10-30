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
import { existsSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { press, read, readTo, there, wait } from './helpers';

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

const rowIn = (sel: string, group: string, name: string) => `(${rowsIn(rootOf(sel), sel, group)}.find((el) => {
  const n = el.querySelector('.c-name .nm');
  return !!n && n.textContent.trim() === ${JSON.stringify(name)};
}) || null)`;

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
  const srcFile = path.join(app.proj, '.hergy', 'roles', 'builder.yaml');
  const dstFile = path.join(app.cfg, 'hergy', 'roles', 'builder.yaml');
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
  const srcFile = path.join(app.cfg, 'hergy', 'roles', 'lead.yaml');
  const dstFile = path.join(app.proj, '.hergy', 'roles', 'lead.yaml');
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
  expect(readFileSync(dstFile, 'utf8'), 'the global team.yaml holds the copied mission').toContain('注文の流れを作り直す');
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
  expect(readFileSync(dstFile, 'utf8'), 'the project team.yaml holds the copied mission').toContain('困りごとの相談');
  expect(readFileSync(srcFile, 'utf8'), 'copying left the global file alone').toBe(before);
  await countsAfterReload(page, TEAMS, GLOBAL, ALPHA, srcCount, dstCount + 1);
  expect(readFileSync(dstFile, 'utf8'), 'the project file is still the copy after opening the page again').toContain('困りごとの相談');
});
