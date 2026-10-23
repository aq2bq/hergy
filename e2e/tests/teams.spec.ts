// TEAMS-N1, TEAMS-P5, TEAMS-Y2 and TEAMS-V1 — walked
// in a browser. Only what the browser alone can see is compared here (d-3e43):
// the path the shape panel's head writes (TEAMS-V4), 元に戻す standing where
// nothing changed (R5), a definition whose names do not resolve being drawn (R10),
// the words the cards are built with (undefined), the cursor while a field is
// drawn again, a rule opening inside the window (R12), the one line a save writes
// from the screen's own assembly, the line a change would take away standing once
// (R6), the preview against the file (Y6), the box sent to the line a turn added
// (R2), the band following a save and a delete (R7), and the files a 共通 save
// writes and does not write.
import { test, expect, hergyNew } from './fixtures';
import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { DEADLINE, choose, diff, press, q, read, readTo, set, there, wait } from './helpers';

test.use({ reads: 'teams' });

// tRow is one list row, by the definition's name (e2eTeamsRowJS).
const tRow = (name: string) =>
  `[...document.querySelectorAll('#view .tdef')].find(function(r){return r.dataset.name===${JSON.stringify(name)}})`;
// shape is one row's shape panel, and agree its agreement panel.
const shape = (name: string) => q(`.tdef[data-name="${name}"] .panel.team-p`);
const agree = (name: string) => q(`.tdef[data-name="${name}"] .panel.agree-p`);

// draft is the row a person is making: a new definition stands under a name
// beginning ~new until it is saved.
const draft = q('.tdef[data-name^="~new"]');

// there2 is whether one element stands inside one panel expression.
const there2 = (panel: string, find: string) => `!!${panel}.querySelector(${JSON.stringify(find)})`;

// open walks what only a browser shows of a definition whose names do not
// resolve (P1, S5, V1, P9, T1, R3〜R5, R10): the row opens in place, the shape
// panel's head carries the file's path as a person reads it (TEAMS-V4), the
// agreement panel offers no 元に戻す with nothing changed (R5), and docs — a
// profile its worker names that profiles.yaml does not hold — is drawn like one
// that reads, with its one problem on the worker it names (R10).
async function open(page: Parameters<typeof read>[0], base: string): Promise<void> {
  await page.goto(`${base}/teams`);
  await wait(page, there('#view .tdef'));

  // P1: pressing the row opens it right under itself.
  await press(page, `${tRow('core')}.querySelector('.sum')`);
  await wait(page, `${tRow('core')}.classList.contains('open')`);

  // TEAMS-V4: the head is the path alone, as the project reads it — not the
  // working directory's absolute path.
  const panel = shape('core');
  await readTo<string>(
    page,
    `${panel}.querySelector('.ph .path').textContent`,
    "the shape panel's path reads, want the project's own form",
  ).toBe('.hergy/teams/core/team.yaml');

  // R5: nothing has changed, so 元に戻す is not offered.
  await press(page, `${tRow('core')}.querySelector('.dtabs .dtab[data-t=agree]')`);
  await wait(page, `!!${agree('core')}`);
  await readTo<string>(
    page,
    `${agree('core')}.querySelector('.pf [data-act=revert]').hidden+''`,
    "the agreement's foot offers 元に戻す with nothing changed",
  ).toBe('true');

  // 1.3・1.4.5・1.9・R10: a definition whose names do not resolve is drawn like
  // one that reads, and its one problem stands on the worker it names.
  const docs = shape('docs');
  await press(page, `${tRow('docs')}.querySelector('.sum')`);
  await wait(page, there2(docs, '.vlist .vitem .msg'));
  await readTo<number>(
    page,
    `${docs}.querySelectorAll('.cv-canvas svg, footer.pf [data-act=t-save]').length`,
    'the definition whose profile does not resolve draws figures and saves, want the figure and the save',
  ).toBe(2);
  // 1.4.1・R10: the problem stands on the worker 翻訳's own point, with 問題.
  await readTo<string>(
    page,
    `[...document.querySelectorAll('.tdef[data-name="docs"] .panel.team-p .cv-n.worker.err')].map(function(g){return g.dataset.cvW}).join(',')`,
    'the workers in problem read, want the 翻訳 worker alone',
  ).toBe('0');
}

test('TEAMS-V1 a definition whose names do not resolve', async ({ page, app }) => {
  await open(page, app.base);
});

// workerPurpose is e2eTeamsWorkerPurpose's browser-only part: the worker a
// person hires takes its purpose one letter at a time without the cursor leaving
// the field, the card writes no word the check did not give it, and the letters
// add one worker rather than one a letter (X3, 1.4.6).
async function workerPurpose(page: Parameters<typeof read>[0], base: string): Promise<void> {
  await page.goto(`${base}/teams?open=core`);
  await wait(page, there('.tdef[data-name="core"] .panel.team-p .cv-canvas svg'));
  const panel = shape('core');
  const field = `${panel}.querySelector('.cv-side [data-cv-f=w-purpose]')`;

  await press(page, `${panel}.querySelector('.cv-n.member[data-cv-n=lead]')`);
  await wait(page, there2(panel, '.cv-side [data-cv-f=m-name]'));
  await press(page, `${panel}.querySelector('[data-cv-act=w-add]')`);
  await wait(page, `!!${field}`);
  // A worker with no purpose is a problem the saved file did not hold, so the
  // save is shut until the purpose is there (X3).
  await wait(page, `${panel}.querySelector('[data-act=t-save]').disabled`);
  const vm = `[...document.querySelectorAll('.tdef[data-name="core"] .panel.team-p .cv-left .vlist .vitem')].map(function(li){return li.textContent}).join('|')`;
  await readTo<string>(
    page,
    vm,
    "the new worker's validate reads, want no word the check did not give it",
  ).not.toContain('undefined');
  // One letter at a time, the way a person types: the field is not drawn again,
  // so the cursor stays and the word collects.
  const purpose = 'ログの整理';
  const letters = [...purpose];
  for (let i = 1; i <= letters.length; i++) {
    await set(page, field, letters.slice(0, i).join(''));
    await wait(page, `(document.activeElement===${field})`);
    await readTo<string>(
      page,
      `${field}.value`,
      `after ${i} letters the purpose reads, want ${letters.slice(0, i).join('')}`,
    ).toBe(letters.slice(0, i).join(''));
  }
  // One worker, however many letters wrote its purpose (1.4.6).
  await readTo<string>(
    page,
    `${panel}.querySelector('footer.pf .dirty').textContent.trim()`,
    "the worker's foot reads, want 1 か所",
  ).toBe('1 か所');
}

// graphProblems is e2eTeamsGraphProblems' browser-only part: a warning about the
// team's own agreement opens that rule in the ルール tab, inside the window a
// person reads on, with the cursor in its body (1.4.5, 1.7, R9・R12).
async function graphProblems(page: Parameters<typeof read>[0], base: string): Promise<void> {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto(`${base}/teams?open=research`);
  await wait(page, there('.tdef[data-name="research"] .panel.team-p .cv-canvas svg'));
  const rp = shape('research');
  // scout's turn to the human goes, so the team's own agreement's rule RES-6
  // warns.
  await press(page, `${rp}.querySelector('.cv-n.member[data-cv-n=scout]')`);
  await wait(page, there2(rp, '.cv-side [data-cv-f=m-name]'));
  await press(page, `${rp}.querySelector('.cv-side [data-cv-act=turn-del][data-a=scout][data-b=human]')`);
  await wait(page, `!!document.querySelector('.cv-toast')`);
  const item = `[...document.querySelectorAll('.tdef[data-name="research"] .panel.team-p .vlist .vitem')].find(function(b){return b.textContent.indexOf('RES-6')>=0})`;
  await wait(page, `!!${item}`);
  await press(page, item);
  await wait(page, there2(agree('research'), '.rule.open'));

  // R12: the rule the press opened stands in the window — its head below the
  // top, its body above the save's band — and its body has the cursor (1.7).
  const above = `(function(){var p=${agree('research')};var r=p.querySelector('.rule.open'),t=p.querySelector('.rule.open [data-f=text]'),f=p.querySelector('footer.pf');if(!r||!t||!f)return false;var rr=r.getBoundingClientRect(),tr=t.getBoundingClientRect(),fr=f.getBoundingClientRect();return rr.top>=0&&tr.top>rr.top&&tr.bottom<=fr.top})()`;
  const cursor = `(document.activeElement&&document.activeElement.dataset.f)==='text'`;
  await readTo<boolean>(
    page,
    above,
    'the rule the press opened does not stand in the window',
  ).toBe(true);
  await readTo<boolean>(
    page,
    cursor,
    'the rule the press opened does not hold the cursor in its body',
  ).toBe(true);
}

// saveShape is e2eTeamsSaveShape's browser-only part: the mission alone moves and
// the check rides on the typing, and one turn goes with its card's own words —
// the line the change would take away standing once (R6), and the save writing
// that one line (F1・F2, P2・P6・P7).
async function saveShape(page: Parameters<typeof read>[0], app: { base: string; proj: string }): Promise<void> {
  const file = path.join(app.proj, '.hergy', 'teams', 'core', 'team.yaml');
  let before = readFileSync(file, 'utf8');

  // F2: the mission alone moves, and the check rides on the typing (P2).
  await page.goto(`${app.base}/teams?open=core`);
  await wait(page, there('.tdef[data-name="core"] .panel.team-p footer.pf [data-act=t-save]'));
  const mission = 'E2E が書き換えた使命。';
  await set(page, q('.in-mission'), mission);
  await wait(
    page,
    `document.querySelectorAll('.tdef[data-name="core"] .dtabs .dtab[data-t=shape] .dot').length>0`,
  );
  await wait(page, `${shape('core')}.querySelector('footer.pf .dirty').textContent.trim()==='1 か所'`);
  await press(page, `${shape('core')}.querySelector('[data-act=t-save]')`);
  await wait(page, there2(shape('core'), '.ok'));
  let { added, removed } = diff(before, readFileSync(file, 'utf8'));
  expect(
    added.length === 1 && removed.length === 1 && added[0] === `mission: "${mission}"` && removed[0].startsWith('mission: '),
    `the mission save wrote ${JSON.stringify(added)} and lost ${JSON.stringify(removed)}, want the mission line alone`,
  ).toBe(true);
  before = readFileSync(file, 'utf8');

  // F1・P6・P7: one turn goes, and R6: the line the change would take away
  // stands once, in the server's own words.
  const panel = shape('core');
  await press(page, `${panel}.querySelector('.cv-n.member[data-cv-n=impl-b]')`);
  await wait(page, there2(panel, '.cv-side [data-cv-f=m-name]'));
  await press(page, `${panel}.querySelector('.cv-side [data-cv-act=turn-del][data-a=impl-b][data-b=reviewer]')`);
  await wait(page, `!!document.querySelector('.cv-toast')`);
  // 外したことは知らせで言い、そこから戻せる (the mock's toast).
  await readTo<string>(
    page,
    `document.querySelector('.cv-toast').textContent`,
    'the removal notice reads',
  ).toContain('impl-b → reviewer のターンを外した');
  await wait(page, `${panel}.querySelector('footer.pf .dirty').textContent.trim()==='1 か所'`);
  await press(page, `${panel}.querySelector('[data-act=t-save]')`);
  await wait(page, there2(panel, '.ok'));
  ({ added, removed } = diff(before, readFileSync(file, 'utf8')));
  expect(
    added.length === 0 && removed.length === 1 && removed[0].trim() === '- impl-b -> reviewer',
    `the turn save wrote ${JSON.stringify(added)} and lost ${JSON.stringify(removed)}, want the turn's own line alone`,
  ).toBe(true);
}

test('TEAMS-P5 the definition a person edits', async ({ page, app }) => {
  // The edit walk's save writes the file, so the Go run stood it in a project of
  // its own; here the test's own project is that project.
  await workerPurpose(page, app.base);
  await graphProblems(page, app.base);
  await saveShape(page, app);
});

test('TEAMS-N1 add, the shared place and delete', async ({ page, app }) => {
  // B1: a new definition saved from the ルール tab (監査 R9), the starter
  // profile added to the project's profiles.yaml with one edit (R3), the band
  // following the save (R7), the shared place's role documents written and its
  // profiles.yaml not written, 取り消し writing nothing (N8), and a delete taking
  // one definition away (N7).
  await page.goto(`${app.base}/teams`);
  await wait(page, there('#view .tdef'));
  await wait(page, `document.querySelectorAll('#view .tdef').length===6`);
  // list is how many rows the list holds; the number itself is read where a
  // walk counts from it.
  const list = `document.querySelectorAll('#view .tdef').length`;

  const scaffold = hergyNew('ops');
  const profileFile = path.join(app.proj, '.hergy', 'profiles.yaml');
  const beforeProfile = readFileSync(profileFile, 'utf8');
  await press(page, q('.toolbar [data-act=new]'));
  await wait(page, there2(draft, '.in-name'));
  await set(page, `${draft}.querySelector('.in-name')`, 'ops');
  await wait(page, `${draft}.querySelector('.in-name').classList.contains('bad')===false`);
  await wait(page, `${draft}.querySelector('[data-act=t-save]').disabled===false`);
  // 監査 R9: the row that is not there yet is saved from the ルール tab.
  await press(page, `${draft}.querySelector('.dtabs .dtab[data-t=agree]')`);
  await wait(page, there2(draft, '.panel.agree-p footer.pf'));
  await wait(page, `${draft}.querySelector('.panel.agree-p [data-act=save]').disabled===false`);
  await press(page, `${draft}.querySelector('.panel.agree-p [data-act=save]')`);
  await wait(page, `!!${tRow('ops')}`);
  await wait(page, `!document.querySelector('.tdef[data-name^="~new"]')`);
  await readTo<string>(
    page,
    `${q('#view .tdef')}.dataset.name`,
    'the saved row stands first as, want ops',
  ).toBe('ops');
  // R3: the starter profile is added to the project's profiles.yaml with one
  // edit, so every other line a person wrote comes back as it was.
  expect(
    readFileSync(profileFile, 'utf8'),
    "the save rewrote the project's profiles.yaml, want only the starter profile added",
  ).toBe(`${beforeProfile}  claude-default:\n    kind: claude\n`);
  // R7: the band's number follows the save.
  await readTo<string>(
    page,
    `document.querySelector('.screens .scr[data-screen=teams] .n').textContent`,
    'the band counts the saved definition, want 7',
  ).toBe('7');

  // N4: the same name saved into 共通 writes the shared place, with the role
  // documents and the profile that place does not hold (B2, Y3).
  await press(page, q('.toolbar [data-act=new]'));
  await wait(page, there2(draft, '.in-name'));
  // The place comes first: the same name in プロジェクト is taken by the save
  // above.
  await press(page, `${draft}.querySelector('.c-place .seg button[data-x=global]')`);
  await wait(
    page,
    `${draft}.querySelector('.c-place .seg button[data-x=global]').classList.contains('on')`,
  );
  await set(page, `${draft}.querySelector('.in-name')`, 'ops');
  await wait(page, `${draft}.querySelector('[data-act=t-save]').disabled===false`);
  await press(page, `${draft}.querySelector('[data-act=t-save]')`);
  await wait(page, `!document.querySelector('.tdef[data-name^="~new"]')`);
  // The two role documents the shared place lacks are written; the 中 test sees
  // the project's side alone.
  const global = path.join(app.cfg, 'hergy');
  for (const name of ['lead.yaml', 'builder.yaml']) {
    expect(
      readFileSync(path.join(global, 'roles', name), 'utf8'),
      `the shared roles/${name} reads, want the scaffold's own`,
    ).toBe(scaffold.roles.get(name));
  }
  expect(
    readFileSync(path.join(global, 'profiles.yaml'), 'utf8'),
    "the shared profiles.yaml was written although the project's own resolves the starter profile",
  ).not.toContain('claude-default');

  // N8: 取り消し takes a row that is not there yet out and writes nothing.
  let before = await read<number>(page, list);
  await press(page, q('.toolbar [data-act=new]'));
  await wait(page, there2(draft, '.in-name'));
  await press(page, `${draft}.querySelector('[data-act=discard]')`);
  await wait(page, `!document.querySelector('.tdef[data-name^="~new"]')`);
  expect(
    readdirSync(path.join(app.proj, '.hergy', 'teams')).sort(),
    "取り消し left the project's teams as, want the fixture's and the walk's",
  ).toEqual(['core', 'docs', 'migrate', 'ops', 'research']);

  // R7: 削除 draws the band's number again: taking a definition the project
  // alone holds away leaves one fewer, and the band follows the list.
  before = await read<number>(page, list);
  await press(page, `${tRow('migrate')}.querySelector('.sum')`);
  await wait(page, `${tRow('migrate')}.classList.contains('open')`);
  await press(page, `${tRow('migrate')}.querySelector('.dacts [data-act=del]')`);
  await wait(page, there2(tRow('migrate'), '[data-act=del-yes]'));
  await press(page, `${tRow('migrate')}.querySelector('[data-act=del-yes]')`);
  await wait(page, `!document.querySelector('.tdef[data-name="migrate"]')`);
  await readTo<string>(
    page,
    `document.querySelector('.screens .scr[data-screen=teams] .n').textContent`,
    'the band counts the definition the delete took away',
  ).toBe(String(before - 1));
});

test('TEAMS-Y2 the preview and the line a turn adds', async ({ page, app }) => {
  // e2eTeamsTopology's browser-only part: the preview the topology tab draws is
  // the saved team.yaml line for line (Y6), and a turn added from a member's
  // card stands in the preview with the box sent to it (Y2, R2).
  const panel = shape('core');
  const line = (find: string) => `${panel}.querySelector(${JSON.stringify(`.tprev .yl${find}`)})`;

  await page.goto(`${app.base}/teams?open=core`);
  await wait(page, there('#view .tdef'));
  await press(page, `${panel}.querySelector('.cv-sh [data-cv-side=yaml]')`);
  await wait(page, there2(panel, '.tprev .ybox .yl'));

  // Y6: the preview is the saved team.yaml line for line, comments and all. The
  // reading is held to the file until it matches; a line the preview draws as a
  // single space stands for a blank line of the file.
  const text = readFileSync(path.join(app.proj, '.hergy', 'teams', 'core', 'team.yaml'), 'utf8');
  const want = text.replace(/\n$/, '').split('\n');
  const preview = `[...document.querySelectorAll('.tdef[data-name="core"] .tprev .yl .yt')].map(function(t){return t.textContent}).join('\u0001')`;
  await expect
    .poll(
      async () => {
        const got = (await read<string>(page, preview)).split('\u0001');
        if (got.length !== want.length) {
          return `the preview holds ${got.length} lines, want the file's ${want.length}`;
        }
        const at = want.findIndex(
          (line, i) => got[i] !== line && !(got[i] === ' ' && line.trim() === ''),
        );
        if (at < 0) return '';
        return `the preview's line ${at + 1} reads ${JSON.stringify(got[at])}, want ${JSON.stringify(want[at])}`;
      },
      { timeout: DEADLINE, message: 'the preview is the saved team.yaml line for line' },
    ).toBe('');

  // Y2: a turn added from analyst to reviewer stands at the end of the preview
  // as an added line, and the box was sent to it (B7, R2).
  await press(page, `${panel}.querySelector('.cv-sh [data-cv-side=attr]')`);
  await press(page, `${panel}.querySelector('.cv-n.member[data-cv-n=analyst]')`);
  await wait(page, there2(panel, '.cv-side select[data-cv-f=turn-add]'));
  await choose(page, `${panel}.querySelector('.cv-side select[data-cv-f=turn-add]')`, 'reviewer');
  await wait(page, there2(panel, '.cv-side .cv-t'));
  await press(page, `${panel}.querySelector('.cv-sh [data-cv-side=yaml]')`);
  await wait(page, `!!${line('.ins')} && ${line('.ins')}.textContent.indexOf('analyst -> reviewer')>=0`);
  await readTo<string>(
    page,
    `(function(){var b=${panel}.querySelector('.tprev .ybox'),l=${line('.ins')};if(!b||!l)return false;var br=b.getBoundingClientRect(),lr=l.getBoundingClientRect();return lr.top>=br.top-1&&lr.bottom<=br.bottom+1;})()+''`,
    'the box was not sent to the line the turn added',
  ).toBe('true');
});

test('TEAMS-C1 the canvas adds a member, removes it, and adds a worker', async ({ page, app }) => {
  // ac-333c: a member added from the canvas is saved and then removed, the
  // toolbar's ＋ワーカー works with no point picked (it asks whose worker), and
  // the blank space's double press asks the same. The turn is drawn from a
  // point's edge handle (the canvas's own gesture).
  const panel = shape('core');
  const css = '.tdef[data-name="core"] .panel.team-p';
  const file = path.join(app.proj, '.hergy', 'teams', 'core', 'team.yaml');
  await page.goto(`${app.base}/teams?open=core`);
  await wait(page, there2(panel, '.cv-canvas svg'));

  // (1) ＋メンバー puts a point in the figure, picks it, and puts the cursor in
  // its name; the save keeps it even before a turn is drawn (r-df77).
  await press(page, `${panel}.querySelector('[data-cv-act=m-add]')`);
  await wait(page, `(document.activeElement&&document.activeElement.dataset.cvF)==='m-name'`);
  const name = await read<string>(page, `${panel}.querySelector('[data-cv-f=m-name]').value`);
  expect(/^[a-z][a-z0-9_-]*$/.test(name), `the new member's name reads ${name}`).toBe(true);
  await wait(page, `${panel}.querySelector('[data-act=t-save]').disabled===false`);
  await press(page, `${panel}.querySelector('[data-act=t-save]')`);
  await wait(page, there2(panel, '.ok'));
  expect(
    (readFileSync(file, 'utf8').match(/^  - name: /gm) || []).length,
    'the save did not write the added member',
  ).toBe(6);

  // (2) the point goes with Delete and the save takes it away. ⌘Z puts it back.
  await press(page, `${panel}.querySelector('.cv-n.member[data-cv-n="${name}"]')`);
  await page.keyboard.press('Delete');
  await wait(page, `!!document.querySelector('.cv-toast')`);
  await press(page, `document.querySelector('.cv-toast [data-act=cv-undo]')`);
  await wait(page, `!!${panel}.querySelector('.cv-n.member[data-cv-n="${name}"]')`);
  await press(page, `${panel}.querySelector('.cv-n.member[data-cv-n="${name}"]')`);
  await page.keyboard.press('Delete');
  await wait(page, `${panel}.querySelector('[data-act=t-save]').disabled===false`);
  await press(page, `${panel}.querySelector('[data-act=t-save]')`);
  await wait(page, there2(panel, '.ok'));
  expect(
    (readFileSync(file, 'utf8').match(/^  - name: /gm) || []).length,
    'the save did not take the removed member away',
  ).toBe(5);

  // (3) ＋ワーカー asks whose worker when no point is picked, and the blank
  // space's double press asks the same.
  await page.keyboard.press('Escape');
  await press(page, `${panel}.querySelector('[data-cv-act=w-add]')`);
  await wait(page, there2(panel, '.cv-menu'));
  await readTo<string>(page, `${panel}.querySelector('.cv-menu').textContent`, 'the menu reads').toContain('どのメンバーの');
  // Esc closes the menu with nothing picked (監査 2-8).
  await page.keyboard.press('Escape');
  await wait(page, `!${panel}.querySelector('.cv-menu')`);
  await press(page, `${panel}.querySelector('[data-cv-act=w-add]')`);
  await wait(page, there2(panel, '.cv-menu'));
  await press(page, `${panel}.querySelector('.cv-menu [data-cv-act=menu-w]')`);
  await wait(page, there2(panel, '.cv-side [data-cv-f=w-purpose]'));

  const box = await page.locator(`${css} .cv-canvas`).boundingBox();
  expect(box, 'the canvas has no box').not.toBeNull();
  const x = box!.x + 10, y = box!.y + 10;
  await page.mouse.dblclick(x, y);
  await wait(page, there2(panel, '.cv-menu'));
  await readTo<string>(page, `${panel}.querySelector('.cv-menu').textContent`, 'the blank menu reads').toContain('メンバーを足す');

  // The turn: from analyst's edge handle to reviewer (missing in the fixture).
  await page.keyboard.press('Escape');
  await press(page, `${panel}.querySelector('.cv-n.member[data-cv-n=analyst]')`);
  await wait(page, there2(panel, '.cv-h'));
  const target = await page.locator(`${css} .cv-n.member[data-cv-n=reviewer]`).boundingBox();
  let best: { x: number; y: number; d: number } | null = null;
  for (const h of await page.locator(`${css} .cv-h`).all()) {
    const hb = await h.boundingBox();
    if (!hb || !target) continue;
    const d = Math.hypot(hb.x + hb.width / 2 - (target.x + target.width / 2), hb.y + hb.height / 2 - (target.y + target.height / 2));
    if (!best || d < best.d) best = { x: hb.x + hb.width / 2, y: hb.y + hb.height / 2, d };
  }
  expect(best, 'the picked point has no handle').not.toBeNull();
  await page.mouse.move(best!.x, best!.y);
  await page.mouse.down();
  await page.mouse.move(target!.x + target!.width / 2, target!.y + target!.height / 2, { steps: 10 });
  await page.mouse.up();
  await wait(page, there2(panel, '.cv-side .cv-t'));
  await readTo<string>(page, `${panel}.querySelector('.cv-side .cv-t').textContent`, 'the picked line reads').toContain('analyst');
});

test('TEAMS-C2 undo after letting go, a line from the keyboard, and no two points over each other', async ({ page, app }) => {
  // 監査 1: (1) ⌘Z takes back the newest operation even with nothing picked,
  // (2) a line is picked with Tab and Enter and goes with Delete, and (4) no
  // point stands on another.
  const panel = shape('core');
  const css = '.tdef[data-name="core"] .panel.team-p';
  await page.goto(`${app.base}/teams?open=core`);
  await wait(page, there2(panel, '.cv-canvas svg'));
  const nodes = `document.querySelectorAll('.tdef[data-name="core"] .cv-n').length`;

  // (1) add, let the pick go on the blank space, and ⌘Z.
  const before = await read<number>(page, nodes);
  await press(page, `${panel}.querySelector('[data-cv-act=m-add]')`);
  await wait(page, `${nodes}===${before + 1}`);
  // The notice's own word is 取り消す; the save band's is 元に戻す (3).
  await readTo<string>(page, `document.querySelector('.cv-toast [data-act=cv-undo]').textContent`, 'the notice button reads').toBe('取り消す');
  await readTo<string>(page, `${panel}.querySelector('[data-act=t-revert]').textContent`, 'the save band button reads').toBe('元に戻す');
  await page.locator(`${css} .cv-canvas`).click({ position: { x: 8, y: 8 } });
  await page.keyboard.press('Meta+z');
  await wait(page, `${nodes}===${before}`);
  await readTo<string>(page, `document.querySelector('.cv-toast').textContent`, 'the undo notice reads').toContain('を取り消した');

  // (2) a line picked with the keyboard: Enter opens its 属性, Delete takes both
  // directions away.
  // The line is focused directly and Enter picks it: the Tab order depends on
  // how far the panel's redraw has got, and a walk that sends Tab before the
  // redraw settles lands on another control (lead's report: one run in three).
  const hit = page.locator(`${css} .cv-hit`).first();
  await hit.waitFor();
  await hit.focus();
  await page.keyboard.press('Enter');
  await wait(page, `!!${panel}.querySelector('.cv-side .cv-t')`);
  await readTo<string>(page, `${panel}.querySelector('.cv-side .cv-t').textContent`, 'the picked line reads').toContain('⇄');
  await page.keyboard.press('Delete');
  await wait(page, `!!document.querySelector('.cv-toast')`);
  await readTo<string>(page, `document.querySelector('.cv-toast').textContent`, 'the removed line reads').toContain('のターンを外した');

  // (4) no two points stand over each other, including a worker just added.
  const overlap = `(function(){var b=[...document.querySelectorAll('.tdef[data-name="core"] .cv-n')].map(function(e){return e.getBoundingClientRect()});for(var i=0;i<b.length;i++)for(var j=i+1;j<b.length;j++){var a=b[i],c=b[j];if(a.left<c.right-1&&c.left<a.right-1&&a.top<c.bottom-1&&c.top<a.bottom-1)return 'point '+i+' and '+j+' cover each other';}return '';})()`;
  await readTo<string>(page, overlap, 'two points cover each other').toBe('');
  await press(page, `${panel}.querySelector('.cv-n.member[data-cv-n=lead]')`);
  await wait(page, there2(panel, '.cv-side [data-cv-f=m-name]'));
  await press(page, `${panel}.querySelector('[data-cv-act=w-add]')`);
  await wait(page, there2(panel, '.cv-side [data-cv-f=w-purpose]'));
  await readTo<string>(page, overlap, 'the worker just added covers a point').toBe('');
});

test('TEAMS-R1 the rules tab is the same switch and the same three fields', async ({ page, app }) => {
  // d-85dc: the team's ルール tab mounts the same part as the role screen; the
  // row carries the switch, and the opened rule holds ID, 本文 and メモ alone.
  await page.goto(`${app.base}/teams?open=core`);
  await wait(page, there('.tdef[data-name="core"] .panel.team-p'));
  await press(page, q('.tdef[data-name="core"] .dtab[data-t=agree]'));
  await wait(page, there('.tdef[data-name="core"] .panel.agree-p .rule'));
  await readTo<number>(page, `document.querySelectorAll('.tdef[data-name="core"] .panel.agree-p .rule .onsw').length`, 'the rules tab has a switch').toBeGreaterThan(0);
  await press(page, q('.tdef[data-name="core"] .panel.agree-p .rule .rsum'));
  await wait(page, there('.tdef[data-name="core"] .panel.agree-p .rule.open .redit'));
  const labels = await page.$$eval('.tdef[data-name="core"] .panel.agree-p .rule.open .redit .field > label, .tdef[data-name="core"] .panel.agree-p .rule.open .redit .field > .fl', (els) => els.map((e) => (e.textContent || '').trim()));
  expect(labels, 'the team rule holds ID, 本文 and メモ alone').toEqual(['ID', '本文', 'メモ']);
});
