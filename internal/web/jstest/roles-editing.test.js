import { expect, test } from 'bun:test';
import { layout, later, serve } from './page.js';
const doc = (text, name = 'builder') => ({ name, title: 'Build', text });
const check = { problems: [], preview: { lines: [], chars: 1 }, effect: { diff: { added: [], removedBefore: [], removedAtEnd: false } }, readers: [] };

test('role state keeps the editing instance, late drafts and the carried text basis', async () => {
  layout('roles-data', { role: { items: [{ name: 'builder', places: { project: { doc: doc('old'), text: 'old', hash: 'h1' } }, summary: {} }] }, team: { items: [] } });
  const pending = [];
  serve((method) => {
    if (method === 'PUT') { const one = later(); pending.push(one); return one.promise; }
    return check;
  });
  const state = await import('../src/roles/state');
  await state.boot();
  const row = state.roles.value[0];
  const editing = row.project.editing;
  state.changeDraft(row.uid, 'project', doc('sent', 'renamed'));
  const saving = state.doSave(row.uid, 'project');
  state.changeDraft(row.uid, 'project', doc('typed', 'again'));
  pending[0].resolve({ doc: doc('sent', 'renamed'), text: 'sent', hash: 'h2' });
  await saving;
  expect(state.roles.value[0].project.editing).toBe(editing);
  expect(editing.read.value.saved).toEqual(doc('sent', 'renamed'));
  expect(state.roles.value[0].project.draft).toEqual(doc('typed', 'again'));
  expect(state.dirty()).toBe(true);

  editing.yaml.change('raw sent');
  const rawSaving = editing.yaml.save();
  editing.yaml.change('raw typed');
  pending[1].resolve({ doc: doc('raw sent', 'renamed'), text: 'raw sent', hash: 'h3' });
  await rawSaving;
  expect(state.roles.value[0].project.text).toBe('raw sent');
  expect(state.roles.value[0].project.hash).toBe('h3');
  expect(state.docText(state.roles.value[0], 'project')).toBe('raw typed');
  const carried = state.take();
  expect(carried.entries[0].yaml.hash).toBe('h3');
  editing.yaml.discard();
  state.restore(carried);
  expect(editing.yaml.read.value.draft).toBe('raw typed');
  editing.yaml.discard();
  editing.rebase('h4');
  state.restore(carried);
  expect(editing.yaml.read.value.draft).toBe('raw sent');
  editing.dispose();
});
