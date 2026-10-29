// r-1db7 L2: the 使っているチーム column names every group, the global layer
// included: 「グローバル: docs」, the way a project's teams are 「alpha: shop」.
import { expect, test } from 'bun:test';
import { layout, serve, settle } from './page.js';

const projects = {
  mode: 'listed',
  current: { path: '/work/alpha', name: 'alpha' },
  projects: [{ path: '/work/alpha', name: 'alpha' }],
};

const role = (name) => ({
  name, effective: 'global',
  places: { global: { path: `~/.config/hergy/roles/${name}.yaml`, hash: 'h1', doc: {
    name, title: name, revision: 1,
    sections: [{ heading: 'すること', level: 0, intro: '', rules: [{ id: 'R-1', text: 'する。' }] }],
  } } },
  summary: {},
});

const team = (name, project, member) => ({
  project, list: { items: [{ name, summary: { members: [{ name: member, role: 'writer' }] } }] },
});

test('a global role names グローバル beside the teams that read it', async () => {
  history.replaceState({}, '', '/roles');
  layout('roles-data', {
    places: { project: '.hergy/roles/', global: '~/.config/hergy/roles/' },
    texts: {},
    hq: {
      projects,
      roles: [{ global: true, list: { items: [role('writer'), role('reader')] } }],
      teams: [
        team('shop-dev', { path: '/work/alpha', name: 'alpha' }, 'lead'),
        { global: true, list: { items: [{ name: 'docs', summary: { members: [{ name: 'lead', role: 'writer' }, { name: 'writer', role: 'reader' }] } }] } },
      ],
    },
    role: { items: [] },
    team: { items: [] },
  });
  serve(() => undefined);
  await import('../src/roles/main');
  await settle();

  const cells = [...document.querySelectorAll('.c-readers span')].map((el) => el.textContent);
  expect(cells).toContain('alpha: shop-dev / グローバル: docs');
  expect(cells).toContain('グローバル: docs');
});
