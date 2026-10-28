// SHOWCASE-A1, 反映 (n-4b22, d-36ef): the band counts the running members whose
// definition change is not in the team yet; the sheet names each member's way
// (送り直す・起動する・起動し直す・反映できない) and what changed, starts with the
// members a person can apply picked, and says that 起動し直す loses the
// conversation. A save only reads the number again, and sends nothing; the
// send posts the picked herdr names and hands the launch to the start's own
// sheet, which says how far it got. The answers are the test's, so what the part
// makes of them is what is read.
import { expect, test } from 'bun:test';
import { serve, settle } from './page.js';

// layout writes the band the part mounts its button in, and a place for the
// sheet and the notice.
function layout() {
  document.body.innerHTML = '<div class="wrap"><div class="bar"><nav class="screens"></nav></div><div id="view"></div></div>';
}

const change = (kind, document, rule, text) => ({ kind, document, rule, text });
// answer is the sheet's reading: core's four members, one of each way.
const answer = () => ({
  herdr: { ok: true },
  count: 4,
  teams: [{
    team: 'core', definition: 'core', project_name: '注文 API',
    notes: ['ワーカー w-1 のプロファイルが変わった。次に雇うワーカーから効く'],
    members: [
      { name: 'core-impl-a', role: 'builder', state: 'idle', how: 'resend', can_apply: true, changes: [change('rule', '.hergy/roles/builder.yaml', 'BUILD-5', '規則 BUILD-5 を書き換えた')] },
      { name: 'core-spec2', role: 'builder', state: 'absent', how: 'start', can_apply: true, changes: [change('prompt', '.hergy/teams/core/team.yaml', '', 'メンバー spec2 を足した')] },
      { name: 'core-lead', role: 'lead', state: 'idle', how: 'restart', can_apply: true, conversation_lost: true, changes: [change('prompt', '.hergy/teams/core/team.yaml', '', 'プロファイルを変えた')] },
      { name: 'core-busy', role: 'builder', state: 'active', how: 'cannot', can_apply: false, why_not: '仕事中なので、今は反映できません', changes: [] },
    ],
  }],
});

const launch = {
  id: 'l-9', action: 'apply', team: 'core', definition: 'core', place: 'project',
  project: '/work/alpha', project_name: '注文 API',
  workspace: { id: 'w1', label: '本体' }, tab: { id: 't1', label: 'core' },
  members: ['core-impl-a', 'core-spec2', 'core-lead'], phase: 'starting', waiting: null, error: '',
  steps: [{ at: '', kind: 'member', text: 'core-impl-a に初期プロンプトを送り直しました' }],
};

const bandCount = () => Number(document.querySelector('.bar .apl-bandbtn b')?.textContent ?? -1);
const boxes = () => [...document.querySelectorAll('.apl-sheet input[type=checkbox]')];
const checked = () => boxes().filter((el) => el.checked).map((el) => el.dataset.aplName).join(',');
const disabled = () => boxes().filter((el) => el.disabled).map((el) => el.dataset.aplName).join(',');

test('the band, the sheet and the apply hold the members a change is not in yet', async () => {
  history.replaceState({}, '', '/teams');
  layout();
  let count = 4;
  const asked = serve((method, url) => {
    if (url === '/api/apply/count') return { count };
    if (url === '/api/apply') return method === 'POST' ? { launch } : answer();
    if (url.startsWith('/api/hq/launches/')) return { launch: Object.assign({}, launch, { phase: 'done' }) };
    return undefined;
  });
  await import('../src/lib/apply');
  await settle();

  // The band's number, and the button that opens the sheet.
  expect(bandCount()).toBe(4);
  expect(document.querySelector('.bar .apl-bandbtn').hidden).toBe(false);
  document.querySelector('.bar .apl-bandbtn').click();
  await settle();

  // The sheet: the members a person can apply are picked; the one working is
  // not, and says why.
  expect(checked()).toBe('core-impl-a,core-spec2,core-lead');
  expect(disabled()).toBe('core-busy');
  expect(document.querySelector('.apl-sheet .apl-why').textContent).toBe('仕事中なので、今は反映できません');
  // Each member carries its way, and 起動し直す says the conversation is lost.
  const hows = [...document.querySelectorAll('.apl-sheet .apl-how')].map((el) => el.textContent);
  expect(hows).toEqual(['送り直す', '起動する', '起動し直す', '反映できない']);
  expect(document.querySelector('.apl-sheet .apl-lost').textContent).toBe('会話が消えます');
  // The team's own note stands under the team's heading (a worker's profile).
  expect(document.querySelector('.apl-sheet .apl-tnote').textContent).toContain('ワーカー w-1 のプロファイル');
  // The changed rule opens its own place.
  expect(document.querySelector('.apl-sheet .apl-chg').getAttribute('href')).toBe('/roles?open=builder&rule=BUILD-5');
  // Opening the sheet sent nothing.
  expect(asked.filter((one) => one.method === 'POST')).toEqual([]);

  // Closing it asks for nothing either.
  document.querySelector('.apl-sheet [data-apl=close]').click();
  await settle();
  expect(document.querySelector('.apl-sheet')).toBe(null);
  expect(asked.filter((one) => one.method === 'POST')).toEqual([]);

  // A save does not open the sheet and does not send: it reads the number again.
  count = 5;
  document.dispatchEvent(new CustomEvent('hergy:saved'));
  await settle();
  expect(document.querySelector('.apl-sheet')).toBe(null);
  expect(bandCount()).toBe(5);
  expect(asked.filter((one) => one.method === 'POST')).toEqual([]);

  // The band's button opens the sheet again, with the answer's own picks.
  document.querySelector('.bar .apl-bandbtn').click();
  await settle();
  expect(document.querySelector('.apl-sheet')).not.toBe(null);

  // Applying posts the picked herdr names, in the answer's own order, and hands
  // the launch to the start's own sheet.
  document.querySelector('.apl-sheet [data-apl=send]').click();
  await settle();
  const sent = asked.filter((one) => one.method === 'POST');
  expect(sent.length).toBe(1);
  expect(sent[0].body.members).toEqual(['core-impl-a', 'core-spec2', 'core-lead']);
  expect(document.querySelector('.apl-sheet')).toBe(null);
  expect(document.querySelector('.apl-toast').textContent).toBe('反映しました: core-impl-a・core-spec2・core-lead');
  const sheet = () => document.querySelector('.hq-sheet');
  expect(sheet()).not.toBe(null);
  expect(sheet().querySelector('[data-role=title]').textContent).toBe('起動しています');
  expect(sheet().querySelector('[data-role=steps]').textContent).toContain('core-impl-a に初期プロンプトを送り直しました');
});
