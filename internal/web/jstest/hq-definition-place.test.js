import { expect, test } from 'bun:test';
import { st, definitionOf } from '../src/lib/hq/state';

const row = (name, place, description) => ({
  name, effective: place, places: { [place]: { doc: { description } } },
});

test('a launch place selects its exact document without changing legacy resolution', () => {
  st.groups.value = { roles: [], profiles: [], teams: [
    { project: { path: '/work/alpha' }, list: { items: [row('shared', 'project', 'local'), row('local-only', 'project', 'local only')] } },
    { project: { path: '/work/beta' }, list: { items: [row('other', 'project', 'another project')] } },
    { global: true, list: { items: [row('shared', 'global', 'global'), row('global-only', 'global', 'global only')] } },
  ] };
  expect(definitionOf('/work/alpha', 'shared', 'global').description).toBe('global');
  expect(definitionOf('/work/alpha', 'shared', 'project').description).toBe('local');
  expect(definitionOf('/work/alpha', 'global-only', 'project')).toBe(null);
  expect(definitionOf('/work/alpha', 'local-only', 'global')).toBe(null);
  expect(definitionOf('/work/alpha', 'other', 'project')).toBe(null);
  expect(definitionOf('/work/alpha', 'shared').description).toBe('local');
  expect(definitionOf('/work/alpha', 'global-only').description).toBe('global only');
});
