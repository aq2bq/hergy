// TEAMS-A1, a running team with a change not applied yet (n-4b22, d-36ef): the
// 動き column names the members the definition has that no agent is live for,
// and offers 反映していない変更 N instead of starting them one by one. The press
// opens the sheet narrowed to that team, and applying posts POST /api/apply for
// the members picked there; the start's own sheet says how far it got. The
// answers are the test's, so what the column and the sheet make of them is what
// is read.
import { expect, test } from 'bun:test';
import { serve, settle } from './page.js';

function page() {
  document.body.innerHTML = '<div class="wrap"><div class="bar"><nav class="screens" data-key="k">'
    + '<span class="scr on" data-screen="teams"><span class="n">0</span></span>'
    + '<span class="scr" data-screen="activity"><span class="n">0</span></span>'
    + '</nav></div><div id="tabs"></div><div id="view"></div></div>';
  const data = document.createElement('script');
  data.type = 'application/json';
  data.id = 'teams-data';
  data.textContent = JSON.stringify({
    dirs: { project: '.hergy/teams', global: '~/.config/hergy/teams' },
    profiles: ['coder'],
    hq: {
      projects: { mode: 'listed', current: { path: '/work/alpha', name: 'alpha' }, projects: [{ path: '/work/alpha', name: 'alpha', listed: true, definitions: 1 }] },
      teams: [{
        project: { path: '/work/alpha', name: 'alpha', display: '注文 API' },
        list: { items: [{
          name: 'core', effective: 'project',
          places: { project: { path: '.hergy/teams/core/team.yaml', hash: 'h1', doc: {
            mission: '注文 API を作り直す',
            members: [
              { name: 'lead', role: 'lead', profile: 'coder' },
              { name: 'builder', role: 'builder', profile: 'coder' },
              { name: 'spec2', role: 'builder', profile: 'coder' },
            ],
            workers: [], turns: [['human', 'lead'], ['lead', 'human']],
          } } },
        }] },
      }],
      roles: [], profiles: [],
    },
    team: { items: [], files: [] },
  });
  document.body.appendChild(data);
}

// running is the headquarters' reading: core is up with its two agents live, and
// spec2 is written in the definition but no agent is live for it.
const running = () => ({
  herdr: { ok: true, text: '' },
  teams: [{
    team: 'core', definition: 'core', place: 'project', project: '/work/alpha', project_name: 'alpha',
    workspace: { id: 'w1', label: '本体' }, tab: { id: 't1', label: 'core' },
    members: [
      { name: 'lead', herdr: 'core-lead', live: true, state: 'idle' },
      { name: 'builder', herdr: 'core-builder', live: true, state: 'idle' },
      { name: 'spec2', herdr: '', live: false, state: 'absent' },
    ],
    not_up: ['spec2'], workers: [], waiting: [], starting: false,
  }],
});

// applyAnswer is the apply reading: the member the definition added is started
// by the apply, and the lead's rule changed.
const applyAnswer = () => ({
  herdr: { ok: true }, count: 2,
  teams: [{
    team: 'core', definition: 'core', project: '/work/alpha', project_name: 'alpha',
    members: [
      { name: 'core-spec2', state: 'absent', how: 'start', can_apply: true, changes: [{ kind: 'prompt', text: 'メンバー spec2 を足した' }] },
      { name: 'core-lead', state: 'idle', how: 'resend', can_apply: true, changes: [{ kind: 'rule', document: '.hergy/roles/lead.yaml', rule: 'LEAD-1', text: '規則 LEAD-1 を書き換えた' }] },
    ],
  }],
});

// launch is the answer POST /api/apply gives: the start's own shape.
const launch = (over) => Object.assign({
  id: 'l-7', action: 'apply', team: 'core', definition: 'core', place: 'project',
  project: '/work/alpha', project_name: 'alpha',
  workspace: { id: 'w1', label: '本体' }, tab: { id: 't1', label: 'core' },
  members: ['core-spec2', 'core-lead'], phase: 'starting', waiting: null, error: '',
  steps: [{ at: '', kind: 'member', text: 'core-spec2 を起動し、初期プロンプトを送りました' }],
}, over || {});

// band answers the reads every screen makes of the headquarters.
const band = (method, url) => {
  if (url === '/api/hq/running') return running();
  if (url === '/api/hq/projects') {
    return { mode: 'listed', current: { path: '/work/alpha', name: 'alpha' }, projects: [{ path: '/work/alpha', name: 'alpha', listed: true, definitions: 1 }], global: { definitions: 0 } };
  }
  if (url === '/api/hq/workspaces') return { workspaces: [] };
  if (url === '/api/hq/teams') return { groups: [] };
  if (url === '/api/hq/roles') return { groups: [] };
  if (url === '/api/hq/profiles') return { groups: [] };
  if (url === '/api/apply/count') return { count: 2 };
  if (url === '/api/apply') return method === 'POST' ? { launch: launch() } : applyAnswer();
  if (url.startsWith('/api/team/check')) return { problems: [], members: [] };
  if (url.startsWith('/api/role')) return { items: [] };
  if (url.startsWith('/api/hq/launches/')) return { launch: launch({ phase: 'done' }) };
  return undefined;
};

test('the 動き column carries the count a change is not applied yet, and the sheet is that team only', async () => {
  history.replaceState({}, '', '/teams');
  page();
  let asked = serve(band);
  await import('../src/teams/main');
  await import('../src/lib/apply');
  await settle();

  const cell = () => document.querySelector('.tdef[data-name="core|/work/alpha"] .hq-runc');
  // The member that is not up stands by its name, and no row starts it by hand:
  // 足したメンバーは反映（起動する）の 1 つで扱う (d-36ef).
  expect(cell().querySelector('.hq-upm code').textContent).toBe('spec2');
  expect(cell().querySelectorAll('[data-hq=raise]').length).toBe(0);
  // A running team with a change not applied carries 反映していない変更 N.
  const mark = () => cell().querySelector('[data-hq=apply]');
  expect(mark()).not.toBe(null);
  expect(mark().textContent).toContain('反映していない変更');
  expect(mark().querySelector('b').textContent).toBe('2');

  mark().click();
  await settle();

  // The sheet is narrowed to that team, and its members are picked.
  const sheet = () => document.querySelector('.apl-sheet');
  expect(sheet()).not.toBe(null);
  expect(sheet().querySelector('.apl-h b').textContent).toBe('反映: core');
  expect([...sheet().querySelectorAll('[data-apl-name]')].map((el) => el.dataset.aplName)).toEqual(['core-spec2', 'core-lead']);

  // The press is the apply of the picked members.
  sheet().querySelector('[data-apl=send]').click();
  await settle();
  const sent = asked.filter((a) => a.method === 'POST' && a.url === '/api/apply');
  expect(sent.length).toBe(1);
  expect(sent[0].body).toEqual({ members: ['core-spec2', 'core-lead'] });

  // The start's own sheet says how far it got.
  const opened = document.querySelector('.hq-sheet');
  expect(opened).not.toBe(null);
  expect(opened.querySelector('[data-role=title]').textContent).toBe('反映しています');
  expect(opened.querySelector('[data-role=steps]').textContent).toContain('core-spec2 を起動し、初期プロンプトを送りました');

  // A member that asks a question while it starts is answered in its pane, and
  // the same sheet names it (d-d589, r-5d1a).
  document.querySelector('.hq-sheet .hq-x').click();
  await settle();
  asked = serve((method, url) => {
    if (url === '/api/apply' && method === 'POST') {
      return {
        launch: launch({
          phase: 'awaiting',
          waiting: {
            member: 'spec2', herdr: 'core-spec2', workspace: '本体', tab: 'core (t1)',
            pane: 'core-spec2', command: 'hergy apply core-spec2',
          },
        }),
      };
    }
    return band(method, url);
  });
  mark().click();
  await settle();
  document.querySelector('.apl-sheet [data-apl=send]').click();
  await settle();

  const box = document.querySelector('.hq-sheet [data-role=waitbox]');
  expect(box.textContent).toContain('core-spec2');
  expect(box.textContent).toContain('確認に答えを待っています');
  expect(box.querySelector('.hq-where').textContent).toContain('ペイン core-spec2');
});
