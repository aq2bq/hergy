import { expect, test } from 'bun:test';
import { editRole, restoreRole } from '../src/lib/editing';
import { resourceIdentity } from '../src/lib/resource-identity';

test('a new page cannot mistake its new row for the carried row of the preceding page', () => {
  // The preceding page left both this snapshot and the last allocated number.
  window.sessionStorage.setItem('hergy.editing.draft-sequence', '1');
  const carry = { hash: '', saved: null, born: { name: 'carried', title: '', text: 'born' }, draft: { name: 'carried', title: '', text: 'typed' },
    target: { kind: 'role', place: 'project', project: '/work/alpha', name: null, draftId: 'draft-1' } };
  const target = resourceIdentity('role', { place: 'project', project: '/work/alpha' });
  const current = editRole(target, { saved: null, draft: { name: 'current', title: '', text: 'another input' }, hash: '', text: '' });
  expect(current.matches(carry)).toBe(false);
  expect(current.restore(carry)).toBe(false);
  const restored = restoreRole(target, carry);
  expect(restored.read.value.draft.text).toBe('typed');
  expect(current.read.value.draft.text).toBe('another input');
  current.dispose(); restored.dispose();
});
