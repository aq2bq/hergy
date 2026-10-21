// teams-v2.js — チーム定義の画面（r-0f8c, spec-teams 版 7）。一覧の行はその場で開き、
// 「トポロジー」（team.yaml を点と線で見て直す）と「ルール」（ロールの画面と同じ
// 共有の部品で agreement.yaml を直す）の 2 つのタブを持つ。一覧の上から定義を足し、
// 行ごとに複製・コピー・削除ができる。YAML のタブは両方の置き場所の team.yaml と
// agreement.yaml を文字で見せる。
//
// 見た目と図の置き方はモック（mock-teams.html 版 7）が正。図の配置 layout() と
// 札の置き場所 placeCard() はそこから逐語で写した。検査の文はすべてサーバ
// （check.Team / check.Agreement）が作るので、この画面は文を作らない。
(() => {
  'use strict';

  const view = document.getElementById('view');
  // The screen's tabs stand outside #view, so the press that changes the tab is
  // caught on the whole wrap (監査 R2).
  const wrapEl = document.querySelector('.wrap');
  if (!view) return;
  const DATA = JSON.parse(document.getElementById('teams-data').textContent);
  const API = window.hergyAPI;
  const RD = window.hergyRuleDoc;
  const YAML = window.hergyYaml;
  const PLACES = ['project', 'global'];
  const PLACE = DATA.places || { project: 'プロジェクト', global: '共通' };
  const DIR = DATA.dirs || { project: '', global: '' };
  const PROFILES = DATA.profiles || [];
  // The island is the same list answer GET /api/team gives (spec-boundary 5):
  // the items, and the files the YAML tab can open.
  const LIST = DATA.team || { items: [], files: [] };

  const $ = (s, root = document) => root.querySelector(s);
  const $$ = (s, root = document) => Array.from(root.querySelectorAll(s));
  const esc = RD.esc;
  const num = RD.num;
  const clone = RD.clone;
  const rulesOf = RD.rulesOf;
  const outNote = RD.outNote;
  const outCount = RD.outCount;
  const sevChip = RD.sevChip;
  const vmItem = RD.vmItem;
  const counts = RD.counts;
  const badge = RD.badge;

  // dkey is the key the screen's own state and DOM stand on: the definition's
  // saved name, or, for a definition that is not there yet (its name is what the
  // person is typing), a stable id of its own (spec-teams B2).
  const dkey = (d) => d.dom + '.' + d.place;
  const pairKey = (a, b) => [a, b].sort().join('|');
  const other = (p) => (p === 'project' ? 'global' : 'project');
  const herdr = (d, n) => d.name + '-' + n;
  const nodeName = (x) => (x === 'human' ? '人間' : x);
  const same = (a, b) => a && b && a.from === b.from && a.purpose === b.purpose && a.profile === b.profile;
  const hasTurn = (t, a, b) => (t.turns || []).some(([p, q]) => p === a && q === b);
  const memberOf = (t, n) => (t.members || []).find((m) => m.name === n);

  // ---- サーバとのやり取り ---------------------------------------------
  // Every check and every write goes through the boundary (spec-boundary 5):
  // this screen says which kind and carries its own part, and reads the answer
  // with the fields api.js lays out. It builds no payload shape of its own.
  const timers = {};
  const debounce = (k, fn, ms = 260) => { clearTimeout(timers[k]); timers[k] = setTimeout(fn, ms); };
  // A definition that is not there yet carries how it was made: the starter's own
  // files (new), or the definition it stands on (base). The server reads the base
  // from there and checks the row as if it were saved (spec-teams B2, B3).
  const draftIn = (r) => (!r.draftRow ? {} : r.how === 'new' ? { new: true } : { base: r.base });
  const teamIn = (r, extra) => Object.assign({ place: r.place, name: r.name, part: 'team' }, draftIn(r), extra || {});
  const agreeIn = (r, extra) => Object.assign({ place: r.place, name: r.name, part: 'agreement' }, draftIn(r), extra || {});

  // ---- 下書き（姿） ----------------------------------------------------
  const cloneTeam = (t) => ({
    mission: (t && t.mission) || '',
    members: ((t && t.members) || []).map((m) => ({ name: m.name, role: m.role, profile: m.profile })),
    workers: ((t && t.workers) || []).map((w) => ({ from: w.from, purpose: w.purpose, profile: w.profile })),
    turns: ((t && t.turns) || []).map((x) => (Array.isArray(x) ? [x[0], x[1]] : [x.from, x.to])),
  });
  function applyOps(saved, ops) {
    const t = cloneTeam(saved);
    for (const op of ops) {
      switch (op.kind) {
        case 'mission': t.mission = op.text; break;
        case 'addTurn': if (!hasTurn(t, op.from, op.to)) t.turns.push([op.from, op.to]); break;
        case 'removeTurn': t.turns = t.turns.filter(([a, b]) => !(a === op.from && b === op.to)); break;
        case 'addWorker': t.workers.push({ from: op.from, purpose: op.purpose, profile: op.profile }); break;
        case 'removeWorker': t.workers = t.workers.filter((w) => !(w.from === op.from && w.purpose === op.purpose)); break;
        case 'workerProfile': { const w = t.workers.find((w) => w.from === op.from && w.purpose === op.purpose); if (w) w.profile = op.profile; break; }
        case 'workerPurpose': { const w = t.workers.find((w) => w.from === op.from && w.purpose === op.old); if (w) w.purpose = op.new; break; }
        case 'memberRole': { const m = t.members.find((m) => m.name === op.member); if (m) m.role = op.role; break; }
        case 'memberProfile': { const m = t.members.find((m) => m.name === op.member); if (m) m.profile = op.profile; break; }
        default: break;
      }
    }
    return t;
  }

  const turnLabel = (d, t, b) => (b === 'human'
    ? '人間（ターンを渡したら、止まって人の答えを待つ）'
    : `${herdr(d, b)}（ロール ${(memberOf(t, b) || {}).role || '?'}）`);

  // ---- 記録（1 定義ごと） ---------------------------------------------
  const recs = new Map();
  let uids = 0;

  // defOf is one item of GET /api/team read as the record the screen draws: the
  // definition as one place reads it, the team's own agreement document, and the
  // number of problems the server counted for the row (api.js lays the list out,
  // so this screen reads no answer field by hand).
  function defOf(item) {
    const places = item.places || {};
    const eff = item.effective || (places.project ? 'project' : 'global');
    const place = places[eff] || {};
    const doc = place.doc || {};
    const agree = doc.agreement || { path: '', hash: '', has: false, doc: null };
    const key = (item.name || '') + '.' + eff;
    let r = recs.get(key);
    if (r) return r;
    let saved = null;
    if (agree.doc) {
      saved = RD.fromJSON(agree.doc);
      saved.__path = agree.path;
    }
    const hidden = (item.hidden || []).filter((p) => places[p]).map((p) => ({
      place: p, path: (places[p] || {}).path, hash: (places[p] || {}).hash,
    }));
    r = {
      name: item.name || '', dom: item.name || '', place: eff, places: PLACES.filter((p) => places[p]), key,
      teamPath: place.path || '', agreePath: agree.path || '',
      hash: place.hash || '', agreeHash: agree.hash || '', hasAgree: !!agree.has, broken: !!doc.broken,
      hidden, ins: doc.prompt || { turns: {}, workers: {} },
      ops: [], uid: ++uids,
      team: { saved: cloneTeam(doc), draft: cloneTeam(doc) },
      agree: {
        saved,
        draft: saved ? clone(saved) : RD.fromJSON({ name: '', title: (item.name || '') + ' チームのルール', revision: 1, sections: [] }),
        // The saved document's own problems are the agreement save's baseline:
        // a save stops only when an error the saved text did not hold has
        // appeared (spec-teams X3).
        savedIssues: null,
      },
      savedTurns: cloneTeam(doc).turns,
      findings: [], turnsFindings: [],
      problems: item.problems || {}, report: null, check: null, req: 0, areq: 0,
    };
    recs.set(key, r);
    return r;
  }
  const byKey = (k) => recs.get(k);
  const byName = (n) => [...recs.values()].find((r) => r.dom === n);
  // The list is drawn in the order the records stand in, so a row that is added
  // or read again is put back where a person was looking: firstIndexOf is the
  // place of the first record one rule names, and orderRows puts one row there
  // (spec-teams B1-B6).
  const firstIndexOf = (want) => {
    const all = [...recs.values()];
    for (let i = 0; i < all.length; i++) if (want(all[i])) return i;
    return -1;
  };
  function orderRows(r, at) {
    const all = [...recs.values()].filter((x) => x !== r);
    all.splice(Math.max(0, Math.min(at < 0 ? 0 : at, all.length)), 0, r);
    recs.clear();
    all.forEach((x) => recs.set(x.key, x));
  }
  const live = (n) => byName(n);

  // ---- まだ無い定義（B2, B3） ------------------------------------------
  // A definition that is not there yet is one row the screen holds rather than a
  // saved file: the name the person is typing, the place it would go to, the
  // definition it stands on, and what the server answered about it. The screen
  // builds no document of its own; it draws the check's own answer (spec-teams
  // B2, spec-boundary 5).
  function draftRecord(how, place, name, base) {
    const r = {
      dom: '~new' + (++uids), draftRow: true, how, base: base || null,
      name: name || '', place: place || 'project', places: [],
      key: '', teamPath: '', agreePath: '', hash: '', agreeHash: '', hasAgree: false, broken: false,
      hidden: [], ins: { turns: {}, workers: {} }, ops: [], uid: uids,
      team: { saved: cloneTeam(null), draft: cloneTeam(null) },
      agree: {
        saved: null,
        draft: RD.fromJSON({ name: '', title: 'チームのルール', revision: 1, sections: [] }),
        savedIssues: null,
      },
      savedTurns: [], findings: [], turnsFindings: [], problems: {}, report: null, check: null,
      req: 0, areq: 0, starter: false, seeded: false, writes: [], nameIssues: [],
      preview: { lines: [], diff: {} }, baseFindings: [],
    };
    r.key = dkey(r);
    recs.set(r.key, r);
    return r;
  }
  // seedDraft reads the first check of a definition that is not there yet into
  // the row: the base text its figure and its rules stand on, and where a save
  // would write it (spec-teams B2, B3).
  function seedDraft(r, json) {
    const doc = json.doc || {}, agree = doc.agreement || {};
    r.seeded = true;
    r.team.saved = cloneTeam(doc);
    r.team.draft = cloneTeam(doc);
    r.savedTurns = cloneTeam(doc).turns;
    r.broken = !!doc.broken;
    r.ins = doc.prompt || { turns: {}, workers: {} };
    r.teamPath = json.path || '';
    r.agreePath = agree.path || '';
    r.hash = json.hash || '';
    r.agreeHash = agree.hash || '';
    r.hasAgree = !!agree.has;
    r.starter = !!json.starter;
    if (agree.doc) { const d = RD.fromJSON(agree.doc); d.__path = agree.path; r.agree.saved = d; r.agree.draft = clone(d); }
  }
  // theName is the name a duplicate takes: the source's with -2, -3 … until one
  // is free, so the row starts without the name problem the same name would raise
  // (spec-teams B3).
  function theName(base) {
    const taken = new Set([...recs.values()].map((r) => r.name));
    for (let i = 2; i < 1000; i++) {
      const n = `${base}-${i}`;
      if (!taken.has(n) && n.length <= 25) return n;
    }
    return base;
  }
  // addDraft makes a row for a definition that is not there yet, checks it once,
  // and draws it: its figure, its rules and its preview are the server's own
  // reading of the base (spec-teams B2).
  async function addDraft(how, place, name, base) {
    const r = draftRecord(how, place, name, base);
    await checkTeam(r);
    // The row stands where the mock draws it: one added from the scaffold stands
    // first, and a duplicate or a copy stands right under the row it came from
    // (spec-teams B1, B3, B4).
    let at = 0;
    if (base) { const i = firstIndexOf((x) => x.name === base.name); at = i >= 0 ? i + 1 : 0; }
    orderRows(r, at);
    state.open.add(r.dom);
    state.dtab[r.dom] = 'shape';
    render();
    const el = $(`.tdef[data-name="${CSS.escape(r.dom)}"] .in-name`) || $(`.tdef[data-name="${CSS.escape(r.dom)}"] .sum`);
    // The name a copy already holds is chosen, so the first letter a person
    // types replaces it rather than landing beside it (監査 R8).
    if (el) { el.focus({ preventScroll: true }); if (el.select) el.select(); }
    return r;
  }
  // dropDraft takes a row that is not there yet away without writing anything
  // (spec-teams B2).
  function dropDraft(r) {
    recs.delete(dkey(r));
    state.open.delete(r.dom); delete state.dtab[r.dom]; delete state.gsel[dkey(r)];
    delete state.pmode['t' + dkey(r)]; delete state.confirm['del' + dkey(r)];
    render();
  }

  function ruleKeyAt(doc, si, ri) {
    const s = doc && doc.sections[si];
    if (!s) return 0;
    const x = s.rules[ri];
    return x ? x._k : 0;
  }
  const convProblems = (doc, list) => (list || []).map((p) => ({
    sev: p.sev, k: p.section >= 0 ? ruleKeyAt(doc, p.section, p.rule) : 0,
    f: p.field || '', msg: p.text || '', definition: p.definition || '',
  }));
  const buildCheck = (doc, json) => {
    const effect = json.effect || {};
    return {
      issues: convProblems(doc, json.problems || []),
      markdown: json.preview || { lines: [], chars: 0 },
      diff: effect.diff || { added: [], removedBefore: [], removedAtEnd: false },
      turns: effect.turns || [], readers: json.readers || [],
    };
  };

  // figFindings is the figure's problems as the panel draws them: the field the
  // server names ("mission", "members[1].profile", "workers[0]", "turns[2]", a
  // rule) becomes the node, the worker or the word a row stands by. The server
  // writes every sentence; this only says where it goes (spec-teams X1).
  function figFindings(r, problems) {
    const t = r.team.draft;
    return (problems || []).map((p) => {
      const at = p.at || p.field || '';
      let node = null, worker = null, field = at, m;
      if ((m = /^members\[(\d+)\]/.exec(at))) { const mem = t.members[Number(m[1])]; node = mem ? mem.name : null; field = 'members'; }
      else if ((m = /^workers\[(\d+)\]/.exec(at))) { worker = Number(m[1]); field = 'workers'; }
      // A sentence about a node ("node:analyst") stands beside that point: the
      // graph's own problems reach the card and the figure that way (監査 R9).
      else if (at.startsWith('node:')) { node = at.slice('node:'.length); field = 'node'; }
      else if (at === 'mission') { field = 'mission'; }
      else if (at === 'name') { field = 'name'; }
      else if (at.startsWith('rule:')) { field = at; }
      else { field = ''; }
      return { sev: p.sev, field, node, worker, text: p.text };
    });
  }
  const agreeIssues = (r) => (r.check ? r.check.issues : []);
  const agreeDiff = (r) => RD.diffCount(r.agree);
  const teamIssues = (r) => r.findings;
  const pcountList = (c) => [...Array((c && c.err) || 0)].map(() => ({ sev: 'err' })).concat([...Array((c && c.warn) || 0)].map(() => ({ sev: 'warn' })));
  // rowCounts is the problems a row's own chip shows: the server's count for a
  // saved definition, and the findings the server answered for a definition that
  // is not there yet (spec-teams B2).
  const rowCounts = (r) => (r.draftRow ? counts(teamIssues(r)) : counts(pcountList(r.problems)));
  // A definition that is not there yet counts as unsaved whatever it holds: it is
  // written only when 保存 is pressed (spec-teams B2).
  const dirtyDef = (r) => r.draftRow || changeCount(r) > 0 || (r.agree.saved ? agreeDiff(r) > 0 : false);

  // workerOrigin is the worker of the saved file a draft worker stands in, read
  // back through the ops that changed its purpose or profile, or null when the
  // draft added it: it is how a purpose typed one letter at a time still counts
  // as one worker (spec-teams 1.4.6).
  function workerOrigin(w, ops) {
    let from = w.from, purpose = w.purpose, profile = w.profile;
    for (let i = ops.length - 1; i >= 0; i--) {
      const op = ops[i];
      if (op.kind === 'workerPurpose' && op.from === from && op.new === purpose) purpose = op.old;
      else if (op.kind === 'workerProfile' && op.from === from && op.purpose === purpose) profile = op.profile;
      else if (op.kind === 'addWorker' && op.from === from && op.purpose === purpose) return null;
    }
    return { from, purpose, profile };
  }

  // changeCount is 1.4.6's `N か所`: the mission once, each turn that came in or
  // left, each worker added or taken away, and each worker whose purpose or
  // profile changed — one apiece, however many keystrokes wrote it.
  function changeCount(r) {
    const t = r.team.draft, s = r.team.saved, ops = r.ops || [];
    let n = t.mission !== s.mission ? 1 : 0;
    const key = (a, b) => a + '\x00' + b;
    const nowT = new Set((t.turns || []).map(([a, b]) => key(a, b)));
    const wasT = new Set((s.turns || []).map(([a, b]) => key(a, b)));
    nowT.forEach((k) => { if (!wasT.has(k)) n++; });
    wasT.forEach((k) => { if (!nowT.has(k)) n++; });
    const wasW = new Map((s.workers || []).map((w) => [key(w.from, w.purpose), w]));
    const claimed = new Set();
    (t.workers || []).forEach((w) => {
      const o = workerOrigin(w, ops);
      if (!o) { n++; return; }
      claimed.add(key(o.from, o.purpose));
      const sw = wasW.get(key(o.from, o.purpose));
      if (!sw) { n++; return; }
      if (w.purpose !== o.purpose || w.profile !== o.profile) n++;
    });
    wasW.forEach((sw, k) => { if (!claimed.has(k)) n++; });
    (t.members || []).forEach((m) => {
      const sm = (s.members || []).find((x) => x.name === m.name);
      if (sm && (sm.role !== m.role || sm.profile !== m.profile)) n++;
    });
    return n;
  }

  // ---- 姿：図の配置（モック 1.4.2 から逐語） ---------------------------
  // The figure's own width is the viewBox's width. It is set here, after the SVG
  // is drawn, because a style attribute on an SVG is what CSP stops (監査 R1):
  // the sheet's `width: min(100%, var(--gw))` then holds, so the figure is drawn
  // at its own size and the card has the empty room beside it to stand in.
  function fitFigure(panel) {
    const wrap = $('.gwrap', panel);
    if (!wrap) return;
    const svg = $('svg', wrap);
    if (!svg) return;
    const w = Number((svg.dataset.vb || '').split(' ')[2]);
    if (w) svg.style.setProperty('--gw', w + 'px');
  }
  const textW = (s, mono) => [...String(s)].reduce((w, c) => w + (/[\u3000-\u9fff\uff00-\uffef]/.test(c) ? 13 : mono ? 8.5 : 7.4), 0);
  const narrow = () => window.innerWidth <= 760;
  function layout(t) {
    const nodes = [{ id: 'human', kind: 'human', w: 72, h: 36 }, ...t.members.map((m) => ({ id: m.name, kind: 'member', role: m.role, w: Math.max(textW(m.name, true), textW(m.role)) + 30, h: 48 }))];
    const n = nodes.length, R = n <= 3 ? 140 : 112 + n * 14, thin = narrow();
    const rx = thin ? (n <= 3 ? 95 : 105) : R, ry = thin ? 70 + n * 34 : R * (n <= 3 ? 0.8 : 0.86);
    nodes.forEach((nd, i) => { const th = (-90 + i * 360 / n) * Math.PI / 180; nd.th = th; nd.x = rx * Math.cos(th); nd.y = ry * Math.sin(th); });
    const wn = [];
    t.members.forEach((m) => {
      const ws = t.workers.map((w, i) => ({ w, i })).filter((z) => z.w.from === m.name);
      const host = nodes.find((x) => x.id === m.name), k = ws.length;
      ws.forEach(({ w, i }, j) => {
        const label = w.purpose || '（目的）', ww = Math.max(textW(label), textW(w.profile, true) * 0.92) + 24;
        let x, y;
        if (thin) {
          const dir = host.y <= 0 ? -1 : 1;
          x = host.x + Math.sign(host.x || 1) * (18 + Math.abs(host.y) * 0.08); y = host.y + dir * (host.h / 2 + 34 + j * 46);
        } else {
          const k2 = Math.max(k, 1), spread = Math.min(0.42, 1.6 / k2), th = host.th + (j - (k2 - 1) / 2) * spread, r2 = 98 + (k2 > 3 ? (j % 2) * 34 : 0);
          x = host.x + r2 * Math.cos(th) * 1.15; y = host.y + r2 * Math.sin(th);
        }
        wn.push({ id: 'w' + i, kind: 'worker', wi: i, from: m.name, w: ww, h: 38, x, y, label, prof: w.profile });
      });
    });
    const all = [...nodes, ...wn];
    const xs = all.flatMap((z) => [z.x - z.w / 2, z.x + z.w / 2]), ys = all.flatMap((z) => [z.y - z.h / 2, z.y + z.h / 2]);
    const pad = 28;
    const minX = Math.min(...xs) - pad, maxX = Math.max(...xs) + pad, minY = Math.min(...ys) - pad - 10, maxY = Math.max(...ys) + pad;
    return { nodes, wn, all, byId: Object.fromEntries(all.map((z) => [z.id, z])), vb: [minX, minY, maxX - minX, maxY - minY] };
  }
  function edgePt(a, b) {
    const dx = b.x - a.x, dy = b.y - a.y, len = Math.hypot(dx, dy) || 1, ux = dx / len, uy = dy / len;
    const t = Math.min((a.w / 2 + 3) / Math.abs(ux || 1e-9), (a.h / 2 + 3) / Math.abs(uy || 1e-9));
    return { x: a.x + ux * t, y: a.y + uy * t };
  }
  function bdg(m, z) {
    const label = m.sev === 'err' ? '問題' : '警告', w = 34;
    return `<g class="bdg ${m.sev}" transform="translate(${(z.w / 2 - w / 2 - 2).toFixed(1)},${(-z.h / 2 - 6).toFixed(1)})"><rect x="${-w / 2}" y="-9" width="${w}" height="17" rx="3"></rect><text x="0" y="4" text-anchor="middle">${label}</text></g>`;
  }

  function graphSVG(r) {
    const t = r.team.draft, s = r.team.saved, L = layout(t), sel = state.gsel[dkey(r)] || null, issues = teamIssues(r);
    const byId = L.byId;
    const pk = new Map();
    const touch = (a, b, which) => { const k = pairKey(a, b); if (!pk.has(k)) { const [x, y] = k.split('|'); pk.set(k, { a: x, b: y, d: {}, s: {} }); } pk.get(k)[which][a + '>' + b] = true; };
    t.turns.forEach(([a, b]) => touch(a, b, 'd'));
    s.turns.forEach(([a, b]) => { if (byId[a] && byId[b]) touch(a, b, 's'); });
    const selKey = sel && sel.type === 'edge' ? sel.k : null;
    const focusNode = sel && sel.type === 'node' ? sel.id : null;
    const edges = [...pk.values()].map((p) => {
      const A = byId[p.a], Bn = byId[p.b];
      if (!A || !Bn) return '';
      const ab = p.a + '>' + p.b, ba = p.b + '>' + p.a;
      const now = [p.d[ab], p.d[ba]], was = [p.s[ab], p.s[ba]];
      const gone = !now[0] && !now[1];
      const added = !gone && ((now[0] && !was[0]) || (now[1] && !was[1]));
      const k = p.a + '|' + p.b, isSel = selKey === k;
      const dim = focusNode && p.a !== focusNode && p.b !== focusNode;
      const p1 = edgePt(A, Bn), p2 = edgePt(Bn, A);
      const mk = isSel ? 'a' : added || gone ? 'c' : 'n';
      const start = (gone ? was[1] : now[1]) ? `marker-start="url(#ar-${mk})"` : '', end = (gone ? was[0] : now[0]) ? `marker-end="url(#ar-${mk})"` : '';
      const dd = `M${p1.x.toFixed(1)},${p1.y.toFixed(1)} L${p2.x.toFixed(1)},${p2.y.toFixed(1)}`;
      return `<g><path class="hit" d="${dd}" data-act="edge" data-k="${k}" tabindex="0" role="button" aria-label="${nodeName(p.a)} と ${nodeName(p.b)} のターン"></path><path class="ge ${isSel ? 'sel' : ''} ${added ? 'add' : ''} ${gone ? 'gone' : ''} ${dim ? 'dim' : ''}" d="${dd}" ${start} ${end}></path></g>`;
    }).join('');
    const wedges = L.wn.map((w) => {
      const host = byId[w.from], p1 = edgePt(host, w), p2 = edgePt(w, host);
      const dim = focusNode && w.from !== focusNode;
      return `<path class="ge wk ${dim ? 'dim' : ''}" d="M${p1.x.toFixed(1)},${p1.y.toFixed(1)} L${p2.x.toFixed(1)},${p2.y.toFixed(1)}"></path>`;
    }).join('');
    const nodes = L.nodes.map((z) => {
      const is = issues.filter((m) => m.node === z.id), err = is.some((m) => m.sev === 'err');
      const dim = focusNode && focusNode !== z.id && !hasTurn(t, focusNode, z.id) && !hasTurn(t, z.id, focusNode);
      const label = z.kind === 'human' ? '<text class="nm" x="0" y="5" text-anchor="middle">人間</text>'
        : `<text class="nm" x="0" y="-3" text-anchor="middle">${esc(z.id)}</text><text class="rl" x="0" y="14" text-anchor="middle">${esc(z.role)}</text>`;
      return `<g class="gn ${z.kind} ${focusNode === z.id ? 'sel' : ''} ${err ? 'err' : ''} ${dim ? 'dim' : ''}" transform="translate(${z.x.toFixed(1)},${z.y.toFixed(1)})" data-act="node" data-id="${esc(z.id)}" tabindex="0" role="button" aria-label="${esc(nodeName(z.id))}">
      <rect class="box" x="${-z.w / 2}" y="${-z.h / 2}" width="${z.w}" height="${z.h}" rx="${z.kind === 'human' ? 18 : 6}"></rect>${label}${is.length ? bdg(is[0], z) : ''}</g>`;
    }).join('');
    const wnodes = L.wn.map((z) => {
      const is = issues.filter((m) => m.worker === z.wi), err = is.some((m) => m.sev === 'err');
      const isSel = sel && sel.type === 'worker' && sel.i === z.wi, dim = focusNode && z.from !== focusNode;
      const fresh = !s.workers.some((w) => same(w, t.workers[z.wi]));
      return `<g class="gn worker ${isSel ? 'sel' : ''} ${err ? 'err' : ''} ${dim ? 'dim' : ''} ${fresh ? 'newer' : ''}" transform="translate(${z.x.toFixed(1)},${z.y.toFixed(1)})" data-act="worker" data-i="${z.wi}" tabindex="0" role="button" aria-label="${esc(z.from)} の ${esc(z.label)}">
      <rect class="box" x="${-z.w / 2}" y="${-z.h / 2}" width="${z.w}" height="${z.h}" rx="5"></rect>
      <text class="nm" x="0" y="-2" text-anchor="middle">${esc(z.label)}</text><text class="rl" x="0" y="13" text-anchor="middle">${esc(z.prof)}</text>${is.length ? bdg(is[0], z) : ''}</g>`;
    }).join('');
    const plus = focusNode ? L.nodes.filter((z) => z.id !== focusNode && !hasTurn(t, focusNode, z.id)).map((z) => {
      const p = edgePt(z, byId[focusNode]);
      return `<g class="plus" transform="translate(${p.x.toFixed(1)},${p.y.toFixed(1)})" data-act="turn-add" data-a="${esc(focusNode)}" data-b="${esc(z.id)}" tabindex="0" role="button" aria-label="${esc(nodeName(focusNode))} から ${esc(nodeName(z.id))} へ渡す"><circle r="11"></circle><text x="0" y="5" text-anchor="middle">+</text></g>`;
    }).join('') : '';
    const [x, y, w, h] = L.vb;
    const mi = dkey(r).replace('.', '-');
    const marker = (id, cls) => `<marker id="ar-${mi}-${id}" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7.5" markerHeight="7.5" orient="auto-start-reverse"><path class="mk-${cls}" d="M0,0 L10,5 L0,10 z"></path></marker>`;
    const svg = `<svg viewBox="${x.toFixed(0)} ${y.toFixed(0)} ${w.toFixed(0)} ${h.toFixed(0)}" data-vb="${[x, y, w, h].map((v) => v.toFixed(0)).join(' ')}" role="group" aria-label="${esc(teamLabel(r))} のトポロジー">
    <defs>${marker('n', 'n')}${marker('a', 'a')}${marker('c', 'c')}${marker('w', 'w')}</defs>
    ${wedges}${edges}${nodes}${wnodes}${plus}</svg>`;
    return svg.replace(/url\(#ar-(\w)\)/g, (m, c) => `url(#ar-${mi}-${c})`);
  }

  // ---- 押した点・線の札 ------------------------------------------------
  // The card shows what one more op would move and what it would add; the
  // sentences come from the server (check.TeamEffect, C4), so neither this file
  // nor the card writes one (spec-teams X1).
  let effectSeq = 0;
  async function askEffect(r, op, pk, after) {
    const id = ++effectSeq;
    const res = await API.check('team', teamIn(r, { doc: { ops: r.ops, op } }));
    if (id !== effectSeq || !res.ok) return;
    const effect = API.answer(res).effect || {};
    state.effect[pk] = { lines: effect.lines || [], problems: figFindings(r, effect.problems || []) };
    if (after) after();
  }
  // The effect's lines are the prompt's own (spec-teams 1.4.4, X3): the screen
  // groups them under the member and part they stand for and writes no sentence
  // of its own. What the change takes away is struck through, once; what comes
  // in is marked. The sentences the change would add are the server's problems.
  const effectOf = (pk, member, part) => {
    const e = state.effect[pk];
    if (!e) return null;
    const hit = (e.lines || []).filter((l) => l.member === member && (!part || l.part === part));
    return {
      removed: hit.flatMap((l) => l.removed || []),
      added: hit.flatMap((l) => l.added || []),
      problems: e.problems || [],
    };
  };
  const effLinesHTML = (ef) => (!ef ? ''
    : (ef.removed || []).map((x) => `<s>${esc(x)}</s>`).join('') + (ef.added || []).map((x) => `<ins>${esc(x)}</ins>`).join(''));
  const effProblemsHTML = (ef) => (ef && ef.problems.length
    ? `<ul class="vm">${ef.problems.map((m) => vmItem({ sev: m.sev, msg: m.text })).join('')}</ul>` : '');
  // vmList is the check's own sentences of a card: the finding's text, not the
  // finding (its field and where it stands are the panel's, not the sentence's).
  const vmList = (ms) => (ms.length ? `<ul class="vm">${ms.map((m) => vmItem({ sev: m.sev, msg: m.text })).join('')}</ul>` : '');

  function dirHTML(r, a, b) {
    const d = r, t = r.team.draft, s = r.team.saved, here = hasTurn(t, a, b), was = hasTurn(s, a, b);
    const pl = a === 'human' ? '' : `<div class="gpl"><span class="pk">${esc(herdr(d, a))} · ターンを渡す相手</span>${was ? `<span class="ln2">- ${esc(turnLabel(d, t, b))}</span>` : `<ins>- ${esc(turnLabel(d, t, b))}</ins>`}</div>`;
    if (!here) return `<div class="gdir"><div class="gdl"><code>${esc(nodeName(a))} → ${esc(nodeName(b))}</code><span class="tag">消した</span><span class="acts"><button class="btn sm" data-act="turn-add" data-a="${esc(a)}" data-b="${esc(b)}">戻す</button></span></div></div>`;
    const pk = `t:${dkey(d)}:${a}>${b}`, asking = state.confirm[pk];
    return `<div class="gdir">
    <div class="gdl"><code>${esc(nodeName(a))} → ${esc(nodeName(b))}</code>${!was ? '<span class="tag">足した</span>' : ''}
      <span class="acts">${asking ? `<button class="btn danger solid sm" data-act="turn-del" data-a="${esc(a)}" data-b="${esc(b)}">消す</button><button class="btn sm" data-act="turn-ask-no" data-pk="${esc(pk)}">やめる</button>` : `<button class="btn danger sm" data-act="turn-ask" data-a="${esc(a)}" data-b="${esc(b)}" data-pk="${esc(pk)}">消す</button>`}</span></div>
    ${asking ? (a === 'human' ? '' : `<div class="gpl"><span class="pk">${esc(herdr(d, a))} · ターンを渡す相手</span>${effLinesHTML(effectOf(pk, herdr(d, a), 'turns'))}</div>`) + effProblemsHTML(effectOf(pk, herdr(d, a), 'turns')) : pl}
  </div>`;
  }

  function cardHTML(r) {
    const sel = state.gsel[dkey(r)];
    if (!sel) return '';
    const t = r.team.draft, s = r.team.saved;
    if (sel.type === 'edge') {
      const [a, b] = sel.k.split('|');
      const dirs = [[a, b], [b, a]].filter(([p, q]) => hasTurn(t, p, q) || hasTurn(s, p, q));
      return `<div class="gcard" data-sel="edge" data-k="${esc(sel.k)}">
      <div class="gch"><code>${esc(nodeName(a))}</code><span class="ar">${dirs.length > 1 ? '⇄' : '→'}</span><code>${esc(nodeName(b))}</code><button class="x" data-act="gclose" aria-label="閉じる">×</button></div>
      ${dirs.map(([p, q]) => dirHTML(r, p, q)).join('')}</div>`;
    }
    if (sel.type === 'worker') {
      const w = t.workers[sel.i];
      if (!w) return '';
      const is = teamIssues(r).filter((m) => m.worker === sel.i), pk = `w:${dkey(r)}:${sel.i}`, asking = state.confirm[pk];
      const profs = (PROFILES.includes(w.profile) ? [] : [w.profile]).concat(PROFILES);
      return `<div class="gcard" data-sel="worker" data-i="${sel.i}">
      <div class="gch"><code>${esc(w.from)}</code><span class="ar">→</span><span class="rl">ワーカー</span><button class="x" data-act="gclose" aria-label="閉じる">×</button></div>
      <div class="wrow"><input class="in-purpose" data-i="${sel.i}" value="${esc(w.purpose)}" placeholder="目的" autocomplete="off" aria-label="目的">
        <span class="sel"><select class="pickp" data-act-change="worker-prof" data-i="${sel.i}" aria-label="プロファイル">${profs.map((p) => `<option ${p === w.profile ? 'selected' : ''}>${esc(p)}</option>`).join('')}</select></span></div>
      ${vmList(is)}
      <div class="gdir"><div class="gpl"><span class="pk">${esc(herdr(r, w.from))} · ワーカーの追加</span>${asking ? effLinesHTML(effectOf(pk, herdr(r, w.from), 'workers')) : `<span class="ln2">${esc(w.purpose || '（目的）')} → ${esc(w.profile)}</span>`}</div>
        ${asking ? effProblemsHTML(effectOf(pk, herdr(r, w.from), 'workers')) : ''}
        <div class="gdl"><span class="acts">${asking ? `<button class="btn danger solid sm" data-act="worker-del" data-i="${sel.i}">消す</button><button class="btn sm" data-act="turn-ask-no" data-pk="${esc(pk)}">やめる</button>` : `<button class="btn danger sm" data-act="turn-ask" data-pk="${esc(pk)}">消す</button>`}</span></div></div>
    </div>`;
    }
    const id = sel.id, m = memberOf(t, id), outs = t.turns.filter(([p]) => p === id).map(([, q]) => q);
    const gone = s.turns.filter(([p, q]) => p === id && !hasTurn(t, p, q)).map(([, q]) => q);
    const cand = ['human', ...t.members.map((x) => x.name)].filter((x) => x !== id && !outs.includes(x));
    const ws = t.workers.map((w, i) => ({ w, i })).filter((z) => z.w.from === id);
    const is = teamIssues(r).filter((x) => x.node === id);
    return `<div class="gcard" data-sel="node" data-id="${esc(id)}">
      <div class="gch"><code>${esc(nodeName(id))}</code><button class="x" data-act="gclose" aria-label="閉じる">×</button></div>
      ${m ? `<div class="links"><span class="uchip">ロール ${esc(m.role)}</span><span class="uchip">プロファイル ${esc(m.profile)}</span></div>` : ''}
      ${vmList(is)}
      <div class="tl"><span class="gsub">${id === 'human' ? '人間が渡す相手' : `${esc(herdr(r, id))} · ターンを渡す相手`}</span>
        ${outs.map((q) => `<div class="row ${hasTurn(s, id, q) ? '' : 'new'}"><span>- ${esc(id === 'human' ? nodeName(q) : turnLabel(r, t, q))}</span><button class="x" data-act="pick-edge" data-a="${esc(id)}" data-b="${esc(q)}" aria-label="${esc(nodeName(q))} へのターン">›</button></div>`).join('')}
        ${gone.map((q) => `<div class="row"><span><s>- ${esc(nodeName(q))}</s></span><span class="tag">消した</span><button class="x" data-act="turn-add" data-a="${esc(id)}" data-b="${esc(q)}" aria-label="戻す">↺</button></div>`).join('')}
        ${cand.length ? `<span class="sel"><select class="pickp" data-act-change="turn-add-sel" data-a="${esc(id)}" aria-label="渡す相手を足す"><option value="">＋ 渡す相手</option>${cand.map((c) => `<option value="${esc(c)}">${esc(nodeName(c))}</option>`).join('')}</select></span>` : ''}
      </div>
      ${m ? `<div class="tl"><span class="gsub">${esc(herdr(r, id))} · ワーカーの追加</span>
        ${ws.map(({ w, i }) => `<div class="row"><span>${esc(w.purpose || '（目的）')} → ${esc(w.profile)}</span><button class="x" data-act="worker" data-i="${i}" aria-label="${esc(w.purpose)}">›</button></div>`).join('')}
        <button class="add sm" data-act="worker-add" data-from="${esc(id)}" style="justify-self:start">＋ ワーカー</button></div>` : ''}
    </div>`;
  }

  // 札を、押した点・線の横に置く。点を隠さない場所を、右・左・下・上・図の外の右から選ぶ。
  // 版 7: 右に YAML が並ぶと図の横が狭くなるので、どれも点を隠すなら図のすぐ下に置く（B8）
  function placeCard(panel) {
    fitFigure(panel);
    const card = $('.gcard', panel), wrap = $('.gwrap', panel), svg = wrap ? $('svg', wrap) : null;
    if (!card || !svg) return;
    card.classList.remove('below');
    if (getComputedStyle(card).position !== 'absolute') return;
    const [vx, vy, vw, vh] = svg.dataset.vb.split(' ').map(Number), sc = svg.clientWidth / vw;
    const d = byKey(panel.dataset.key), L = layout(d.team.draft);
    const sel = state.gsel[dkey(d)];
    if (!sel) return;
    let ax, ay, aw = 6, ah = 6;
    if (sel.type === 'edge') { const [a, b] = sel.k.split('|'); if (!L.byId[a] || !L.byId[b]) return; ax = (L.byId[a].x + L.byId[b].x) / 2; ay = (L.byId[a].y + L.byId[b].y) / 2; }
    else { const z = sel.type === 'node' ? L.byId[sel.id] : L.byId['w' + sel.i]; if (!z) return; ax = z.x; ay = z.y; aw = z.w / 2; ah = z.h / 2; }
    const svg0 = svg.getBoundingClientRect(), wrap0 = wrap.getBoundingClientRect();
    const ox = svg0.left - wrap0.left, oy = svg0.top - wrap0.top;
    const toPx = (x, y) => [ox + (x - vx) * sc, oy + (y - vy) * sc];
    const [px, py] = toPx(ax, ay), W = wrap.clientWidth, H = Math.max(wrap.clientHeight, 200), cw = card.offsetWidth, ch = card.offsetHeight, g = 16;
    const boxes = L.all.map((z) => { const [x, y] = toPx(z.x, z.y); return [x - z.w / 2 * sc, y - z.h / 2 * sc, x + z.w / 2 * sc, y + z.h / 2 * sc]; });
    const clampX = (x) => Math.max(8, Math.min(x, W - cw - 8)), clampY = (y) => Math.max(8, Math.min(y, Math.max(8, H - ch - 8)));
    const cands = [
      [px + aw * sc + g, py - 30], [px - aw * sc - g - cw, py - 30],
      [px - cw / 2, py + ah * sc + g], [px - cw / 2, py - ah * sc - g - ch],
      [ox + svg.clientWidth + 12, py - 30], [ox - cw - 12, py - 30],
    ].map(([x, y]) => [clampX(x), clampY(y)]);
    const cover = ([x, y]) => boxes.reduce((s2, [l, t, rr, bb]) => s2 + Math.max(0, Math.min(rr, x + cw) - Math.max(l, x)) * Math.max(0, Math.min(bb, y + ch) - Math.max(t, y)), 0)
      + (px > x - 4 && px < x + cw + 4 && py > y - 4 && py < y + ch + 4 ? 1e6 : 0);
    const best = cands.map((c) => [c, cover(c) + Math.hypot(c[0] + cw / 2 - px, c[1] + ch / 2 - py) * 2]).sort((a, b) => a[1] - b[1])[0][0];
    // 版 7: どの置き場所も点を隠すなら、札は図のすぐ下に置く（B8）
    if (cover(best) > 0) { card.classList.add('below'); return; }
    card.style.left = best[0] + 'px'; card.style.top = best[1] + 'px';
  }

  // ownAgreement is whether a rule finding is about the team's own agreement:
  // those are the team's promises and stand under チームのルール, while the rules of
  // the role documents its members read stand under ロールの文書 (spec-teams 1.4.5,
  // 1.7, 監査 R9).
  const ownAgreement = (r, m) => {
    const path = (m.field || '').replace(/^rule:/, '').split('#')[0];
    return !!r.agreePath && path.endsWith('/teams/' + r.name + '/agreement.yaml');
  };
  const ruleID = (m) => ((m.field || '').split('#')[1] || '');

  function vlistHTML(r) {
    const issues = teamIssues(r);
    const at = (m) => m.field && m.field.startsWith('rule:');
    const out = issues.filter((m) => at(m) && !ownAgreement(r, m));
    const teamOut = issues.filter((m) => at(m) && ownAgreement(r, m));
    const own = issues.filter((m) => !at(m));
    const where = (m) => (m.node ? nodeName(m.node) : m.worker != null && m.worker >= 0 ? ((r.team.draft.workers[m.worker] || {}).purpose || 'ワーカー') : m.field === 'mission' ? '使命' : m.field === 'name' ? '名前' : '');
    const file = (m) => esc((m.field.split('#')[0] || '').replace(/^rule:/, '').split('/').pop());
    return `<div class="vhead"><span class="vt">validate</span>${issues.length ? `<span class="vcount">${counts(issues)}</span>` : '<span class="vclean">✓ 問題なし</span>'}</div>`
      + own.map((m) => `<button class="vitem" data-act="vpick" ${m.node ? `data-node="${esc(m.node)}"` : ''} ${m.worker != null && m.worker >= 0 ? `data-worker="${m.worker}"` : ''} ${m.field === 'name' ? 'data-name-field="1"' : ''}>${sevChip(m.sev)}<span class="lno">${esc(where(m))}</span><span class="msg">${esc(m.text)}</span></button>`).join('')
      + (teamOut.length ? `<div class="vsub">チームのルール</div>` + teamOut.map((m) => `<button class="vitem" data-act="vrule" data-name="${esc(r.dom)}" data-id="${esc(ruleID(m))}">${sevChip(m.sev)}<span class="lno">${file(m)}</span><span class="msg">${esc(m.text)}</span></button>`).join('') : '')
      + (out.length ? `<div class="vsub">ロールの文書</div>` + out.map((m) => `<a class="vitem" href="/roles">${sevChip(m.sev)}<span class="lno">${file(m)}</span><span class="msg">${esc(m.text)}</span></a>`).join('') : '');
  }

  // teamPanelHTML is the トポロジー tab: the figure, the check column, and, beside
  // them, the team.yaml the row would write, line by line (spec-teams B7). The
  // 編集 / YAML seg picks which of the two the narrow places show.
  function teamPanelHTML(r) {
    const k = dkey(r), mode = state.pmode['t' + k] || 'edit';
    const head = `<header class="ph"><code class="path">${esc(r.teamPath)}</code>
      <span class="phr">${(r.places || []).length > 1 ? `<span class="state ${r.place === 'global' ? 'off' : 'on'}">${r.place === 'global' ? '隠れている' : '効いている'}</span>` : ''}
        <span class="seg pview">${[['edit', '編集'], ['yaml', 'YAML']].map(([v, label]) => `<button class="${mode === v ? 'on' : ''}" data-act="tmode" data-v="${v}" aria-pressed="${mode === v}">${label}</button>`).join('')}</span></span></header>`;
    // A team.yaml that does not read draws no figure and offers no shape save:
    // the check column alone says what stopped it, and the YAML tab is where the
    // text is put right (spec-teams 1.9).
    if (r.broken) return `<section class="panel team-p" data-key="${k}" data-mode="${mode}">
    ${head}
    <div class="gbody"><div class="vlist">${vlistHTML(r)}</div></div>
  </section>`;
    return `<section class="panel team-p" data-key="${k}" data-mode="${mode}">
    ${head}
    <div class="pmain">
      <div class="pedit"><div class="gbody">
        <div class="gwrap"><div class="gscroll">${graphSVG(r)}</div>${cardHTML(r)}</div>
        <div class="vlist">${vlistHTML(r)}</div>
      </div></div>
      ${tprevHTML(r)}
    </div>
    ${footHTML(r)}
  </section>`;
  }

  // yamlHL is the YAML a preview line shows, with the keys and the arrows marked
  // (モック 版 7 から逐語).
  const yamlHL = (t) => {
    let m = t.match(/^(\s*)(- )?([A-Za-z_]+):( .*)?$/);
    if (m) return `${m[1]}${m[2] ? '<span class="ar">- </span>' : ''}<span class="k">${m[3]}:</span>${esc(m[4] || '')}`;
    m = t.match(/^(\s*- )(\S+) -> (\S+)$/);
    if (m) return `<span class="ar">${m[1]}</span>${esc(m[2])}<span class="ar"> -> </span>${esc(m[3])}`;
    return esc(t);
  };
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
  const isCurLine = (z, cur) => {
    if (!cur || !z || !z.at) return false;
    const p = atParts(z.at);
    if (cur.type === 'node') return p.node === cur.id || p.a === cur.id || p.b === cur.id;
    if (cur.type === 'edge') return !!p.a && pairKey(p.a, p.b) === cur.k;
    if (cur.type === 'worker') return p.worker === cur.i;
    return false;
  };
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
  // tprevHTML is the トポロジー tab's team.yaml: the text a save would write, line
  // by line, with the lines one edit changed marked. The lines, what each stands
  // for and the marks all come from the server (spec-teams B7, spec-boundary 5).
  function tprevHTML(r) {
    const key = dkey(r);
    const pv = r.preview || { lines: [], diff: {} }, at = pv.lines || [], df = pv.diff || {};
    const added = df.added || [], removed = df.removedBefore || [], lastDel = !!df.removedAtEnd;
    const lines = at.map((z) => z.text);
    const prev = state.tlast[key];
    // The first answer for a row is not a move: the text was not drawn before,
    // so the row's whole file is not marked as changed (監査 R1).
    if (prev && prev.length && prev.join('\n') !== lines.join('\n')) { state.tstep[key] = [...stepOf(prev, lines)]; state.tglow[key] = Date.now(); }
    state.tlast[key] = lines;
    const step = new Set(state.tstep[key] || []);
    const marks = lineMarks(r), cur = state.gsel[key], n = at.length;
    const body = at.map((z, i) => {
      const p = atParts(z.at);
      const go = !!p.node || p.worker != null || !!p.a || p.f === 'mission';
      const cls = [marks[z.at] === 'err' ? 'err' : marks[z.at] ? 'warn' : '', added.includes(i) ? 'ins' : '', removed.includes(i) ? 'del' : '', lastDel && i === n - 1 ? 'dele' : '', isCurLine(z, cur) ? 'cur' : '', step.has(i) ? 'step' : '', go ? 'go' : ''].filter(Boolean).join(' ');
      const data = p.node ? `data-node="${esc(p.node)}"` : p.worker != null ? `data-worker="${p.worker}"` : p.a ? `data-a="${esc(p.a)}" data-b="${esc(p.b)}"` : p.f ? `data-f="${p.f}"` : '';
      return `<div class="yl ${cls}"${go ? ` data-act="tyline" ${data}` : ''}><span class="lno">${i + 1}</span><span class="yt">${yamlHL(z.text) || ' '}</span></div>`;
    }).join('');
    const issues = teamIssues(r), nadd = added.length, ndel = removed.length + (lastDel ? 1 : 0);
    return `<aside class="yprev tprev" aria-label="team.yaml"><div class="yph"><span class="pkl">YAML</span><span class="yinfo"><span class="vt">${n} 行</span>${nadd || ndel ? `<span class="dirty">+${nadd} −${ndel}</span>` : ''}<span class="yr">${issues.length ? `<span class="vcount">${counts(issues)}</span>` : '<span class="vclean">✓ 問題なし</span>'}</span></span></div><div class="ybox">${body}</div></aside>`;
  }
  // fitYbox caps the topology tab's YAML box at the room between the box's top
  // and the save's band, which stands at the bottom of the window while the
  // panel is taller than the window (spec-teams B7, 監査 R2). Without it the box
  // runs on under the band, and the line a person just moved stands where the
  // band hides it. The screen keeps it because where the band stands is only
  // known once the panel is drawn; it is a cap, so a box with less text is left
  // as it is.
  function fitYbox(panel) {
    const box = $('.tprev', panel);
    if (!box) return;
    if (getComputedStyle(box).display === 'none') { box.style.maxHeight = ''; return; }
    const foot = $('.pf', panel), fh = foot ? foot.offsetHeight : 0;
    const h = Math.max(160, Math.floor(window.innerHeight - fh - box.getBoundingClientRect().top - 8));
    if (box.style.maxHeight !== h + 'px') box.style.maxHeight = h + 'px';
  }
  function fitYboxes(root) {
    $$('.panel.team-p', root || view).forEach(fitYbox);
  }
  // afterTPrev brings the lines a person is looking at into the box (spec-teams
  // B7): the lines one edit just moved after an edit, and the lines the figure
  // has picked after a pick. It moves the box's own scroll and nothing else, so
  // opening a row leaves the page where it stands (監査 R1); the box itself
  // stops above the save's band (監査 R2).
  function afterTPrev(r) {
    const key = dkey(r);
    const panel = $(`.panel.team-p[data-key="${CSS.escape(key)}"]`);
    if (!panel) return;
    const box = $('.ybox', panel);
    if (!box || !box.clientHeight) return;
    const at = (r.preview || { lines: [] }).lines || [];
    const step = state.tstep[key] || [];
    const df = (r.preview || {}).diff || {};
    // The lines a change moved are the glow's own step, and also the lines the
    // server's diff names — the ones it added, and the place it took one from
    // (spec-teams B7). A step the glow marks alone can miss a removal the added
    // line's shift hides (監査 R2), so both are read, and the last of them is
    // where the box is sent.
    let want = -1;
    [...step, ...(df.added || []), ...(df.removedBefore || []), ...(df.removedAtEnd ? [at.length - 1] : [])]
      .forEach((i) => { if (i > want && i < at.length) want = i; });
    if (want < 0) { const cur = state.gsel[key]; if (cur) { for (let i = 0; i < at.length; i++) if (isCurLine(at[i], cur)) { want = i; break; } } }
    if (want >= 0 && box.children[want]) {
      const br = box.getBoundingClientRect(), lr = box.children[want].getBoundingClientRect();
      if (lr.top < br.top) box.scrollTop -= br.top - lr.top;
      else if (lr.bottom > br.bottom) box.scrollTop += lr.bottom - br.bottom;
    }
    if (state.tglow[key]) {
      const glowed = state.tglow[key];
      clearTimeout(state.tglowTimer && state.tglowTimer[key]);
      state.tglowTimer = state.tglowTimer || {};
      state.tglowTimer[key] = setTimeout(() => {
        if (Date.now() - glowed < 1700) return;
        $$('.yl.step', box).forEach((el) => el.classList.remove('step'));
      }, 1800);
    }
  }

  // footHTML is the shape panel's foot: the save of the ops, the count 1.4.6
  // counts, and 元に戻す while there is something to put back. A definition that
  // is not there yet shows what its save would write, and no 元に戻す: there is
  // nothing saved to go back to (spec-teams B2).
  function footHTML(r) {
    const n = changeCount(r), k = dkey(r);
    const ok = state.ok['t' + k] ? (delete state.ok['t' + k], '<span class="ok">保存しました</span>') : '';
    // The gate is the saved team.yaml's own errors (spec-teams 1.3, X3): a save
    // stops only when an error the file did not hold has appeared.
    const issues = teamIssues(r);
    const before = new Set((r.baseFindings || []).filter((m) => m.sev === 'err').map((m) => m.text));
    const grew = issues.some((m) => m.sev === 'err' && !before.has(m.text));
    if (r.draftRow) {
      // A row that is not there yet is saved only when its name is one the
      // server will take: the name's problems are the answer's own, and while
      // the first answer is still out nothing is written from the row
      // (spec-teams B2, N2).
      const named = r.seeded && !(r.nameIssues || []).length;
      return `<footer class="pf">
      <button class="btn primary" data-act="t-save" ${grew || !named ? 'disabled' : ''}>保存</button><span class="dirty">新規</span>${ok}
      <span class="also">${(r.writes || []).map((p) => `<code>${esc(String(p).split('/').pop())}</code>`).join('・')}</span>
    </footer>`;
    }
    return `<footer class="pf">
      <button class="btn primary" data-act="t-save" ${!n || grew ? 'disabled' : ''}>保存</button><span class="dirty">${n ? `${n} か所` : ''}</span>${ok}
      <span class="acts"><button class="btn" data-act="t-revert" ${n ? '' : 'hidden'}>元に戻す</button></span>
    </footer>`;
  }

  // ---- ルールのパネル（ロールの画面と同じ共有の部品） --------------------
  function agreePanelHTML(r) {
    const e = r.agree, both = (r.places || []).length > 1, shadowed = r.place === 'global' && (r.places || []).includes('project');
    const mode = state.pmode[dkey(r)] || 'edit';
    const ok = state.ok['a' + dkey(r)] ? (delete state.ok['a' + dkey(r)], '<span class="ok">保存しました</span>') : '';
    return `<section class="panel agree-p ${shadowed ? 'shadowed' : ''}" data-uid="${r.uid}" data-l="${r.place}" data-key="${dkey(r)}" data-mode="${mode}">
    <header class="ph"><code class="path">${esc(r.agreePath)}</code>
      <span class="phr">
        ${both ? `<span class="state ${shadowed ? 'off' : 'on'}">${shadowed ? '隠れている' : '効いている'}</span>` : ''}
        <span class="seg pview">${[['edit', '編集'], ['md', 'Markdown']].map(([v, label]) => `<button class="${mode === v ? 'on' : ''}" data-act="pmode" data-v="${v}" aria-pressed="${mode === v}">${label}</button>`).join('')}</span>
      </span></header>
    <div class="pmain">${RD.body(rowOfPanel(r))}</div>
    <footer class="pf">
      <button class="btn primary" data-act="save">保存</button><span class="errline doc" hidden></span><span class="dirty"></span><span class="vcount"></span>${ok}
      <span class="acts"><button class="btn" data-act="revert" ${agreeDiff(r) > 0 ? '' : 'hidden'}>元に戻す</button></span>
    </footer></section>`;
  }
  function rowOfPanel(r) {
    const e = r.agree;
    return {
      uid: r.uid, l: r.place, e, saved: e.saved, draft: e.draft,
      issues: agreeIssues(r),
      savedIssues: e.savedIssues,
      markdown: r.check ? r.check.markdown : { lines: [], chars: 0 },
      diff: r.check ? r.check.diff : { added: [], removedBefore: [], removedAtEnd: false },
      turns: r.check ? r.check.turns : [],
    };
  }
  const markPanel = (panel) => {
    if (!panel) return;
    RD.mark(panel);
    const r = byKey(panel.dataset.key);
    const rev = $('[data-act="revert"]', panel);
    if (r && rev) rev.hidden = !(agreeDiff(r) > 0);
  };

  // ---- 一覧の行 --------------------------------------------------------
  // teamLabel is the word the screen calls a row by: its name, or, while a
  // definition that is not there yet has no name, what it is (spec-teams B2).
  const teamLabel = (r) => r.name || '新しいチーム定義';
  // nameCell is the row's name column: a field while a definition that is not
  // there yet is being named, and the name itself otherwise. A copy keeps the
  // name it copies, so it shows the name rather than a field (spec-teams B3).
  function nameCell(r, open) {
    const tag = r.draftRow ? '<span class="tag">新規</span>' : '';
    if (!r.draftRow) return `<span class="nm">${esc(r.name)}</span>${tag}`;
    if (r.how === 'copy' || !open) return `<span class="nm">${esc(r.name) || '<span class="none">—</span>'}</span>${tag}`;
    return `<input id="nm-${esc(r.dom)}" class="in-name ${(r.nameIssues || []).length ? 'bad' : ''}" data-f="name" value="${esc(r.name)}" placeholder="名前" spellcheck="false" autocomplete="off" aria-label="チーム定義名">${tag}`;
  }
  function sumInner(r) {
    const t = r.team.draft, open = state.open.has(r.dom), rules = rulesOf(r.agree.draft);
    // A team.yaml that does not read has no figure and no numbers to count, so
    // its row says 使命 —, keeps the problems validate named, and leaves the
    // counts blank (spec-teams 1.9). It is fixed in the YAML tab.
    const broken = r.broken;
    // A definition that is not there yet chooses its place: a row added from the
    // scaffold can go either way, and a copy is bound to the place it goes to
    // (spec-teams B2, B3).
    let pills;
    if (r.draftRow && r.how === 'copy') pills = `<span class="pill pending">${PLACE[r.place]}</span>`;
    else if (r.draftRow) pills = `<span class="seg" role="group" aria-label="置き場所">${PLACES.map((p) => `<button class="${p === r.place ? 'on' : ''}" data-act="pick-place" data-x="${p}" aria-pressed="${p === r.place}">${PLACE[p]}</button>`).join('')}</span>`;
    else pills = (r.places || []).map((p) => `<span class="pill ${p === r.place ? 'on' : 'shadow'}">${PLACE[p]}</span>`).join('');
    const cell = broken ? '—'
      : open
        ? `<input id="m-${esc(r.dom)}" class="in-mission ${t.mission !== r.team.saved.mission ? 'changed' : ''}" data-f="mission" value="${esc(t.mission)}" autocomplete="off" aria-label="${esc(teamLabel(r))} の使命">`
        : t.mission ? esc(t.mission) : '—';
    return `<span class="chev"></span>
    <span class="c-name">${nameCell(r, open)}<span class="vcount">${rowCounts(r)}</span></span>
    <span class="c-mission ${broken || !(t.mission || open) ? 'none' : ''}">${cell}</span>
    <span class="c-place">${pills}</span>
    <span class="c-members c-num">${broken ? '—' : t.members.length}</span>
    <span class="c-turns c-num">${broken ? '—' : t.workers.length}</span>
    <span class="c-agree c-num">${rules.length}${outNote(outCount(rules))}</span>`;
  }
  const dtabOf = (r) => state.dtab[r.dom] || 'shape';
  function dtabsHTML(r) {
    const cur = dtabOf(r), dot = (on) => (on ? '<i class="dot"></i>' : '');
    return [['shape', 'トポロジー', '', changeCount(r) > 0], ['agree', 'ルール', String(rulesOf(r.agree.draft).length), r.agree.saved ? agreeDiff(r) > 0 : false]]
      .map(([t, label, n, ch]) => `<button class="dtab ${cur === t ? 'on' : ''}" data-act="dtab" data-t="${t}" aria-pressed="${cur === t}">${label}${n !== '' ? `<span class="n">${n}</span>` : ''}${dot(ch)}</button>`).join('');
  }
  const dtabsNav = (r) => `<span class="dtl">${dtabsHTML(r)}</span><span class="dacts">${dactsHTML(r)}</span>`;
  // mvmHTML is what one definition's own field is told: the name's problems
  // first — the name is the field the row is read by, and a name a place
  // already holds is what its save stops on — then the mission's warnings
  // (spec-teams B2, 監査 R4). The sentences are the server's.
  const mvmHTML = (r) => [
    ...(r.nameIssues || []),
    ...r.findings.filter((m) => m.field === 'mission'),
  ].map((m) => vmItem({ sev: m.sev, msg: m.text })).join('');
  function detailHTML(r) {
    const hidden = r.hidden || [];
    return `<div class="detail">
    <ul class="vm mvm">${mvmHTML(r)}</ul>
    <nav class="dtabs">${dtabsNav(r)}</nav>
    ${dtabOf(r) === 'shape' ? teamPanelHTML(r) : agreePanelHTML(r)}
    ${hidden.map((h) => `<section class="panel shadowed"><header class="ph"><span class="ln">${PLACE[h.place]}</span><code class="path">${esc((DIR[h.place] || '') + r.name)}/</code><span class="state off">隠れている</span><span class="phr"><button class="btn sm" data-act="to-yaml" data-key="${esc(r.name)}.${h.place}.team">YAML</button></span></header></section>`).join('')}
  </div>`;
  }
  // dactsHTML is what one definition's whole folder is offered: 複製, a copy into
  // the other place, and 削除 with its own confirm. A definition that is not
  // there yet is only taken back (spec-teams B3, B4, B5).
  function dactsHTML(r) {
    if (r.draftRow) return '<button class="btn danger" data-act="discard">取り消し</button>';
    const o = other(r.place);
    const copying = [...recs.values()].some((x) => x.draftRow && x.how === 'copy' && x.name === r.name && x.place === o);
    const ck = 'del' + dkey(r);
    return `<button class="btn" data-act="dup">複製</button>
    ${!(r.places || []).includes(o) && !copying ? `<button class="btn" data-act="copy">${PLACE[o]}にコピー</button>` : ''}
    ${state.confirm[ck] ? '<button class="btn danger solid" data-act="del-yes">削除する（team.yaml と agreement.yaml）</button><button class="btn" data-act="del-no">やめる</button>'
      : '<button class="btn danger" data-act="del">削除</button>'}`;
  }
  function rowHTML(r) {
    const open = state.open.has(r.dom);
    return `<div class="tdef ${open ? 'open' : ''} ${dirtyDef(r) ? 'unsaved' : ''}" data-name="${esc(r.dom)}" data-key="${dkey(r)}">
    <div class="sum" role="button" tabindex="0" aria-expanded="${open}" data-act="toggle">${sumInner(r)}</div>
    ${open ? detailHTML(r) : ''}</div>`;
  }
  function listHTML() {
    const toolbar = '<div class="toolbar"><button class="btn primary" data-act="new">＋ チーム定義を追加</button></div>';
    if (!recs.size) return `${toolbar}<div class="list"><p class="none">チーム定義はありません</p></div>`;
    return `${toolbar}<div class="list">
    <div class="head"><span></span><span>名前</span><span>使命</span><span>置き場所</span><span>メンバー</span><span>ワーカー</span><span>ルール</span></div>
    ${[...recs.values()].map(rowHTML).join('')}</div>`;
  }
  function refreshRow(name) {
    const r = byName(name);
    if (!r) return;
    const row = $(`.tdef[data-name="${CSS.escape(name)}"]`);
    if (!row) return;
    const t = r.team.draft;
    row.classList.toggle('unsaved', dirtyDef(r));
    const q = (s) => row.querySelector(s);
    const vc = q('.c-name .vcount'); if (vc) vc.innerHTML = rowCounts(r);
    const ni = q('.in-name'); if (ni) ni.classList.toggle('bad', (r.nameIssues || []).length > 0);
    const ms = q('.c-members'); if (ms && !r.broken) ms.textContent = t.members.length;
    const tu = q('.c-turns'); if (tu && !r.broken) tu.textContent = t.workers.length;
    const ag = q('.c-agree'); if (ag) { const rs = rulesOf(r.agree.draft); ag.innerHTML = `${rs.length}${outNote(outCount(rs))}`; }
    const dt = q('.dtabs .dtab[data-t=agree]');
    if (dt) { const rs = rulesOf(r.agree.draft); dt.innerHTML = `ルール<span class="n">${rs.length}</span>${r.agree.saved && agreeDiff(r) > 0 ? '<i class="dot"></i>' : ''}`; }
    const st = q('.dtabs .dtab[data-t=shape]');
    if (st) { const dot = st.querySelector('.dot'); const ch = changeCount(r) > 0; if (ch && !dot) st.insertAdjacentHTML('beforeend', '<i class="dot"></i>'); else if (!ch && dot) dot.remove(); }
    const cell = q('.c-mission .in-mission');
    if (cell) cell.classList.toggle('changed', cell.value !== r.team.saved.mission);
    const mvm = q('.mvm');
    if (mvm) mvm.innerHTML = mvmHTML(r);
  }

  // ---- YAML のタブ ----------------------------------------------------
  // The tab itself is the shared part (yamltab.js, spec-boundary 7): this screen
  // draws the file nav and one section per file, and hands the section to the
  // part, which runs the check and the save through /api/team with the file's own
  // part. The part shows 保存しました and the refusal (r-0f8c).
  const FILES = (LIST.files || []).map((f) => ({
    file: `${f.name}.${f.place}.${f.kind}`, place: f.place, name: f.name, kind: f.kind,
    label: f.label || PLACE[f.place] || '', path: f.path || '', hash: f.hash || '',
    text: f.text || '', hidden: !!f.hidden, problems: f.problems || [],
  }));
  const fileByKey = new Map(FILES.map((f) => [f.file, f]));
  const recFor = (f) => { const r = byName(f.name); return r && r.place === f.place ? r : null; };
  const fileText = (file) => (state.ytext[file] !== undefined ? state.ytext[file] : ((fileByKey.get(file) || {}).text || ''));
  const fileDirty = (file) => state.ytext[file] !== undefined && state.ytext[file] !== ((fileByKey.get(file) || {}).text || '');
  function yIssues(file) { return state.yc[file] || (fileByKey.get(file) || {}).problems || []; }
  function filesHTML() {
    const keys = PLACES.flatMap((p) => FILES.filter((f) => f.place === p).map((f) => f.file));
    if (!state.yfile || !keys.includes(state.yfile)) state.yfile = keys[0] || null;
    return `<nav class="files">${PLACES.map((p) => {
      const fs = FILES.filter((f) => f.place === p);
      if (!fs.length) return '';
      return `<div class="fgroup"><div class="gh"><b>${PLACE[p]}</b><code>${esc(DIR[p] || '')}</code></div>${fs.map((f) => {
        const r = recFor(f);
        const n = f.kind === 'agreement' && r ? rulesOf(r.agree.saved || r.agree.draft).length : '';
        return `<button class="file ${state.yfile === f.file ? 'on' : ''} ${f.hidden ? 'hid' : ''}" data-act="yfile" data-file="${esc(f.file)}" aria-pressed="${state.yfile === f.file}"><code>${esc(f.name)}/${f.kind === 'agreement' ? 'agreement.yaml' : 'team.yaml'}</code><span class="fs">${fileDirty(f.file) ? '<span class="dirty">●</span>' : ''}${badge(yIssues(f.file)) || n}</span><span class="ft">${f.hidden ? '隠れている' : esc(f.path)}</span></button>`;
      }).join('')}</div>`;
    }).join('')}</nav>`;
  }
  function refreshFileDot(file) {
    const b = $(`.file[data-file="${CSS.escape(file)}"]`);
    if (!b) return;
    const fs = $('.fs', b), dot = $('.dirty', b);
    if (fileDirty(file) && !dot) fs.insertAdjacentHTML('afterbegin', '<span class="dirty">●</span>');
    if (!fileDirty(file) && dot) dot.remove();
  }
  function yedHTML() {
    const key = state.yfile, f = fileByKey.get(key);
    if (!key || !f) return '<section class="yed"><p class="none">ファイルを選んでください</p></section>';
    const r = recFor(f), both = r ? (r.places || []).length > 1 : false;
    return `<section class="yed" data-key="${esc(key)}" data-hash="${esc(f.hash)}">
    <header class="ph"><span class="ln">${esc(f.label)}</span><code class="path">${esc(f.path)}</code>${both ? `<span class="state ${f.hidden ? 'off' : 'on'}">${f.hidden ? '隠れている' : '効いている'}</span>` : ''}</header>
    <div class="ybody">
      <div class="code"><pre class="gutter" aria-hidden="true"></pre><textarea class="ytext" wrap="off" spellcheck="false" autocomplete="off" aria-label="${esc(f.path)}">${esc(fileText(key))}</textarea></div>
      <div class="vlist"></div>
    </div>
    <footer class="pf">
      <button class="btn primary" data-act="yaml-save">保存</button><span class="dirty"></span>
      <span class="acts"><button class="btn" data-act="yaml-revert" hidden>元に戻す</button></span>
    </footer></section>`;
  }
  // mountYed hands one file's section to the shared part: the kind is this
  // screen's own, the part word says which of the definition's two documents the
  // file is, and the problems the part finds come back so the file nav marks a
  // file that is wrong.
  function mountYed(sec) {
    const f = sec && fileByKey.get(sec.dataset.key);
    if (!f) return;
    sec.dataset.hash = f.hash;
    YAML.mount(sec, {
      kind: 'team', place: f.place, name: f.name, extra: { part: f.kind },
      problems: yIssues(f.file),
      onProblems: (section, probs) => { state.yc[f.file] = probs; },
      onSaved: (section, answer) => {
        const json = API.answer(answer);
        f.hash = json.hash || f.hash;
        f.text = section.querySelector('.ytext').value;
        state.ytext[f.file] = f.text;
        delete state.yc[f.file];
      },
    });
  }
  const yamlTabHTML = () => `<div class="ylay">${filesHTML()}${yedHTML()}</div>`;

  // ---- 描く ------------------------------------------------------------
  function render() {
    $('#tabs').innerHTML = [['list', '一覧'], ['yaml', 'YAML']]
      .map(([t, label]) => `<button class="tab ${state.tab === t ? 'on' : ''}" data-act="tab" data-t="${t}" aria-pressed="${state.tab === t}">${label}</button>`).join('');
    view.innerHTML = state.tab === 'list' ? listHTML() : yamlTabHTML();
    setStick();
    if (state.tab === 'list') {
      $$('.panel.team-p', view).forEach(placeCard);
      fitYboxes();
      $$('.panel.team-p', view).forEach((p) => { const r = byKey(p.dataset.key); if (r) afterTPrev(r); });
      $$('.panel.agree-p', view).forEach(markPanel);
      $$('.tdef.open', view).forEach((row) => { const r = byName(row.dataset.name); if (r && dtabOf(r) === 'agree') checkAgree(r); });
    } else {
      mountYed($('.yed', view));
    }
    setBandCount();
    tellDirty();
  }
  function rerenderTeam(r, sel) {
    const panel = $(`.panel.team-p[data-key="${CSS.escape(dkey(r))}"]`);
    if (!panel) { refreshRow(r.dom); return; }
    const box = $('.gscroll', panel);
    const scroll = box ? box.scrollLeft : 0;
    panel.outerHTML = teamPanelHTML(r);
    const fresh = $(`.panel.team-p[data-key="${CSS.escape(dkey(r))}"]`);
    if (fresh) {
      const gc = $('.gscroll', fresh);
      if (gc) gc.scrollLeft = scroll;
      placeCard(fresh);
      fitYbox(fresh);
      afterTPrev(r);
      if (sel) { const el = fresh.querySelector(sel); if (el) el.focus({ preventScroll: true }); }
    }
    refreshRow(r.dom);
  }
  // softTeam draws again what a worker's purpose moves while a person types it
  // (spec-teams 1.4.4, 監査 R7): the point's letters, that card's own checks and
  // the foot's count. The field the person is typing in is not touched, so the
  // cursor stays where it was.
  function softTeam(r) {
    const panel = $(`.panel.team-p[data-key="${CSS.escape(dkey(r))}"]`);
    if (!panel) { refreshRow(r.dom); return; }
    const sc = $('.gscroll', panel);
    if (sc) {
      const left = sc.scrollLeft;
      sc.outerHTML = `<div class="gscroll">${graphSVG(r)}</div>`;
      const ns = $('.gscroll', panel);
      if (ns) ns.scrollLeft = left;
    }
    const card = $('.gcard[data-sel=worker]', panel);
    if (card) {
      const i = +card.dataset.i, is = teamIssues(r).filter((m) => m.worker === i), w = r.team.draft.workers[i];
      const ul = $('.vm', card);
      if (is.length) {
        const html = vmList(is);
        if (ul) ul.outerHTML = html;
        else { const gdir = $('.gdir', card); if (gdir) gdir.insertAdjacentHTML('beforebegin', html); }
      } else if (ul) ul.remove();
      const ln = $('.gpl .ln2', card);
      if (w && ln) ln.textContent = `${w.purpose || '（目的）'} → ${w.profile}`;
    }
    const vl = $('.vlist', panel);
    if (vl) vl.outerHTML = `<div class="vlist">${vlistHTML(r)}</div>`;
    // The preview follows the text a person is typing (spec-teams B7, Y5).
    const pv = $('.tprev', panel);
    if (pv) pv.outerHTML = tprevHTML(r);
    const ft = $('footer.pf', panel);
    if (ft) ft.outerHTML = footHTML(r);
    placeCard(panel);
    fitYbox(panel);
    afterTPrev(r);
    refreshRow(r.dom);
  }
  function rerenderAgree(panel, sel) {
    const r = panel && byKey(panel.dataset.key);
    if (!r) return panel;
    const box = $('.ybox', panel);
    if (box && box.clientHeight) RD.state.yscroll[r.uid + r.place] = box.scrollTop;
    panel.outerHTML = agreePanelHTML(r);
    const fresh = $(`.panel.agree-p[data-key="${CSS.escape(dkey(r))}"]`);
    if (fresh) { markPanel(fresh); if (sel) { const el = fresh.querySelector(sel); if (el) { el.focus({ preventScroll: true }); el.select?.(); } } }
    refreshRow(r.dom);
    return fresh;
  }
  function rerenderSides(r) {
    const row = $(`.tdef[data-name="${CSS.escape(r.dom)}"]`);
    if (!row) return;
    if (!state.open.has(r.dom)) { row.outerHTML = rowHTML(r); return; }
    const team = $(`.panel.team-p`, row);
    if (team) team.outerHTML = teamPanelHTML(r);
    const ag = $(`.panel.agree-p`, row);
    if (ag) ag.outerHTML = agreePanelHTML(r);
    $$('.dtabs', row).forEach((n) => { n.innerHTML = dtabsNav(r); });
    if (state.open.has(r.dom)) $$('.panel.team-p', row).forEach(placeCard);
    $$('.panel.team-p', row).forEach(fitYbox);
    afterTPrev(r);
    $$('.panel.agree-p', row).forEach(markPanel);
    row.classList.toggle('unsaved', dirtyDef(r));
    refreshRow(r.dom);
  }
  function rerenderRow(r) {
    const row = $(`.tdef[data-name="${CSS.escape(r.dom)}"]`);
    if (row) row.outerHTML = rowHTML(r);
  }
  const setStick = () => { const bar = $('.bar'); document.documentElement.style.setProperty('--stick', (bar ? bar.offsetHeight : 48) + 'px'); };
  // setBandCount keeps the band's number the same as the list's own: the band
  // counts the definitions, which are the rows that are saved — a row that is
  // not there yet is not one (spec-nav 1.1, 監査 R7).
  function setBandCount() {
    if (!window.hergyNav) return;
    let n = 0;
    for (const r of recs.values()) if (!r.draftRow) n++;
    window.hergyNav.setCount('teams', n);
  }

  function tellDirty() {
    let dirty = false;
    for (const r of recs.values()) if (dirtyDef(r)) dirty = true;
    for (const f of FILES) if (fileDirty(f.file)) dirty = true;
    if (window.hergyPage) window.hergyPage.dirty = dirty;
  }

  // ---- 検査と保存 ------------------------------------------------------
  function syncTeam(r) {
    r.team.draft = applyOps(r.team.saved, r.ops);
    rerenderSides(r);
    checkTeam(r);
  }
  function syncAgree(r, panel) {
    rerenderRowForAgree(r);
    checkAgree(r, () => { const p = $(`.panel.agree-p[data-key="${CSS.escape(dkey(r))}"]`); if (p) markPanel(p); const ag = $(`.tdef[data-name="${CSS.escape(r.dom)}"] .c-agree`); if (ag) { const rs = rulesOf(r.agree.draft); ag.innerHTML = `${rs.length}${outNote(outCount(rs))}`; } void panel; });
  }
  function rerenderRowForAgree(r) {
    const row = $(`.tdef[data-name="${CSS.escape(r.dom)}"]`);
    if (!row) return;
    const dt = $('.dtabs .dtab[data-t=agree]', row);
    if (dt) { const rs = rulesOf(r.agree.draft); dt.innerHTML = `ルール<span class="n">${rs.length}</span>${r.agree.saved && agreeDiff(r) > 0 ? '<i class="dot"></i>' : ''}`; }
    row.classList.toggle('unsaved', dirtyDef(r));
    tellDirty();
  }
  async function checkTeam(r) {
    const id = ++r.req;
    // A check sent with no ops reads the saved file, so its findings are the
    // baseline the save gate is held to (spec-teams X3). It is taken from the
    // request rather than from the answer's turn, and kept even when a later
    // check has made this answer too old to draw: the file it read is the one a
    // save would write over, and a person who writes before the first answer
    // has come back still gets the file's own problems as the baseline.
    const based = !r.ops.length;
    const res = await API.check('team', teamIn(r, { doc: { ops: r.ops } }));
    if (!res.ok) return;
    const json = API.answer(res);
    // The first answer for a definition that is not there yet is the base it
    // stands on, and it is read once (spec-teams B2).
    if (r.draftRow && !r.seeded) seedDraft(r, json);
    const findings = figFindings(r, json.problems || []);
    if (based) r.baseFindings = findings;
    if (id !== r.req) return;
    r.findings = findings;
    // Both a saved definition and one that is not there yet get the topology
    // tab's own preview from the check (spec-teams B7).
    r.preview = json.preview || r.preview;
    if (r.draftRow) {
      r.writes = json.writes || r.writes;
      r.nameIssues = (json.problems || []).filter((p) => p.at === 'name' || p.field === 'name');
      // The paths the row would be written to are the server's, and they move
      // with the name a person is typing: both panels' heads follow the check
      // (spec-teams B2, 監査 R5).
      if (json.path) r.teamPath = json.path;
      const ag = (json.doc || {}).agreement || {};
      if (ag.path) {
        r.agreePath = ag.path;
        if (r.agree.saved) r.agree.saved.__path = ag.path;
        if (r.agree.draft) r.agree.draft.__path = ag.path;
      }
    }
    // Someone typing a worker's purpose or the mission keeps the field they are
    // typing in: that card's checks, the point's letters and the preview are drawn
    // again, the panel is not (spec-teams 1.4.4, B7, 監査 R7).
    const act = document.activeElement;
    const row = $(`.tdef[data-name="${CSS.escape(r.dom)}"]`);
    const typed = act && act.matches && (act.matches('.in-purpose') || act.matches('.in-mission')) && act.closest('.tdef') === row;
    if (typed) softTeam(r);
    else rerenderSides(r);
  }
  // afterAgree re-marks the panel the check answered for: the shared part reads
  // the Markdown, the diff and the problems through host.row, so it draws the
  // answer without this screen rebuilding the panel (spec-boundary 7).
  const afterAgree = (r) => { const p = $(`.panel.agree-p[data-key="${CSS.escape(dkey(r))}"]`); if (p) markPanel(p); };
  async function checkAgree(r, after) {
    const id = ++r.areq;
    const doc = RD.toDoc(r.agree.draft, r.agree.saved);
    const res = await API.check('team', agreeIn(r, { doc: { ops: r.ops, doc } }));
    if (id !== r.areq || !res.ok) return;
    r.check = buildCheck(r.agree.draft, API.answer(res));
    if (!agreeDiff(r)) r.agree.savedIssues = r.check.issues;
    (after || afterAgree)(r);
  }
  function scheduleAgree(r, panel) { debounce('a' + r.key, () => syncAgree(r, panel)); }

  async function saveTeam(r, panel) {
    if (r.draftRow) return saveFolder(r, panel);
    const res = await API.save('team', r.name, teamIn(r, { hash: r.hash, doc: { ops: r.ops } }));
    if (!res.ok) { API.refusal(panel, API.problemText(res, '保存できませんでした')); return; }
    const json = API.answer(res);
    r.hash = json.hash || r.hash;
    r.team.saved = cloneTeam(r.team.draft);
    r.savedTurns = cloneTeam(r.team.draft).turns;
    r.ops = [];
    r.baseFindings = r.findings;
    setProblems(json.items);
    state.ok['t' + dkey(r)] = 1;
    delete state.gsel[dkey(r)];
    rerenderRow(r);
    tellDirty();
  }
  async function saveAgree(r, panel) {
    if (r.draftRow) return saveFolder(r, panel);
    const doc = RD.toDoc(r.agree.draft, r.agree.saved);
    const res = await API.save('team', r.name, agreeIn(r, { hash: r.agreeHash, doc: { ops: r.ops, doc } }));
    if (!res.ok) { showErr(panel, API.problemText(res, '保存できませんでした')); return; }
    const json = API.answer(res);
    r.agreeHash = json.hash || r.agreeHash;
    r.hasAgree = true;
    if (json.doc) { const d2 = RD.fromJSON(json.doc); d2.__path = r.agreePath; r.agree.saved = d2; r.agree.draft = clone(d2); }
    setProblems(json.items);
    state.ok['a' + dkey(r)] = 1;
    rerenderSides(r);
    tellDirty();
    // The saved text's own problems are the new baseline, and the check the
    // panel shows is the one it now holds.
    checkAgree(r);
  }
  // saveFolder writes a definition that is not there yet, whole (spec-teams B2,
  // B6, C8): its team.yaml and its agreement.yaml, and, for one made from the
  // scaffold, the layer's role documents and the starter profile. Either tab's
  // 保存 writes the whole folder. The row becomes the saved definition in its
  // place, so the person keeps working on it.
  async function saveFolder(r, panel) {
    const doc = RD.toDoc(r.agree.draft, r.agree.saved);
    const body = Object.assign(teamIn(r), {
      hash: r.hash, agreement_hash: r.agreeHash, part: 'folder', doc: { ops: r.ops, doc },
    });
    const res = await API.save('team', r.name, body);
    if (!res.ok) {
      const tp = $(`.panel.team-p[data-key="${CSS.escape(dkey(r))}"]`);
      API.refusal(panel || tp, API.problemText(res, '保存できませんでした'));
      return;
    }
    const json = API.answer(res);
    const name = r.name, dom = r.dom, dtab = state.dtab[dom] || 'shape', opens = state.open.has(dom);
    // The row stands where the row a person was looking at stood: the first
    // reading of this name keeps its place, and a copy's save joins into it
    // (spec-teams B1-B4).
    const at = firstIndexOf((x) => x.dom === dom || x.name === name);
    // Every reading of this name is taken again from the answer: a copy's save
    // joins the two places into one row (spec-teams B4).
    [...recs.values()].filter((x) => x.dom === dom || x.name === name).forEach((x) => {
      recs.delete(x.key); delete state.confirm['del' + x.key]; delete state.gsel[x.key];
    });
    setProblems(json.items);
    const item = (json.items || []).find((it) => it.name === name);
    if (!item) { state.open.delete(dom); delete state.dtab[dom]; render(); return; }
    const nr = defOf(item);
    orderRows(nr, at);
    if (opens) state.open.add(nr.dom);
    state.dtab[nr.dom] = dtab;
    render();
    // The definition the save wrote holds its own checks now: the figure's and
    // the rules' are read again from the file it wrote (spec-boundary 5). The
    // 保存しました stands on the row's own foot after that reading, so the
    // check's own draw does not take it away (1.4.6, 監査 R7).
    await checkTeam(nr);
    if (nr.agree.saved) await checkAgree(nr);
    state.ok[(dtabOf(nr) === 'agree' ? 'a' : 't') + dkey(nr)] = 1;
    rerenderSides(nr);
    syncURL();
  }
  // doDelete takes one definition's folder out of one place (spec-teams B6): its
  // team.yaml and its agreement.yaml, and the directory itself when the two files
  // were all it held. Every record of that name is read again from the answer,
  // because the other place's reading may be the one in effect now.
  async function doDelete(r, panel) {
    const res = await API.del('team', r.name, r.place, r.hash, r.agreeHash);
    if (!res.ok) { API.refusal(panel, API.problemText(res, '削除できませんでした')); return; }
    const json = API.answer(res);
    const name = r.name;
    // The row keeps its place in the list: taking one place's folder away leaves
    // the definition where a person was looking at it (spec-teams B6, N7).
    const at = firstIndexOf((x) => x.name === name);
    [...recs.values()].filter((x) => x.name === name).forEach((x) => {
      recs.delete(x.key); delete state.confirm['del' + x.key]; delete state.gsel[x.key];
    });
    setProblems(json.items);
    const item = (json.items || []).find((it) => it.name === name);
    if (item) {
      const nr = defOf(item);
      orderRows(nr, at);
      state.dtab[nr.dom] = state.dtab[nr.dom] || 'shape';
    }
    render();
    syncURL();
  }
  function showErr(panel, text) {
    if (!panel) return;
    let el = $('.errline.doc', panel);
    if (!el) { el = document.createElement('span'); el.className = 'errline doc'; const pf = $('.pf', panel); if (pf) pf.appendChild(el); }
    if (el) { el.hidden = false; el.textContent = text || '保存できませんでした'; }
  }

  // ---- URL -------------------------------------------------------------
  function syncURL() {
    const params = new URLSearchParams();
    if (state.tab === 'yaml') { if (state.yfile) params.set('file', state.yfile); }
    else {
      const opens = [...state.open].filter((n) => { const r = byName(n); return r && !r.draftRow; });
      if (opens.length) params.set('open', opens.join(','));
      const agrees = opens.filter((n) => { const r = byName(n); return r && dtabOf(r) === 'agree'; });
      if (agrees.length) params.set('agree', agrees.join(','));
      const rules = [];
      opens.forEach((n) => {
        const r = byName(n);
        if (!r) return;
        RD.state.ropen.forEach((kk) => {
          const head = `${r.uid}${r.place}:`;
          if (!kk.startsWith(head)) return;
          const hit = rulesOf(r.agree.draft).find((x) => x._k === Number(kk.slice(head.length)));
          if (hit && hit.id) rules.push(`${r.name}.${hit.id}`);
        });
      });
      if (rules.length) params.set('rule', rules.join(','));
    }
    const path = state.tab === 'yaml' ? '/teams/yaml' : '/teams';
    const q = params.toString();
    history.replaceState({}, '', q ? `${path}?${q}` : path);
  }
  function readURL() {
    const p = new URLSearchParams(location.search);
    const open = p.get('open');
    if (open) open.split(',').filter(Boolean).forEach((n) => state.open.add(n));
    const agree = p.get('agree');
    if (agree) agree.split(',').filter(Boolean).forEach((n) => { state.dtab[n] = 'agree'; });
    const rule = p.get('rule');
    state.wantRule = null;
    if (rule) rule.split(',').filter(Boolean).forEach((spec) => {
      const i = spec.indexOf('.');
      if (i < 0) return;
      const r = byName(spec.slice(0, i));
      if (!r) return;
      const hit = rulesOf(r.agree.draft).find((x) => x.id === spec.slice(i + 1));
      if (hit) {
        state.open.add(r.dom);
        state.dtab[r.dom] = 'agree';
        RD.state.ropen.add(`${r.uid}${r.place}:${hit._k}`);
        state.wantRule = { name: r.dom, k: hit._k };
      }
    });
    const file = p.get('file');
    if (file) state.yfile = file;
    if (location.pathname === '/teams/yaml') state.tab = 'yaml';
  }

  // jumpToRule opens one rule of a team's own agreement and puts the cursor in
  // its body, brought into the window so the save's band never hides it
  // (spec-teams 1.4.5, spec-roles 1.2.4, 監査 R12). The check's column reads
  // through this, and so does the address — jumpWantRule is the same jump once
  // the panel the address names has been drawn.
  function jumpToRule(r, k) {
    const p = $(`.panel.agree-p[data-key="${CSS.escape(dkey(r))}"]`);
    if (p) RD.goRule(p, k);
  }
  function jumpWantRule() {
    const w = state.wantRule;
    state.wantRule = null;
    if (!w) return;
    const r = byName(w.name);
    if (r) jumpToRule(r, w.k);
  }

  // ---- 操作 ------------------------------------------------------------
  function pushOp(r, op) { r.ops.push(op); }
  // setProblems takes the counts a save answered with back into the rows, so the
  // row's 問題/警告 chip is the server's own count again (spec-boundary 5).
  function setProblems(items) {
    (items || []).forEach((item) => {
      const places = item.places || {};
      const eff = item.effective || (places.project ? 'project' : 'global');
      const r = recs.get((item.name || '') + '.' + eff);
      if (r) r.problems = item.problems || {};
    });
  }

  const state = {
    tab: view.dataset.tab === 'yaml' ? 'yaml' : 'list',
    open: new Set(), dtab: {}, gsel: {}, confirm: {}, effect: {},
    ok: {}, yfile: null, ytext: {}, yc: {}, pmode: {}, wantRule: null,
    // The topology tab's preview bookkeeping: the lines drawn last, the lines one
    // edit just moved, and when they lit up (spec-teams B7).
    tlast: {}, tstep: {}, tglow: {}, tglowTimer: {},
  };
  (LIST.items || []).forEach(defOf);
  readURL();

  const ctx = (el) => {
    const panel = el.closest('.panel.agree-p');
    if (!panel) return {};
    const r = byKey(panel.dataset.key);
    if (!r) return {};
    const a = r.agree.draft;
    const ruleEl = el.closest('.rule'), f = ruleEl ? RD.findRule(a, ruleEl.dataset.k) : null;
    const secEl = el.closest('.sec'), si = f ? f.si : secEl ? +secEl.dataset.si : null;
    const oi = el.closest('.orow') ? +el.closest('.orow').dataset.oi : null;
    return { panel, d: r, a, x: f && f.x, si, ri: f && f.ri, oi, key: `${dkey(r)}-${f ? f.x._k : ''}` };
  };

  wrapEl.addEventListener('click', async (e) => {
    if (e.target.closest('.in-mission, .in-name')) return;
    const b = e.target.closest('[data-act]');
    if (!b) return;
    const act = b.dataset.act, row = b.closest('.tdef'), n = row && row.dataset.name;
    const tp = b.closest('.panel.team-p'), td = tp && byKey(tp.dataset.key), tk = td && dkey(td);
    const c = ctx(b);
    const re = (sel) => rerenderAgree(c.panel, sel);
    switch (act) {
      case 'tab': state.tab = b.dataset.t; render(); break;
      case 'toggle': if (state.open.has(n)) state.open.delete(n); else state.open.add(n); render(); break;
      // The list's own toolbar adds a definition from the scaffold, and each row
      // duplicates itself, copies itself into the other place, or goes away
      // (spec-teams B1-B5).
      case 'new': await addDraft('new', 'project', '', null); break;
      case 'dup': { const r = byName(n); if (r) await addDraft('dup', r.place, theName(r.name), { place: r.place, name: r.name }); break; }
      case 'copy': { const r = byName(n); if (r) await addDraft('copy', other(r.place), r.name, { place: r.place, name: r.name }); break; }
      case 'pick-place': {
        const r = byName(n);
        if (!r || !r.draftRow || r.place === b.dataset.x) break;
        // The row keeps its place in the list while its place changes
        // (spec-teams B1, N4).
        const at = firstIndexOf((x) => x === r);
        recs.delete(dkey(r));
        r.place = b.dataset.x;
        r.key = dkey(r);
        recs.set(r.key, r);
        orderRows(r, at);
        delete state.gsel[r.key];
        // The base is read again: the other place's role documents and profile
        // are not the ones the row stood on (spec-teams B2).
        r.seeded = false;
        r.team.saved = cloneTeam(null); r.team.draft = cloneTeam(null); r.savedTurns = [];
        r.agree.saved = null;
        await checkTeam(r);
        render();
        const el = $(`.tdef[data-name="${CSS.escape(r.dom)}"] [data-act=pick-place][data-x="${b.dataset.x}"]`);
        if (el) el.focus({ preventScroll: true });
        break;
      }
      case 'discard': { const r = byName(n); if (r) dropDraft(r); break; }
      case 'del': { const r = byName(n); if (r) { state.confirm['del' + dkey(r)] = 1; rerenderSides(r); } break; }
      case 'del-no': { const r = byName(n); if (r) { delete state.confirm['del' + dkey(r)]; rerenderSides(r); } break; }
      case 'del-yes': { const r = byName(n); if (r) await doDelete(r, row); break; }
      case 'tmode': if (td) { state.pmode['t' + tk] = b.dataset.v; rerenderTeam(td, `[data-act=tmode][data-v="${b.dataset.v}"]`); } break;
      // A press on a line of the preview picks what the line stands for, the way
      // a press on the figure's own point does (spec-teams B7). A mission line
      // puts the cursor in the mission field. Where the places are one column, the
      // tab comes back to 編集 first, because what it picks is drawn there.
      case 'tyline': if (td) {
        // One column (≤1100) shows the figure again when a line is pressed, the
        // way the rule document's Markdown does, so the picked point's card
        // stands beside the line a person pressed (spec-teams B7, Y7).
        state.pmode['t' + tk] = 'edit';
        if (b.dataset.f === 'mission') {
          rerenderTeam(td);
          const el = $(`.tdef[data-name="${CSS.escape(td.dom)}"] .in-mission`);
          if (el) { el.focus({ preventScroll: true }); el.select && el.select(); }
          break;
        }
        if (b.dataset.node) state.gsel[tk] = { type: 'node', id: b.dataset.node };
        else if (b.dataset.worker != null) state.gsel[tk] = { type: 'worker', i: +b.dataset.worker };
        else if (b.dataset.a) state.gsel[tk] = { type: 'edge', k: pairKey(b.dataset.a, b.dataset.b) };
        else delete state.gsel[tk];
        rerenderTeam(td);
      } break;
      case 'node': { const id = b.dataset.id, cur = state.gsel[tk]; state.gsel[tk] = cur && cur.type === 'node' && cur.id === id ? null : { type: 'node', id }; rerenderTeam(td, `.gn[data-id="${CSS.escape(id)}"]`); break; }
      case 'edge': { const k = b.dataset.k, cur = state.gsel[tk]; state.gsel[tk] = cur && cur.type === 'edge' && cur.k === k ? null : { type: 'edge', k }; rerenderTeam(td, `.hit[data-k="${CSS.escape(k)}"]`); break; }
      case 'pick-edge': state.gsel[tk] = { type: 'edge', k: pairKey(b.dataset.a, b.dataset.b) }; rerenderTeam(td); break;
      case 'worker': { const i = +b.dataset.i, cur = state.gsel[tk]; state.gsel[tk] = cur && cur.type === 'worker' && cur.i === i && b.closest('svg') ? null : { type: 'worker', i }; rerenderTeam(td); break; }
      case 'gclose': delete state.gsel[tk]; rerenderTeam(td); break;
      case 'turn-add': { const a = b.dataset.a, bb = b.dataset.b; pushOp(td, { kind: 'addTurn', from: a, to: bb }); state.gsel[tk] = { type: 'edge', k: pairKey(a, bb) }; syncTeam(td); break; }
      case 'turn-ask': {
        const pk = b.dataset.pk; state.confirm[pk] = 1; rerenderTeam(td, '.gcard .btn.danger.solid');
        const card = b.closest('.gcard');
        let op = null;
        if (b.dataset.a) op = { kind: 'removeTurn', from: b.dataset.a, to: b.dataset.b };
        else if (card && card.dataset.sel === 'worker') { const w = td.team.draft.workers[+card.dataset.i]; if (w) op = { kind: 'removeWorker', from: w.from, purpose: w.purpose }; }
        if (op) askEffect(td, op, pk, () => rerenderTeam(td));
        break;
      }
      case 'turn-ask-no': delete state.confirm[b.dataset.pk]; rerenderTeam(td); break;
      case 'turn-del': { const a = b.dataset.a, bb = b.dataset.b; delete state.confirm[`t:${tk}:${a}>${bb}`]; pushOp(td, { kind: 'removeTurn', from: a, to: bb }); syncTeam(td); break; }
      case 'worker-add': {
        pushOp(td, { kind: 'addWorker', from: b.dataset.from, purpose: '', profile: PROFILES[0] || '' });
        state.gsel[tk] = { type: 'worker', i: td.team.draft.workers.length };
        syncTeam(td);
        // The card opens with the cursor in the purpose (spec-teams 1.4.4).
        const pi = $(`.panel.team-p[data-key="${CSS.escape(tk)}"] .gcard[data-sel=worker] .in-purpose`);
        if (pi) pi.focus({ preventScroll: true });
        break;
      }
      case 'worker-del': { const pk = `w:${tk}:${b.dataset.i}`; delete state.confirm[pk]; const w = td.team.draft.workers[+b.dataset.i]; if (w) pushOp(td, { kind: 'removeWorker', from: w.from, purpose: w.purpose }); delete state.gsel[tk]; syncTeam(td); break; }
      case 'vpick':
        if (b.dataset.nameField) { const el = $(`.tdef[data-name="${CSS.escape(n)}"] .in-name`); if (el) { el.focus({ preventScroll: true }); el.select && el.select(); } break; }
        if (b.dataset.node) state.gsel[tk] = { type: 'node', id: b.dataset.node };
        else if (b.dataset.worker != null) state.gsel[tk] = { type: 'worker', i: +b.dataset.worker };
        rerenderTeam(td);
        break;
      case 'vrule': {
        // A warning about the team's own agreement opens that rule (spec-teams
        // 1.4.5, 監査 R9) in the ルール tab, where a person can see it, with the
        // cursor in its body (監査 R12).
        const r2 = byName(b.dataset.name);
        if (!r2) break;
        state.open.add(r2.dom);
        state.dtab[r2.dom] = 'agree';
        const hit = rulesOf(r2.agree.draft).find((x) => String(x.id) === b.dataset.id);
        render();
        checkAgree(r2);
        syncURL();
        if (hit) jumpToRule(r2, hit._k);
        break;
      }
      case 't-save': saveTeam(td, tp); break;
      case 't-revert': td.ops = []; delete state.gsel[tk]; state.confirm = {}; syncTeam(td); rerenderRow(td); break;
      case 'to-yaml': state.tab = 'yaml'; state.yfile = `${b.dataset.key}.team`; render(); window.scrollTo(0, 0); break;
      case 'dtab': state.dtab[n] = b.dataset.t; { const det = $('.detail', row); if (det) det.outerHTML = detailHTML(byName(n)); if (b.dataset.t === 'agree') checkAgree(byName(n)); } $$(`.tdef[data-name="${CSS.escape(n)}"] .panel.team-p`).forEach(placeCard); $$(`.tdef[data-name="${CSS.escape(n)}"] .panel.team-p`).forEach(fitYbox); $$(`.tdef[data-name="${CSS.escape(n)}"] .panel.agree-p`).forEach(markPanel); refreshRow(n); break;
      case 'yfile': state.yfile = b.dataset.file; render(); break;
      case 'yaml-jump': YAML.jump(b.closest('.yed'), Number(b.dataset.line)); break;
      case 'yaml-revert': { const sec = b.closest('.yed'); delete state.ytext[sec.dataset.key]; YAML.revert(sec); refreshFileDot(sec.dataset.key); break; }
      case 'yaml-save': YAML.save(b.closest('.yed')); break;
      case 'save': if (c.d) saveAgree(c.d, c.panel); break;
      case 'revert': if (c.d) { c.d.agree.draft = clone(c.d.agree.saved); checkAgree(c.d, () => re()); re(); } break;
      case 'pmode': if (c.d) { state.pmode[dkey(c.d)] = b.dataset.v; re(`[data-act=pmode][data-v="${b.dataset.v}"]`); } break;
      default: break;
    }
    // The list's tab, the open rows, the open rule and the chosen file ride in
    // the address (spec-teams 2 章): a reload comes back to the same place.
    if (act === 'tab' || act === 'toggle' || act === 'to-yaml' || act === 'yfile' || act === 'dtab') syncURL();
  });

  view.addEventListener('change', (e) => {
    const t = e.target;
    if (t.dataset.actChange === 'turn-add-sel' && t.value) {
      const p = t.closest('.panel'), r = byKey(p.dataset.key), a = t.dataset.a, bb = t.value;
      pushOp(r, { kind: 'addTurn', from: a, to: bb });
      state.gsel[dkey(r)] = { type: 'edge', k: pairKey(a, bb) };
      syncTeam(r);
      return;
    }
    if (t.dataset.actChange === 'worker-prof') { const p = t.closest('.panel'), r = byKey(p.dataset.key), i = +t.dataset.i, w = r.team.draft.workers[i]; if (w) pushOp(r, { kind: 'workerProfile', from: w.from, purpose: w.purpose, profile: t.value }); syncTeam(r); return; }
  });

  view.addEventListener('input', (e) => {
    const t = e.target;
    if (t.matches('.ytext')) { const sec = t.closest('.yed'); state.ytext[sec.dataset.key] = t.value; YAML.schedule(sec); refreshFileDot(sec.dataset.key); tellDirty(); return; }
    if (t.matches('.in-mission')) {
      const r = byName(t.closest('.tdef').dataset.name);
      const text = t.value.replace(/\n/g, ' ');
      r.ops = r.ops.filter((o) => o.kind !== 'mission');
      if (text !== r.team.saved.mission) pushOp(r, { kind: 'mission', text });
      r.team.draft = applyOps(r.team.saved, r.ops);
      // The field keeps its place and its cursor while the person types: the
      // preview and the row are drawn again around it (spec-teams B7, Y5).
      refreshRow(r.dom);
      softTeam(r);
      checkTeam(r);
      return;
    }
    if (t.matches('.in-name')) {
      // The name a definition that is not there yet takes is checked, and the
      // files a save would write are read again: the server owns both (spec-teams
      // B2). The field keeps its place and its cursor while the person types.
      const r = byName(t.closest('.tdef').dataset.name);
      if (!r) return;
      r.name = t.value;
      debounce('n' + r.dom, () => checkTeam(r));
      return;
    }
    if (t.matches('.in-purpose')) {
      // The field keeps its place and its cursor while the person types: only
      // what the sentence moves is drawn again (spec-teams 1.4.4, 監査 R7).
      const p = t.closest('.panel'), r = byKey(p.dataset.key), i = +t.dataset.i;
      const cur = r.team.draft.workers[i];
      if (!cur) return;
      pushOp(r, { kind: 'workerPurpose', from: cur.from, old: cur.purpose, new: t.value });
      r.team.draft = applyOps(r.team.saved, r.ops);
      softTeam(r);
      checkTeam(r);
      return;
    }
    // The agreement panel's own inputs are the shared part's: ruledoc updates the
    // draft and calls host.edited (r-02f6). This screen only owns the mission and
    // the worker's purpose, which are not in the agreement document.
  });

  // 入力の途中でも、ルールのまとめと Markdown を行番号に合わせて引き直す
  view.addEventListener('focusin', (e) => {
    const t = e.target;
    if (!t.closest('.pedit')) return;
    const c = ctx(t);
    if (!c.panel) return;
    RD.state.pfocus[c.d.uid + c.d.place] = { k: c.x ? c.x._k : 0 };
  });
  view.addEventListener('scroll', (e) => {
    if (e.target.matches?.('.ytext')) { const g = $('.gutter', e.target.closest('.code')); if (g) g.scrollTop = e.target.scrollTop; }
    if (e.target.matches?.('.ybox')) { const p = e.target.closest('.panel'); if (p) RD.state.yscroll[p.dataset.uid + p.dataset.l] = e.target.scrollTop; }
  }, true);
  view.addEventListener('keydown', (e) => {
    if ((e.key === 'Enter' || e.key === ' ') && e.target.matches('.sum, .vitem, [role=button]')) { e.preventDefault(); e.target.dispatchEvent(new MouseEvent('click', { bubbles: true })); }
    if (e.key === 'Escape' && e.target.closest('.gwrap')) { const p = e.target.closest('.panel'); delete state.gsel[p.dataset.key]; rerenderTeam(byKey(p.dataset.key)); }
    if (e.key === 'Tab' && e.target.matches('.ytext') && !e.shiftKey) { e.preventDefault(); YAML.insertTab(e.target); }
    if (e.key === 'Enter' && e.target.matches('.in-mission, .in-purpose, .in-name')) e.preventDefault();
  });

  // ---- 共有の部品の受け口 ----------------------------------------------
  const host = {
    hasName: false,
    // Only the ルール panel is a rule document's: the トポロジー panel wears the
    // same .pedit and .yprev — the mock draws the two the same way (spec-teams
    // B7) — but its team.yaml is this screen's own preview, and the shared part
    // must not draw the agreement's Markdown into it.
    row: (panel) => {
      if (!panel || !panel.classList.contains('agree-p')) return null;
      const r = byKey(panel.dataset.key);
      return r ? rowOfPanel(r) : null;
    },
    rerender: (panel, sel) => rerenderAgree(panel, sel),
    edited: (c) => { const p = c && c.panel; const r = p && byKey(p.dataset.key); if (r) scheduleAgree(r, p); },
    readerCount: () => 0,
    filtering: () => false,
    shown: () => true,
    query: () => '',
    path: (l, d) => (d && d.__path) || '',
    mode: (panel, v) => { const r = panel && byKey(panel.dataset.key); if (r) state.pmode[dkey(r)] = v; },
    url: () => syncURL(),
  };
  RD.mount(view, host);

  let wasNarrow = narrow();
  let rt = null;
  window.addEventListener('resize', () => {
    clearTimeout(rt);
    rt = setTimeout(() => {
      setStick();
      if (narrow() !== wasNarrow) { wasNarrow = narrow(); $$('.panel.team-p').forEach((p) => rerenderTeam(byKey(p.dataset.key))); }
      else { $$('.panel.team-p').forEach(placeCard); fitYboxes(); }
    }, 160);
  });
  // Where the save's band stands is a matter of where the page is, so the box
  // is capped again as the page moves (監査 R2). The work is left to one frame.
  let ft = false;
  window.addEventListener('scroll', () => {
    if (ft || state.tab !== 'list') return;
    ft = true;
    requestAnimationFrame(() => { ft = false; fitYboxes(); });
  }, { passive: true });
  window.addEventListener('popstate', () => { state.open = new Set(); readURL(); render(); jumpWantRule(); });

  function take() {
    const teams = [];
    for (const r of recs.values()) if (!r.draftRow && changeCount(r) > 0) teams.push({ place: r.place, name: r.name, hash: r.hash, ops: r.ops });
    const agrees = [];
    for (const r of recs.values()) if (!r.draftRow && r.agree.saved && agreeDiff(r)) agrees.push({ place: r.place, name: r.name, hash: r.agreeHash, doc: RD.toDoc(r.agree.draft, r.agree.saved) });
    const files = FILES.filter((f) => fileDirty(f.file)).map((f) => ({ file: f.file, hash: f.hash, text: state.ytext[f.file] }));
    return { teams, agrees, files };
  }
  window.hergyPage = { screen: 'teams', dirty: false, take, tellDirty };

  render();
  setStick();
  // Every definition is checked once at open, so the figure's problems and the
  // node badges are the server's own reading of the saved file, and the
  // agreement's saved problems are the baseline its save is held to
  // (spec-teams C2, X3).
  const first = [...recs.values()];
  Promise.all(first.flatMap((r) => [checkTeam(r), checkAgree(r)])).then(() => {
    // A rule the address names opens with the cursor in its body, once the
    // panels the first checks redrew have settled (spec-teams 2, spec-roles
    // 1.2.4, 監査 R12).
    jumpWantRule();
  });
})();