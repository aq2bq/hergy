// Each waiting member has one row in its team card, with the question and pane action.
import { expect, test } from 'bun:test';
import { serve, settle, later } from './page.js';

function layout() {
  document.body.innerHTML = '<div class="wrap"><div class="bar"></div><div id="list"></div></div>';
  const words = document.createElement('script');
  words.type = 'application/json';
  words.id = 'activity-words';
  words.textContent = '{}';
  document.body.appendChild(words);
}

const team = (name, state, since = null, turn = null) => ({
  name, project: '', project_name: '注文 API',
  workspace: { id: 'w1', label: '本体' },
  summary: { definition: name, state, since, turn },
});
const sheet = (nodes, question = '') => ({ doc: { nodes, hired: [], question } });

test('cards show every waiting member once and keep the pane action states', async () => {
  history.replaceState({}, '', '/activity');
  layout();
  const at = (minutes) => new Date(2000, 9, 17, 11, minutes).getTime();
  const focused = later();
  let fail = false;
  const asked = serve((method, url) => {
    if (url === '/api/activity') return {
      items: [team('core', 'handed', { seconds: 120, at: at(1), lower: false }), team('migrate', 'waiting-for-permission', { seconds: 60, at: at(3), lower: true }), team('stalled', 'stalled', null, { holder: 'lead', from: 'human', text: 'turn: lead' })],
      herdr: { ok: true }, running: 3, waiting: 3,
    };
    if (url === '/api/activity/core') return sheet([
      { name: 'lead', herdr: 'core-lead', state: 'handed', at: at(1), question: 'Continue with the change?' },
      { name: 'reviewer', herdr: 'core-reviewer', state: 'waiting-for-permission', at: at(2), question: 'Allow this command?' },
      ...['working', 'idle', 'not-running'].map((state) => ({ name: state, herdr: `core-${state}`, state })),
    ]);
    if (url === '/api/activity/migrate') return sheet([
      { name: 'tester', herdr: 'migrate-tester', state: 'waiting-for-permission', at: at(3) },
    ], 'Trust this folder?');
    if (url === '/api/activity/stalled') return sheet([{ name: 'lead', herdr: 'stalled-lead', state: 'idle', at: null }]);
    if (url === '/api/activity/focus') {
      if (fail) throw new Error('herdr でペインへ移動できませんでした');
      return focused.promise;
    }
    return undefined;
  });
  await import('../src/activity/main');
  await settle();

  const row = (team, member) => document.querySelector(`.hq-tile[data-team="${team}"] .hq-wait[data-member="${member}"]`);
  expect(document.querySelectorAll('.hq-wait')).toHaveLength(3);
  expect(document.querySelector('.hq-att')).toBe(null);
  for (const [name, member, question] of [
    ['core', 'core-lead', 'Continue with the change?'],
    ['core', 'core-reviewer', 'Allow this command?'],
    ['migrate', 'migrate-tester', 'Trust this folder?'],
  ]) {
    const waiting = row(name, member);
    expect(waiting.querySelector('.st')).toBe(null);
    expect(waiting.querySelector('code').textContent).toBe(member);
    expect(waiting.querySelector('.hq-question').textContent).toBe(question);
    expect(waiting.querySelector('.hq-where').textContent).toBe(`ワークスペース 本体 · タブ ${name} · ペイン ${member}`);
    expect(waiting.querySelector('.hq-tl')).toBe(null);
  }
  for (const [name, state, since] of [
    ['core', '人間の返事待ち', '2 分 10/17 11:01 から'],
    ['migrate', '許可待ち', '1 分以上 10/17 11:03 より前'],
  ]) {
    const tile = document.querySelector(`.hq-tile[data-team="${name}"]`);
    expect(tile.querySelector('.hq-th .st').textContent).toBe(state);
    expect(tile.querySelectorAll('.hq-tl')).toHaveLength(1);
    expect(tile.querySelector('.hq-th .hq-tl').textContent).toBe(since);
  }
  expect(document.querySelector('.hq-tile[data-team="stalled"] .hq-wait')).toBe(null);
  expect(document.querySelector('.hq-tile[data-team="stalled"] .hq-why').textContent).not.toContain('誰もターンを持っていません');
  expect(document.querySelector('.hq-tile[data-team="stalled"] .hq-why').textContent).not.toContain('人間の返事待ちのメンバーはいません');

  const button = row('core', 'core-lead').querySelector('button');
  button.click();
  await settle();
  expect(button.textContent).toBe('移動中…');
  expect(button.disabled).toBe(true);
  expect(button.getAttribute('aria-busy')).toBe('true');
  expect(row('core', 'core-reviewer').querySelector('button').disabled).toBe(true);
  expect(asked.filter((r) => r.url === '/api/activity/focus')).toEqual([
    { method: 'POST', url: '/api/activity/focus', body: { team: 'core', member: 'core-lead' } },
  ]);
  focused.resolve({ ok: true });
  await settle();
  expect(button.textContent).toBe('herdr で移動しました');
  expect(button.disabled).toBe(false);

  fail = true;
  const other = row('core', 'core-reviewer').querySelector('button');
  other.click();
  await settle();
  expect(other.disabled).toBe(false);
  expect(other.textContent).toBe('herdr でペインへ移動');
  expect(document.querySelector('.hq-tile[data-team="core"] [role="alert"]').textContent).toContain('herdr でペインへ移動できませんでした');
});
