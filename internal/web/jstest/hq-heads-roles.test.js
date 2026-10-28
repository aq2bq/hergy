// SHOWCASE-H1, the roles list: a headquarters with projects listed draws the
// list under each project's head, then the global layer's once (d-73fc,
// ac-8bf5). The roles list drew nothing here once — a head called a function its
// module did not import — so this file holds that the three heads and the
// global one are drawn.
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

const role = (name) => ({
  name, effective: 'project',
  places: { project: { path: `.hergy/roles/${name}.yaml`, hash: 'h1', doc: {
    name, title: name, revision: 1,
    sections: [{ heading: 'すること', level: 0, intro: '', rules: [{ id: 'R-1', text: 'する。' }] }],
  } } },
  summary: {},
});

test('the roles list stands under the three projects and the global layer', async () => {
  history.replaceState({}, '', '/roles');
  layout('roles-data', {
    places: { project: '.hergy/roles/', global: '~/.config/hergy/roles/' },
    texts: {},
    hq: {
      projects,
      roles: [...projects.projects.map((p) => ({ project: p, list: { items: [role(p.name)] } })), { global: true, list: { items: [role('shared')] } }],
      teams: [],
    },
    role: { items: [] },
    team: { items: [] },
  });
  serve((method, url) => {
    if (url.startsWith('/api/role/check')) return { problems: [], preview: { lines: [], chars: 0 }, effect: { diff: {}, turns: [] } };
    if (url.startsWith('/api/hq/')) return undefined;
    return undefined;
  });
  await import('../static/roles/main.js');
  await settle();

  expect(document.querySelectorAll('#rows .hq-gh:not(.global)').length).toBe(3);
  expect(document.querySelectorAll('#rows .hq-gh.global').length).toBe(1);
});
