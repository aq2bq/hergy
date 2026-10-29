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

type RowItem<T> = { item: T; width: number };
type FigureSlot<T> = { item: T; width: number; x: number; y: number; row: number };
type FigureColumn = {
  member: FigureMember | null;
  owner: string | null;
  width: number;
  rows: RowItem<FigureWorker>[][];
  x: number;
};

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
  const owned = new Map<string, FigureWorker[]>();
  const free: FigureWorker[] = [];
  for (const worker of workers) {
    if (!crew.some((member) => member.name === worker.from)) { free.push(worker); continue; }
    const list = owned.get(worker.from) || [];
    list.push(worker);
    owned.set(worker.from, list);
  }
  const workerWidth = (worker: FigureWorker) => Math.max(28, labelWidth(worker.name, 10) + 16);
  const workerGap = 0;
  const columns: FigureColumn[] = crew.map((member) => {
    const group = owned.get(member.name) || [];
    const widest = group.map(workerWidth).sort((a, b) => b - a)[0] || 0;
    const width = Math.max(38, labelWidth(member.name, 11.5) + 8, widest);
    return {
      member,
      owner: member.name,
      width,
      rows: rowsOf(group, workerWidth, widest, workerGap),
      x: 0,
    };
  });
  if (free.length) {
    const width = Math.max(...free.map(workerWidth));
    columns.push({
      member: null,
      owner: null,
      width,
      rows: rowsOf(free, workerWidth, width, workerGap),
      x: 0,
    });
  }

  // Keep each member and the workers they hired in one column. A column grows
  // to fit its names and its widest worker row; the SVG grows with it instead
  // of moving a worker into another member's column or clipping a label.
  const columnGap = 8;
  const contentWidth = columns.reduce((sum, column) => sum + column.width, 0) + Math.max(0, columns.length - 1) * columnGap;
  const viewWidth = Math.max(W, contentWidth + 16);
  let left = (viewWidth - contentWidth) / 2;
  for (const column of columns) {
    column.x = left + column.width / 2;
    left += column.width + columnGap;
  }

  const memberY = 136;
  const workerTop = 210;
  const workerStep = 48;
  const workerSlots = new Map<FigureColumn, FigureSlot<FigureWorker>[]>();
  let workerRowCount = 0;
  for (const column of columns) {
    const placed = placeRows(column.rows, column.width, workerGap, workerTop, workerStep)
      .map((slot) => ({ ...slot, x: slot.x + column.x - column.width / 2 }));
    workerSlots.set(column, placed);
    workerRowCount = Math.max(workerRowCount, column.rows.length);
  }
  const lastWorkerLabel = workerRowCount ? workerTop + (workerRowCount - 1) * workerStep + 20 : 0;
  const H = Math.max(236, memberY + 50, lastWorkerLabel + 18);
  const pos: Record<string, Point> = { human: [viewWidth / 2, 30] };
  columns.forEach((column) => {
    if (column.member) pos[column.member.name] = [column.x, memberY];
  });

  const humanEdges: JSX.Element[] = [];
  const turnEdges: JSX.Element[] = [];
  const seen = new Set<string>();
  for (const [a, b] of turns || []) {
    if (!pos[a] || !pos[b]) continue;
    const key = [a, b].sort().join('|');
    if (seen.has(key)) continue;
    seen.add(key);
    const [x1, y1] = pos[a], [x2, y2] = pos[b];
    if (a === 'human' || b === 'human') {
      const memberX = a === 'human' ? x2 : x1;
      humanEdges.push(<line class="hq-e turn" data-from={a} data-to={b}
        x1={a === 'human' ? viewWidth / 2 : memberX} y1={a === 'human' ? 44 : memberY - 15}
        x2={b === 'human' ? viewWidth / 2 : memberX} y2={b === 'human' ? 44 : memberY - 15} />);
      continue;
    }
    const archY = Math.max(58, Math.min(y1, y2) - 64 - Math.abs(x2 - x1) * 0.025);
    turnEdges.push(<path class="hq-e turn" data-from={a} data-to={b} data-control-y={archY}
      d={`M${x1} ${y1 - 15} Q${(x1 + x2) / 2} ${archY} ${x2} ${y2 - 15}`} />);
  }
  const calls: JSX.Element[] = [];
  for (const member of crew) {
    if (member.state !== 'blocked' && member.state !== 'handed') continue;
    const [x, y] = pos[member.name];
    calls.push(<line class="hq-call" x1={x} y1={y - 14} x2={viewWidth / 2} y2="44" />);
  }
  const columnNodes = columns.map((column) => {
    const edges: JSX.Element[] = [];
    const nodes: JSX.Element[] = [];
    const slots = workerSlots.get(column) || [];
    const left = column.x - column.width / 2;
    const right = column.x + column.width / 2;
    if (column.member) {
      const member = column.member;
      const state = nodeState(member.state);
      const label = Math.max(38, labelWidth(member.name, 11.5) + 8);
      nodes.push(<g class={'hq-n ' + member.state}>
        <title>{`${member.name} · ${state}${member.doing ? ` · ${member.doing}` : ''}`}</title>
        <circle class="ring" cx={column.x} cy={memberY} r="15" />
        <circle class="dot" cx={column.x} cy={memberY} r="11" />
        {member.state === 'blocked' ? <text class="bang" x={column.x} y={memberY + 4}>!</text> : ''}
        <text class="lbl" x={column.x} y={memberY + 30} data-label-width={label} data-label-row="member">{member.name}</text>
      </g>);
      if (slots.length) {
        // The stem leaves the member beside its name, stays in this column's
        // gutter, and branches only within the same column to its workers.
        const gutterX = right - 3;
        for (const slot of slots) {
          edges.push(<path class="hq-e wk" data-owner={member.name}
            d={`M${column.x + 15} ${memberY + 4} H${gutterX} V${slot.y} H${slot.x} V${slot.y - 6}`} />);
        }
      }
    }
    slots.forEach((slot) => {
      const worker = slot.item;
      const owner = column.owner;
      const state = nodeState(worker.state);
      const ownerText = owner ? `${owner} のワーカー` : '持ち主を特定できないワーカー';
      nodes.push(<g class={'hq-n wkr ' + worker.state} data-owner={owner || ''}>
        <title>{`${worker.name} · ${ownerText} · ${state}${worker.doing ? ` · ${worker.doing}` : ''}`}</title>
        <circle class="dot" cx={slot.x} cy={slot.y} r="6" />
        <text class="lbl s" x={slot.x} y={slot.y + 20} data-label-width={slot.width} data-label-row={`worker-${slot.row}`}>{worker.name}</text>
      </g>);
    });
    return <g class="hq-column" data-member={column.owner || ''} data-x-left={left} data-x-right={right}>{edges}{nodes}</g>;
  });

  const nodes: JSX.Element[] = [<g class="hq-n human"><rect x={viewWidth / 2 - 26} y="16" width="52" height="28" rx="6" /><text x={viewWidth / 2} y="35">人間</text></g>];
  const summary = [
    `メンバー: ${crew.map((member) => `${member.name} ${nodeState(member.state)}`).join('、') || 'なし'}`,
    `ワーカー: ${workers.map((worker) => `${worker.name}（${worker.from || '持ち主不明'}） ${nodeState(worker.state)}`).join('、') || 'なし'}`,
  ].join('。');
  return <svg class="hq-svg packed" viewBox={`0 0 ${viewWidth} ${H}`} role="img" aria-label={`${t.name} のアクティビティ図。${summary}`}>
    {humanEdges}{turnEdges}{calls}{nodes}{columnNodes}
  </svg>;
}
