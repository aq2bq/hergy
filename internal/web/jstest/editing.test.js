import { expect, test } from 'bun:test';
import { later, serve } from './page.js';
import { editProfile, editProfileAssignments, editYaml, profileFiles } from '../src/lib/editing';
import { resourceIdentity } from '../src/lib/resource-identity';

const draft = (name = 'fast') => ({ name, kind: 'claude', args: [], env: [] });
const id = (project = '/work/alpha', name = 'fast', place = 'project') => resourceIdentity('profile', { place, project, name });
const open = () => editProfile(id(), { saved: draft(), draft: draft(), hash: 'h1' });
const problem = { sev: 'err', at: 'name', text: 'taken' };

test('checks retain the accepted result across reordered responses, failure and draft replacement', async () => {
  const pending = [];
  serve(() => { const one = later(); pending.push(one); return one.promise; });
  const editing = open();
  const old = editing.check();
  const current = editing.check();
  pending[1].resolve({ problems: [problem] });
  await current;
  pending[0].resolve({ problems: [] });
  await old;
  expect(editing.read.value.problems).toEqual([problem]);
  const failed = editing.check();
  pending[2].reject(new Error('offline'));
  await failed;
  expect(editing.read.value.problems).toEqual([problem]);
  const obsolete = editing.check();
  editing.change(draft('new'));
  pending[3].resolve({ problems: [] });
  await obsolete;
  expect(editing.read.value.problems).toEqual([problem]);
  const clean = editing.check();
  pending[4].resolve({ problems: [] });
  await clean;
  expect(editing.read.value.problems).toEqual([]);
  editing.dispose();
});

test('save writes a captured draft and retains typing made while its response is pending', async () => {
  const pending = later();
  const asked = serve(() => pending.promise);
  const editing = open();
  editing.change(draft('first'));
  const saving = editing.save();
  editing.change(draft('second'));
  pending.resolve({ hash: 'h2' });
  await saving;
  expect(asked[0].body).toEqual({ place: 'project', project: '/work/alpha', original: 'fast', doc: draft('first'), hash: 'h1', rename_users: true });
  expect(editing.read.value.saved).toEqual(draft('first'));
  expect(editing.read.value.draft).toEqual(draft('second'));
  expect(editing.read.value.hash).toBe('h2');
  expect(editing.dirty.value).toBe(true);
  editing.dispose();
});

test('a disposed editor drops a late save and a late check', async () => {
  const pending = [];
  serve(() => { const one = later(); pending.push(one); return one.promise; });
  const editing = open();
  const check = editing.check();
  const save = editing.save();
  editing.dispose();
  pending[0].resolve({ problems: [problem] });
  pending[1].resolve({ hash: 'h2' });
  await Promise.all([check, save]);
  expect(editing.read.value.hash).toBe('h1');
  expect(editing.read.value.problems).toEqual([]);
});

test('file bases and carried drafts cannot cross projects', () => {
  const files = profileFiles();
  files.learn(id('/work/alpha'), 'a1');
  files.learn(id('/work/beta'), 'b1');
  files.learn(id('/work/alpha', 'fast', 'global'), 'g1');
  expect(files.hash(id('/work/alpha'))).toBe('a1');
  expect(files.hash(id('/work/beta'))).toBe('b1');
  expect(files.hash(id('/work/gamma'))).toBe('');
  expect(files.hash(id('/work/beta', 'fast', 'global'))).toBe('g1');
  const editing = open();
  expect(editing.restore({ hash: 'different', draft: draft('other') })).toBe(false);
  expect(editing.restore({ hash: 'h1', draft: draft('typed') })).toBe(true);
  const carry = editing.take();
  carry.draft.name = 'outside';
  expect(editing.read.value.draft.name).toBe('typed');
  const read = editing.read.value;
  read.draft.name = 'outside';
  // The read projection is a snapshot and cannot mutate the private state.
  expect(editing.take().draft.name).toBe('typed');
  editing.dispose();
});

test('profile assignment operations are derived from the saved picks', async () => {
  const asked = serve(() => ({ hash: 't2' }));
  const editing = editProfileAssignments(resourceIdentity('team', { name: 'sample', place: 'global', project: '/work/alpha' }), [
    { key: 'm0', name: 'lead', role: 'lead', profile: 'first' },
    { key: 'w0', name: 'build', role: 'lead', profile: 'first' },
  ], 't1');
  editing.change((rows) => rows.map((row) => ({ ...row, profile: 'second' })));
  await editing.save();
  expect(asked[0].body).toEqual({ place: 'global', doc: { ops: [
    { kind: 'memberProfile', member: 'lead', profile: 'second' },
    { kind: 'workerProfile', from: 'lead', purpose: 'build', profile: 'second' },
  ] }, hash: 't1' });
  expect(editing.dirty.value).toBe(false);
  expect(editing.read.value.hash).toBe('t2');
  editing.dispose();
});

test('YAML checks preserve the last accepted problems and restore only the matching basis', async () => {
  const pending = [];
  serve(() => { const one = later(); pending.push(one); return one.promise; });
  const editing = editYaml('profile', { place: 'project', name: '_yaml', text: 'version: 1', hash: 'y1', problems: [problem] });
  const old = editing.check();
  editing.change('version: 2');
  const current = editing.check();
  pending[1].reject(new Error('offline'));
  await current;
  pending[0].resolve({ problems: [] });
  await old;
  expect(editing.read.value.problems).toEqual([problem]);
  const carry = editing.take();
  editing.discard();
  expect(editing.dirty.value).toBe(false);
  expect(editing.restore(carry)).toBe(true);
  expect(editing.read.value.draft).toBe('version: 2');
  editing.rebase('y2');
  expect(editing.restore(carry)).toBe(false);
  editing.dispose();
});
