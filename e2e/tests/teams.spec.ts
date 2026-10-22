// TestE2ETeams' four 大 items, written again in Playwright (d-3c9b):
// internal/app/e2e_teams_test.go's e2eTeamsAddCopyDelete (TEAMS-N1),
// e2eTeamsOpen + e2eTeamsWorkerPurpose + e2eTeamsGraphProblems +
// e2eTeamsSaveShape (TEAMS-P5), e2eTeamsTopology (TEAMS-Y2), and e2eTeamsOpen
// (TEAMS-V1), assertion for assertion.
import { test, expect, type Page } from '@playwright/test';
import { existsSync, readFileSync, readdirSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import {
  choose, focus, go, makeFixture, number, press, readFile, removeRoot, repo,
  setField, startServe, text, wait, waitFor, watch, type Fixture,
} from './harness';

// rowJS is a JS expression for the list row one definition name stands on.
function rowJS(name: string): string {
  return `[...document.querySelectorAll('#view .tdef')].find(function(r){return r.dataset.name===${JSON.stringify(name)}})`;
}
// panelJS is a JS expression for one row's shape panel.
function panelJS(name: string): string {
  return `document.querySelector(${JSON.stringify(`.tdef[data-name="${name}"] .panel.team-p`)})`;
}
// agreeJS is a JS expression for one row's agreement panel.
function agreeJS(name: string): string {
  return `document.querySelector(${JSON.stringify(`.tdef[data-name="${name}"] .panel.agree-p`)})`;
}

// teamsDiff is what one file gained and lost, counted by the text of the line.
function teamsDiff(before: string, after: string): [string[], string[]] {
  const count = (s: string) => {
    const m = new Map<string, number>();
    for (const l of s.split('\n')) m.set(l, (m.get(l) ?? 0) + 1);
    return m;
  };
  const bm = count(before);
  const am = count(after);
  const added: string[] = [];
  const out: string[] = [];
  for (const [l, n] of am) for (let i = 0; i < n - (bm.get(l) ?? 0); i++) added.push(l);
  for (const [l, n] of bm) for (let i = 0; i < n - (am.get(l) ?? 0); i++) out.push(l);
  added.sort();
  out.sort();
  return [added, out];
}

// teamsOpen is S4〜S6, P1〜P3・P9, V1: a row opens under itself with the shape
// panel first, the check column carries validate's sentences, the ルール tab
// draws the shared part, and a definition in both places says 効いている and
// 隠れている.
async function teamsOpen(page: Page, base: string): Promise<void> {
  await go(page, `${base}/teams`);
  await waitFor(page, '#view .tdef');

  // P1: pressing the row opens it right under itself, and the address keeps it.
  await press(page, `${rowJS('core')}.querySelector('.sum')`);
  await wait(page, `${rowJS('core')}.classList.contains('open')`);
  expect(await text(page, 'window.location.search'), 'opening the core row left open=core in the address').toContain('open=core');

  // S4: the mission's check list, the two tabs, and the shape panel below them.
  expect(await text(page, `${rowJS('core')}.querySelector('.detail > ul.mvm').tagName`), "the open row's first child").toBe('UL');
  expect(await text(page, `${rowJS('core')}.querySelector('.dtabs').nextElementSibling.className`), 'the shape panel under the tabs').toContain('team-p');
  expect(await text(page, `[...document.querySelectorAll(${JSON.stringify(`.tdef[data-name="core"] .dtabs .dtab`)})].map(function(b){return b.textContent.trim()}).join(',')`),
    'the tabs').toBe('トポロジー,ルール53');

  // S5: the shape panel is 頭 → 図 → 検査の列 → 足.
  const panel = panelJS('core');
  for (const want of ['.ph', '.gbody .gwrap svg', '.gbody .vlist', 'footer.pf [data-act=t-save]']) {
    expect(await number(page, `${panel}.querySelectorAll(${JSON.stringify(want)}).length`), `the shape panel at ${want}`).toBeGreaterThan(0);
  }
  expect(await text(page, `${panel}.querySelector('.ph .path').textContent`), "the shape panel's path").toMatch(/teams\/core\/team\.yaml$/);
  expect(await number(page, `${panel}.querySelectorAll('.gn.member').length`), 'the figure members').toBe(5);
  expect(await number(page, `${panel}.querySelectorAll('.gn.worker').length`), 'the figure workers').toBe(2);
  expect(await text(page, `[...document.querySelectorAll(${JSON.stringify(`.tdef[data-name="core"] .panel.team-p .gn.member.err`)})].map(function(g){return g.dataset.id}).join(',')`),
    'the members in problem').toBe('impl-a,impl-b');

  // V1: the check column carries validate's own sentences.
  expect(await text(page, `${panel}.querySelector('.vlist .vhead .vt').textContent`), "the check column's head").toBe('validate');
  expect(await text(page, `${panel}.querySelector('.vlist .vcount').textContent.trim()`), "the figure's own check").toBe('問題 2警告 1');
  expect(await number(page, `${panel}.querySelectorAll('.vlist .vitem').length`), 'the check column items').toBe(3);
  expect(await text(page, `(function(){var m=${panel}.querySelector('.vlist .vsub');return m?m.textContent:''})()`), "the role documents' head").toBe('ロールの文書');
  expect(await text(page, `${panel}.querySelector('.vlist .vitem .msg').textContent`), 'the first problem').toContain('BUILD-4');

  // P9: pressing one item of the check column picks its node and shows its card.
  await press(page, `${panel}.querySelector('.vlist .vitem[data-node]')`);
  await wait(page, `!!${panel}.querySelector('.gcard[data-sel=node]')`);
  expect(await text(page, `${panel}.querySelector('.gcard[data-sel=node]').dataset.id`), 'the first problem picked').toBe('impl-a');
  expect(await text(page, `${panel}.querySelector('.gcard .links .uchip').textContent`), "the card's role chip").toBe('ロール builder');
  expect(await text(page, `${panel}.querySelector('.gcard .tl .gsub').textContent`), "the card's head").toBe('core-impl-a · ターンを渡す相手');
  await press(page, `${panel}.querySelector('.gcard .x')`);
  await wait(page, `!${panel}.querySelector('.gcard')`);

  // P3: the ルール tab draws the shared part, with the team's own rules and none
  // of the name, copy or delete a role's own panel carries (S6).
  await press(page, `${rowJS('core')}.querySelector('.dtabs .dtab[data-t=agree]')`);
  await wait(page, `!!${agreeJS('core')}`);
  const agree = agreeJS('core');
  expect(await number(page, `${agree}.querySelectorAll('.rule').length`), 'the agreement panel draws no rules').toBeGreaterThan(0);
  expect(await text(page, `${agree}.querySelector('.ph .path').textContent`), "the agreement panel's path").toMatch(/teams\/core\/agreement\.yaml$/);
  expect(await number(page, `${agree}.querySelectorAll('[data-act=copy],[data-act=del],[data-act=dup],[data-f=name]').length`),
    "the team's agreement panel carries a name, copy or delete of its own").toBe(0);
  expect(await number(page, `getComputedStyle(${agree}.querySelector('.pmain')).gridTemplateColumns.split(' ').filter(Boolean).length`),
    '2000: the agreement panel columns').toBe(2);
  expect(await text(page, `[...document.querySelectorAll(${JSON.stringify(`.tdef[data-name="core"] .dtabs .dtab`)})].map(function(b){return b.dataset.t+'='+b.textContent.trim()}).join(',')`),
    "the open row's tabs").toBe('shape=トポロジー,agree=ルール53');
  expect(await number(page, `${agree}.querySelectorAll('.yprev .yl.warn').length`), "the agreement's preview warnings").toBe(1);
  expect(await number(page, `${agree}.querySelectorAll('.yprev .yl.err').length`), "the agreement's preview problems").toBe(0);
  expect(await text(page, `(${agree}.querySelector('.ph').textContent.indexOf('プロジェクト')>=0)+''`), "the agreement panel's head names a place").toBe('false');
  expect(await text(page, `${agree}.querySelector('.pf [data-act=revert]').hidden+''`), "the agreement's foot offers 元に戻す").toBe('true');

  // 1.3・1.4.5・1.9・R10: a definition whose names do not resolve is drawn like
  // one that reads.
  const docsPanel = panelJS('docs');
  await press(page, `${rowJS('docs')}.querySelector('.sum')`);
  await wait(page, `!!${docsPanel}.querySelector('.vlist .vitem .msg')`);
  expect(await number(page, `${docsPanel}.querySelectorAll('.gwrap svg, footer.pf [data-act=t-save]').length`),
    'the definition whose profile does not resolve draws figures and saves').toBe(2);
  expect(await text(page, `${docsPanel}.querySelector('.vlist .vcount').textContent.trim()`), "the docs panel's check").toBe('問題 1');
  expect(await text(page, `${docsPanel}.querySelector('.vlist .vitem .msg').textContent`), "the docs figure's problem").toContain('gemini-flash');
  expect(await text(page, `[...document.querySelectorAll(${JSON.stringify(`.tdef[data-name="docs"] .panel.team-p .gn.worker.err`)})].map(function(g){return g.dataset.i}).join(',')`),
    'the workers in problem').toBe('0');
  expect(await text(page, `[...document.querySelectorAll(${JSON.stringify(`.tdef[data-name="docs"] .panel.team-p .gn.worker.err`)})].map(function(g){return g.querySelector('.bdg text').textContent}).join(',')`),
    "the worker's badge").toBe('問題');
  expect(await text(page, `${docsPanel}.querySelector('.ph .state').textContent`), "the shared definition's state").toBe('効いている');
  expect(await text(page, `${rowJS('docs')}.querySelector('.panel.shadowed .state').textContent`), "the hidden panel's state").toBe('隠れている');
}

// teamsWorkerPurpose is R7: ＋ ワーカー opens the card with the cursor in the
// purpose, the purpose takes the whole word, and the card writes the check's own
// sentence rather than 問題 undefined.
async function teamsWorkerPurpose(page: Page, base: string): Promise<void> {
  await go(page, `${base}/teams?open=core`);
  await waitFor(page, `.tdef[data-name="core"] .panel.team-p .gwrap svg`);
  const panel = panelJS('core');
  const field = `${panel}.querySelector('.gcard[data-sel=worker] .in-purpose')`;

  await press(page, `${panel}.querySelector('.gn.member[data-id=lead]')`);
  await wait(page, `!!${panel}.querySelector('.gcard[data-sel=node]')`);
  await press(page, `${panel}.querySelector('.gcard [data-act=worker-add]')`);
  await wait(page, `!!${field}`);
  expect(await text(page, `(document.activeElement===${field})+''`), '＋ ワーカー left the cursor outside the purpose').toBe('true');
  await wait(page, `${panel}.querySelector('[data-act=t-save]').disabled+''==='true'`);
  expect(await text(page, `[...document.querySelectorAll(${JSON.stringify(`.tdef[data-name="core"] .panel.team-p .gcard .vm li`)})].map(function(li){return li.textContent}).join('|')`),
    "the new worker's card").toContain('purpose');
  expect(await text(page, `[...document.querySelectorAll(${JSON.stringify(`.tdef[data-name="core"] .panel.team-p .gcard .vm li`)})].map(function(li){return li.textContent}).join('|')`),
    "the new worker's card says undefined").not.toContain('undefined');
  const purpose = 'ログの整理';
  for (let i = 1; i <= [...purpose].length; i++) {
    const letters = [...purpose].slice(0, i).join('');
    await setField(page, field, letters);
    await wait(page, `(document.activeElement===${field})+''==='true'`);
    expect(await text(page, `${field}.value`), `after ${i} letters the purpose`).toBe(letters);
  }
  await wait(page, `${panel}.querySelector('footer.pf .dirty').textContent.trim()==='1 か所'`);
  await wait(page, `${panel}.querySelector('[data-act=t-save]').disabled+''==='false'`);
  expect(await text(page, `(function(){var u=${panel}.querySelector('.gcard[data-sel=worker] .vm');return u?u.textContent:''})()`),
    "the worker's card still holds its purpose's problem").not.toContain('purpose');
  await wait(page, `!!${panel}.querySelector('[data-act=t-revert]:not([hidden])')`);
  await press(page, `${panel}.querySelector('[data-act=t-revert]')`);
  await wait(page, `${panel}.querySelector('footer.pf .dirty').textContent.trim()===''`);
}

// teamsGraphProblems is R8・R9, with R11・R12: the card of a line that would take
// a member's last turn says the problem before it is made, the problem then
// stands on that member's own point, and the team's own agreement's warning
// opens that rule in the ルール tab.
async function teamsGraphProblems(page: Page, base: string): Promise<void> {
  await go(page, `${base}/teams?open=core`);
  await waitFor(page, `.tdef[data-name="core"] .panel.team-p .gwrap svg`);
  const panel = panelJS('core');
  const stop = `[...document.querySelectorAll(${JSON.stringify(`.tdef[data-name="core"] .panel.team-p .gcard .vm li`)})].some(function(li){return li.textContent.indexOf('analyst から出るターンがありません')>=0})`;

  // R8: analyst hands out three turns and hires no worker.
  for (const to of ['impl-a', 'impl-b', 'lead']) {
    await press(page, `${panel}.querySelector('.gn.member[data-id=analyst]')`);
    await wait(page, `!!${panel}.querySelector('.gcard[data-sel=node]')`);
    await press(page, `${panel}.querySelector('.gcard .row button[data-act=pick-edge][data-b=${to}]')`);
    await wait(page, `!!${panel}.querySelector('.gcard[data-sel=edge]')`);
    await press(page, `${panel}.querySelector('.gcard [data-act=turn-ask][data-a=analyst][data-b=${to}]')`);
    await wait(page, `!!${panel}.querySelector('.gcard [data-act=turn-del]')`);
    if (to === 'lead') {
      await wait(page, stop);
      await wait(page, `!!${panel}.querySelector('.gcard .gpl s')`);
      expect(await text(page, `${panel}.querySelector('.gcard .gpl s').textContent`), "the card's line that would leave").toBe('- core-lead（ロール lead）');
    } else {
      expect(await text(page, `(${stop})+''`), `the card of the ${to} turn already names the problem`).toBe('false');
    }
    await press(page, `${panel}.querySelector('.gcard [data-act=turn-del]')`);
    await wait(page, `!!${panel}.querySelector('.gcard .gdl .tag')`);
  }

  // R9: the problem of the draft stands on analyst's own point.
  await wait(page, `!!${panel}.querySelector('.gn.member[data-id=analyst].err')`);
  expect(await text(page, `${panel}.querySelector('.gn.member[data-id=analyst] .bdg text').textContent`), "analyst's badge").toBe('問題');
  expect(await text(page, `[...document.querySelectorAll(${JSON.stringify(`.tdef[data-name="core"] .panel.team-p .vlist .vitem`)})].filter(function(b){return b.textContent.indexOf('analyst から出るターンがありません')>=0}).map(function(b){return b.querySelector('.lno').textContent}).join(',')`),
    "the check column puts analyst's problem at").toBe('analyst');

  // R9: scout's turn to the human goes, so RES-6 warns, and its press opens that
  // rule in the ルール tab at 1440×1000.
  await page.setViewportSize({ width: 1440, height: 1000 });
  await go(page, `${base}/teams?open=research`);
  await waitFor(page, `.tdef[data-name="research"] .panel.team-p .gwrap svg`);
  const rp = panelJS('research');
  await press(page, `${rp}.querySelector('.gn.member[data-id=scout]')`);
  await wait(page, `!!${rp}.querySelector('.gcard[data-sel=node]')`);
  await press(page, `${rp}.querySelector('.gcard .row button[data-act=pick-edge][data-b=human]')`);
  await wait(page, `!!${rp}.querySelector('.gcard[data-sel=edge]')`);
  await press(page, `${rp}.querySelector('.gcard [data-act=turn-ask][data-a=scout][data-b=human]')`);
  await wait(page, `!!${rp}.querySelector('.gcard [data-act=turn-del]')`);
  await press(page, `${rp}.querySelector('.gcard [data-act=turn-del]')`);
  await wait(page, `!!${rp}.querySelector('.gcard .gdl .tag')`);
  const item = `[...document.querySelectorAll(${JSON.stringify(`.tdef[data-name="research"] .panel.team-p .vlist .vitem`)})].find(function(b){return b.textContent.indexOf('RES-6')>=0})`;
  await wait(page, `!!${item}`);
  expect(await text(page, `${rp}.querySelector('.vlist .vsub').textContent`), "the check column's first heading").toBe('チームのルール');
  expect(await text(page, `${item}.querySelector('.lno').textContent`), "the team agreement's warning stands at").toBe('agreement.yaml');
  await press(page, item);
  await wait(page, `!!${agreeJS('research')}.querySelector('.rule.open')`);
  expect(await text(page, `${agreeJS('research')}.querySelector('.rule.open .rid b').textContent`), 'the press opened the rule').toBe('RES-6');
  expect(await text(page, 'window.location.search'), 'opening the rule left agree=research in the address').toContain('agree=research');

  // R12: the rule the press opened stands in the window, and its body has the cursor.
  const above = `(function(){var p=${agreeJS('research')};var r=p.querySelector('.rule.open'),t=p.querySelector('.rule.open [data-f=text]'),f=p.querySelector('footer.pf');if(!r||!t||!f)return false;var rr=r.getBoundingClientRect(),tr=t.getBoundingClientRect(),fr=f.getBoundingClientRect();return rr.top>=0&&tr.top>rr.top&&tr.bottom<=fr.top})()`;
  const cursor = `(document.activeElement&&document.activeElement.dataset.f)==='text'`;
  await wait(page, above);
  await wait(page, cursor);

  // R12: the address opens the same rule the same way.
  await go(page, `${base}/teams?agree=research&rule=research.RES-6`);
  await waitFor(page, `.tdef[data-name="research"] .panel.agree-p .rule.open`);
  await wait(page, above);
  await wait(page, cursor);
  await page.setViewportSize({ width: 2000, height: 1100 });
}

// teamsSaveShape is F1・F2・F7 and P2・P6・P7: the mission's one line, one turn's
// one line with its 確かめ, and a broken definition whose mission can still be
// written because the save only stops when a new problem appears.
async function teamsSaveShape(page: Page, base: string, f: Fixture): Promise<void> {
  const at = path.join(f.proj, '.hergy', 'teams', 'core', 'team.yaml');
  let before = readFile(at);

  // F2: the mission alone moves, and the check rides on the typing (P2).
  await go(page, `${base}/teams?open=core`);
  await waitFor(page, `.tdef[data-name="core"] .panel.team-p footer.pf [data-act=t-save]`);
  const mission = 'E2E が書き換えた使命。';
  await setField(page, `document.querySelector('.in-mission')`, mission);
  await wait(page, `document.querySelectorAll(${JSON.stringify(`.tdef[data-name="core"] .dtabs .dtab[data-t=shape] .dot`)}).length>0`);
  await wait(page, `${panelJS('core')}.querySelector('footer.pf .dirty').textContent.trim()==='1 か所'`);
  await press(page, `${panelJS('core')}.querySelector('[data-act=t-save]')`);
  await wait(page, `!!${panelJS('core')}.querySelector('.ok')`);
  expect(await text(page, `${panelJS('core')}.querySelector('.ok').textContent`), 'the shape save line').toBe('保存しました');
  let after = readFile(at);
  let [added, removed] = teamsDiff(before, after);
  expect([added.length, removed.length], 'the mission save').toEqual([1, 1]);
  expect(added[0], 'the mission line').toBe(`mission: "${mission}"`);
  expect(removed[0].startsWith('mission: '), 'the lost mission line').toBe(true);
  before = after;

  // F1・P6・P7: one turn goes, its card shows the line that would leave and the
  // sentence that would appear, and the save writes the one line.
  const panel = panelJS('core');
  await press(page, `${panel}.querySelector('.gn.member[data-id=impl-b]')`);
  await wait(page, `!!${panel}.querySelector('.gcard[data-sel=node]')`);
  await press(page, `${panel}.querySelector('.gcard .row button[data-act=pick-edge][data-b=reviewer]')`);
  await wait(page, `!!${panel}.querySelector('.gcard[data-sel=edge]')`);
  await press(page, `${panel}.querySelector('.gcard [data-act=turn-ask][data-a=impl-b][data-b=reviewer]')`);
  await wait(page, `!!${panel}.querySelector('.gcard [data-act=turn-del]')`);
  await wait(page, `!!${panel}.querySelector('.gcard .gpl s')`);
  expect(await text(page, `${panel}.querySelector('.gcard .gpl s').textContent`), 'the line that would leave').toContain('core-reviewer');
  await wait(page, `[...document.querySelectorAll(${JSON.stringify(`.tdef[data-name="core"] .panel.team-p .gcard .vm li`)})].some(function(li){return li.textContent.indexOf('CORE-12')>=0})`);
  expect(await number(page, `(function(){var s=${panel}.querySelector('.gcard .gpl s');return s?[...s.parentElement.children].filter(function(n){return n.tagName==='S'&&n.textContent.indexOf('core-reviewer')>=0}).length:0})()`),
    'the card strikes the line that would leave').toBe(1);
  expect(await number(page, `[...${panel}.querySelectorAll('.gcard .vm li')].filter(function(li){return li.textContent.indexOf('core-reviewer')>=0}).length`),
    'the check column holds lines of the change as a problem').toBe(0);
  await press(page, `${panel}.querySelector('.gcard [data-act=turn-del]')`);
  await wait(page, `!!${panel}.querySelector('.gcard .gdl .tag')`);
  expect(await text(page, `${panel}.querySelector('.gcard .gdl .tag').textContent`), "the removed turn's tag").toBe('消した');
  await wait(page, `${panel}.querySelector('footer.pf .dirty').textContent.trim()==='1 か所'`);
  await press(page, `${panel}.querySelector('[data-act=t-save]')`);
  await wait(page, `!!${panel}.querySelector('.ok')`);
  after = readFile(at);
  [added, removed] = teamsDiff(before, after);
  expect([added.length, removed.length], 'the turn save').toEqual([0, 1]);
  expect(removed[0].trim(), "the turn's own line").toBe('- impl-b -> reviewer');
  expect(after, 'the turn save lost the mission').toContain(mission);

  // F7・R10: docs' team.yaml reads — only a profile its worker names is not in
  // profiles.yaml — so a mission edit adds no problem and the save goes.
  const docsPath = path.join(f.proj, '.hergy', 'teams', 'docs', 'team.yaml');
  const docsBefore = readFile(docsPath);
  await go(page, `${base}/teams/yaml?file=docs.project.team`);
  await waitFor(page, '.yed .ytext');
  await wait(page, `document.querySelector('.yed .path').textContent.indexOf('teams/docs/team.yaml')>=0`);
  await wait(page, `document.querySelectorAll('.yed .vitem').length>0`);
  const docsText = await text(page, `document.querySelector('.yed .ytext').value`);
  const docsMission = 'E2E が直した文書の使命。';
  const edited = docsText.replace('mission: "利用者向けの文書を整える"', `mission: "${docsMission}"`);
  expect(edited, 'the fixture no longer holds the mission the test moves').not.toBe(docsText);
  await setField(page, `document.querySelector('.yed .ytext')`, edited);
  await wait(page, `!document.querySelector('[data-act=yaml-save]').disabled`);
  await press(page, `document.querySelector('[data-act=yaml-save]')`);
  await wait(page, `!!document.querySelector('.yed .ok')`);
  const docsAfter = readFile(docsPath);
  [added, removed] = teamsDiff(docsBefore, docsAfter);
  expect([added.length, removed.length], 'the docs mission save').toEqual([1, 1]);
  expect(added[0], 'the docs mission line').toBe(`mission: "${docsMission}"`);

  // F7・R10: the same definition's shape panel offers its save now.
  await go(page, `${base}/teams?open=docs`);
  await waitFor(page, `.tdef[data-name="docs"] .panel.team-p footer.pf [data-act=t-save]`);
  const docsPanel = panelJS('docs');
  const docsMission2 = 'E2E が姿から直した使命。';
  await setField(page, `document.querySelector('.in-mission')`, docsMission2);
  await wait(page, `${docsPanel}.querySelector('footer.pf .dirty').textContent.trim()==='1 か所'`);
  expect(await text(page, `${docsPanel}.querySelector('[data-act=t-save]').disabled+''`), 'the docs shape save is shut').toBe('false');
  await press(page, `${docsPanel}.querySelector('[data-act=t-save]')`);
  await wait(page, `!!${docsPanel}.querySelector('.ok')`);
  const docsAfter2 = readFile(docsPath);
  [added, removed] = teamsDiff(docsAfter, docsAfter2);
  expect([added.length, removed.length], 'the docs shape save').toEqual([1, 1]);
  expect(added[0], 'the docs shape mission line').toBe(`mission: "${docsMission2}"`);
}

// teamsAddCopyDelete is B1〜B6 and N1〜N8: ＋ チーム定義を追加, a new row's name
// problems and the files a save would write, a duplicate, a copy into the other
// place, taking one place's folder away, and 取り消し.
async function teamsAddCopyDelete(page: Page, base: string, f: Fixture): Promise<void> {
  await page.setViewportSize({ width: 2000, height: 1100 });
  await go(page, `${base}/teams`);
  await waitFor(page, '#view .tdef');
  await wait(page, `document.querySelectorAll('#view .tdef').length===6`);

  const draft = `document.querySelector('.tdef[data-name^="~new"]')`;
  const rows = () => number(page, `document.querySelectorAll('#view .tdef').length`);

  // B1: the toolbar's button stands above the list.
  expect(await text(page, `document.querySelector('.toolbar [data-act=new]').textContent.trim()`), "the toolbar's add").toBe('＋ チーム定義を追加');

  // N1: pressing it puts a new row first, open, with the cursor in its name.
  await press(page, `document.querySelector('.toolbar [data-act=new]')`);
  await wait(page, `!!${draft}.querySelector('.in-name')`);
  expect(await text(page, `document.querySelector('#view .tdef').dataset.name.indexOf('~new')===0?'first':'no'`), 'the new row stands first').toBe('first');
  expect(await text(page, `(document.activeElement===${draft}.querySelector('.in-name'))+''`), 'the new row left the cursor outside its name field').toBe('true');
  expect(await text(page, `${draft}.querySelector('.c-place .seg button.on').textContent.trim()`), "the new row's place").toBe('プロジェクト');
  expect(await text(page, `${draft}.querySelector('.in-mission').value`), "the new row's mission").toBe('このチームの使命を 1 行で');
  await wait(page, `${draft}.querySelector('.c-name .vcount').textContent.indexOf('問題 1')>=0`);
  await wait(page, `${draft}.querySelector('.c-name .vcount').textContent.indexOf('警告')>=0`);
  expect(await text(page, `${draft}.querySelector('.c-name .vcount').textContent.trim()`), 'the new row counts').not.toContain('問題 2');
  expect(await text(page, `${draft}.querySelector('.in-name').classList.contains('bad')+''`), "the empty name's field is not marked").toBe('true');
  expect(await text(page, `[...document.querySelectorAll('.tdef[data-name^="~new"] .vlist .vitem')].map(function(v){return v.querySelector('.lno').textContent+':'+v.querySelector('.msg').textContent}).join('|')`),
    'the check column').toContain('名前:チーム定義名を書いてください');
  expect(await text(page, `${draft}.querySelector('[data-act=t-save]').disabled+''`), 'the save stands although the name is a problem').toBe('true');
  expect(await text(page, `${draft}.querySelector('footer.pf .dirty').textContent.trim()`), "the new row's foot").toBe('新規');
  expect(await number(page, `${draft}.querySelectorAll('footer.pf .also code').length`), "the new row's files").toBe(3);
  // N2: a name hergy new takes clears the problem and opens the save.
  await setField(page, `${draft}.querySelector('.in-name')`, 'ops');
  await wait(page, `${draft}.querySelector('.in-name').classList.contains('bad')+''==='false'`);
  await wait(page, `${draft}.querySelector('.c-name .vcount').textContent.indexOf('問題')<0`);
  await wait(page, `${draft}.querySelector('[data-act=t-save]').disabled+''==='false'`);
  expect(await number(page, `document.querySelectorAll('#view .tdef').length`), 'the list with the draft open').toBe(7);

  // N3: the save writes hergy new's own files, one letter apart from none.
  const scaffoldTeam = ['team.yaml', 'agreement.yaml'].map((name) => ({
    name,
    data: readFileSync(path.join(repo, 'internal', 'scaffold', 'files', 'ja', name), 'utf8'),
  }));
  const profilePath = path.join(f.proj, '.hergy', 'profiles.yaml');
  const beforeProfile = readFile(profilePath);
  await press(page, `${draft}.querySelector('.dtabs .dtab[data-t=agree]')`);
  await wait(page, `!!${draft}.querySelector('.panel.agree-p footer.pf')`);
  expect(await text(page, `${draft}.querySelector('.panel.agree-p footer.pf .dirty').textContent.trim()`), "the new row's rule foot").toBe('新規');
  expect(await text(page, `[...document.querySelectorAll('.tdef[data-name^="~new"] .panel.agree-p footer.pf .also code')].map(function(c){return c.textContent}).join(',')`),
    "the new row's rule foot files").toBe('team.yaml,agreement.yaml,profiles.yaml');
  await wait(page, `${draft}.querySelector('.panel.agree-p [data-act=save]').disabled+''==='false'`);
  await press(page, `${draft}.querySelector('.panel.agree-p [data-act=save]')`);
  await wait(page, `!!${rowJS('ops')}`);
  await wait(page, `!document.querySelector('.tdef[data-name^="~new"]')`);
  expect(await number(page, `document.querySelectorAll('#view .tdef').length`), 'the list after the save').toBe(7);
  expect(await text(page, `document.querySelector('#view .tdef').dataset.name`), 'the saved row stands first as').toBe('ops');
  for (const file of scaffoldTeam) {
    expect(readFile(path.join(f.proj, '.hergy', 'teams', 'ops', file.name)), `the saved ops/${file.name}`).toBe(file.data);
  }
  expect(readFile(profilePath), "the save rewrote the project's profiles.yaml")
    .toBe(`${beforeProfile}  claude-default:\n    kind: claude\n`);
  // R7: the band's number follows the save, and the row's own foot says 保存しました.
  await wait(page, `document.querySelector('.screens .scr[data-screen=teams] .n').textContent==='7'`);
  await wait(page, `${rowJS('ops')}.querySelector('footer.pf .ok')!==null`);
  expect(await text(page, `${rowJS('ops')}.querySelector('footer.pf .ok').textContent.trim()`), "the saved row's foot").toBe('保存しました');

  // N4: the same name saved into 共通 writes the shared place.
  await press(page, `document.querySelector('.toolbar [data-act=new]')`);
  await wait(page, `!!${draft}.querySelector('.in-name')`);
  await press(page, `${draft}.querySelector('.c-place .seg button[data-x=global]')`);
  await wait(page, `${draft}.querySelector('.c-place .seg button[data-x=global]').classList.contains('on')+''==='true'`);
  await setField(page, `${draft}.querySelector('.in-name')`, 'ops');
  await wait(page, `${draft}.querySelector('[data-act=t-save]').disabled+''==='false'`);
  expect(await text(page, `[...document.querySelectorAll('.tdef[data-name^="~new"] footer.pf .also code')].map(function(c){return c.textContent}).join(',')`),
    'the new row in 共通 names').toBe('team.yaml,agreement.yaml,lead.yaml,builder.yaml');
  await press(page, `${draft}.querySelector('[data-act=t-save]')`);
  await wait(page, `!document.querySelector('.tdef[data-name^="~new"]')`);
  const global = path.join(f.cfg, 'hergy');
  for (const file of scaffoldTeam) {
    expect(readFile(path.join(global, 'teams', 'ops', file.name)), `the shared ops/${file.name}`).toBe(file.data);
  }
  for (const name of ['lead.yaml', 'builder.yaml']) {
    const want = readFileSync(path.join(repo, 'internal', 'scaffold', 'files', 'ja', 'roles', name), 'utf8');
    expect(readFile(path.join(global, 'roles', name)), `the shared roles/${name}`).toBe(want);
  }
  expect(readFile(path.join(global, 'profiles.yaml')), 'the shared profiles.yaml was written').not.toContain('claude-default');
  await wait(page, `${rowJS('ops')}.querySelectorAll('.c-place .pill').length===2`);
  expect(await text(page, `${rowJS('ops')}.querySelector('.c-place .pill.on').textContent.trim()`), 'the row in both places holds in effect').toBe('プロジェクト');

  // N5: core's 複製 opens core-2 right under it, and its save writes core's own two files.
  await press(page, `${rowJS('core')}.querySelector('.sum')`);
  await wait(page, `${rowJS('core')}.classList.contains('open')+''==='true'`);
  await press(page, `${rowJS('core')}.querySelector('.dacts [data-act=dup]')`);
  await wait(page, `!!${draft}.querySelector('.in-name')`);
  expect(await text(page, `${draft}.querySelector('.in-name').value`), "the duplicate's name").toBe('core-2');
  expect(await text(page, `${rowJS('core')}.nextElementSibling.dataset.name.indexOf('~new')===0?'below':'no'`), 'the duplicate stands right under core').toBe('below');
  expect(await text(page, `(document.activeElement===${draft}.querySelector('.in-name'))+''`), 'the duplicate left the cursor outside its name field').toBe('true');
  await press(page, `${draft}.querySelector('[data-act=t-save]')`);
  await wait(page, `!!${rowJS('core-2')}`);
  await wait(page, `!document.querySelector('.tdef[data-name^="~new"]')`);
  for (const name of ['team.yaml', 'agreement.yaml']) {
    expect(readFile(path.join(f.proj, '.hergy', 'teams', 'core-2', name)), `the duplicate's ${name}`)
      .toBe(readFile(path.join(f.proj, '.hergy', 'teams', 'core', name)));
  }
  expect(await text(page, `${rowJS('core')}.nextElementSibling.dataset.name`), 'the saved duplicate stands at').toBe('core-2');

  // N6: review (共通) copied プロジェクトに leaves one row.
  await press(page, `${rowJS('review')}.querySelector('.sum')`);
  await wait(page, `${rowJS('review')}.classList.contains('open')+''==='true'`);
  expect(await text(page, `${rowJS('review')}.querySelector('.dacts [data-act=copy]').textContent.trim()`), "review's copy").toBe('プロジェクトにコピー');
  await press(page, `${rowJS('review')}.querySelector('.dacts [data-act=copy]')`);
  await wait(page, `!!${draft}.querySelector('.c-place .pill.pending')`);
  expect(await text(page, `${draft}.querySelector('.c-place .pill.pending').textContent.trim()`), "the copy's place").toBe('プロジェクト');
  expect(await number(page, `${draft}.querySelectorAll('.in-name').length`), 'the copy offers a name field').toBe(0);
  expect(await text(page, `${rowJS('review')}.nextElementSibling.dataset.name.indexOf('~new')===0?'below':'no'`), 'the copy stands right under review').toBe('below');
  await press(page, `${draft}.querySelector('[data-act=t-save]')`);
  await wait(page, `!document.querySelector('.tdef[data-name^="~new"]')`);
  for (const name of ['team.yaml', 'agreement.yaml']) {
    expect(readFile(path.join(f.proj, '.hergy', 'teams', 'review', name)), `the copy's ${name}`)
      .toBe(readFile(path.join(global, 'teams', 'review', name)));
  }
  await wait(page, `${rowJS('review')}.querySelectorAll('.c-place .pill').length===2`);
  expect(await text(page, `[...document.querySelectorAll(${JSON.stringify(`.tdef[data-name="review"] .c-place .pill`)})].map(function(p){return p.textContent.trim()+'='+p.className}).join(',')`),
    "review's places").toBe('プロジェクト=pill on,共通=pill shadow');

  // N7: 削除 takes the project's folder away, and the row stands on 共通 alone.
  await press(page, `${rowJS('review')}.querySelector('.dacts [data-act=del]')`);
  await wait(page, `!!${rowJS('review')}.querySelector('[data-act=del-yes]')`);
  await press(page, `${rowJS('review')}.querySelector('[data-act=del-yes]')`);
  await wait(page, `!document.querySelector(${JSON.stringify(`.tdef[data-name="review"] [data-act=del-yes]`)})`);
  expect(existsSync(path.join(f.proj, '.hergy', 'teams', 'review', 'team.yaml')), "the project's review/team.yaml still stands").toBe(false);
  expect(existsSync(path.join(global, 'teams', 'review', 'team.yaml')), 'taking the project folder away took the shared one').toBe(true);
  await wait(page, `${rowJS('review')}.querySelectorAll('.c-place .pill').length===1`);
  expect(await text(page, `${rowJS('review')}.querySelector('.c-place .pill.on').textContent.trim()`), 'the row stands on').toBe('共通');

  // N8: 取り消し takes a row that is not there yet out and writes nothing.
  let before = await rows();
  await press(page, `document.querySelector('.toolbar [data-act=new]')`);
  await wait(page, `!!${draft}.querySelector('.in-name')`);
  expect(await rows(), 'the list with the new row open').toBe(before + 1);
  await press(page, `${draft}.querySelector('[data-act=discard]')`);
  await wait(page, `!document.querySelector('.tdef[data-name^="~new"]')`);
  expect(await rows(), '取り消し left rows').toBe(before);
  expect(readdirSync(path.join(f.proj, '.hergy', 'teams')).sort(), "取り消し left the project's teams")
    .toEqual(['core', 'core-2', 'docs', 'migrate', 'ops', 'research']);

  // R7: 削除 draws the band's number again.
  before = await rows();
  if ((await text(page, `${rowJS('core-2')}.classList.contains('open')+''`)) !== 'true') {
    await press(page, `${rowJS('core-2')}.querySelector('.sum')`);
    await wait(page, `${rowJS('core-2')}.classList.contains('open')+''==='true'`);
  }
  await press(page, `${rowJS('core-2')}.querySelector('.dacts [data-act=del]')`);
  await wait(page, `!!${rowJS('core-2')}.querySelector('[data-act=del-yes]')`);
  await press(page, `${rowJS('core-2')}.querySelector('[data-act=del-yes]')`);
  await wait(page, `!document.querySelector(${JSON.stringify(`.tdef[data-name="core-2"]`)})`);
  expect(await rows(), 'the delete left rows').toBe(before - 1);
  await wait(page, `document.querySelector('.screens .scr[data-screen=teams] .n').textContent===${JSON.stringify(String(before - 1))}`);
}

// teamsTopology is the トポロジー tab's team.yaml preview: the lines a picked
// point owns, a press on a turn's line, a turn added and one taken away, the
// mission typed while the preview moves under it, the preview against the file,
// the narrow 編集 / YAML switch, and the card that leaves every point uncovered.
async function teamsTopology(page: Page, base: string, f: Fixture): Promise<void> {
  const panel = panelJS('core');
  const line = (find: string) => `${panel}.querySelector(${JSON.stringify(`.tprev .yl${find}`)})`;

  await page.setViewportSize({ width: 2000, height: 1100 });
  await go(page, `${base}/teams?open=core`);
  await waitFor(page, '#view .tdef');
  await wait(page, `!!${panel}.querySelector('.tprev .ybox .yl')`);
  expect(await text(page, `${panel}.querySelector('.tprev .yph .vt').textContent`), "the preview's head").toBe('42 行');
  expect(await text(page, `${panel}.querySelector('.tprev .yph .dirty')+''`), 'the preview of an unchanged definition shows a change count').toBe('null');

  // Y1: picking analyst lines up its own member lines and every turn that comes
  // in or leaves it.
  await press(page, `${panel}.querySelector('.gn.member[data-id=analyst]')`);
  await wait(page, `!!${panel}.querySelector('.gcard[data-sel=node]')`);
  await wait(page, `${panel}.querySelectorAll('.tprev .yl.cur').length>0`);
  expect(await number(page, `${panel}.querySelectorAll('.tprev .yl.cur').length`), 'analyst lines up preview lines').toBe(7);

  // Y4: pressing a turn's line picks that line of the figure.
  await press(page, line(`[data-a=impl-b][data-b=reviewer]`));
  await wait(page, `!!${panel}.querySelector('.gcard[data-sel=edge]')`);
  expect(await text(page, `${panel}.querySelector('.gcard[data-sel=edge]').dataset.k`), 'pressing the impl-b → reviewer line picked').toBe('impl-b|reviewer');

  // Y3: taking that turn away marks the gap and counts one line less.
  await press(page, `${panel}.querySelector('.gcard [data-act=turn-ask][data-a=impl-b][data-b=reviewer]')`);
  await wait(page, `!!${panel}.querySelector('.gcard [data-act=turn-del]')`);
  await press(page, `${panel}.querySelector('.gcard [data-act=turn-del]')`);
  await wait(page, `${panel}.querySelector('.tprev .yph .dirty') && ${panel}.querySelector('.tprev .yph .dirty').textContent.trim()==='+0 −1'`);
  expect(await number(page, `${panel}.querySelectorAll('.tprev .yl.del,.tprev .yl.dele').length`), 'the removed turn marks lines').toBe(1);
  await press(page, `${panel}.querySelector('footer.pf [data-act=t-revert]')`);
  await wait(page, `!${panel}.querySelector('.tprev .yph .dirty')`);
  await wait(page, `${rowJS('core')}.classList.contains('unsaved')+''==='false'`);

  // Y6: the preview is the saved team.yaml line for line, comments and all.
  const want = readFile(path.join(f.proj, '.hergy', 'teams', 'core', 'team.yaml')).replace(/\n$/, '').split('\n');
  const got = (await text(page, `[...document.querySelectorAll(${JSON.stringify(`.tdef[data-name="core"] .tprev .yl .yt`)})].map(function(t){return t.textContent}).join('\\u0001')`)).split('\u0001');
  expect(got.length, 'the preview holds lines').toBe(want.length);
  for (let i = 0; i < want.length; i++) {
    expect(got[i] === want[i] || (got[i] === ' ' && want[i].trim() === ''), `the preview's line ${i + 1}`).toBe(true);
  }

  // Y2: a turn added from analyst to reviewer stands at the end of the preview as
  // an added line, and the box was sent to it.
  await press(page, `${panel}.querySelector('.gn.member[data-id=analyst]')`);
  await wait(page, `!!${panel}.querySelector('.gcard select[data-act-change=turn-add-sel]')`);
  await choose(page, `${panel}.querySelector('.gcard select[data-act-change=turn-add-sel]')`, 'reviewer');
  await wait(page, `!!${panel}.querySelector('.gcard[data-sel=edge]')`);
  await wait(page, `${line('.ins')} && ${line('.ins')}.textContent.indexOf('analyst -> reviewer')>=0`);
  expect(await text(page, `${panel}.querySelector('.tprev .yph .dirty').textContent.trim()`), 'the head after the turn was added').toBe('+1 −0');
  expect(await text(page, `(function(){var b=${panel}.querySelector('.tprev .ybox'),l=${line('.ins')};if(!b||!l)return false;var br=b.getBoundingClientRect(),lr=l.getBoundingClientRect();return lr.top>=br.top-1&&lr.bottom<=br.bottom+1;})()+''`),
    'the box was not sent to the line the turn added').toBe('true');
  await press(page, line(`[data-a=analyst][data-b=reviewer]`));
  await wait(page, `!!${panel}.querySelector('.gcard[data-sel=edge]')`);
  expect(await text(page, `${panel}.querySelector('.gcard[data-sel=edge]').dataset.k`), 'pressing the added line picked').toBe('analyst|reviewer');
  expect(await number(page, `${panel}.querySelectorAll('.tprev .yl.cur').length`), "the added turn's line takes preview lines").toBe(1);

  // Y5: the mission's line follows the typing, and the cursor stays in the field.
  const mission = 'E2E がプレビューで打った使命';
  const field = `document.querySelector(${JSON.stringify(`.tdef[data-name="core"] .in-mission`)})`;
  await waitFor(page, `.tdef[data-name="core"] .in-mission`);
  await focus(page, `.tdef[data-name="core"] .in-mission`);
  await wait(page, `(document.activeElement===${field})+''==='true'`);
  await setField(page, field, mission);
  await wait(page, `[...document.querySelectorAll(${JSON.stringify(`.tdef[data-name="core"] .tprev .yl`)})].some(function(l){return l.textContent.indexOf(${JSON.stringify(mission)})>=0})`);
  expect(await text(page, `(document.activeElement===${field})+''`), 'typing the mission moved the cursor out of the field').toBe('true');
  expect(await text(page, `[...document.querySelectorAll(${JSON.stringify(`.tdef[data-name="core"] .tprev .yl.ins`)})].some(function(l){return l.textContent.indexOf(${JSON.stringify(mission)})>=0})+''`),
    "the mission's typed line does not stand in the preview as an added line").toBe('true');
  await press(page, `${panel}.querySelector('footer.pf [data-act=t-revert]')`);
  await wait(page, `!${panel}.querySelector('.tprev .yph .dirty')`);

  // Y7: at 1100 or less the 編集 / YAML switch picks which column shows.
  await page.setViewportSize({ width: 1100, height: 900 });
  await wait(page, `!!${panel}.querySelector('[data-act=tmode][data-v=yaml]')`);
  await press(page, `${panel}.querySelector('.gn.member[data-id=impl-a]')`);
  await wait(page, `!!${panel}.querySelector('.gcard[data-sel=node]')`);
  await press(page, `${panel}.querySelector('[data-act=tmode][data-v=yaml]')`);
  await wait(page, `${panel}.closest('.panel').dataset.mode==='yaml'`);
  expect(await text(page, `getComputedStyle(${panel}.querySelector('.pedit')).display`), '1100: the YAML switch left the figure showing').toBe('none');
  await wait(page, `${panel}.querySelector('.tprev .yl.cur')!==null`);
  await press(page, line(`[data-node=reviewer]`));
  await wait(page, `${panel}.closest('.panel').dataset.mode==='edit'`);
  expect(await text(page, `${panel}.querySelector('.gcard[data-sel=node]').dataset.id`), 'pressing the reviewer line in the YAML column picked').toBe('reviewer');
  await press(page, `${panel}.querySelector('.gcard .x')`);
  await wait(page, `!${panel}.querySelector('.gcard')`);
  await press(page, `${panel}.querySelector('footer.pf [data-act=t-revert]')`);
  await wait(page, `!${panel}.querySelector('.tprev .yph .dirty')`);

  // Y8: at 1440 every point's card leaves every point uncovered, or stands below
  // the figure.
  await page.setViewportSize({ width: 1440, height: 1000 });
  await wait(page, `!!${panel}.querySelector('.gwrap svg')`);
  const cover = `(function(){var p=${panel},c=p.querySelector('.gcard');if(!c)return -1;var cr=c.getBoundingClientRect(),n=0;p.querySelectorAll('.gn .box').forEach(function(b){var r=b.getBoundingClientRect();if(Math.min(cr.right,r.right)-Math.max(cr.left,r.left)>0&&Math.min(cr.bottom,r.bottom)-Math.max(cr.top,r.top)>0)n++;});return n;})()`;
  const points = await text(page, `[...document.querySelectorAll(${JSON.stringify(`.tdef[data-name="core"] .panel.team-p .gn`)})].map(function(g){return g.classList.contains('worker')?'w'+g.dataset.i:'n'+g.dataset.id}).join(' ')`);
  for (const p of points.split(/\s+/).filter(Boolean)) {
    const pick = p[0] === 'w'
      ? `${panel}.querySelector('.gn.worker[data-i="${p.slice(1)}"]')`
      : `${panel}.querySelector('.gn[data-id="${p.slice(1)}"]')`;
    await press(page, pick);
    await wait(page, `!!${panel}.querySelector('.gcard')`);
    if ((await number(page, cover)) !== 0) {
      expect(await text(page, `${panel}.querySelector('.gcard').classList.contains('below')+''`), `1440: the card of ${p} covers points and does not stand below the figure`).toBe('true');
    }
    await press(page, `${panel}.querySelector('.gcard .x')`);
    await wait(page, `!${panel}.querySelector('.gcard')`);
  }
}

test('TEAMS-N1 add, copy and delete', async ({ page }) => {
  const f = makeFixture('teams');
  const serve = await startServe(f);
  try {
    const said = watch(page, false);
    await teamsAddCopyDelete(page, serve.url, f);
    expect(said, 'the team screen logged console errors').toEqual([]);
  } finally {
    await serve.stop();
    removeRoot(f);
  }
});

test('TEAMS-P5 the definition a person edits', async ({ page }) => {
  const f = makeFixture('teams');
  const f2 = makeFixture('teams');
  const serve = await startServe(f);
  const serve2 = await startServe(f2);
  try {
    const said = watch(page, false);
    await teamsOpen(page, serve.url);
    await teamsWorkerPurpose(page, serve.url);
    await teamsGraphProblems(page, serve.url);
    await teamsSaveShape(page, serve2.url, f2);
    expect(said, 'the team screen logged console errors').toEqual([]);
  } finally {
    await serve2.stop();
    await serve.stop();
    removeRoot(f2);
    removeRoot(f);
  }
});

test('TEAMS-Y2 the topology and the YAML switch', async ({ page }) => {
  const f = makeFixture('teams');
  const serve = await startServe(f);
  try {
    const said = watch(page, false);
    await teamsTopology(page, serve.url, f);
    expect(said, 'the team screen logged console errors').toEqual([]);
  } finally {
    await serve.stop();
    removeRoot(f);
  }
});

test('TEAMS-V1 a definition whose names do not resolve', async ({ page }) => {
  const f = makeFixture('teams');
  const serve = await startServe(f);
  try {
    const said = watch(page, false);
    await teamsOpen(page, serve.url);
    expect(said, 'the team screen logged console errors').toEqual([]);
  } finally {
    await serve.stop();
    removeRoot(f);
  }
});
