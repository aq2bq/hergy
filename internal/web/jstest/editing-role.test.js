import { expect, test } from 'bun:test';
import { serve, later } from './page.js';
import { editRole, restoreRole, editRoleAssignments } from '../src/lib/editing';
import { resourceIdentity } from '../src/lib/resource-identity';
import { Refusal } from '../src/lib/api';
const doc = (text = 'old', name = 'builder') => ({ name, title: 'Build', text });
const identity = () => resourceIdentity('role', { name: 'builder', place: 'project', project: '/work/alpha' });
const open = () => editRole(identity(), { saved: { ...doc(), name: identity().name }, draft: doc(), hash: 'h1', text: 'old' });
const problem = (text) => ({ sev: 'err', at: 'text', field: 'text', text });
const result = (problems) => ({ problems, preview: { lines: [], chars: 1 }, effect: { diff: { added: [], removedBefore: [], removedAtEnd: false } }, readers: [] });

for (const checkFirst of [false, true]) test(`a copied role keeps the save refusal when a valid-name check answers, check first ${checkFirst}`, async () => {
  const pending = [];
  const refusal = 'そのロール名は既にあります。別のロール名を指定してください';
  serve((method) => {
    if (method === 'PUT') return Promise.reject(new Refusal(422, refusal, { at: 'name', text: refusal }));
    const one = later(); pending.push(one); return one.promise;
  });
  const editing = editRole(resourceIdentity('role', { place: 'global' }), { saved: null, draft: doc('copy', 'builder'), hash: '', text: '', base: identity() });
  editing.change(doc('copy', 'lead'));
  const checking = editing.check();
  const answer = async () => { pending.forEach((one) => one.resolve(result([]))); await checking; };
  if (checkFirst) await answer();
  await editing.save();
  if (!checkFirst) await answer();
  expect(editing.read.value.refusal).toBe(refusal);
  expect(editing.read.value.refusalAt).toBe('name');
  // Rechecking the same input cannot establish that a target can be written.
  const sameName = editing.check(); pending.at(-1).resolve(result([])); await sameName;
  expect(editing.read.value.refusal).toBe(refusal);
  // A new name is checked on its own; the preceding name's refusal can clear.
  editing.change(doc('copy', 'another'));
  const corrected = editing.check(); pending.at(-1).resolve(result([])); await corrected;
  expect(editing.read.value.refusal).toBe('');
  editing.dispose();
});

test('a refusal for a preceding name does not replace a checked correction', async () => {
  const saving = later();
  serve((method) => method === 'PUT' ? saving.promise : result([]));
  const editing = editRole(resourceIdentity('role', { place: 'global' }), { saved: null, draft: doc('copy', 'taken'), hash: '', text: '', base: identity() });
  const response = editing.save();
  editing.change(doc('copy', 'available'));
  await editing.check();
  saving.reject(new Refusal(422, 'the preceding name is taken', { at: 'name' }));
  await response;
  expect(editing.read.value.refusal).toBe('');
  expect(editing.read.value.draft.name).toBe('available');
  editing.dispose();
});

test('original problems and draft problems are compared even when their answers arrive backwards', async () => {
  const pending = [];
  serve((method, url, body) => { const one = later(); pending.push({ ...one, body }); return one.promise; });
  const editing = open();
  editing.change(doc('typed'));
  const checking = editing.check();
  expect(pending[0].body.doc.text).toBe('old');
  expect(pending[1].body.doc.text).toBe('typed');
  pending[1].resolve(result([problem('existing'), problem('new')]));
  pending[0].resolve(result([problem('existing')]));
  await checking;
  expect(editing.read.value.baseProblems).toEqual([problem('existing')]);
  expect(editing.grew.value).toEqual([problem('new')]);
  expect(editing.blocked.value).toBe(true);
  editing.dispose();
});

test('an error already in the saved document does not block saving an edited document', async () => {
  serve(() => result([problem('existing')]));
  const editing = open();
  editing.change(doc('typed'));
  await editing.check();
  expect(editing.grew.value).toEqual([]);
  expect(editing.blocked.value).toBe(false);
  editing.dispose();
});

test('replaced and failed role checks keep the last accepted issues and metadata', async () => {
  const pending = [];
  serve(() => { const one = later(); pending.push(one); return one.promise; });
  const editing = open();
  const first = editing.check();
  pending[0].resolve(result([]));
  pending[1].resolve(result([problem('accepted')]));
  await first;
  editing.change(doc('new'));
  const old = editing.check();
  const latest = editing.check();
  pending[3].resolve({ ...result([problem('latest')]), size: 17 });
  await latest;
  pending[2].resolve(result([]));
  await old;
  expect(editing.read.value.problems).toEqual([problem('latest')]);
  expect(editing.validation.value.size).toBe(17);
  const failed = editing.check();
  pending[4].reject(new Error('offline'));
  await failed;
  expect(editing.read.value.problems).toEqual([problem('latest')]);
  editing.dispose();
});

for (const typed of [false, true]) test(`role save retains the sent baseline with later input ${typed}`, async () => {
  const pending = later();
  const asked = serve(() => pending.promise);
  const editing = open();
  editing.change(doc('sent', 'renamed'));
  editing.renameUsers(false);
  const saving = editing.save();
  if (typed) editing.change(doc('typed', 'again'));
  pending.resolve({ doc: doc('sent', 'renamed'), text: 'sent', hash: 'h2' });
  await saving;
  expect(asked[0].body).toEqual({ place: 'project', project: '/work/alpha', original: 'builder', doc: doc('sent', 'renamed'), hash: 'h1', rename_users: false });
  expect(editing.read.value.saved).toEqual(doc('sent', 'renamed'));
  expect(editing.read.value.draft).toEqual(typed ? doc('typed', 'again') : doc('sent', 'renamed'));
  expect(editing.yaml.read.value.saved).toBe('sent');
  expect(editing.yaml.read.value.hash).toBe('h2');
  expect(editing.dirty.value).toBe(typed);
  editing.dispose();
});

test('role YAML preserves late typing and updates the document and hash on its save', async () => {
  const pending = later();
  serve(() => pending.promise);
  const editing = open();
  editing.yaml.change('sent');
  const saving = editing.yaml.save();
  editing.yaml.change('typed');
  editing.change(doc('document typed'));
  pending.resolve({ doc: doc('sent'), text: 'sent', hash: 'h2' });
  await saving;
  expect(editing.yaml.read.value.saved).toBe('sent');
  expect(editing.yaml.read.value.draft).toBe('typed');
  expect(editing.read.value.saved).toEqual(doc('sent'));
  expect(editing.read.value.draft).toEqual(doc('document typed'));
  expect(editing.read.value.hash).toBe('h2');
  editing.dispose();
});

test('a role conflict retains its draft and can read, undo and overwrite the observed file', async () => {
  let changed = true;
  const asked = serve(() => changed ? Promise.reject(new Refusal(409, 'changed', { text: 'changed', file: { exists: true, path: 'builder.md', hash: 'h2', text: 'remote', doc: doc('remote') } })) : { doc: doc('mine'), text: 'mine', hash: 'h3' });
  const editing = open();
  editing.change(doc('mine'));
  await editing.save();
  expect(editing.read.value.draft.text).toBe('mine');
  editing.readConflict();
  expect(editing.read.value.draft.text).toBe('remote');
  expect(editing.read.value.text).toBe('remote');
  editing.undoConflict();
  expect(editing.read.value.draft.text).toBe('mine');
  expect(editing.read.value.hash).toBe('h1');
  await editing.save();
  changed = false;
  await editing.overwrite();
  expect(asked.at(-1).body.hash).toBe('h2');
  editing.dispose();
});

test('role assignment saves only memberRole operations and keeps a pick made before the response', async () => {
  const pending = later();
  const asked = serve(() => pending.promise);
  const editing = editRoleAssignments(resourceIdentity('team', { name: 'sample', place: 'global' }), [{ name: 'lead', saved: 'first', draft: 'second' }], 't1');
  const saving = editing.save();
  editing.change([{ name: 'lead', role: 'third' }]);
  pending.resolve({ hash: 't2' });
  await saving;
  expect(asked[0].body).toEqual({ place: 'global', part: 'team', hash: 't1', doc: { ops: [{ kind: 'memberRole', member: 'lead', role: 'second' }] } });
  expect(editing.read.value.saved).toEqual([{ name: 'lead', role: 'second' }]);
  expect(editing.read.value.draft).toEqual([{ name: 'lead', role: 'third' }]);
  editing.dispose();
});

test('role copying keeps a typed source and concurrent saves keep the existing refusal', async () => {
  const pending = later();
  const asked = serve(() => pending.promise);
  const editing = editRole(resourceIdentity('role', { place: 'global' }), { saved: null, draft: doc('copy', 'copied'), hash: '', text: '', base: identity() });
  const saving = editing.save();
  expect(await editing.save()).toBeUndefined();
  expect(asked.length).toBe(1);
  expect(editing.read.value.refusal).toBe('ロールを保存しています。完了をお待ちください');
  expect(asked[0].body.base).toEqual({ place: 'project', project: '/work/alpha', name: 'builder' });
  pending.resolve({ doc: doc('copy', 'copied'), text: 'copy', hash: 'h1' });
  await saving;
  expect(editing.read.value.refusal).toBe('');
  editing.dispose();
});


test('complete role carries preserve a copied draft and reject another target or a missing saved document', async () => {
  const target = resourceIdentity('role', { place: 'global' });
  const copy = editRole(target, { saved: null, draft: doc('born', 'copy'), hash: '', text: '', base: identity() });
  copy.change(doc('typed', 'copy'));
  const carry = copy.take();
  const restored = restoreRole(target, carry);
  expect(restored.read.value.draft).toEqual(doc('typed', 'copy'));
  expect(restored.read.value.born).toEqual(doc('born', 'copy'));
  const asked = serve(() => ({ doc: doc('typed', 'copy'), hash: 'h2', text: 'typed' }));
  await restored.save();
  expect(asked[0].body.base).toEqual({ name: 'builder', project: '/work/alpha', place: 'project' });
  expect(restoreRole(resourceIdentity('role', { place: 'project', project: '/work/beta' }), carry)).toBeUndefined();
  const saved = open();
  expect(restoreRole(identity(), saved.take())).toBeUndefined();
  const other = editRole(target, { saved: null, draft: doc('other', 'other'), hash: '', text: '' });
  expect(other.matches(carry)).toBe(false);
  expect(other.restore(carry)).toBe(false);
  for (const editing of [copy, restored, saved, other]) editing.dispose();
});

for (const baseFirst of [false, true]) test(`typing before the original check answers keeps its original basis, base first ${baseFirst}`, async () => {
  const pending = [];
  serve(() => { const one = later(); pending.push(one); return one.promise; });
  const editing = open();
  const old = editing.check();
  editing.change(doc('typed'));
  const latest = editing.check();
  if (baseFirst) pending[0].resolve(result([problem('existing')]));
  pending[2].resolve(result([problem('existing'), problem('new')]));
  pending[1].resolve(result([]));
  if (!baseFirst) pending[0].resolve(result([problem('existing')]));
  await Promise.all([old, latest]);
  expect(editing.read.value.baseProblems).toEqual([problem('existing')]);
  expect(editing.grew.value).toEqual([problem('new')]);
  editing.dispose();
});

test('a late document save cannot replace a later accepted text save', async () => {
  const pending = [];
  serve(() => { const one = later(); pending.push(one); return one.promise; });
  const editing = open();
  editing.change(doc('document'));
  const document = editing.save();
  editing.yaml.change('raw');
  const raw = editing.yaml.save();
  pending[1].resolve({ doc: doc('raw'), text: 'raw', hash: 'h3' });
  await raw;
  pending[0].resolve({ doc: doc('document'), text: 'document', hash: 'h2' });
  expect(await document).toBeUndefined();
  expect(editing.read.value.saved).toEqual(doc('raw'));
  expect(editing.read.value.hash).toBe('h3');
  editing.dispose();
});

test('a rename with identical file bytes rechecks the new original name while later input is pending', async () => {
  let save;
  serve((method, url, body) => {
    if (method === 'PUT') { save = later(); return save.promise; }
    return result(body.doc.name === 'builder' ? [problem('old name')] : []);
  });
  const editing = open();
  await editing.check();
  editing.change(doc('old', 'renamed'));
  const saving = editing.save();
  editing.change(doc('typed', 'again'));
  save.resolve({ doc: doc('old', 'renamed'), text: 'old', hash: 'h1' });
  await saving;
  await editing.check();
  expect(editing.read.value.baseProblems).toEqual([]);
  editing.dispose();
});
test('the accepted save owns the displayed file path after a rename', async () => {
 serve(() => ({ doc: doc('renamed', 'next'), hash: 'h2', text: 'renamed', path: 'roles/next.md' }));
 const e = open(); e.change(doc('renamed', 'next')); await e.save();
 expect(e.read.value.path).toBe('roles/next.md'); e.dispose();
});
