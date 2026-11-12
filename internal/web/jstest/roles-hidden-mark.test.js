import { expect, test } from 'bun:test';
import { layout, serve, settle } from './page.js';
const item = (name, place, mark, broken = false) => ({name, effective: place, mark, places: {[place]: {doc: {name, title: name, text: '本文'}, text: '本文', hash: name + place, path: `/${place}/${name}.md`, ...(broken ? {problem: '読めません', state: 'unreadable'} : {})}}, summary: {}});
const items = [item('both', 'project', 'in_effect'), item('both', 'global', 'hidden'), item('local-only', 'project', ''), item('global-only', 'global', ''), item('unreadable-peer', 'project', 'in_effect', true), item('unreadable-peer', 'global', 'hidden')];
test('role Markdown files follow the server mark for both, single and unreadable-peer rows', async () => {
  history.replaceState({}, '', '/roles/yaml');
  layout('roles-data', {role: {items}, team: {items: []}, places: {}, texts: {}});
  serve((method, url) => url.startsWith('/api/role/check') ? {problems: [], preview: {lines: []}} : undefined);
  const state = await import('../src/roles/state');
  await import('../src/roles/main');
  await settle();
  const compared = [];
  for (const {r, l, e} of state.filesList()) {
    const button = document.querySelector(`.files [data-key="${r.uid}${l}"]`);
    expect(button).not.toBeNull();
    compared.push([e.saved.name, l, button.classList.contains('hid') === (r.mark === 'hidden'), button.textContent.includes('・無効') === (r.mark === 'hidden')]);
  }
  expect(compared).toHaveLength(5);
  expect(compared).toEqual(state.filesList().map(({l, e}) => [e.saved.name, l, true, true]));
});
