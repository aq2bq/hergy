// ACTIVITY-A1, the opened team on the activity screen (n-4b22, d-36ef): a
// member the definition has and no agent is live for is not started from its
// row any more, and a member whose definition changed carries no mark of its
// own; the team carries 反映していない変更 N once, and the press opens the sheet
// narrowed to that team. Applying posts POST /api/apply, and the start's own
// sheet says how far it got. The answers are the test's, so what the screen makes
// of them is what is read.
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
// members the definition has, the second one with no agent, and the first one
// whose definition changed.
const team = {
  name: 'core', project: '/work/alpha', project_name: '注文 API',
  workspace: { id: 'w1', label: '本体' },
  summary: { definition: 'core', state: 'working' },
};
const unchangedTeam = {
  name: 'quiet', project: '/work/alpha', project_name: '注文 API',
  workspace: { id: 'w2', label: 'quiet' },
  summary: { definition: 'quiet', state: 'working' },
};
const sheet = {
  doc: {
    nodes: [
      { name: 'lead', herdr: 'core-lead', state: 'idle', at: '', stale: true },
      { name: 'spec2', herdr: '', state: 'absent', at: '', stale: false },
    ],
    hired: [],
    question: '',
  },
};

// applyAnswer is the apply reading: both members need something, one 送り直す
// and one 起動する.
const applyAnswer = () => ({
  herdr: { ok: true }, count: 2,
  teams: [{
    team: 'core', definition: 'core', project: '/work/alpha', project_name: '注文 API',
    members: [
      { name: 'core-lead', state: 'idle', how: 'resend', can_apply: true, changes: [{ kind: 'rule', document: '.hergy/roles/lead.yaml', rule: 'PAAF', text: '規則 PAAF を書き換えた' }] },
      { name: 'core-spec2', state: 'absent', how: 'start', can_apply: true, changes: [{ kind: 'prompt', text: 'メンバー spec2 を足した' }] },
    ],
  }],
});

// launch is the answer POST /api/apply gives: the start's own shape.
const launch = {
  id: 'l-8', action: 'apply', team: 'core', definition: 'core', place: 'project',
  project: '/work/alpha', project_name: 'alpha',
  workspace: { id: 'w1', label: '本体' }, tab: { id: 't1', label: 'core' },
  members: ['core-lead', 'core-spec2'], phase: 'starting', waiting: null, error: '',
  steps: [{ at: '', kind: 'member', text: 'core-spec2 を起動し、初期プロンプトを送りました' }],
};

test('the opened team carries the count once, and the sheet applies that team', async () => {
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
    if (url === '/api/apply/count') return { count: 2 };
    if (url === '/api/apply') return method === 'POST' ? { launch } : applyAnswer();
    if (url === '/api/activity') return { items: [team, unchangedTeam], herdr: { ok: true }, running: 2, waiting: 0 };
    if (url === '/api/activity/core') return sheet;
    if (url === '/api/activity/quiet') return { doc: { nodes: [], hired: [], question: '' } };
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
  await import('../src/activity/main');
  await import('../src/lib/apply');
  await settle();

  const tile = () => document.querySelector('.hq-tile[data-team="core"]');
  const quietTile = () => document.querySelector('.hq-tile[data-team="quiet"]');
  // No member row starts itself by hand, and none carries a mark of its own:
  // the team's count stands once, in the tile's action row.
  expect(tile().querySelectorAll('[data-hq=raise]').length).toBe(0);
  expect(tile().querySelectorAll('.apl-mark').length).toBe(0);
  expect(tile().querySelector('.hq-tactions a').textContent).toBe('チーム定義を開く');
  expect(tile().querySelector('[data-hq=stop]').textContent).toBe('チームを終了する…');
  const marks = tile().querySelectorAll('[data-hq=apply]');
  expect(marks.length).toBe(1);
  expect(marks[0].textContent).toContain('反映していない変更');
  expect(marks[0].querySelector('b').textContent).toBe('2');
  expect(quietTile().querySelector('[data-hq=apply]')).toBe(null);
  expect(quietTile().querySelector('.hq-tactions a').textContent).toBe('チーム定義を開く');
  expect(quietTile().querySelector('[data-hq=stop]').textContent).toBe('チームを終了する…');

  marks[0].click();
  await settle();

  // The sheet is narrowed to this team, with both members picked.
  const opened = document.querySelector('.apl-sheet');
  expect(opened).not.toBe(null);
  expect(opened.querySelector('.apl-h b').textContent).toBe('反映: core');
  expect([...opened.querySelectorAll('[data-apl-name]')].map((el) => el.dataset.aplName)).toEqual(['core-lead', 'core-spec2']);
  expect([...opened.querySelectorAll('.apl-how')].map((el) => el.textContent)).toEqual(['送り直す', '起動する']);

  opened.querySelector('[data-apl=send]').click();
  await settle();

  // The press is the apply of the picked members.
  const sent = asked.filter((a) => a.method === 'POST' && a.url === '/api/apply');
  expect(sent.length).toBe(1);
  expect(sent[0].body).toEqual({ members: ['core-lead', 'core-spec2'] });

  // The start's own sheet says how far it got.
  const run = document.querySelector('.hq-sheet');
  expect(run).not.toBe(null);
  expect(run.querySelector('[data-role=title]').textContent).toBe('反映しています');
  expect(run.querySelector('[data-role=steps]').textContent).toContain('core-spec2 を起動し、初期プロンプトを送りました');
});
