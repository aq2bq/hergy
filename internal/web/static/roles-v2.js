// The role-document screen's behaviour (r-778c). The server draws the page's
// data island and answers /roles/check, /roles/save, /roles/delete,
// /roles/yaml/check, /roles/yaml/save and /roles/assign/save. This script keeps
// the drafts in the page, asks the server to check a draft while it is typed,
// draws the Markdown a member's prompt will carry from the server's lines, and
// mirrors what can be shared (open rows and rules, the filter, the search, the
// band and the chosen file) into the URL. Nothing is held anywhere but the page.
(() => {
  'use strict';

  const view = document.getElementById('view');
  if (!view) return;
  const token = view.dataset.token;
  let DATA = { project: '', roles: [], teams: [] };
  const island = document.getElementById('roles-data');
  if (island) { try { DATA = JSON.parse(island.textContent || '{}'); } catch (e) { DATA = { project: '', roles: [], teams: [] }; } }
  DATA.roles = DATA.roles || [];
  DATA.teams = DATA.teams || [];

  const PLACES = ['project', 'global'];
  const DIR = Object.assign({ project: '.hergy/roles/', global: '~/.config/hergy/roles/' }, DATA.places || {});
  const TEXT = DATA.texts || {};
  // fill is the server's sentence with the screen's numbers in it: the words
  // stay the server's (the same ones hergy validate prints).
  const fill = (s, ...a) => String(s || '').replace(/%[sd]/g, () => (a.length ? a.shift() : ''));
  const LNAME = { project: 'プロジェクト', global: '共通' };
  const KIND = { incident: 'インシデント', instruction: '指示', derived: '導出' };
  const LIMIT = { rules: 20, chars: 6000 };
  const clone = (o) => JSON.parse(JSON.stringify(o));
  const esc = (s) => String(s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
  const reEsc = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const other = (l) => (l === 'project' ? 'global' : 'project');
  const num = (n) => Number(n || 0).toLocaleString('ja-JP');
  const $ = (sel, root = document) => root.querySelector(sel);
  const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));
  let uid = 0;
  let kseq = 0;

  async function post(url, payload) {
    const body = new URLSearchParams();
    body.set('token', token);
    body.set('payload', JSON.stringify(payload));
    const res = await fetch(url, {
      method: 'POST', credentials: 'same-origin',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded;charset=UTF-8' }, body,
    });
    const text = await res.text();
    let json = null;
    if ((res.headers.get('content-type') || '').includes('application/json')) {
      try { json = JSON.parse(text); } catch (e) { json = null; }
    }
    return { ok: res.ok, status: res.status, json, text };
  }

  // ---- the model -------------------------------------------------------
  function fromJSON(d) {
    return {
      name: d.name || '', title: d.title || '', revision: d.revision === null || d.revision === undefined ? null : d.revision,
      sections: (d.sections || []).map((s) => ({
        heading: s.heading || '', level: s.level || 0, intro: s.intro || '',
        rules: (s.rules || []).map((x) => ({
          _k: ++kseq, id: x.id || '', text: x.text || '', why: x.why || '', send_why: x.send_why !== false, in_prompt: x.in_prompt !== false,
          note: x.note || '', report: x.report || '', tests: (x.tests || []).slice(),
          origin: (x.origin || []).map((o) => ({ kind: o.kind || 'incident', date: o.date || '', refs: (o.refs || []).slice(), note: o.note || '' })),
        })),
      })),
    };
  }

  // toDoc is the draft as the server reads it, with each rule's origin in the
  // saved document (its section and rule) so the server can rewrite one line.
  function toDoc(d, saved) {
    const at = new Map();
    if (saved) saved.sections.forEach((s, si) => s.rules.forEach((x, ri) => at.set(x._k, [si, ri])));
    return {
      name: d.name, title: d.title, revision: d.revision,
      sections: (d.sections || []).map((s) => ({
        heading: s.heading, level: s.level, intro: s.intro,
        rules: (s.rules || []).map((x) => {
          const f = at.get(x._k);
          return {
            from: f ? { section: f[0], rule: f[1] } : null, id: x.id, text: x.text, why: x.why, send_why: x.send_why, in_prompt: x.in_prompt,
            note: x.note, report: x.report, tests: x.tests, origin: x.origin,
          };
        }),
      })),
    };
  }

  const roles = DATA.roles.map((r) => {
    const row = { uid: r.uid };
    if (r.markdown) { row.markdown = r.markdown; row.name = r.name || ''; row.project = null; row.global = null; return row; }
    for (const l of PLACES) {
      if (r[l]) {
        const draft = fromJSON(r[l]);
        // A role no layer holds a document for opens as a new one (spec-roles
        // 1.2.10): nothing is saved yet, so the first 保存 writes the file, and
        // the row counts no rules and no amount until it is saved.
        const saved = r.noDoc ? null : draft;
        row[l] = { saved, draft: clone(draft), hash: r[l].hash || '', text: r[l].text || '' };
      } else { row[l] = null; }
    }
    if (r.noDoc) row.noDoc = true;
    if (r.isNew) row.isNew = true;
    return row;
  });
  const teams = DATA.teams.map((t) => ({
    id: t.id, hash: t.hash || '', turns: t.turns || [],
    agree: t.agreement || { rules: 0, chars: 0 },
    members: (t.members || []).map((m) => ({ name: m.name, saved: m.saved, draft: m.draft })),
  }));
  // A row a person adds gets an id of its own: start past the ones the island
  // carries, so a new row never takes an opened row's place.
  uid = roles.reduce((m, r) => Math.max(m, r.uid || 0), 0);

  // ---- lookups ---------------------------------------------------------
  const byUid = (id) => roles.find((r) => r.uid === Number(id));
  const live = (r, l) => !!(r[l] && r[l].saved);
  const eff = (r) => (live(r, 'project') ? 'project' : live(r, 'global') ? 'global' : (r.project ? 'project' : 'global'));
  const cur = (r) => r[eff(r)].draft;
  const curName = (r) => (r.markdown ? r.name || '' : (r[eff(r)].saved || r[eff(r)].draft).name);
  const savedRoles = () => roles.filter((r) => live(r, 'project') || live(r, 'global'));
  const roleByName = (n) => savedRoles().find((r) => curName(r) === n);
  const whoReads = (n) => teams.flatMap((t) => t.members.filter((m) => m.saved === n).map((m) => ({ t, m })));
  const readersCell = (name) => {
    const rs = whoReads(name);
    return rs.length ? `<b>${rs.length}</b><span>${esc([...new Set(rs.map((x) => x.t.id))].join(' · '))}</span>` : '<span class="none">0</span>';
  };
  const rulesOf = (d) => d.sections.flatMap((s) => s.rules);
  const findRule = (d, k) => { for (const [si, s] of d.sections.entries()) { const ri = s.rules.findIndex((x) => x._k === Number(k)); if (ri >= 0) return { si, ri, x: s.rules[ri] }; } return null; };
  const savedRule = (e, k) => (e.saved ? (findRule(e.saved, k) || { x: null }).x : null);
  const pathOf = (l, d) => DIR[l] + (d.name || '…') + '.yaml';

  // ---- server checks ---------------------------------------------------
  const checks = {};
  const reqs = {};
  const timers = {};
  const keyOf = (r, l) => r.uid + l;

  function ruleKeyAt(d, si, ri) {
    const s = d.sections[si];
    if (!s) return 0;
    const x = s.rules[ri];
    return x ? x._k : 0;
  }

  function convertProblems(d, list) {
    return (list || []).map((p) => ({
      sev: p.sev, k: p.section >= 0 ? ruleKeyAt(d, p.section, p.rule) : 0,
      f: p.field || '', msg: p.text || '', definition: p.definition || '',
    }));
  }

  function issuesOf(r, l) { const c = checks[keyOf(r, l)]; return c ? c.issues : []; }
  function markdownOf(r, l) { const c = checks[keyOf(r, l)]; return c ? c.markdown : { lines: [], chars: 0 }; }
  function diffOf(r, l) { const c = checks[keyOf(r, l)]; return c ? c.diff : { added: [], removedBefore: [], removedAtEnd: false }; }
  function turnsOf(r, l) { const c = checks[keyOf(r, l)]; return c ? c.turns : []; }

  async function askCheck(r, l, cb) {
    const e = r[l];
    if (!e) return;
    const k = keyOf(r, l);
    const id = (reqs[k] || 0) + 1;
    reqs[k] = id;
    const res = await post('/roles/check', { place: l, original: e.saved ? e.saved.name : null, doc: toDoc(e.draft, e.saved || e.base) });
    if (reqs[k] !== id) return;
    if (!res.ok || !res.json) return;
    checks[k] = {
      issues: convertProblems(e.draft, res.json.problems), markdown: res.json.markdown || { lines: [], chars: 0 },
      diff: res.json.diff || { added: [], removedBefore: [], removedAtEnd: false },
      turns: res.json.turns || [], readers: res.json.readers || [],
    };
    if (cb) cb();
  }

  function scheduleCheck(r, l, sec) {
    const k = keyOf(r, l);
    clearTimeout(timers[k]);
    timers[k] = setTimeout(() => askCheck(r, l, () => afterCheck(r, l, sec)), 300);
  }

  function afterCheck(r, l, sec) {
    const el = (sec && document.contains(sec)) ? sec : $(`.panel[data-uid="${r.uid}"][data-l="${l}"]`);
    if (el) markPanel(el);
    updateRow(r);
    updateCounts();
  }

  // ---- the shape of a problem, as the mock draws it --------------------
  const sevChip = (sev) => `<span class="sev ${sev === 'err' ? 'err' : 'warn'}">${sev === 'err' ? '問題' : '警告'}</span>`;
  const vmItem = (m) => `<li>${sevChip(m.sev)}<span>${esc(m.msg)}</span></li>`;
  const counts = (list) => {
    const e = list.filter((m) => m.sev === 'err').length, w = list.length - e;
    return (e ? `<span class="sev err">問題 ${e}</span>` : '') + (w ? `<span class="sev warn">警告 ${w}</span>` : '');
  };
  const badge = (list) => {
    const e = list.filter((m) => m.sev === 'err').length;
    return e ? `<span class="sev err">問題 ${e}</span>` : list.length ? `<span class="sev warn">警告 ${list.length}</span>` : '';
  };

  // ---- state -----------------------------------------------------------
  const state = {
    tab: 'roles', open: new Set(), ropen: new Set(), filter: null, q: '', roleHi: null,
    confirm: {}, ok: {}, yfile: null, pmode: {}, pfocus: {}, yscroll: {}, introOpen: null, wantFocus: null,
  };

  const ARROW_RE = /([A-Za-z][\w-]*|人間)\s*(→|⇄)\s*([A-Za-z][\w-]*|人間)/g;
  const MODEL_RE = /\b(claude|codex|opencode|gemini|opus|sonnet|haiku|fable|gpt-[\w.]+|deepseek[\w.-]*|qwen[\w.-]*)\b/gi;
  const arrowsOf = (text) => [...String(text).matchAll(ARROW_RE)].map((m) => ({ a: m[1], op: m[2], b: m[3], s: m[0] }));
  function turnState(t, ar) {
    const isHuman = (x) => x === '人間' || x === 'human';
    const who = (x) => (isHuman(x) ? ['human'] : t.members.filter((m) => m.name === x || m.saved === x).map((m) => m.name));
    const A = who(ar.a), B = who(ar.b);
    if (!A.length || !B.length) return { st: 'unknown', name: !A.length ? ar.a : ar.b };
    const has = (p, q) => p.some((x) => q.some((y) => t.turns.includes(`${x} -> ${y}`)));
    return has(A, B) && (ar.op !== '⇄' || has(B, A)) ? { st: 'ok' } : { st: 'missing' };
  }

  // ---- URL -------------------------------------------------------------
  // openNewPanel opens the row a screen shows for a role no document holds: its
  // one empty rule stands open and its check runs once, the way ＋ ロールを追加
  // does (spec-roles 1.2.4, 1.2.10). Until the check comes back the preview would
  // say 0 行 · 0 文字, which is not what the row holds.
  function openNewPanel(r) {
    const l = eff(r), e = r[l];
    const first = e && e.draft.sections[0] && e.draft.sections[0].rules[0];
    if (!first) return;
    state.ropen.add(rkey(r, l, first));
    if (!checks[keyOf(r, l)]) askCheck(r, l, () => render());
  }

  function parseURL() {
    const path = location.pathname;
    state.tab = /\/roles\/assign$/.test(path) ? 'assign' : /\/roles\/yaml$/.test(path) ? 'yaml' : 'roles';
    const qs = new URLSearchParams(location.search);
    if (state.tab === 'roles') {
      const f = qs.get('filter');
      state.filter = (f === 'noorigin' || f === 'nowhy' || f === 'issues') ? f : null;
      state.q = qs.get('q') || '';
      (qs.get('open') || '').split(',').filter(Boolean).forEach((name) => {
        // A row is found by the name it shows: a saved name, a written one, or
        // the name of a document only Markdown holds (spec-roles 2 節).
        const r = roles.find((x) => curName(x) === name);
        if (!r) return;
        state.open.add(r.uid);
        if (r.noDoc) openNewPanel(r);
      });
      (qs.get('rule') || '').split(',').filter(Boolean).forEach((spec) => {
        const m = /^(.*)\.(project|global)\.(.+)$/.exec(spec);
        if (!m) return;
        const r = roleByName(m[1]);
        if (!r || !r[m[2]] || !r[m[2]].saved) return;
        const f = findRule(r[m[2]].saved, 0);
        const hit = rulesOf(r[m[2]].saved).find((x) => x.id === m[3]);
        if (hit) { state.open.add(r.uid); state.ropen.add(`${r.uid}${m[2]}:${hit._k}`); }
      });
    } else if (state.tab === 'assign') {
      state.roleHi = qs.get('role') || null;
    } else {
      const f = qs.get('file');
      if (f) { const m = /^(project|global)\.(.+)$/.exec(f); if (m) { const r = roleByName(m[2]); if (r && r[m[1]] && r[m[1]].saved && eff(r) === m[1]) state.yfile = r.uid + m[1]; } }
    }
  }

  function syncURL(push) {
    let path = '/roles';
    const q = new URLSearchParams();
    if (state.tab === 'assign') {
      path = '/roles/assign';
      if (state.roleHi) q.set('role', state.roleHi);
    } else if (state.tab === 'yaml') {
      path = '/roles/yaml';
      if (state.yfile) { const m = /^(\d+)(project|global)$/.exec(state.yfile); if (m) { const r = byUid(m[1]); if (r) q.set('file', m[2] + '.' + curName(r)); } }
    } else {
      if (state.filter) q.set('filter', state.filter);
      if (state.q) q.set('q', state.q);
      const open = [...state.open].map((u) => { const r = byUid(u); return r ? curName(r) : ''; }).filter(Boolean);
      if (open.length) q.set('open', open.join(','));
      const rule = [...state.ropen].map((kk) => {
        const m = /^(\d+)(project|global):(\d+)$/.exec(kk);
        if (!m) return '';
        const r = byUid(m[1]);
        if (!r || !r[m[2]] || !r[m[2]].saved) return '';
        const hit = rulesOf(r[m[2]].saved).find((x) => x._k === Number(m[3]));
        return hit && hit.id ? `${curName(r)}.${m[2]}.${hit.id}` : '';
      }).filter(Boolean);
      if (rule.length) q.set('rule', rule.join(','));
    }
    const url = path + (q.toString() ? '?' + q.toString() : '');
    if (push) history.pushState(null, '', url); else history.replaceState(null, '', url);
  }

  // ---- decoration ------------------------------------------------------
  function deco(text) {
    const marks = state.q ? [new RegExp(reEsc(state.q), 'gi')] : [];
    return esc(text)
      .replace(ARROW_RE, (m) => `<span class="arrow">${m}</span>`)
      .replace(/(<[^>]+>)|([^<]+)/g, (m, tag, txt) => tag || txt.replace(MODEL_RE, (w) => `<span class="mw">${w}</span>`))
      .replace(/(<[^>]+>)|([^<]+)/g, (m, tag, txt) => tag || marks.reduce((s, re) => s.replace(re, (w) => `<mark>${w}</mark>`), txt));
  }
  function firstPhrase(t) { const i = t.indexOf('。'); return i >= 0 && i < t.length - 1 ? t.slice(0, i + 1) : t; }
  function snippet(t) {
    const f = firstPhrase(t), q = state.q.toLowerCase();
    if (!q || f.toLowerCase().includes(q) || !t.toLowerCase().includes(q)) return f;
    const hit = t.split(/(?<=。)/).find((x) => x.toLowerCase().includes(q));
    return hit ? `${f} … ${hit}` : f;
  }

  // ---- render ----------------------------------------------------------
  function render() {
    const people = teams.reduce((n, t) => n + t.members.length, 0);
    document.getElementById('tabs').innerHTML = [['roles', '一覧', ''], ['assign', '割り当て', people], ['yaml', 'YAML', '']]
      .map(([t, label, n]) => `<button class="tab ${state.tab === t ? 'on' : ''}" data-act="tab" data-t="${t}" aria-pressed="${state.tab === t}">${label}${n !== '' ? `<span class="n">${n}</span>` : ''}</button>`).join('');
    view.innerHTML = state.tab === 'roles' ? viewRoles() : state.tab === 'assign' ? viewAssign() : viewYAML();
    if (state.tab === 'roles') renderList();
    if (state.tab === 'yaml') $$('.yed').forEach(refreshYed);
    updateTabs();
    applyStyles();
    // The number of roles stands in the band now; the first tab is 一覧 and
    // carries none (spec-nav.md 1.1, 1.2). Every save, delete and copy comes
    // through here, so the band's number is the list's own.
    if (window.hergyNav) window.hergyNav.setCount('roles', savedRoles().length);
    tellDirty();
    // A row a person just added opens on its name, after the last redraw the
    // check caused (spec-roles 1.2「道具の列」).
    if (state.wantFocus) {
      const { uid: u, sel } = state.wantFocus;
      state.wantFocus = null;
      const el = $(`.panel[data-uid="${u}"] ${sel}`);
      if (el) { el.scrollIntoView({ block: 'nearest' }); el.focus({ preventScroll: true }); }
    }
  }
  function updateCounts() {
    const people = teams.reduce((n, t) => n + t.members.length, 0);
    [['roles', savedRoles().length], ['assign', people]].forEach(([t, n]) => {
      const el = $(`#tabs .tab[data-t="${t}"] .n`);
      if (el) el.textContent = n;
    });
    // The band's number is written where the tabs are drawn (render).
    $$('.fchip').forEach((c, i) => { const n = c.querySelector('.n'); if (n && FILTERS[i]) n.textContent = filterCount(FILTERS[i][0]); });
  }

  // ロール
  const FILTERS = [[null, 'すべて'], ['noorigin', '由来なし'], ['nowhy', 'なぜなし'], ['issues', '問題・警告']];
  const allEntries = () => roles.flatMap((r) => PLACES.filter((l) => r[l]).map((l) => ({ r, l, e: r[l] })));
  function ruleShown(x, issues) {
    if (state.filter === 'noorigin' && x.origin.length) return false;
    if (state.filter === 'nowhy' && x.why) return false;
    if (state.filter === 'issues' && !issues.some((m) => m.k === x._k)) return false;
    if (state.q) { const q = state.q.toLowerCase(); if (![x.id, x.text, x.why, x.note].some((s) => String(s).toLowerCase().includes(q))) return false; }
    return true;
  }
  const filtering = () => !!(state.filter || state.q);

  function filterCount(f) {
    return allEntries().reduce((c, { r, l, e }) => {
      const is = issuesOf(r, l);
      return c + rulesOf(e.draft).filter((x) => f === null || (f === 'noorigin' ? !x.origin.length : f === 'nowhy' ? !x.why : is.some((m) => m.k === x._k))).length;
    }, 0);
  }

  function viewRoles() {
    return `<div class="toolbar">
      <button class="btn primary" data-act="new">＋ ロールを追加</button>
      <div class="filters">${FILTERS.map(([f, label]) => `<button class="fchip ${state.filter === f ? 'on' : ''}" data-act="filter" data-f="${f || ''}" aria-pressed="${state.filter === f}">${label}<span class="n">${filterCount(f)}</span></button>`).join('')}</div>
      <input id="q" class="search" type="search" placeholder="規則を探す" value="${esc(state.q)}" autocomplete="off">
    </div>
    <div class="list">
      <div class="head"><span></span><span>名前</span><span>置き場所</span><span>規則</span><span>由来・なぜ・確かめ</span><span>1 人が読む量</span><span>読んでいる人</span></div>
      <div id="rows"></div>
    </div>`;
  }

  function renderList() {
    const shown = roles.filter((r) => !filtering() || r.isNew || r.markdown
      || PLACES.some((l) => r[l] && rulesOf(r[l].draft).some((x) => ruleShown(x, issuesOf(r, l)))));
    document.getElementById('rows').innerHTML = shown.map(roleHTML).join('')
      || `<div class="sum"><span></span><span class="none">${roles.length ? '0 件' : 'ロールはありません'}</span></div>`;
    $$('.panel').forEach(markPanel);
    applyStyles();
  }

  function rowIssues(r) {
    return PLACES.filter((l) => r[l]).flatMap((l) => issuesOf(r, l));
  }

  function maxReader(r) {
    // The most a person reads this role: its rules and characters, plus the
    // agreements of each team that plays it, counted from the island so a
    // definition the server cannot build a prompt for still shows its amount.
    const name = curName(r), seen = new Set(), zs = [];
    for (const { t } of whoReads(name)) {
      if (seen.has(t.id)) continue;
      seen.add(t.id);
      const z = sizeOf(t, name);
      if (z) zs.push(z);
    }
    zs.sort((a, b) => b.rules - a.rules);
    return zs[0] || null;
  }

  function amtHTML(rules, chars, withChars) {
    const over = rules > LIMIT.rules || chars > LIMIT.chars;
    const w = Math.min(100, rules / (LIMIT.rules * 1.25) * 100);
    return `<span class="amt ${over ? 'over' : ''}"><span class="meter ${over ? 'over' : ''}"><i data-w="${w}"></i></span><span class="v">${rules}<span class="lim">/${LIMIT.rules}</span></span>${withChars ? `<span class="lim">${num(chars)} 字</span>` : ''}</span>`;
  }

  // The page's CSP refuses a style="…" written as markup, so what a person
  // computed (a meter's width, a Markdown line's indent) is put on the elements
  // after they are drawn, through the stylesheet object (spec-roles 1.2.6).
  function applyStyles(root = view) {
    $$('[data-w]', root).forEach((el) => { el.style.width = el.dataset.w + '%'; });
    $$('[data-h]', root).forEach((el) => { el.style.setProperty('--h', el.dataset.h); });
  }

  function sumInner(r) {
    if (r.markdown) {
      return `<span class="chev"></span>
        <span class="c-name"><span class="nm">${esc(r.name)}</span><span class="sev err">問題 1</span></span>
        <span class="c-place"><span class="none">—</span></span>
        <span class="c-count"><span class="none">—</span></span>
        <span class="c-backing"></span>
        <span class="c-amount"><span class="none">—</span></span>
        <span class="c-readers">${readersCell(r.name)}</span>`;
    }
    const d = cur(r), saved = live(r, 'project') || live(r, 'global');
    const readers = whoReads(curName(r));
    const pills = r.noDoc ? '' : PLACES.map((l) => (live(r, l)
      ? `<span class="pill ${l === 'global' && live(r, 'project') ? 'shadow' : 'on'}">${LNAME[l]}</span>`
      : r[l] ? `<span class="pill pending">${LNAME[l]}</span>` : '')).join('');
    const issues = rowIssues(r);
    const rs = rulesOf(d);
    const tip = `由来 ${rs.filter((x) => x.origin.length).length}/${rs.length}・なぜ ${rs.filter((x) => x.why).length}/${rs.length}・確かめ ${rs.filter((x) => x.tests.length || x.report).length}/${rs.length}`;
    const mx = rs.map((x) => `<i class="p ${x.origin[0] ? 'o-' + x.origin[0].kind : ''}"></i><i class="p ${x.why ? 'why' : ''}"></i><i class="p ${x.tests.length || x.report ? 'tst' : ''}"></i>`).join('');
    const top = saved ? maxReader(r) : null;
    const teamsTxt = [...new Set(readers.map((x) => x.t.id))].join(' · ');
    return `<span class="chev"></span>
      <span class="c-name"><span class="nm">${esc(d.name) || '<span class="none">—</span>'}</span><span class="ttl">${esc(d.title)}</span>${r.isNew ? '<span class="tag">新規</span>' : ''}${saved ? badge(issues) : readers.length ? `<span class="sev err">問題 ${readers.length}</span>` : ''}</span>
      <span class="c-place">${pills}</span>
      <span class="c-count">${saved ? rs.length : '<span class="none">—</span>'}</span>
      <span class="c-backing" title="${tip}"><span class="mx" aria-label="${tip}">${mx}</span></span>
      <span class="c-amount">${top ? amtHTML(top.rules, top.chars) : '<span class="none">—</span>'}</span>
      <span class="c-readers">${readers.length ? `<b>${readers.length}</b><span>${esc(teamsTxt)}</span>` : '<span class="none">0</span>'}</span>`;
  }

  function updateRow(r) {
    const el = $(`.role[data-uid="${r.uid}"]`);
    if (!el) return;
    const sum = el.querySelector('.sum');
    if (sum) sum.innerHTML = sumInner(r);
    const usage = el.querySelector('.usage');
    if (usage) usage.innerHTML = usageHTML(r);
    applyStyles(el);
  }

  function roleHTML(r) {
    const open = filtering() || state.open.has(r.uid);
    return `<div class="role ${open ? 'open' : ''}" data-uid="${r.uid}">
      <div class="sum" role="button" tabindex="0" aria-expanded="${open}" data-act="toggle">${sumInner(r)}</div>
      ${open ? detailHTML(r) : ''}
    </div>`;
  }

  function usageHTML(r) {
    const readers = whoReads(curName(r));
    if (!readers.length) return '';
    const l = eff(r);
    return `<span class="lbl">読んでいる人</span>` + readers.map(({ t, m }) => {
      const z = sizeOf(t, curName(r)), over = z && (z.rules > LIMIT.rules || z.chars > LIMIT.chars);
      return `<button class="uchip ${over ? 'over' : ''}" data-act="goto" data-t="${t.id}" data-m="${esc(m.name)}" data-i="${t.members.indexOf(m)}"><span class="g">${t.id}</span>${esc(m.name)}<span class="z">${z ? z.rules : '—'}/${LIMIT.rules}</span></button>`;
    }).join('');
  }

  function detailHTML(r) {
    if (r.markdown) {
      return `<div class="detail">
      <div class="usage">${usageHTML(r)}</div>
      <div class="panels"><div class="dh"><ul class="vm dhvm">${vmItem({ sev: 'err', msg: r.markdown })}</ul></div></div>
    </div>`;
    }
    const ls = PLACES.filter((l) => r[l]);
    return `<div class="detail">
      <div class="usage">${usageHTML(r)}</div>
      <div class="panels">${ls.map((l) => panelHTML(r, l)).join('')}</div>
    </div>`;
  }

  function panelHTML(r, l) {
    const e = r[l], d = e.draft, key = r.uid + l, both = live(r, 'project') && live(r, 'global');
    const shadowed = l === 'global' && live(r, 'project');
    const nread = e.saved && !r[other(l)] ? whoReads(e.saved.name).length : 0;
    const ok = state.ok[key] ? (delete state.ok[key], '<span class="ok">保存しました</span>') : '';
    const mode = state.pmode[key] || 'edit';
    const issues = issuesOf(r, l);
    return `<section class="panel ${shadowed ? 'shadowed' : ''}" data-uid="${r.uid}" data-l="${l}" data-mode="${mode}">
      <header class="ph">
        ${r.isNew
          ? `<div class="seg">${PLACES.map((x) => `<button class="${l === x ? 'on' : ''}" data-act="pick-place" data-x="${x}">${LNAME[x]}</button>`).join('')}</div>`
          : `<span class="ln">${LNAME[l]}</span>`}
        <code class="path">${esc(pathOf(l, d))}</code>
        <span class="phr">
          ${both ? `<span class="state ${shadowed ? 'off' : 'on'}">${shadowed ? '隠れている' : '効いている'}</span>` : ''}
          <span class="seg pview">${[['edit', '編集'], ['md', 'Markdown']].map(([v, label]) => `<button class="${mode === v ? 'on' : ''}" data-act="pmode" data-v="${v}" aria-pressed="${mode === v}">${label}</button>`).join('')}</span>
        </span>
      </header>
      <div class="pmain">
      <div class="pedit">
      <div class="dh">
        <div class="dhf"><label for="n${key}">名前</label><input id="n${key}" class="in-name" data-f="name" value="${esc(d.name)}" spellcheck="false" autocomplete="off"><span class="errline name" hidden></span></div>
        <div class="dhf"><label for="t${key}">タイトル</label><input id="t${key}" class="in-title" data-f="title" value="${esc(d.title)}" autocomplete="off"></div>
        <div class="dhf"><label for="v${key}">revision</label><input id="v${key}" class="in-rev" data-f="revision" type="number" min="1" value="${d.revision === null ? '' : d.revision}"></div>
        <label class="rename" hidden><input type="checkbox" id="rn${key}" checked><span></span></label>
      </div>
      <ul class="vm dhvm"></ul>
      ${d.sections.map((sec, si) => secHTML(r, l, si)).join('')}
      <button class="add secadd" data-act="sec-add">＋ 節</button>
      </div>
      <aside class="yprev" aria-label="Markdown"><div class="yph"><span class="pkl">Markdown</span><span class="yinfo"></span></div><div class="ybox"></div></aside>
      </div>
      <footer class="pf">
        <button class="btn primary" data-act="save">保存</button><span class="errline doc" hidden></span><span class="dirty"></span><span class="vcount"></span>${ok}
        <span class="acts">
          ${e.saved ? `<button class="btn" data-act="dup">複製</button>` : ''}
          ${e.saved && !r[other(l)] ? `<button class="btn" data-act="copy">${LNAME[other(l)]}にコピー</button>` : ''}
          ${!e.saved ? `<button class="btn danger" data-act="discard">取り消し</button>`
            : state.confirm[key] ? `<button class="btn danger solid" data-act="del-yes">削除する${nread ? `（${nread} 人が読んでいる）` : ''}</button><button class="btn" data-act="del-no">やめる</button>`
            : `<button class="btn danger" data-act="del">削除</button>`}
        </span>
      </footer>
    </section>`;
  }

  function secHTML(r, l, si) {
    const d = r[l].draft, sec = d.sections[si], key = `${r.uid}${l}-s${si}`;
    const issues = issuesOf(r, l);
    const rules = sec.rules.filter((x) => !filtering() || ruleShown(x, issues));
    if (filtering() && !rules.length) return '';
    const ck = `${r.uid}${l}s${si}`;
    return `<section class="sec" data-si="${si}">
      <div class="sech">
        <input id="h${key}" class="in-head" data-f="heading" value="${esc(sec.heading)}" placeholder="見出し" autocomplete="off">
        <span class="sel"><select id="lv${key}" class="mini" data-f="level" aria-label="level">${[0, 2, 3, 4].map((x) => `<option value="${x}" ${sec.level === x ? 'selected' : ''}>${x ? 'level ' + x : 'level —'}</option>`).join('')}</select></span>
        <span class="end">
          ${sec.intro === '' && state.introOpen !== key && !state.confirm[ck] ? `<button class="add sm" data-act="intro-add">＋ 前置き</button>` : ''}
          ${state.confirm[ck] ? `<button class="btn danger solid sm" data-act="sec-del-yes">節を削除する（${sec.rules.length} 規則）</button><button class="btn sm" data-act="sec-del-no">やめる</button>`
            : `<button class="x" data-act="sec-del" aria-label="節 ${esc(sec.heading)} を消す">×</button>`}
        </span>
      </div>
      ${sec.intro !== '' || state.introOpen === key ? `<textarea id="in${key}" class="intro" data-f="intro" rows="1" placeholder="前置き">${esc(sec.intro)}</textarea>` : ''}
      <div class="rules">${rules.map((x) => ruleHTML(r, l, x)).join('')}</div>
      <button class="add secadd" data-act="rule-add">＋ 規則</button>
    </section>`;
  }

  const rkey = (r, l, x) => `${r.uid}${l}:${x._k}`;
  function ruleChanged(e, x) { return JSON.stringify(savedRule(e, x._k) || null) !== JSON.stringify(x); }

  function rsumInner(e, x) {
    const o = x.origin[0], mine = e.__issues ? e.__issues.filter((m) => m.k === x._k) : [];
    const org = o
      ? `<span class="org"><i class="sw o-${o.kind}"></i><span class="lbl">${KIND[o.kind] || esc(o.kind)}</span>${o.kind === 'derived' ? `<span class="dt">${esc(o.refs.join(' '))}</span>` : `<span class="dt">${esc(String(o.date).slice(5))}</span>`}${x.origin.length > 1 ? `<span class="more">+${x.origin.length - 1}</span>` : ''}</span>`
      : `<span class="org empty">—</span>`;
    return `<span class="grip" aria-hidden="true">⋮</span>
      <span class="rid"><b>${deco(x.id) || '<span class="none">—</span>'}</b>${badge(mine)}</span>
      <span class="rtx">${deco(snippet(x.text))}</span>
      <span class="marks">${org}<span>${x.why ? `<span class="mk why ${x.send_why ? '' : 'held'}">なぜ</span>` : ''}</span><span>${x.tests.length || x.report ? '<span class="mk tst">確かめ</span>' : ''}</span></span>`;
  }

  function ruleHTML(r, l, x) {
    const e = r[l], open = state.ropen.has(rkey(r, l, x));
    return `<div class="rule ${open ? 'open' : ''} ${ruleChanged(e, x) ? 'unsaved' : ''}" data-k="${x._k}">
      <div class="rsum" role="button" tabindex="0" aria-expanded="${open}" data-act="rtoggle">${rsumInner(e, x)}</div>
      ${open ? editHTML(r, l, x) : ''}
    </div>`;
  }

  function editHTML(r, l, x) {
    const key = `${r.uid}${l}-${x._k}`, ck = 'r' + key;
    return `<div class="redit">
      <div class="field"><label for="id${key}">ID</label><div><input id="id${key}" class="in-id" data-f="id" value="${esc(x.id)}" spellcheck="false" autocomplete="off"><ul class="vm" data-for="id"></ul></div></div>
      <div class="field"><label for="tx${key}">本文</label><div><textarea id="tx${key}" class="ta" data-f="text" rows="2">${esc(x.text)}</textarea><div class="tchk"></div><ul class="vm" data-for="text"></ul></div></div>
      <div class="field"><label for="wy${key}">なぜ</label><div><textarea id="wy${key}" class="ta" data-f="why" rows="1">${esc(x.why)}</textarea>
        <div class="whyrow"><div class="seg" data-f="send_why">${[[true, 'プロンプトに含める'], [false, '含めない']].map(([v, label]) => `<button class="${x.send_why === v ? 'on' : ''}" data-act="send-why" data-v="${v}" aria-pressed="${x.send_why === v}">${label}</button>`).join('')}</div></div></div></div>
      <div class="field"><span class="fl">由来</span><div><div class="origins">${x.origin.map((o, oi) => `<div class="orow" data-oi="${oi}">
          <div class="seg" data-f="o-kind">${Object.entries(KIND).map(([k, label]) => `<button class="${o.kind === k ? 'on' : ''}" data-act="o-kind" data-v="${k}" aria-pressed="${o.kind === k}"><i class="sw o-${k}"></i>${label}</button>`).join('')}</div>
          <input id="od${key}-${oi}" data-f="o-date" value="${esc(o.date)}" placeholder="YYYY-MM-DD" spellcheck="false" autocomplete="off" aria-label="日付">
          <div class="chips">${o.refs.map((ref, j) => `<span class="chip"><input id="rf${key}-${oi}-${j}" data-f="ref" data-j="${j}" value="${esc(ref)}" size="${Math.max(String(ref).length, 5)}" spellcheck="false" autocomplete="off" aria-label="参照"><button class="x" data-act="ref-del" data-j="${j}" aria-label="${esc(ref)} を消す">×</button></span>`).join('')}<button class="add sm" data-act="ref-add" aria-label="参照を足す">＋</button></div>
          <input id="on${key}-${oi}" data-f="o-note" value="${esc(o.note)}" placeholder="メモ" autocomplete="off">
          <button class="x" data-act="o-del" aria-label="由来を消す">×</button>
        </div>`).join('')}<button class="add sm" data-act="o-add" aria-label="由来を足す">＋</button></div><ul class="vm" data-for="origin"></ul></div></div>
      <div class="field"><span class="fl">確かめ</span><div class="ckrow">
        <span class="chips">${x.tests.map((tst, j) => `<span class="chip"><input id="ts${key}-${j}" data-f="test" data-j="${j}" value="${esc(tst)}" size="${Math.max(String(tst).length, 6)}" spellcheck="false" autocomplete="off" aria-label="テスト"><button class="x" data-act="test-del" data-j="${j}" aria-label="${esc(tst)} を消す">×</button></span>`).join('')}<button class="add sm" data-act="test-add" aria-label="テストを足す">＋ テスト</button></span>
        <span class="lb">報告</span><input id="rp${key}" class="in-report" data-f="report" value="${esc(x.report)}" placeholder="report.field" spellcheck="false" autocomplete="off">
      </div></div>
      <div class="field"><label for="nt${key}">メモ</label><div><input id="nt${key}" class="in-note" data-f="note" value="${esc(x.note)}" autocomplete="off"></div></div>
      <footer class="rf">
        <button class="btn sm" data-act="rule-up" aria-label="上へ">↑</button><button class="btn sm" data-act="rule-down" aria-label="下へ">↓</button>
        <span class="acts">
          <button class="btn sm" data-act="rule-revert" hidden>元に戻す</button>
          ${state.confirm[ck] ? `<button class="btn danger solid sm" data-act="rule-del-yes">規則を削除する</button><button class="btn sm" data-act="rule-del-no">やめる</button>`
            : `<button class="btn danger sm" data-act="rule-del">削除</button>`}
        </span>
      </footer>
    </div>`;
  }

  function diffCount(e) {
    const s = e.saved, d = e.draft;
    if (!s) return 1;
    // A rule that left its section counts, even when its values are the same and
    // the order of the keys did not change (spec-roles 1.2.8).
    const secAt = new Map();
    s.sections.forEach((sec, si) => sec.rules.forEach((x) => secAt.set(x._k, si)));
    let n = ['name', 'title', 'revision'].filter((f) => s[f] !== d[f]).length;
    const sr = new Map(rulesOf(s).map((x) => [x._k, JSON.stringify(x)])), dr = rulesOf(d);
    d.sections.forEach((sec, si) => sec.rules.forEach((x) => {
      if (sr.get(x._k) !== JSON.stringify(x) || secAt.get(x._k) !== si) n++;
    }));
    sr.forEach((v, k) => { if (!dr.some((x) => x._k === k)) n++; });
    for (let i = 0; i < Math.max(s.sections.length, d.sections.length); i++) {
      const a = s.sections[i], b = d.sections[i];
      if (!a || !b || ['heading', 'level', 'intro'].some((f) => a[f] !== b[f])) n++;
    }
    if (!n && sr.size && rulesOf(s).map((x) => x._k).join() !== dr.map((x) => x._k).join()) n++;
    return n;
  }

  function markPanel(sec) {
    const r = byUid(sec.dataset.uid), l = sec.dataset.l, e = r[l], s = e.saved, d = e.draft;
    const issues = issuesOf(r, l);
    e.__issues = issues;
    const mark = (el, ch) => el && el.classList.toggle('changed', !!ch);
    mark(sec.querySelector('[data-f=name]'), !s || s.name !== d.name);
    mark(sec.querySelector('[data-f=title]'), !s || s.title !== d.title);
    mark(sec.querySelector('[data-f=revision]'), !s || s.revision !== d.revision);
    const dhvm = sec.querySelector('.dhvm');
    if (dhvm) dhvm.innerHTML = issues.filter((m) => m.k === 0).map(vmItem).join('');
    sec.querySelectorAll('.sec').forEach((el) => {
      const si = +el.dataset.si, a = s && s.sections[si], b = d.sections[si];
      if (!b) return;
      mark(el.querySelector('[data-f=heading]'), !a || a.heading !== b.heading);
      mark(el.querySelector('[data-f=level]'), !a || a.level !== b.level);
      mark(el.querySelector('[data-f=intro]'), !a || a.intro !== b.intro);
    });
    sec.querySelectorAll('.rule').forEach((el) => {
      const f = findRule(d, el.dataset.k);
      if (!f) return;
      const mine = issues.filter((m) => m.k === f.x._k);
      el.querySelector('.rsum').innerHTML = rsumInner(e, f.x);
      el.classList.toggle('unsaved', ruleChanged(e, f.x));
      const ed = el.querySelector('.redit');
      if (ed) markEdit(ed, r, l, f.x, mine);
    });
    const hint = sec.querySelector('.rename'), nread = s && d.name && d.name !== s.name ? whoReads(s.name).length : 0;
    if (hint) { hint.hidden = !nread; if (nread) hint.querySelector('span').textContent = `読んでいる ${nread} 人の role も ${d.name || '—'} に書き換える`; }
    const n = diffCount(e);
    const foot = sec.querySelector('.pf');
    foot.querySelector('.vcount').innerHTML = counts(issues);
    foot.querySelector('.dirty').textContent = s && n ? `${n} か所` : '';
    const before = new Set(s ? issuesOfSaved(r, l, s).filter((m) => m.sev === 'err').map((m) => m.msg) : []);
    const saveBtn = sec.querySelector('[data-act=save]');
    if (saveBtn) saveBtn.disabled = !s ? issues.some((m) => m.sev === 'err') : (issues.some((m) => m.sev === 'err' && !before.has(m.msg)) || diffCount(e) === 0);
    const row = sec.closest('.role');
    if (row) {
      row.classList.toggle('unsaved', $$('.panel', row).some((p) => { const pe = byUid(p.dataset.uid)[p.dataset.l]; return pe && (!pe.saved || diffCount(pe) > 0); }));
    }
    refreshPrev(sec, issues);
  }

  // The problems of the saved document: the server's own run over it. Kept from
  // the first check of a panel (before any edit) so a save is refused only by a
  // problem the saved text did not already have (X3).
  function issuesOfSaved(r, l, s) {
    if (r[l].__savedIssues) return r[l].__savedIssues;
    return issuesOf(r, l);
  }

  function markEdit(ed, r, l, x, issues) {
    const sr = savedRule(r[l], x._k);
    const mark = (el, ch) => el && el.classList.toggle('changed', !!ch);
    ['id', 'text', 'why', 'note', 'report'].forEach((f) => { const el = ed.querySelector(`[data-f=${f}]`); mark(el, !sr || sr[f] !== x[f]); });
    mark(ed.querySelector('[data-f=send_why]'), sr && sr.send_why !== x.send_why);
    ed.querySelectorAll('.orow').forEach((row) => {
      const oi = +row.dataset.oi, so = sr && sr.origin[oi], o = x.origin[oi];
      if (!o) return;
      mark(row.querySelector('[data-f=o-kind]'), !so || so.kind !== o.kind);
      mark(row.querySelector('[data-f=o-date]'), !so || so.date !== o.date);
      mark(row.querySelector('[data-f=o-note]'), !so || so.note !== o.note);
      row.querySelectorAll('[data-f=ref]').forEach((el) => mark(el.closest('.chip'), !so || so.refs[+el.dataset.j] !== o.refs[+el.dataset.j]));
    });
    ed.querySelectorAll('[data-f=test]').forEach((el) => mark(el.closest('.chip'), !sr || sr.tests[+el.dataset.j] !== x.tests[+el.dataset.j]));
    ed.querySelectorAll('.bad').forEach((el) => el.classList.remove('bad'));
    ed.querySelectorAll('.vm[data-for]').forEach((u) => { u.innerHTML = issues.filter((m) => (m.f.split('.')[0] || '') === u.dataset.for).map(vmItem).join(''); });
    issues.forEach((m) => {
      const parts = m.f.split('.'), f = parts[0], oi = parts[1];
      let el = null;
      if (f === 'id') el = ed.querySelector('[data-f=id]');
      else if (f === 'text' && m.sev === 'err') el = ed.querySelector('[data-f=text]');
      else if (f === 'origin') {
        el = parts[2] === 'refs' ? (ed.querySelector(`.orow[data-oi="${oi}"] [data-f=ref][data-j="${parts[3]}"]`) || {}).closest?.('.chip')
          : ed.querySelector(`.orow[data-oi="${oi}"] [data-f=o-date]`);
      }
      if (el) el.classList.add('bad');
    });
    const tchk = ed.querySelector('.tchk');
    if (tchk) {
      const here = findRule(r[l].draft, x._k);
      const mine = here ? turnsOf(r, l).filter((z) => z.section === here.si && z.rule === here.ri) : [];
      const arrows = [...new Set(mine.map((z) => z.arrow))];
      tchk.innerHTML = arrows.map((a) => `<span class="arrow">${esc(a)}</span>` + mine.filter((z) => z.arrow === a)
        .map((z) => `<span class="tk ${z.ok ? 'okk' : 'warn'}">${esc(z.definition)}${z.ok ? ' ✓' : ''}</span>`).join('')).join('<span class="gap"></span>');
    }
    const rev = ed.querySelector('[data-act=rule-revert]');
    if (rev) rev.hidden = !sr || !ruleChanged(r[l], x);
  }

  const activeEntry = (r, l) => live(r, l) && eff(r) === l;

  function rerenderPanel(sec, focusSel) {
    const r = byUid(sec.dataset.uid), l = sec.dataset.l;
    const box = sec.querySelector('.ybox');
    if (box && box.clientHeight) state.yscroll[r.uid + l] = box.scrollTop;
    sec.outerHTML = panelHTML(r, l);
    const fresh = $(`.panel[data-uid="${r.uid}"][data-l="${l}"]`);
    if (fresh) markPanel(fresh);
    if (focusSel) { const el = fresh && fresh.querySelector(focusSel); if (el) { el.focus(); el.select?.(); } }
    return fresh;
  }

  // ---- the Markdown preview -------------------------------------------
  function lineIndex(lines, at) { return lines.map((_, i) => i); }
  function mdHL(t, held) {
    const heldHTML = held ? `<span class="held" aria-hidden="true"><span class="mk why held">なぜ</span><s>${esc(held)}</s></span>` : '';
    const body = (x) => esc(x).replace(MODEL_RE, (w) => `<span class="mw">${w}</span>`);
    let m = t.match(/^(#+) (.*)$/);
    if (m) return { h: 0, html: `<span class="mm">${m[1]} </span><span class="mh h${m[1].length}">${esc(m[2])}</span>${heldHTML}` };
    m = t.match(/^- \*\*(.*?)\*\* (.*)$/);
    if (m) return { h: 2, html: `<span class="mm">- **</span><b>${esc(m[1])}</b><span class="mm">**</span> ${body(m[2])}${heldHTML}` };
    m = t.match(/^(- |\d+\. |   )(.*)$/);
    if (m) return { h: m[1].length, html: `<span class="mm">${m[1]}</span>${body(m[2])}${heldHTML}` };
    return { h: 0, html: body(t) + heldHTML };
  }

  function refreshPrev(sec, issues) {
    const r = byUid(sec.dataset.uid), l = sec.dataset.l, e = r[l];
    const all = issues || issuesOf(r, l);
    const aside = sec.querySelector('.yprev');
    if (!aside) return;
    const md = markdownOf(r, l), lines = md.lines || [];
    const df = diffOf(r, l);
    const marks = {};
    lines.forEach((z, i) => {
      const k = z.section >= 0 ? ruleKeyAt(e.draft, z.section, z.rule) : 0;
      const ms = all.filter((m) => (z.section >= 0 ? m.k === k : m.k === 0));
      if (ms.length) marks[i] = ms;
    });
    const fc0 = state.pfocus[r.uid + l];
    const fc = fc0 && { k: fc0.k, f: ['id', 'text', 'why'].includes(fc0.f) ? 'text' : fc0.k ? '' : fc0.f };
    const added = new Set(df.added || []), removedBefore = new Set(df.removedBefore || []);
    const box = aside.querySelector('.ybox');
    box.innerHTML = lines.map((z, i) => {
      const ms = marks[i] || [], sev = ms.some((m) => m.sev === 'err') ? 'err' : ms.length ? 'warn' : '';
      const k = z.section >= 0 ? ruleKeyAt(e.draft, z.section, z.rule) : 0;
      const { h, html } = mdHL(z.text || '', z.held || '');
      const go = k || z.section >= 0 || z.part === 'title';
      const here = fc && fc.k && k === fc.k;
      const around = here && fc.f === 'text';
      const partNear = fc && !fc.k && (z.part === 'title' || z.part === 'heading' || z.part === 'intro')
        && ((z.part === 'title' && fc.f === 'title') || (z.part !== 'title' && fc.f.startsWith('sec.')));
      const cls = [sev, added.has(i) && 'ins', removedBefore.has(i) && 'del', here && 'cur', (around || partNear) && 'curf', go && 'go'].filter(Boolean).join(' ');
      return `<div class="yl ${cls}"${go ? ` data-act="yline" data-k="${k}" data-si="${z.section}" data-part="${z.part}"` : ''}${ms.length ? ` title="${esc(ms.map((m) => m.msg).join('\n'))}"` : ''}><span class="lno">${i + 1}</span><span class="yt" data-h="${h}">${html || ' '}</span></div>`;
    }).join('');
    const nadd = added.size, ndel = removedBefore.size;
    aside.querySelector('.yinfo').innerHTML = `<span class="vt">${lines.length} 行 · ${num(md.chars)} 文字</span>${nadd || ndel ? `<span class="dirty">+${nadd} −${ndel}</span>` : ''}<span class="yr">${all.length ? `<span class="vcount">${counts(all)}</span>` : '<span class="vclean">✓ 問題なし</span>'}</span>`;
    box.scrollTop = state.yscroll[r.uid + l] || 0;
    follow(box, r.uid + l);
    applyStyles(aside);
  }

  function follow(box, key) {
    const el = box.querySelector('.yl.curf') || box.querySelector('.yl.cur');
    if (!el || !box.clientHeight) return;
    const last = [...box.querySelectorAll(el.classList.contains('curf') ? '.yl.curf' : '.yl.cur')].pop();
    const top = el.offsetTop, bot = last.offsetTop + last.offsetHeight;
    if (top < box.scrollTop + 4 || bot > box.scrollTop + box.clientHeight - 4) box.scrollTop = Math.max(0, top - Math.round(box.clientHeight * 0.3));
    state.yscroll[key] = box.scrollTop;
  }

  function focusField(t) {
    const f = (t.closest('[data-f]') || {}).dataset ? t.closest('[data-f]').dataset.f : '';
    const oi = t.closest('.orow') ? t.closest('.orow').dataset.oi : null;
    if (oi != null) return `origin.${oi}` + ({ 'o-date': '.date', 'o-note': '.note', ref: '.refs' }[f] || '');
    if (['id', 'text', 'why', 'note', 'report', 'title', 'revision'].includes(f)) return f;
    if (f === 'send_why') return 'why';
    if (f === 'test' || t.closest('.ckrow')) return 'tests';
    if (['heading', 'level'].includes(f)) { const se = t.closest('.sec'); return se ? `sec.${se.dataset.si}` : ''; }
    if (f === 'intro') { const se = t.closest('.sec'); return se ? `sec.${se.dataset.si}.intro` : ''; }
    return '';
  }

  // ---- the assign tab --------------------------------------------------
  function viewAssign() {
    const cnt = {};
    teams.forEach((t) => t.members.forEach((m) => { cnt[m.draft] = (cnt[m.draft] || 0) + 1; }));
    if (!teams.length) return `<p class="none">チーム定義はありません</p>`;
    return `<div class="rolebar">${savedRoles().filter((r) => cnt[curName(r)]).map((r) => {
      const n = curName(r);
      return `<button class="rseg ${state.roleHi === n ? 'on' : state.roleHi ? 'dim' : ''}" data-act="role-hi" data-n="${esc(n)}" aria-pressed="${state.roleHi === n}"><code>${esc(n)}</code><span>${esc(cur(r).title)}</span><b>${cnt[n]}</b></button>`;
    }).join('')}</div>
      <div class="teams">${teams.map(teamHTML).join('')}</div>`;
  }

  function roleOptions(curRole) {
    const rs = savedRoles();
    const missing = !rs.some((r) => curName(r) === curRole);
    return (missing ? `<option selected>${esc(curRole)}</option>` : '') + PLACES.map((l) => {
      const mine = rs.filter((r) => eff(r) === l);
      return mine.length ? `<optgroup label="${LNAME[l]}">${mine.map((r) => `<option value="${esc(curName(r))}" ${curName(r) === curRole ? 'selected' : ''}>${esc(curName(r))}　${esc(cur(r).title)}</option>`).join('')}</optgroup>` : '';
    }).join('');
  }

  // A member's amount, counted from the island: the rules and characters of the
  // role the member plays, plus the agreements the team reads (spec-roles 1.3).
  // It is not asked of the server, so a definition the server cannot build a
  // prompt for — a profile that is not there — still shows its amount (6 C6),
  // and a role picked for a member is counted before the definition reads it.
  function sizeOf(t, roleName) {
    const r = roleByName(roleName);
    if (!r) return null;
    const l = eff(r), c = checks[r.uid + l];
    const rules = rulesOf(r[l].draft).filter((x) => x.in_prompt !== false).length + t.agree.rules;
    const chars = (c ? c.markdown.chars : 0) + t.agree.chars;
    return { rules, chars, over: rules > LIMIT.rules || chars > LIMIT.chars };
  }

  function arowHTML(t, m, i) {
    const r = roleByName(m.draft), z = r ? sizeOf(t, m.draft) : null;
    const dim = state.roleHi && m.draft !== state.roleHi ? 'dim' : '';
    const errs = [];
    if (!r) {
      // The sentence the server would print for validate, with its values: the
      // member's own name, and where hergy looked, as a person reads a path.
      const places = [DIR.project + m.draft + '.yaml', DIR.global + m.draft + '.yaml'].join(TEXT.separator || '、');
      errs.push({ sev: 'err', msg: fill(TEXT.no_role_doc, m.name, m.draft, places) });
    }
    if (z && z.rules > LIMIT.rules) errs.push({ sev: 'warn', msg: fill(TEXT.member_rules, `${t.id}-${m.name}`, z.rules, LIMIT.rules) });
    return `<div class="arow ${m.draft !== m.saved ? 'changed' : ''} ${r ? '' : 'missing'} ${dim}" data-m="${esc(m.name)}">
      <div class="who"><code>${esc(m.name)}</code></div>
      <div class="pick">${m.draft !== m.saved ? `<s class="was">${esc(m.saved)}</s>` : ''}<span class="sel"><select class="pickrole" id="s-${t.id}-${i}" data-t="${t.id}" data-i="${i}" aria-label="${esc(m.name)} のロール">${roleOptions(m.draft)}</select></span></div>
      ${z ? amtHTML(z.rules, z.chars, true) : ''}
      ${errs.length ? `<ul class="vm">${errs.map(vmItem).join('')}</ul>` : ''}
    </div>`;
  }

  function teamHTML(t) {
    const dirty = t.members.filter((m) => m.draft !== m.saved).length;
    const errs = t.members.filter((m) => !roleByName(m.draft)).length;
    const ok = state.ok['t' + t.id] ? (delete state.ok['t' + t.id], '<span class="ok">保存しました</span>') : '';
    return `<section class="team" data-t="${t.id}">
      <header class="th"><span class="tl">チーム定義</span><code class="tid">${t.id}</code><span class="tn">${t.members.length}</span>
        <span class="tside">${errs ? `<span class="sev err">問題 ${errs}</span>` : ''}${dirty ? `<span class="dirty">${dirty} 人</span><button class="btn primary" data-act="t-save" ${errs ? 'disabled' : ''}>保存</button>` : ok}</span>
        <code class="path">.hergy/teams/${t.id}/team.yaml</code>
        <span class="agree">約束 <b>${t.agree.rules}</b> 規則 · <b>${num(t.agree.chars)}</b> 字</span>
      </header>
      ${t.members.map((m, i) => arowHTML(t, m, i)).join('')}
    </section>`;
  }

  // ---- the YAML tab ----------------------------------------------------
  const ychecks = {};
  const ybase = {};
  function ykey(place, name) { return place + '.' + name; }

  async function askYAML(place, name, text, base) {
    const res = await post('/roles/yaml/check', { place, name, text });
    if (!res.ok || !res.json) return;
    const probs = res.json.problems || [];
    if (base) ybase[ykey(place, name)] = new Set(probs.filter((m) => m.sev === 'err').map((m) => m.text));
    ychecks[ykey(place, name)] = probs;
  }

  function filesList() { return allEntries().filter(({ r, l }) => live(r, l)); }

  function viewYAML() {
    const files = filesList();
    if (!files.length) return `<p class="none">ロールの文書はありません</p>`;
    if (!state.yfile || !files.some(({ r, l }) => r.uid + l === state.yfile)) state.yfile = files[0] ? files[0].r.uid + files[0].l : null;
    return `<div class="ylay">
      <nav class="files">${PLACES.map((l) => `<div class="fgroup"><div class="gh"><b>${LNAME[l]}</b><code>${DIR[l]}</code></div>${files.filter((f) => f.l === l).map(({ r, e }) => {
        const key = r.uid + l, is = issuesOf(r, l), hid = l === 'global' && live(r, 'project'), y = ychecks[ykey(l, e.saved.name)];
        const dirty = y && y !== ybase ? false : false;
        return `<button class="file ${state.yfile === key ? 'on' : ''} ${hid ? 'hid' : ''}" data-act="yfile" data-key="${key}" aria-pressed="${state.yfile === key}"><code>${esc(e.saved.name)}.yaml</code><span class="fs">${ychecksText(l, e.saved.name)}${badge(is) || rulesOf(e.saved).length}</span><span class="ft">${esc(e.saved.title)}${hid ? '・隠れている' : ''}</span></button>`;
      }).join('')}</div>`).join('')}</nav>
      ${state.yfile ? yedHTML(state.yfile) : '<div class="none">ロールの文書はありません</div>'}
    </div>`;
  }
  function ychecksText(place, name) {
    const y = ychecks[ykey(place, name)];
    return y && y.some((m) => m.sev === 'err') ? '<span class="dirty">●</span>' : '';
  }

  function docText(r, l) { const e = r[l]; return e.ytext !== undefined ? e.ytext : (e.text || ''); }

  function yedHTML(key) {
    const m = /^(\d+)(project|global)$/.exec(key);
    if (!m) return '';
    const r = byUid(m[1]), l = m[2], e = r[l];
    const hid = l === 'global' && live(r, 'project');
    const ok = state.ok['y' + key] ? (delete state.ok['y' + key], '<span class="ok">保存しました</span>') : '';
    return `<section class="yed" data-key="${key}">
      <header class="ph"><span class="ln">${LNAME[l]}</span><code class="path">${esc(pathOf(l, e.saved))}</code>${live(r, 'project') && live(r, 'global') ? `<span class="state ${hid ? 'off' : 'on'}">${hid ? '隠れている' : '効いている'}</span>` : ''}</header>
      <div class="ybody">
        <div class="code"><pre class="gutter" aria-hidden="true"></pre><textarea id="y-${key}" class="ytext" wrap="off" spellcheck="false" autocomplete="off" aria-label="${esc(pathOf(l, e.saved))}">${esc(docText(r, l))}</textarea></div>
        <div class="vlist"></div>
      </div>
      <footer class="pf">
        <button class="btn primary" data-act="yaml-save">保存</button><span class="dirty"></span>${ok}
        <span class="acts"><button class="btn" data-act="yaml-revert" hidden>元に戻す</button></span>
      </footer>
      <span class="errline" hidden></span>
    </section>`;
  }

  function refreshYed(sec) {
    const m = /^(\d+)(project|global)$/.exec(sec.dataset.key);
    if (!m) return;
    const r = byUid(m[1]), l = m[2], e = r[l];
    const ta = sec.querySelector('.ytext');
    if (!ta) return;
    const text = ta.value;
    const name = e.saved ? e.saved.name : e.draft.name;
    const probs = ychecks[ykey(l, name)] || [];
    const marks = {};
    probs.forEach((mm) => { if (marks[mm.line] !== 'err') marks[mm.line] = mm.sev; });
    const n = text.split('\n').length;
    sec.querySelector('.gutter').innerHTML = Array.from({ length: n }, (_, i) => `<span class="${marks[i + 1] || ''}">${i + 1}</span>`).join('');
    sec.querySelector('.code').style.height = Math.min(Math.max(n, 6), 34) * 20 + 32 + 'px';
    sec.querySelector('.gutter').scrollTop = ta.scrollTop;
    sec.querySelector('.vlist').innerHTML = `<div class="vhead"><span class="vt">validate</span>${probs.length ? `<span class="vcount">${counts(probs.map((mm) => ({ sev: mm.sev })))}</span>` : '<span class="vclean">✓ 問題なし</span>'}</div>`
      + probs.map((mm) => `<button class="vitem" data-act="yaml-jump" data-line="${mm.line}">${sevChip(mm.sev)}<span class="lno">${mm.line} 行</span><span class="msg">${esc(mm.text)}</span></button>`).join('');
    const base = ybase[ykey(l, name)] || new Set();
    const errs = probs.filter((mm) => mm.sev === 'err');
    const dirty = text !== (e.text || '');
    sec.querySelector('.dirty').textContent = dirty ? '変更あり' : '';
    const grew = errs.some((mm) => !base.has(mm.text));
    sec.querySelector('[data-act=yaml-save]').disabled = !dirty || grew;
    sec.querySelector('[data-act=yaml-revert]').hidden = !dirty;
    tellDirty();
  }

  async function saveYAML(key) {
    const m = /^(\d+)(project|global)$/.exec(key);
    if (!m) return;
    const r = byUid(m[1]), l = m[2], e = r[l], sec = $(`.yed[data-key="${key}"]`);
    if (!sec) return;
    const text = sec.querySelector('.ytext').value;
    const name = e.saved ? e.saved.name : e.draft.name;
    const res = await post('/roles/yaml/save', { place: l, name, text, hash: e.hash || '' });
    if (!res.ok) {
      const el = sec.querySelector('.errline');
      if (el) { el.hidden = false; el.textContent = (res.json && res.json.text) || '保存できませんでした'; }
      return;
    }
    e.hash = (res.json && res.json.hash) || e.hash;
    e.text = text;
    if (res.json && res.json.doc) { e.saved = fromJSON(res.json.doc); e.draft = clone(e.saved); }
    e.ytext = undefined;
    state.ok['y' + key] = 1;
    render();
  }

  // ---- operations ------------------------------------------------------
  const view2 = document.getElementById('view');
  document.getElementById('tabs').addEventListener('click', (e) => {
    const b = e.target.closest('[data-act=tab]');
    if (b) { state.tab = b.dataset.t; render(); syncURL(true); }
  });

  view.addEventListener('keydown', (e) => {
    if ((e.key === 'Enter' || e.key === ' ') && e.target.matches('.sum, .rsum')) { e.preventDefault(); e.target.click(); }
    if (e.key === 'Tab' && e.target.matches('.ytext') && !e.shiftKey) {
      e.preventDefault();
      e.target.setRangeText('  ', e.target.selectionStart, e.target.selectionEnd, 'end');
      e.target.dispatchEvent(new Event('input', { bubbles: true }));
    }
  });

  view.addEventListener('focusin', (e) => {
    const t = e.target;
    if (!t.closest('.pedit')) return;
    const c = ctx(t);
    if (!c.sec) return;
    const k = c.r.uid + c.l, fc = { k: c.x ? c.x._k : 0, f: focusField(t) };
    const was = state.pfocus[k];
    if (was && was.k === fc.k && was.f === fc.f) return;
    state.pfocus[k] = fc;
    refreshPrev(c.sec);
  });

  view.addEventListener('scroll', (e) => {
    if (e.target.matches?.('.ytext')) { const g = e.target.closest('.code').querySelector('.gutter'); if (g) g.scrollTop = e.target.scrollTop; }
    if (e.target.matches?.('.ybox')) { const p = e.target.closest('.panel'); if (p) state.yscroll[p.dataset.uid + p.dataset.l] = e.target.scrollTop; }
  }, true);

  const ctx = (el) => {
    const sec = el.closest('.panel');
    if (!sec) return {};
    const r = byUid(sec.dataset.uid), l = sec.dataset.l, d = r[l].draft;
    const ruleEl = el.closest('.rule'), f = ruleEl ? findRule(d, ruleEl.dataset.k) : null;
    const secEl = el.closest('.sec'), si = secEl ? +secEl.dataset.si : null;
    const oi = el.closest('.orow') ? +el.closest('.orow').dataset.oi : null;
    return { sec, r, l, d, x: f && f.x, si: f ? f.si : si, ri: f && f.ri, oi, key: `${r.uid}${l}-${f ? f.x._k : ''}` };
  };

  view.addEventListener('input', (e) => {
    const t = e.target;
    if (t.id === 'q') { state.q = t.value; render(); syncURL(false); const q = document.getElementById('q'); q.focus(); q.setSelectionRange(q.value.length, q.value.length); return; }
    if (t.matches('.ytext')) {
      const sec = t.closest('.yed'), m = /^(\d+)(project|global)$/.exec(sec.dataset.key);
      const r = byUid(m[1]), l = m[2], e = r[l];
      e.ytext = t.value;
      clearTimeout(timers['y' + sec.dataset.key]);
      timers['y' + sec.dataset.key] = setTimeout(async () => {
        await askYAML(l, e.saved ? e.saved.name : e.draft.name, e.ytext);
        refreshYed(sec);
      }, 300);
      refreshYed(sec);
      return;
    }
    const c = ctx(t);
    if (!c.sec || !c.d) return;
    const v = t.value, j = +t.dataset.j;
    if (!t.dataset.f && !t.closest('[data-f]')) {
      // nothing
    }
    const f = t.dataset.f || (t.closest('[data-f]') ? t.closest('[data-f]').dataset.f : '');
    switch (f) {
      case 'name': c.d.name = v; { const p = c.sec.querySelector('.path'); if (p) p.textContent = pathOf(c.l, c.d); } break;
      case 'title': c.d.title = v; break;
      case 'revision': c.d.revision = v === '' ? null : +v; break;
      case 'heading': c.d.sections[c.si].heading = v; break;
      case 'intro': c.d.sections[c.si].intro = v; break;
      case 'id': case 'text': case 'why': case 'note': case 'report': c.x[f] = v; break;
      case 'o-date': c.x.origin[c.oi].date = v; break;
      case 'o-note': c.x.origin[c.oi].note = v; break;
      case 'ref': c.x.origin[c.oi].refs[j] = v; t.size = Math.max(v.length, 5); break;
      case 'test': c.x.tests[j] = v; t.size = Math.max(v.length, 6); break;
      default: return;
    }
    markPanel(c.sec);
    scheduleCheck(c.r, c.l, c.sec);
    // A field inside a row changes what is unsaved without a redraw, so the
    // band is told here (R3).
    tellDirty();
  });

  view.addEventListener('change', (e) => {
    const t = e.target;
    if (t.matches('.pickrole')) {
      const team = teams.find((x) => x.id === t.dataset.t);
      if (team) { team.members[+t.dataset.i].draft = t.value; render(); const el = document.getElementById(t.id); if (el) el.focus(); }
      return;
    }
    const c = ctx(t);
    if (!c.sec || !c.d) return;
    const f = t.dataset.f || (t.closest('[data-f]') ? t.closest('[data-f]').dataset.f : '');
    if (f === 'level') { c.d.sections[c.si].level = +t.value; markPanel(c.sec); scheduleCheck(c.r, c.l, c.sec); }
    tellDirty();
  });

  function moveRule(d, si, ri, dir) {
    const s = d.sections[si], removed = s.rules.splice(ri, 1), x = removed[0];
    if (ri + dir >= 0 && ri + dir <= s.rules.length) s.rules.splice(ri + dir, 0, x);
    else if (dir < 0 && si > 0) d.sections[si - 1].rules.push(x);
    else if (dir > 0 && si < d.sections.length - 1) d.sections[si + 1].rules.unshift(x);
    else s.rules.splice(ri, 0, x);
  }

  view.addEventListener('click', async (e) => {
    const b = e.target.closest('[data-act]');
    if (!b) return;
    const act = b.dataset.act, c = ctx(b);
    const row = b.closest('.role'), r = c.r || (row && byUid(row.dataset.uid));
    const pkey = c.sec && r.uid + c.l, ck = c.x ? 'r' + c.key : null;
    const sk = c.sec && c.si != null ? `${r.uid}${c.l}s${c.si}` : null;
    const re = (sel) => rerenderPanel(c.sec, sel);

    switch (act) {
      case 'toggle':
        if (state.open.has(r.uid)) state.open.delete(r.uid);
        else { state.open.add(r.uid); if (r.noDoc) openNewPanel(r); }
        renderList(); syncURL(false);
        { const s = $(`.role[data-uid="${r.uid}"] .sum`); if (s) s.focus(); }
        break;
      case 'rtoggle': {
        const k = rkey(r, c.l, c.x);
        if (state.ropen.has(k)) state.ropen.delete(k); else state.ropen.add(k);
        re(`.rule[data-k="${c.x._k}"] .rsum`); syncURL(false); break;
      }
      case 'filter': state.filter = b.dataset.f || null; render(); syncURL(false); break;
      case 'new': {
        const d = { name: '', title: '', revision: 1, sections: [{ heading: '使命', level: 0, intro: '', rules: [{ _k: ++kseq, id: '', text: '', why: '', send_why: true, in_prompt: true, note: '', report: '', tests: [], origin: [] }] }] };
        const nr = { uid: ++uid, isNew: true, project: { saved: null, draft: d, hash: '', text: '' }, global: null };
        roles.unshift(nr); state.open.add(nr.uid); state.ropen.add(`${nr.uid}project:${d.sections[0].rules[0]._k}`); state.filter = null; state.q = '';
        render(); askCheck(nr, 'project', () => { state.wantFocus = { uid: nr.uid, sel: '[data-f=name]' }; render(); });
        break;
      }
      case 'pick-place':
        if (b.dataset.x !== c.l) { r[b.dataset.x] = r[c.l]; r[c.l] = null; renderList(); }
        break;
      case 'sec-add':
        c.d.sections.push({ heading: '', level: 0, intro: '', rules: [] });
        re(`.sec[data-si="${c.d.sections.length - 1}"] [data-f=heading]`);
        scheduleCheck(r, c.l, c.sec); break;
      case 'sec-del': state.confirm[sk] = 1; re(`.sec[data-si="${c.si}"] [data-act=sec-del-yes]`); break;
      case 'sec-del-no': delete state.confirm[sk]; re(); break;
      case 'sec-del-yes': delete state.confirm[sk]; c.d.sections.splice(c.si, 1); re(); scheduleCheck(r, c.l, c.sec); break;
      case 'intro-add': state.introOpen = `${r.uid}${c.l}-s${c.si}`; re(`.sec[data-si="${c.si}"] [data-f=intro]`); break;
      case 'rule-add': {
        const x = { _k: ++kseq, id: '', text: '', why: '', send_why: true, in_prompt: true, note: '', report: '', tests: [], origin: [] };
        c.d.sections[c.si].rules.push(x); state.ropen.add(rkey(r, c.l, x));
        re(`.rule[data-k="${x._k}"] [data-f=id]`); scheduleCheck(r, c.l, c.sec); break;
      }
      case 'rule-up': case 'rule-down': moveRule(c.d, c.si, c.ri, act === 'rule-up' ? -1 : 1); re(`.rule[data-k="${c.x._k}"] [data-act=${act}]`); scheduleCheck(r, c.l, c.sec); break;
      case 'rule-revert': { const sx = clone(savedRule(r[c.l], c.x._k)); if (sx) c.d.sections[c.si].rules[c.ri] = sx; re(`.rule[data-k="${c.x._k}"] .rsum`); scheduleCheck(r, c.l, c.sec); break; }
      case 'rule-del': state.confirm[ck] = 1; re(`.rule[data-k="${c.x._k}"] [data-act=rule-del-yes]`); break;
      case 'rule-del-no': delete state.confirm[ck]; re(); break;
      case 'rule-del-yes': delete state.confirm[ck]; c.d.sections[c.si].rules.splice(c.ri, 1); re(); scheduleCheck(r, c.l, c.sec); break;
      case 'send-why': c.x.send_why = b.dataset.v === 'true'; re(`.rule[data-k="${c.x._k}"] [data-act=send-why].on`); scheduleCheck(r, c.l, c.sec); break;
      case 'o-kind': c.x.origin[c.oi].kind = b.dataset.v; re(`.rule[data-k="${c.x._k}"] .orow[data-oi="${c.oi}"] [data-act=o-kind].on`); scheduleCheck(r, c.l, c.sec); break;
      case 'o-add': c.x.origin.push({ kind: 'incident', date: '', refs: [], note: '' }); re(`.rule[data-k="${c.x._k}"] .orow[data-oi="${c.x.origin.length - 1}"] [data-f=o-date]`); scheduleCheck(r, c.l, c.sec); break;
      case 'o-del': c.x.origin.splice(c.oi, 1); re(); scheduleCheck(r, c.l, c.sec); break;
      case 'ref-add': c.x.origin[c.oi].refs.push(''); re(`.rule[data-k="${c.x._k}"] .orow[data-oi="${c.oi}"] [data-f=ref][data-j="${c.x.origin[c.oi].refs.length - 1}"]`); break;
      case 'ref-del': c.x.origin[c.oi].refs.splice(+b.dataset.j, 1); re(); scheduleCheck(r, c.l, c.sec); break;
      case 'test-add': c.x.tests.push(''); re(`.rule[data-k="${c.x._k}"] [data-f=test][data-j="${c.x.tests.length - 1}"]`); break;
      case 'test-del': c.x.tests.splice(+b.dataset.j, 1); re(); scheduleCheck(r, c.l, c.sec); break;
      case 'save': await doSave(r, c.l); break;
      case 'pmode': state.pmode[pkey] = b.dataset.v; re(`[data-act=pmode][data-v="${b.dataset.v}"]`); break;
      case 'yline': {
        const k = +b.dataset.k, si = b.dataset.si, part = b.dataset.part;
        if (k) state.ropen.add(`${r.uid}${c.l}:${k}`);
        state.pmode[pkey] = 'edit';
        state.pfocus[pkey] = { k, f: k ? 'text' : part === 'title' ? 'title' : part === 'intro' ? `sec.${si}.intro` : `sec.${si}` };
        const fresh = re();
        const el = fresh && fresh.querySelector(k ? `.rule[data-k="${k}"] [data-f=text]` : part === 'title' ? '[data-f=title]' : `.sec[data-si="${si}"] [data-f=${part === 'intro' ? 'intro' : 'heading'}]`);
        if (el) { el.scrollIntoView({ block: 'nearest' }); el.focus({ preventScroll: true }); }
        syncURL(false);
        break;
      }
      case 'dup': {
        // The copy keeps the places its rules stood in (their _k), so the server
        // writes the new file from the original's own lines (spec-roles 1.2.9).
        const src = r[c.l].saved;
        const d = clone(src); d.name += '-2';
        const nr = { uid: ++uid, isNew: true, [c.l]: { saved: null, draft: d, base: clone(src), basePlace: { place: c.l, name: src.name }, hash: '', text: '' } };
        nr.project = nr.project || null; nr.global = nr.global || null;
        roles.splice(roles.indexOf(r) + 1, 0, nr); state.open.add(nr.uid); renderList();
        askCheck(nr, c.l, () => { state.wantFocus = { uid: nr.uid, sel: '[data-f=name]' }; render(); });
        break;
      }
      case 'copy':
        r[other(c.l)] = { saved: null, draft: clone(r[c.l].saved), base: clone(r[c.l].saved), basePlace: { place: c.l, name: r[c.l].saved.name }, hash: '', text: '' };
        renderList(); askCheck(r, other(c.l), () => render()); break;
      case 'del': state.confirm[pkey] = 1; re('[data-act=del-yes]'); break;
      case 'del-no': delete state.confirm[pkey]; re(); break;
      case 'del-yes': await doDelete(r, c.l); break;
      case 'discard': {
        delete state.confirm[pkey];
        if (r[c.l].saved) { r[c.l].draft = clone(r[c.l].saved); }
        else { delete r[c.l]; }
        if (!r.project && !r.global) { const i = roles.indexOf(r); if (i >= 0) roles.splice(i, 1); state.open.delete(r.uid); }
        renderList(); break;
      }
      case 'goto': {
        state.tab = 'assign'; render(); syncURL(true);
        const el = $(`.team[data-t="${b.dataset.t}"] .arow[data-m="${b.dataset.m}"]`);
        if (el) {
          el.scrollIntoView({ block: 'center' }); el.classList.add('flash');
          const s = el.querySelector('select'); if (s) s.focus({ preventScroll: true });
        }
        // The row a person came to is named in the URL, so a reload comes back to
        // it (spec-roles 2「割り当てのタブ」).
        history.replaceState(null, '', `${location.pathname}${location.search}#${b.dataset.t}.m-${b.dataset.i}`);
        break;
      }
      case 'role-hi': state.roleHi = state.roleHi === b.dataset.n ? null : b.dataset.n; render(); syncURL(false); break;
      case 't-save': await doAssignSave(b.closest('.team').dataset.t); break;
      case 'yfile': state.yfile = b.dataset.key; render(); syncURL(false); await ensureYAML(b.dataset.key); break;
      case 'yaml-jump': {
        const ta = b.closest('.yed').querySelector('.ytext'), line = +b.dataset.line;
        const lines = ta.value.split('\n'), start = lines.slice(0, line - 1).reduce((n2, s) => n2 + s.length + 1, 0);
        ta.focus({ preventScroll: true }); ta.setSelectionRange(start, start + (lines[line - 1] || '').length);
        ta.scrollTop = Math.max(0, (line - 4) * 20); ta.scrollLeft = 0;
        break;
      }
      case 'yaml-revert': {
        const el = b.closest('.yed'), m = /^(\d+)(project|global)$/.exec(el.dataset.key);
        const rr = byUid(m[1]), l = m[2], e = rr[l], name = e.saved ? e.saved.name : e.draft.name;
        // The text goes back to the saved one, so the checked text and its
        // findings go with it: a result for the thrown-away text must not stay.
        clearTimeout(timers['y' + el.dataset.key]);
        e.ytext = undefined;
        delete ychecks[ykey(l, name)];
        render();
        await askYAML(l, name, docText(rr, l), true);
        const sec = $(`.yed[data-key="${el.dataset.key}"]`);
        if (sec) refreshYed(sec);
        break;
      }
      case 'yaml-save': await saveYAML(b.closest('.yed').dataset.key); break;
    }
  });

  async function ensureYAML(key) {
    const m = /^(\d+)(project|global)$/.exec(key);
    if (!m) return;
    const r = byUid(m[1]), l = m[2], e = r[l];
    if (!e || !e.saved) return;
    const name = e.saved.name;
    if (ychecks[ykey(l, name)] === undefined) {
      await askYAML(l, name, docText(r, l), true);
      const sec = $(`.yed[data-key="${key}"]`);
      if (sec) refreshYed(sec);
    }
  }

  // ---- writes ----------------------------------------------------------
  // A refusal stands where its field does: a name's refusal under the name
  // field, the document's in the panel's footer (spec-roles 1.2.9).
  function clearRefusal(sec) {
    if (!sec) return;
    $$('.errline', sec).forEach((el) => { el.hidden = true; el.textContent = ''; });
  }

  function showRefusal(sec, json) {
    if (!sec) return;
    clearRefusal(sec);
    const el = sec.querySelector(json && json.field === 'name' ? '.errline.name' : '.errline.doc');
    if (el) { el.hidden = false; el.textContent = (json && json.text) || '保存できませんでした'; }
  }

  async function doSave(r, l) {
    const e = r[l], sec = $(`.panel[data-uid="${r.uid}"][data-l="${l}"]`);
    const keepRename = (() => { const cb = document.getElementById('rn' + r.uid + l); return cb ? cb.checked : true; })();
    const payload = { place: l, original: e.saved ? e.saved.name : null, doc: toDoc(e.draft, e.saved || e.base), renameUsers: keepRename, hash: e.hash || '' };
    if (e.basePlace) payload.base = e.basePlace;
    const res = await post('/roles/save', payload);
    if (!res.ok) {
      showRefusal(sec, res.json);
      return;
    }
    clearRefusal(sec);
    const old = e.saved;
    const newName = e.draft.name;
    e.saved = clone(e.draft);
    e.hash = (res.json && res.json.hash) || e.hash;
    // The written text comes back with the answer, so the YAML tab shows the file
    // this save just made (a moved line, a rule the write re-emitted).
    if (res.json && typeof res.json.text === 'string') e.text = res.json.text;
    delete e.base;
    delete e.basePlace;
    delete r.isNew;
    delete r.noDoc;
    if (old && old.name !== newName && keepRename) {
      teams.forEach((t) => t.members.forEach((m) => {
        if (m.saved === old.name) m.saved = newName;
        if (m.draft === old.name) m.draft = newName;
      }));
      const o = other(l);
      if (r[o]) { const split = { uid: ++uid, project: null, global: null }; split[o] = r[o]; delete r[o]; roles.splice(roles.indexOf(r) + 1, 0, split); }
    }
    e.ytext = undefined;
    ychecks[ykey(l, newName)] = undefined;
    state.ok[r.uid + l] = 1;
    await askCheck(r, l, null);
    render();
    // A rename changes the name the address opens the row by, so the address
    // follows the save (spec-roles 2 節).
    syncURL(false);
    await ensureYAML(keyOf(r, l));
  }

  async function doDelete(r, l) {
    const e = r[l];
    if (!e || !e.saved) return;
    const res = await post('/roles/delete', { place: l, name: e.saved.name, hash: e.hash || '' });
    if (!res.ok) { const sec = $(`.panel[data-uid="${r.uid}"][data-l="${l}"]`); showRefusal(sec, res.json); return; }
    ychecks[ykey(l, e.saved.name)] = undefined;
    delete r[l];
    if (!r.project && !r.global) { const i = roles.indexOf(r); if (i >= 0) roles.splice(i, 1); state.open.delete(r.uid); }
    state.tab = 'roles'; render(); syncURL(false);
  }

  async function doAssignSave(id) {
    const t = teams.find((x) => x.id === id);
    if (!t) return;
    const changes = t.members.filter((m) => m.draft !== m.saved).map((m) => ({ member: m.name, role: m.draft }));
    if (!changes.length) return;
    const res = await post('/roles/assign/save', { definition: id, changes, hash: t.hash || '' });
    if (!res.ok) {
      const card = $(`.team[data-t="${id}"]`);
      if (card) { let el = card.querySelector('.errline'); if (!el) { el = document.createElement('span'); el.className = 'errline'; card.querySelector('.th').appendChild(el); } el.hidden = false; el.textContent = (res.json && res.json.text) || '保存できませんでした'; }
      return;
    }
    t.members.forEach((m) => { m.saved = m.draft; });
    t.hash = (res.json && res.json.hash) || t.hash;
    state.ok['t' + id] = 1;
    await Promise.all(savedRoles().map((r) => { const l = eff(r); return askCheck(r, l, null); }));
    render();
  }

  // ---- 帯への報告と持ち越し (spec-nav.md 1.4, 1.5) ---------------------
  // dirty is whether the screen holds something no save wrote yet: a row's
  // draft, a row a person added, a pick in the assign tab, or a YAML text. The
  // band shows its dot while it is true, and carries the draft over when a
  // person walks to the other screen with the band.
  function rowDirty(r) {
    return PLACES.some((l) => r[l] && (!r[l].saved || JSON.stringify(r[l].draft) !== JSON.stringify(r[l].saved)));
  }

  function dirty() {
    const rows = roles.some(rowDirty);
    const members = teams.some((t) => t.members.some((m) => m.draft !== m.saved));
    const ytexts = allEntries().some(({ r, l }) => r[l].ytext !== undefined && r[l].ytext !== (r[l].text || ''));
    return rows || members || ytexts;
  }

  function tellDirty() {
    document.dispatchEvent(new CustomEvent('hergy:dirty', { detail: dirty() }));
  }

  // take is the draft the band puts aside: every row's draft that is not what
  // the file holds, every row a person added, every pick that was made, every
  // YAML text that was typed — each with the hash of the file it was read from,
  // so a file that moved outside is not written over by a page that read
  // another one.
  function take() {
    const rows = [], ytexts = [], members = [];
    roles.forEach((r) => {
      PLACES.forEach((l) => {
        const e = r[l];
        if (!e) return;
        if (!e.saved) {
          rows.push({ place: l, saved: null, name: e.draft.name, hash: e.hash || '', draft: e.draft, isNew: !!r.isNew, noDoc: !!r.noDoc });
        } else if (JSON.stringify(e.draft) !== JSON.stringify(e.saved)) {
          rows.push({ place: l, saved: e.saved.name, name: e.draft.name, hash: e.hash || '', draft: e.draft });
        }
        if (e.ytext !== undefined && e.ytext !== (e.text || '')) {
          ytexts.push({ place: l, name: (e.saved || e.draft).name, hash: e.hash || '', text: e.ytext });
        }
      });
    });
    teams.forEach((t) => t.members.forEach((m) => {
      if (m.draft !== m.saved) members.push({ team: t.id, hash: t.hash || '', name: m.name, saved: m.saved, draft: m.draft });
    }));
    return { rows, ytexts, members };
  }

  // restore puts the carried draft back, skipping every part whose file moved
  // outside since the draft was made (spec-nav.md 1.5).
  function restore(carried) {
    (carried.rows || []).forEach((c) => {
      const r = c.saved
        ? roles.find((x) => x[c.place] && x[c.place].saved && x[c.place].saved.name === c.saved)
        : roles.find((x) => x[c.place] && !x[c.place].saved && (x.noDoc || x.isNew || x[c.place].draft.name === c.name));
      if (!r || !r[c.place]) return;
      const e = r[c.place];
      if (c.hash && e.hash !== c.hash) return;
      e.draft = clone(c.draft);
      rulesOf(e.draft).forEach((x) => { if (x._k > kseq) kseq = x._k; });
      if (c.isNew) r.isNew = true;
      clearTimeout(timers[r.uid + c.place]);
      askCheck(r, c.place, null);
    });
    (carried.ytexts || []).forEach((c) => {
      const r = roles.find((x) => x[c.place] && (x[c.place].saved || x[c.place].draft).name === c.name);
      if (!r) return;
      const e = r[c.place];
      if (c.hash && e.hash !== c.hash) return;
      e.ytext = c.text;
    });
    (carried.members || []).forEach((c) => {
      const t = teams.find((x) => x.id === c.team);
      if (!t || (c.hash && t.hash !== c.hash)) return;
      const m = t.members.find((x) => x.name === c.name && x.saved === c.saved);
      if (m) m.draft = c.draft;
    });
    render();
  }

  window.hergyPage = { screen: 'roles', dirty, take };

  // ---- boot ------------------------------------------------------------
  function updateTabs() { /* counts are drawn by render() */ }

  async function boot() {
    parseURL();
    await Promise.all(roles.flatMap((r) => PLACES.filter((l) => r[l] && r[l].saved).map((l) => askCheck(r, l, () => { r[l].__savedIssues = (checks[r.uid + l] || {}).issues; }))));
    // remember each saved document's own problems for the save gate (X3)
    roles.forEach((r) => PLACES.forEach((l) => { if (r[l] && r[l].saved) { const c = checks[r.uid + l]; if (c) r[l].__savedIssues = c.issues; } }));
    // The band's promise: what one screen left unsaved is handed to this one,
    // and dropped whether or not it could be put back.
    const carried = window.hergyNav && window.hergyNav.takeDraft('roles');
    if (carried) restore(carried);
    render();
    if (state.yfile) await ensureYAML(state.yfile);
  }

  boot();
})();