import { expect, test } from 'bun:test';
import { copyGroupOf, projectOfGroup } from '../src/lib/copy';

test('API project selectors keep other projects and omit the global layer', () => {
  expect(projectOfGroup(undefined)).toBeUndefined();
  expect(projectOfGroup('')).toBeUndefined();
  expect(projectOfGroup('global')).toBeUndefined();
  expect(projectOfGroup('/work/beta')).toBe('/work/beta');
  expect(projectOfGroup('/work/alpha')).toBe('/work/alpha');
  expect(projectOfGroup(copyGroupOf('global', '/work/beta', '/work/alpha'))).toBeUndefined();
  expect(projectOfGroup(copyGroupOf('project', 'global', '/work/alpha'))).toBe('/work/alpha');
});
