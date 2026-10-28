// lib/hq/figure.tsx — チームの図（spec 6 章）。アクティビティの札と
// 図を描くための部品（webui-preact-design 5 章 1。前は hq.js が SVG の文字列を返し、
// アクティビティの画面が dangerouslySetInnerHTML で流し込んでいた）。
//
// 人間が上、メンバーが 1 列、ワーカーはそれを足したメンバーの下。ターンは線で、
// 人間を待っているメンバーは人間への線で描く。図は「操作しない図」: 押せるものも
// フォーカスの入るものも無い（role="img"）。
import type { JSX } from 'preact';

type FigureMember = { name: string; state: string; doing?: string };
type FigureWorker = { name: string; state: string; from: string; doing?: string };
type FigureTeam = { name: string; crew?: FigureMember[]; workers?: FigureWorker[] };
type Point = [number, number];
type Props = { team: FigureTeam; turns?: [string, string][]; W?: number; H?: number; layout?: 'default' | 'packed' };

export function TeamFigure({ team, turns, W = 440, H = 236, layout = 'default' }: Props): JSX.Element {
  if (layout === 'packed') return <PackedFigure team={team} turns={turns} W={W} />;
  const t = team || {};
  const crew = t.crew || [];
  const pos: Record<string, Point> = { human: [W / 2, 30] };
  const n = crew.length;
  crew.forEach((m, i) => { pos[m.name] = [n === 1 ? W / 2 : 46 + (i * (W - 92)) / (n - 1), 124]; });
  const seen = new Set<string>();
  const edges: JSX.Element[] = [];
  for (const [a, b] of turns || []) {
    if (!pos[a] || !pos[b]) continue;
    const k = [a, b].sort().join('|');
    if (seen.has(k)) continue;
    seen.add(k);
    const [x1, y1] = pos[a], [x2, y2] = pos[b];
    if (y1 === y2) {
      const mx = (x1 + x2) / 2, my = y1 + 58 + Math.abs(x2 - x1) * 0.1;
      edges.push(<path class="hq-e" d={`M${x1} ${y1} Q${mx} ${my} ${x2} ${y2}`} />);
    } else {
      edges.push(<line class="hq-e" x1={x1} y1={y1} x2={x2} y2={y2} />);
    }
  }
  const calls: JSX.Element[] = [];
  for (const m of crew) {
    if (m.state !== 'blocked' && m.state !== 'handed') continue;
    const [x, y] = pos[m.name];
    calls.push(<line class="hq-call" x1={x} y1={y - 14} x2={W / 2} y2="44" />);
  }
  const nodes: JSX.Element[] = [];
  nodes.push(<g class="hq-n human"><rect x={W / 2 - 26} y="16" width="52" height="28" rx="6" /><text x={W / 2} y="35">人間</text></g>);
  for (const m of crew) {
    const [x, y] = pos[m.name];
    nodes.push(<g class={'hq-n ' + m.state}><circle class="ring" cx={x} cy={y} r="15" /><circle class="dot" cx={x} cy={y} r="11" />{m.state === 'blocked' ? <text class="bang" x={x} y={y + 4}>!</text> : ''}<text class="lbl" x={x} y={y + 30}>{m.name}</text></g>);
  }
  const byMember: Record<string, FigureWorker[]> = {};
  const free: FigureWorker[] = [];
  for (const w of t.workers || []) {
    if (!pos[w.from]) { free.push(w); continue; }
    (byMember[w.from] || (byMember[w.from] = [])).push(w);
  }
  for (const [from, ws] of Object.entries(byMember)) {
    const [x] = pos[from];
    ws.forEach((w, i) => {
      const wx = x + (i - (ws.length - 1) / 2) * 34, wy = 204;
      nodes.push(<><line class="hq-e wk" x1={x} y1="139" x2={wx} y2={wy - 7} /><g class={'hq-n wkr ' + w.state}><circle class="dot" cx={wx} cy={wy} r="6" /><text class="lbl s" x={wx} y={wy + 20}>{w.name}</text></g></>);
    });
  }
  // A worker whose member the reading does not name stands on the row below,
  // spread out and unconnected: the figure does not draw a line it cannot stand
  // behind.
  free.forEach((w, i) => {
    const wx = free.length === 1 ? W / 2 : 30 + (i * (W - 60)) / (free.length - 1), wy = 204;
    nodes.push(<g class={'hq-n wkr ' + w.state}><circle class="dot" cx={wx} cy={wy} r="6" /><text class="lbl s" x={wx} y={wy + 20}>{w.name}</text></g>);
  });
  return <svg class="hq-svg" viewBox={`0 0 ${W} ${H}`} role="img" aria-label={`${t.name} のメンバーとターン`}>{edges}{calls}{nodes}</svg>;
}

type PackedWorker = { worker: FigureWorker; owner: string | null };
type RowItem<T> = { item: T; width: number };
type FigureSlot<T> = { item: T; width: number; x: number; y: number; row: number };

const labelUnits = (value: string): number => {
  let units = 0;
  for (const ch of value) {
    const cp = ch.codePointAt(0) || 0;
    if (cp >= 0x300 && cp <= 0x36f) continue;
    units += cp <= 0xff ? 0.6 : 1;
  }
  return units;
};

// Names use the monospaced label face in hq.css. Count narrow Latin glyphs and
// full-width glyphs at their em widths, leaving a small margin for the font's
// actual side bearings. The same measured width drives both packing and the
// geometry assertion in the activity screen's unit test.
const labelWidth = (value: string, size: number): number => Math.ceil(labelUnits(value) * size * 1.05);

function rowsOf<T>(items: T[], widthOf: (item: T) => number, maxWidth: number, gap: number): RowItem<T>[][] {
  const rows: RowItem<T>[][] = [];
  let row: RowItem<T>[] = [], used = 0;
  for (const item of items) {
    const width = Math.max(28, widthOf(item));
    if (row.length && used + gap + width > maxWidth) {
      rows.push(row);
      row = [];
      used = 0;
    }
    if (row.length) used += gap;
    row.push({ item, width });
    used += width;
  }
  if (row.length) rows.push(row);
  return rows;
}

function placeRows<T>(rows: RowItem<T>[][], W: number, gap: number, y: number, rowStep: number): FigureSlot<T>[] {
  const slots: FigureSlot<T>[] = [];
  rows.forEach((row, rowIndex) => {
    const rowWidth = row.reduce((sum, cell) => sum + cell.width, 0) + Math.max(0, row.length - 1) * gap;
    let left = (W - rowWidth) / 2;
    for (const cell of row) {
      slots.push({ item: cell.item, width: cell.width, x: left + cell.width / 2, y: y + rowIndex * rowStep, row: rowIndex });
      left += cell.width + gap;
    }
  });
  return slots;
}

const nodeState = (state: string): string => {
  const labels: Record<string, string> = {
    active: '仕事中', idle: '止まっている', blocked: '許可待ち', handed: '人間に渡した', absent: '動いていない',
  };
  return labels[state] || state || '状態不明';
};

function PackedFigure({ team, turns, W }: { team: FigureTeam; turns?: [string, string][]; W: number }): JSX.Element {
  const t = team || {}, crew = t.crew || [], workers = t.workers || [];
  const memberRows = rowsOf(crew, (m) => Math.max(38, labelWidth(m.name, 11.5) + 20), W - 32, 12);
  const memberSlots = placeRows(memberRows, W, 12, 124, 84);
  const pos: Record<string, Point> = { human: [W / 2, 30] };
  memberSlots.forEach((slot) => { pos[slot.item.name] = [slot.x, slot.y]; });

  const owned = new Map<string, FigureWorker[]>();
  const free: FigureWorker[] = [];
  for (const worker of workers) {
    if (!pos[worker.from]) { free.push(worker); continue; }
    const list = owned.get(worker.from) || [];
    list.push(worker);
    owned.set(worker.from, list);
  }
  const workerItems: PackedWorker[] = [];
  for (const member of crew) {
    for (const worker of owned.get(member.name) || []) workerItems.push({ worker, owner: member.name });
  }
  for (const worker of free) workerItems.push({ worker, owner: null });

  const workerRows = rowsOf(workerItems, (entry) => Math.max(28, labelWidth(entry.worker.name, 10) + 16), W - 32, 12);
  const workerTop = memberRows.length ? 188 + (memberRows.length - 1) * 84 : 104;
  const workerSlots = placeRows(workerRows, W, 12, workerTop, 54);
  workerSlots.forEach((slot) => { if (slot.item.owner) pos[`worker:${slot.item.worker.name}`] = [slot.x, slot.y]; });
  const lastMemberLabel = memberRows.length ? 124 + (memberRows.length - 1) * 84 + 30 : 0;
  const lastWorkerLabel = workerRows.length ? workerTop + (workerRows.length - 1) * 54 + 20 : 0;
  const H = Math.max(236, lastMemberLabel + 22, lastWorkerLabel + 16);

  const edges: JSX.Element[] = [];
  const seen = new Set<string>();
  for (const [a, b] of turns || []) {
    if (!pos[a] || !pos[b]) continue;
    const key = [a, b].sort().join('|');
    if (seen.has(key)) continue;
    seen.add(key);
    const [x1, y1] = pos[a], [x2, y2] = pos[b];
    const low = Math.max(y1, y2) + 42 + Math.abs(x2 - x1) * 0.06;
    edges.push(<path class="hq-e" d={`M${x1} ${y1} Q${(x1 + x2) / 2} ${low} ${x2} ${y2}`} />);
  }
  const calls: JSX.Element[] = [];
  for (const member of crew) {
    if (member.state !== 'blocked' && member.state !== 'handed') continue;
    const [x, y] = pos[member.name];
    calls.push(<line class="hq-call" x1={x} y1={y - 14} x2={W / 2} y2="44" />);
  }
  const workerLines: JSX.Element[] = [];
  workerSlots.forEach((slot) => {
    const owner = slot.item.owner;
    const worker = pos[`worker:${slot.item.worker.name}`];
    if (!owner || !worker) return;
    const [x1, y1] = pos[owner], [x2, y2] = worker;
    const mid = (y1 + y2) / 2;
    workerLines.push(<path class="hq-e wk" d={`M${x1} ${y1 + 15} C${x1} ${mid} ${x2} ${mid} ${x2} ${y2 - 6}`} />);
  });

  const nodes: JSX.Element[] = [<g class="hq-n human"><rect x={W / 2 - 26} y="16" width="52" height="28" rx="6" /><text x={W / 2} y="35">人間</text></g>];
  memberSlots.forEach((slot) => {
    const member = slot.item;
    const state = nodeState(member.state);
    nodes.push(<g class={'hq-n ' + member.state}>
      <title>{`${member.name} · ${state}${member.doing ? ` · ${member.doing}` : ''}`}</title>
      <circle class="ring" cx={slot.x} cy={slot.y} r="15" />
      <circle class="dot" cx={slot.x} cy={slot.y} r="11" />
      {member.state === 'blocked' ? <text class="bang" x={slot.x} y={slot.y + 4}>!</text> : ''}
      <text class="lbl" x={slot.x} y={slot.y + 30} data-label-width={slot.width} data-label-row={`member-${slot.row}`}>{member.name}</text>
    </g>);
  });
  workerSlots.forEach((slot) => {
    const { worker, owner } = slot.item;
    const state = nodeState(worker.state);
    const ownerText = owner ? `${owner} のワーカー` : '持ち主を特定できないワーカー';
    nodes.push(<g class={'hq-n wkr ' + worker.state} data-owner={owner || ''}>
      <title>{`${worker.name} · ${ownerText} · ${state}${worker.doing ? ` · ${worker.doing}` : ''}`}</title>
      <circle class="dot" cx={slot.x} cy={slot.y} r="6" />
      <text class="lbl s" x={slot.x} y={slot.y + 20} data-label-width={slot.width} data-label-row={`worker-${slot.row}`}>{worker.name}</text>
    </g>);
  });
  const summary = [
    `メンバー: ${crew.map((member) => `${member.name} ${nodeState(member.state)}`).join('、') || 'なし'}`,
    `ワーカー: ${workers.map((worker) => `${worker.name}（${worker.from || '持ち主不明'}） ${nodeState(worker.state)}`).join('、') || 'なし'}`,
  ].join('。');
  return <svg class="hq-svg packed" viewBox={`0 0 ${W} ${H}`} role="img" aria-label={`${t.name} のアクティビティ図。${summary}`}>
    {edges}{calls}{workerLines}{nodes}
  </svg>;
}
