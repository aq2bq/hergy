import { expect, test } from 'bun:test';
import { st } from '../src/lib/hq/state';
import { bandCounts, bandActivity } from '../src/lib/band-counts';
const count = (name) => document.querySelector(`.scr[data-screen=${name}] .n`)?.textContent;
function mount() { document.body.innerHTML = '<div class="screens"><span class="scr" data-screen="teams"><span class="n">0</span></span><span class="scr" data-screen="roles"><span class="n">0</span></span><span class="scr" data-screen="profiles"><span class="n">0</span></span><span class="scr" data-screen="activity"></span></div>'; }
test('the band reads the server counts for listed projects and the global layer', () => {
  mount();
  st.projects.value = {mode: 'listed', projects: [{listed: true, definitions: 2, roles: 3, profiles: 4}, {listed: false, definitions: 100}], global: {definitions: 1, roles: 2, profiles: 3}, current: {definitions: 100}};
  bandCounts();
  expect(['teams', 'roles', 'profiles'].map(count)).toEqual(['3', '5', '7']);
  st.projects.value = {mode: 'none', current: {definitions: 5, roles: 6, profiles: 7}, global: {definitions: 1, roles: 1, profiles: 1}};
  bandCounts();
  expect(['teams', 'roles', 'profiles'].map(count)).toEqual(['6', '7', '8']);
});
test('the band derives activity and waiting from the latest HQ reading only', () => {
  mount();
  st.running.value = {herdr: {ok: true}, teams: [{waiting: [{}, {}]}, {waiting: []}]};
  bandActivity();
  expect(count('activity')).toBe('2');
  expect(document.querySelector('.wait').textContent).toBe('待ち 2');
  st.running.value = {herdr: {ok: false}, teams: []};
  bandActivity();
  expect(count('activity')).toBeUndefined();
  expect(document.querySelector('.wait')).toBeNull();
});
