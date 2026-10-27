// topocanvas.js — the トポロジー tab as a canvas for editing the nodes of a team
// definition (n-6be7, the designer's mock of 2026-10-23). The screen draws the
// figure and the attribute column; every change is one of the screen's own ops,
// handed back through the host, so the server checks it and writes it exactly
// as team.yaml (spec-teams B7, spec-boundary 5).
//
// The part keeps only what is not in the definition: which point or line is
// picked, which of the two columns (属性 / YAML) is shown, and where a point was
// dragged. Where a point stands is not saved; opening the row again lays the
// figure out afresh (d-a575).
//
// A screen mounts once, on the element its panels stand in:
//
//   hergyCanvas.mount(root, {
//     key(row), draft(row), saved(row), findings(row), name(row), path(row),
//     roles(), profiles(),
//     act(row, ops, { message, undo, focus }),  // one operation, one undo
//     typing(row, op, field), endTyping(row), undo(row),
//     canvas(row), validate(row), yaml(row), toAgree(row),
//   });
(() => {
  'use strict';

  const $ = (s, root = document) => root.querySelector(s);
  const $$ = (s, root = document) => Array.from(root.querySelectorAll(s));
  const esc = (s) => String(s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
  const NAME = /^[a-z][a-z0-9_-]{0,31}$/;
  const clone = (x) => JSON.parse(JSON.stringify(x));
  const nodeName = (x) => (x === 'human' ? '人間' : x);
  const hasTurn = (t, a, b) => (t.turns || []).some(([p, q]) => p === a && q === b);
  const memberOf = (t, n) => (t.members || []).find((m) => m.name === n);
  const pair = (a, b) => [a, b].sort().join('|');
  const textW = (s, mono) => [...String(s)].reduce((w, c) => w + (/[\u3000-\u9fff\uff00-\uffef]/.test(c) ? 13 : mono ? 8.4 : 7.4), 0);

  const S = { sel: {}, side: {}, pos: {}, drag: null, last: null, menu: null };
  let host = {};
  let root = null;

  const keyOf = (r) => host.key(r);
  const got = (map, r) => { const k = keyOf(r); if (!map[k]) map[k] = {}; return map[k]; };
  const selOf = (r) => got(S.sel, r).v || null;
  const sideOf = (r) => got(S.side, r).v || 'attr';
  const posOf = (r) => { const k = keyOf(r); if (!S.pos[k]) S.pos[k] = {}; return S.pos[k]; };

  // ---- 形 --------------------------------------------------------------
  function boxOf(id, t) {
    if (id === 'human') return { w: 76, h: 36 };
    const w = (t.workers || []).find((x, i) => 'w' + i === id);
    if (w) return { w: Math.max(108, textW(w.purpose || '（目的）') + 26), h: 40 };
    return { w: Math.max(96, textW(id, true) + 30), h: 46 };
  }
  // autoPos is where the figure lays a row out the first time: the human, the
  // members round it, and each member's workers in the nearest room that no
  // other point holds. A point never stands on another (監査 1-4).
  const GAP = 10;
  function hits(t, p, id, c) {
    const b = boxOf(id, t);
    return Object.keys(p).some((oid) => {
      if (oid === id || !p[oid]) return false;
      const q = p[oid], ob = boxOf(oid, t);
      return Math.abs(c[0] - q[0]) < (b.w + ob.w) / 2 + GAP && Math.abs(c[1] - q[1]) < (b.h + ob.h) / 2 + GAP;
    });
  }
  // freeNear keeps the wanted place when it is free, and otherwise walks round
  // it in rings until it finds room. It is the one rule for the automatic
  // places, an added worker, and a place a double press asked for.
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
    const host0 = p[from] || [0, 0], hbox = boxOf(from, t);
    const side = host0[0] >= 0 ? 1 : -1;
    return [host0[0] + side * (hbox.w / 2 + wbox.w / 2 + 30), host0[1]];
  }
  function autoPos(t) {
    const ids = ['human', ...t.members.map((m) => m.name)];
    const n = Math.max(ids.length, 2), thin = window.innerWidth <= 760;
    const R = n <= 3 ? 140 : 112 + n * 14;
    const rx = thin ? (n <= 3 ? 95 : 105) : R, ry = thin ? 70 + n * 34 : R * (n <= 3 ? 0.8 : 0.86);
    const out = {};
    ids.forEach((id, i) => {
      const a = (-90 + i * 360 / n) * Math.PI / 180;
      out[id] = [rx * Math.cos(a), ry * Math.sin(a)];
    });
    t.workers.forEach((w, i) => {
      const id = 'w' + i, b = boxOf(id, t);
      out[id] = freeNear(t, out, id, workerWant(t, out, w.from, b), 58);
    });
    return out;
  }
  function ensurePos(r) {
    const t = host.draft(r), p = posOf(r), ids = ['human', ...t.members.map((m) => m.name), ...t.workers.map((w, i) => 'w' + i)];
    Object.keys(p).forEach((k) => { if (!ids.includes(k)) delete p[k]; });
    if (!Object.keys(p).length) Object.assign(p, autoPos(t));
    const missing = ids.filter((id) => !p[id]);
    if (missing.length) {
      const known = Object.values(p);
      let bx = known.length ? Math.min(...known.map((q) => q[0])) : 0, by = known.length ? Math.max(...known.map((q) => q[1])) + 100 : 0;
      missing.forEach((id) => {
        const wi = /^w(\d+)$/.exec(id);
        if (wi) {
          const w = t.workers[Number(wi[1])], b = boxOf(id, t);
          p[id] = freeNear(t, p, id, workerWant(t, p, w.from, b), 58);
        } else {
          const b = boxOf(id, t);
          p[id] = freeNear(t, p, id, [bx, by], 54);
          bx += b.w + 30;
        }
      });
    }
    // A point that a change of purpose made wider, or one put down on another
    // by a double press, is moved to the nearest room: workers only, so a
    // member a person picked stays where it stands.
    t.workers.forEach((w, i) => {
      const id = 'w' + i;
      if (hits(t, p, id, p[id])) p[id] = freeNear(t, p, id, p[id], 40);
    });
    return p;
  }
  const edgePt = (a, box, b) => {
    const dx = b[0] - a[0], dy = b[1] - a[1];
    if (!dx && !dy) return a;
    const k = Math.min((box.w / 2 + 3) / Math.abs(dx || 1e-6), (box.h / 2 + 3) / Math.abs(dy || 1e-6));
    return [a[0] + dx * k, a[1] + dy * k];
  };
  function bdg(m, b) {
    const w = 34;
    return `<g class="bdg ${m.sev} ${m.sev === 'err' ? 'err' : 'warn'}" transform="translate(${(b.w / 2 - w / 2 - 2).toFixed(1)},${(-b.h / 2 - 6).toFixed(1)})"><rect x="${-w / 2}" y="-9" width="${w}" height="17" rx="3"></rect><text x="0" y="4" text-anchor="middle">${m.sev === 'err' ? '問題' : '警告'}</text></g>`;
  }

  function canvasHTML(r) {
    const t = host.draft(r), s = host.saved(r), p = ensurePos(r), sel = selOf(r), issues = host.findings(r) || [];
    const ids = ['human', ...t.members.map((m) => m.name)];
    const all = ids.concat(t.workers.map((w, i) => 'w' + i));
    const boxes = Object.fromEntries(all.map((id) => [id, boxOf(id, t)]));
    const xs = all.flatMap((id) => [p[id][0] - boxes[id].w / 2, p[id][0] + boxes[id].w / 2]);
    const ys = all.flatMap((id) => [p[id][1] - boxes[id].h / 2, p[id][1] + boxes[id].h / 2]);
    const pad = 40, vx = Math.min(...xs) - pad, vy = Math.min(...ys) - pad, vw = Math.max(...xs) - vx + pad, vh = Math.max(...ys) - vy + pad;
    const mi = keyOf(r).replace(/[^A-Za-z0-9_-]/g, '_');

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
      const ex = selE === k ? `<g class="cv-ex" data-cv-act="edge-del" data-k="${esc(k)}" transform="translate(${mx.toFixed(1)},${my.toFixed(1)})" role="button" tabindex="0" aria-label="この線を外す（両方の向き）"><title>この線を外す（両方の向き）</title><circle r="11"></circle><text x="0" y="4.5" text-anchor="middle">×</text></g>` : '';
      return `<path class="cv-hit" d="${dd}" data-cv-e="${esc(k)}" tabindex="0" role="button" aria-label="${esc(nodeName(a))} と ${esc(nodeName(b))} のターン"></path><path class="cv-e${isNew ? ' new' : ''}${selE === k ? ' sel' : ''}${S.flash === k ? ' cv-flash' : ''}" d="${dd}" ${fwd ? `marker-end="url(#cv-ar-${mi}-${mk})"` : ''} ${back ? `marker-start="url(#cv-ar-${mi}-${mk})"` : ''}></path>${ex}`;
    }).join('');
    const wedges = t.workers.map((w, i) => {
      if (!p['w' + i] || !p[w.from]) return '';
      const p1 = edgePt(p[w.from], boxes[w.from], p['w' + i]), p2 = edgePt(p['w' + i], boxes['w' + i], p[w.from]);
      return `<path class="cv-e wk" d="M${p1[0].toFixed(1)},${p1[1].toFixed(1)} L${p2[0].toFixed(1)},${p2[1].toFixed(1)}"></path>`;
    }).join('');

    const savedNames = new Set(s.members.map((m) => m.name));
    const node = (id) => {
      const b = boxes[id], x = p[id][0], y = p[id][1], mine = sel && sel.type === 'node' && sel.id === id;
      const is = issues.filter((m) => m.node === id);
      if (id === 'human') {
        return `<g class="cv-n human${mine ? ' sel' : ''}${is.some((m) => m.sev === 'err') ? ' err' : ''}" data-cv-n="human" transform="translate(${x.toFixed(1)},${y.toFixed(1)})" tabindex="0" role="button" aria-label="人間"><rect x="${-b.w / 2}" y="${-b.h / 2}" width="${b.w}" height="${b.h}" rx="18"></rect><text class="nm" x="0" y="5" text-anchor="middle">人間</text>${is.length ? bdg(is[0], b) : ''}</g>`;
      }
      const m = memberOf(t, id), sm = s.members.find((z) => z.name === id);
      const cls = !savedNames.has(id) ? ' new' : sm && (sm.role !== m.role || sm.profile !== m.profile) ? ' chg' : '';
      return `<g class="cv-n member${cls}${mine ? ' sel' : ''}${is.some((m) => m.sev === 'err') ? ' err' : ''}" data-cv-n="${esc(id)}" transform="translate(${x.toFixed(1)},${y.toFixed(1)})" tabindex="0" role="button" aria-label="${esc(id)}"><rect x="${-b.w / 2}" y="${-b.h / 2}" width="${b.w}" height="${b.h}" rx="6"></rect><text class="nm" x="0" y="-3" text-anchor="middle">${esc(id)}</text><text class="rl" x="0" y="14" text-anchor="middle">${esc(m.role)}</text>${is.length ? bdg(is[0], b) : ''}</g>`;
    };
    const wexists = new Set(s.workers.map((w) => w.from + '\u0000' + w.purpose));
    const wnode = (w, i) => {
      const b = boxes['w' + i], x = p['w' + i][0], y = p['w' + i][1], mine = sel && sel.type === 'worker' && sel.i === i, is = issues.filter((m) => m.worker === i);
      return `<g class="cv-n worker${wexists.has(w.from + '\u0000' + w.purpose) ? '' : ' new'}${mine ? ' sel' : ''}${is.some((m) => m.sev === 'err') ? ' err' : ''}" data-cv-w="${i}" transform="translate(${x.toFixed(1)},${y.toFixed(1)})" tabindex="0" role="button" aria-label="${esc(w.from)} の ${esc(w.purpose || '（目的）')}"><rect x="${-b.w / 2}" y="${-b.h / 2}" width="${b.w}" height="${b.h}" rx="5"></rect><text class="nm" x="0" y="-2" text-anchor="middle">${esc(w.purpose || '（目的）')}</text><text class="rl" x="0" y="13" text-anchor="middle">${esc(w.profile)}</text>${is.length ? bdg(is[0], b) : ''}</g>`;
    };
    let handles = '';
    if (sel && sel.type === 'node' && p[sel.id] && boxes[sel.id]) {
      const b = boxes[sel.id], [x, y] = p[sel.id];
      handles = [[0, -b.h / 2], [b.w / 2, 0], [0, b.h / 2], [-b.w / 2, 0]].map(([dx, dy]) => `<circle class="cv-h" cx="${(x + dx).toFixed(1)}" cy="${(y + dy).toFixed(1)}" r="6" data-cv-h="${esc(sel.id)}"><title>引いて、ターンを渡す相手につなぐ</title></circle>`).join('');
    }
    const marker = (id, cls) => `<marker id="cv-ar-${mi}-${id}" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7.5" markerHeight="7.5" orient="auto-start-reverse"><path class="cv-mk-${cls}" d="M0,0 L10,5 L0,10 z"></path></marker>`;
    return `<svg viewBox="${vx.toFixed(0)} ${vy.toFixed(0)} ${vw.toFixed(0)} ${vh.toFixed(0)}" role="group" aria-label="${esc(host.name(r))} のトポロジー">
      <defs>${marker('n', 'n')}${marker('a', 'a')}${marker('c', 'c')}</defs>
      ${wedges}${edges}${t.workers.map(wnode).join('')}${ids.map(node).join('')}${handles}<path class="cv-ghost" d="" hidden="hidden"></path><text class="cv-gl" x="0" y="0" hidden="hidden"></text></svg>`;
  }

  // ---- the right column: 属性 -----------------------------------------
  function diffList(r) {
    const s = host.saved(r), t = host.draft(r), out = [];
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
  const turnLabel = (t, b) => (b === 'human' ? '人間（ターンを渡したら、止まって人の答えを待つ）' : `${b}（ロール ${(memberOf(t, b) || {}).role || '?'}）`);
  function turnRows(t, s, id, dir) {
    const list = dir === 'out' ? t.turns.filter(([a]) => a === id).map(([, b]) => [id, b]) : t.turns.filter(([, b]) => b === id).map(([a]) => [a, id]);
    if (!list.length) return `<span class="cv-empty">なし</span>`;
    return list.map(([a, b]) => `<div class="cv-li${hasTurn(s, a, b) ? '' : ' new'}"><span>${dir === 'out' ? '→' : '←'} ${esc(nodeName(dir === 'out' ? b : a))}</span><button class="cv-x" type="button" data-cv-act="turn-del" data-a="${esc(a)}" data-b="${esc(b)}" aria-label="${esc(nodeName(a))} から ${esc(nodeName(b))} へのターンを外す" title="外す">×</button></div>`).join('');
  }
  function addTurnSel(t, id) {
    const cand = ['human', ...t.members.map((m) => m.name)].filter((x) => x !== id && !hasTurn(t, id, x));
    return cand.length ? `<span class="sel"><select data-cv-f="turn-add"><option value="">＋ 渡す相手を足す</option>${cand.map((c) => `<option value="${esc(c)}">${esc(nodeName(c))}</option>`).join('')}</select></span>` : '';
  }
  function fieldFindings(r, id) {
    return (host.findings(r) || []).filter((m) => m.node === id && (m.field === 'members' || m.field === 'name' || m.field === 'node'));
  }
  function memberMsgs(r, t, m) {
    const msgs = [];
    if (!NAME.test(m.name)) msgs.push('英小文字で始め、英小文字・数字・ハイフン・下線で 32 文字以内にしてください');
    if (t.members.filter((x) => x.name === m.name).length > 1) msgs.push('このチームにもういます');
    if (`${host.name(r)}-${m.name}`.length > 32) msgs.push(`herdr 名 ${host.name(r)}-${m.name} が 32 文字を超えます`);
    return msgs;
  }

  function sideAttr(r) {
    const t = host.draft(r), s = host.saved(r), sel = selOf(r), team = host.name(r);
    if (!sel) {
      const d = diffList(r), humanIn = t.turns.some(([, b]) => b === 'human');
      return `<div class="cv-t"><code>${esc(team)}</code><span class="cv-k">チーム定義</span></div>
        <div class="cv-f"><label for="cv-mi-${esc(team)}">使命</label><input id="cv-mi-${esc(team)}" class="cv-body" data-cv-f="mission" value="${esc(t.mission)}" autocomplete="off"></div>
        <div class="cv-note">メンバー ${t.members.length} · ワーカー ${t.workers.length} · ターン ${t.turns.length}</div>
        ${humanIn ? '' : '<div class="cv-warn">人間に届くターンがありません。人間の点の縁の丸から、または人間へ渡すメンバーの縁の丸から、線を引いてください</div>'}
        <div class="cv-sec"><span class="cv-sl">保存していない変更 ${d.length}</span>${d.length ? `<div class="cv-diff">${d.map(([k, x]) => `<span class="cv-${k}">${k === 'a' ? '+ ' : k === 'd' ? '− ' : '~ '}${esc(x)}</span>`).join('')}</div>` : '<span class="cv-empty">なし</span>'}</div>
        <div class="cv-sec"><span class="cv-sl">操作</span><span class="cv-note">点や線を押すと、ここにその属性が出る。選んだ点の縁の丸から、ほかの点へ引くとターンができる。<span class="cv-kbd">Delete</span> で外す、<span class="cv-kbd">Esc</span> で選ぶのをやめる、<span class="cv-kbd">⌘Z</span> で 1 つ取り消す。</span></div>`;
    }
    if (sel.type === 'edge') {
      const [a, b] = sel.k.split('|');
      const row = (p, q) => hasTurn(t, p, q)
        ? `<div class="cv-li${hasTurn(s, p, q) ? '' : ' new'}"><span>${esc(nodeName(p))} → ${esc(nodeName(q))}</span><button class="cv-x" type="button" data-cv-act="turn-del" data-a="${esc(p)}" data-b="${esc(q)}" title="外す" aria-label="外す">×</button></div>`
        : `<div class="cv-li"><span class="cv-empty">${esc(nodeName(p))} → ${esc(nodeName(q))}</span><button class="btn sm" type="button" data-cv-act="turn-add" data-a="${esc(p)}" data-b="${esc(q)}">この向きも足す</button></div>`;
      return `<div class="cv-t"><code>${esc(nodeName(a))} ⇄ ${esc(nodeName(b))}</code><span class="cv-k">ターン</span></div><div class="cv-sec"><span class="cv-sl">向き</span>${row(a, b)}${row(b, a)}</div>
        <div class="cv-note"><span class="cv-kbd">Delete</span> で両方の向きを外す。</div>`;
    }
    if (sel.type === 'worker') {
      const w = t.workers[sel.i];
      if (!w) return '';
      const profs = (host.profiles(r).includes(w.profile) ? [] : [w.profile]).concat(host.profiles(r));
      return `<div class="cv-t"><code>${esc(w.purpose || '（目的）')}</code><span class="cv-k">ワーカー</span></div>
        <div class="cv-f"><label for="cv-wp-${esc(team)}">目的</label><input id="cv-wp-${esc(team)}" data-cv-f="w-purpose" value="${esc(w.purpose)}" placeholder="例: 調べ物" autocomplete="off" spellcheck="false"></div>
        <div class="cv-f"><label for="cv-wf-${esc(team)}">追加するメンバー</label><span class="sel"><select id="cv-wf-${esc(team)}" data-cv-f="w-from">${t.members.map((m) => `<option ${m.name === w.from ? 'selected' : ''}>${esc(m.name)}</option>`).join('')}</select></span></div>
        <div class="cv-f"><label for="cv-wpf-${esc(team)}">プロファイル</label><span class="sel"><select id="cv-wpf-${esc(team)}" data-cv-f="w-profile">${profs.map((p) => `<option ${p === w.profile ? 'selected' : ''}>${esc(p)}</option>`).join('')}</select></span><span class="cv-sub">ワーカーは規則を受けない。安いプロファイルで</span></div>
        ${workerPromptHTML(r, w, sel.i)}
        <button class="btn danger sm cv-danger" type="button" data-cv-act="del">ワーカーを外す</button>`;
    }
    const id = sel.id;
    if (id === 'human') {
      return `<div class="cv-t"><code>人間</code><span class="cv-k">あなた。チームに 1 人だけで、外せない</span></div>
        <div class="cv-sec"><span class="cv-sl">ターンを渡す相手</span>${turnRows(t, s, id, 'out')}${addTurnSel(t, id)}</div>
        <div class="cv-sec"><span class="cv-sl">ターンを受け取る相手</span>${turnRows(t, s, id, 'in')}</div>`;
    }
    const m = memberOf(t, id);
    if (!m) return '';
    const msgs = memberMsgs(r, t, m).concat(fieldFindings(r, id).map((x) => x.text));
    const outs = t.turns.filter(([a]) => a === id).map(([, b]) => b);
    const ws = t.workers.map((w, i) => ({ w, i })).filter((z) => z.w.from === id);
    const nt = t.turns.filter(([a, b]) => a === id || b === id).length;
    const role = (host.roles(r) || []).find((x) => x.name === m.role) || {};
    return `<div class="cv-t"><code>${esc(m.name)}</code><span class="cv-k">メンバー${role.title ? ` · ${esc(role.title)}` : ''}</span></div>
      <div class="cv-f"><label for="cv-mn-${esc(team)}">名前</label><input id="cv-mn-${esc(team)}" data-cv-f="m-name" value="${esc(m.name)}" spellcheck="false" autocomplete="off">${msgs.map((x) => `<span class="cv-msg">${esc(x)}</span>`).join('')}<span class="cv-sub">herdr 名 ${esc(team)}-${esc(m.name)}</span></div>
      <div class="cv-f"><label for="cv-mr-${esc(team)}">ロール</label><span class="sel"><select id="cv-mr-${esc(team)}" data-cv-f="m-role">${(host.roles(r) || []).map((x) => `<option value="${esc(x.name)}" ${x.name === m.role ? 'selected' : ''}>${esc(x.name)}${x.title ? ` · ${esc(x.title)}` : ''}</option>`).join('')}</select></span><span class="cv-sub">無いロールは <a href="/roles">ロールの画面</a> で先に作る</span></div>
      <div class="cv-f"><label for="cv-mp-${esc(team)}">プロファイル</label><span class="sel"><select id="cv-mp-${esc(team)}" data-cv-f="m-profile">${host.profiles(r).map((p) => `<option ${p === m.profile ? 'selected' : ''}>${esc(p)}</option>`).join('')}</select></span></div>
      <div class="cv-sec"><span class="cv-sl">ターンを渡す相手</span>${turnRows(t, s, id, 'out')}${outs.length ? '' : '<span class="cv-note">まだ誰にもターンを渡していません。縁の丸から、渡す相手の点へ引いてください</span>'}${addTurnSel(t, id)}</div>
      <div class="cv-sec"><span class="cv-sl">ターンを受け取る相手</span>${turnRows(t, s, id, 'in')}</div>
      <div class="cv-sec"><span class="cv-sl">ワーカー</span>${ws.length ? ws.map(({ w, i }) => `<div class="cv-li${s.workers.some((x) => x.from === w.from && x.purpose === w.purpose) ? '' : ' new'}"><span>${esc(w.purpose || '（目的）')} · ${esc(w.profile)}</span><span class="cv-liend"><button class="cv-link" type="button" data-cv-act="pick-w" data-i="${i}">選ぶ</button><button class="cv-x" type="button" data-cv-act="w-del" data-i="${i}" title="外す" aria-label="外す">×</button></span></div>`).join('') : '<span class="cv-empty">なし</span>'}
        <button class="btn sm cv-danger" type="button" data-cv-act="w-add">＋ ワーカー</button></div>
      ${memberPromptHTML(r, m)}
      <div class="cv-sec"><button class="btn danger sm cv-danger" type="button" data-cv-act="del">メンバーを外す</button><span class="cv-note">ターン ${nt} 本とワーカー ${ws.length} 件も一緒に外れる。<span class="cv-kbd">⌘Z</span> で取り消せる。</span></div>`;
  }


  // ---- the prompt a member (or a worker) is handed --------------------
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
  const pline = (t) => {
    const m = String(t).match(/^(#{1,6} )(.*)$/);
    return m ? `<span class="cv-mm">${esc(m[1])}</span><span class="cv-ph">${esc(m[2])}</span>` : esc(t);
  };
  const plines = (lines, a, b, fn) => lines.slice(a, b).map((t, k) => `<div class="yl" data-ln="${a + k}"><span class="lno">${a + k + 1}</span><span class="yt">${fn ? fn(t) : pline(t)}</span></div>`).join('');
  const prange = (a, b) => (b - a > 1 ? `${a + 1}–${b}` : `${a + 1}`);
  const pcount = (lines, a, b) => lines.slice(a, b).filter((x) => x !== '').length;
  function templateChip(r, seg) {
    const o = seg.origin || {};
    if (o.kind === 'team') return `<span class="cv-ptpl own">ひな形: チームの prompt:</span><button class="cv-go" type="button" data-cv-act="to-yaml">YAML で直す</button>`;
    if (o.kind === 'member') return `<span class="cv-ptpl own">ひな形: メンバー ${esc(o.name || '')} の prompt:</span><button class="cv-go" type="button" data-cv-act="to-yaml">YAML で直す</button>`;
    return '<span class="cv-ptpl">ひな形: hergy の既定</span>';
  }
  const PLABEL = { roster: '名簿', turns: 'ターンを渡す相手', workers: 'ワーカーの追加' };
  // d-20ac: a definition with a problem is not given a prompt, by hergy up or by
  // this column. The words are the screen's own; the problem itself is the
  // server's, under the canvas.
  const PROMPT_BLOCKED = 'このチーム定義には問題があるので、hergy はプロンプトを組みません（hergy up も止まります）。下の validate の欄の問題を直すと、ここに出ます。';
  function memberPromptHTML(r, m) {
    const hit = (host.prompts(r) || []).find((x) => x.name === m.name);
    let body;
    if (host.promptBlocked && host.promptBlocked(r)) body = `<span class="cv-note">${PROMPT_BLOCKED}</span>`;
    else if (!hit) body = '<span class="cv-note">プロンプトを読んでいます。</span>';
    else {
      const { lines, blocks } = promptBlocks(hit.segments || []);
      let head = false, labelled = '';
      body = blocks.map(({ seg, a, b }) => {
        const o = seg.origin || {};
        if (seg.kind === 'role_doc') {
          return `<div class="cv-fold" data-a="${a}" data-b="${b}" data-doc="role" data-role="${esc(o.name || '')}" data-place="${esc(o.place || '')}"><span class="cv-range">${prange(a, b)}</span><b>ロールの文書</b><code>${esc(o.path || '')}</code><span>${pcount(lines, a, b)} 行</span><a href="/roles?open=${encodeURIComponent(o.name || '')}">ロールの画面で見る</a></div>`;
        }
        if (seg.kind === 'team_rules') {
          return `<div class="cv-fold" data-a="${a}" data-b="${b}" data-doc="team"><span class="cv-range">${prange(a, b)}</span><b>チームのルール</b><span>${pcount(lines, a, b)} 行</span><button class="cv-go" type="button" data-cv-act="to-agree">ルールのタブで見る</button></div>`;
        }
        if (seg.kind === 'herdr') {
          return `<div class="cv-fold cv-fixed" data-a="${a}" data-b="${b}"><span class="cv-range">${prange(a, b)}</span><details><summary>連絡のしかた<span> · hergy の決まった文 · どのチームでも同じ</span></summary>${plines(lines, a, b)}</details></div>`;
        }
        let label = '';
        // The heading of a section ("## 名簿") stands in the template just
        // before the section's own segment: the label goes above the heading,
        // and the segment that follows does not say it a second time.
        const heading = (lines.slice(a, b).find((x) => /^##\s/.test(x)) || '').replace(/^##\s*/, '');
        if (seg.kind === 'template' && PLABEL[seg.kind]) label = '';
        if (seg.kind === 'template' || seg.kind === 'actor') {
          if (!head) { head = true; label = `<div class="cv-pbl"><b>冒頭</b>${templateChip(r, seg)}</div>`; }
          const named = Object.values(PLABEL).find((x) => x === heading);
          if (named) { label += `<div class="cv-pbl"><b>${named}</b></div>`; labelled = named; }
        } else if (PLABEL[seg.kind]) {
          label = labelled === PLABEL[seg.kind] ? '' : `<div class="cv-pbl"><b>${PLABEL[seg.kind]}</b></div>`;
          labelled = PLABEL[seg.kind];
        }
        return label + plines(lines, a, b);
      }).join('');
    }
    return `<details class="cv-sec cv-prompt"><summary class="cv-sl">プロンプト（このチーム定義から渡る行）</summary><div class="cv-pr">${body}</div></details>`;
  }
  function workerPromptHTML(r, w, i) {
    if (host.promptBlocked && host.promptBlocked(r)) return `<details class="cv-sec cv-prompt"><summary class="cv-sl">プロンプト（hire のときに渡る文）</summary><span class="cv-note">${PROMPT_BLOCKED}</span></details>`;
    if (!w.purpose) return `<details class="cv-sec cv-prompt"><summary class="cv-sl">プロンプト（hire のときに渡る文）</summary><span class="cv-note">目的を書くと、hire のときに渡る文が出ます。</span></details>`;
    const pv = host.workerPrompt(r, i);
    let body;
    if (!pv) body = '<span class="cv-note">プロンプトを読んでいます。</span>';
    else if (pv.err) body = `<span class="cv-note">${esc(pv.err)}</span>`;
    else {
      const { lines, blocks } = promptBlocks(pv.segments || []);
      const full = `${host.name(r)}-${pv.name}`;
      const slotText = (t) => {
        const withName = esc(t).split(esc(full)).join(`${esc(host.name(r))}-<span class="cv-slot" data-v="${esc(pv.name)}">&lt;ワーカー名&gt;</span>`);
        return withName.split(esc(pv.task)).join(`<span class="cv-slot" data-v="${esc(pv.task)}">&lt;依頼&gt;</span>`);
      };
      const taskAt = blocks.findIndex(({ seg }) => seg.kind === 'task');
      const agreeAt = blocks.findIndex(({ seg, a, b }) => seg.kind === 'template' && lines.slice(a, b).some((x) => x.startsWith('## 依頼')));
      let herdrSeen = false;
      body = blocks.map(({ seg, a, b }, n) => {
        if (seg.kind === 'herdr') {
          herdrSeen = true;
          return `<div class="cv-fold cv-fixed" data-a="${a}" data-b="${b}"><span class="cv-range">${prange(a, b)}</span><details><summary>連絡のしかた<span> · hergy の決まった文 · どのワーカーでも同じ</span></summary>${plines(lines, a, b, slotText)}</details></div>`;
        }
        let label = '';
        if (n === 0) label = '<div class="cv-pbl"><b>冒頭</b><span class="cv-ptpl">ひな形: hergy の既定</span></div>';
        else if (n === agreeAt) label = '<div class="cv-pbl"><b>依頼</b><span>hire の --task</span></div>';
        else if (herdrSeen) label = '<div class="cv-pbl"><b>報告</b><span>追加したメンバーへ</span></div>';
        return label + plines(lines, a, b, slotText);
      }).join('');
    }
    return `<details class="cv-sec cv-prompt"><summary class="cv-sl">プロンプト（hire のときに渡る文）</summary><div class="cv-pr">${body}</div></details>`;
  }

  // ---- パネル ----------------------------------------------------------
  function panel(r) {
    // A picked point or line that the draft no longer holds is no pick: the
    // panel would show an attribute of nothing.
    const t = host.draft(r), sel = selOf(r);
    if (sel && sel.type === 'node' && sel.id !== 'human' && !memberOf(t, sel.id)) got(S.sel, r).v = null;
    if (sel && sel.type === 'worker' && !t.workers[sel.i]) got(S.sel, r).v = null;
    if (sel && sel.type === 'edge' && !sel.k.split('|').every((x) => x === 'human' || memberOf(t, x))) got(S.sel, r).v = null;
    const cur = selOf(r), side = sideOf(r);
    const picked = cur && cur.type === 'node' && cur.id !== 'human' ? cur.id : '';
    const hint = picked ? `${esc(picked)} を選んでいる。縁の丸から引くとターン、<span class="cv-kbd">Delete</span> で外す` : '点や線を押すと属性が出る。余白をダブルクリックすると、そこに足せる';
    return `<div class="cv-main"><div class="cv-left">
      <div class="cv-tools"><button class="btn sm" type="button" data-cv-act="m-add"${rolesWait()}>＋ メンバー</button><button class="btn sm" type="button" data-cv-act="w-add">＋ ワーカー</button><span class="cv-hint">${hint}</span></div>
      <div class="cv-canvas">${canvasHTML(r)}</div>
      <div class="vlist">${host.validate(r)}</div>
    </div><aside class="cv-side" aria-label="選んだものの属性"><div class="cv-sh"><button type="button" class="${side === 'attr' ? 'on' : ''}" data-cv-side="attr">属性</button><button type="button" class="${side === 'yaml' ? 'on' : ''}" data-cv-side="yaml">YAML</button></div>
      <div class="cv-sb">${side === 'attr' ? sideAttr(r) : host.yaml(r)}</div></aside></div>`;
  }

  // ---- 操作 ------------------------------------------------------------
  const rowOf = (el) => { const row = el && el.closest && el.closest('.tdef'); return row ? host.row(row) : null; };
  const key = (r) => keyOf(r);
  function select(r, sel, side) { got(S.sel, r).v = sel; if (side) got(S.side, r).v = side; host.redraw(r); }
  function menuClose() { S.menu = null; document.querySelectorAll('.cv-menu').forEach((m) => m.remove()); }
  function openMenu(r, at, e, workersOnly) {
    menuClose();
    const panelEl = $(`.panel.team-p[data-key="${CSS.escape(key(r))}"]`);
    const canvas = panelEl && $('.cv-canvas', panelEl);
    if (!canvas) return;
    const t = host.draft(r);
    canvas.insertAdjacentHTML('beforeend', `<div class="cv-menu" role="menu" aria-label="${workersOnly ? 'ワーカーを足す' : 'ここに足す'}">
      ${workersOnly ? '' : `<button type="button" role="menuitem" data-cv-act="menu-m"${rolesWait()}>メンバーを足す</button>`}
      <div class="cv-mh${workersOnly ? ' cv-mh0' : ''}">ワーカーを足す（どのメンバーの？）</div>
      <div class="cv-mw">${t.members.map((m) => `<button type="button" role="menuitem" data-cv-act="menu-w" data-from="${esc(m.name)}">${esc(m.name)}</button>`).join('')}</div></div>`);
    const menu = $('.cv-menu', canvas), cr = canvas.getBoundingClientRect();
    const mw = menu.offsetWidth, mh = menu.offsetHeight;
    const cx = e ? e.clientX - cr.left : 6, cy = e ? e.clientY - cr.top : 6;
    menu.style.setProperty('left', `${Math.max(6, Math.min(cx, cr.width - mw - 6))}px`);
    menu.style.setProperty('top', `${Math.max(6, Math.min(cy, cr.height - mh - 6))}px`);
    if (at) { menu.dataset.x = at[0]; menu.dataset.y = at[1]; }
    const first = $('button', menu); if (first) first.focus();
    S.menu = menu;
  }
  const svgPt = (svg, e) => { const p = svg.createSVGPoint(); p.x = e.clientX; p.y = e.clientY; return p.matrixTransform(svg.getScreenCTM().inverse()); };
  // rolesWait shuts the two ways of adding a member until the roles a new member
  // is named after have come (d-c5fe).
  const rolesWait = () => (host.rolesIn && !host.rolesIn() ? ' disabled' : '');
  function firstRole(r) { const rs = host.roles(r) || []; return (rs[0] || {}).name || ''; }
  function firstProfile(cheap, r) {
    const ps = host.profiles(r);
    return (cheap ? ps.find((p) => /haiku|mini|flash/.test(p)) : '') || ps[0] || '';
  }
  function addMember(r, at) {
    const t = host.draft(r), role = firstRole(r), profile = firstProfile(false, r);
    let n = 1; const base = role || 'member';
    while (t.members.some((m) => m.name === `${base}-${n}`)) n++;
    const name = `${base}-${n}`;
    if (at) posOf(r)[name] = at;
    got(S.sel, r).v = { type: 'node', id: name }; got(S.side, r).v = 'attr';
    host.act(r, [{ kind: 'addMember', member: name, role, profile }], {
      message: `メンバー ${name} を足した。名前・ロール・プロファイルを右で決める`,
      undo: `メンバー ${name} を足したのを取り消した`, focus: '[data-cv-f="m-name"]',
    });
  }
  function addWorker(r, from, at) {
    const t = host.draft(r), idx = t.workers.length, profile = firstProfile(true, r);
    const ops = [{ kind: 'addWorker', from, purpose: '', profile }];
    if (at) posOf(r)['w' + idx] = at;
    got(S.sel, r).v = { type: 'worker', i: idx }; got(S.side, r).v = 'attr';
    host.act(r, ops, { message: `${from} のワーカーを足した。目的を右で決める`, undo: 'ワーカーを足したのを取り消した', focus: '[data-cv-f="w-purpose"]' });
  }
  function del(r) {
    const t = host.draft(r), sel = selOf(r);
    if (!sel) return;
    if (sel.type === 'node' && sel.id !== 'human') {
      const id = sel.id, nt = t.turns.filter(([a, b]) => a === id || b === id).length, nw = t.workers.filter((w) => w.from === id).length;
      got(S.sel, r).v = null;
      host.act(r, [{ kind: 'removeMember', member: id }], { message: `${id} を外した（ターン ${nt} 本・ワーカー ${nw} 件も）`, undo: `${id} を外したのを取り消した` });
    } else if (sel.type === 'edge') {
      const [a, b] = sel.k.split('|'), ops = [];
      if (hasTurn(t, a, b)) ops.push({ kind: 'removeTurn', from: a, to: b });
      if (hasTurn(t, b, a)) ops.push({ kind: 'removeTurn', from: b, to: a });
      got(S.sel, r).v = null;
      if (ops.length) host.act(r, ops, { message: `${nodeName(a)} ⇄ ${nodeName(b)} のターンを外した`, undo: `${nodeName(a)} ⇄ ${nodeName(b)} のターンを外したのを取り消した` });
    } else if (sel.type === 'worker') {
      const w = t.workers[sel.i];
      if (!w) return;
      got(S.sel, r).v = null;
      host.act(r, [{ kind: 'removeWorker', from: w.from, purpose: w.purpose }], { message: `ワーカー ${w.purpose || '（目的）'} を外した`, undo: 'ワーカーを外したのを取り消した' });
    }
  }
  const focusAfter = (r, sel) => setTimeout(() => {
    const panelEl = $(`.panel.team-p[data-key="${CSS.escape(key(r))}"]`);
    const el = panelEl && $(sel, panelEl); if (el) { el.focus(); if (el.select) el.select(); }
  }, 0);

  function onPointerDown(e) {
    const svg = e.target.closest && e.target.closest('.cv-canvas svg');
    if (!svg) return;
    const r = rowOf(svg); if (!r) return;
    const t = host.draft(r);
    if (e.target.closest('[data-cv-act]')) return;
    const h = e.target.closest('[data-cv-h]');
    if (h) { S.drag = { what: 'conn', r, from: h.dataset.cvH, svg, start: svgPt(svg, e) }; svg.setPointerCapture(e.pointerId); e.preventDefault(); return; }
    const n = e.target.closest('[data-cv-n], [data-cv-w]');
    if (n) {
      const id = n.dataset.cvN || ('w' + n.dataset.cvW);
      S.drag = { what: 'move', r, id, wi: n.dataset.cvW != null ? Number(n.dataset.cvW) : null, svg, start: svgPt(svg, e), orig: [...ensurePos(r)[id]], moved: false };
      svg.setPointerCapture(e.pointerId); return;
    }
    const ed = e.target.closest('[data-cv-e]');
    if (ed) { got(S.sel, r).v = { type: 'edge', k: ed.dataset.cvE }; got(S.side, r).v = 'attr'; host.redraw(r); return; }
    const now = Date.now(), pt = svgPt(svg, e);
    if (S.last && now - S.last.t < 400 && Math.hypot(pt.x - S.last.x, pt.y - S.last.y) < 8 && S.last.r === r) {
      S.last = null; openMenu(r, [pt.x, pt.y], e); return;
    }
    S.last = { t: now, x: pt.x, y: pt.y, r };
    menuClose();
    if (selOf(r)) { got(S.sel, r).v = null; host.redraw(r); }
  }
  function onPointerMove(e) {
    const d = S.drag; if (!d) return;
    const r = d.r, t = host.draft(r), svg = d.svg;
    const p = svgPt(svg, e);
    if (d.what === 'move') {
      const dx = p.x - d.start.x, dy = p.y - d.start.y;
      if (!d.moved && Math.hypot(dx, dy) < 4) return;
      d.moved = true;
      ensurePos(r)[d.id] = [d.orig[0] + dx, d.orig[1] + dy];
      const g = svg.querySelector(d.wi != null ? `[data-cv-w="${d.wi}"]` : `[data-cv-n="${CSS.escape(d.id)}"]`);
      if (g) g.setAttribute('transform', `translate(${ensurePos(r)[d.id][0]},${ensurePos(r)[d.id][1]})`);
      return;
    }
    const ghost = svg.querySelector('.cv-ghost'), label = svg.querySelector('.cv-gl');
    if (!ghost || !label) return;
    ghost.removeAttribute('hidden');
    ghost.setAttribute('d', `M${d.start.x},${d.start.y} L${p.x},${p.y}`);
    $$('.cv-n.drop', svg).forEach((g) => g.classList.remove('drop'));
    const over = document.elementFromPoint(e.clientX, e.clientY), g = over && over.closest('[data-cv-n]');
    const to = g && g.dataset.cvN !== d.from ? g.dataset.cvN : null;
    if (to) g.classList.add('drop');
    label.removeAttribute('hidden');
    // The words stand to the right of the point's own box and below it, so they
    // never cover the name the person is aiming at (監査 1-6).
    if (to) {
      const c = ensurePos(r)[to], b = boxOf(to, t);
      label.setAttribute('x', c[0] + b.w / 2 + 12); label.setAttribute('y', c[1] + b.h / 2 + 18);
    } else {
      label.setAttribute('x', p.x + 24); label.setAttribute('y', p.y + 34);
    }
    label.textContent = to ? (hasTurn(t, d.from, to) ? `${nodeName(d.from)} → ${nodeName(to)} はもうある` : `${nodeName(d.from)} → ${nodeName(to)} に渡す`) : `${nodeName(d.from)} から、渡す相手の点まで引く`;
  }
  function onPointerUp(e) {
    const d = S.drag; if (!d) return;
    S.drag = null;
    const r = d.r;
    if (d.what === 'move') {
      if (d.moved) { host.redraw(r); return; }
      got(S.sel, r).v = d.wi != null ? { type: 'worker', i: d.wi } : { type: 'node', id: d.id };
      got(S.side, r).v = 'attr'; host.redraw(r); return;
    }
    const over = document.elementFromPoint(e.clientX, e.clientY), g = over && over.closest('[data-cv-n]');
    const to = g && g.dataset.cvN;
    if (!to || to === d.from) { host.redraw(r); return; }
    const t = host.draft(r);
    if (hasTurn(t, d.from, to)) { got(S.sel, r).v = { type: 'edge', k: pair(d.from, to) }; got(S.side, r).v = 'attr'; host.redraw(r); host.say(r, `${nodeName(d.from)} → ${nodeName(to)} はもうある`); return; }
    const k = pair(d.from, to);
    host.act(r, [{ kind: 'addTurn', from: d.from, to }], { message: `${nodeName(d.from)} → ${nodeName(to)} のターンを足した`, undo: `${nodeName(d.from)} → ${nodeName(to)} のターンを足したのを取り消した` });
    got(S.sel, r).v = { type: 'edge', k }; got(S.side, r).v = 'attr';
    S.flash = k;
    host.redraw(r);
    setTimeout(() => { const cur = S.flash; if (cur === k) { S.flash = null; const panelEl = $(`.panel.team-p[data-key="${CSS.escape(key(r))}"]`); const el = panelEl && $('.cv-e.cv-flash', panelEl); if (el) el.classList.remove('cv-flash'); } }, 1400);
  }
  function onClick(e) {
    // A press made without a pointer (the keyboard's Enter, or a click a walk
    // sends) picks the point or line too: the pointer path is not the only one.
    const hit = e.target.closest('[data-cv-n], [data-cv-w], [data-cv-e]');
    if (hit) {
      const r = rowOf(hit); if (!r) return;
      const sel = hit.dataset.cvN ? { type: 'node', id: hit.dataset.cvN }
        : hit.dataset.cvW != null ? { type: 'worker', i: Number(hit.dataset.cvW) }
          : { type: 'edge', k: hit.dataset.cvE };
      if (JSON.stringify(selOf(r)) === JSON.stringify(sel)) return;
      got(S.sel, r).v = sel; got(S.side, r).v = 'attr'; host.redraw(r); return;
    }
    const b = e.target.closest('[data-cv-act], [data-cv-side]');
    if (!b) return;
    const r = rowOf(b); if (!r) return;
    const t = host.draft(r), a = b.dataset.cvAct;
    if (b.dataset.cvSide) { got(S.side, r).v = b.dataset.cvSide; host.redraw(r); return; }
    if (a === 'm-add') { addMember(r, null); return; }
    if (a === 'w-add') {
      const sel = selOf(r), from = sel && sel.type === 'node' && sel.id !== 'human' ? sel.id : null;
      if (!from) { openMenu(r, null, null, true); return; }
      addWorker(r, from, null); return;
    }
    if (a === 'menu-m' || a === 'menu-w') {
      const menu = b.closest('.cv-menu'), at = menu && menu.dataset.x != null ? [+menu.dataset.x, +menu.dataset.y] : null;
      menuClose();
      if (a === 'menu-m') addMember(r, at);
      else addWorker(r, b.dataset.from, at);
      return;
    }
    if (a === 'to-yaml') { got(S.side, r).v = 'yaml'; host.redraw(r); return; }
    if (a === 'to-agree') { host.toAgree(r); return; }
    if (a === 'del') { del(r); return; }
    if (a === 'edge-del') {
      const [p, q] = (b.dataset.k || '').split('|'), ops = [];
      if (hasTurn(t, p, q)) ops.push({ kind: 'removeTurn', from: p, to: q });
      if (hasTurn(t, q, p)) ops.push({ kind: 'removeTurn', from: q, to: p });
      got(S.sel, r).v = null;
      if (ops.length) host.act(r, ops, { message: `${nodeName(p)} ⇄ ${nodeName(q)} のターンを外した`, undo: `${nodeName(p)} ⇄ ${nodeName(q)} のターンを外したのを取り消した` });
      return;
    }
    if (a === 'turn-del') {
      host.act(r, [{ kind: 'removeTurn', from: b.dataset.a, to: b.dataset.b }], { message: `${nodeName(b.dataset.a)} → ${nodeName(b.dataset.b)} のターンを外した`, undo: `${nodeName(b.dataset.a)} → ${nodeName(b.dataset.b)} のターンを足したのを取り消した` });
      return;
    }
    if (a === 'turn-add') {
      host.act(r, [{ kind: 'addTurn', from: b.dataset.a, to: b.dataset.b }], { message: `${nodeName(b.dataset.a)} → ${nodeName(b.dataset.b)} のターンを足した`, undo: `${nodeName(b.dataset.a)} → ${nodeName(b.dataset.b)} のターンを足したのを取り消した` });
      return;
    }
    if (a === 'w-del') {
      const w = t.workers[+b.dataset.i]; if (!w) return;
      host.act(r, [{ kind: 'removeWorker', from: w.from, purpose: w.purpose }], { message: `ワーカー ${w.purpose || '（目的）'} を外した`, undo: 'ワーカーを外したのを取り消した' });
      return;
    }
    if (a === 'pick-w') { got(S.sel, r).v = { type: 'worker', i: +b.dataset.i }; got(S.side, r).v = 'attr'; host.redraw(r); }
  }
  function onInput(e) {
    const f = e.target.dataset && e.target.dataset.cvF; if (!f) return;
    const r = rowOf(e.target); if (!r) return;
    const t = host.draft(r), sel = selOf(r), v = e.target.value;
    if (f === 'turn-add') {
      if (v) { host.act(r, [{ kind: 'addTurn', from: sel.id, to: v }], { message: `${nodeName(sel.id)} → ${nodeName(v)} のターンを足した`, undo: `${nodeName(sel.id)} → ${nodeName(v)} のターンを足したのを取り消した` }); e.target.value = ''; }
      return;
    }
    if (e.type === 'change' && e.target.tagName !== 'SELECT') return;
    if (e.type === 'input' && e.target.tagName === 'SELECT') return;
    if (f === 'mission') { host.typing(r, { kind: 'mission', text: v }, 'mission'); return; }
    if (f === 'm-name') {
      const m = sel && sel.type === 'node' ? memberOf(t, sel.id) : null;
      if (!m) return;
      // The messages stand beside the field as it is typed, so a name the
      // server would refuse is said before the field is left (監査 R9).
      const msgs = [];
      if (!NAME.test(v)) msgs.push('英小文字で始め、英小文字・数字・ハイフン・下線で 32 文字以内にしてください');
      if (t.members.some((x) => x !== m && x.name === v)) msgs.push('このチームにもういます');
      if (`${host.name(r)}-${v}`.length > 32) msgs.push(`herdr 名 ${host.name(r)}-${v} が 32 文字を超えます`);
      const field = e.target.closest('.cv-f');
      if (field) {
        $$('.cv-msg', field).forEach((x) => x.remove());
        const sub = $('.cv-sub', field);
        msgs.forEach((msg) => { const span = document.createElement('span'); span.className = 'cv-msg'; span.textContent = msg; field.insertBefore(span, sub || null); });
      }
      const ok = NAME.test(v) && v !== m.name;
      if (ok) {
        const p = posOf(r); if (p[m.name]) { p[v] = p[m.name]; delete p[m.name]; }
        host.typing(r, { kind: 'renameMember', member: m.name, new: v }, 'm-name');
        got(S.sel, r).v = { type: 'node', id: v };
      }
      return;
    }
    if (f === 'm-role' || f === 'm-profile') {
      if (!sel || sel.type !== 'node') return;
      host.act(r, [{ kind: f === 'm-role' ? 'memberRole' : 'memberProfile', member: sel.id, [f === 'm-role' ? 'role' : 'profile']: v }], { message: `メンバー ${sel.id} の${f === 'm-role' ? 'ロール' : 'プロファイル'}を変えた`, undo: `メンバー ${sel.id} の${f === 'm-role' ? 'ロール' : 'プロファイル'}を変えたのを取り消した` });
      return;
    }
    if (f === 'w-purpose' || f === 'w-from' || f === 'w-profile') {
      const w = sel && sel.type === 'worker' ? t.workers[sel.i] : null; if (!w) return;
      if (f === 'w-purpose') { host.typing(r, { kind: 'workerPurpose', from: w.from, old: w.purpose, new: v }, 'w-purpose'); return; }
      if (f === 'w-from') { host.act(r, [{ kind: 'moveWorker', from: w.from, purpose: w.purpose, to: v }], { message: `ワーカー ${w.purpose || '（目的）'} を ${v} の下へ移した`, undo: `ワーカー ${w.purpose || '（目的）'} を移したのを取り消した` }); return; }
      host.act(r, [{ kind: 'workerProfile', from: w.from, purpose: w.purpose, profile: v }], { message: `ワーカー ${w.purpose || '（目的）'} のプロファイルを変えた`, undo: 'ワーカーのプロファイルを変えたのを取り消した' });
    }
  }
  function onKey(e) {
    const inField = e.target.closest && e.target.closest('input, select, textarea');
    // The keys stand on the row a person is working in, not only on what has the
    // focus: a point is picked by the pointer, and the Delete that follows is
    // still meant for it (the mock of 2026-10-23).
    const inside = e.target.closest && e.target.closest('.panel.team-p');
    const panels = $$('.panel.team-p').filter((el) => rowOf(el));
    const panelEl = (inside && rowOf(inside)) ? inside : panels.find((el) => selOf(rowOf(el)));
    const r = panelEl ? rowOf(panelEl) : null;
    // ⌘Z stands for the newest operation even after the pick was let go (the
    // blank space, Esc, or the line's ×): the row that has something to take
    // back is the one it acts on. The field's own ⌘Z is left to the field.
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'z' && !inField) {
      const target = r || (host.undoRow && host.undoRow());
      if (target) { e.preventDefault(); host.undo(target); }
      return;
    }
    // Esc inside a field leaves the field and keeps what was typed; the next
    // Esc lets the pick go (監査 1-7).
    if (inField) { if (e.key === 'Escape') { e.preventDefault(); e.target.blur(); } return; }
    // The blank space's menu and the toolbar's ask stand even when no point is
    // picked; Esc closes them before anything else (監査 2-8).
    if (e.key === 'Escape' && S.menu) { e.preventDefault(); menuClose(); return; }
    if (!r) return;
    const sel = selOf(r);
    if (e.key === 'Delete' || e.key === 'Backspace') { if (sel) { e.preventDefault(); del(r); } return; }
    if (e.key === 'Escape') { menuClose(); if (sel) { e.preventDefault(); got(S.sel, r).v = null; host.redraw(r); } return; }
    if ((e.key === 'Enter' || e.key === ' ') && e.target.matches && e.target.matches('.cv-n, .cv-hit, .cv-ex')) {
      e.preventDefault();
      if (e.target.matches('.cv-hit')) { got(S.sel, r).v = { type: 'edge', k: e.target.dataset.cvE }; got(S.side, r).v = 'attr'; host.redraw(r); }
      else if (e.target.matches('.cv-ex')) e.target.dispatchEvent(new MouseEvent('click', { bubbles: true }));
      else { const id = e.target.dataset.cvN || ('w' + e.target.dataset.cvW); got(S.sel, r).v = e.target.dataset.cvN ? { type: 'node', id } : { type: 'worker', i: Number(e.target.dataset.cvW) }; got(S.side, r).v = 'attr'; host.redraw(r); }
    }
  }

  function mount(el, h) {
    root = el; host = h || {};
    root.addEventListener('pointerdown', onPointerDown);
    root.addEventListener('click', onClick);
    root.addEventListener('input', onInput);
    root.addEventListener('change', onInput);
    // The key is meant for the picked point even after the pointer took the
    // focus away from it (Delete after a press), so it stands on the page.
    document.addEventListener('keydown', onKey);
    root.addEventListener('focusout', (e) => {
      const f = e.target.dataset && e.target.dataset.cvF;
      if (f === 'm-name' || f === 'w-purpose' || f === 'mission') {
        const r = rowOf(e.target); if (r) host.endTyping(r);
      }
    });
    document.addEventListener('pointermove', onPointerMove);
    document.addEventListener('pointerup', onPointerUp);
  }

  window.hergyCanvas = {
    mount, panel, canvas: (r) => canvasHTML(r), side: (r) => sideAttr(r),
    select: (r, sel, side) => { got(S.sel, r).v = sel; if (side) got(S.side, r).v = side; },
    sel: (r) => selOf(r),
    reset: (k) => { delete S.sel[k]; delete S.side[k]; delete S.pos[k]; },
    drawn: (r) => { ensurePos(r); },
  };
})();
