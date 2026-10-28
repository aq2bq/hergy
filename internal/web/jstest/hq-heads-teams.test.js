// SHOWCASE-H1, the teams list: a headquarters with projects listed draws the
// list under each project's head, then the global layer's once (d-73fc,
// ac-8bf5). The groups are the island's, so what the list makes of them is what
// is read. The three screens stand apart because each imports its own module.
import { expect, test } from 'bun:test';
import { layout, serve, settle } from './page.js';

const projects = {
  mode: 'listed',
  current: { path: '/work/alpha', name: 'alpha' },
  projects: [
    { path: '/work/alpha', name: 'alpha' },
    { path: '/work/beta', name: 'beta' },
    { path: '/work/gamma', name: 'gamma' },
  ],
};

const team = (name) => ({
  name, effective: 'project',
  places: { project: { path: `.hergy/teams/${name}/team.yaml`, hash: 'h1', doc: {
    mission: `${name} の使命`, members: [], workers: [], turns: [],
  } } },
});

const groups = (kind) => projects.projects.map((p, i) => ({
  project: p, list: { items: [kind(projects.projects[i].name)] },
}));

test('the teams list stands under the three projects and the global layer', async () => {
  history.replaceState({}, '', '/teams');
  layout('teams-data', {
    dirs: { project: '.hergy/teams', global: '~/.config/hergy/teams' },
    profiles: ['coder'],
    hq: {
      projects,
      teams: [...groups(team), { global: true, list: { items: [team('shared')] } }],
      roles: [], profiles: [],
    },
    team: { items: [], files: [] },
  });
  serve((method, url) => {
    if (url.startsWith('/api/team/check')) return { problems: [], members: [] };
    if (url.startsWith('/api/hq/')) return undefined;
    return undefined;
  });
  await import('../static/teams/main.js');
  await settle();

  expect(document.querySelectorAll('.hq-gh:not(.global)').length).toBe(3);
  expect(document.querySelectorAll('.hq-gh.global').length).toBe(1);
});
