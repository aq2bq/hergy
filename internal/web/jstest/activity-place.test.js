import { expect, test } from 'bun:test';
import { serve, settle } from './page.js';

const project = '/work/alpha';
const row = (name, place, members, to) => ({ name, effective: place, places: { [place]: { doc: {
  members: members.map((member) => ({ name: member })),
  turns: [{ from: 'human', to }],
} } } });

test('activity links and figures use the launch place even when another place holds the name', async () => {
  document.body.innerHTML = '<div class="bar"></div><div id="tabs"></div><div id="list"></div><script id="activity-words" type="application/json">{}</script>';
  history.replaceState({}, '', '/activity');
  const items = [
    { name: 'global-run', place: 'global', summary: { definition: 'shared', state: 'working' } },
    { name: 'local-run', place: 'project', summary: { definition: 'shared', state: 'working' } },
    { name: 'missing-run', place: 'global', summary: { definition: 'removed', state: 'working' } },
  ].map((item) => ({ ...item, project, project_name: 'alpha', error: '' }));
  serve((method, url) => {
    if (url === '/api/activity') return { items, herdr: { ok: true }, running: 3, waiting: 0 };
    if (url.startsWith('/api/activity/')) return { doc: { nodes: [{ name: 'lead', state: 'active' }, { name: 'spec', state: 'active' }], hired: [] } };
    if (url === '/api/hq/teams') return { groups: [
      { project: { path: project }, list: { items: [row('shared', 'project', ['lead', 'spec'], 'lead'), row('removed', 'project', ['lead'], 'lead')] } },
      { global: true, list: { items: [row('shared', 'global', ['spec', 'lead'], 'spec')] } },
    ] };
    return undefined;
  });
  await import('../src/activity/main');
  const { teams } = await import('../src/activity/state');
  await settle();
  const link = (name) => new URL(document.querySelector(`.hq-tile[data-team="${name}"] .hq-tactions a`).href).searchParams.get('open');
  expect(link('global-run')).toBe('shared|global');
  expect(link('local-run')).toBe('shared|/work/alpha');
  expect(link('missing-run')).toBe('removed|global');
  const global = teams.value.find((team) => team.name === 'global-run');
  const local = teams.value.find((team) => team.name === 'local-run');
  expect(global.turns).toEqual([['human', 'spec']]);
  expect(global.crew.map((member) => member.name)).toEqual(['spec', 'lead']);
  expect(local.turns).toEqual([['human', 'lead']]);
  expect(local.crew.map((member) => member.name)).toEqual(['lead', 'spec']);
  expect(teams.value.find((team) => team.name === 'missing-run').turns).toEqual([]);
});
