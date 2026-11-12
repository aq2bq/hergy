// r-56f0 / d-cc89: a qualified address selects one place without changing the
// row identities used by editing, or the old unqualified opening behavior.
import { beforeAll, beforeEach, expect, test } from 'bun:test';
import { layout, serve, settle } from './page.js';
import { island } from './teams-page.js';

const project = '/work/alpha';
let S;
let asked;
beforeAll(async () => {
  const data = island();
  const item = data.team.items[0];
  // The answer carries one place per row (r-ea17): the global place is its own
  // item, not a second side of the same one.
  data.team.items.push({
    ...item,
    effective: 'global',
    places: { global: { ...structuredClone(item.places.project), hash: 'global-hash' } },
  });
  data.hq = { projects: { mode: 'none', current: { path: project, name: 'alpha' }, projects: [] } };
  layout('teams-data', data);
  asked = serve((method, url) => {
    if (url.startsWith('/api/team/check')) return { problems: [] };
    if (url.includes('/context')) return { state: 'missing', errors: [], uses: [], scanned: [], path: '', problem: 'ロールがありません' };
    if (url.startsWith('/api/role')) return { items: [] };
    return undefined;
  });
  S = await import('../src/teams/state');
  await import('../src/teams/main');
  await settle();
});
beforeEach(() => {
  S.state.value = { ...S.state.value, dtab: {} };
  S.allRecs().forEach((r) => { S.setOps(r, []); S.forgetUndo(r); });
});
async function address(open, agree) {
  const p = new URLSearchParams({ open });
  if (agree) p.set('agree', agree);
  history.replaceState({}, '', `/teams?${p}`);
  S.readAddress();
  await settle();
}
const opened = () => [...document.querySelectorAll('.tdef.open')].map((r) => r.dataset.key);

test('qualified addresses open only the named place and retain it through tabs and address reads', async () => {
  for (const [group, place] of [['global', 'global'], [project, 'project']]) {
    const ref = `core|${group}`;
    await address(ref, ref);
    expect(opened()).toEqual([`core.${place}`]);
    expect(S.dtabOf(S.byKey(`core.${place}`))).toBe('agree');
    // The one-project screen writes its own side as `project`; the global row
    // keeps `global` (⑥-2, decision 2).
    const written = group === 'global' ? 'core|global' : 'core|project';
    S.syncURL();
    expect(new URLSearchParams(location.search).get('open')).toBe(written);
    expect(new URLSearchParams(location.search).get('agree')).toBe(written);
    S.readAddress();
    await settle();
    expect(opened()).toEqual([`core.${place}`]);
    S.pressDtab(S.byKey(`core.${place}`), 'shape');
    await settle();
    expect(opened()).toEqual([`core.${place}`]);
    expect(new URLSearchParams(location.search).get('open')).toBe(written);
    expect(new URLSearchParams(location.search).has('agree')).toBe(false);
  }
});

test('unknown projects and absent definitions do not fall back to the other place', async () => {
  for (const ref of ['core|/work/other', 'absent|global', `absent|${project}`]) {
    await address(ref);
    expect(opened()).toEqual([]);
    expect(S.missingOpen.value).toEqual([ref.split('|')[0]]);
  }
  const global = S.byKey('core.global');
  S.recs.value = S.recs.value.filter((r) => r !== global);
  await address('core|global');
  expect(opened()).toEqual([]);
  expect(S.missingOpen.value).toEqual(['core']);
  S.recs.value = [...S.recs.value, global];
});

test('unqualified addresses and normal toggles retain shared opening and close qualified references', async () => {
  await address('core');
  expect(opened()).toEqual(['core.project', 'core.global']);
  await address('core|global');
  S.pressToggle(S.byKey('core.global'));
  await settle();
  expect(opened()).toEqual([]);
  expect(S.state.value.open.size).toBe(0);
  S.pressToggle(S.byKey('core.project'));
  await settle();
  // The row a person pressed opens alone, and the address names its place:
  // the old bare name opened both rows of that name (d-a143).
  expect(opened()).toEqual(['core.project']);
  expect(new URLSearchParams(location.search).get('open')).toBe('core|project');
});

test('two qualified references survive together and editing keeps place and hash identities', async () => {
  await address(`core|global,core|${project}`);
  expect(opened()).toEqual(['core.project', 'core.global']);
  S.syncURL();
  expect(new URLSearchParams(location.search).get('open')).toBe('core|global,core|project');
  for (const place of ['project', 'global']) {
    const r = S.byKey(`core.${place}`);
    expect(r.dom).toBe('core');
    expect(S.promptComparisonInput(r)).toMatchObject({ name: 'core', place, hash: place === 'project' ? 'h1' : 'global-hash' });
  }
  const global = S.byKey('core.global');
  S.setOps(global, [{ kind: 'description', text: 'global draft' }]);
  const carried = S.take();
  expect(carried.resources).toHaveLength(1);
  expect(carried.resources[0].team.target).toMatchObject({ place: 'global', name: 'core' });
  expect(carried.resources[0].team.hash).toBe('global-hash');
  expect(carried.resources[0].actions).toEqual(global.ops.value);
  S.setOps(global, []);
  S.restore(carried);
  expect(global.draft.value.description).toBe('global draft');
  expect(S.byKey('core.project').draft.value.description).not.toBe('global draft');
});

test('copying a URL-opened row closes its reference and keeps a draft open across place changes', async () => {
  await address('core|global');
  await S.pressCopy(S.byKey('core.global'));
  const copy = S.allRecs().find((r) => r.draftRow);
  expect(copy.base).toEqual({ place: 'global', name: 'core', project: undefined });
  expect(copy.place.value).toBe('project');
  expect(S.isOpenRow(S.byKey('core.global'))).toBe(false);
  expect(S.isOpenRow(copy)).toBe(true);
  await S.pressPlace(copy, 'global');
  // The address follows the row to the place it moved to.
  expect(S.isOpenRow(copy)).toBe(true);
  expect(new URLSearchParams(location.search).get('open')).toBe('core|global');
  expect(asked.filter((q) => q.url.startsWith('/api/team/check')).at(-1).body).toMatchObject({ place: 'global', name: 'core', base: { place: 'global', name: 'core' } });
  S.pressDiscard(copy);
});

test('undo chooses the actually opened row rather than a newer edit in its closed namesake', async () => {
  const global = S.byKey('core.global'), local = S.byKey('core.project');
  S.canvasAct(global, [{ kind: 'description', text: 'global edit' }]);
  S.canvasAct(local, [{ kind: 'description', text: 'later project edit' }]);
  await address('core|global');
  expect(S.undoRow()).toBe(global);
  S.canvasUndo(S.undoRow());
  expect(global.ops.value).toEqual([]);
  expect(local.draft.value.description).toBe('later project edit');
});
