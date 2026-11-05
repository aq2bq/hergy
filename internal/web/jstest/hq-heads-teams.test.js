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

const team = (name, place = 'project') => ({
  name, effective: place,
  places: { [place]: { path: `.hergy/teams/${name}/team.yaml`, hash: 'h1', doc: {
    description: `${name} の説明`, members: [], workers: [], turns: [],
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
      teams: [...groups(team).map((g, i) => i === 0 ? { ...g, list: { items: [...g.list.items, team('shared')] } } : g),
        { global: true, list: { items: [team('shared', 'global')] } }],
      roles: [], profiles: [],
    },
    team: { items: [], files: [] },
  });
  serve((method, url) => {
    if (url.startsWith('/api/team/check')) return { problems: [], members: [] };
    if (url.startsWith('/api/hq/')) return undefined;
    return undefined;
  });
  await import('../src/teams/main');
  const S = await import('../src/teams/state');
  await settle();

  expect(document.querySelectorAll('.hq-gh:not(.global)').length).toBe(3);
  expect(document.querySelectorAll('.hq-gh.global').length).toBe(1);
  for (const [group, place] of [['global', 'global'], ['/work/alpha', 'project']]) {
    const ref = `shared|${group}`;
    history.replaceState({}, '', `/teams?${new URLSearchParams({ open: ref, agree: ref })}`);
    S.readAddress();
    await settle();
    expect([...document.querySelectorAll('.tdef.open')].map((r) => r.dataset.key)).toEqual([`${ref}.${place}`]);
    S.syncURL();
    expect(new URLSearchParams(location.search).get('open')).toBe(ref);
    expect(new URLSearchParams(location.search).get('agree')).toBe(ref);
  }
  // The listed view's legacy unqualified agree key is left as it was.
  S.state.value = { ...S.state.value, dtab: {} };
  history.replaceState({}, '', '/teams?open=shared&agree=shared');
  S.readAddress();
  expect(S.dtabOf(S.byName('shared|/work/alpha'))).toBe('shape');
});
