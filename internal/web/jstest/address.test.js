// ⑥-2 / d-a143: the address a screen writes for the rows it has open, and the
// address it reads back. A name alone is the old link: it names every row of
// that name, so a screen opens both sides of it.
import { describe, expect, test } from 'bun:test';
import { groupPlace, parseOpen, readOpen, rowsFor, token, writeOpen, rowAddress } from '../src/lib/address';

describe('the address', () => {
  test('a name and its place, written and read back', () => {
    const params = new URLSearchParams();
    writeOpen(params, [rowAddress('core', 'global'), rowAddress('core', '/work/alpha'), rowAddress('core', 'project')]);
    expect(params.get('open')).toBe('core|global,core|/work/alpha,core|project');
    expect(readOpen(params)).toEqual([
      { name: 'core', group: 'global' },
      { name: 'core', group: '/work/alpha' },
      { name: 'core', group: 'project' },
    ]);
  });
  test('no rows open writes no open value', () => {
    const params = new URLSearchParams('open=core|global');
    writeOpen(params, []);
    expect(params.has('open')).toBe(false);
  });
  test('a row with no place is not written', () => {
    expect(() => rowAddress('core', '')).toThrow();
  });
  test('a name alone is the old link: no place, so every row of that name', () => {
    expect(parseOpen('core,other|global')).toEqual([
      { name: 'core', group: '' },
      { name: 'other', group: 'global' },
    ]);
    expect(token('core')).toBe('core');
    expect(token('core', 'global')).toBe('core|global');
  });
  test('a place is this project, the global layer, or another project', () => {
    expect(groupPlace('project', '/work/alpha')).toBe('project');
    expect(groupPlace('/work/alpha', '/work/alpha')).toBe('project');
    expect(groupPlace('global', '/work/alpha')).toBe('global');
    expect(groupPlace('/work/beta', '/work/alpha')).toBe(null);
  });
});

// The same three readings on the three screens: a place opens that row alone, an
// old name with no place opens this project's and the global row, and a name
// neither holds keeps the first row of that name (d-a143 and its note).
describe('which rows an address names', () => {
  const HERE = '/work/alpha';
  const row = (n, g) => ({ n, g });
  const screens = [
    {
      name: 'team definitions and roles (one-project view)',
      rows: [row('core', 'project'), row('core', 'global'), row('beta-only', '/work/beta')],
      of: (one) => ({ name: one.n, group: one.g }),
    },
    {
      name: 'profiles (the project row is the empty group)',
      rows: [row('core', ''), row('core', 'global'), row('beta-only', '/work/beta')],
      of: (one) => ({ name: one.n, group: one.g === '' ? 'project' : one.g }),
    },
  ];
  for (const screen of screens) {
    test(`${screen.name}: a place names its row alone`, () => {
      const named = (ref) => rowsFor(ref, screen.rows, screen.of, HERE).map((one) => screen.of(one).group);
      expect(named({ name: 'core', group: 'project' })).toEqual(['project']);
      expect(named({ name: 'core', group: HERE })).toEqual(['project']);
      expect(named({ name: 'core', group: 'global' })).toEqual(['global']);
      expect(named({ name: 'core', group: '/work/beta' })).toEqual([]);
    });
    test(`${screen.name}: the old name opens both, or the first row elsewhere`, () => {
      const named = (ref) => rowsFor(ref, screen.rows, screen.of, HERE).map((one) => screen.of(one).group);
      expect(named({ name: 'core', group: '' })).toEqual(['project', 'global']);
      expect(named({ name: 'beta-only', group: '' })).toEqual(['/work/beta']);
      expect(named({ name: 'absent', group: '' })).toEqual([]);
    });
  }
});
