// APPLY-C1 — walked in a browser (r-a52a, d-7b45): the apply sheet tells a
// person what may be pressed and what is only news. The check and its name are
// one line; the state is a word and a sentence outside it, so pressing the
// sentence toggles nothing. The way hergy would apply a member is the check's
// own name (a noun), and a member it cannot apply has no check at all. The
// handed state reads 人間の返事待ち in the sheet, on the activity screen and in
// its figure, as hergy status says it.
import { test, expect } from './fixtures';
import { press, readTo, wait } from './helpers';

test.use({ reads: 'activity' });

const change = (kind: string, document: string, rule: string, text: string) => ({ kind, document, rule, text });

// answer is the sheet's reading: one member of each way, and two hergy cannot
// apply now.
const answer = {
  herdr: { ok: true },
  count: 5,
  teams: [{
    team: 'core', definition: 'core', project_name: '注文 API',
    notes: [],
    members: [
      { name: 'core-lead', role: 'lead', state: 'handed', how: 'resend', can_apply: true, changes: [change('rule', '.hergy/roles/lead.yaml', 'PAAF', '規則 PAAF を書き換えた')] },
      { name: 'core-impl-a', role: 'builder', state: 'idle', how: 'restart', can_apply: true, conversation_lost: true, changes: [change('prompt', '.hergy/teams/core/team.yaml', '', 'プロファイルを変えた')] },
      { name: 'core-spec2', role: 'builder', state: 'absent', how: 'start', can_apply: true, changes: [change('prompt', '.hergy/teams/core/team.yaml', '', 'メンバー spec2 を足した')] },
      { name: 'core-busy', role: 'builder', state: 'active', how: 'cannot', can_apply: false, why_not: 'core-busy は仕事中か許可待ちなので、今は反映できません', changes: [] },
    ],
  }],
};

// where is what the three places say about the handed state: the activity's
// first tier, the legend under the tile, and the tile's own figure.
const where = `(function(){
  var at=[...document.querySelectorAll('.hq-at .st')].map(function(e){return e.textContent});
  var legend=[...document.querySelectorAll('.hq-tc')].map(function(e){return e.textContent}).join('|');
  var fig=[...document.querySelectorAll('.hq-n.handed title')].map(function(e){return e.textContent}).join('|');
  return {at:at, legend:legend, fig:fig, handed:document.querySelectorAll('.hq-n.handed').length};
})()`;

// sheet is the apply sheet as a person reads it: the check-line of each member,
// the state's word and sentence with whether it stands in a label and how its
// frame reads, the way's words, and whether the disabled member has a check.
const sheet = `(function(){
  var s=document.querySelector('.apl-sheet');
  if(!s) return null;
  var rows=[...s.querySelectorAll('.apl-m')].map(function(row){
    var st=row.querySelector('.apl-state'), nm=row.querySelector('.apl-mm code');
    var box=row.querySelector('.apl-mm input');
    return {
      name:nm?nm.textContent:'',
      state:st?st.textContent:'',
      word:st?st.querySelector('.apl-st').textContent:'',
      inLabel:!!(st&&st.closest('label')),
      border:st?getComputedStyle(st).borderTopWidth:'',
      check:!!box,
      checked:!!(box&&box.checked),
      cursor:getComputedStyle(row.querySelector('.apl-mm')).cursor,
      how:row.querySelector('.apl-how')?row.querySelector('.apl-how').textContent:''
    };
  });
  var text=s.textContent;
  return {rows:rows,
    lost:[...s.querySelectorAll('.apl-cost.lost')].map(function(e){return e.textContent}).join('|'),
    handed:!!text.match(/人間の返事待ち/),
    old:!!text.match(/人間に渡した|人間の番|送り直す|起動し直す/),
    intro:text.indexOf('チェックしたメンバーに')>=0};
})()`;

test('the sheet separates what is pressed from what is told, and handed reads one way', async ({ page, app }) => {
  await page.route('**/api/apply/count', (route) => route.fulfill({ json: { count: 5 } }));
  await page.route('**/api/apply', (route) => route.fulfill({ json: answer }));
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(`${app.base}/activity`);
  await wait(page, `!!document.querySelector('.hq-tile') && !!document.querySelector('.bar .apl-bandbtn')`);
  await wait(page, `!document.querySelector('.bar .apl-bandbtn').hidden`);

  // The activity says 人間の返事待ち in its first tier, in the legend under the
  // tile and in the tile's figure.
  await readTo(page, `(function(){var w=${where};return [w.handed>0, w.at.indexOf('人間の返事待ち')>=0, w.legend.indexOf('人間の返事待ち')>=0, w.fig.indexOf('人間の返事待ち')>=0];})()`,
    'the activity never said 人間の返事待ち in all three places').toEqual([true, true, true, true]);

  // The band's button opens the sheet on the reading the serve gave it.
  await press(page, `document.querySelector('.bar .apl-bandbtn')`);
  await wait(page, `!!document.querySelector('.apl-sheet')`);

  // Every member's state is a word and a sentence, outside any label, with no
  // frame; the way is a noun inside the check's line, and 起動し直し says the
  // conversation goes.
  await readTo(page, `(function(){var s=${sheet};return s.rows.map(function(r){return [r.name, r.word, r.inLabel, r.border, r.check, r.cursor, r.how];});})()`,
    'the sheet never drew every member the way d-7b45 says').toEqual([
    ['core-lead', '人間の返事待ち', false, '0px', true, 'pointer', '送り直し（会話は残る）'],
    ['core-impl-a', '止まっている', false, '0px', true, 'pointer', '起動し直し（会話が消える）'],
    ['core-spec2', '動いていない', false, '0px', true, 'pointer', '起動（新しいペインで始める）'],
    ['core-busy', '仕事中', false, '0px', false, 'default', '反映できない'],
  ]);
  await readTo(page, `(function(){var s=${sheet};return [s.intro, s.handed, s.old, s.lost];})()`,
    'the sheet never said the state and the ways the way d-7b45 says').toEqual([true, true, false, '（会話が消える）']);

  // The state's sentence is not the check: pressing it toggles nothing, and
  // pressing the check's line does.
  await press(page, `document.querySelector('.apl-sheet [data-apl-name="core-impl-a"]').closest('label')`);
  await readTo(page, `document.querySelector('.apl-sheet [data-apl-name="core-impl-a"]').checked`,
    'the check did not go off when its own line was pressed').toBe(false);
  await press(page, `[...document.querySelectorAll('.apl-sheet .apl-state')].find(function(p){return p.closest('.apl-m').querySelector('code').textContent==='core-impl-a'})`);
  await readTo(page, `document.querySelector('.apl-sheet [data-apl-name="core-impl-a"]').checked`,
    'pressing the state sentence toggled the check').toBe(false);
  await press(page, `[...document.querySelectorAll('.apl-sheet .apl-mm')].find(function(l){return l.querySelector('code').textContent==='core-impl-a'})`);
  await readTo(page, `document.querySelector('.apl-sheet [data-apl-name="core-impl-a"]').checked`,
    'the check did not come back when its line was pressed').toBe(true);

  // The member hergy cannot apply knows nothing of a check: the reason is a
  // sentence of its own.
  await readTo(page, `document.querySelector('.apl-sheet .apl-m.no .apl-why').textContent`,
    'the reason a member cannot be applied never stood on its own').toContain('今は反映できません');
});
