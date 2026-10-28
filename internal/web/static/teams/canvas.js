// teams/canvas.js — トポロジー（team.yaml を点と線で見て直す）のカンバス
// （n-6be7、設計者のモック 2026-10-23。webui-preact-design 4 章）。
//
// 画面が図と属性の欄を描き、変える所はすべて画面の ops として state.js へ渡す。
// サーバーがそれを検査し、team.yaml としてそのまま書く（spec-teams B7、
// spec-boundary 5）。この部品が持つのは定義に無いものだけ: どの点・線を選んだか、
// 右の欄が 属性 か YAML か、点をどこへ引いたか。点の位置は保存しない（開き直すと
// 引き直す、d-a575）。
//
// 押し込みは部品が自分の器で受ける（5 章 1）。図は vnode で描くので、描き直しで
// 点や線の要素が入れ替わらない（Tab で当てたカーソルが外れない、9 章）。
import { html, useLayoutEffect, useRef } from '../vendor/preact.js';
import { Counts } from '../lib/ruledoc.js';
import { boxOf } from './place.js';
import {
  recs, byKey, dkey, memberOf, nodeName, teamIssues, rolesFor, profilesFor, rolesIn,
  promptsOf, promptBlockedOf, workerPrompt, canvasAct, canvasTyping, canvasEndTyping,
  canvasUndo, canvasToast, undoRow, openAgree, selectPick, pressLine,
} from './state.js';

const NAME = /^[a-z][a-z0-9_-]{0,31}$/;

// ---- the part's own state ---------------------------------------------
// The picks stand on the record of the definition they belong to (state.js
// keeps them, and drops them when the row is saved, deleted or takes another
// place, d-a575). Where the points stand is the record's too (r.placed, and the
// layout built from it, teams/place.js): a drag writes the element's transform
// as the pointer moves and puts the place in the record, so the figure drawn
// after it stands where the pointer left the point.
let root = null;
// GLOW is the preview's own bookkeeping: the lines drawn last, the lines one
// edit just moved, and when they lit up (spec-teams B7).
const GLOW = { last: {}, step: {}, glow: {}, timer: {} };

// livePick is the pick the draft still holds: a point the draft no longer has
// is no pick, and the panel would show an attribute of nothing.
function livePick(r, sel) {
  if (!sel) return null;
  const t = r.draft.value;
  if (sel.type === 'node' && sel.id !== 'human' && !memberOf(t, sel.id)) return null;
  if (sel.type === 'worker' && !t.workers[sel.i]) return null;
  if (sel.type === 'edge' && !sel.k.split('|').every((x) => x === 'human' || memberOf(t, x))) return null;
  return sel;
}
export const selOf = (r) => livePick(r, r.sel.value);
const sideOf = (r) => r.side.value || 'attr';
// layoutOf is every point's place as this row's drawing has it: the places a
// person put points down at, and an automatic place for the rest. It is a
// computed on the record, so a change to the draft or to a place draws the
// figure again (webui-components 1 章).
const layoutOf = (r) => r.layout.value;
// putPlace puts one point down where a person put it (a double press's menu, or
// a drag that was let go).
function putPlace(r, id, at) {
  r.placed.value = { ...r.placed.value, [id]: [at[0], at[1]] };
}
// renamePlace carries the place of a member the person renamed.
function renamePlace(r, from, to) {
  const { [from]: was, ...rest } = r.placed.value;
  r.placed.value = was ? { ...rest, [to]: was } : rest;
}

const edgePt = (a, box, b) => {
  const dx = b[0] - a[0], dy = b[1] - a[1];
  if (!dx && !dy) return a;
  const k = Math.min((box.w / 2 + 3) / Math.abs(dx || 1e-6), (box.h / 2 + 3) / Math.abs(dy || 1e-6));
  return [a[0] + dx * k, a[1] + dy * k];
};
const hasTurn = (t, a, b) => (t.turns || []).some(([p, q]) => p === a && q === b);
const pair = (a, b) => [a, b].sort().join('|');

// bdg is the 問題 / 警告 mark a point carries (監査 R9).
function Bdg({ m, b }) {
  const w = 34;
  return html`<g class=${'bdg ' + m.sev} transform=${`translate(${(b.w / 2 - w / 2 - 2).toFixed(1)},${(-b.h / 2 - 6).toFixed(1)})`}><rect x=${-w / 2} y="-9" width=${w} height="17" rx="3"></rect><text x="0" y="4" text-anchor="middle">${m.sev === 'err' ? '問題' : '警告'}</text></g>`;
}

// ---- 図 --------------------------------------------------------------
function Figure({ r }) {
  const t = r.draft.value, s = r.saved.value, p = layoutOf(r), sel = selOf(r), issues = teamIssues(r) || [];
  const ids = ['human', ...t.members.map((m) => m.name)];
  const all = ids.concat(t.workers.map((w, i) => 'w' + i));
  const boxes = {};
  all.forEach((id) => { boxes[id] = boxOf(id, t); });
  const xs = all.flatMap((id) => [p[id][0] - boxes[id].w / 2, p[id][0] + boxes[id].w / 2]);
  const ys = all.flatMap((id) => [p[id][1] - boxes[id].h / 2, p[id][1] + boxes[id].h / 2]);
  const pad = 40, vx = Math.min(...xs) - pad, vy = Math.min(...ys) - pad, vw = Math.max(...xs) - vx + pad, vh = Math.max(...ys) - vy + pad;
  const mi = dkey(r).replace(/[^A-Za-z0-9_-]/g, '_');

  const turns = new Map();
  t.turns.forEach(([a, b]) => { const k = pair(a, b); if (!turns.has(k)) turns.set(k, []); turns.get(k).push([a, b]); });
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
    return html`<path class="cv-hit" d=${dd} data-cv-e=${k} tabindex="0" role="button" aria-label=${`${nodeName(a)} と ${nodeName(b)} のターン`}></path><path class=${'cv-e' + (isNew ? ' new' : '') + (selE === k ? ' sel' : '') + (r.flash.value === k ? ' cv-flash' : '')} d=${dd} marker-end=${fwd ? `url(#cv-ar-${mi}-${mk})` : null} marker-start=${back ? `url(#cv-ar-${mi}-${mk})` : null}></path>${selE === k ? html`<g class="cv-ex" data-cv-act="edge-del" data-k=${k} transform=${`translate(${mx.toFixed(1)},${my.toFixed(1)})`} role="button" tabindex="0" aria-label="この線を外す（両方の向き）"><title>この線を外す（両方の向き）</title><circle r="11"></circle><text x="0" y="4.5" text-anchor="middle">×</text></g>` : ''}`;
  });
  const wedges = t.workers.map((w, i) => {
    if (!p['w' + i] || !p[w.from]) return '';
    const p1 = edgePt(p[w.from], boxes[w.from], p['w' + i]), p2 = edgePt(p['w' + i], boxes['w' + i], p[w.from]);
    return html`<path class="cv-e wk" d=${`M${p1[0].toFixed(1)},${p1[1].toFixed(1)} L${p2[0].toFixed(1)},${p2[1].toFixed(1)}`}></path>`;
  });

  const savedNames = new Set(s.members.map((m) => m.name));
  const node = (id) => {
    const b = boxes[id], x = p[id][0], y = p[id][1], mine = sel && sel.type === 'node' && sel.id === id;
    const is = issues.filter((m) => m.node === id);
    if (id === 'human') {
      return html`<g class=${'cv-n human' + (mine ? ' sel' : '') + (is.some((m) => m.sev === 'err') ? ' err' : '')} data-cv-n="human" transform=${`translate(${x.toFixed(1)},${y.toFixed(1)})`} tabindex="0" role="button" aria-label="人間"><rect x=${-b.w / 2} y=${-b.h / 2} width=${b.w} height=${b.h} rx="18"></rect><text class="nm" x="0" y="5" text-anchor="middle">人間</text>${is.length ? html`<${Bdg} m=${is[0]} b=${b} />` : ''}</g>`;
    }
    const m = memberOf(t, id) || {}, sm = s.members.find((z) => z.name === id);
    const cls = !savedNames.has(id) ? ' new' : sm && (sm.role !== m.role || sm.profile !== m.profile) ? ' chg' : '';
    return html`<g class=${'cv-n member' + cls + (mine ? ' sel' : '') + (is.some((m) => m.sev === 'err') ? ' err' : '')} data-cv-n=${id} transform=${`translate(${x.toFixed(1)},${y.toFixed(1)})`} tabindex="0" role="button" aria-label=${id}><rect x=${-b.w / 2} y=${-b.h / 2} width=${b.w} height=${b.h} rx="6"></rect><text class="nm" x="0" y="-3" text-anchor="middle">${id}</text><text class="rl" x="0" y="14" text-anchor="middle">${m.role}</text>${is.length ? html`<${Bdg} m=${is[0]} b=${b} />` : ''}</g>`;
  };
  const wexists = new Set(s.workers.map((w) => w.from + '\u0000' + w.purpose));
  const wnode = (w, i) => {
    const b = boxes['w' + i], x = p['w' + i][0], y = p['w' + i][1], mine = sel && sel.type === 'worker' && sel.i === i, is = issues.filter((m) => m.worker === i);
    return html`<g class=${'cv-n worker' + (wexists.has(w.from + '\u0000' + w.purpose) ? '' : ' new') + (mine ? ' sel' : '') + (is.some((m) => m.sev === 'err') ? ' err' : '')} data-cv-w=${i} transform=${`translate(${x.toFixed(1)},${y.toFixed(1)})`} tabindex="0" role="button" aria-label=${`${w.from} の ${w.purpose || '（目的）'}`}><rect x=${-b.w / 2} y=${-b.h / 2} width=${b.w} height=${b.h} rx="5"></rect><text class="nm" x="0" y="-2" text-anchor="middle">${w.purpose || '（目的）'}</text><text class="rl" x="0" y="13" text-anchor="middle">${w.profile}</text>${is.length ? html`<${Bdg} m=${is[0]} b=${b} />` : ''}</g>`;
  };
  let handles = '';
  if (sel && sel.type === 'node' && p[sel.id] && boxes[sel.id]) {
    const b = boxes[sel.id], [x, y] = p[sel.id];
    handles = [[0, -b.h / 2], [b.w / 2, 0], [0, b.h / 2], [-b.w / 2, 0]].map(([dx, dy]) => html`<circle class="cv-h" cx=${(x + dx).toFixed(1)} cy=${(y + dy).toFixed(1)} r="6" data-cv-h=${sel.id}><title>引いて、ターンを渡す相手につなぐ</title></circle>`);
  }
  const marker = (id, cls) => html`<marker id=${`cv-ar-${mi}-${id}`} viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7.5" markerHeight="7.5" orient="auto-start-reverse"><path class=${'cv-mk-' + cls} d="M0,0 L10,5 L0,10 z"></path></marker>`;
  return html`<svg viewBox=${`${vx.toFixed(0)} ${vy.toFixed(0)} ${vw.toFixed(0)} ${vh.toFixed(0)}`} role="group" aria-label=${`${r.name.value} のトポロジー`}>
      <defs>${marker('n', 'n')}${marker('a', 'a')}${marker('c', 'c')}</defs>
      ${wedges}${edges}${t.workers.map(wnode)}${ids.map(node)}${handles}<path class="cv-ghost" d="" hidden="hidden"></path><text class="cv-gl" x="0" y="0" hidden="hidden"></text></svg>`;
}

// Opt is one choice of a select: the option carries the selected attribute a
// person (and the walks) read, and the property the browser picks by, so the two
// say the same thing (the markup hergy wrote before carried selected="").
function Opt({ v, on, children }) {
  return html`<option value=${v} selected=${on} ref=${(el) => { if (el) el.toggleAttribute('selected', !!on); }}>${children}</option>`;
}

// ---- the right column: 属性 -----------------------------------------
function diffList(r) {
  const s = r.saved.value, t = r.draft.value, out = [];
  const sm = new Map(s.members.map((m) => [m.name, m])), cm = new Map(t.members.map((m) => [m.name, m]));
  if (s.mission !== t.mission) out.push(['c', `使命: ${t.mission}`]);
  cm.forEach((m, n) => {
    if (!sm.has(n)) out.push(['a', `メンバー ${n}（${m.role} · ${m.profile}）`]);
    else { const o = sm.get(n); if (o.role !== m.role) out.push(['c', `${n} のロール ${o.role} → ${m.role}`]); if (o.profile !== m.profile) out.push(['c', `${n} のプロファイル ${o.profile} → ${m.profile}`]); }
  });
  sm.forEach((m, n) => { if (!cm.has(n)) out.push(['d', `メンバー ${n}`]); });
  const sw = new Map(s.workers.map((w) => [w.from + '\u0000' + w.purpose, w])), cw = new Map(t.workers.map((w) => [w.from + '\u0000' + w.purpose, w]));
  cw.forEach((w, k) => {
    if (!sw.has(k)) out.push(['a', `ワーカー ${w.purpose || '（目的）'}（${w.from} · ${w.profile}）`]);
    else { const o = sw.get(k); if (o.purpose !== w.purpose || o.profile !== w.profile || o.from !== w.from) out.push(['c', `ワーカー ${w.purpose || '（目的）'}（${w.from} · ${w.profile}）`]); }
  });
  sw.forEach((w, k) => { if (!cw.has(k)) out.push(['d', `ワーカー ${w.purpose || '（目的）'}`]); });
  t.turns.forEach(([a, b]) => { if (!hasTurn(s, a, b)) out.push(['a', `ターン ${nodeName(a)} → ${nodeName(b)}`]); });
  s.turns.forEach(([a, b]) => { if (!hasTurn(t, a, b)) out.push(['d', `ターン ${nodeName(a)} → ${nodeName(b)}`]); });
  return out;
}
function turnRows(t, s, id, dir) {
  const list = dir === 'out' ? t.turns.filter(([a]) => a === id).map(([, b]) => [id, b]) : t.turns.filter(([, b]) => b === id).map(([a]) => [a, id]);
  if (!list.length) return html`<span class="cv-empty">なし</span>`;
  return list.map(([a, b]) => html`<div class=${'cv-li' + (hasTurn(s, a, b) ? '' : ' new')}><span>${dir === 'out' ? '→' : '←'} ${nodeName(dir === 'out' ? b : a)}</span><button class="cv-x" type="button" data-cv-act="turn-del" data-a=${a} data-b=${b} aria-label=${`${nodeName(a)} から ${nodeName(b)} へのターンを外す`} title="外す">×</button></div>`);
}
function addTurnSel(t, id) {
  const cand = ['human', ...t.members.map((m) => m.name)].filter((x) => x !== id && !hasTurn(t, id, x));
  if (!cand.length) return '';
  return html`<span class="sel"><select data-cv-f="turn-add"><option value="">＋ 渡す相手を足す</option>${cand.map((c) => html`<option value=${c}>${nodeName(c)}</option>`)}</select></span>`;
}
function fieldFindings(r, id) {
  return (teamIssues(r) || []).filter((m) => m.node === id && (m.field === 'members' || m.field === 'name' || m.field === 'node'));
}
function memberMsgs(r, t, m) {
  const msgs = [];
  if (!NAME.test(m.name)) msgs.push('英小文字で始め、英小文字・数字・ハイフン・下線で 32 文字以内にしてください');
  if (t.members.filter((x) => x.name === m.name).length > 1) msgs.push('このチームにもういます');
  if (`${r.name.value}-${m.name}`.length > 32) msgs.push(`herdr 名 ${r.name.value}-${m.name} が 32 文字を超えます`);
  return msgs;
}

function SideAttr({ r }) {
  const t = r.draft.value, s = r.saved.value, sel = selOf(r), team = r.name.value;
  if (!sel) {
    const d = diffList(r), humanIn = t.turns.some(([, b]) => b === 'human');
    return html`<div class="cv-t"><code>${team}</code><span class="cv-k">チーム定義</span></div>
        <div class="cv-f"><label for=${`cv-mi-${team}`}>使命</label><input id=${`cv-mi-${team}`} class="cv-body" data-cv-f="mission" value=${t.mission} autocomplete="off" /></div>
        <div class="cv-note">メンバー ${t.members.length} · ワーカー ${t.workers.length} · ターン ${t.turns.length}</div>
        ${humanIn ? '' : html`<div class="cv-warn">人間に届くターンがありません。人間の点の縁の丸から、または人間へ渡すメンバーの縁の丸から、線を引いてください</div>`}
        <div class="cv-sec"><span class="cv-sl">保存していない変更 ${d.length}</span>${d.length ? html`<div class="cv-diff">${d.map(([k, x]) => html`<span class=${'cv-' + k}>${k === 'a' ? '+ ' : k === 'd' ? '− ' : '~ '}${x}</span>`)}</div>` : html`<span class="cv-empty">なし</span>`}</div>
        <div class="cv-sec"><span class="cv-sl">操作</span><span class="cv-note">点や線を押すと、ここにその属性が出る。選んだ点の縁の丸から、ほかの点へ引くとターンができる。<span class="cv-kbd">Delete</span> で外す、<span class="cv-kbd">Esc</span> で選ぶのをやめる、<span class="cv-kbd">⌘Z</span> で 1 つ取り消す。</span></div>`;
  }
  if (sel.type === 'edge') {
    const [a, b] = sel.k.split('|');
    const row = (p, q) => (hasTurn(t, p, q)
      ? html`<div class=${'cv-li' + (hasTurn(s, p, q) ? '' : ' new')}><span>${nodeName(p)} → ${nodeName(q)}</span><button class="cv-x" type="button" data-cv-act="turn-del" data-a=${p} data-b=${q} title="外す" aria-label="外す">×</button></div>`
      : html`<div class="cv-li"><span class="cv-empty">${nodeName(p)} → ${nodeName(q)}</span><button class="btn sm" type="button" data-cv-act="turn-add" data-a=${p} data-b=${q}>この向きも足す</button></div>`);
    return html`<div class="cv-t"><code>${`${nodeName(a)} ⇄ ${nodeName(b)}`}</code><span class="cv-k">ターン</span></div><div class="cv-sec"><span class="cv-sl">向き</span>${row(a, b)}${row(b, a)}</div>
        <div class="cv-note"><span class="cv-kbd">Delete</span> で両方の向きを外す。</div>`;
  }
  if (sel.type === 'worker') {
    const w = t.workers[sel.i];
    if (!w) return '';
    const profs = (profilesFor(r).includes(w.profile) ? [] : [w.profile]).concat(profilesFor(r));
    return html`<div class="cv-t"><code>${w.purpose || '（目的）'}</code><span class="cv-k">ワーカー</span></div>
        <div class="cv-f"><label for=${`cv-wp-${team}`}>目的</label><input id=${`cv-wp-${team}`} data-cv-f="w-purpose" value=${w.purpose} placeholder="例: 調べ物" autocomplete="off" spellcheck="false" /></div>
        <div class="cv-f"><label for=${`cv-wf-${team}`}>追加するメンバー</label><span class="sel"><select id=${`cv-wf-${team}`} data-cv-f="w-from">${t.members.map((m) => html`<${Opt} v=${m.name} on=${m.name === w.from}>${m.name}</${Opt}>`)}</select></span></div>
        <div class="cv-f"><label for=${`cv-wpf-${team}`}>プロファイル</label><span class="sel"><select id=${`cv-wpf-${team}`} data-cv-f="w-profile">${profs.map((p) => html`<${Opt} v=${p} on=${p === w.profile}>${p}</${Opt}>`)}</select></span><span class="cv-sub">ワーカーは規則を受けない。安いプロファイルで</span></div>
        <${WorkerPrompt} r=${r} w=${w} i=${sel.i} />
        <button class="btn danger sm cv-danger" type="button" data-cv-act="del">ワーカーを外す</button>`;
  }
  const id = sel.id;
  if (id === 'human') {
    return html`<div class="cv-t"><code>人間</code><span class="cv-k">あなた。チームに 1 人だけで、外せない</span></div>
        <div class="cv-sec"><span class="cv-sl">ターンを渡す相手</span>${turnRows(t, s, id, 'out')}${addTurnSel(t, id)}</div>
        <div class="cv-sec"><span class="cv-sl">ターンを受け取る相手</span>${turnRows(t, s, id, 'in')}</div>`;
  }
  const m = memberOf(t, id);
  if (!m) return '';
  // The field holds what a person is typing: a name the server would refuse is
  // not put into the draft, so the part keeps the typed letters itself and draws
  // them with the words beside them (5 章 2).
  const typed = r.typed.value && r.typed.value.id === id ? r.typed.value : null;
  const msgs = (typed ? typed.msgs : memberMsgs(r, t, m)).concat(fieldFindings(r, id).map((x) => x.text));
  const outs = t.turns.filter(([a]) => a === id).map(([, b]) => b);
  const ws = t.workers.map((w, i) => ({ w, i })).filter((z) => z.w.from === id);
  const nt = t.turns.filter(([a, b]) => a === id || b === id).length;
  const role = (rolesFor(r) || []).find((x) => x.name === m.role) || {};
  return html`<div class="cv-t"><code>${m.name}</code><span class="cv-k">メンバー${role.title ? ` · ${role.title}` : ''}</span></div>
      <div class="cv-f"><label for=${`cv-mn-${team}`}>メンバー名</label><input id=${`cv-mn-${team}`} data-cv-f="m-name" value=${typed ? typed.value : m.name} spellcheck="false" autocomplete="off" />${msgs.map((x) => html`<span class="cv-msg">${x}</span>`)}<span class="cv-sub">herdr 名 ${`${team}-${m.name}`}</span></div>
      <div class="cv-f"><label for=${`cv-mr-${team}`}>ロール</label><span class="sel"><select id=${`cv-mr-${team}`} data-cv-f="m-role">${(rolesFor(r) || []).map((x) => html`<${Opt} v=${x.name} on=${x.name === m.role}>${x.name}${x.title ? ` · ${x.title}` : ''}</${Opt}>`)}</select></span><span class="cv-sub">無いロールは <a href="/roles">ロールの画面</a> で先に作る</span></div>
      <div class="cv-f"><label for=${`cv-mp-${team}`}>プロファイル</label><span class="sel"><select id=${`cv-mp-${team}`} data-cv-f="m-profile">${profilesFor(r).map((p) => html`<${Opt} v=${p} on=${p === m.profile}>${p}</${Opt}>`)}</select></span></div>
      <div class="cv-sec"><span class="cv-sl">ターンを渡す相手</span>${turnRows(t, s, id, 'out')}${outs.length ? '' : html`<span class="cv-note">まだ誰にもターンを渡していません。縁の丸から、渡す相手の点へ引いてください</span>`}${addTurnSel(t, id)}</div>
      <div class="cv-sec"><span class="cv-sl">ターンを受け取る相手</span>${turnRows(t, s, id, 'in')}</div>
      <div class="cv-sec"><span class="cv-sl">ワーカー</span>${ws.length ? ws.map(({ w, i }) => html`<div class=${'cv-li' + (s.workers.some((x) => x.from === w.from && x.purpose === w.purpose) ? '' : ' new')}><span>${`${w.purpose || '（目的）'} · ${w.profile}`}</span><span class="cv-liend"><button class="cv-link" type="button" data-cv-act="pick-w" data-i=${i}>選ぶ</button><button class="cv-x" type="button" data-cv-act="w-del" data-i=${i} title="外す" aria-label="外す">×</button></span></div>`) : html`<span class="cv-empty">なし</span>`}
        <button class="btn sm cv-danger" type="button" data-cv-act="w-add">＋ ワーカー</button></div>
      <${MemberPrompt} r=${r} m=${m} />
      <div class="cv-sec"><button class="btn danger sm cv-danger" type="button" data-cv-act="del">メンバーを外す</button><span class="cv-note">ターン ${nt} 本とワーカー ${ws.length} 件も一緒に外れる。<span class="cv-kbd">⌘Z</span> で取り消せる。</span></div>`;
}

// ---- the prompt a member (or a worker) is handed ---------------------
// The lines are hergy's own, cut into the segments r-4cf9 names and where each
// came from (spec-boundary 5). The screen lays them out and says where a
// folded document is read; it builds no line (n-56f7, d-b87a).
function lineAtMaker(full) {
  const starts = [0];
  for (let i = 0; i < full.length; i++) if (full[i] === '\n') starts.push(i + 1);
  return (off) => {
    let lo = 0, hi = starts.length - 1;
    while (lo < hi) { const mid = (lo + hi + 1) >> 1; if (starts[mid] <= off) lo = mid; else hi = mid - 1; }
    return lo;
  };
}
function promptBlocks(segments) {
  const full = segments.map((x) => x.text).join('');
  const lines = full.split('\n');
  const at = lineAtMaker(full);
  const out = [];
  let off = 0;
  segments.forEach((seg) => {
    const len = seg.text.length;
    if (len) {
      let a = at(off), b = at(off + len - 1) + 1;
      // The blank lines a segment starts with belong to the block before it:
      // a folded document starts at its first line, not at the gap above it.
      const lead = (seg.text.match(/^\n+/) || [''])[0].length;
      if (lead && out.length && a === out[out.length - 1].b && at(off + lead - 1) + 1 > a) {
        out[out.length - 1].b = at(off + lead - 1) + 1;
        a = out[out.length - 1].b;
      }
      if (out.length && a < out[out.length - 1].b) a = out[out.length - 1].b;
      if (a < b) out.push({ seg, a, b });
    }
    off += len;
  });
  // The blank lines between two segments belong to the block before them:
  // every line of the prompt is held by one block, so a fold's range covers
  // them and nothing is left between two blocks.
  for (let i = 1; i < out.length; i++) if (out[i].a > out[i - 1].b) out[i - 1].b = out[i].a;
  if (out.length) out[out.length - 1].b = Math.max(out[out.length - 1].b, lines.length);
  return { lines, blocks: out };
}
// pline marks the heading marks of a prompt line (## 名簿).
function PLine({ text }) {
  const m = String(text).match(/^(#{1,6} )(.*)$/);
  if (m) return html`<span class="cv-mm">${m[1]}</span><span class="cv-ph">${m[2]}</span>`;
  return text;
}
function PLines({ lines, a, b, slot }) {
  return lines.slice(a, b).map((t, k) => html`<div class="yl" data-ln=${a + k}><span class="lno">${a + k + 1}</span><span class="yt">${slot ? slot(t) : html`<${PLine} text=${t} />`}</span></div>`);
}
const prange = (a, b) => (b - a > 1 ? `${a + 1}–${b}` : `${a + 1}`);
const pcount = (lines, a, b) => lines.slice(a, b).filter((x) => x !== '').length;
function TemplateChip({ seg }) {
  const o = seg.origin || {};
  if (o.kind === 'team') return html`<span class="cv-ptpl own">ひな形: チームの prompt:</span><button class="cv-go" type="button" data-cv-act="to-yaml">YAML で直す</button>`;
  if (o.kind === 'member') return html`<span class="cv-ptpl own">ひな形: メンバー ${o.name || ''} の prompt:</span><button class="cv-go" type="button" data-cv-act="to-yaml">YAML で直す</button>`;
  return html`<span class="cv-ptpl">ひな形: hergy の既定</span>`;
}
const PLABEL = { roster: '名簿', turns: 'ターンを渡す相手', workers: 'ワーカーの追加' };
// d-20ac: a definition with a problem is not given a prompt, by hergy up or by
// this column. The words are the screen's own; the problem itself is the
// server's, under the canvas.
const PROMPT_BLOCKED = 'このチーム定義には問題があるので、hergy はプロンプトを組みません（hergy up も止まります）。下の validate の欄の問題を直すと、ここに出ます。';
function MemberPrompt({ r, m }) {
  const hit = (promptsOf(r) || []).find((x) => x.name === m.name);
  let body;
  if (promptBlockedOf(r)) body = html`<span class="cv-note">${PROMPT_BLOCKED}</span>`;
  else if (!hit) body = html`<span class="cv-note">プロンプトを読んでいます。</span>`;
  else {
    const { lines, blocks } = promptBlocks(hit.segments || []);
    let head = false, labelled = '';
    body = blocks.map(({ seg, a, b }) => {
      const o = seg.origin || {};
      if (seg.kind === 'role_doc') {
        return html`<div class="cv-fold" data-a=${a} data-b=${b} data-doc="role" data-role=${o.name || ''} data-place=${o.place || ''}><span class="cv-range">${prange(a, b)}</span><b>ロールの文書</b><code>${o.path || ''}</code><span>${pcount(lines, a, b)} 行</span><a href=${`/roles?open=${encodeURIComponent(o.name || '')}`}>ロールの画面で見る</a></div>`;
      }
      if (seg.kind === 'team_rules') {
        return html`<div class="cv-fold" data-a=${a} data-b=${b} data-doc="team"><span class="cv-range">${prange(a, b)}</span><b>チームのルール</b><span>${pcount(lines, a, b)} 行</span><button class="cv-go" type="button" data-cv-act="to-agree">ルールのタブで見る</button></div>`;
      }
      if (seg.kind === 'herdr') {
        return html`<div class="cv-fold cv-fixed" data-a=${a} data-b=${b}><span class="cv-range">${prange(a, b)}</span><details><summary>連絡のしかた<span> · hergy の決まった文 · どのチームでも同じ</span></summary><${PLines} lines=${lines} a=${a} b=${b} /></details></div>`;
      }
      let label = '';
      // The heading of a section ("## 名簿") stands in the template just
      // before the section's own segment: the label goes above the heading,
      // and the segment that follows does not say it a second time.
      const heading = (lines.slice(a, b).find((x) => /^##\s/.test(x)) || '').replace(/^##\s*/, '');
      if (seg.kind === 'template' || seg.kind === 'actor') {
        if (!head) { head = true; label = html`<div class="cv-pbl"><b>冒頭</b><${TemplateChip} seg=${seg} /></div>`; }
        const named = Object.values(PLABEL).find((x) => x === heading);
        if (named) { label = html`${label}<div class="cv-pbl"><b>${named}</b></div>`; labelled = named; }
      } else if (PLABEL[seg.kind]) {
        label = labelled === PLABEL[seg.kind] ? '' : html`<div class="cv-pbl"><b>${PLABEL[seg.kind]}</b></div>`;
        labelled = PLABEL[seg.kind];
      }
      return html`${label}<${PLines} lines=${lines} a=${a} b=${b} />`;
    });
  }
  return html`<details class="cv-sec cv-prompt"><summary class="cv-sl">プロンプト（このチーム定義から渡る行）</summary><div class="cv-pr">${body}</div></details>`;
}
function WorkerPrompt({ r, w, i }) {
  if (promptBlockedOf(r)) return html`<details class="cv-sec cv-prompt"><summary class="cv-sl">プロンプト（hire のときに渡る文）</summary><span class="cv-note">${PROMPT_BLOCKED}</span></details>`;
  if (!w.purpose) return html`<details class="cv-sec cv-prompt"><summary class="cv-sl">プロンプト（hire のときに渡る文）</summary><span class="cv-note">目的を書くと、hire のときに渡る文が出ます。</span></details>`;
  const pv = workerPrompt(r, i);
  let body;
  if (!pv) body = html`<span class="cv-note">プロンプトを読んでいます。</span>`;
  else if (pv.err) body = html`<span class="cv-note">${pv.err}</span>`;
  else {
    const { lines, blocks } = promptBlocks(pv.segments || []);
    const full = `${r.name.value}-${pv.name}`;
    // A worker's name and task are chosen at hire, so the lines show the slots
    // they stand in (<ワーカー名> / <依頼>) instead.
    const slotText = (t) => {
      const out = [];
      let s = String(t);
      for (;;) {
        const a = s.indexOf(full), b = s.indexOf(pv.task);
        const at = a < 0 ? b : b < 0 ? a : Math.min(a, b);
        if (at < 0) { out.push(s); return out; }
        out.push(s.slice(0, at));
        if (at === a) {
          out.push(`${r.name.value}-`);
          out.push(html`<span class="cv-slot" data-v=${pv.name}>${'<ワーカー名>'}</span>`);
          s = s.slice(at + full.length);
        } else {
          out.push(html`<span class="cv-slot" data-v=${pv.task}>${'<依頼>'}</span>`);
          s = s.slice(at + pv.task.length);
        }
      }
    };
    const agreeAt = blocks.findIndex(({ seg, a, b }) => seg.kind === 'template' && lines.slice(a, b).some((x) => x.startsWith('## 依頼')));
    let herdrSeen = false;
    body = blocks.map(({ seg, a, b }, n) => {
      if (seg.kind === 'herdr') {
        herdrSeen = true;
        return html`<div class="cv-fold cv-fixed" data-a=${a} data-b=${b}><span class="cv-range">${prange(a, b)}</span><details><summary>連絡のしかた<span> · hergy の決まった文 · どのワーカーでも同じ</span></summary><${PLines} lines=${lines} a=${a} b=${b} slot=${slotText} /></details></div>`;
      }
      let label = '';
      if (n === 0) label = html`<div class="cv-pbl"><b>冒頭</b><span class="cv-ptpl">ひな形: hergy の既定</span></div>`;
      else if (n === agreeAt) label = html`<div class="cv-pbl"><b>依頼</b><span>hire の --task</span></div>`;
      else if (herdrSeen) label = html`<div class="cv-pbl"><b>報告</b><span>追加したメンバーへ</span></div>`;
      return html`${label}<${PLines} lines=${lines} a=${a} b=${b} slot=${slotText} />`;
    });
  }
  return html`<details class="cv-sec cv-prompt"><summary class="cv-sl">プロンプト（hire のときに渡る文）</summary><div class="cv-pr">${body}</div></details>`;
}


// ---- the team.yaml a save would write (this tab's preview) -------------
// TPrev is the トポロジー tab's team.yaml: the text a save would write, line by
// line, with the lines one edit changed marked. The lines, what each stands
// for and the marks all come from the server (spec-teams B7, spec-boundary 5).
function TPrev({ r }) {
  const key = dkey(r);
  const pv = r.preview.value || { lines: [], diff: {} }, at = pv.lines || [], df = pv.diff || {};
  const added = df.added || [], removed = df.removedBefore || [], lastDel = !!df.removedAtEnd;
  const lines = at.map((z) => z.text);
  const prev = GLOW.last[key];
  // The first answer for a row is not a move: the text was not drawn before,
  // so the row's whole file is not marked as changed (監査 R1).
  if (prev && prev.length && prev.join('\n') !== lines.join('\n')) { GLOW.step[key] = [...stepOf(prev, lines)]; GLOW.glow[key] = Date.now(); }
  GLOW.last[key] = lines;
  const step = new Set(GLOW.step[key] || []);
  const marks = lineMarks(r), cur = selOf(r), n = at.length;
  const nadd = added.length, ndel = removed.length + (lastDel ? 1 : 0);
  const issues = teamIssues(r);
  return html`<aside class="yprev tprev" aria-label="team.yaml"><div class="yph"><span class="pkl">YAML</span><span class="yinfo"><span class="vt">${n} 行</span>${nadd || ndel ? html`<span class="dirty">+${nadd} −${ndel}</span>` : ''}<span class="yr">${issues.length ? html`<span class="vcount"><${Counts} list=${issues} /></span>` : html`<span class="vclean">✓ 問題なし</span>`}</span></span></div><div class="ybox">${at.map((z, i) => {
    const p = atParts(z.at);
    const go = !!p.node || p.worker != null || !!p.a || p.f === 'mission';
    const cls = [marks[z.at] === 'err' ? 'err' : marks[z.at] ? 'warn' : '', added.includes(i) ? 'ins' : '', removed.includes(i) ? 'del' : '', lastDel && i === n - 1 ? 'dele' : '', isCurLine(z, cur) ? 'cur' : '', step.has(i) ? 'step' : '', go ? 'go' : ''].filter(Boolean).join(' ');
    return html`<div class=${'yl ' + cls} data-act=${go ? 'tyline' : null} data-node=${p.node || null} data-worker=${p.worker != null ? p.worker : null} data-a=${p.a || null} data-b=${p.b || null} data-f=${p.f || null} onClick=${go ? () => pressLine(r, p) : null}><span class="lno">${i + 1}</span><span class="yt"><${YamlLine} text=${z.text} /></span></div>`;
  })}</div></aside>`;
}

// yamlHL is the YAML a preview line shows, with the keys and the arrows marked
// (モック 版 7 から逐語).
function YamlLine({ text }) {
  let m = text.match(/^(\s*)(- )?([A-Za-z_]+):( .*)?$/);
  if (m) return html`${m[1]}${m[2] ? html`<span class="ar">- </span>` : ''}<span class="k">${m[3]}:</span>${m[4] || ''}`;
  m = text.match(/^(\s*- )(\S+) -> (\S+)$/);
  if (m) return html`<span class="ar">${m[1]}</span>${m[2]}<span class="ar"> -> </span>${m[3]}`;
  return text;
}

// atParts is what one line of the preview stands for, as the data attributes a
// press on the line carries back to the figure.
const atParts = (at) => {
  if (!at) return {};
  if (at.startsWith('node:')) return { node: at.slice(5) };
  if (at.startsWith('worker:')) return { worker: Number(at.slice(7)) };
  if (at.startsWith('turn:')) { const [a, b] = at.slice(5).split('->'); return { a, b }; }
  if (at === 'mission') return { f: 'mission' };
  return {};
};
// isCurLine is whether a preview line is one the figure has picked: a point
// takes its own lines and every turn that comes in or leaves it, a line takes
// both directions of its turn, and a worker its own lines (spec-teams B7).
function isCurLine(z, cur) {
  if (!cur || !z || !z.at) return false;
  const p = atParts(z.at);
  if (cur.type === 'node') return p.node === cur.id || p.a === cur.id || p.b === cur.id;
  if (cur.type === 'edge') return !!p.a && pair(p.a, p.b) === cur.k;
  if (cur.type === 'worker') return p.worker === cur.i;
  return false;
}
// stepOf marks the lines one edit just moved, against the text drawn a moment
// ago: it is only for the 1.8s glow, and the saved text's own diff — the `+N
// −M` the head shows — is the server's (spec-teams B7, spec-boundary 5).
function stepOf(prev, now) {
  const out = new Set();
  if (!prev) return out;
  let i = 0;
  while (i < prev.length && i < now.length && prev[i] === now[i]) i++;
  let j = 0;
  while (j < prev.length - i && j < now.length - i && prev[prev.length - 1 - j] === now[now.length - 1 - j]) j++;
  for (let k = i; k < now.length - j; k++) out.add(k);
  return out;
}
// lineMarks is the severest finding each line stands for, so the line a
// problem is about is marked the way the figure's point is (spec-teams B7).
function lineMarks(r) {
  const out = {};
  for (const m of teamIssues(r)) {
    const at = m.field === 'mission' ? 'mission' : m.node ? 'node:' + m.node : m.worker != null && m.worker >= 0 ? 'worker:' + m.worker : '';
    if (at && out[at] !== 'err') out[at] = m.sev;
  }
  return out;
}

// fitYbox caps the topology tab's YAML box at the room between the box's top
// and the save's band, which stands at the bottom of the window while the
// panel is taller than the window (spec-teams B7, 監査 R2). Without it the box
// runs on under the band, and the line a person just moved stands where the
// band hides it.
function fitYbox(panel) {
  const box = panel.querySelector('.tprev');
  if (!box) return;
  if (getComputedStyle(box).display === 'none') { box.style.maxHeight = ''; return; }
  const foot = panel.querySelector('.pf'), fh = foot ? foot.offsetHeight : 0;
  const h = Math.max(160, Math.floor(window.innerHeight - fh - box.getBoundingClientRect().top - 8));
  if (box.style.maxHeight !== h + 'px') box.style.maxHeight = h + 'px';
}
export function fitYboxes() {
  Array.from(document.querySelectorAll('.panel.team-p')).forEach(fitYbox);
}

// afterTPrev brings the lines a person is looking at into the box (spec-teams
// B7): the lines one edit just moved after an edit, and the lines the figure
// has picked after a pick. It moves the box's own scroll and nothing else, so
// opening a row leaves the page where it stands (監査 R1).
export function afterTPrev(r) {
  const key = dkey(r);
  const panel = document.querySelector(`.panel.team-p[data-key="${CSS.escape(key)}"]`);
  if (!panel) return;
  const box = panel.querySelector('.ybox');
  if (!box || !box.clientHeight) return;
  const at = (r.preview.value || { lines: [] }).lines || [];
  const step = GLOW.step[key] || [];
  const df = (r.preview.value || {}).diff || {};
  // The lines a change moved are the glow's own step, and also the lines the
  // server's diff names — the ones it added, and the place it took one from
  // (spec-teams B7). A step the glow marks alone can miss a removal the added
  // line's shift hides (監査 R2), so both are read, and the last of them is
  // where the box is sent.
  let want = -1;
  [...step, ...(df.added || []), ...(df.removedBefore || []), ...(df.removedAtEnd ? [at.length - 1] : [])]
    .forEach((i) => { if (i > want && i < at.length) want = i; });
  if (want < 0) {
    const cur = selOf(r);
    if (cur) { for (let i = 0; i < at.length; i++) if (isCurLine(at[i], cur)) { want = i; break; } }
  }
  if (want >= 0 && box.children[want]) {
    const br = box.getBoundingClientRect(), lr = box.children[want].getBoundingClientRect();
    if (lr.top < br.top) box.scrollTop -= br.top - lr.top;
    else if (lr.bottom > br.bottom) box.scrollTop += lr.bottom - br.bottom;
  }
  if (GLOW.glow[key]) {
    const glowed = GLOW.glow[key];
    clearTimeout(GLOW.timer[key]);
    GLOW.timer[key] = setTimeout(() => {
      if (Date.now() - glowed < 1700) return;
      Array.from(box.querySelectorAll('.yl.step')).forEach((el) => el.classList.remove('step'));
    }, 1800);
  }
}

// ---- the blank space's menu ------------------------------------------
// A double press on the canvas opens the two ways of adding at that place: a
// member, or a worker under one member (the mock of 2026-10-23).
function Menu({ r }) {
  const box = useRef(null);
  const m = r.menu.value;
  useLayoutEffect(() => {
    const el = box.current;
    if (!el || !m) return;
    const cv = el.closest('.cv-canvas');
    if (!cv) return;
    const cr = cv.getBoundingClientRect();
    const w = el.offsetWidth, h = el.offsetHeight;
    el.style.left = `${Math.max(6, Math.min(m.x, cr.width - w - 6))}px`;
    el.style.top = `${Math.max(6, Math.min(m.y, cr.height - h - 6))}px`;
    const first = el.querySelector('button');
    if (first) first.focus();
  });
  if (!m) return null;
  const t = r.draft.value;
  return html`<div class="cv-menu" role="menu" aria-label=${m.workersOnly ? 'ワーカーを足す' : 'ここに足す'} ref=${box} style=${{ left: `${m.x}px`, top: `${m.y}px` }}>
    ${m.workersOnly ? '' : html`<button type="button" role="menuitem" data-cv-act="menu-m" disabled=${!rolesIn()}>メンバーを足す</button>`}
    <div class=${'cv-mh' + (m.workersOnly ? ' cv-mh0' : '')}>ワーカーを足す（どのメンバーの？）</div>
    <div class="cv-mw">${t.members.map((x) => html`<button type="button" role="menuitem" data-cv-act="menu-w" data-from=${x.name}>${x.name}</button>`)}</div></div>`;
}

// ---- the canvas itself ------------------------------------------------
export function Canvas({ r, vlist }) {
  const box = useRef(null);
  useLayoutEffect(() => {
    const el = box.current;
    if (el) fitYbox(el);
    afterTPrev(r);
  });
  const sel = selOf(r);
  const cur = sel, side = sideOf(r);
  const picked = cur && cur.type === 'node' && cur.id !== 'human' ? cur.id : '';
  const hint = picked ? `${picked} を選んでいる。縁の丸から引くとターン、Delete で外す` : '点や線を押すと属性が出る。余白をダブルクリックすると、そこに足せる';
  return html`<div class="cv-main" ref=${box} onPointerDown=${(e) => onPointerDown(r, e)} onClick=${(e) => onClick(r, e)} onInput=${(e) => onInput(r, e)} onChange=${(e) => onInput(r, e)}>
      <div class="cv-left">
        <div class="cv-tools"><button class="btn sm" type="button" data-cv-act="m-add" disabled=${!rolesIn()}>＋ メンバー</button><button class="btn sm" type="button" data-cv-act="w-add">＋ ワーカー</button><span class="cv-hint">${hint}</span></div>
        <div class="cv-canvas"><${Figure} r=${r} /><${Menu} r=${r} /></div>
        <div class="vlist">${vlist}</div>
      </div>
      <aside class="cv-side" aria-label="選んだものの属性"><div class="cv-sh"><button type="button" class=${side === 'attr' ? 'on' : ''} data-cv-side="attr">属性</button><button type="button" class=${side === 'yaml' ? 'on' : ''} data-cv-side="yaml">YAML</button></div>
        <div class="cv-sb">${side === 'attr' ? html`<${SideAttr} r=${r} />` : html`<${TPrev} r=${r} />`}</div></aside></div>`;
}

// ---- 操作 ------------------------------------------------------------
// endField takes the focus away from the field the person was typing in: a press
// on the figure is a press on a point, and the Delete that follows is meant for
// the point.
function endField() {
  const act = document.activeElement;
  if (act && act.matches && act.matches('input, select, textarea') && act.closest('.panel.team-p')) act.blur();
}

function menuClose(r) {
  if (!r || !r.menu.value) return;
  r.menu.value = null;
}
// openMenu opens the blank space's two ways of adding at one place (the mock of
// 2026-10-23).
function openMenu(r, at, e, workersOnly) {
  const panelEl = document.querySelector(`.panel.team-p[data-key="${CSS.escape(dkey(r))}"]`);
  const canvas = panelEl && panelEl.querySelector('.cv-canvas');
  if (!canvas) return;
  const cr = canvas.getBoundingClientRect();
  const x = e ? e.clientX - cr.left : 6, y = e ? e.clientY - cr.top : 6;
  r.menu.value = { x, y, ax: at ? at[0] : null, ay: at ? at[1] : null, workersOnly: !!workersOnly };
}
const rowOf = (el) => {
  const row = el && el.closest && el.closest('.tdef');
  return row ? byKey(row.dataset.key) : null;
};
const svgPt = (svg, e) => { const p = svg.createSVGPoint(); p.x = e.clientX; p.y = e.clientY; return p.matrixTransform(svg.getScreenCTM().inverse()); };
function firstRole(r) { const rs = rolesFor(r) || []; return (rs[0] || {}).name || ''; }
function firstProfile(cheap, r) {
  const ps = profilesFor(r);
  return (cheap ? ps.find((p) => /haiku|mini|flash/.test(p)) : '') || ps[0] || '';
}
function addMember(r, at) {
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
function addWorker(r, from, at) {
  const t = r.draft.value, idx = t.workers.length, profile = firstProfile(true, r);
  const ops = [{ kind: 'addWorker', from, purpose: '', profile }];
  if (at) putPlace(r, 'w' + idx, at);
  selectPick(r, { type: 'worker', i: idx }, 'attr');
  canvasAct(r, ops, { message: `${from} のワーカーを足した。目的を右で決める`, undo: 'ワーカーを足したのを取り消した', focus: '[data-cv-f="w-purpose"]' });
}
function del(r) {
  const t = r.draft.value, sel = selOf(r);
  if (!sel) return;
  if (sel.type === 'node' && sel.id !== 'human') {
    const id = sel.id, nt = t.turns.filter(([a, b]) => a === id || b === id).length, nw = t.workers.filter((w) => w.from === id).length;
    selectPick(r, null);
    canvasAct(r, [{ kind: 'removeMember', member: id }], { message: `${id} を外した（ターン ${nt} 本・ワーカー ${nw} 件も）`, undo: `${id} を外したのを取り消した` });
  } else if (sel.type === 'edge') {
    const [a, b] = sel.k.split('|'), ops = [];
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

// lastPress and DRAG are the part's own short-lived memory: the double press on
// the blank space, and the pointer's drag while it is out.
let lastPress = null;
let DRAG = null;
function onPointerDown(r, e) {
  const svg = e.target.closest && e.target.closest('.cv-canvas svg');
  if (!svg) return;
  // A press on the figure ends what the field held: the person has moved on to
  // a point, and the next Delete is meant for the point, not for the text.
  endField();
  if (e.target.closest('[data-cv-act]')) return;
  const h = e.target.closest('[data-cv-h]');
  if (h) { DRAG = { what: 'conn', r, from: h.dataset.cvH, svg, start: svgPt(svg, e) }; svg.setPointerCapture(e.pointerId); e.preventDefault(); return; }
  const n = e.target.closest('[data-cv-n], [data-cv-w]');
  if (n) {
    const id = n.dataset.cvN || ('w' + n.dataset.cvW);
    DRAG = { what: 'move', r, id, wi: n.dataset.cvW != null ? Number(n.dataset.cvW) : null, svg, start: svgPt(svg, e), orig: [...layoutOf(r)[id]], moved: false };
    svg.setPointerCapture(e.pointerId); return;
  }
  const ed = e.target.closest('[data-cv-e]');
  if (ed) { selectPick(r, { type: 'edge', k: ed.dataset.cvE }, 'attr'); return; }
  const now = Date.now(), pt = svgPt(svg, e);
  if (lastPress && now - lastPress.t < 400 && Math.hypot(pt.x - lastPress.x, pt.y - lastPress.y) < 8 && lastPress.r === r) {
    lastPress = null; openMenu(r, [pt.x, pt.y], e); return;
  }
  lastPress = { t: now, x: pt.x, y: pt.y, r };
  menuClose(r);
  if (selOf(r)) selectPick(r, null);
}
function onPointerMove(e) {
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
    const at = [d.orig[0] + dx, d.orig[1] + dy];
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
  const over = document.elementFromPoint(e.clientX, e.clientY), g = over && over.closest('[data-cv-n]');
  const to = g && g.dataset.cvN !== d.from ? g.dataset.cvN : null;
  if (to) g.classList.add('drop');
  label.removeAttribute('hidden');
  // The words stand to the right of the point's own box and below it, so they
  // never cover the name the person is aiming at (監査 1-6).
  if (to) {
    const c = layoutOf(r)[to], b = boxOf(to, t);
    label.setAttribute('x', c[0] + b.w / 2 + 12); label.setAttribute('y', c[1] + b.h / 2 + 18);
  } else {
    label.setAttribute('x', p.x + 24); label.setAttribute('y', p.y + 34);
  }
  label.textContent = to ? (hasTurn(t, d.from, to) ? `${nodeName(d.from)} → ${nodeName(to)} はもうある` : `${nodeName(d.from)} → ${nodeName(to)} に渡す`) : `${nodeName(d.from)} から、渡す相手の点まで引く`;
}
function onPointerUp(e) {
  const d = DRAG; if (!d) return;
  DRAG = null;
  const r = d.r;
  if (d.what === 'move') {
    if (d.moved) { selectPick(r, null); return; }
    selectPick(r, d.wi != null ? { type: 'worker', i: d.wi } : { type: 'node', id: d.id }, 'attr');
    return;
  }
  const over = document.elementFromPoint(e.clientX, e.clientY), g = over && over.closest('[data-cv-n]');
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
function onClick(r, e) {
  // A press made without a pointer (the keyboard's Enter, or a click a walk
  // sends) picks the point or line too: the pointer path is not the only one.
  if (e.target.closest('.cv-canvas')) endField();
  const hit = e.target.closest('[data-cv-n], [data-cv-w], [data-cv-e]');
  if (hit) {
    const sel = hit.dataset.cvN ? { type: 'node', id: hit.dataset.cvN }
      : hit.dataset.cvW != null ? { type: 'worker', i: Number(hit.dataset.cvW) }
        : { type: 'edge', k: hit.dataset.cvE };
    if (JSON.stringify(selOf(r)) === JSON.stringify(sel)) return;
    selectPick(r, sel, 'attr');
    return;
  }
  const b = e.target.closest('[data-cv-act], [data-cv-side]');
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
    const menu = r.menu.value;
    const at = menu && menu.ax != null ? [menu.ax, menu.ay] : null;
    r.menu.value = null;
    if (a === 'menu-m') addMember(r, at);
    else addWorker(r, b.dataset.from, at);
    return;
  }
  if (a === 'to-yaml') { r.side.value = 'yaml'; return; }
  if (a === 'to-agree') { openAgree(r); return; }
  if (a === 'del') { del(r); return; }
  if (a === 'edge-del') {
    const [p, q] = (b.dataset.k || '').split('|'), ops = [];
    if (hasTurn(t, p, q)) ops.push({ kind: 'removeTurn', from: p, to: q });
    if (hasTurn(t, q, p)) ops.push({ kind: 'removeTurn', from: q, to: p });
    selectPick(r, null);
    if (ops.length) canvasAct(r, ops, { message: `${nodeName(p)} ⇄ ${nodeName(q)} のターンを外した`, undo: `${nodeName(p)} ⇄ ${nodeName(q)} のターンを外したのを取り消した` });
    return;
  }
  if (a === 'turn-del') {
    canvasAct(r, [{ kind: 'removeTurn', from: b.dataset.a, to: b.dataset.b }], { message: `${nodeName(b.dataset.a)} → ${nodeName(b.dataset.b)} のターンを外した`, undo: `${nodeName(b.dataset.a)} → ${nodeName(b.dataset.b)} のターンを足したのを取り消した` });
    return;
  }
  if (a === 'turn-add') {
    canvasAct(r, [{ kind: 'addTurn', from: b.dataset.a, to: b.dataset.b }], { message: `${nodeName(b.dataset.a)} → ${nodeName(b.dataset.b)} のターンを足した`, undo: `${nodeName(b.dataset.a)} → ${nodeName(b.dataset.b)} のターンを足したのを取り消した` });
    return;
  }
  if (a === 'w-del') {
    const w = t.workers[+b.dataset.i]; if (!w) return;
    canvasAct(r, [{ kind: 'removeWorker', from: w.from, purpose: w.purpose }], { message: `ワーカー ${w.purpose || '（目的）'} を外した`, undo: 'ワーカーを外したのを取り消した' });
    return;
  }
  if (a === 'pick-w') selectPick(r, { type: 'worker', i: +b.dataset.i }, 'attr');
}
function onInput(r, e) {
  const f = e.target.dataset && e.target.dataset.cvF; if (!f) return;
  const t = r.draft.value, sel = selOf(r), v = e.target.value;
  if (f === 'turn-add') {
    if (v) { canvasAct(r, [{ kind: 'addTurn', from: sel.id, to: v }], { message: `${nodeName(sel.id)} → ${nodeName(v)} のターンを足した`, undo: `${nodeName(sel.id)} → ${nodeName(v)} のターンを足したのを取り消した` }); e.target.value = ''; }
    return;
  }
  if (e.type === 'change' && e.target.tagName !== 'SELECT') return;
  if (e.type === 'input' && e.target.tagName === 'SELECT') return;
  if (f === 'mission') { canvasTyping(r, { kind: 'mission', text: v }, 'mission'); return; }
  if (f === 'm-name') {
    const m = sel && sel.type === 'node' ? memberOf(t, sel.id) : null;
    if (!m) return;
    const ok = NAME.test(v) && v !== m.name;
    if (ok) {
      renamePlace(r, m.name, v);
      r.typed.value = null;
      canvasTyping(r, { kind: 'renameMember', member: m.name, new: v }, 'm-name');
      selectPick(r, { type: 'node', id: v }, 'attr');
      return;
    }
    // The messages stand beside the field as it is typed, so a name the server
    // would refuse is said before the field is left (監査 R9).
    r.typed.value = { id: sel.id, value: v, msgs: nameMsgs(r, t, m, v) };
    return;
  }
  if (f === 'm-role' || f === 'm-profile') {
    if (!sel || sel.type !== 'node') return;
    canvasAct(r, [{ kind: f === 'm-role' ? 'memberRole' : 'memberProfile', member: sel.id, [f === 'm-role' ? 'role' : 'profile']: v }], { message: `メンバー ${sel.id} の${f === 'm-role' ? 'ロール' : 'プロファイル'}を変えた`, undo: `メンバー ${sel.id} の${f === 'm-role' ? 'ロール' : 'プロファイル'}を変えたのを取り消した` });
    return;
  }
  if (f === 'w-purpose' || f === 'w-from' || f === 'w-profile') {
    const w = sel && sel.type === 'worker' ? t.workers[sel.i] : null; if (!w) return;
    if (f === 'w-purpose') { canvasTyping(r, { kind: 'workerPurpose', from: w.from, old: w.purpose, new: v }, 'w-purpose'); return; }
    if (f === 'w-from') { canvasAct(r, [{ kind: 'moveWorker', from: w.from, purpose: w.purpose, to: v }], { message: `ワーカー ${w.purpose || '（目的）'} を ${v} の下へ移した`, undo: `ワーカー ${w.purpose || '（目的）'} を移したのを取り消した` }); return; }
    canvasAct(r, [{ kind: 'workerProfile', from: w.from, purpose: w.purpose, profile: v }], { message: `ワーカー ${w.purpose || '（目的）'} のプロファイルを変えた`, undo: 'ワーカーのプロファイルを変えたのを取り消した' });
  }
}
function nameMsgs(r, t, m, v) {
  const msgs = [];
  if (!NAME.test(v)) msgs.push('英小文字で始め、英小文字・数字・ハイフン・下線で 32 文字以内にしてください');
  if (t.members.some((x) => x !== m && x.name === v)) msgs.push('このチームにもういます');
  if (`${r.name.value}-${v}`.length > 32) msgs.push(`herdr 名 ${r.name.value}-${v} が 32 文字を超えます`);
  return msgs;
}
// menuRow is the row whose blank space's menu stands open: Esc closes it even
// when no point is picked (監査 2-8).
const menuRow = () => recs.value.find((x) => x.menu.value) || null;
function onKey(e) {
  const inField = e.target.closest && e.target.closest('input, select, textarea');
  // The keys stand on the row a person is working in, not only on what has the
  // focus: a point is picked by the pointer, and the Delete that follows is
  // still meant for it (the mock of 2026-10-23).
  const inside = e.target.closest && e.target.closest('.panel.team-p');
  const panels = Array.from(document.querySelectorAll('.panel.team-p')).filter((el) => rowOf(el));
  const panelEl = (inside && rowOf(inside)) ? inside : panels.find((el) => selOf(rowOf(el)));
  const r = panelEl ? rowOf(panelEl) : null;
  // ⌘Z stands for the newest operation even after the pick was let go (the
  // blank space, Esc, or the line's ×): the row that has something to take
  // back is the one it acts on. The field's own ⌘Z is left to the field.
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'z' && !inField) {
    const target = r || undoRow();
    if (target) { e.preventDefault(); canvasUndo(target); }
    return;
  }
  // Esc inside a field leaves the field and keeps what was typed; the next
  // Esc lets the pick go (監査 1-7).
  if (inField) { if (e.key === 'Escape') { e.preventDefault(); e.target.blur(); } return; }
  // The blank space's menu and the toolbar's ask stand even when no point is
  // picked; Esc closes them before anything else (監査 2-8).
  const openMenuRow = menuRow();
  if (e.key === 'Escape' && openMenuRow) { e.preventDefault(); menuClose(openMenuRow); return; }
  if (!r) return;
  const sel = selOf(r);
  if (e.key === 'Delete' || e.key === 'Backspace') { if (sel) { e.preventDefault(); del(r); } return; }
  if (e.key === 'Escape') { menuClose(r); if (sel) { e.preventDefault(); selectPick(r, null); } return; }
  if ((e.key === 'Enter' || e.key === ' ') && e.target.matches && e.target.matches('.cv-n, .cv-hit, .cv-ex')) {
    e.preventDefault();
    if (e.target.matches('.cv-hit')) selectPick(r, { type: 'edge', k: e.target.dataset.cvE }, 'attr');
    else if (e.target.matches('.cv-ex')) e.target.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    else {
      const id = e.target.dataset.cvN || ('w' + e.target.dataset.cvW);
      selectPick(r, e.target.dataset.cvN ? { type: 'node', id } : { type: 'worker', i: Number(e.target.dataset.cvW) }, 'attr');
    }
  }
}

// mountCanvas puts the part's own listeners on the page once: the keys stand on
// the page (a point's Delete comes after the pointer took the focus away), and
// the pointer's move and up are the document's so a drag that leaves the figure
// still lands.
export function mountCanvas(el) {
  root = el;
  document.addEventListener('keydown', onKey);
  document.addEventListener('pointermove', onPointerMove);
  document.addEventListener('pointerup', onPointerUp);
  if (!root) return;
  root.addEventListener('focusout', (e) => {
    const f = e.target.dataset && e.target.dataset.cvF;
    if (f === 'm-name' || f === 'w-purpose' || f === 'mission') {
      const r = rowOf(e.target); if (r) canvasEndTyping(r);
    }
  });
}
