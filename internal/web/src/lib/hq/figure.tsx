// lib/hq/figure.tsx — チームの図（spec 6 章）。活動の画面のタイルと詳細が同じ 1 つの
// 図を描くための部品（webui-preact-design 5 章 1。前は hq.js が SVG の文字列を返し、
// 活動の画面が dangerouslySetInnerHTML で流し込んでいた）。
//
// 人間が上、メンバーが 1 列、ワーカーはそれを足したメンバーの下。ターンは線で、
// 人間を待っているメンバーは人間への線で描く。図は「操作しない図」: 押せるものも
// フォーカスの入るものも無い（role="img"）。
import type { JSX } from 'preact';

type FigureMember = { name: string; state: string };
type FigureWorker = { name: string; state: string; from: string };
type FigureTeam = { name: string; crew?: FigureMember[]; workers?: FigureWorker[] };
type Point = [number, number];
type Props = { team: FigureTeam; turns?: [string, string][]; W?: number; H?: number };

export function TeamFigure({ team, turns, W = 440, H = 236 }: Props): JSX.Element {
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
