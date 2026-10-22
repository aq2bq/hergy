// TEAMS-N1, TEAMS-P5, TEAMS-Y2 and TEAMS-V1 — checks/teams.md's 大 items, walked
// in a browser. They are internal/app e2eTeamsAddCopyDelete, e2eTeamsOpen (with
// e2eTeamsWorkerPurpose, e2eTeamsGraphProblems and e2eTeamsSaveShape),
// e2eTeamsTopology and e2eTeamsOpen again, test for test and assertion for
// assertion, over the same real-scale fixture (testdata/teams-v2: 4 definitions
// of the project and 3 shared, core's 53 rules with CORE-8 left out).
import { test, expect, hergyNew } from './fixtures';
import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { choose, diff, focus, press, q, read, set, there, wait } from './helpers';

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

// open walks e2eTeamsOpen: the row opens in place, the shape panel is 頭 → 図 →
// 検査の列 → 足, the check column carries validate's sentences, a press on one of
// them picks its node, the ルール tab draws the shared part, and a definition
// whose names do not resolve is drawn like one that reads (P1, S4〜S6, V1, P9,
// P3, T1, T4, R3〜R5, R10).
async function open(page: Parameters<typeof read>[0], base: string): Promise<void> {
  await page.goto(`${base}/teams`);
  await wait(page, there('#view .tdef'));

  // P1: pressing the row opens it right under itself, and the address keeps it.
  await press(page, `${tRow('core')}.querySelector('.sum')`);
  await wait(page, `${tRow('core')}.classList.contains('open')`);
  expect(
    await read<string>(page, `window.location.search`),
    'opening the core row left in the address',
  ).toContain('open=core');

  // S4: the mission's check list, the two tabs, and the shape panel below them.
  expect(
    await read<string>(page, `${tRow('core')}.querySelector('.detail > ul.mvm').tagName`),
    "the open row's first child reads",
  ).toBe('UL');
  expect(
    await read<string>(page, `${tRow('core')}.querySelector('.dtabs').nextElementSibling.className`),
    'the shape panel does not stand under the tabs',
  ).toContain('team-p');
  expect(
    await read<string>(
      page,
      `[...document.querySelectorAll('.tdef[data-name="core"] .dtabs .dtab')].map(function(b){return b.textContent.trim()}).join(',')`,
    ),
    'the tabs read',
  ).toBe('トポロジー,ルール53');

  // S5: the shape panel is 頭 → 図 → 検査の列 → 足.
  const panel = shape('core');
  for (const want of ['.ph', '.gbody .gwrap svg', '.gbody .vlist', 'footer.pf [data-act=t-save]']) {
    expect(
      await read<number>(page, `${panel}.querySelectorAll(${JSON.stringify(want)}).length`),
      `the shape panel has nothing at ${want}`,
    ).toBeGreaterThan(0);
  }
  expect(
    await read<string>(page, `${panel}.querySelector('.ph .path').textContent`),
    "the shape panel's path reads",
  ).toContain('teams/core/team.yaml');
  expect(
    await read<number>(page, `${panel}.querySelectorAll('.gn.member').length`),
    'the figure draws members, want 5',
  ).toBe(5);
  expect(
    await read<number>(page, `${panel}.querySelectorAll('.gn.worker').length`),
    'the figure draws workers, want 2',
  ).toBe(2);
  // 1.4.1: the members whose role document is broken wear 問題, and no one else does.
  expect(
    await read<string>(
      page,
      `[...document.querySelectorAll('.tdef[data-name="core"] .panel.team-p .gn.member.err')].map(function(g){return g.dataset.id}).join(',')`,
    ),
    'the members in problem read',
  ).toBe('impl-a,impl-b');

  // V1: the check column carries validate's own sentences, with the role
  // documents behind them.
  expect(
    await read<string>(page, `${panel}.querySelector('.vlist .vhead .vt').textContent`),
    "the check column's head reads",
  ).toBe('validate');
  expect(
    await read<string>(page, `${panel}.querySelector('.vlist .vcount').textContent.trim()`),
    "the figure's own check reads, want 2 problems and 1 warning",
  ).toBe('問題 2警告 1');
  expect(
    await read<number>(page, `${panel}.querySelectorAll('.vlist .vitem').length`),
    'the check column shows items, want 3',
  ).toBe(3);
  expect(
    await read<string>(page, `(function(){var m=${panel}.querySelector('.vlist .vsub');return m?m.textContent:''})()`),
    "the role documents' head reads",
  ).toBe('ロールの文書');
  expect(
    await read<string>(page, `${panel}.querySelector('.vlist .vitem .msg').textContent`),
    'the first problem reads',
  ).toContain('BUILD-4');

  // P9: pressing one item of the check column picks its node and shows its card.
  await press(page, `${panel}.querySelector('.vlist .vitem[data-node]')`);
  await wait(page, there2(panel, '.gcard[data-sel=node]'));
  expect(
    await read<string>(page, `${panel}.querySelector('.gcard[data-sel=node]').dataset.id`),
    'pressing the first problem picked, want impl-a',
  ).toBe('impl-a');
  // 1.4.4: a member's card carries the assignment chips and the ターンを渡す相手 head.
  expect(
    await read<string>(page, `${panel}.querySelector('.gcard .links .uchip').textContent`),
    "the card's role chip reads",
  ).toBe('ロール builder');
  expect(
    await read<string>(page, `${panel}.querySelector('.gcard .tl .gsub').textContent`),
    "the card's head reads",
  ).toBe('core-impl-a · ターンを渡す相手');
  await press(page, `${panel}.querySelector('.gcard .x')`);
  await wait(page, `!${panel}.querySelector('.gcard')`);

  // P3: the ルール tab draws the shared part, with the team's own rules and none
  // of the name, copy or delete a role's own panel carries (S6).
  await press(page, `${tRow('core')}.querySelector('.dtabs .dtab[data-t=agree]')`);
  await wait(page, `!!${agree('core')}`);
  const coreAgree = agree('core');
  expect(
    await read<number>(page, `${coreAgree}.querySelectorAll('.rule').length`),
    'the agreement panel draws no rules',
  ).toBeGreaterThan(0);
  expect(
    await read<string>(page, `${coreAgree}.querySelector('.ph .path').textContent`),
    "the agreement panel's path reads",
  ).toContain('teams/core/agreement.yaml');
  expect(
    await read<number>(
      page,
      `${coreAgree}.querySelectorAll('[data-act=copy],[data-act=del],[data-act=dup],[data-f=name]').length`,
    ),
    "the team's agreement panel carries a name, copy or delete of its own",
  ).toBe(0);
  expect(
    await read<number>(
      page,
      `getComputedStyle(${coreAgree}.querySelector('.pmain')).gridTemplateColumns.split(' ').filter(Boolean).length`,
    ),
    '2000: the agreement panel stands in columns, want the editor and the preview',
  ).toBe(2);
  // T1・T4: the words of the ルール part stand on the screen.
  expect(
    await read<string>(
      page,
      `[...document.querySelectorAll('.tdef[data-name="core"] .dtabs .dtab')].map(function(b){return b.dataset.t+'='+b.textContent.trim()}).join(',')`,
    ),
    "the open row's tabs read, want トポロジー and ルール53",
  ).toBe('shape=トポロジー,agree=ルール53');
  // R4: a sentence about the document as a whole marks the first line of the
  // preview alone, not every heading and blank line.
  expect(
    await read<number>(page, `${coreAgree}.querySelectorAll('.yprev .yl.warn').length`),
    "the agreement's preview marks lines as a warning, want the first line alone",
  ).toBe(1);
  expect(
    await read<number>(page, `${coreAgree}.querySelectorAll('.yprev .yl.err').length`),
    "the agreement's preview marks lines as a problem, want none",
  ).toBe(0);
  // R3: the head is the path alone — the place stands in the row's pills, not here.
  expect(
    await read<string>(page, `(${coreAgree}.querySelector('.ph').textContent.indexOf('プロジェクト')>=0)+''`),
    "the agreement panel's head names a place",
  ).toBe('false');
  // R5: nothing has changed, so 元に戻す is not offered.
  expect(
    await read<string>(page, `${coreAgree}.querySelector('.pf [data-act=revert]').hidden+''`),
    "the agreement's foot offers 元に戻す with nothing changed",
  ).toBe('true');

  // 1.3・1.4.5・1.9・R10: a definition whose names do not resolve is drawn like
  // one that reads, and its one problem stands on the worker it names.
  const docs = shape('docs');
  await press(page, `${tRow('docs')}.querySelector('.sum')`);
  await wait(page, there2(docs, '.vlist .vitem .msg'));
  expect(
    await read<number>(page, `${docs}.querySelectorAll('.gwrap svg, footer.pf [data-act=t-save]').length`),
    'the definition whose profile does not resolve draws figures and saves, want the figure and the save',
  ).toBe(2);
  // V1・R10: its own sentence stands in the check column, which counts the one
  // problem (the role documents behind the row are not this column's).
  expect(
    await read<string>(page, `${docs}.querySelector('.vlist .vcount').textContent.trim()`),
    "the docs panel's check reads, want its one problem",
  ).toBe('問題 1');
  expect(
    await read<string>(page, `${docs}.querySelector('.vlist .vitem .msg').textContent`),
    "the docs figure's problem reads",
  ).toContain('gemini-flash');
  // 1.4.1・R10: the problem stands on the worker 翻訳's own point, with 問題.
  expect(
    await read<string>(
      page,
      `[...document.querySelectorAll('.tdef[data-name="docs"] .panel.team-p .gn.worker.err')].map(function(g){return g.dataset.i}).join(',')`,
    ),
    'the workers in problem read, want the 翻訳 worker alone',
  ).toBe('0');
  expect(
    await read<string>(
      page,
      `[...document.querySelectorAll('.tdef[data-name="docs"] .panel.team-p .gn.worker.err')].map(function(g){return g.querySelector('.bdg text').textContent}).join(',')`,
    ),
    "the worker's badge reads, want 問題",
  ).toBe('問題');
  expect(
    await read<string>(page, `${docs}.querySelector('.ph .state').textContent`),
    "the shared definition's state reads",
  ).toBe('効いている');
  expect(
    await read<string>(page, `${tRow('docs')}.querySelector('.panel.shadowed .state').textContent`),
    "the hidden panel's state reads",
  ).toBe('隠れている');
}

// there2 is whether one element stands inside one panel expression.
const there2 = (panel: string, find: string) => `!!${panel}.querySelector(${JSON.stringify(find)})`;

test('TEAMS-V1 a definition whose names do not resolve', async ({ page, app }) => {
  await open(page, app.base);
});

// workerPurpose is e2eTeamsWorkerPurpose: the worker a person hires takes its
// purpose one letter at a time without the cursor leaving the field, the empty
// purpose shuts the save, and 元に戻す puts the definition back (X3, 1.4.6).
async function workerPurpose(page: Parameters<typeof read>[0], base: string): Promise<void> {
  await page.goto(`${base}/teams?open=core`);
  await wait(page, there('.tdef[data-name="core"] .panel.team-p .gwrap svg'));
  const panel = shape('core');
  const field = `${panel}.querySelector('.gcard[data-sel=worker] .in-purpose')`;

  await press(page, `${panel}.querySelector('.gn.member[data-id=lead]')`);
  await wait(page, there2(panel, '.gcard[data-sel=node]'));
  await press(page, `${panel}.querySelector('.gcard [data-act=worker-add]')`);
  await wait(page, `!!${field}`);
  expect(
    await read<string>(page, `(document.activeElement===${field})+''`),
    '＋ ワーカー left the cursor outside the purpose',
  ).toBe('true');
  // A worker with no purpose is a problem the saved file did not hold, so the
  // save is shut until the purpose is there (X3).
  await wait(page, `${panel}.querySelector('[data-act=t-save]').disabled`);
  const vm = `[...document.querySelectorAll('.tdef[data-name="core"] .panel.team-p .gcard .vm li')].map(function(li){return li.textContent}).join('|')`;
  const first = await read<string>(page, vm);
  expect(first, "the new worker's card reads, want the check's own sentence").toContain('purpose');
  expect(first, "the new worker's card reads, want the check's own sentence").not.toContain(
    'undefined',
  );
  // One letter at a time, the way a person types: the field is not drawn again,
  // so the cursor stays and the word collects.
  const purpose = 'ログの整理';
  const letters = [...purpose];
  for (let i = 1; i <= letters.length; i++) {
    await set(page, field, letters.slice(0, i).join(''));
    await wait(page, `(document.activeElement===${field})`);
    expect(
      await read<string>(page, `${field}.value`),
      `after ${i} letters the purpose reads, want ${letters.slice(0, i).join('')}`,
    ).toBe(letters.slice(0, i).join(''));
  }
  // One worker, however many letters wrote its purpose (1.4.6), and the problem
  // its empty purpose added is gone, so the save is open again.
  await wait(page, `${panel}.querySelector('footer.pf .dirty').textContent.trim()==='1 か所'`);
  await wait(page, `${panel}.querySelector('[data-act=t-save]').disabled===false`);
  const workerVM = await read<string>(
    page,
    `(function(){var u=${panel}.querySelector('.gcard[data-sel=worker] .vm');return u?u.textContent:''})()`,
  );
  expect(workerVM, "the worker's card still holds its purpose's problem").not.toContain('purpose');
  // 元に戻す puts the definition back, and the count with it.
  await wait(page, there2(panel, '[data-act=t-revert]:not([hidden])'));
  await press(page, `${panel}.querySelector('[data-act=t-revert]')`);
  await wait(page, `${panel}.querySelector('footer.pf .dirty').textContent.trim()===''`);
}

// graphProblems is e2eTeamsGraphProblems: the card of a line that would take a
// member's last turn says so before the change, keeps the line that would leave
// beside it, the problem of the draft stands on the member's own point, and a
// warning about the team's own agreement opens that rule in the ルール tab
// (5.4 M3, 1.4.1, 1.4.4, 1.4.5, 1.7, R8・R9・R11・R12).
async function graphProblems(page: Parameters<typeof read>[0], base: string): Promise<void> {
  await page.goto(`${base}/teams?open=core`);
  await wait(page, there('.tdef[data-name="core"] .panel.team-p .gwrap svg'));
  const panel = shape('core');
  const stop = `[...document.querySelectorAll('.tdef[data-name="core"] .panel.team-p .gcard .vm li')].some(function(li){return li.textContent.indexOf('analyst から出るターンがありません')>=0})`;

  // R8: analyst hands out three turns and hires no worker, so taking the last
  // one away leaves it handing the turn to nobody.
  for (const to of ['impl-a', 'impl-b', 'lead']) {
    await press(page, `${panel}.querySelector('.gn.member[data-id=analyst]')`);
    await wait(page, there2(panel, '.gcard[data-sel=node]'));
    await press(page, `${panel}.querySelector('.gcard .row button[data-act=pick-edge][data-b=${to}]')`);
    await wait(page, there2(panel, '.gcard[data-sel=edge]'));
    await press(page, `${panel}.querySelector('.gcard [data-act=turn-ask][data-a=analyst][data-b=${to}]')`);
    await wait(page, there2(panel, '.gcard [data-act=turn-del]'));
    if (to === 'lead') {
      await wait(page, stop);
      // R11: the line the change would take away stands on the card even though
      // the change itself is what stops the definition (1.4.4).
      await wait(page, there2(panel, '.gcard .gpl s'));
      expect(
        await read<string>(page, `${panel}.querySelector('.gcard .gpl s').textContent`),
        "the card's line that would leave reads, want - core-lead（ロール lead）",
      ).toBe('- core-lead（ロール lead）');
    } else {
      expect(
        await read<boolean>(page, stop),
        `the card of the ${to} turn already names the problem that would come`,
      ).toBe(false);
    }
    await press(page, `${panel}.querySelector('.gcard [data-act=turn-del]')`);
    await wait(page, there2(panel, '.gcard .gdl .tag'));
  }

  // R9: the problem of the draft stands on analyst's own point, wearing 問題, and
  // the check column says where it stands (1.4.1・1.7).
  await wait(page, there('.tdef[data-name="core"] .panel.team-p .gn.member[data-id=analyst].err'));
  expect(
    await read<string>(page, `${panel}.querySelector('.gn.member[data-id=analyst] .bdg text').textContent`),
    "analyst's badge reads, want 問題",
  ).toBe('問題');
  expect(
    await read<string>(
      page,
      `[...document.querySelectorAll('.tdef[data-name="core"] .panel.team-p .vlist .vitem')].filter(function(b){return b.textContent.indexOf('analyst から出るターンがありません')>=0}).map(function(b){return b.querySelector('.lno').textContent}).join(',')`,
    ),
    "the check column puts analyst's problem at, want analyst",
  ).toBe('analyst');

  // R9: scout's turn to the human goes, so the team's own agreement's rule RES-6
  // warns. It stands under チームのルール, and its press opens that rule in the
  // ルール tab (1.4.5), inside the window a person reads on.
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto(`${base}/teams?open=research`);
  await wait(page, there('.tdef[data-name="research"] .panel.team-p .gwrap svg'));
  const rp = shape('research');
  await press(page, `${rp}.querySelector('.gn.member[data-id=scout]')`);
  await wait(page, there2(rp, '.gcard[data-sel=node]'));
  await press(page, `${rp}.querySelector('.gcard .row button[data-act=pick-edge][data-b=human]')`);
  await wait(page, there2(rp, '.gcard[data-sel=edge]'));
  await press(page, `${rp}.querySelector('.gcard [data-act=turn-ask][data-a=scout][data-b=human]')`);
  await wait(page, there2(rp, '.gcard [data-act=turn-del]'));
  await press(page, `${rp}.querySelector('.gcard [data-act=turn-del]')`);
  await wait(page, there2(rp, '.gcard .gdl .tag'));
  const item = `[...document.querySelectorAll('.tdef[data-name="research"] .panel.team-p .vlist .vitem')].find(function(b){return b.textContent.indexOf('RES-6')>=0})`;
  await wait(page, `!!${item}`);
  expect(
    await read<string>(page, `${rp}.querySelector('.vlist .vsub').textContent`),
    "the check column's first heading reads, want チームのルール",
  ).toBe('チームのルール');
  expect(
    await read<string>(page, `${item}.querySelector('.lno').textContent`),
    "the team agreement's warning stands at, want agreement.yaml",
  ).toBe('agreement.yaml');
  await press(page, item);
  await wait(page, there2(agree('research'), '.rule.open'));
  expect(
    await read<string>(page, `${agree('research')}.querySelector('.rule.open .rid b').textContent`),
    'the press opened the rule, want RES-6',
  ).toBe('RES-6');
  expect(
    await read<string>(page, `window.location.search`),
    'opening the rule left in the address',
  ).toContain('agree=research');

  // R12: the rule the press opened stands in the window — its head below the
  // top, its body above the save's band — and its body has the cursor (1.7).
  const above = `(function(){var p=${agree('research')};var r=p.querySelector('.rule.open'),t=p.querySelector('.rule.open [data-f=text]'),f=p.querySelector('footer.pf');if(!r||!t||!f)return false;var rr=r.getBoundingClientRect(),tr=t.getBoundingClientRect(),fr=f.getBoundingClientRect();return rr.top>=0&&tr.top>rr.top&&tr.bottom<=fr.top})()`;
  const cursor = `(document.activeElement&&document.activeElement.dataset.f)==='text'`;
  await wait(page, above);
  await wait(page, cursor);

  // R12: the address opens the same rule the same way.
  await page.goto(`${base}/teams?agree=research&rule=research.RES-6`);
  await wait(page, there('.tdef[data-name="research"] .panel.agree-p .rule.open'));
  await wait(page, above);
  await wait(page, cursor);
  await page.setViewportSize({ width: 2000, height: 1100 });
}

// saveShape is e2eTeamsSaveShape: the mission alone moves and the check rides on
// the typing, one turn goes with its card's own words, and docs' YAML and shape
// panels save a mission that adds no problem (F1・F2・F7, P2・P6・P7, R6・R10, X3).
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
  expect(
    await read<string>(page, `${shape('core')}.querySelector('.ok').textContent`),
    'the shape save line reads',
  ).toBe('保存しました');
  let { added, removed } = diff(before, readFileSync(file, 'utf8'));
  expect(
    added.length === 1 && removed.length === 1 && added[0] === `mission: "${mission}"` && removed[0].startsWith('mission: '),
    `the mission save wrote ${JSON.stringify(added)} and lost ${JSON.stringify(removed)}, want the mission line alone`,
  ).toBe(true);
  before = readFileSync(file, 'utf8');

  // F1・P6・P7: one turn goes, its card shows the line that would leave and the
  // sentence that would appear (M2), and the save writes the one line.
  const panel = shape('core');
  await press(page, `${panel}.querySelector('.gn.member[data-id=impl-b]')`);
  await wait(page, there2(panel, '.gcard[data-sel=node]'));
  await press(page, `${panel}.querySelector('.gcard .row button[data-act=pick-edge][data-b=reviewer]')`);
  await wait(page, there2(panel, '.gcard[data-sel=edge]'));
  await press(page, `${panel}.querySelector('.gcard [data-act=turn-ask][data-a=impl-b][data-b=reviewer]')`);
  await wait(page, there2(panel, '.gcard [data-act=turn-del]'));
  await wait(page, there2(panel, '.gcard .gpl s'));
  expect(
    await read<string>(page, `${panel}.querySelector('.gcard .gpl s').textContent`),
    'the line that would leave reads',
  ).toContain('core-reviewer');
  await wait(
    page,
    `[...document.querySelectorAll('.tdef[data-name="core"] .panel.team-p .gcard .vm li')].some(function(li){return li.textContent.indexOf('CORE-12')>=0})`,
  );
  // R6: the line the change would take away stands once, in the server's own
  // words, and the check column under it holds the server's problems alone.
  expect(
    await read<number>(
      page,
      `(function(){var s=${panel}.querySelector('.gcard .gpl s');return s?[...s.parentElement.children].filter(function(n){return n.tagName==='S'&&n.textContent.indexOf('core-reviewer')>=0}).length:0})()`,
    ),
    'the card strikes the line that would leave times, want once',
  ).toBe(1);
  expect(
    await read<number>(
      page,
      `[...${panel}.querySelectorAll('.gcard .vm li')].filter(function(li){return li.textContent.indexOf('core-reviewer')>=0}).length`,
    ),
    'the check column holds lines of the change as a problem, want none',
  ).toBe(0);
  await press(page, `${panel}.querySelector('.gcard [data-act=turn-del]')`);
  await wait(page, there2(panel, '.gcard .gdl .tag'));
  expect(
    await read<string>(page, `${panel}.querySelector('.gcard .gdl .tag').textContent`),
    "the removed turn's tag reads",
  ).toBe('消した');
  await wait(page, `${panel}.querySelector('footer.pf .dirty').textContent.trim()==='1 か所'`);
  await press(page, `${panel}.querySelector('[data-act=t-save]')`);
  await wait(page, there2(panel, '.ok'));
  ({ added, removed } = diff(before, readFileSync(file, 'utf8')));
  expect(
    added.length === 0 && removed.length === 1 && removed[0].trim() === '- impl-b -> reviewer',
    `the turn save wrote ${JSON.stringify(added)} and lost ${JSON.stringify(removed)}, want the turn's own line alone`,
  ).toBe(true);
  expect(readFileSync(file, 'utf8'), 'the turn save lost the mission the test wrote').toContain(mission);

  // F7・R10: docs' team.yaml reads — only a profile its worker names is not in
  // profiles.yaml — so the YAML tab holds its text and its own problem: a text
  // edit that only moves the mission adds no problem the saved text did not
  // already hold, so the save goes (X3).
  const docsFile = path.join(app.proj, '.hergy', 'teams', 'docs', 'team.yaml');
  const docsBefore = readFileSync(docsFile, 'utf8');
  await page.goto(`${app.base}/teams/yaml?file=docs.project.team`);
  await wait(page, there('.yed .ytext'));
  await wait(page, `document.querySelector('.yed .path').textContent.indexOf('teams/docs/team.yaml')>=0`);
  await wait(page, `document.querySelectorAll('.yed .vitem').length>0`);
  const docsText = await read<string>(page, q('.yed .ytext') + '.value');
  const docsMission = 'E2E が直した文書の使命。';
  const edited = docsText.replace('mission: "利用者向けの文書を整える"', `mission: "${docsMission}"`);
  expect(edited !== docsText, 'the fixture\'s docs/team.yaml no longer holds the mission the test moves').toBe(true);
  await set(page, q('.yed .ytext'), edited);
  // The problem the file already held is not a new one, so the save is open.
  await wait(page, `!${q('[data-act=yaml-save]')}.disabled`);
  await press(page, q('[data-act=yaml-save]'));
  await wait(page, there('.yed .ok'));
  ({ added, removed } = diff(docsBefore, readFileSync(docsFile, 'utf8')));
  expect(
    added.length === 1 && removed.length === 1 && added[0] === `mission: "${docsMission}"`,
    `the docs mission save wrote ${JSON.stringify(added)} and lost ${JSON.stringify(removed)}, want the mission line alone`,
  ).toBe(true);

  // F7・R10: the same definition's shape panel offers its save now (1.9). The
  // problem it holds is the file's own, so a mission edit adds nothing new and
  // the save goes; a change that adds a problem of its own shuts it again (X3).
  await page.goto(`${app.base}/teams?open=docs`);
  await wait(page, there('.tdef[data-name="docs"] .panel.team-p footer.pf [data-act=t-save]'));
  const docsPanel = shape('docs');
  const docsMission2 = 'E2E が姿から直した使命。';
  await set(page, q('.in-mission'), docsMission2);
  await wait(page, `${docsPanel}.querySelector('footer.pf .dirty').textContent.trim()==='1 か所'`);
  expect(
    await read<string>(page, `${docsPanel}.querySelector('[data-act=t-save]').disabled+''`),
    'the docs shape save is shut although the only problem is the file\'s own',
  ).toBe('false');
}

test('TEAMS-P5 the definition a person edits', async ({ page, app }) => {
  // The edit walk's save writes the file, so the Go run stood it in a project of
  // its own; here the test's own project is that project.
  await open(page, app.base);
  await workerPurpose(page, app.base);
  await graphProblems(page, app.base);
  await saveShape(page, app);
});

test('TEAMS-N1 add, copy and delete', async ({ page, app }) => {
  // B1: the toolbar's ＋ チーム定義を追加, a new row's name problems and the files
  // a save would write (B2, N1〜N4), a duplicate (B3, N5), a copy into the other
  // place (B4, N6), taking one place's folder away (B6, N7), and 取り消し (B5,
  // N8). Every save is read back from the filesystem.
  await page.goto(`${app.base}/teams`);
  await wait(page, there('#view .tdef'));
  await wait(page, `document.querySelectorAll('#view .tdef').length===6`);
  const rows = () => read<number>(page, `document.querySelectorAll('#view .tdef').length`);

  // B1: the toolbar's button stands above the list.
  expect(
    await read<string>(page, q('.toolbar [data-act=new]') + '.textContent.trim()'),
    "the toolbar's add reads, want ＋ チーム定義を追加",
  ).toBe('＋ チーム定義を追加');

  // N1: pressing it puts a new row first, open, with the cursor in its name,
  // プロジェクト picked, the starter's mission, a name problem, and the
  // mission's warning beside it (B1, B2).
  await press(page, q('.toolbar [data-act=new]'));
  await wait(page, there2(draft, '.in-name'));
  expect(
    await read<string>(page, `${q('#view .tdef')}.dataset.name.indexOf('~new')===0?'first':'no'`),
    'the new row does not stand first',
  ).toBe('first');
  expect(
    await read<string>(page, `(document.activeElement===${draft}.querySelector('.in-name'))+''`),
    'the new row left the cursor outside its name field',
  ).toBe('true');
  expect(
    await read<string>(page, `${draft}.querySelector('.c-place .seg button.on').textContent.trim()`),
    "the new row's place reads, want プロジェクト",
  ).toBe('プロジェクト');
  expect(
    await read<string>(page, `${draft}.querySelector('.in-mission').value`),
    "the new row's mission reads, want the starter's placeholder",
  ).toBe('このチームの使命を 1 行で');
  // N1: the name's problem and the figure's own sentences both stand in the
  // count — the name is the problem, and the rest warnings (B2).
  await wait(page, `${draft}.querySelector('.c-name .vcount').textContent.indexOf('問題 1')>=0`);
  await wait(page, `${draft}.querySelector('.c-name .vcount').textContent.indexOf('警告')>=0`);
  expect(
    await read<string>(page, `${draft}.querySelector('.c-name .vcount').textContent.trim()`),
    'the new row counts, want the name alone as a problem',
  ).not.toContain('問題 2');
  expect(
    await read<string>(page, `${draft}.querySelector('.in-name').classList.contains('bad')+''`),
    "the empty name's field is not marked",
  ).toBe('true');
  expect(
    await read<string>(
      page,
      `[...document.querySelectorAll('.tdef[data-name^="~new"] .vlist .vitem')].map(function(v){return v.querySelector('.lno').textContent+':'+v.querySelector('.msg').textContent}).join('|')`,
    ),
    "the check column reads, want the name's own problem",
  ).toContain('名前:チーム定義名を書いてください');
  // N2: the name is a problem, so the save is shut; the foot says the row is new
  // and names the files a save would write (B2).
  expect(
    await read<string>(page, `${draft}.querySelector('[data-act=t-save]').disabled+''`),
    'the save stands although the name is a problem',
  ).toBe('true');
  expect(
    await read<string>(page, `${draft}.querySelector('footer.pf .dirty').textContent.trim()`),
    "the new row's foot reads, want 新規",
  ).toBe('新規');
  expect(
    await read<number>(page, `${draft}.querySelectorAll('footer.pf .also code').length`),
    "the new row names files a save would write, want the project's three",
  ).toBe(3);
  // N2: a name hergy new takes clears the problem and opens the save.
  await set(page, `${draft}.querySelector('.in-name')`, 'ops');
  await wait(page, `${draft}.querySelector('.in-name').classList.contains('bad')===false`);
  await wait(page, `${draft}.querySelector('.c-name .vcount').textContent.indexOf('問題')<0`);
  await wait(page, `${draft}.querySelector('[data-act=t-save]').disabled===false`);
  expect(
    await read<number>(page, `document.querySelectorAll('#view .tdef').length`),
    'the list holds rows with the draft open, want 7',
  ).toBe(7);

  // N3: the save writes hergy new's own files, one letter apart from none. A row
  // that is not there yet is saved from either tab (spec-teams B2, B6, 監査 R9).
  const scaffold = hergyNew('ops');
  const profileFile = path.join(app.proj, '.hergy', 'profiles.yaml');
  const beforeProfile = readFileSync(profileFile, 'utf8');
  await press(page, `${draft}.querySelector('.dtabs .dtab[data-t=agree]')`);
  await wait(page, there2(draft, '.panel.agree-p footer.pf'));
  expect(
    await read<string>(page, `${draft}.querySelector('.panel.agree-p footer.pf .dirty').textContent.trim()`),
    "the new row's rule foot reads, want 新規",
  ).toBe('新規');
  expect(
    await read<string>(
      page,
      `[...document.querySelectorAll('.tdef[data-name^="~new"] .panel.agree-p footer.pf .also code')].map(function(c){return c.textContent}).join(',')`,
    ),
    "the new row's rule foot names, want the project's three files",
  ).toBe('team.yaml,agreement.yaml,profiles.yaml');
  await wait(page, `${draft}.querySelector('.panel.agree-p [data-act=save]').disabled===false`);
  await press(page, `${draft}.querySelector('.panel.agree-p [data-act=save]')`);
  await wait(page, `!!${tRow('ops')}`);
  await wait(page, `!document.querySelector('.tdef[data-name^="~new"]')`);
  expect(
    await read<number>(page, `document.querySelectorAll('#view .tdef').length`),
    'the list holds rows after the save, want 7',
  ).toBe(7);
  expect(
    await read<string>(page, `${q('#view .tdef')}.dataset.name`),
    'the saved row stands first as, want ops',
  ).toBe('ops');
  for (const name of ['team.yaml', 'agreement.yaml']) {
    expect(
      readFileSync(path.join(app.proj, '.hergy', 'teams', 'ops', name), 'utf8'),
      `the saved ops/${name} reads, want hergy new's own`,
    ).toBe(scaffold.teams.get(name));
  }
  // R3: the starter profile is added to the project's profiles.yaml with one
  // edit, so every other line a person wrote comes back as it was.
  expect(
    readFileSync(profileFile, 'utf8'),
    "the save rewrote the project's profiles.yaml, want only the starter profile added",
  ).toBe(`${beforeProfile}  claude-default:\n    kind: claude\n`);
  // R7: the band's number follows the save, and the row's own foot says 保存しました.
  await wait(page, `document.querySelector('.screens .scr[data-screen=teams] .n').textContent==='7'`);
  await wait(page, there2(tRow('ops'), 'footer.pf .ok'));
  expect(
    await read<string>(page, `${tRow('ops')}.querySelector('footer.pf .ok').textContent.trim()`),
    "the saved row's foot reads, want 保存しました",
  ).toBe('保存しました');

  // N4: the same name saved into 共通 writes the shared place, with the role
  // documents and the profile that place does not hold (B2, Y3).
  await press(page, q('.toolbar [data-act=new]'));
  await wait(page, there2(draft, '.in-name'));
  // The place comes first: the same name in プロジェクト is taken by N3's save.
  await press(page, `${draft}.querySelector('.c-place .seg button[data-x=global]')`);
  await wait(
    page,
    `${draft}.querySelector('.c-place .seg button[data-x=global]').classList.contains('on')`,
  );
  await set(page, `${draft}.querySelector('.in-name')`, 'ops');
  await wait(page, `${draft}.querySelector('[data-act=t-save]').disabled===false`);
  // The files it would write are the shared place's: the definition's two and
  // the two role documents it lacks. The starter profile is not among them.
  expect(
    await read<string>(
      page,
      `[...document.querySelectorAll('.tdef[data-name^="~new"] footer.pf .also code')].map(function(c){return c.textContent}).join(',')`,
    ),
    "the new row in 共通 names, want the definition's two and the two role documents",
  ).toBe('team.yaml,agreement.yaml,lead.yaml,builder.yaml');
  await press(page, `${draft}.querySelector('[data-act=t-save]')`);
  await wait(page, `!document.querySelector('.tdef[data-name^="~new"]')`);
  const global = path.join(app.cfg, 'hergy');
  for (const name of ['team.yaml', 'agreement.yaml']) {
    expect(
      readFileSync(path.join(global, 'teams', 'ops', name), 'utf8'),
      `the shared ops/${name} reads, want hergy new's own`,
    ).toBe(scaffold.teams.get(name));
  }
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
  // Both places hold ops now, so its row says プロジェクト is in effect and 共通
  // stands beside it — one row, not two (B4).
  await wait(page, `${tRow('ops')}.querySelectorAll('.c-place .pill').length===2`);
  expect(
    await read<string>(page, `${tRow('ops')}.querySelector('.c-place .pill.on').textContent.trim()`),
    'the row in both places holds in effect, want プロジェクト',
  ).toBe('プロジェクト');

  // N5: core's 複製 opens core-2 right under it, its save writes core's own two
  // files one letter apart from none, and it keeps core's place in the list.
  await press(page, `${tRow('core')}.querySelector('.sum')`);
  await wait(page, `${tRow('core')}.classList.contains('open')`);
  await press(page, `${tRow('core')}.querySelector('.dacts [data-act=dup]')`);
  await wait(page, there2(draft, '.in-name'));
  expect(
    await read<string>(page, `${draft}.querySelector('.in-name').value`),
    "the duplicate's name reads, want core-2",
  ).toBe('core-2');
  expect(
    await read<string>(page, `${tRow('core')}.nextElementSibling.dataset.name.indexOf('~new')===0?'below':'no'`),
    'the duplicate does not stand right under core',
  ).toBe('below');
  expect(
    await read<string>(page, `(document.activeElement===${draft}.querySelector('.in-name'))+''`),
    'the duplicate left the cursor outside its name field',
  ).toBe('true');
  await press(page, `${draft}.querySelector('[data-act=t-save]')`);
  await wait(page, `!!${tRow('core-2')}`);
  await wait(page, `!document.querySelector('.tdef[data-name^="~new"]')`);
  for (const name of ['team.yaml', 'agreement.yaml']) {
    expect(
      readFileSync(path.join(app.proj, '.hergy', 'teams', 'core-2', name), 'utf8'),
      `the duplicate's ${name} reads, want core's own`,
    ).toBe(readFileSync(path.join(app.proj, '.hergy', 'teams', 'core', name), 'utf8'));
  }
  expect(
    await read<string>(page, `${tRow('core')}.nextElementSibling.dataset.name`),
    'the saved duplicate stands at, want core-2 right under core',
  ).toBe('core-2');

  // N6: review (共通) copied プロジェクトに leaves one row, プロジェクト in effect
  // and 共通 struck out, and the project's folder holds review's own two files.
  await press(page, `${tRow('review')}.querySelector('.sum')`);
  await wait(page, `${tRow('review')}.classList.contains('open')`);
  expect(
    await read<string>(page, `${tRow('review')}.querySelector('.dacts [data-act=copy]').textContent.trim()`),
    "review's copy reads, want プロジェクトにコピー",
  ).toBe('プロジェクトにコピー');
  await press(page, `${tRow('review')}.querySelector('.dacts [data-act=copy]')`);
  await wait(page, there2(draft, '.c-place .pill.pending'));
  expect(
    await read<string>(page, `${draft}.querySelector('.c-place .pill.pending').textContent.trim()`),
    "the copy's place reads, want プロジェクト",
  ).toBe('プロジェクト');
  expect(
    await read<number>(page, `${draft}.querySelectorAll('.in-name').length`),
    "the copy offers a name field, but its name is review's own",
  ).toBe(0);
  expect(
    await read<string>(page, `${tRow('review')}.nextElementSibling.dataset.name.indexOf('~new')===0?'below':'no'`),
    'the copy does not stand right under review',
  ).toBe('below');
  await press(page, `${draft}.querySelector('[data-act=t-save]')`);
  await wait(page, `!document.querySelector('.tdef[data-name^="~new"]')`);
  for (const name of ['team.yaml', 'agreement.yaml']) {
    expect(
      readFileSync(path.join(app.proj, '.hergy', 'teams', 'review', name), 'utf8'),
      `the copy's ${name} reads, want review's own`,
    ).toBe(readFileSync(path.join(global, 'teams', 'review', name), 'utf8'));
  }
  await wait(page, `${tRow('review')}.querySelectorAll('.c-place .pill').length===2`);
  expect(
    await read<string>(
      page,
      `[...document.querySelectorAll('.tdef[data-name="review"] .c-place .pill')].map(function(p){return p.textContent.trim()+'='+p.className}).join(',')`,
    ),
    "review's places read, want プロジェクト in effect and 共通 hidden",
  ).toBe('プロジェクト=pill on,共通=pill shadow');

  // N7: 削除 takes the project's folder away, and the row stands on 共通 alone.
  await press(page, `${tRow('review')}.querySelector('.dacts [data-act=del]')`);
  await wait(page, there2(tRow('review'), '[data-act=del-yes]'));
  await press(page, `${tRow('review')}.querySelector('[data-act=del-yes]')`);
  await wait(page, `!document.querySelector('.tdef[data-name="review"] [data-act=del-yes]')`);
  expect(
    readdirSync(path.join(app.proj, '.hergy', 'teams')).includes('review'),
    "the project's review/team.yaml still stands",
  ).toBe(false);
  expect(
    readFileSync(path.join(global, 'teams', 'review', 'team.yaml'), 'utf8').length > 0,
    "taking the project's folder away took the shared one with it",
  ).toBe(true);
  await wait(page, `${tRow('review')}.querySelectorAll('.c-place .pill').length===1`);
  expect(
    await read<string>(page, `${tRow('review')}.querySelector('.c-place .pill.on').textContent.trim()`),
    'the row stands on, want 共通',
  ).toBe('共通');

  // N8: 取り消し takes a row that is not there yet out and writes nothing.
  let before = await rows();
  await press(page, q('.toolbar [data-act=new]'));
  await wait(page, there2(draft, '.in-name'));
  expect(await rows(), 'the list holds rows with the new row open').toBe(before + 1);
  await press(page, `${draft}.querySelector('[data-act=discard]')`);
  await wait(page, `!document.querySelector('.tdef[data-name^="~new"]')`);
  expect(await rows(), '取り消し left rows').toBe(before);
  expect(
    readdirSync(path.join(app.proj, '.hergy', 'teams')).sort(),
    "取り消し left the project's teams as, want the fixture's and the walk's",
  ).toEqual(['core', 'core-2', 'docs', 'migrate', 'ops', 'research']);

  // R7: 削除 draws the band's number again: taking core-2's folder away leaves
  // one definition fewer, and the band follows the list.
  before = await rows();
  if ((await read<string>(page, `${tRow('core-2')}.classList.contains('open')+''`)) !== 'true') {
    await press(page, `${tRow('core-2')}.querySelector('.sum')`);
    await wait(page, `${tRow('core-2')}.classList.contains('open')`);
  }
  await press(page, `${tRow('core-2')}.querySelector('.dacts [data-act=del]')`);
  await wait(page, there2(tRow('core-2'), '[data-act=del-yes]'));
  await press(page, `${tRow('core-2')}.querySelector('[data-act=del-yes]')`);
  await wait(page, `!document.querySelector('.tdef[data-name="core-2"]')`);
  expect(await rows(), 'the delete left rows').toBe(before - 1);
  await wait(
    page,
    `document.querySelector('.screens .scr[data-screen=teams] .n').textContent===${JSON.stringify(String(before - 1))}`,
  );
});

test('TEAMS-Y2 the topology and the YAML switch', async ({ page, app }) => {
  // e2eTeamsTopology: the team.yaml preview the topology tab draws (spec-teams
  // B7, B8, C10) — a picked point's lines (Y1), a turn's line (Y4), a turn added
  // and one taken away with the head's own counts (Y2, Y3), the mission typed
  // while the preview moves under it (Y5), the preview against the file (Y6),
  // the narrow places' 編集 / YAML switch (Y7), and the card that leaves every
  // point uncovered (Y8).
  const panel = shape('core');
  const line = (find: string) => `${panel}.querySelector(${JSON.stringify(`.tprev .yl${find}`)})`;

  await page.goto(`${app.base}/teams?open=core`);
  await wait(page, there('#view .tdef'));
  await wait(page, there2(panel, '.tprev .ybox .yl'));
  // The preview is the text a save would write, and the head counts its lines (B7).
  expect(
    await read<string>(page, `${panel}.querySelector('.tprev .yph .vt').textContent`),
    "the preview's head reads, want 42 行",
  ).toBe('42 行');
  expect(
    await read<string>(page, `${panel}.querySelector('.tprev .yph .dirty')+''`),
    'the preview of an unchanged definition shows a change count',
  ).toBe('null');

  // Y1: picking analyst lines up its own member lines and every turn that comes
  // in or leaves it (B7).
  await press(page, `${panel}.querySelector('.gn.member[data-id=analyst]')`);
  await wait(page, there2(panel, '.gcard[data-sel=node]'));
  await wait(page, `${panel}.querySelectorAll('.tprev .yl.cur').length>0`);
  expect(
    await read<number>(page, `${panel}.querySelectorAll('.tprev .yl.cur').length`),
    'analyst lines up preview lines, want its 3 member lines and its 4 turns',
  ).toBe(7);

  // Y4: pressing a turn's line picks that line of the figure, and its card
  // names the two points (B7).
  await press(page, line('[data-a=impl-b][data-b=reviewer]'));
  await wait(page, there2(panel, '.gcard[data-sel=edge]'));
  expect(
    await read<string>(page, `${panel}.querySelector('.gcard[data-sel=edge]').dataset.k`),
    'pressing the impl-b → reviewer line picked',
  ).toBe('impl-b|reviewer');

  // Y3: taking that turn away marks the gap and counts one line less (B7).
  await press(page, `${panel}.querySelector('.gcard [data-act=turn-ask][data-a=impl-b][data-b=reviewer]')`);
  await wait(page, there2(panel, '.gcard [data-act=turn-del]'));
  await press(page, `${panel}.querySelector('.gcard [data-act=turn-del]')`);
  await wait(
    page,
    `${panel}.querySelector('.tprev .yph .dirty') && ${panel}.querySelector('.tprev .yph .dirty').textContent.trim()==='+0 −1'`,
  );
  expect(
    await read<number>(page, `${panel}.querySelectorAll('.tprev .yl.del,.tprev .yl.dele').length`),
    'the removed turn marks lines, want the gap alone',
  ).toBe(1);
  // 元に戻す puts the turn back, so the preview matches the file again.
  await press(page, `${panel}.querySelector('footer.pf [data-act=t-revert]')`);
  await wait(page, `!${panel}.querySelector('.tprev .yph .dirty')`);
  await wait(page, `${tRow('core')}.classList.contains('unsaved')===false`);

  // Y6: the preview is the saved team.yaml line for line, comments and all.
  const text = readFileSync(path.join(app.proj, '.hergy', 'teams', 'core', 'team.yaml'), 'utf8');
  const want = text.replace(/\n$/, '').split('\n');
  const got = (
    await read<string>(
      page,
      `[...document.querySelectorAll('.tdef[data-name="core"] .tprev .yl .yt')].map(function(t){return t.textContent}).join('\u0001')`,
    )
  ).split('\u0001');
  expect(got.length, `the preview holds lines, want the file's ${want.length}`).toBe(want.length);
  for (let i = 0; i < want.length; i++) {
    if (got[i] !== want[i] && !(got[i] === ' ' && want[i].trim() === '')) {
      expect(got[i], `the preview's line ${i + 1} reads, want ${want[i]}`).toBe(want[i]);
    }
  }

  // Y2: a turn added from analyst to reviewer stands at the end of the preview
  // as an added line, the head counts it, and the box was sent to it (B7).
  await press(page, `${panel}.querySelector('.gn.member[data-id=analyst]')`);
  await wait(page, there2(panel, '.gcard select[data-act-change=turn-add-sel]'));
  await choose(page, `${panel}.querySelector('.gcard select[data-act-change=turn-add-sel]')`, 'reviewer');
  await wait(page, there2(panel, '.gcard[data-sel=edge]'));
  await wait(page, `!!${line('.ins')} && ${line('.ins')}.textContent.indexOf('analyst -> reviewer')>=0`);
  expect(
    await read<string>(page, `${panel}.querySelector('.tprev .yph .dirty').textContent.trim()`),
    'the head reads after the turn was added, want +1 −0',
  ).toBe('+1 −0');
  expect(
    await read<string>(
      page,
      `(function(){var b=${panel}.querySelector('.tprev .ybox'),l=${line('.ins')};if(!b||!l)return false;var br=b.getBoundingClientRect(),lr=l.getBoundingClientRect();return lr.top>=br.top-1&&lr.bottom<=br.bottom+1;})()+''`,
    ),
    'the box was not sent to the line the turn added',
  ).toBe('true');
  // And picking that line picks the new turn of the figure (B7).
  await press(page, line('[data-a=analyst][data-b=reviewer]'));
  await wait(page, there2(panel, '.gcard[data-sel=edge]'));
  expect(
    await read<string>(page, `${panel}.querySelector('.gcard[data-sel=edge]').dataset.k`),
    'pressing the added line picked',
  ).toBe('analyst|reviewer');
  expect(
    await read<number>(page, `${panel}.querySelectorAll('.tprev .yl.cur').length`),
    "the added turn's line takes preview lines, want the one line it wrote",
  ).toBe(1);

  // Y5: the mission's line follows the typing, and the cursor stays in the field.
  const mission = 'E2E がプレビューで打った使命';
  const field = q('.tdef[data-name="core"] .in-mission');
  await focus(page, '.tdef[data-name="core"] .in-mission');
  await wait(page, `(document.activeElement===${field})`);
  await set(page, field, mission);
  await wait(
    page,
    `[...document.querySelectorAll('.tdef[data-name="core"] .tprev .yl')].some(function(l){return l.textContent.indexOf(${JSON.stringify(mission)})>=0})`,
  );
  expect(
    await read<string>(page, `(document.activeElement===${field})+''`),
    'typing the mission moved the cursor out of the field',
  ).toBe('true');
  expect(
    await read<string>(
      page,
      `[...document.querySelectorAll('.tdef[data-name="core"] .tprev .yl.ins')].some(function(l){return l.textContent.indexOf(${JSON.stringify(mission)})>=0})+''`,
    ),
    "the mission's typed line does not stand in the preview as an added line",
  ).toBe('true');
  await press(page, `${panel}.querySelector('footer.pf [data-act=t-revert]')`);
  await wait(page, `!${panel}.querySelector('.tprev .yph .dirty')`);

  // Y7: at 1100 or less the 編集 / YAML switch picks which column shows, the box
  // goes to the last changed line, and pressing a line comes back to 編集 with
  // that point picked (B7).
  await page.setViewportSize({ width: 1100, height: 900 });
  await wait(page, there2(panel, '[data-act=tmode][data-v=yaml]'));
  await press(page, `${panel}.querySelector('.gn.member[data-id=impl-a]')`);
  await wait(page, there2(panel, '.gcard[data-sel=node]'));
  await press(page, `${panel}.querySelector('[data-act=tmode][data-v=yaml]')`);
  await wait(page, `${panel}.closest('.panel').dataset.mode==='yaml'`);
  expect(
    await read<string>(page, `getComputedStyle(${panel}.querySelector('.pedit')).display`),
    '1100: the YAML switch left the figure showing',
  ).toBe('none');
  await wait(page, `${panel}.querySelector('.tprev .yl.cur')!==null`);
  await press(page, line('[data-node=reviewer]'));
  await wait(page, `${panel}.closest('.panel').dataset.mode==='edit'`);
  expect(
    await read<string>(page, `${panel}.querySelector('.gcard[data-sel=node]').dataset.id`),
    'pressing the reviewer line in the YAML column picked',
  ).toBe('reviewer');
  await press(page, `${panel}.querySelector('.gcard .x')`);
  await wait(page, `!${panel}.querySelector('.gcard')`);
  await press(page, `${panel}.querySelector('footer.pf [data-act=t-revert]')`);
  await wait(page, `!${panel}.querySelector('.tprev .yph .dirty')`);

  // Y8: at 1440 every point's card leaves every point uncovered, or stands below
  // the figure (B8).
  await page.setViewportSize({ width: 1440, height: 1000 });
  await wait(page, there2(panel, '.gwrap svg'));
  const cover = `(function(){var p=${panel},c=p.querySelector('.gcard');if(!c)return -1;var cr=c.getBoundingClientRect(),n=0;p.querySelectorAll('.gn .box').forEach(function(b){var r=b.getBoundingClientRect();if(Math.min(cr.right,r.right)-Math.max(cr.left,r.left)>0&&Math.min(cr.bottom,r.bottom)-Math.max(cr.top,r.top)>0)n++;});return n;})()`;
  const points = await read<string>(
    page,
    `[...document.querySelectorAll('.tdef[data-name="core"] .panel.team-p .gn')].map(function(g){return g.classList.contains('worker')?'w'+g.dataset.i:'n'+g.dataset.id}).join(' ')`,
  );
  for (const p of points.split(/\s+/).filter(Boolean)) {
    const pick =
      p[0] === 'w'
        ? `${panel}.querySelector('.gn.worker[data-i="${p.slice(1)}"]')`
        : `${panel}.querySelector('.gn[data-id="${p.slice(1)}"]')`;
    await press(page, pick);
    await wait(page, there2(panel, '.gcard'));
    const covered = await read<number>(page, cover);
    if (covered !== 0) {
      expect(
        await read<string>(page, `${panel}.querySelector('.gcard').classList.contains('below')+''`),
        `1440: the card of ${p} covers ${covered} points and does not stand below the figure`,
      ).toBe('true');
    }
    await press(page, `${panel}.querySelector('.gcard .x')`);
    await wait(page, `!${panel}.querySelector('.gcard')`);
  }
});
