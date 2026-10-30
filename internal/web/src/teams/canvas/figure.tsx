// teams/canvas/figure.tsx — 構成図の図（点と線）を描く（spec-teams B7、n-6be7）。
//
// 図は vnode で描くので、描き直しで点や線の要素が入れ替わらない（Tab で当てた
// カーソルが外れない、webui-preact-design 9 章）。点の場所は place.ts が組み立て、
// ここはそれを SVG にするだけ。押し込みは canvas が受ける（5 章 1）。
import type { JSX } from 'preact';
import { boxOf } from '../place.js';
import type { Box, Spot } from '../place.js';
import { dkey, hasTurn, memberOf, nodeName, teamIssues } from '../state.js';
import type { Finding, Rec, Worker } from '../state.js';
import { edgePt, layoutOf, pair, selOf } from './shared.js';

// Bdg is the 問題 / 警告 mark a point carries (監査 R9).
function Bdg({ m, b }: { m: Finding; b: Box }) {
  const w = 34;
  return (
    <g class={'bdg ' + m.sev} transform={`translate(${(b.w / 2 - w / 2 - 2).toFixed(1)},${(-b.h / 2 - 6).toFixed(1)})`}>
      <rect x={-w / 2} y={-9} width={w} height={17} rx={3}></rect>
      <text x={0} y={4} text-anchor="middle">{m.sev === 'err' ? '問題' : '警告'}</text>
    </g>
  );
}

// ---- 図 --------------------------------------------------------------
// Frame is the room the figure is drawn in: the box of every point, and the
// box the viewBox shows (the points' outline with a margin).
export type Frame = { boxes: Record<string, Box>; vx: number; vy: number; vw: number; vh: number };

// frameOf is the figure's own frame, read by the drawing and by the screen that
// gives it the room it has (r-e39c).
export function frameOf(r: Rec): Frame {
  const t = r.draft.value, p = layoutOf(r);
  const ids = ['human', ...t.members.map((m) => m.name)];
  const all = ids.concat(t.workers.map((_w, i) => 'w' + i));
  const boxes: Record<string, Box> = {};
  all.forEach((id) => { boxes[id] = boxOf(id, t); });
  const xs = all.flatMap((id) => [p[id][0] - boxes[id].w / 2, p[id][0] + boxes[id].w / 2]);
  const ys = all.flatMap((id) => [p[id][1] - boxes[id].h / 2, p[id][1] + boxes[id].h / 2]);
  const pad = 40, vx = Math.min(...xs) - pad, vy = Math.min(...ys) - pad, vw = Math.max(...xs) - vx + pad, vh = Math.max(...ys) - vy + pad;
  return { boxes, vx, vy, vw, vh };
}

export function Figure({ r }: { r: Rec }) {
  const t = r.draft.value, s = r.saved.value, p = layoutOf(r), sel = selOf(r), issues = teamIssues(r) || [];
  const { boxes, vx, vy, vw, vh } = frameOf(r);
  const ids = ['human', ...t.members.map((m) => m.name)];
  const mi = dkey(r).replace(/[^A-Za-z0-9_-]/g, '_');

  const turns = new Map<string, [string, string][]>();
  t.turns.forEach(([a, b]) => {
    const k = pair(a, b);
    const list = turns.get(k) || [];
    list.push([a, b]);
    turns.set(k, list);
  });
  const selE = sel && sel.type === 'edge' ? sel.k : null;
  const edges = [...turns.keys()].map((k) => {
    const [a, b] = k.split('|');
    if (!p[a] || !p[b]) return '';
    const p1 = edgePt(p[a], boxes[a], p[b]), p2 = edgePt(p[b], boxes[b], p[a]);
    const fwd = hasTurn(t, a, b), back = hasTurn(t, b, a);
    const isNew = (fwd && !hasTurn(s, a, b)) || (back && !hasTurn(s, b, a));
    const mk = selE === k ? 'a' : isNew ? 'c' : 'n';
    const dd = `M${p1[0].toFixed(1)},${p1[1].toFixed(1)} L${p2[0].toFixed(1)},${p2[1].toFixed(1)}`;
    const [mx, my] = [(p1[0] + p2[0]) / 2, (p1[1] + p2[1]) / 2];
    return (
      <>
        <path class="cv-hit" d={dd} data-cv-e={k} tabindex={0} role="button" aria-label={`${nodeName(a)} と ${nodeName(b)} のターン`}></path>
        <path class={'cv-e' + (isNew ? ' new' : '') + (selE === k ? ' sel' : '') + (r.flash.value === k ? ' cv-flash' : '')} d={dd} marker-end={fwd ? `url(#cv-ar-${mi}-${mk})` : undefined} marker-start={back ? `url(#cv-ar-${mi}-${mk})` : undefined}></path>
        {selE === k ? <g class="cv-ex" data-cv-act="edge-del" data-k={k} transform={`translate(${mx.toFixed(1)},${my.toFixed(1)})`} role="button" tabindex={0} aria-label="この線を外す（両方の向き）"><title>この線を外す（両方の向き）</title><circle r={11}></circle><text x={0} y={4.5} text-anchor="middle">×</text></g> : ''}
      </>
    );
  });
  const wedges = t.workers.map((w, i) => {
    if (!p['w' + i] || !p[w.from]) return '';
    const p1 = edgePt(p[w.from], boxes[w.from], p['w' + i]), p2 = edgePt(p['w' + i], boxes['w' + i], p[w.from]);
    return <path class="cv-e wk" d={`M${p1[0].toFixed(1)},${p1[1].toFixed(1)} L${p2[0].toFixed(1)},${p2[1].toFixed(1)}`}></path>;
  });

  const savedNames = new Set(s.members.map((m) => m.name));
  const node = (id: string) => {
    const b = boxes[id], x = p[id][0], y = p[id][1], mine = sel && sel.type === 'node' && sel.id === id;
    const is = issues.filter((m) => m.node === id);
    if (id === 'human') {
      return (
        <g class={'cv-n human' + (mine ? ' sel' : '') + (is.some((m) => m.sev === 'err') ? ' err' : '')} data-cv-n="human" transform={`translate(${x.toFixed(1)},${y.toFixed(1)})`} tabindex={0} role="button" aria-label="人間">
          <rect x={-b.w / 2} y={-b.h / 2} width={b.w} height={b.h} rx={18}></rect>
          <text class="nm" x={0} y={5} text-anchor="middle">人間</text>
          {is.length ? <Bdg m={is[0]} b={b} /> : ''}
        </g>
      );
    }
    const m = memberOf(t, id), sm = s.members.find((z) => z.name === id);
    const cls = !savedNames.has(id) ? ' new' : m && sm && (sm.role !== m.role || sm.profile !== m.profile) ? ' chg' : '';
    return (
      <g class={'cv-n member' + cls + (mine ? ' sel' : '') + (is.some((m) => m.sev === 'err') ? ' err' : '')} data-cv-n={id} transform={`translate(${x.toFixed(1)},${y.toFixed(1)})`} tabindex={0} role="button" aria-label={id}>
        <rect x={-b.w / 2} y={-b.h / 2} width={b.w} height={b.h} rx={6}></rect>
        <text class="nm" x={0} y={-3} text-anchor="middle">{id}</text>
        <text class="rl" x={0} y={14} text-anchor="middle">{m ? m.role : ''}</text>
        {is.length ? <Bdg m={is[0]} b={b} /> : ''}
      </g>
    );
  };
  const wexists = new Set(s.workers.map((w) => w.from + '\u0000' + w.purpose));
  const wnode = (w: Worker, i: number) => {
    const b = boxes['w' + i], x = p['w' + i][0], y = p['w' + i][1], mine = sel && sel.type === 'worker' && sel.i === i, is = issues.filter((m) => m.worker === i);
    return (
      <g class={'cv-n worker' + (wexists.has(w.from + '\u0000' + w.purpose) ? '' : ' new') + (mine ? ' sel' : '') + (is.some((m) => m.sev === 'err') ? ' err' : '')} data-cv-w={i} transform={`translate(${x.toFixed(1)},${y.toFixed(1)})`} tabindex={0} role="button" aria-label={`${w.from} の ${w.purpose || '（目的）'}`}>
        <rect x={-b.w / 2} y={-b.h / 2} width={b.w} height={b.h} rx={5}></rect>
        <text class="nm" x={0} y={-2} text-anchor="middle">{w.purpose || '（目的）'}</text>
        <text class="rl" x={0} y={13} text-anchor="middle">{w.profile}</text>
        {is.length ? <Bdg m={is[0]} b={b} /> : ''}
      </g>
    );
  };
  let handles: JSX.Element[] | '' = '';
  if (sel && sel.type === 'node' && p[sel.id] && boxes[sel.id]) {
    const b = boxes[sel.id], [x, y] = p[sel.id];
    const grips: Spot[] = [[0, -b.h / 2], [b.w / 2, 0], [0, b.h / 2], [-b.w / 2, 0]];
    handles = grips.map(([dx, dy]) => (
      <circle class="cv-h" cx={(x + dx).toFixed(1)} cy={(y + dy).toFixed(1)} r={6} data-cv-h={sel.id}><title>引いて、ターンを渡す相手につなぐ</title></circle>
    ));
  }
  const marker = (id: string, cls: string) => (
    <marker id={`cv-ar-${mi}-${id}`} viewBox="0 0 10 10" refX={9} refY={5} markerWidth={7.5} markerHeight={7.5} orient="auto-start-reverse">
      <path class={'cv-mk-' + cls} d="M0,0 L10,5 L0,10 z"></path>
    </marker>
  );
  return (
    <svg width={Math.round(vw)} height={Math.round(vh)} viewBox={`${vx.toFixed(0)} ${vy.toFixed(0)} ${vw.toFixed(0)} ${vh.toFixed(0)}`} data-vw={Math.round(vw)} data-vh={Math.round(vh)} role="group" aria-label={`${r.name.value} のトポロジー`}>
      <defs>{marker('n', 'n')}{marker('a', 'a')}{marker('c', 'c')}</defs>
      {wedges}{edges}{t.workers.map(wnode)}{ids.map(node)}{handles}<path class="cv-ghost" d="" hidden="hidden"></path><text class="cv-gl" x={0} y={0} hidden="hidden"></text>
    </svg>
  );
}
