import { expect, test } from 'bun:test';
import { layout, later, serve } from './page.js';

const doc = (name, arg) => ({ name, kind: 'claude', args: [arg], env: [] });
const saved = (name, arg, hash) => ({ hash, hashes: { project: hash }, items: [{
  name, summary: { kind: 'claude' }, problems: { err: 0, warn: 0 },
  places: { project: { doc: doc(name, arg), hash, path: 'profiles.yaml' } },
}], files: [] });

test('the profile list retains input made before the save response, including a second rename', async () => {
  layout('profiles-data', {});
  const pending = later();
  serve((method) => method === 'PUT' ? pending.promise : {});
  const { addRow, changeDraft, savePanel, rows } = await import('../src/profiles/state');
  const panel = addRow('project', '').panels[0];
  changeDraft(panel.key, doc('sent', 'first'));
  const saving = savePanel(panel.key);
  changeDraft(panel.key, doc('typed', 'second'));
  pending.resolve(saved('sent', 'first', 'h2'));
  await saving;
  const current = rows.value.find((r) => r.name === 'sent').panels[0];
  expect(current.saved).toEqual(doc('sent', 'first'));
  expect(current.draft).toEqual(doc('typed', 'second'));
  expect(current.hash).toBe('h2');
  expect(current.editing.dirty.value).toBe(true);
});
