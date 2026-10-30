// PLACE-R1 .. PLACE-P4 — r-410c (d-e77c, ac-e9b7): the roles and profiles lists
// draw a head for each place, the head's own button adds a row under it, and
// the save writes that place's file. Each walk also holds that the list above
// has no add button and that the new row carries no place switch.
//
// Two describes: one on the one-project fixtures (no projects: listed), one on
// the headquarters (testdata/hq, three projects and the global layer).
import { test, expect, type Page } from './fixtures';
import { existsSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { press, read, readTo, set, there, wait } from './helpers';

const ROLES = '#rows .role';
const PROFILES = '#rows .row';
// The teams list has no #rows: its rows and heads stand directly under .list.
const TEAMS = '#view .list .tdef';
const rootOf = (sel: string) => (sel === TEAMS ? '#view .list' : '#rows');

// headOf is one place's head: the first head with that data-place.
const headOf = (place: string, root = '#rows') => `document.querySelector('${root} .hq-gh[data-place="${place}"]')`;
const headCount = (root = '#rows') => `document.querySelectorAll('${root} .hq-gh').length`;

// rowsIn is the rows one place's head stands over: a head and the rows under it
// until the next head.
const rowsIn = (sel: string, place: string) => `(() => {
  const out = [];
  let on = null;
  for (const el of document.querySelector(${JSON.stringify(rootOf(sel))}).children) {
    if (el.classList.contains('hq-gh')) { on = el.dataset.place || null; continue; }
    if (on === ${JSON.stringify(place)} && el.matches(${JSON.stringify(sel)})) out.push(el);
  }
  return out;
})()`;
const countIn = (sel: string, place: string) => `${rowsIn(sel, place)}.length`;
const firstIn = (sel: string, place: string) => `(${rowsIn(sel, place)}[0] || null)`;
const byName = (sel: string, place: string, name: string) => `(${rowsIn(sel, place)}.find((el) => {
  const n = el.querySelector('.c-name .nm');
  return !!n && n.textContent.trim() === ${JSON.stringify(name)};
}) || null)`;

// addUnder presses one place's head button and answers the row it put at the
// top of that head.
async function addUnder(page: Page, sel: string, place: string): Promise<string> {
  await press(page, `${headOf(place, rootOf(sel))}.querySelector('[data-act=new]')`);
  const row = firstIn(sel, place);
  await wait(page, `!!${row}`);
  return row;
}

// noPlaceSwitch holds that one row carries no way to move it to the other place.
async function noPlaceSwitch(page: Page, row: string): Promise<void> {
  await readTo<boolean>(page, `!(${row}).querySelector('[data-act=pick-place], [data-act=pick-layer]')`, 'the new row carries no place switch').toBe(true);
}

test.describe('the one project', () => {
  test.use({ reads: 'roles' });

  test('PLACE-R1 a role added under the project head', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'roles', 'place-p1.yaml');
    const global = path.join(app.cfg, 'hergy', 'roles', 'place-p1.yaml');
    await page.goto(`${app.base}/roles`);
    await wait(page, there(ROLES));
    const beforeGlobal = await read<number>(page, countIn(ROLES, 'global'));
    const beforeProject = await read<number>(page, countIn(ROLES, 'project'));
    await readTo<boolean>(page, `!document.querySelector('.toolbar [data-act=new]')`, 'the list above has no add button').toBe(true);
    await readTo<number>(page, headCount(), 'the list draws the project and the global layer heads').toBe(2);
    const said = await read<string>(page, `${headOf('project')}.querySelector('.hq-gn').textContent`, 'the project head name');
    await readTo<string>(page, `${headOf('project')}.querySelector('[data-act=new]').textContent`, 'the project button says the place').toContain(said);
    expect(existsSync(file), 'the project holds no place-p1 before the add').toBe(false);

    const row = await addUnder(page, ROLES, 'project');
    await noPlaceSwitch(page, row);
    await readTo<string>(page, `${row}.querySelector('.c-place .pill').textContent`, 'the new row says the project').toBe(said);
    await set(page, `${row}.querySelector('[data-f=name]')`, 'place-p1');
    await set(page, `${row}.querySelector('.rule [data-f=text]')`, '置き場所の見出しから足したロール。');
    await press(page, `${row}.querySelector('.panel [data-act=save]')`);
    await wait(page, `!!(${byName(ROLES, 'project', 'place-p1')}).querySelector('.ok')`);

    await readTo<number>(page, countIn(ROLES, 'project'), 'the project head holds the added role').toBe(beforeProject + 1);
    await readTo<number>(page, countIn(ROLES, 'global'), 'the global head keeps its roles').toBe(beforeGlobal);
    expect(existsSync(file), 'the save wrote the project file').toBe(true);
    expect(existsSync(global), 'the save wrote no global file').toBe(false);
    await page.reload();
    await wait(page, there(ROLES));
    await readTo<number>(page, countIn(ROLES, 'project'), 'the added role stands under the project after opening the page again').toBe(beforeProject + 1);
  });

  test('PLACE-R2 a role added under the global head', async ({ page, app }) => {
    const file = path.join(app.cfg, 'hergy', 'roles', 'place-g1.yaml');
    const project = path.join(app.proj, '.hergy', 'roles', 'place-g1.yaml');
    await page.goto(`${app.base}/roles`);
    await wait(page, there(ROLES));
    const beforeProject = await read<number>(page, countIn(ROLES, 'project'));
    const beforeGlobal = await read<number>(page, countIn(ROLES, 'global'));
    await readTo<boolean>(page, `!document.querySelector('.toolbar [data-act=new]')`, 'the list above has no add button').toBe(true);
    await readTo<string>(page, `${headOf('global')}.querySelector('[data-act=new]').textContent`, 'the global button says the place').toContain('グローバル');
    expect(existsSync(file), 'the global layer holds no place-g1 before the add').toBe(false);

    const row = await addUnder(page, ROLES, 'global');
    await noPlaceSwitch(page, row);
    await set(page, `${row}.querySelector('[data-f=name]')`, 'place-g1');
    await set(page, `${row}.querySelector('.rule [data-f=text]')`, 'グローバルの見出しから足したロール。');
    await press(page, `${row}.querySelector('.panel [data-act=save]')`);
    await wait(page, `!!(${byName(ROLES, 'global', 'place-g1')}).querySelector('.ok')`);

    await readTo<number>(page, countIn(ROLES, 'global'), 'the global head holds the added role').toBe(beforeGlobal + 1);
    await readTo<number>(page, countIn(ROLES, 'project'), 'the project head keeps its roles').toBe(beforeProject);
    expect(existsSync(file), 'the save wrote the global file').toBe(true);
    expect(existsSync(project), 'the save wrote no project file').toBe(false);
    await page.reload();
    await wait(page, there(ROLES));
    await readTo<number>(page, countIn(ROLES, 'global'), 'the added role stands under the global layer after opening the page again').toBe(beforeGlobal + 1);
  });
});

test.describe('the one project, profiles', () => {
  test.use({ reads: 'profiles' });

  test('PLACE-P1 a profile added under the project head', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'profiles.yaml');
    const global = path.join(app.cfg, 'hergy', 'profiles.yaml');
    const before = readFileSync(file, 'utf8');
    await page.goto(`${app.base}/profiles`);
    await wait(page, there(PROFILES));
    const beforeGlobal = await read<number>(page, countIn(PROFILES, 'global'));
    const beforeProject = await read<number>(page, countIn(PROFILES, 'project'));
    await readTo<boolean>(page, `!document.querySelector('.toolbar [data-act=new]')`, 'the list above has no add button').toBe(true);
    const said = await read<string>(page, `${headOf('project')}.querySelector('.hq-gn').textContent`, 'the project head name');
    await readTo<string>(page, `${headOf('project')}.querySelector('[data-act=new]').textContent`, 'the project button says the place').toContain(said);
    expect(before, 'the project holds no place-p1 before the add').not.toContain('place-p1');

    const row = await addUnder(page, PROFILES, 'project');
    await noPlaceSwitch(page, row);
    await readTo<string>(page, `${row}.querySelector('.pill.pending').textContent`, 'the new row says the project').toBe(said);
    await set(page, `${row}.querySelector('.in-name')`, 'place-p1');
    await press(page, `${row}.querySelector('.panel [data-act=save]')`);
    await wait(page, `!!(${byName(PROFILES, 'project', 'place-p1')}).querySelector('.ok')`);

    await readTo<number>(page, countIn(PROFILES, 'project'), 'the project head holds the added profile').toBe(beforeProject + 1);
    await readTo<number>(page, countIn(PROFILES, 'global'), 'the global head keeps its profiles').toBe(beforeGlobal);
    expect(readFileSync(file, 'utf8'), 'the save wrote the project file').toContain('place-p1');
    expect(readFileSync(global, 'utf8'), 'the save wrote no global file').not.toContain('place-p1');
    await page.reload();
    await wait(page, there(PROFILES));
    await readTo<number>(page, countIn(PROFILES, 'project'), 'the added profile stands under the project after opening the page again').toBe(beforeProject + 1);
  });

  test('PLACE-P2 a profile added under the global head', async ({ page, app }) => {
    const file = path.join(app.cfg, 'hergy', 'profiles.yaml');
    const project = path.join(app.proj, '.hergy', 'profiles.yaml');
    const before = readFileSync(file, 'utf8');
    await page.goto(`${app.base}/profiles`);
    await wait(page, there(PROFILES));
    const beforeProject = await read<number>(page, countIn(PROFILES, 'project'));
    const beforeGlobal = await read<number>(page, countIn(PROFILES, 'global'));
    await readTo<boolean>(page, `!document.querySelector('.toolbar [data-act=new]')`, 'the list above has no add button').toBe(true);
    await readTo<string>(page, `${headOf('global')}.querySelector('[data-act=new]').textContent`, 'the global button says the place').toContain('グローバル');
    expect(before, 'the global layer holds no place-g1 before the add').not.toContain('place-g1');

    const row = await addUnder(page, PROFILES, 'global');
    await noPlaceSwitch(page, row);
    await set(page, `${row}.querySelector('.in-name')`, 'place-g1');
    await press(page, `${row}.querySelector('.panel [data-act=save]')`);
    await wait(page, `!!(${byName(PROFILES, 'global', 'place-g1')}).querySelector('.ok')`);

    await readTo<number>(page, countIn(PROFILES, 'global'), 'the global head holds the added profile').toBe(beforeGlobal + 1);
    await readTo<number>(page, countIn(PROFILES, 'project'), 'the project head keeps its profiles').toBe(beforeProject);
    expect(readFileSync(file, 'utf8'), 'the save wrote the global file').toContain('place-g1');
    expect(readFileSync(project, 'utf8'), 'the save wrote no project file').not.toContain('place-g1');
    await page.reload();
    await wait(page, there(PROFILES));
    await readTo<number>(page, countIn(PROFILES, 'global'), 'the added profile stands under the global layer after opening the page again').toBe(beforeGlobal + 1);
  });
});

test.describe('the headquarters', () => {
  test.use({ reads: 'hq' });

  test('PLACE-R3 a role added under a listed project head', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'roles', 'place-p3.yaml');
    const global = path.join(app.cfg, 'hergy', 'roles', 'place-p3.yaml');
    await page.goto(`${app.base}/roles`);
    await wait(page, there(ROLES));
    const beforeProject = await read<number>(page, countIn(ROLES, 'project'));
    const beforeGlobal = await read<number>(page, countIn(ROLES, 'global'));
    await readTo<boolean>(page, `!document.querySelector('.toolbar [data-act=new]')`, 'the list above has no add button').toBe(true);
    await readTo<number>(page, headCount(), 'the list draws a head for every listed project and the global layer').toBe(4);
    const said = await read<string>(page, `${headOf('project')}.querySelector('.hq-gn').textContent`, 'the first project head name');
    await readTo<string>(page, `${headOf('project')}.querySelector('[data-act=new]').textContent`, 'the project button says the place').toContain(said);

    const row = await addUnder(page, ROLES, 'project');
    await noPlaceSwitch(page, row);
    await set(page, `${row}.querySelector('[data-f=name]')`, 'place-p3');
    await set(page, `${row}.querySelector('.rule [data-f=text]')`, '本部のプロジェクトの見出しから足したロール。');
    await press(page, `${row}.querySelector('.panel [data-act=save]')`);
    await wait(page, `!!(${byName(ROLES, 'project', 'place-p3')}).querySelector('.ok')`);

    await readTo<number>(page, countIn(ROLES, 'project'), 'the project head holds the added role').toBe(beforeProject + 1);
    await readTo<number>(page, countIn(ROLES, 'global'), 'the global head keeps its roles').toBe(beforeGlobal);
    expect(existsSync(file), 'the save wrote the listed project file').toBe(true);
    expect(existsSync(global), 'the save wrote no global file').toBe(false);
    await page.reload();
    await wait(page, there(ROLES));
    await readTo<number>(page, countIn(ROLES, 'project'), 'the added role stands under the project after opening the page again').toBe(beforeProject + 1);
  });

  test('PLACE-R4 a role added under the global head', async ({ page, app }) => {
    const file = path.join(app.cfg, 'hergy', 'roles', 'place-g4.yaml');
    const project = path.join(app.proj, '.hergy', 'roles', 'place-g4.yaml');
    await page.goto(`${app.base}/roles`);
    await wait(page, there(ROLES));
    const beforeProject = await read<number>(page, countIn(ROLES, 'project'));
    const beforeGlobal = await read<number>(page, countIn(ROLES, 'global'));
    await readTo<string>(page, `${headOf('global')}.querySelector('[data-act=new]').textContent`, 'the global button says the place').toContain('グローバル');

    const row = await addUnder(page, ROLES, 'global');
    await noPlaceSwitch(page, row);
    await set(page, `${row}.querySelector('[data-f=name]')`, 'place-g4');
    await set(page, `${row}.querySelector('.rule [data-f=text]')`, '本部のグローバルの見出しから足したロール。');
    await press(page, `${row}.querySelector('.panel [data-act=save]')`);
    await wait(page, `!!(${byName(ROLES, 'global', 'place-g4')}).querySelector('.ok')`);

    await readTo<number>(page, countIn(ROLES, 'global'), 'the global head holds the added role').toBe(beforeGlobal + 1);
    await readTo<number>(page, countIn(ROLES, 'project'), 'the project head keeps its roles').toBe(beforeProject);
    expect(existsSync(file), 'the save wrote the global file').toBe(true);
    expect(existsSync(project), 'the save wrote no project file').toBe(false);
    await page.reload();
    await wait(page, there(ROLES));
    await readTo<number>(page, countIn(ROLES, 'global'), 'the added role stands under the global layer after opening the page again').toBe(beforeGlobal + 1);
  });

  test('PLACE-P3 a profile added under a listed project head', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'profiles.yaml');
    const global = path.join(app.cfg, 'hergy', 'profiles.yaml');
    const before = readFileSync(file, 'utf8');
    await page.goto(`${app.base}/profiles`);
    await wait(page, there(PROFILES));
    const beforeProject = await read<number>(page, countIn(PROFILES, 'project'));
    const beforeGlobal = await read<number>(page, countIn(PROFILES, 'global'));
    await readTo<boolean>(page, `!document.querySelector('.toolbar [data-act=new]')`, 'the list above has no add button').toBe(true);
    await readTo<number>(page, headCount(), 'the list draws a head for every listed project and the global layer').toBe(4);
    expect(before, 'the project holds no place-p3 before the add').not.toContain('place-p3');

    const row = await addUnder(page, PROFILES, 'project');
    await noPlaceSwitch(page, row);
    await set(page, `${row}.querySelector('.in-name')`, 'place-p3');
    await press(page, `${row}.querySelector('.panel [data-act=save]')`);
    await wait(page, `!!(${byName(PROFILES, 'project', 'place-p3')}).querySelector('.ok')`);

    await readTo<number>(page, countIn(PROFILES, 'project'), 'the project head holds the added profile').toBe(beforeProject + 1);
    await readTo<number>(page, countIn(PROFILES, 'global'), 'the global head keeps its profiles').toBe(beforeGlobal);
    expect(readFileSync(file, 'utf8'), 'the save wrote the listed project file').toContain('place-p3');
    expect(readFileSync(global, 'utf8'), 'the save wrote no global file').not.toContain('place-p3');
    await page.reload();
    await wait(page, there(PROFILES));
    await readTo<number>(page, countIn(PROFILES, 'project'), 'the added profile stands under the project after opening the page again').toBe(beforeProject + 1);
  });

  test('PLACE-P4 a profile added under the global head', async ({ page, app }) => {
    const file = path.join(app.cfg, 'hergy', 'profiles.yaml');
    const project = path.join(app.proj, '.hergy', 'profiles.yaml');
    const before = readFileSync(file, 'utf8');
    await page.goto(`${app.base}/profiles`);
    await wait(page, there(PROFILES));
    const beforeProject = await read<number>(page, countIn(PROFILES, 'project'));
    const beforeGlobal = await read<number>(page, countIn(PROFILES, 'global'));
    await readTo<string>(page, `${headOf('global')}.querySelector('[data-act=new]').textContent`, 'the global button says the place').toContain('グローバル');
    expect(before, 'the global layer holds no place-g4 before the add').not.toContain('place-g4');

    const row = await addUnder(page, PROFILES, 'global');
    await noPlaceSwitch(page, row);
    await set(page, `${row}.querySelector('.in-name')`, 'place-g4');
    await press(page, `${row}.querySelector('.panel [data-act=save]')`);
    await wait(page, `!!(${byName(PROFILES, 'global', 'place-g4')}).querySelector('.ok')`);

    await readTo<number>(page, countIn(PROFILES, 'global'), 'the global head holds the added profile').toBe(beforeGlobal + 1);
    await readTo<number>(page, countIn(PROFILES, 'project'), 'the project head keeps its profiles').toBe(beforeProject);
    expect(readFileSync(file, 'utf8'), 'the save wrote the global file').toContain('place-g4');
    expect(readFileSync(project, 'utf8'), 'the save wrote no project file').not.toContain('place-g4');
    await page.reload();
    await wait(page, there(PROFILES));
    await readTo<number>(page, countIn(PROFILES, 'global'), 'the added profile stands under the global layer after opening the page again').toBe(beforeGlobal + 1);
  });
});

test.describe('the one project, teams', () => {
  test.use({ reads: 'teams' });

  test('PLACE-T1 a team definition added under the project head', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'teams', 'place-t1', 'team.yaml');
    const global = path.join(app.cfg, 'hergy', 'teams', 'place-t1', 'team.yaml');
    await page.goto(`${app.base}/teams`);
    await wait(page, there(TEAMS));
    const beforeGlobal = await read<number>(page, countIn(TEAMS, 'global'));
    const beforeProject = await read<number>(page, countIn(TEAMS, 'project'));
    await readTo<boolean>(page, `!document.querySelector('.toolbar [data-act=new]')`, 'the list above has no add button').toBe(true);
    await readTo<number>(page, headCount(rootOf(TEAMS)), 'the list draws the project and the global layer heads').toBe(2);
    const said = await read<string>(page, `${headOf('project', rootOf(TEAMS))}.querySelector('.hq-gn').textContent`, 'the project head name');
    await readTo<string>(page, `${headOf('project', rootOf(TEAMS))}.querySelector('[data-act=new]').textContent`, 'the project button says the place').toContain(said);
    expect(existsSync(file), 'the project holds no place-t1 before the add').toBe(false);

    const row = await addUnder(page, TEAMS, 'project');
    await noPlaceSwitch(page, row);
    await readTo<string>(page, `${row}.querySelector('.pill.pending').textContent`, 'the new row says the project').toBe(said);
    await set(page, `${row}.querySelector('.in-name')`, 'place-t1');
    await press(page, `${row}.querySelector('.detail [data-act=t-save]')`);
    await wait(page, `!!(${byName(TEAMS, 'project', 'place-t1')}).querySelector('.ok')`);

    await readTo<number>(page, countIn(TEAMS, 'project'), 'the project head holds the added definition').toBe(beforeProject + 1);
    await readTo<number>(page, countIn(TEAMS, 'global'), 'the global head keeps its definitions').toBe(beforeGlobal);
    expect(existsSync(file), 'the save wrote the project file').toBe(true);
    expect(existsSync(global), 'the save wrote no global file').toBe(false);
    await page.reload();
    await wait(page, there(TEAMS));
    await readTo<number>(page, countIn(TEAMS, 'project'), 'the added definition stands under the project after opening the page again').toBe(beforeProject + 1);
  });

  test('PLACE-T2 a team definition added under the global head', async ({ page, app }) => {
    const file = path.join(app.cfg, 'hergy', 'teams', 'place-t2', 'team.yaml');
    const project = path.join(app.proj, '.hergy', 'teams', 'place-t2', 'team.yaml');
    await page.goto(`${app.base}/teams`);
    await wait(page, there(TEAMS));
    const beforeProject = await read<number>(page, countIn(TEAMS, 'project'));
    const beforeGlobal = await read<number>(page, countIn(TEAMS, 'global'));
    await readTo<boolean>(page, `!document.querySelector('.toolbar [data-act=new]')`, 'the list above has no add button').toBe(true);
    await readTo<string>(page, `${headOf('global', rootOf(TEAMS))}.querySelector('[data-act=new]').textContent`, 'the global button says the place').toContain('グローバル');
    expect(existsSync(file), 'the global layer holds no place-t2 before the add').toBe(false);

    const row = await addUnder(page, TEAMS, 'global');
    await noPlaceSwitch(page, row);
    await set(page, `${row}.querySelector('.in-name')`, 'place-t2');
    await press(page, `${row}.querySelector('.detail [data-act=t-save]')`);
    await wait(page, `!!(${byName(TEAMS, 'global', 'place-t2')}).querySelector('.ok')`);

    await readTo<number>(page, countIn(TEAMS, 'global'), 'the global head holds the added definition').toBe(beforeGlobal + 1);
    await readTo<number>(page, countIn(TEAMS, 'project'), 'the project head keeps its definitions').toBe(beforeProject);
    expect(existsSync(file), 'the save wrote the global file').toBe(true);
    expect(existsSync(project), 'the save wrote no project file').toBe(false);
    await page.reload();
    await wait(page, there(TEAMS));
    await readTo<number>(page, countIn(TEAMS, 'global'), 'the added definition stands under the global layer after opening the page again').toBe(beforeGlobal + 1);
  });
});

test.describe('the headquarters, teams', () => {
  test.use({ reads: 'hq' });

  test('PLACE-T3 a team definition added under a listed project head', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'teams', 'place-t3', 'team.yaml');
    const global = path.join(app.cfg, 'hergy', 'teams', 'place-t3', 'team.yaml');
    await page.goto(`${app.base}/teams`);
    await wait(page, there(TEAMS));
    const beforeProject = await read<number>(page, countIn(TEAMS, 'project'));
    const beforeGlobal = await read<number>(page, countIn(TEAMS, 'global'));
    await readTo<boolean>(page, `!document.querySelector('.toolbar [data-act=new]')`, 'the list above has no add button').toBe(true);
    await readTo<number>(page, headCount(rootOf(TEAMS)), 'the list draws a head for every listed project and the global layer').toBe(4);
    const said = await read<string>(page, `${headOf('project', rootOf(TEAMS))}.querySelector('.hq-gn').textContent`, 'the first project head name');
    await readTo<string>(page, `${headOf('project', rootOf(TEAMS))}.querySelector('[data-act=new]').textContent`, 'the project button says the place').toContain(said);

    const row = await addUnder(page, TEAMS, 'project');
    await noPlaceSwitch(page, row);
    await set(page, `${row}.querySelector('.in-name')`, 'place-t3');
    await press(page, `${row}.querySelector('.detail [data-act=t-save]')`);
    await wait(page, `!!(${byName(TEAMS, 'project', 'place-t3')}).querySelector('.ok')`);

    await readTo<number>(page, countIn(TEAMS, 'project'), 'the project head holds the added definition').toBe(beforeProject + 1);
    await readTo<number>(page, countIn(TEAMS, 'global'), 'the global head keeps its definitions').toBe(beforeGlobal);
    expect(existsSync(file), 'the save wrote the listed project file').toBe(true);
    expect(existsSync(global), 'the save wrote no global file').toBe(false);
    await page.reload();
    await wait(page, there(TEAMS));
    await readTo<number>(page, countIn(TEAMS, 'project'), 'the added definition stands under the project after opening the page again').toBe(beforeProject + 1);
  });

  test('PLACE-T4 a team definition added under the global head', async ({ page, app }) => {
    const file = path.join(app.cfg, 'hergy', 'teams', 'place-t4', 'team.yaml');
    const project = path.join(app.proj, '.hergy', 'teams', 'place-t4', 'team.yaml');
    await page.goto(`${app.base}/teams`);
    await wait(page, there(TEAMS));
    const beforeProject = await read<number>(page, countIn(TEAMS, 'project'));
    const beforeGlobal = await read<number>(page, countIn(TEAMS, 'global'));
    await readTo<string>(page, `${headOf('global', rootOf(TEAMS))}.querySelector('[data-act=new]').textContent`, 'the global button says the place').toContain('グローバル');

    const row = await addUnder(page, TEAMS, 'global');
    await noPlaceSwitch(page, row);
    await set(page, `${row}.querySelector('.in-name')`, 'place-t4');
    await press(page, `${row}.querySelector('.detail [data-act=t-save]')`);
    await wait(page, `!!(${byName(TEAMS, 'global', 'place-t4')}).querySelector('.ok')`);

    await readTo<number>(page, countIn(TEAMS, 'global'), 'the global head holds the added definition').toBe(beforeGlobal + 1);
    await readTo<number>(page, countIn(TEAMS, 'project'), 'the project head keeps its definitions').toBe(beforeProject);
    expect(existsSync(file), 'the save wrote the global file').toBe(true);
    expect(existsSync(project), 'the save wrote no project file').toBe(false);
    await page.reload();
    await wait(page, there(TEAMS));
    await readTo<number>(page, countIn(TEAMS, 'global'), 'the added definition stands under the global layer after opening the page again').toBe(beforeGlobal + 1);
  });
});
