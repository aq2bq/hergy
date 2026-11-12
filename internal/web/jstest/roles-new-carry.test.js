import { expect, test } from 'bun:test';
import { layout, serve } from './page.js';

test('a carried unsaved role can reconstruct its row after visiting another screen', async () => {
  layout('roles-data', { role: { items: [] }, team: { items: [] } });
  serve(() => ({ problems: [] }));
  const state = await import('../src/roles/state');
  const draft = { name: 'unfinished', title: 'Build', text: 'not saved' };
  state.restore({ rows: [{ place: 'project', group: '', saved: null, name: draft.name, hash: '', draft, isNew: true }] });
  expect(state.roles.value).toHaveLength(1);
  expect(state.roles.value[0].project.draft).toEqual(draft);
  expect(state.dirty()).toBe(true);
  const carry = state.take();
  state.roles.value[0].project.editing.dispose();
  state.roles.value = [];
  state.restore(carry);
  expect(state.roles.value).toHaveLength(1);
  expect(state.roles.value[0].project.draft).toEqual(draft);
});
