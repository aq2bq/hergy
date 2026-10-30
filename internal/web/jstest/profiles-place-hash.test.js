// PROFILES-HASH-BY-GROUP — r-57d8: a write to a listed project carries that
// project's file's hash, never this serve's. The screen holds one profiles.yaml
// per group (d-73fc), and the page's own two hashes are this serve's project's
// alone. Three presses are read here, each with the request it sent: a delete
// after a save in this serve's own group (the answer's hash is not another
// group's), the save of a profile a person adds under a listed project (a new
// panel has no hash of its own, and the file's is that project's), and the save
// of a group whose file no answer read (nothing is sent in place of it).
import { expect, test } from 'bun:test';
import { serve, settle } from './page.js';

const ALPHA = '/work/alpha';
const BETA = '/work/beta';

const place = (name, hash) => ({ doc: { name, kind: 'claude', args: [], env: [] }, text: '', hash, path: '', problems: { err: 0, warn: 0 } });
const item = (name, hash) => ({ name, places: { project: place(name, hash) }, effective: 'project', hidden: null, summary: { kind: 'claude' }, problems: { err: 0, warn: 0 } });
const list = (items, hash) => ({ items, hashes: { project: hash, global: 'g1' }, files: [{ place: 'project', label: 'プロジェクト', path: '', text: '', hash, problems: [] }] });

function layout() {
  document.body.innerHTML = '<div class="wrap"><div class="bar"></div><div id="tabs"></div>'
    + `<main id="view" data-tab="profiles" data-token="k" data-hash-project="a1" data-hash-shared="g1"></main></div>`;
  const data = document.createElement('script');
  data.type = 'application/json';
  data.id = 'profiles-data';
  data.textContent = JSON.stringify({
    profile: list([item('alpha-fast', 'a1')], 'a1'),
    hq: {
      projects: {
        mode: 'listed',
        current: { path: ALPHA, name: 'alpha' },
        projects: [{ path: ALPHA, name: 'alpha' }, { path: BETA, name: 'beta' }],
      },
      profiles: [
        { project: { path: ALPHA, name: 'alpha' }, list: list([item('alpha-fast', 'a1')], 'a1') },
        { project: { path: BETA, name: 'beta' }, list: list([item('beta-fast', 'b1')], 'b1') },
        { global: true, list: { items: [], hashes: { global: 'g1' }, files: [] } },
      ],
    },
    assign: { empty: true, teams: [] },
  });
  document.body.appendChild(data);
}

test('a write carries its own group’s file hash', async () => {
  history.replaceState({}, '', '/profiles');
  layout();
  const asked = serve((method, url) => {
    if (method === 'PUT' && url.startsWith('/api/profile/')) {
      // The save of this serve's own group: the answer read alpha's file again.
      return { items: [item('alpha-fast', 'a2')], hashes: { project: 'a2', global: 'g1' }, files: [] };
    }
    return undefined;
  });
  const { rows, savePanel, deletePanel, addRow } = await import('../src/profiles/state');
  await settle();

  const panel = (group, file) => rows.value.find((row) => (row.group || '') === group).panels.find((p) => p.file === file);

  // The save of this serve's project writes its own file's hash.
  await savePanel(panel(ALPHA, 'project').key);
  await settle();
  const saves = asked.filter((one) => one.method === 'PUT' && one.url.startsWith('/api/profile/'));
  expect(saves.length).toBe(1);
  expect(saves[0].body.hash).toBe('a1');

  // The delete of the listed project's profile is not refused: it carries
  // beta's file hash, which is not the one this serve's answer just read.
  await deletePanel(panel(BETA, 'project').key);
  await settle();
  const deleted = asked.filter((one) => one.method === 'DELETE' && one.url.startsWith('/api/profile/'));
  expect(deleted.length).toBe(1);
  expect(deleted[0].url).toContain('hash=b1');
  expect(deleted[0].url).toContain('project=%2Fwork%2Fbeta');

  // A profile a person adds under the listed project stands on that project's
  // file: the panel has no hash of its own yet.
  const added = addRow('project', BETA);
  await savePanel(added.panels[0].key);
  await settle();
  const betaSaves = asked.filter((one) => one.method === 'PUT' && one.url.startsWith('/api/profile/'));
  expect(betaSaves.length).toBe(2);
  expect(betaSaves[1].body.hash).toBe('b1');

  // A group whose file no answer read sends nothing rather than another
  // project's hash, and the server refuses it with its own reason.
  const unknown = addRow('project', '/work/gamma');
  await savePanel(unknown.panels[0].key);
  await settle();
  const gammaSaves = asked.filter((one) => one.method === 'PUT' && one.url.startsWith('/api/profile/'));
  expect(gammaSaves.length).toBe(3);
  expect(gammaSaves[2].body.hash).toBe('');
});
