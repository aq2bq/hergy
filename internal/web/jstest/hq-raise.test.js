// TEAMS-R1, 起きていないメンバーを起こす (r-5d1a, ac-d090): the 動き column of a
// definition whose team is running names the members the definition has that no
// agent is live for, each with 起こす; the press calls POST /api/hq/refresh for
// that member, and the start's own sheet says how far it got — and, when the agent
// asks a question while it starts, the pane to answer in. The answers are the
// test's, so what the column and the sheet make of them is what is read. The
// screen is mounted once in a file (its state stands for its page), so the two
// presses are one test.
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

// launch is the answer POST /api/hq/refresh gives: the start's own shape, with the
// step the raise has reached and the question it may wait on.
const launch = (over) => Object.assign({
  id: 'l-7', action: 'refresh', team: 'core', definition: 'core', place: 'project',
  project: '/work/alpha', project_name: 'alpha',
  workspace: { id: 'w1', label: '本体' }, tab: { id: 't1', label: 'core' },
  members: ['spec2'], phase: 'starting', waiting: null, error: '',
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
  if (url.startsWith('/api/team/check')) return { problems: [], members: [] };
  if (url.startsWith('/api/role')) return { items: [] };
  if (url.startsWith('/api/hq/launches/')) return { launch: launch({ phase: 'done' }) };
  return undefined;
};

test('the 動き column names the member that is not up, and the start sheet says how the raise goes', async () => {
  history.replaceState({}, '', '/teams');
  page();
  let asked = serve((method, url) => {
    if (method === 'POST' && url === '/api/hq/refresh') return { launch: launch() };
    return band(method, url);
  });
  await import('../static/teams/main.js');
  await settle();

  const cell = () => document.querySelector('.tdef[data-name="core|/work/alpha"] .hq-runc');
  const raise = () => cell().querySelector('[data-hq=raise]');
  // The member that is not up stands by its name, with 起こす. The agents that
  // are live are not offered, so there is one button.
  expect(raise()).not.toBe(null);
  expect(raise().dataset.member).toBe('spec2');
  expect(raise().closest('.hq-upm').querySelector('code').textContent).toBe('spec2');
  expect(cell().querySelectorAll('[data-hq=raise]').length).toBe(1);

  raise().click();
  await settle();

  // The press is the raise of that member of that team.
  const sent = asked.filter((a) => a.method === 'POST' && a.url === '/api/hq/refresh');
  expect(sent.length).toBe(1);
  expect(sent[0].body).toEqual({ team: 'core', members: ['spec2'] });

  // The start's own sheet says how far it got, in the raise's words.
  const sheet = () => document.querySelector('.hq-sheet');
  expect(sheet()).not.toBe(null);
  expect(sheet().querySelector('[data-role=title]').textContent).toBe('起こしています');
  const steps = [...sheet().querySelectorAll('[data-role=steps] li')].map((li) => li.textContent);
  expect(steps.join('\n')).toContain('core-spec2 を起動し、初期プロンプトを送りました');
  expect(sheet().querySelector('[data-hq=abort]').textContent).toBe('起こすのをやめる');

  // A member that asks a question while it starts is answered in its pane, and
  // the same sheet names it (d-d589, r-5d1a).
  sheet().querySelector('.hq-x').click();
  await settle();
  asked = serve((method, url) => {
    if (method === 'POST' && url === '/api/hq/refresh') {
      return {
        launch: launch({
          phase: 'awaiting',
          waiting: {
            member: 'spec2', herdr: 'core-spec2', workspace: '本体', tab: 'core (t1)',
            pane: 'core-spec2', command: 'hergy refresh core-spec2',
          },
        }),
      };
    }
    return band(method, url);
  });
  raise().click();
  await settle();

  const box = sheet().querySelector('[data-role=waitbox]');
  expect(box.textContent).toContain('core-spec2');
  expect(box.textContent).toContain('確認に答えを待っています');
  expect(box.querySelector('.hq-where').textContent).toContain('ペイン core-spec2');
});
