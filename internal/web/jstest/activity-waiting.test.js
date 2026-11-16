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

const team = (name, state) => ({
  name, project: '', project_name: '注文 API',
  workspace: { id: 'w1', label: '本体' },
  summary: { definition: name, state },
});
const sheet = (nodes, question = '') => ({ doc: { nodes, hired: [], question } });

test('cards show every waiting member once and keep the pane action states', async () => {
  history.replaceState({}, '', '/activity');
  layout();
  const focused = later();
  let fail = false;
  const asked = serve((method, url) => {
    if (url === '/api/activity') return {
      items: [team('core', 'turn'), team('migrate', 'answer'), team('stalled', 'stalled')],
      herdr: { ok: true }, running: 3, waiting: 3,
    };
    if (url === '/api/activity/core') return sheet([
      { name: 'lead', herdr: 'core-lead', state: 'handed', since_at: '11:01', question: 'Continue with the change?' },
      { name: 'reviewer', herdr: 'core-reviewer', state: 'blocked', since_at: '11:02', question: 'Allow this command?' },
      ...['active', 'idle', 'absent'].map((state) => ({ name: state, herdr: `core-${state}`, state })),
    ]);
    if (url === '/api/activity/migrate') return sheet([
      { name: 'tester', herdr: 'migrate-tester', state: 'blocked', at: '11:03' },
    ], 'Trust this folder?');
    if (url === '/api/activity/stalled') return sheet([{ name: 'lead', herdr: 'stalled-lead', state: 'idle', at: '' }]);
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
  for (const [name, member, state, question, since] of [
    ['core', 'core-lead', '人間の返事待ち', 'Continue with the change?', '11:01'],
    ['core', 'core-reviewer', '許可待ち', 'Allow this command?', '11:02'],
    ['migrate', 'migrate-tester', '許可待ち', 'Trust this folder?', '11:03'],
  ]) {
    const waiting = row(name, member);
    expect(waiting.querySelector('.st').textContent).toBe(state);
    expect(waiting.querySelector('code').textContent).toBe(member);
    expect(waiting.querySelector('.hq-question').textContent).toBe(question);
    expect(waiting.querySelector('.hq-where').textContent).toBe(`ワークスペース 本体 · タブ ${name} · ペイン ${member}`);
    expect(waiting.querySelector('.hq-tl').textContent).toBe(since);
  }
  expect(document.querySelector('.hq-tile[data-team="stalled"] .hq-wait')).toBe(null);
  expect(document.querySelector('.hq-tile[data-team="stalled"] .hq-why').textContent).toContain('誰もターンを持っていません');

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
