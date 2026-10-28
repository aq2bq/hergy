// spec-teams B7, webui-components 1 章: where the figure draws each point.
//
// place reads the draft and the places a person put points down at, and answers
// a new map with a place for every point of the draft. It writes nothing, so a
// reading is not also a change, and the same reading draws the same figure. The
// screen's own drawing then reads that map (teams/place.js, teams/state.js's
// layout) — nothing is worked out while the figure is drawn.
import { expect, test } from 'bun:test';
import { boxOf, place } from '../static/teams/place.js';
import { layout, serve, settle } from './page.js';
import { click, island } from './teams-page.js';

const draft = (over) => ({
  mission: '注文 API を作り直す',
  members: [{ name: 'lead', role: 'lead', profile: 'coder' }, { name: 'builder', role: 'builder', profile: 'coder' }],
  workers: [{ from: 'lead', purpose: '実装', profile: 'coder' }],
  turns: [['human', 'lead']],
  ...(over || {}),
});

const overlap = (t, p) => {
  const ids = Object.keys(p);
  for (let i = 0; i < ids.length; i++) {
    for (let j = i + 1; j < ids.length; j++) {
      const a = p[ids[i]], b = p[ids[j]], ba = boxOf(ids[i], t), bb = boxOf(ids[j], t);
      if (Math.abs(a[0] - b[0]) < (ba.w + bb.w) / 2 && Math.abs(a[1] - b[1]) < (ba.h + bb.h) / 2) return `${ids[i]} and ${ids[j]}`;
    }
  }
  return '';
};

test('every point of the draft has the place the figure draws, and a point added has one at once', () => {
  const t = draft();
  const one = place(t, { human: [-200, -120] }, false);
  expect(Object.keys(one).sort()).toEqual(['builder', 'human', 'lead', 'w0']);
  // The place a person put a point down at is the place drawn.
  expect(one.human).toEqual([-200, -120]);
  // Every other point has a place of its own, and no two points stand on each
  // other (監査 1-4).
  ['lead', 'builder', 'w0'].forEach((id) => {
    expect(Array.isArray(one[id])).toBe(true);
    expect(one[id].every((n) => Number.isFinite(n))).toBe(true);
  });
  expect(overlap(t, one)).toBe('');

  // A point the draft gains has a place the same reading, without any drawing
  // having filled one in.
  const t2 = draft({ members: [...t.members, { name: 'reviewer', role: 'builder', profile: 'coder' }] });
  const two = place(t2, { human: [-200, -120] }, false);
  expect(two.reviewer).toBeDefined();
  expect(two.reviewer.every((n) => Number.isFinite(n))).toBe(true);
  expect(overlap(t2, two)).toBe('');
});

test('a reading writes nothing, and a point the draft no longer has is left out', () => {
  const t = draft();
  const placed = { lead: [10, 20] };
  const before = JSON.stringify(placed);
  const one = place(t, placed, false);
  // The places a person put down are not written into by the reading.
  expect(JSON.stringify(placed)).toBe(before);
  expect(one.lead).toEqual([10, 20]);
  // The same reading draws the same figure.
  expect(place(t, placed, false)).toEqual(one);

  const gone = place(draft({ members: [{ name: 'lead', role: 'lead', profile: 'coder' }], workers: [] }), { builder: [0, 0] }, false);
  expect(Object.keys(gone).sort()).toEqual(['human', 'lead']);
});

test('a point added on the canvas is drawn where the record holds it', async () => {
  history.replaceState({}, '', '/teams?open=core');
  layout('teams-data', island());
  serve((method, url) => {
    if (url.startsWith('/api/team/check')) return { problems: [] };
    if (url.startsWith('/api/role')) return { items: [] };
    return undefined;
  });
  const S = await import('../static/teams/state.js');
  await import('../static/teams/main.js');
  await settle();

  click(document.querySelector('.tdef[data-name="core"] [data-cv-act=m-add]'));
  await settle();
  const r = S.byName('core');
  const name = Object.keys(r.layout.value).find((id) => id.startsWith('member-'));
  expect(name).toBeDefined();
  // The record holds the place, and the figure draws the point there.
  const at = r.layout.value[name];
  expect(at.every((n) => Number.isFinite(n))).toBe(true);
  const drawn = document.querySelector(`.tdef[data-name="core"] .cv-n.member[data-cv-n="${name}"]`);
  expect(drawn).not.toBe(null);
  expect(drawn.getAttribute('transform')).toBe(`translate(${at[0].toFixed(1)},${at[1].toFixed(1)})`);
});
