// spec 3 章: the band's search. The text in the field, the row the arrows stand
// on and whether the list stands are the part's state (signals), and the list
// draws itself from them: nothing calls a drawing by hand any more
// (webui-components 1 章).
import { expect, test } from 'bun:test';
import { serve, settle } from './page.js';

function page() {
  document.body.innerHTML = '<div class="wrap"><div class="bar"><div class="hq-search"><input id="hq-q" /></div></div></div>';
}

const team = (name, mission) => ({
  name, effective: 'project',
  places: { project: { path: `.hergy/teams/${name}/team.yaml`, doc: { mission, members: [], workers: [], turns: [] } } },
});

test('the search draws its list from its own state, and the arrows walk it', async () => {
  history.replaceState({}, '', '/teams');
  page();
  serve((method, url) => {
    if (url === '/api/hq/projects') return { mode: 'listed', current: { path: '/p/a', name: 'alpha' }, projects: [{ path: '/p/a', name: 'alpha', listed: true }] };
    if (url === '/api/hq/running') return { herdr: { ok: true }, teams: [] };
    if (url === '/api/hq/workspaces') return { workspaces: [] };
    if (url === '/api/hq/teams') return { groups: [{ project: { path: '/p/a', name: 'alpha' }, list: { items: [team('core', '注文 API を作り直す'), team('docs', 'core の文書を書く')] } }] };
    if (url === '/api/hq/roles') return { groups: [] };
    if (url === '/api/hq/profiles') return { groups: [] };
    return undefined;
  });
  await import('../static/lib/hq.js');
  const S = await import('../static/lib/hq/sheets.js');
  // The page calls this itself once the document is read; a test reads the
  // module after that moment, so it mounts the part the same way.
  S.mountSearch();
  await settle();

  const input = document.getElementById('hq-q');
  input.value = 'core';
  input.dispatchEvent(new Event('input', { bubbles: true }));
  await settle();

  // The list stands, with the rows the reading holds.
  const rows = () => [...document.querySelectorAll('.hq-sr .hq-ri')];
  expect(document.querySelector('.hq-sr')).not.toBe(null);
  expect(rows().length).toBeGreaterThan(1);
  expect(rows()[0].classList.contains('sel')).toBe(true);
  // The row's address is the row's own: a definition opens on the teams screen.
  expect(rows().map((r) => r.getAttribute('href')).filter(Boolean).length).toBeGreaterThan(0);

  // The arrows move the picked row, and the list follows it without a drawing
  // being called by hand.
  S.moveSel(1);
  await settle();
  expect(S.searchSel()).toBe(1);
  expect(rows()[1].classList.contains('sel')).toBe(true);
  expect(rows()[0].classList.contains('sel')).toBe(false);

  // A text nothing matches says so, in the part's own words.
  input.value = 'zzz-nothing';
  input.dispatchEvent(new Event('input', { bubbles: true }));
  await settle();
  expect(rows().length).toBe(0);
  expect(document.querySelector('.hq-sr').textContent).toContain('に合うものはありません');

  // Esc's way in clears the field and puts the list away.
  S.resetSearch();
  await settle();
  expect(document.querySelector('.hq-sr')).toBe(null);
  expect(input.value).toBe('');
  expect(S.searchBoxOpen()).toBe(false);
});
