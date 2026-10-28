// SHOWCASE-H1, the profiles list: a headquarters with projects listed draws the
// list under each project's head, then the global layer's once (d-73fc,
// ac-8bf5). The groups are the island's, so what the list makes of them is what
// is read.
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

const profile = (name) => ({
  name,
  places: { project: { path: `.hergy/profiles.yaml`, hash: 'h1', doc: { name, kind: 'claude', args: [], env: [] } } },
  summary: { kind: 'claude' },
});

test('the profiles list stands under the three projects and the global layer', async () => {
  history.replaceState({}, '', '/profiles');
  layout('profiles-data', {
    hq: {
      projects,
      profiles: [...projects.projects.map((p) => ({ project: p, list: { items: [profile(p.name)] } })), { global: true, list: { items: [profile('shared')] } }],
      teams: [], roles: [],
    },
    profile: { items: [], files: [], hashes: {} },
    assign: { empty: true, teams: [] },
  });
  serve(() => undefined);
  await import('../src/profiles/main');
  await settle();

  expect(document.querySelectorAll('#rows .hq-gh:not(.global)').length).toBe(3);
  expect(document.querySelectorAll('#rows .hq-gh.global').length).toBe(1);
});
