// r-b30d/ac-fa61: real list rows contain one place each. Compare the server's
// mark with the agreement column, including unreadable peers.
import { expect, test } from 'bun:test';
import { layout, serve, settle } from './page.js';
const doc = {description: '説明', members: [], workers: [], turns: [], agreement: {doc: {title: '規則', text: '本文'}, hash: 'agree'}};
const item = (name, place, mark, broken = false) => ({name, effective: place, mark, places: {[place]: {doc: {...doc, broken}, hash: name + place, path: `/${place}/${name}/team.yaml`, ...(broken ? {problem: '読めません', state: 'unreadable'} : {})}}, summary: {}, problems: {err: 0, warn: 0}});
const items = [item('both', 'project', 'in_effect'), item('both', 'global', 'hidden'), item('local-only', 'project', ''), item('global-only', 'global', ''), item('unreadable-peer', 'project', 'in_effect', true), item('unreadable-peer', 'global', 'hidden')];
test('team summary and agreement follow the server mark for both, single and unreadable-peer rows', async () => {
  history.replaceState({}, '', '/teams?open=both,local-only,global-only,unreadable-peer');
  layout('teams-data', {team: {items, files: []}, dirs: {}, profiles: []});
  serve((method, url) => url.startsWith('/api/team/check') ? {problems: []} : url.startsWith('/api/role') ? {items: []} : undefined);
  const state = await import('../src/teams/state');
  await import('../src/teams/main');
  await settle();
  const compared = [];
  for (const r of state.allRecs()) {
    if (r.problem.value) continue;
    state.pressDtab(r, 'agree');
    await settle();
    const panel = document.querySelector(`.tdef[data-key="${state.dkey(r)}"] .agree-p`);
    expect(panel, r.name.value + ' ' + r.place.value).not.toBeNull();
    compared.push({name: r.name.value, place: r.place.value, expected: r.mark.value === 'hidden', actual: panel.classList.contains('shadowed')});
  }
  expect(compared).toHaveLength(5);
  expect(compared.map(({name, place, expected, actual}) => [name, place, actual === expected])).toEqual(compared.map(({name, place}) => [name, place, true]));
});
