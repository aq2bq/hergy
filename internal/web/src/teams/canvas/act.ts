// teams/canvas/act.ts — the canvas's own presses, keys and drags（spec-teams B7）。
//
// 押し込みはここに集める: 図と属性の列は描き、押し込みは自分の器で受けてここの
// 関数を呼ぶ（webui-preact-design 5 章 1）。キーはページに、ドラッグの move と
// up は document に立てる（点の Delete はポインタが焦点を外した後に来るし、
// 図の外へ出たドラッグも落ちる）。
import { boxOf } from '../place.js';
import type { Spot } from '../place.js';
import {
  byKey, canvasAct, canvasEndTyping, canvasToast, canvasTyping, canvasUndo, dkey,
  hasTurn, memberOf, nodeName, openAgree, profilesFor, recs, rolesFor,
  selectPick, undoRow,
} from '../state.js';
import type { CanvasPick, Member, Op, Rec, Team, Worker } from '../state.js';
import { NAME, layoutOf, menuOf, pair, putPlace, renamePlace, selOf, setMenu, setTyped } from './shared.js';

// ---- 操作 ------------------------------------------------------------
// endField takes the focus away from the field the person was typing in: a press
// on the figure is a press on a point, and the Delete that follows is meant for
// the point.
function endField(): void {
  const act = document.activeElement;
  if (act instanceof HTMLElement && act.matches('input, select, textarea') && act.closest('.panel.team-p')) act.blur();
}

function menuClose(r: Rec | null): void {
  if (!r || !menuOf(r)) return;
  setMenu(r, null);
}
// openMenu opens the blank space's two ways of adding at one place (the mock of
// 2026-10-23).
function openMenu(r: Rec, at: Spot | null, e: { clientX: number; clientY: number } | null, workersOnly?: boolean): void {
  const panelEl = document.querySelector<HTMLElement>(`.panel.team-p[data-key="${CSS.escape(dkey(r))}"]`);
  const canvas = panelEl && panelEl.querySelector('.cv-canvas');
  if (!canvas) return;
  const cr = canvas.getBoundingClientRect();
  const x = e ? e.clientX - cr.left : 6, y = e ? e.clientY - cr.top : 6;
  setMenu(r, { x, y, ax: at ? at[0] : null, ay: at ? at[1] : null, workersOnly: !!workersOnly });
}
const rowOf = (el: Element | null): Rec | null => {
  if (!el) return null;
  const row = el.closest('.tdef');
  return row instanceof HTMLElement && row.dataset.key ? byKey(row.dataset.key) : null;
};
const svgPt = (svg: SVGSVGElement, at: { clientX: number; clientY: number }): DOMPoint => {
  const p = svg.createSVGPoint();
  p.x = at.clientX; p.y = at.clientY;
  const m = svg.getScreenCTM();
  return m ? p.matrixTransform(m.inverse()) : p;
};
function firstRole(r: Rec): string {
  const rs = rolesFor(r) || [];
  return (rs[0] || { name: '' }).name;
}
function firstProfile(cheap: boolean, r: Rec): string {
  const ps = profilesFor(r);
  return (cheap ? ps.find((p) => /haiku|mini|flash/.test(p)) : '') || ps[0] || '';
}
function addMember(r: Rec, at: Spot | null): void {
  const t = r.draft.value, role = firstRole(r), profile = firstProfile(false, r);
  let n = 1; const base = role || 'member';
  while (t.members.some((m) => m.name === `${base}-${n}`)) n++;
  const name = `${base}-${n}`;
  if (at) putPlace(r, name, at);
  selectPick(r, { type: 'node', id: name }, 'attr');
  canvasAct(r, [{ kind: 'addMember', member: name, role, profile }], {
    message: `メンバー ${name} を足した。メンバー名・ロール・プロファイルを右で決める`,
    undo: `メンバー ${name} を足したのを取り消した`, focus: '[data-cv-f="m-name"]',
  });
}
// copyName is the free name a copy of one member takes: the member's own name with
// -2, then -3, -4, …, until the draft holds no such name and the herdr name still
// fits its pattern (d-48a9). Empty when no number fits: a copy needs a name hergy
// can start.
function copyName(r: Rec, t: Team, m: Member): string {
  for (let n = 2; ; n++) {
    const name = `${m.name}-${n}`;
    if (!NAME.test(name) || `${r.name.value}-${name}`.length > 32) return '';
    if (!t.members.some((x) => x.name === name)) return name;
  }
}
// copyMember adds one member that is a copy of the one picked: the same role and
// profile, the same turns out of it and into it, and its workers' purposes
// (d-48a9). It is one operation for the undo — the same edits a person would make
// by hand, so no new kind of op — and the copy stands picked, its name field
// waiting.
function copyMember(r: Rec): void {
  const t = r.draft.value, sel = selOf(r);
  const m = sel && sel.type === 'node' ? memberOf(t, sel.id) : null;
  if (!m) return;
  const name = copyName(r, t, m);
  if (!name) {
    canvasToast(r, `メンバー ${m.name} の複製に使える名前がありません（herdr 名が 32 文字を超えます）`, false);
    return;
  }
  const ops: Op[] = [{ kind: 'addMember', member: name, role: m.role, profile: m.profile }];
  t.turns.forEach(([a, b]) => {
    // A member that hands the turn to itself is one turn, not two.
    if (a === m.name) ops.push({ kind: 'addTurn', from: name, to: b });
    else if (b === m.name) ops.push({ kind: 'addTurn', from: a, to: name });
  });
  t.workers.forEach((w) => {
    if (w.from === m.name) ops.push({ kind: 'addWorker', from: name, purpose: w.purpose, profile: w.profile });
  });
  selectPick(r, { type: 'node', id: name }, 'attr');
  canvasAct(r, ops, {
    message: `メンバー ${m.name} を複製した（${name}）`,
    undo: `メンバー ${m.name} の複製を取り消した`,
    focus: '[data-cv-f="m-name"]',
  });
}

function addWorker(r: Rec, from: string, at: Spot | null): void {
  const t = r.draft.value, idx = t.workers.length, profile = firstProfile(true, r);
  const ops: Op[] = [{ kind: 'addWorker', from, purpose: '', profile }];
  if (at) putPlace(r, 'w' + idx, at);
  selectPick(r, { type: 'worker', i: idx }, 'attr');
  canvasAct(r, ops, { message: `${from} のワーカーを足した。目的を右で決める`, undo: 'ワーカーを足したのを取り消した', focus: '[data-cv-f="w-purpose"]' });
}
function del(r: Rec): void {
  const t = r.draft.value, sel = selOf(r);
  if (!sel) return;
  if (sel.type === 'node' && sel.id !== 'human') {
    const id = sel.id, nt = t.turns.filter(([a, b]) => a === id || b === id).length, nw = t.workers.filter((w) => w.from === id).length;
    selectPick(r, null);
    canvasAct(r, [{ kind: 'removeMember', member: id }], { message: `${id} を外した（ターン ${nt} 本・ワーカー ${nw} 件も）`, undo: `${id} を外したのを取り消した` });
  } else if (sel.type === 'edge') {
    const [a, b] = sel.k.split('|'), ops: Op[] = [];
    if (hasTurn(t, a, b)) ops.push({ kind: 'removeTurn', from: a, to: b });
    if (hasTurn(t, b, a)) ops.push({ kind: 'removeTurn', from: b, to: a });
    selectPick(r, null);
    if (ops.length) canvasAct(r, ops, { message: `${nodeName(a)} ⇄ ${nodeName(b)} のターンを外した`, undo: `${nodeName(a)} ⇄ ${nodeName(b)} のターンを外したのを取り消した` });
  } else if (sel.type === 'worker') {
    const w = t.workers[sel.i];
    if (!w) return;
    selectPick(r, null);
    canvasAct(r, [{ kind: 'removeWorker', from: w.from, purpose: w.purpose }], { message: `ワーカー ${w.purpose || '（目的）'} を外した`, undo: 'ワーカーを外したのを取り消した' });
  }
}

// Drag is the pointer's own drag while it is out: a point being moved, or a line
// being drawn from one point's handle to another.
type DragMove = { what: 'move'; r: Rec; id: string; wi: number | null; svg: SVGSVGElement; start: DOMPoint; orig: Spot; moved: boolean };
type DragConn = { what: 'conn'; r: Rec; from: string; svg: SVGSVGElement; start: DOMPoint };
type Drag = DragMove | DragConn;

// lastPress and DRAG are the part's own short-lived memory: the double press on
// the blank space, and the pointer's drag while it is out.
let lastPress: { t: number; x: number; y: number; r: Rec } | null = null;
let DRAG: Drag | null = null;

export function onPointerDown(r: Rec, e: PointerEvent): void {
  const target = e.target instanceof Element ? e.target : null;
  if (!target) return;
  const found = target.closest('.cv-canvas svg');
  const svg = found instanceof SVGSVGElement ? found : null;
  if (!svg) return;
  // A press on the figure ends what the field held: the person has moved on to
  // a point, and the next Delete is meant for the point, not for the text.
  endField();
  if (target.closest('[data-cv-act]')) return;
  const h = target.closest<HTMLElement>('[data-cv-h]');
  if (h) { DRAG = { what: 'conn', r, from: h.dataset.cvH || '', svg, start: svgPt(svg, e) }; svg.setPointerCapture(e.pointerId); e.preventDefault(); return; }
  const n = target.closest<HTMLElement>('[data-cv-n], [data-cv-w]');
  if (n) {
    const id = n.dataset.cvN || ('w' + n.dataset.cvW);
    const spot = layoutOf(r)[id];
    DRAG = { what: 'move', r, id, wi: n.dataset.cvW != null ? Number(n.dataset.cvW) : null, svg, start: svgPt(svg, e), orig: [spot[0], spot[1]], moved: false };
    svg.setPointerCapture(e.pointerId); return;
  }
  const ed = target.closest<HTMLElement>('[data-cv-e]');
  if (ed) { selectPick(r, { type: 'edge', k: ed.dataset.cvE || '' }, 'attr'); return; }
  const now = Date.now(), pt = svgPt(svg, e);
  if (lastPress && now - lastPress.t < 400 && Math.hypot(pt.x - lastPress.x, pt.y - lastPress.y) < 8 && lastPress.r === r) {
    lastPress = null; openMenu(r, [pt.x, pt.y], e); return;
  }
  lastPress = { t: now, x: pt.x, y: pt.y, r };
  menuClose(r);
  if (selOf(r)) selectPick(r, null);
}
function onPointerMove(e: PointerEvent): void {
  const d = DRAG; if (!d) return;
  const r = d.r, t = r.draft.value, svg = d.svg;
  const p = svgPt(svg, e);
  if (d.what === 'move') {
    const dx = p.x - d.start.x, dy = p.y - d.start.y;
    if (!d.moved && Math.hypot(dx, dy) < 4) return;
    d.moved = true;
    // The place goes into the record as the pointer moves, and the element is
    // moved by hand meanwhile: a drawing that comes while the drag is out draws
    // the point where the pointer has it (5 章 1).
    const at: Spot = [d.orig[0] + dx, d.orig[1] + dy];
    putPlace(r, d.id, at);
    const g = svg.querySelector(d.wi != null ? `[data-cv-w="${d.wi}"]` : `[data-cv-n="${CSS.escape(d.id)}"]`);
    if (g) g.setAttribute('transform', `translate(${at[0]},${at[1]})`);
    return;
  }
  // A point is being joined: the line and the words follow the pointer until
  // the drop, and nothing is drawn again meanwhile (the classes stay put).
  const ghost = svg.querySelector('.cv-ghost'), label = svg.querySelector('.cv-gl');
  if (!ghost || !label) return;
  ghost.removeAttribute('hidden');
  ghost.setAttribute('d', `M${d.start.x},${d.start.y} L${p.x},${p.y}`);
  Array.from(svg.querySelectorAll('.cv-n.drop')).forEach((g) => g.classList.remove('drop'));
  const over = document.elementFromPoint(e.clientX, e.clientY), g = over && over.closest<HTMLElement>('[data-cv-n]');
  const to = g && g.dataset.cvN !== d.from ? g.dataset.cvN : null;
  if (to && g) g.classList.add('drop');
  label.removeAttribute('hidden');
  // The words stand to the right of the point's own box and below it, so they
  // never cover the name the person is aiming at (監査 1-6).
  if (to) {
    const c = layoutOf(r)[to], b = boxOf(to, t);
    label.setAttribute('x', String(c[0] + b.w / 2 + 12)); label.setAttribute('y', String(c[1] + b.h / 2 + 18));
  } else {
    label.setAttribute('x', String(p.x + 24)); label.setAttribute('y', String(p.y + 34));
  }
  label.textContent = to ? (hasTurn(t, d.from, to) ? `${nodeName(d.from)} → ${nodeName(to)} はもうある` : `${nodeName(d.from)} → ${nodeName(to)} に渡す`) : `${nodeName(d.from)} から、渡す相手の点まで引く`;
}
function onPointerUp(e: PointerEvent): void {
  const d = DRAG; if (!d) return;
  DRAG = null;
  const r = d.r;
  if (d.what === 'move') {
    if (d.moved) { selectPick(r, null); return; }
    selectPick(r, d.wi != null ? { type: 'worker', i: d.wi } : { type: 'node', id: d.id }, 'attr');
    return;
  }
  const over = document.elementFromPoint(e.clientX, e.clientY), g = over && over.closest<HTMLElement>('[data-cv-n]');
  const to = g && g.dataset.cvN;
  if (!to || to === d.from) return;
  const t = r.draft.value;
  if (hasTurn(t, d.from, to)) {
    selectPick(r, { type: 'edge', k: pair(d.from, to) }, 'attr');
    canvasToast(r, `${nodeName(d.from)} → ${nodeName(to)} はもうある`, false);
    return;
  }
  const k = pair(d.from, to);
  canvasAct(r, [{ kind: 'addTurn', from: d.from, to }], { message: `${nodeName(d.from)} → ${nodeName(to)} のターンを足した`, undo: `${nodeName(d.from)} → ${nodeName(to)} のターンを足したのを取り消した` });
  selectPick(r, { type: 'edge', k }, 'attr');
  r.flash.value = k;
  setTimeout(() => {
    if (r.flash.value !== k) return;
    r.flash.value = null;
  }, 1400);
}
export function onClick(r: Rec, e: MouseEvent): void {
  const target = e.target instanceof Element ? e.target : null;
  if (!target) return;
  // A press made without a pointer (the keyboard's Enter, or a click a walk
  // sends) picks the point or line too: the pointer path is not the only one.
  if (target.closest('.cv-canvas')) endField();
  const hit = target.closest<HTMLElement>('[data-cv-n], [data-cv-w], [data-cv-e]');
  if (hit) {
    const sel: CanvasPick = hit.dataset.cvN ? { type: 'node', id: hit.dataset.cvN }
      : hit.dataset.cvW != null ? { type: 'worker', i: Number(hit.dataset.cvW) }
        : { type: 'edge', k: hit.dataset.cvE || '' };
    if (JSON.stringify(selOf(r)) === JSON.stringify(sel)) return;
    selectPick(r, sel, 'attr');
    return;
  }
  const b = target.closest<HTMLElement>('[data-cv-act], [data-cv-side]');
  if (!b) return;
  const t = r.draft.value, a = b.dataset.cvAct;
  if (b.dataset.cvSide) { r.side.value = b.dataset.cvSide; return; }
  if (a === 'm-add') { addMember(r, null); return; }
  if (a === 'w-add') {
    const sel = selOf(r), from = sel && sel.type === 'node' && sel.id !== 'human' ? sel.id : null;
    if (!from) { openMenu(r, null, null, true); return; }
    addWorker(r, from, null); return;
  }
  if (a === 'menu-m' || a === 'menu-w') {
    const menu = menuOf(r);
    const at: Spot | null = menu && menu.ax != null && menu.ay != null ? [menu.ax, menu.ay] : null;
    setMenu(r, null);
    if (a === 'menu-m') addMember(r, at);
    else addWorker(r, b.dataset.from || '', at);
    return;
  }
  if (a === 'to-yaml') { r.side.value = 'yaml'; return; }
  if (a === 'to-agree') { openAgree(r); return; }
  if (a === 'del') { del(r); return; }
  if (a === 'm-copy') { copyMember(r); return; }
  if (a === 'edge-del') {
    const [p, q] = (b.dataset.k || '').split('|'), ops: Op[] = [];
    if (hasTurn(t, p, q)) ops.push({ kind: 'removeTurn', from: p, to: q });
    if (hasTurn(t, q, p)) ops.push({ kind: 'removeTurn', from: q, to: p });
    selectPick(r, null);
    if (ops.length) canvasAct(r, ops, { message: `${nodeName(p)} ⇄ ${nodeName(q)} のターンを外した`, undo: `${nodeName(p)} ⇄ ${nodeName(q)} のターンを外したのを取り消した` });
    return;
  }
  if (a === 'turn-del') {
    canvasAct(r, [{ kind: 'removeTurn', from: b.dataset.a || '', to: b.dataset.b || '' }], { message: `${nodeName(b.dataset.a || '')} → ${nodeName(b.dataset.b || '')} のターンを外した`, undo: `${nodeName(b.dataset.a || '')} → ${nodeName(b.dataset.b || '')} のターンを足したのを取り消した` });
    return;
  }
  if (a === 'turn-add') {
    canvasAct(r, [{ kind: 'addTurn', from: b.dataset.a || '', to: b.dataset.b || '' }], { message: `${nodeName(b.dataset.a || '')} → ${nodeName(b.dataset.b || '')} のターンを足した`, undo: `${nodeName(b.dataset.a || '')} → ${nodeName(b.dataset.b || '')} のターンを足したのを取り消した` });
    return;
  }
  if (a === 'w-del') {
    const w = t.workers[Number(b.dataset.i)]; if (!w) return;
    canvasAct(r, [{ kind: 'removeWorker', from: w.from, purpose: w.purpose }], { message: `ワーカー ${w.purpose || '（目的）'} を外した`, undo: 'ワーカーを外したのを取り消した' });
    return;
  }
  if (a === 'pick-w') selectPick(r, { type: 'worker', i: Number(b.dataset.i) }, 'attr');
}
export function onInput(r: Rec, e: Event): void {
  const el = e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement || e.target instanceof HTMLSelectElement ? e.target : null;
  if (!el) return;
  const f = el.dataset.cvF; if (!f) return;
  const t = r.draft.value, sel = selOf(r), v = el.value;
  if (f === 'turn-add') {
    if (v && sel && sel.type === 'node') {
      canvasAct(r, [{ kind: 'addTurn', from: sel.id, to: v }], { message: `${nodeName(sel.id)} → ${nodeName(v)} のターンを足した`, undo: `${nodeName(sel.id)} → ${nodeName(v)} のターンを足したのを取り消した` });
      el.value = '';
    }
    return;
  }
  if (e.type === 'change' && el.tagName !== 'SELECT') return;
  if (e.type === 'input' && el.tagName === 'SELECT') return;
  if (f === 'mission') { canvasTyping(r, { kind: 'mission', text: v }, 'mission'); return; }
  if (f === 'm-name') {
    const node = sel && sel.type === 'node' ? sel : null;
    const m = node ? memberOf(t, node.id) : null;
    if (!node || !m) return;
    // A name another member holds is not put into the draft, even for a moment:
    // the field keeps the letters and the words beside it say why, so the member
    // being typed cannot take another member's name — and with it that member's
    // turns and workers — while it is typed (r-4fd7, n-4b24).
    const ok = NAME.test(v) && v !== m.name && !nameTaken(t, m, v);
    if (ok) {
      renamePlace(r, m.name, v);
      setTyped(r, null);
      canvasTyping(r, { kind: 'renameMember', member: m.name, new: v }, 'm-name');
      selectPick(r, { type: 'node', id: v }, 'attr');
      return;
    }
    // The messages stand beside the field as it is typed, so a name the server
    // would refuse is said before the field is left (監査 R9).
    setTyped(r, { id: node.id, value: v, msgs: nameMsgs(r, t, m, v) });
    return;
  }
  if (f === 'm-role' || f === 'm-profile') {
    if (!sel || sel.type !== 'node') return;
    canvasAct(r, [{ kind: f === 'm-role' ? 'memberRole' : 'memberProfile', member: sel.id, [f === 'm-role' ? 'role' : 'profile']: v }], { message: `メンバー ${sel.id} の${f === 'm-role' ? 'ロール' : 'プロファイル'}を変えた`, undo: `メンバー ${sel.id} の${f === 'm-role' ? 'ロール' : 'プロファイル'}を変えたのを取り消した` });
    return;
  }
  if (f === 'w-purpose' || f === 'w-from' || f === 'w-profile') {
    const w: Worker | null = sel && sel.type === 'worker' ? t.workers[sel.i] : null; if (!w) return;
    if (f === 'w-purpose') { canvasTyping(r, { kind: 'workerPurpose', from: w.from, old: w.purpose, new: v }, 'w-purpose'); return; }
    if (f === 'w-from') { canvasAct(r, [{ kind: 'moveWorker', from: w.from, purpose: w.purpose, to: v }], { message: `ワーカー ${w.purpose || '（目的）'} を ${v} の下へ移した`, undo: `ワーカー ${w.purpose || '（目的）'} を移したのを取り消した` }); return; }
    canvasAct(r, [{ kind: 'workerProfile', from: w.from, purpose: w.purpose, profile: v }], { message: `ワーカー ${w.purpose || '（目的）'} のプロファイルを変えた`, undo: 'ワーカーのプロファイルを変えたのを取り消した' });
  }
}
// nameTaken is whether another member already holds a name: the draft cannot
// hold two, and a rename would reach the other member's turns and workers.
const nameTaken = (t: Team, m: Member, v: string): boolean => t.members.some((x) => x !== m && x.name === v);
function nameMsgs(r: Rec, t: Team, m: Member, v: string): string[] {
  const msgs: string[] = [];
  if (!NAME.test(v)) msgs.push('英小文字で始め、英小文字・数字・ハイフン・下線で 32 文字以内にしてください');
  if (nameTaken(t, m, v)) msgs.push('このチームにもういます');
  if (`${r.name.value}-${v}`.length > 32) msgs.push(`herdr 名 ${r.name.value}-${v} が 32 文字を超えます`);
  return msgs;
}
// menuRow is the row whose blank space's menu stands open: Esc closes it even
// when no point is picked (監査 2-8).
const menuRow = (): Rec | null => recs.value.find((x) => menuOf(x)) || null;
function onKey(e: KeyboardEvent): void {
  const target = e.target instanceof Element ? e.target : null;
  const inField = target ? target.closest('input, select, textarea') : null;
  // The keys stand on the row a person is working in, not only on what has the
  // focus: a point is picked by the pointer, and the Delete that follows is
  // still meant for it (the mock of 2026-10-23).
  const inside = target ? target.closest('.panel.team-p') : null;
  const panels = Array.from(document.querySelectorAll<HTMLElement>('.panel.team-p')).filter((el) => rowOf(el));
  const picked = (el: HTMLElement): boolean => { const row = rowOf(el); return !!row && !!selOf(row); };
  const panelEl = (inside && rowOf(inside)) ? inside : panels.find(picked);
  const r = panelEl ? rowOf(panelEl) : null;
  // ⌘Z stands for the newest operation even after the pick was let go (the
  // blank space, Esc, or the line's ×): the row that has something to take
  // back is the one it acts on. The field's own ⌘Z is left to the field.
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'z' && !inField) {
    const targetRow = r || undoRow();
    if (targetRow) { e.preventDefault(); canvasUndo(targetRow); }
    return;
  }
  // Esc inside a field leaves the field and keeps what was typed; the next
  // Esc lets the pick go (監査 1-7).
  if (inField) { if (e.key === 'Escape') { e.preventDefault(); if (target instanceof HTMLElement) target.blur(); } return; }
  // The blank space's menu and the toolbar's ask stand even when no point is
  // picked; Esc closes them before anything else (監査 2-8).
  const openMenuRow = menuRow();
  if (e.key === 'Escape' && openMenuRow) { e.preventDefault(); menuClose(openMenuRow); return; }
  if (!r) return;
  const sel = selOf(r);
  if (e.key === 'Delete' || e.key === 'Backspace') { if (sel) { e.preventDefault(); del(r); } return; }
  if (e.key === 'Escape') { menuClose(r); if (sel) { e.preventDefault(); selectPick(r, null); } return; }
  if ((e.key === 'Enter' || e.key === ' ') && (target instanceof HTMLElement || target instanceof SVGElement) && target.matches('.cv-n, .cv-hit, .cv-ex')) {
    e.preventDefault();
    if (target.matches('.cv-hit')) selectPick(r, { type: 'edge', k: target.dataset.cvE || '' }, 'attr');
    else if (target.matches('.cv-ex')) target.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    else {
      const id = target.dataset.cvN || ('w' + target.dataset.cvW);
      selectPick(r, target.dataset.cvN ? { type: 'node', id } : { type: 'worker', i: Number(target.dataset.cvW) }, 'attr');
    }
  }
}

// mountCanvas puts the part's own listeners on the page once: the keys stand on
// the page (a point's Delete comes after the pointer took the focus away), and
// the pointer's move and up are the document's so a drag that leaves the figure
// still lands.
export function mountCanvas(el: HTMLElement): void {
  document.addEventListener('keydown', onKey);
  document.addEventListener('pointermove', onPointerMove);
  document.addEventListener('pointerup', onPointerUp);
  if (!el) return;
  el.addEventListener('focusout', (e) => {
    const target = e.target instanceof HTMLElement ? e.target : null;
    const f = target ? target.dataset.cvF : undefined;
    if (f === 'm-name' || f === 'w-purpose' || f === 'mission') {
      const r = rowOf(target); if (r) canvasEndTyping(r);
    }
  });
}
