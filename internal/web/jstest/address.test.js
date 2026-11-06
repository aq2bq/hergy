// ⑥-2 / d-a143: the address a screen writes for the rows it has open, and the
// address it reads back. A name alone is the old link: it names every row of
// that name, so a screen opens both sides of it.
import { describe, expect, test } from 'bun:test';
import { groupPlace, parseOpen, readOpen, token, writeOpen } from '../src/lib/address';

describe('the address', () => {
  test('a name and its place, written and read back', () => {
    const params = new URLSearchParams();
    writeOpen(params, [{ name: 'core', group: 'global' }, { name: 'core', group: '/work/alpha' }, { name: 'core', group: 'project' }]);
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
    expect(() => writeOpen(new URLSearchParams(), [{ name: 'core', group: '' }])).toThrow();
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
