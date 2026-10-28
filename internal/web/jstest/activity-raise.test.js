// ACTIVITY-R1, 起きていないメンバーを起こす (r-5d1a, ac-d090): the opened team on the
// activity screen names the member its definition has that no agent is live for,
// and offers 起こす on that member's row. The press calls POST /api/hq/refresh for
// that member, and the start's own sheet says how far the raise got. The answers
// are the test's, so what the screen makes of them is what is read.
import { expect, test } from 'bun:test';
import { serve, settle } from './page.js';

// layout writes the activity page's body: the board's list and hergy's words.
function layout() {
  document.body.innerHTML = '<div class="wrap"><div class="bar"><nav class="screens" data-key="k">'
    + '<span class="scr" data-screen="teams"><span class="n">0</span></span>'
    + '<span class="scr on" data-screen="activity"><span class="n">0</span></span>'
    + '</nav></div><div id="tabs"></div><div id="list"></div></div>';
  const words = document.createElement('script');
  words.type = 'application/json';
  words.id = 'activity-words';
  words.textContent = '{}';
  document.body.appendChild(words);
}

// team is the activity reading's one team, and sheet its own reading: the two
// members the definition has, the second one with no agent.
const team = {
  name: 'core', project: '/work/alpha', project_name: '注文 API',
  workspace: { id: 'w1', label: '本体' },
  summary: { definition: 'core', state: 'working', last: '' },
};
const sheet = {
  doc: {
    nodes: [
      { name: 'lead', herdr: 'core-lead', state: 'idle', at: '' },
      { name: 'spec2', herdr: '', state: 'absent', at: '' },
    ],
    hired: [], tail: { lines: [] }, herdr: '',
  },
};

// launch is the answer POST /api/hq/refresh gives: the start's own shape.
const launch = {
  id: 'l-8', action: 'refresh', team: 'core', definition: 'core', place: 'project',
  project: '/work/alpha', project_name: 'alpha',
  workspace: { id: 'w1', label: '本体' }, tab: { id: 't1', label: 'core' },
  members: ['spec2'], phase: 'starting', waiting: null, error: '',
  steps: [{ at: '', kind: 'member', text: 'core-spec2 を起動し、初期プロンプトを送りました' }],
};

test('the opened team offers 起こす on the member that is not up, and the raise shows its progress', async () => {
  history.replaceState({}, '', '/activity');
  layout();
  const running = {
    herdr: { ok: true, text: '' },
    teams: [{
      team: 'core', definition: 'core', place: 'project', project: '/work/alpha', project_name: 'alpha',
      workspace: { id: 'w1', label: '本体' }, tab: { id: 't1', label: 'core' },
      members: [
        { name: 'lead', herdr: 'core-lead', live: true, state: 'idle' },
        { name: 'spec2', herdr: '', live: false, state: 'absent' },
      ],
      not_up: ['spec2'], workers: [], waiting: [], starting: false,
    }],
  };
  const asked = serve((method, url) => {
    if (method === 'POST' && url === '/api/hq/refresh') return { launch };
    if (url === '/api/activity') return { items: [team], herdr: { ok: true }, running: 1, waiting: 0 };
    if (url === '/api/activity/core') return sheet;
    if (url === '/api/hq/running') return running;
    if (url === '/api/hq/projects') {
      return { mode: 'listed', current: { path: '/work/alpha', name: 'alpha' }, projects: [{ path: '/work/alpha', name: 'alpha', listed: true, definitions: 1 }], global: { definitions: 0 } };
    }
    if (url === '/api/hq/workspaces') return { workspaces: [] };
    if (url === '/api/hq/teams') return { groups: [] };
    if (url === '/api/hq/roles') return { groups: [] };
    if (url === '/api/hq/profiles') return { groups: [] };
    if (url.startsWith('/api/hq/launches/')) return { launch: Object.assign({}, launch, { phase: 'done' }) };
    return undefined;
  });
  await import('../static/activity/main.js');
  await settle();

  // The member with no agent is on its own row, by name, with 起こす; the one that
  // is live is not offered.
  const detail = () => document.querySelector('.hq-detail');
  const row = () => detail().querySelector('li.absent');
  expect(row()).not.toBe(null);
  expect(row().querySelector('.nm').textContent).toBe('spec2');
  const raise = row().querySelector('[data-hq=raise]');
  expect(raise).not.toBe(null);
  expect(raise.dataset.member).toBe('spec2');
  expect(detail().querySelectorAll('[data-hq=raise]').length).toBe(1);

  raise.click();
  await settle();

  // The press is the raise of that member of that team.
  const sent = asked.filter((a) => a.method === 'POST' && a.url === '/api/hq/refresh');
  expect(sent.length).toBe(1);
  expect(sent[0].body).toEqual({ team: 'core', members: ['spec2'] });

  // The start's own sheet says how far the raise got.
  const opened = document.querySelector('.hq-sheet');
  expect(opened).not.toBe(null);
  expect(opened.querySelector('[data-role=title]').textContent).toBe('起こしています');
  expect(opened.querySelector('[data-role=steps]').textContent).toContain('core-spec2 を起動し、初期プロンプトを送りました');
});
