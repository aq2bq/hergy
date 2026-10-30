// teams/place.ts — 図の点の置き場所（spec-teams B7、d-a575）。
//
// 描く位置は、人が置いた点（placed）と、置いていない点の自動の場所から組み立てる。
// place は新しい Map を返すだけで、渡されたものを書き換えない。描く関数が状態を
// 書かないので、何が画面を描き直すのかが読める（webui-components 1 章）。
// 位置は保存しない: 開き直すと引き直す（d-a575）。

// Spot is where one point stands, in the figure's own coordinates.
export type Spot = [number, number];

// Figure is the part of a team a figure is drawn from: its members, and the
// workers each of them hires. The draft and a saved document both answer it.
export type Figure = {
  members: { name: string }[];
  workers: { purpose?: string; from: string }[];
};

// Positions is every point's place, by the point's name ('human', a member's
// name, or 'w<i>' for the i-th worker).
export type Positions = Record<string, Spot>;

// Box is the room one point's words need.
export type Box = { w: number; h: number };

// textW is how wide a word is drawn: a full-width letter is 13, and a Latin one
// is 8.4 in a name's monospace and 7.4 elsewhere.
const textW = (s: unknown, mono?: boolean): number => [...String(s)].reduce((w, c) => w + (/[\u3000-\u9fff\uff00-\uffef]/.test(c) ? 13 : mono ? 8.4 : 7.4), 0);

// boxOf is the room one point's words need.
export function boxOf(id: string, t: Figure): Box {
  if (id === 'human') return { w: 76, h: 36 };
  const w = (t.workers || []).find((_worker, i) => 'w' + i === id);
  if (w) return { w: Math.max(108, textW(w.purpose || '（目的）') + 26), h: 40 };
  return { w: Math.max(96, textW(id, true) + 30), h: 46 };
}

// autoPos is where the figure lays the points out: the human, the members round
// it, and each member's workers in the nearest room no other point holds. A point
// never stands on another (監査 1-4).
const GAP = 10;
function hits(t: Figure, p: Positions, id: string, c: Spot): boolean {
  const b = boxOf(id, t);
  return Object.keys(p).some((oid) => {
    if (oid === id) return false;
    const q = p[oid], ob = boxOf(oid, t);
    return Math.abs(c[0] - q[0]) < (b.w + ob.w) / 2 + GAP && Math.abs(c[1] - q[1]) < (b.h + ob.h) / 2 + GAP;
  });
}
// freeNear keeps the wanted place when it is free, and otherwise walks round
// it in rings until it finds room.
function freeNear(t: Figure, p: Positions, id: string, want: Spot, startR?: number): Spot {
  if (!hits(t, p, id, want)) return want;
  for (let r = startR || 54; r <= 560; r += 26) {
    for (let k = 0; k < 20; k++) {
      const a = (-100 + k * 18) * Math.PI / 180;
      const c: Spot = [want[0] + r * Math.cos(a), want[1] + r * Math.sin(a)];
      if (!hits(t, p, id, c)) return c;
    }
  }
  return [want[0], want[1] + 160];
}
function workerWant(t: Figure, p: Positions, from: string, wbox: Box): Spot {
  const from0 = p[from] || [0, 0], hbox = boxOf(from, t);
  const side = from0[0] >= 0 ? 1 : -1;
  return [from0[0] + side * (hbox.w / 2 + wbox.w / 2 + 30), from0[1]];
}
function ring(t: Figure, narrowPage: boolean): Positions {
  const ids = ['human', ...t.members.map((m) => m.name)];
  const n = Math.max(ids.length, 2);
  const R = n <= 3 ? 140 : 112 + n * 14;
  const rx = narrowPage ? (n <= 3 ? 95 : 105) : R, ry = narrowPage ? 70 + n * 34 : R * (n <= 3 ? 0.8 : 0.86);
  const out: Positions = {};
  ids.forEach((id, i) => {
    const a = (-90 + i * 360 / n) * Math.PI / 180;
    out[id] = [rx * Math.cos(a), ry * Math.sin(a)];
  });
  return out;
}

// pack lays the points in rows under the human: the members across, and each
// member's workers stacked under it in the same column. It is the shape a ring
// too tall for the screen is drawn in (d-943f): wide and short, so a team of
// eight fits at a readable size.
function pack(t: Figure): Positions {
  const ids = t.members.map((m) => m.name);
  const n = ids.length;
  if (!n) return { human: [0, 0] };
  const rows = Math.ceil(n / 4);
  const cols = Math.ceil(n / rows);
  const colW = Math.max(96, ...ids.map((id) => boxOf(id, t).w)) + 56;
  const rowY: number[] = [];
  const rowH: number[] = [];
  let y = 84;
  for (let r = 0; r < rows; r++) {
    const row = ids.slice(r * cols, (r + 1) * cols);
    const memberH = Math.max(...row.map((id) => boxOf(id, t).h));
    const workers = Math.max(0, ...row.map((id) => t.workers.filter((w) => w.from === id).length));
    const workerH = workers ? Math.max(...t.workers.map((_w, i) => boxOf('w' + i, t).h)) : 0;
    rowH[r] = memberH + (workers ? 26 + workers * (workerH + 8) : 0) + 26;
    rowY[r] = y + memberH / 2;
    y += rowH[r];
  }
  const out: Positions = { human: [0, 0] };
  ids.forEach((id, i) => {
    const r = Math.floor(i / cols), c = i % cols;
    const inRow = Math.min(cols, n - r * cols);
    out[id] = [(c - (inRow - 1) / 2) * colW, rowY[r]];
  });
  t.workers.forEach((w, i) => {
    const from = out[w.from], id = 'w' + i;
    if (!from) return;
    const b = boxOf(id, t), hb = boxOf(w.from, t);
    const k = t.workers.slice(0, i).filter((x) => x.from === w.from).length;
    out[id] = [from[0], from[1] + hb.h / 2 + 26 + k * (b.h + 8)];
  });
  return out;
}

// place is every point's place as the figure draws it: the places already in the
// record, and, for a point the record does not hold yet, a place in the nearest
// room no other point holds. With no place at all it lays the row out afresh:
// the human, the members round it, and their workers beside them. The places of
// points the draft no longer holds are left out. wide lays the automatic places
// in rows instead, keeping the places a person put down (r-e39c).
export function place(t: Figure, placed?: Positions | null, narrowPage?: boolean, wide?: boolean): Positions {
  const put = placed || {};
  if (wide) {
    const base = pack(t);
    Object.keys(put).forEach((id) => { if (base[id]) base[id] = [put[id][0], put[id][1]]; });
    return base;
  }
  const ids = ['human', ...t.members.map((m) => m.name), ...(t.workers || []).map((_worker, i) => 'w' + i)];
  const out: Positions = {};
  ids.forEach((id) => { if (put[id]) out[id] = [put[id][0], put[id][1]]; });
  if (!Object.keys(out).length) {
    const first = ring(t, !!narrowPage);
    t.workers.forEach((w, i) => {
      const id = 'w' + i, b = boxOf(id, t);
      first[id] = freeNear(t, first, id, workerWant(t, first, w.from, b), 58);
    });
    return first;
  }
  // A point the record does not hold yet stands beside the points it has: the
  // room to the right of them, and below them, as the figure always laid one
  // out. It goes into the answer, and the screen puts it in the record (so a
  // point that stands does not move when another is added).
  const known = Object.values(out);
  let bx = known.length ? Math.min(...known.map((q) => q[0])) : 0;
  let by = known.length ? Math.max(...known.map((q) => q[1])) + 100 : 0;
  ids.filter((id) => !out[id]).forEach((id) => {
    const wi = /^w(\d+)$/.exec(id);
    if (wi) {
      const w = t.workers[Number(wi[1])], b = boxOf(id, t);
      out[id] = freeNear(t, out, id, workerWant(t, out, w.from, b), 58);
    } else {
      const b = boxOf(id, t);
      out[id] = freeNear(t, out, id, [bx, by], 54);
      bx += b.w + 30;
    }
  });
  // A worker that a change of purpose made wider, or one put down on another by
  // a double press, is moved to the nearest room: workers only, so a member a
  // person picked stays where it stands.
  (t.workers || []).forEach((_worker, i) => {
    const id = 'w' + i;
    if (hits(t, out, id, out[id])) out[id] = freeNear(t, out, id, out[id], 40);
  });
  return out;
}
