// teams/place.js — 図の点の置き場所（spec-teams B7、d-a575）。
//
// 描く位置は、人が置いた点（placed）と、置いていない点の自動の場所から組み立てる。
// place は新しい Map を返すだけで、渡されたものを書き換えない。描く関数が状態を
// 書かないので、何が画面を描き直すのかが読める（webui-components 1 章）。
// 位置は保存しない: 開き直すと引き直す（d-a575）。
// textW is how wide a word is drawn: a full-width letter is 13, and a Latin one
// is 8.4 in a name's monospace and 7.4 elsewhere.
const textW = (s, mono) => [...String(s)].reduce((w, c) => w + (/[\u3000-\u9fff\uff00-\uffef]/.test(c) ? 13 : mono ? 8.4 : 7.4), 0);

// boxOf is the room one point's words need.
export function boxOf(id, t) {
  if (id === 'human') return { w: 76, h: 36 };
  const w = (t.workers || []).find((x, i) => 'w' + i === id);
  if (w) return { w: Math.max(108, textW(w.purpose || '（目的）') + 26), h: 40 };
  return { w: Math.max(96, textW(id, true) + 30), h: 46 };
}

// autoPos is where the figure lays the points out: the human, the members round
// it, and each member's workers in the nearest room no other point holds. A point
// never stands on another (監査 1-4).
const GAP = 10;
function hits(t, p, id, c) {
  const b = boxOf(id, t);
  return Object.keys(p).some((oid) => {
    if (oid === id) return false;
    const q = p[oid], ob = boxOf(oid, t);
    return Math.abs(c[0] - q[0]) < (b.w + ob.w) / 2 + GAP && Math.abs(c[1] - q[1]) < (b.h + ob.h) / 2 + GAP;
  });
}
// freeNear keeps the wanted place when it is free, and otherwise walks round
// it in rings until it finds room.
function freeNear(t, p, id, want, startR) {
  if (!hits(t, p, id, want)) return want;
  for (let r = startR || 54; r <= 560; r += 26) {
    for (let k = 0; k < 20; k++) {
      const a = (-100 + k * 18) * Math.PI / 180;
      const c = [want[0] + r * Math.cos(a), want[1] + r * Math.sin(a)];
      if (!hits(t, p, id, c)) return c;
    }
  }
  return [want[0], want[1] + 160];
}
function workerWant(t, p, from, wbox) {
  const from0 = p[from] || [0, 0], hbox = boxOf(from, t);
  const side = from0[0] >= 0 ? 1 : -1;
  return [from0[0] + side * (hbox.w / 2 + wbox.w / 2 + 30), from0[1]];
}
function ring(t, narrowPage) {
  const ids = ['human', ...t.members.map((m) => m.name)];
  const n = Math.max(ids.length, 2);
  const R = n <= 3 ? 140 : 112 + n * 14;
  const rx = narrowPage ? (n <= 3 ? 95 : 105) : R, ry = narrowPage ? 70 + n * 34 : R * (n <= 3 ? 0.8 : 0.86);
  const out = {};
  ids.forEach((id, i) => {
    const a = (-90 + i * 360 / n) * Math.PI / 180;
    out[id] = [rx * Math.cos(a), ry * Math.sin(a)];
  });
  return out;
}

// place is every point's place as the figure draws it: the place a person put a
// point down at, and, for a point they did not, the nearest room around the
// ring. The places of points the draft no longer holds are left out.
export function place(t, placed, narrowPage) {
  const put = placed || {};
  const ids = ['human', ...t.members.map((m) => m.name)];
  const workers = (t.workers || []).map((w, i) => [`w${i}`, w]);
  const auto = ring(t, narrowPage);
  const out = {};
  ids.forEach((id) => { out[id] = put[id] ? [put[id][0], put[id][1]] : auto[id]; });
  // A worker stands beside the member it was added under, in the nearest room
  // no other point holds.
  workers.forEach(([id, w]) => {
    if (put[id]) { out[id] = [put[id][0], put[id][1]]; return; }
    const b = boxOf(id, t);
    out[id] = freeNear(t, out, id, workerWant(t, out, w.from, b), 58);
  });
  // A worker that a change of purpose made wider, or one put down on another by
  // a double press, is moved to the nearest room: workers only, so a member a
  // person picked stays where it stands.
  workers.forEach(([id]) => {
    if (hits(t, out, id, out[id])) out[id] = freeNear(t, out, id, out[id], 40);
  });
  return out;
}
