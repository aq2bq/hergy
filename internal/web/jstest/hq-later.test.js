// The headquarters' readings come after the first frame: the list's 動き column
// and the band's numbers are drawn again when they do, with nothing keeping a
// number to know that they must (webui-components 1 章).
//
// The list is read through the running reading alone here (setRunning, the way
// the activity screen hands its own reading over): no other reading changes with
// it, so only a subscription to the headquarters' state can draw the row again.
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
      projects: { mode: 'listed', current: { path: '/work/alpha', name: 'alpha' }, projects: [{ path: '/work/alpha', name: 'alpha' }] },
      teams: [{
        project: { path: '/work/alpha', name: 'alpha', display: '注文 API' },
        list: { items: [{
          name: 'core', effective: 'project',
          places: { project: { path: '.hergy/teams/core/team.yaml', hash: 'h1', doc: { mission: '注文 API を作り直す', members: [], workers: [], turns: [] } } },
        }] },
      }],
      roles: [], profiles: [],
    },
    team: { items: [], files: [] },
  });
  document.body.appendChild(data);
}

const run = (teams) => ({ herdr: { ok: true, text: '' }, teams });
const live = {
  team: 'core', definition: 'core', place: 'project', project: '/work/alpha', project_name: 'alpha',
  workspace: { id: 'w1', label: '本体' }, members: [], workers: [], waiting: [],
};

test('a reading that comes later draws the 動き column again, and the band holds the projects reading\'s number', async () => {
  history.replaceState({}, '', '/teams');
  page();
  serve((method, url) => {
    if (url === '/api/hq/running') return run([]);
    if (url === '/api/hq/projects') {
      return {
        mode: 'listed', current: { path: '/work/alpha', name: 'alpha' },
        projects: [{ path: '/work/alpha', name: 'alpha', listed: true, definitions: 7 }],
        global: { definitions: 0 },
      };
    }
    if (url === '/api/hq/workspaces') return { workspaces: [] };
    if (url === '/api/hq/teams') return { groups: [] };
    if (url === '/api/hq/roles') return { groups: [] };
    if (url === '/api/hq/profiles') return { groups: [] };
    if (url.startsWith('/api/team/check')) return { problems: [], members: [] };
    if (url.startsWith('/api/role')) return { items: [] };
    return undefined;
  });
  await import('../src/teams/main');
  const A = await import('../src/lib/hq');
  await settle();

  // Nothing is running yet: the row offers the start button.
  const cell = () => document.querySelector('.tdef[data-name="core|/work/alpha"] .hq-runc');
  expect(cell()).not.toBe(null);
  expect(cell().querySelector('[data-hq=start]')).not.toBe(null);
  expect(cell().querySelector('.hq-on')).toBe(null);

  // The running reading comes, and the row says so without the screen asking for
  // a draw.
  A.setRunning(run([live]));
  await settle();
  expect(cell().querySelector('.hq-on')).not.toBe(null);
  expect(cell().querySelector('.hq-on').textContent).toContain('動いている');
  expect(cell().querySelector('[data-hq=stop]').textContent).toBe('終了する');

  // The band's number is the projects reading's own: seven definitions, where
  // the screen's list holds the one row.
  expect(document.querySelector('.screens .scr[data-screen=teams] .n').textContent).toBe('7');
});
