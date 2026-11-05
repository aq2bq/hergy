// PLACE-R1 .. PLACE-P4 — r-410c (d-e77c, ac-e9b7): the roles and profiles lists
// draw a head for each place, the head's own button adds a row under it, and
// the save writes that place's file. Each walk also holds that the list above
// has no add button and that the new row carries no place switch.
//
// Two describes: one on the one-project fixtures (no projects: listed), one on
// the headquarters (testdata/hq, three projects and the global layer).
import { test, expect } from './fixtures';
import { existsSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { read, readTo, there, wait } from './helpers';

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

test.describe('the one project', () => {
  test.use({ reads: 'roles' });

  test('PLACE-R1 a role added under the project head', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'roles', 'place-p1.md');
    const global = path.join(app.cfg, 'hergy', 'roles', 'place-p1.md');
    await page.goto(`${app.base}/roles`);
    await wait(page, there(ROLES));
    const beforeGlobal = await read<number>(page, countIn(ROLES, 'global'));
    const beforeProject = await read<number>(page, countIn(ROLES, 'project'));
    await readTo<boolean>(page, `!document.querySelector('.toolbar [data-act=new]')`, 'the list above has no add button').toBe(true);
    await readTo<number>(page, headCount(), 'the list draws the project and the global layer heads').toBe(2);
    const said = await read<string>(page, `${headOf('project')}.querySelector('.hq-gn').textContent`, 'the project head name');
    await readTo<string>(page, `${headOf('project')}.querySelector('[data-act=new]').textContent`, 'the project button says the place').toContain(said);
    const head = page.locator('#rows .hq-gh[data-place="project"]');
    expect(existsSync(file), 'the project holds no place-p1 before the add').toBe(false);

    // The row a head adds carries 新規 until its save, so the walk finds the row
    // it made by that mark (r-410c). A wait for the first row under the head
    // answers before the press is drawn instead, because rows of that place
    // already stand there, and the walk then read a row it did not make.
    await head.getByRole('button', { name: 'にロールを追加' }).click();
    const added = page.locator('#rows .role').filter({ has: page.locator('.tag') });
    await expect(added, 'the head added one row').toHaveCount(1);
    await expect(added.locator('[data-act=pick-place], [data-act=pick-layer]'), 'the new row carries no place switch').toHaveCount(0);
    await expect(added.locator('.c-place .pill'), 'the new row says the project').toHaveText(said);
    // The screen puts the cursor in the name once the check has answered; typing
    // before that would land in the name when the cursor moves under it.
    await expect(added.locator('[data-f=name]'), 'the new row offers its name').toBeFocused();
    await added.locator('[data-f=name]').fill('place-p1');
    const row = page.locator('#rows .role').filter({ has: page.locator('.c-name .nm', { hasText: 'place-p1' }) });
    await row.getByRole('textbox', { name: '規則の全文' }).fill('置き場所の見出しから足したロール。');
    await row.locator('.panel [data-act=save]').click();
    await expect(row.locator('.ok'), 'the save answered').toBeVisible();

    await readTo<number>(page, countIn(ROLES, 'project'), 'the project head holds the added role').toBe(beforeProject + 1);
    await readTo<number>(page, countIn(ROLES, 'global'), 'the global head keeps its roles').toBe(beforeGlobal);
    expect(existsSync(file), 'the save wrote the project file').toBe(true);
    expect(existsSync(global), 'the save wrote no global file').toBe(false);
    await page.reload();
    await wait(page, there(ROLES));
    await readTo<number>(page, countIn(ROLES, 'project'), 'the added role stands under the project after opening the page again').toBe(beforeProject + 1);
  });

  test('PLACE-R2 a role added under the global head', async ({ page, app }) => {
    const file = path.join(app.cfg, 'hergy', 'roles', 'place-g1.md');
    const project = path.join(app.proj, '.hergy', 'roles', 'place-g1.md');
    await page.goto(`${app.base}/roles`);
    await wait(page, there(ROLES));
    const beforeProject = await read<number>(page, countIn(ROLES, 'project'));
    const beforeGlobal = await read<number>(page, countIn(ROLES, 'global'));
    await readTo<boolean>(page, `!document.querySelector('.toolbar [data-act=new]')`, 'the list above has no add button').toBe(true);
    await readTo<string>(page, `${headOf('global')}.querySelector('[data-act=new]').textContent`, 'the global button says the place').toContain('グローバル');
    expect(existsSync(file), 'the global layer holds no place-g1 before the add').toBe(false);

    // The row is found by the 新規 mark it carries until its save (PLACE-R1).
    const head = page.locator('#rows .hq-gh[data-place="global"]');
    await head.getByRole('button', { name: 'にロールを追加' }).click();
    const added = page.locator('#rows .role').filter({ has: page.locator('.tag') });
    await expect(added, 'the head added one row').toHaveCount(1);
    await expect(added.locator('[data-act=pick-place], [data-act=pick-layer]'), 'the new row carries no place switch').toHaveCount(0);
    // The screen puts the cursor in the name once the check has answered.
    await expect(added.locator('[data-f=name]'), 'the new row offers its name').toBeFocused();
    await added.locator('[data-f=name]').fill('place-g1');
    const row = page.locator('#rows .role').filter({ has: page.locator('.c-name .nm', { hasText: 'place-g1' }) });
    await row.getByRole('textbox', { name: '規則の全文' }).fill('グローバルの見出しから足したロール。');
    await row.locator('.panel [data-act=save]').click();
    await expect(row.locator('.ok'), 'the save answered').toBeVisible();

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

    // The row is found by the 新規 mark it carries until its save (PLACE-R1).
    const head = page.locator('#rows .hq-gh[data-place="project"]');
    await head.getByRole('button', { name: 'にプロファイルを追加' }).click();
    const added = page.locator('#rows .row').filter({ has: page.locator('.tag') });
    await expect(added, 'the head added one row').toHaveCount(1);
    await expect(added.locator('[data-act=pick-place], [data-act=pick-layer]'), 'the new row carries no place switch').toHaveCount(0);
    await expect(added.locator('.pill.pending'), 'the new row says the project').toHaveText(said);
    // The screen puts the cursor in the name once the check has answered.
    await expect(added.locator('.in-name'), 'the new row offers its name').toBeFocused();
    await added.locator('.in-name').fill('place-p1');
    const row = page.locator('#rows .row').filter({ has: page.locator('.c-name .nm', { hasText: 'place-p1' }) });
    await row.locator('.panel [data-act=save]').click();
    await expect(row.locator('.ok'), 'the save answered').toBeVisible();

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

    // The row is found by the 新規 mark it carries until its save (PLACE-R1).
    const head = page.locator('#rows .hq-gh[data-place="global"]');
    await head.getByRole('button', { name: 'にプロファイルを追加' }).click();
    const added = page.locator('#rows .row').filter({ has: page.locator('.tag') });
    await expect(added, 'the head added one row').toHaveCount(1);
    await expect(added.locator('[data-act=pick-place], [data-act=pick-layer]'), 'the new row carries no place switch').toHaveCount(0);
    // The screen puts the cursor in the name once the check has answered.
    await expect(added.locator('.in-name'), 'the new row offers its name').toBeFocused();
    await added.locator('.in-name').fill('place-g1');
    const row = page.locator('#rows .row').filter({ has: page.locator('.c-name .nm', { hasText: 'place-g1' }) });
    await row.locator('.panel [data-act=save]').click();
    await expect(row.locator('.ok'), 'the save answered').toBeVisible();

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
    const file = path.join(app.proj, '.hergy', 'roles', 'place-p3.md');
    const global = path.join(app.cfg, 'hergy', 'roles', 'place-p3.md');
    await page.goto(`${app.base}/roles`);
    await wait(page, there(ROLES));
    const beforeProject = await read<number>(page, countIn(ROLES, 'project'));
    const beforeGlobal = await read<number>(page, countIn(ROLES, 'global'));
    await readTo<boolean>(page, `!document.querySelector('.toolbar [data-act=new]')`, 'the list above has no add button').toBe(true);
    await readTo<number>(page, headCount(), 'the list draws a head for every listed project and the global layer').toBe(4);
    const said = await read<string>(page, `${headOf('project')}.querySelector('.hq-gn').textContent`, 'the first project head name');
    await readTo<string>(page, `${headOf('project')}.querySelector('[data-act=new]').textContent`, 'the project button says the place').toContain(said);

    // The row is found by the 新規 mark it carries until its save (PLACE-R1).
    const head = page.locator('#rows .hq-gh[data-place="project"]').first();
    await head.getByRole('button', { name: 'にロールを追加' }).click();
    const added = page.locator('#rows .role').filter({ has: page.locator('.tag') });
    await expect(added, 'the head added one row').toHaveCount(1);
    await expect(added.locator('[data-act=pick-place], [data-act=pick-layer]'), 'the new row carries no place switch').toHaveCount(0);
    // The screen puts the cursor in the name once the check has answered; typing
    // before that would go into the name when the cursor moves under it.
    await expect(added.locator('[data-f=name]'), 'the new row offers its name').toBeFocused();
    await added.locator('[data-f=name]').fill('place-p3');
    const row = page.locator('#rows .role').filter({ has: page.locator('.c-name .nm', { hasText: 'place-p3' }) });
    await row.getByRole('textbox', { name: '規則の全文' }).fill('本部のプロジェクトの見出しから足したロール。');
    await row.locator('.panel [data-act=save]').click();
    await expect(row.locator('.ok'), 'the save answered').toBeVisible();

    await readTo<number>(page, countIn(ROLES, 'project'), 'the project head holds the added role').toBe(beforeProject + 1);
    await readTo<number>(page, countIn(ROLES, 'global'), 'the global head keeps its roles').toBe(beforeGlobal);
    expect(existsSync(file), 'the save wrote the listed project file').toBe(true);
    expect(existsSync(global), 'the save wrote no global file').toBe(false);
    await page.reload();
    await wait(page, there(ROLES));
    await readTo<number>(page, countIn(ROLES, 'project'), 'the added role stands under the project after opening the page again').toBe(beforeProject + 1);
  });

  test('PLACE-R4 a role added under the global head', async ({ page, app }) => {
    const file = path.join(app.cfg, 'hergy', 'roles', 'place-g4.md');
    const project = path.join(app.proj, '.hergy', 'roles', 'place-g4.md');
    await page.goto(`${app.base}/roles`);
    await wait(page, there(ROLES));
    const beforeProject = await read<number>(page, countIn(ROLES, 'project'));
    const beforeGlobal = await read<number>(page, countIn(ROLES, 'global'));
    await readTo<string>(page, `${headOf('global')}.querySelector('[data-act=new]').textContent`, 'the global button says the place').toContain('グローバル');

    // The row is found by the 新規 mark it carries until its save (PLACE-R1).
    const head = page.locator('#rows .hq-gh[data-place="global"]');
    await head.getByRole('button', { name: 'にロールを追加' }).click();
    const added = page.locator('#rows .role').filter({ has: page.locator('.tag') });
    await expect(added, 'the head added one row').toHaveCount(1);
    await expect(added.locator('[data-act=pick-place], [data-act=pick-layer]'), 'the new row carries no place switch').toHaveCount(0);
    // The screen puts the cursor in the name once the check has answered.
    await expect(added.locator('[data-f=name]'), 'the new row offers its name').toBeFocused();
    await added.locator('[data-f=name]').fill('place-g4');
    const row = page.locator('#rows .role').filter({ has: page.locator('.c-name .nm', { hasText: 'place-g4' }) });
    await row.getByRole('textbox', { name: '規則の全文' }).fill('本部のグローバルの見出しから足したロール。');
    await row.locator('.panel [data-act=save]').click();
    await expect(row.locator('.ok'), 'the save answered').toBeVisible();

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

    // The row is found by the 新規 mark it carries until its save (PLACE-R1).
    const head = page.locator('#rows .hq-gh[data-place="project"]').first();
    await head.getByRole('button', { name: 'にプロファイルを追加' }).click();
    const added = page.locator('#rows .row').filter({ has: page.locator('.tag') });
    await expect(added, 'the head added one row').toHaveCount(1);
    await expect(added.locator('[data-act=pick-place], [data-act=pick-layer]'), 'the new row carries no place switch').toHaveCount(0);
    // The screen puts the cursor in the name once the check has answered.
    await expect(added.locator('.in-name'), 'the new row offers its name').toBeFocused();
    await added.locator('.in-name').fill('place-p3');
    const row = page.locator('#rows .row').filter({ has: page.locator('.c-name .nm', { hasText: 'place-p3' }) });
    await row.locator('.panel [data-act=save]').click();
    await expect(row.locator('.ok'), 'the save answered').toBeVisible();

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

    // The row is found by the 新規 mark it carries until its save (PLACE-R1).
    const head = page.locator('#rows .hq-gh[data-place="global"]');
    await head.getByRole('button', { name: 'にプロファイルを追加' }).click();
    const added = page.locator('#rows .row').filter({ has: page.locator('.tag') });
    await expect(added, 'the head added one row').toHaveCount(1);
    await expect(added.locator('[data-act=pick-place], [data-act=pick-layer]'), 'the new row carries no place switch').toHaveCount(0);
    // The screen puts the cursor in the name once the check has answered.
    await expect(added.locator('.in-name'), 'the new row offers its name').toBeFocused();
    await added.locator('.in-name').fill('place-g4');
    const row = page.locator('#rows .row').filter({ has: page.locator('.c-name .nm', { hasText: 'place-g4' }) });
    await row.locator('.panel [data-act=save]').click();
    await expect(row.locator('.ok'), 'the save answered').toBeVisible();

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

    // The row is found by the 新規 mark it carries until its save (PLACE-R1).
    const head = page.locator('#view .list .hq-gh[data-place="project"]');
    await head.getByRole('button', { name: 'にチーム定義を追加' }).click();
    const added = page.locator('#view .list .tdef').filter({ has: page.locator('.tag') });
    await expect(added, 'the head added one row').toHaveCount(1);
    await expect(added.locator('[data-act=pick-place], [data-act=pick-layer]'), 'the new row carries no place switch').toHaveCount(0);
    await expect(added.locator('.pill.pending'), 'the new row says the project').toHaveText(said);
    // The screen puts the cursor in the name once the check has answered.
    await expect(added.locator('.in-name'), 'the new row offers its name').toBeFocused();
    await added.locator('.in-name').fill('place-t1');
    await added.locator('.detail [data-act=t-save]').click();
    // A definition being named shows its name in the field, not in .nm, so the
    // row is found by its name once the save has drawn it.
    const row = page.locator('#view .list .tdef').filter({ has: page.locator('.c-name .nm', { hasText: 'place-t1' }) });
    await expect(row.locator('.ok'), 'the save answered').toBeVisible();

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

    // The row is found by the 新規 mark it carries until its save (PLACE-R1).
    const head = page.locator('#view .list .hq-gh[data-place="global"]');
    await head.getByRole('button', { name: 'にチーム定義を追加' }).click();
    const added = page.locator('#view .list .tdef').filter({ has: page.locator('.tag') });
    await expect(added, 'the head added one row').toHaveCount(1);
    await expect(added.locator('[data-act=pick-place], [data-act=pick-layer]'), 'the new row carries no place switch').toHaveCount(0);
    // The screen puts the cursor in the name once the check has answered.
    await expect(added.locator('.in-name'), 'the new row offers its name').toBeFocused();
    await added.locator('.in-name').fill('place-t2');
    await added.locator('.detail [data-act=t-save]').click();
    const row = page.locator('#view .list .tdef').filter({ has: page.locator('.c-name .nm', { hasText: 'place-t2' }) });
    await expect(row.locator('.ok'), 'the save answered').toBeVisible();

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

    // The row is found by the 新規 mark it carries until its save (PLACE-R1).
    const head = page.locator('#view .list .hq-gh[data-place="project"]').first();
    await head.getByRole('button', { name: 'にチーム定義を追加' }).click();
    const added = page.locator('#view .list .tdef').filter({ has: page.locator('.tag') });
    await expect(added, 'the head added one row').toHaveCount(1);
    await expect(added.locator('[data-act=pick-place], [data-act=pick-layer]'), 'the new row carries no place switch').toHaveCount(0);
    // The screen puts the cursor in the name once the check has answered.
    await expect(added.locator('.in-name'), 'the new row offers its name').toBeFocused();
    await added.locator('.in-name').fill('place-t3');
    await added.locator('.detail [data-act=t-save]').click();
    const row = page.locator('#view .list .tdef').filter({ has: page.locator('.c-name .nm', { hasText: 'place-t3' }) });
    await expect(row.locator('.ok'), 'the save answered').toBeVisible();

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

    // The row is found by the 新規 mark it carries until its save (PLACE-R1).
    const head = page.locator('#view .list .hq-gh[data-place="global"]');
    await head.getByRole('button', { name: 'にチーム定義を追加' }).click();
    const added = page.locator('#view .list .tdef').filter({ has: page.locator('.tag') });
    await expect(added, 'the head added one row').toHaveCount(1);
    await expect(added.locator('[data-act=pick-place], [data-act=pick-layer]'), 'the new row carries no place switch').toHaveCount(0);
    // The screen puts the cursor in the name once the check has answered.
    await expect(added.locator('.in-name'), 'the new row offers its name').toBeFocused();
    await added.locator('.in-name').fill('place-t4');
    await added.locator('.detail [data-act=t-save]').click();
    const row = page.locator('#view .list .tdef').filter({ has: page.locator('.c-name .nm', { hasText: 'place-t4' }) });
    await expect(row.locator('.ok'), 'the save answered').toBeVisible();

    await readTo<number>(page, countIn(TEAMS, 'global'), 'the global head holds the added definition').toBe(beforeGlobal + 1);
    await readTo<number>(page, countIn(TEAMS, 'project'), 'the project head keeps its definitions').toBe(beforeProject);
    expect(existsSync(file), 'the save wrote the global file').toBe(true);
    expect(existsSync(project), 'the save wrote no project file').toBe(false);
    await page.reload();
    await wait(page, there(TEAMS));
    await readTo<number>(page, countIn(TEAMS, 'global'), 'the added definition stands under the global layer after opening the page again').toBe(beforeGlobal + 1);
  });
});
