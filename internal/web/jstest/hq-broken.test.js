// TEAMS-R2, a definition that cannot be built says why (r-fc16): the 動き column
// of a running team whose definition hergy cannot build names no member to offer
// 起動 for. The headquarters' answer carries the reason in hergy status's words,
// with the way to fix it, and the column shows it rather than going silent.
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
              { name: 'spec2', role: '進行管理', profile: 'coder' },
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

// reason is the sentence the reading carries: hergy status's words for a
// definition it cannot build, which also say how to fix the name.
const reason = '.hergy/teams/core/team.yaml: ロール名「進行管理」は使えません。ロール名は小文字で始めます。人に読ませる名前は title に書きます';

// running is the headquarters' reading: core is up, only its lead is live, and
// the definition cannot be built, so no member is named as not up.
const running = () => ({
  herdr: { ok: true, text: '' },
  teams: [{
    team: 'core', definition: 'core', place: 'project', project: '/work/alpha', project_name: 'alpha',
    workspace: { id: 'w1', label: '本体' }, tab: { id: 't1', label: 'core' },
    members: [{ name: 'lead', herdr: 'core-lead', live: true, state: 'idle' }],
    not_up: [], workers: [], waiting: [], starting: false, error: reason,
  }],
});

test('the 動き column says why the definition cannot be built instead of going silent', async () => {
  history.replaceState({}, '', '/teams');
  page();
  serve((method, url) => {
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
    return undefined;
  });
  await import('../static/teams/main.js');
  await settle();

  const cell = () => document.querySelector('.tdef[data-name="core|/work/alpha"] .hq-runc');
  // The reason stands in the column, in the reading's own words.
  const broken = cell().querySelector('[data-role=running-error]');
  expect(broken).not.toBe(null);
  expect(broken.textContent).toBe(reason);
  // No member is named as one to start, because the definition names none it can
  // start: the sentence is what a person reads instead.
  expect(cell().querySelectorAll('[data-hq=raise]').length).toBe(0);
  expect(cell().querySelector('[data-role=not-up]')).toBe(null);
});
