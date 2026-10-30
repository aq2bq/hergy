// HQ-WRITE-P1 — r-57d8 (ac-e1d4), walked in a browser: a write under a listed
// project's head carries that project's own file's hash.
//
// The screen holds one profiles.yaml (or role, or team.yaml) per group (d-73fc),
// and a save in this serve's own group used to put its file's hash under every
// project's panel (profiles/state.ts's applySaved, roles/state.ts's and
// teams/state.ts's rebase): the next delete of another project's profile was
// then refused as 「ほかのところで書き換えられました」 (n-7e5e: the person deleted
// gy's profile while serve ran in hergy, after hergy's own file had moved).
//
// So every walk here writes in this serve's project first, on the same page,
// then works under the listed project's head: a delete and a save that the page
// must not refuse. The two projects' files hold different words before the walk
// (the measurement's own setup), so one project's hash is never the other's, and
// the served project's files are read before and after the listed project's work:
// the second reading is the first, byte for byte. The first walk then rewrites
// the listed project's file outside the page and reads the refusal that is still
// the screen's: a file that really moved is not written over.
import { test, expect } from './fixtures';
import { existsSync, mkdirSync, readFileSync, realpathSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { press, q, readTo, set, there, wait } from './helpers';

test.use({ reads: 'hq' });

const PROFILES = '#rows .row';
const ROLES = '#rows .role';
const TEAMS = '.tdef';
const ROWS = '#rows';
const LIST = '#view .list';
const ALPHA = 'alpha';
const BETA = 'beta';

// projects are the two places the walk stands between: this serve's project,
// which the serve reads as its own, and the listed one under its head. The
// fixture's home is a temporary directory, and the serve names it through the
// project's own root (git), so the walk reads the path the page carries.
function projects(app: { home: string }) {
  const home = realpathSync(app.home);
  const alpha = path.join(home, 'src', ALPHA);
  const beta = path.join(home, 'src', BETA);
  return { alpha, beta };
}

// rowsIn is the rows one place's head stands over: the list draws a head and the
// rows under it until the next head.
const rowsIn = (root: string, sel: string, group: string) => `(() => {
  const out = [];
  let on = null;
  const list = document.querySelector(${JSON.stringify(root)});
  if (!list) return out;
  for (const el of list.children) {
    if (el.classList.contains('hq-gh')) {
      on = el.dataset.group || '';
      continue;
    }
    if (on === ${JSON.stringify(group)} && el.matches(${JSON.stringify(sel)})) out.push(el);
  }
  return out;
})()`;

// rowIn is one row under one place's head, found by the name it shows: the name
// a saved row carries, or the field a row a person is adding holds.
const rowIn = (root: string, sel: string, group: string, name: string) => `(${rowsIn(root, sel, group)}.find((el) => {
  if (el.dataset.name === ${JSON.stringify(name)}) return true;
  const n = el.querySelector('.c-name .nm');
  if (n && n.textContent.trim() === ${JSON.stringify(name)}) return true;
  const f = el.querySelector('.c-name .in-name');
  return !!f && f.value === ${JSON.stringify(name)};
}) || null)`;

// drawn is whether one element stands on the page: the expression the walks
// build from a row's own place rather than a selector.
const drawn = (find: string) => `!!${find}`;

// refusalIn is the sentence a refused write leaves on the row it was pressed in.
const refusalIn = (row: string) => `(${row} && [...${row}.querySelectorAll('.refusal')].map((el) => el.textContent.trim()).join(' ')) || ''`;

const read = (file: string) => readFileSync(file, 'utf8');

// listedFiles writes the listed project's three files with words its own: the
// fixture's roles are the same document in every project, and a hash two files
// share hides the mistake this walk is about.
function listedFiles(beta: string): { profiles: string; role: string; team: string } {
  const dir = path.join(beta, '.hergy');
  mkdirSync(path.join(dir, 'roles'), { recursive: true });
  mkdirSync(path.join(dir, 'teams', `${BETA}-dev`), { recursive: true });
  const profiles = path.join(dir, 'profiles.yaml');
  const role = path.join(dir, 'roles', 'builder.yaml');
  const team = path.join(dir, 'teams', `${BETA}-dev`, 'team.yaml');
  writeFileSync(profiles, 'version: 1\nprofiles:\n  beta-fast:\n    kind: claude\n    args: ["--model", "claude-haiku-5-5"]\n  beta-slow:\n    kind: codex\n    args: ["-m", "gpt-5-5"]\n');
  writeFileSync(role, 'version: 1\ntitle: 見積もりを組む\nsections:\n  - heading: 作り方\n    rules:\n      - id: DBAA\n        text: 見積もりは、前提を 1 つ書いてから出す。\n');
  writeFileSync(team, 'version: 1\ndescription: "見積もりの計算をそろえる"\nmembers:\n  - name: lead\n    role: lead\n    profile: beta-fast\n  - name: builder\n    role: builder\n    profile: beta-fast\nturns:\n  - human -> lead\n  - lead -> builder\n  - builder -> human\n');
  return { profiles, role, team };
}

// servedFiles are this serve's project's files, read as they stand.
function servedFiles(alpha: string) {
  const dir = path.join(alpha, '.hergy');
  return {
    profiles: path.join(dir, 'profiles.yaml'),
    role: path.join(dir, 'roles', 'builder.yaml'),
    team: path.join(dir, 'teams', 'alpha-dev', 'team.yaml'),
  };
}

test('HQ-WRITE-P1 a listed project’s profile is deleted and saved with its own file’s hash', async ({ page, app }) => {
  const { alpha, beta: betaDir } = projects(app);
  const beta = listedFiles(betaDir);
  const served = servedFiles(alpha);

  await page.goto(`${app.base}/profiles`);
  await wait(page, drawn(rowIn(ROWS, PROFILES, alpha, 'alpha-fast')));

  // This serve's own project moves first: its delete answers with the file it
  // wrote, and that answer is the served project's alone.
  await press(page, `${rowIn(ROWS, PROFILES, alpha, 'alpha-fast')}.querySelector('.sum')`);
  await wait(page, drawn(`${rowIn(ROWS, PROFILES, alpha, 'alpha-fast')}.querySelector('.panel[data-file=project] [data-act=del]')`));
  await press(page, `${rowIn(ROWS, PROFILES, alpha, 'alpha-fast')}.querySelector('.panel[data-file=project] [data-act=del]')`);
  await press(page, `${rowIn(ROWS, PROFILES, alpha, 'alpha-fast')}.querySelector('.panel[data-file=project] [data-act=del-yes]')`);
  await wait(page, `!${rowIn(ROWS, PROFILES, alpha, 'alpha-fast')}`);
  const before = { profiles: read(served.profiles), role: read(served.role), team: read(served.team) };

  // The listed project's own profile goes: not refused, and its file no longer
  // holds it.
  await press(page, `${rowIn(ROWS, PROFILES, betaDir, 'beta-fast')}.querySelector('.sum')`);
  await wait(page, drawn(`${rowIn(ROWS, PROFILES, betaDir, 'beta-fast')}.querySelector('.panel[data-file=project] [data-act=del]')`));
  await press(page, `${rowIn(ROWS, PROFILES, betaDir, 'beta-fast')}.querySelector('.panel[data-file=project] [data-act=del]')`);
  await press(page, `${rowIn(ROWS, PROFILES, betaDir, 'beta-fast')}.querySelector('.panel[data-file=project] [data-act=del-yes]')`);
  await wait(page, `!${rowIn(ROWS, PROFILES, betaDir, 'beta-fast')}`);
  expect(read(beta.profiles), 'the listed project’s file still holds the deleted profile').not.toContain('beta-fast:');

  // A profile a person adds under the listed project's head stands on that
  // project's file, which no answer for this serve's project had read. The row
  // a person is adding is the one panel the answer has not written yet.
  const fresh = `(() => { const p = document.querySelector('#rows .row .panel[data-file=project]:not([data-saved])'); return p ? p.closest('.row') : null; })()`;
  await press(page, q(`.hq-gh[data-group="${betaDir}"] [data-act=new]`));
  await wait(page, `!!${fresh}`);
  await set(page, `${fresh}.querySelector('.panel[data-file=project] [data-f=name]')`, 'beta-new');
  await wait(page, `!${fresh}.querySelector('.panel[data-file=project] [data-act=save]').disabled`);
  await press(page, `${fresh}.querySelector('.panel[data-file=project] [data-act=save]')`);
  await wait(page, drawn(`${rowIn(ROWS, PROFILES, betaDir, 'beta-new')}.querySelector('.panel[data-file=project] .ok')`));
  const written = read(beta.profiles);
  expect(written, 'the listed project’s file does not hold the profile the walk added').toContain('beta-new:');
  expect(written, 'the delete and the save lost the profile the file already held').toContain('beta-slow:');

  // The served project's files are the ones the walk left them: no hash of its
  // own carried the listed project's write.
  expect(read(served.profiles), 'the served project’s profiles.yaml moved').toBe(before.profiles);
  expect(read(served.role), 'the served project’s role moved').toBe(before.role);
  expect(read(served.team), 'the served project’s team.yaml moved').toBe(before.team);

  // A file that really moved outside the page is still refused: the panel sends
  // the hash it read, and the answer names the file as it is now.
  writeFileSync(beta.profiles, `${written}  beta-outside:\n    kind: codex\n`);
  await press(page, `${rowIn(ROWS, PROFILES, betaDir, 'beta-new')}.querySelector('.panel[data-file=project] [data-act=del]')`);
  await press(page, `${rowIn(ROWS, PROFILES, betaDir, 'beta-new')}.querySelector('.panel[data-file=project] [data-act=del-yes]')`);
  await readTo<string>(
    page,
    refusalIn(rowIn(ROWS, PROFILES, betaDir, 'beta-new')),
    'a file the page did not read is written over, want the refusal',
  ).toContain('書き換えられました');
  expect(read(beta.profiles), 'the refused delete took the profile away').toContain('beta-new:');
});

test('HQ-WRITE-P1 a listed project’s role is saved and deleted with its own file’s hash', async ({ page, app }) => {
  const { alpha, beta: betaDir } = projects(app);
  const beta = listedFiles(betaDir);
  const served = servedFiles(alpha);

  await page.goto(`${app.base}/roles`);
  await wait(page, drawn(rowIn(ROWS, ROLES, alpha, 'builder')));

  // This serve's own role moves first: the save answers with its project's
  // files, and the listed project's role of the same name keeps its own hash.
  await press(page, `${rowIn(ROWS, ROLES, alpha, 'builder')}.querySelector('.sum')`);
  await wait(page, drawn(`${rowIn(ROWS, ROLES, alpha, 'builder')}.querySelector('.panel .rule')`));
  await press(page, `${rowIn(ROWS, ROLES, alpha, 'builder')}.querySelector('.rule .rsum')`);
  await set(page, `${rowIn(ROWS, ROLES, alpha, 'builder')}.querySelector('.rule.open .redit [data-f=note]')`, 'served のメモ。');
  await press(page, `${rowIn(ROWS, ROLES, alpha, 'builder')}.querySelector('.panel .pf [data-act=save]')`);
  await wait(page, drawn(`${rowIn(ROWS, ROLES, alpha, 'builder')}.querySelector('.panel .ok')`));
  const before = { profiles: read(served.profiles), role: read(served.role), team: read(served.team) };

  // The listed project's role saves, then goes: neither is refused.
  await press(page, `${rowIn(ROWS, ROLES, betaDir, 'builder')}.querySelector('.sum')`);
  await wait(page, drawn(`${rowIn(ROWS, ROLES, betaDir, 'builder')}.querySelector('.panel .rule')`));
  await press(page, `${rowIn(ROWS, ROLES, betaDir, 'builder')}.querySelector('.rule .rsum')`);
  await set(page, `${rowIn(ROWS, ROLES, betaDir, 'builder')}.querySelector('.rule.open .redit [data-f=note]')`, 'listed のメモ。');
  await press(page, `${rowIn(ROWS, ROLES, betaDir, 'builder')}.querySelector('.panel .pf [data-act=save]')`);
  await wait(page, drawn(`${rowIn(ROWS, ROLES, betaDir, 'builder')}.querySelector('.panel .ok')`));
  expect(read(beta.role), 'the listed project’s role does not hold the memo the save wrote').toContain('listed のメモ。');

  await press(page, `${rowIn(ROWS, ROLES, betaDir, 'builder')}.querySelector('.panel .pf [data-act=del]')`);
  await press(page, `${rowIn(ROWS, ROLES, betaDir, 'builder')}.querySelector('.panel .pf [data-act=del-yes]')`);
  await wait(page, `!${rowIn(ROWS, ROLES, betaDir, 'builder')}`);
  expect(existsSync(beta.role), 'the listed project’s role file is still there after the delete').toBe(false);

  expect(read(served.profiles), 'the served project’s profiles.yaml moved').toBe(before.profiles);
  expect(read(served.role), 'the served project’s role moved').toBe(before.role);
  expect(read(served.team), 'the served project’s team.yaml moved').toBe(before.team);
});

test('HQ-WRITE-P1 a listed project’s team definition is saved and deleted with its own file’s hash', async ({ page, app }) => {
  const { alpha, beta: betaDir } = projects(app);
  listedFiles(betaDir);
  const served = servedFiles(alpha);
  // A definition of the same name stands in both projects: its files are each
  // project's own, and a save of one is not a save of the other.
  const shared = path.join(betaDir, '.hergy', 'teams', 'alpha-dev', 'team.yaml');
  mkdirSync(path.dirname(shared), { recursive: true });
  writeFileSync(shared, 'version: 1\ndescription: "beta 側の同じ名前"\nmembers:\n  - name: lead\n    role: lead\n    profile: beta-fast\nturns:\n  - human -> lead\n  - lead -> human\n');

  // The list names a definition by its name and the project it stands in.
  const alphaName = `alpha-dev|${alpha}`;
  const betaName = `alpha-dev|${betaDir}`;

  await page.goto(`${app.base}/teams`);
  await wait(page, drawn(rowIn(LIST, TEAMS, alpha, alphaName)));

  // This serve's own definition moves first.
  await press(page, `${rowIn(LIST, TEAMS, alpha, alphaName)}.querySelector('.sum')`);
  await wait(page, drawn(`${rowIn(LIST, TEAMS, alpha, alphaName)}.querySelector('.in-description')`));
  await set(page, `${rowIn(LIST, TEAMS, alpha, alphaName)}.querySelector('.in-description')`, 'served の説明。');
  await press(page, `${rowIn(LIST, TEAMS, alpha, alphaName)}.querySelector('.panel.team-p [data-act=t-save]')`);
  await wait(page, drawn(`${rowIn(LIST, TEAMS, alpha, alphaName)}.querySelector('.panel.team-p .ok')`));
  const before = { profiles: read(served.profiles), role: read(served.role), team: read(served.team) };

  // The listed project's definition of the same name saves with its own file's
  // hash and then goes.
  await press(page, `${rowIn(LIST, TEAMS, betaDir, betaName)}.querySelector('.sum')`);
  await wait(page, drawn(`${rowIn(LIST, TEAMS, betaDir, betaName)}.querySelector('.in-description')`));
  await set(page, `${rowIn(LIST, TEAMS, betaDir, betaName)}.querySelector('.in-description')`, 'listed の説明。');
  await press(page, `${rowIn(LIST, TEAMS, betaDir, betaName)}.querySelector('.panel.team-p [data-act=t-save]')`);
  await wait(page, drawn(`${rowIn(LIST, TEAMS, betaDir, betaName)}.querySelector('.panel.team-p .ok')`));
  expect(read(shared), 'the listed project’s team.yaml does not hold the description the save wrote').toContain('listed の説明。');

  await press(page, `${rowIn(LIST, TEAMS, betaDir, betaName)}.querySelector('nav.dtabs .dacts [data-act=del]')`);
  await press(page, `${rowIn(LIST, TEAMS, betaDir, betaName)}.querySelector('nav.dtabs .dacts [data-act=del-yes]')`);
  await wait(page, `!${rowIn(LIST, TEAMS, betaDir, betaName)}`);
  expect(existsSync(shared), 'the listed project’s team.yaml is still there after the delete').toBe(false);

  expect(read(served.profiles), 'the served project’s profiles.yaml moved').toBe(before.profiles);
  expect(read(served.role), 'the served project’s role moved').toBe(before.role);
  expect(read(served.team), 'the served project’s team.yaml moved').toBe(before.team);
});
