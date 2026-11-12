import { expect, test } from 'bun:test';
import { layout, later, serve, settle } from './page.js';
const team = { description: 'example', members: [], turns: [], workers: [] };
test('saving new folders in two places updates their drawn address without leaving a blank new row', async () => {
 layout('teams-data', { team: { items: [], files: [] } });
 let pendingSave;
 serve((method, url, body) => {
  if (url === '/api/team/check') return { problems: [], doc: team, draft: { hash: '', path: 'team.yaml', writes: [] } };
  if (method === 'PUT' && pendingSave) return pendingSave.promise;
  if (method === 'PUT') return { hash: 't2', text: '', path: 'team.yaml', items: [{ name: body.name, places: { [body.place]: { doc: team, hash: 't2', path: 'team.yaml' } }, effective: body.place }] };
  if (url.startsWith('/api/role')) return { items: [] };
 });
 await import('../src/teams/main'); const state = await import('../src/teams/state'); await settle();
 for (const place of ['project', 'global']) {
  const r = await state.addDraft('new', place, 'sample', null, place === 'global' ? 'global' : '');
  await state.saveTeam(r); await settle();
  expect(r.draftRow).toBe(false); expect(r.dom).toBe('sample');
  expect(document.querySelectorAll('.tdef[data-name^="~new"]')).toHaveLength(0);
 }
 for (const typed of [false, true]) {
  const name = typed ? 'with-name' : 'without-name';
  const r = await state.addDraft('new', 'project', name, null, '');
  pendingSave = later(); const saving = state.saveTeam(r);
  if (typed) state.pressName(r, 'later-name');
  pendingSave.resolve({ name, hash: 't2', text: '', path: 'team.yaml', items: [] });
  await saving; pendingSave = undefined; await settle();
  const row = document.querySelector(`.tdef[data-name="${name}"]`);
  const rename = row.querySelector('[aria-label="チーム定義名の変更"] input');
  expect(!!rename).toBe(typed); if (typed) expect(rename.value).toBe('later-name');
 }
});
