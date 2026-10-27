// teams-v2.js — チーム定義の画面（r-0f8c, spec-teams 版 7）。一覧の行はその場で開き、
// 「トポロジー」（team.yaml を点と線で見て直す）と「ルール」（ロールの画面と同じ
// 共有の部品で agreement.yaml を直す）の 2 つのタブを持つ。一覧の上から定義を足し、
// 行ごとに複製・コピー・削除ができる。YAML のタブは両方の置き場所の team.yaml と
// agreement.yaml を文字で見せる。
//
// トポロジーのタブは、ノード編集のカンバス（topocanvas.js）が描く。点や線を
// 選ぶと右の属性の欄に出て、そこから直す。編集は本物の ops で、保存前に
// /api/team/check に当てて検証と YAML のプレビューを受ける。検査の文はすべて
// サーバ（check.Team / check.Agreement）が作るので、この画面は文を作らない。
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
  const SIZE = window.hergyMeasure;
  const PLACES = ['project', 'global'];
  const PLACE = { project: 'プロジェクト', global: 'グローバル' };
  const DIR = DATA.dirs || { project: '', global: '' };
  const PROFILES = DATA.profiles || [];
  // HQ is the headquarters lists (d-73fc): the projects' groups of definitions,
  // roles and profiles. Without it (a server with no headquarters reading) the
  // screen is the one project's, as it always was.
  const HQ = DATA.hq || null;
  // GROUPED is the headquarters with projects: listed: the rows stand in groups
  // and carry their project. Without projects: the screen is the one project
  // the way it always was (d-73fc: the old use is not broken).
  const GROUPED = !!(HQ && HQ.projects && HQ.projects.mode === 'listed');
  const GROUPS = GROUPED ? (HQ.teams || []) : null;
  const groupKeyOf = (g) => (g.global ? 'global' : (g.project ? g.project.path : ''));
  const groupNameOf = (g) => (g.global ? 'グローバル' : (g.project ? g.project.name : ''));
  // currentGroup is the key of the project this serve was started in: a row the
  // screen adds goes there, and a row without a group is its own.
  const CURRENT = HQ ? ((HQ.projects && HQ.projects.current && HQ.projects.current.path) || '') : '';
  const rowsIn = (g) => ((g.list && g.list.items) || []);
  // ROLES_BY and PROFILES_BY are the names each project's member 属性 offers:
  // another project's definition picks from that project's own documents and
  // the global layer's (spec 4 章).
  const ROLES_BY = {};
  const PROFILES_BY = {};
  if (HQ) {
    (HQ.roles || []).forEach((g) => {
      ROLES_BY[groupKeyOf(g)] = rowsIn(g).map((item) => {
        const places = item.places || {}, eff = item.effective || (places.project ? 'project' : 'global');
        return { name: item.name || '', title: ((places[eff] || {}).doc || {}).title || '' };
      }).filter((x) => x.name);
    });
    (HQ.profiles || []).forEach((g) => { PROFILES_BY[groupKeyOf(g)] = rowsIn(g).map((item) => item.name).filter(Boolean); });
  }
  if (HQ) document.body.classList.add('hq-teams');
  const CANVAS = window.hergyCanvas;
  // ROLES is the role documents the member's 属性 offers: the same list the
  // role screen shows, with the title each document carries. It is read once
  // from GET /api/role; the team answer itself carries no role documents.
  let ROLES = [];
  // rolesIn turns true once the roles a new member is named after have answered:
  // until then ＋メンバー and the blank space's メンバーを足す cannot be pressed,
  // since a member added before them has no role (d-c5fe). A read that fails lets
  // them be pressed as before.
  let rolesIn = false;
  const rolesCame = () => { if (rolesIn) return; rolesIn = true; if ($('.panel.team-p')) render(); };
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
  const narrow = () => window.innerWidth <= 760;

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
  // projectIn is the project a write names: a listed project's path, and nothing
  // for the global layer (which the server writes without being told) or for this
  // serve's own project.
  const projectIn = (project) => (project && project !== 'global' ? { project } : {});
  const teamIn = (r, extra) => Object.assign({ place: r.place, name: r.name, part: 'team' }, projectIn(r.project), draftIn(r), extra || {});
  const agreeIn = (r, extra) => Object.assign({ place: r.place, name: r.name, part: 'agreement' }, projectIn(r.project), draftIn(r), extra || {});

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
        case 'addMember': if (!t.members.some((m) => m.name === op.member)) t.members.push({ name: op.member, role: op.role, profile: op.profile }); break;
        case 'removeMember': t.members = t.members.filter((m) => m.name !== op.member); t.turns = t.turns.filter(([a, b]) => a !== op.member && b !== op.member); t.workers = t.workers.filter((w) => w.from !== op.member); break;
        case 'renameMember': {
          const m = t.members.find((m) => m.name === op.member);
          if (m) m.name = op.new;
          t.turns = t.turns.map(([a, b]) => [a === op.member ? op.new : a, b === op.member ? op.new : b]);
          t.workers.forEach((w) => { if (w.from === op.member) w.from = op.new; });
          break;
        }
        case 'moveWorker': { const w = t.workers.find((w) => w.from === op.from && w.purpose === op.purpose); if (w) w.from = op.to; break; }
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

  const domOf = (name, group) => (GROUPED ? name + '|' + (group || '') : name);
  const groupByKey = (key) => (GROUPS || []).find((g) => groupKeyOf(g) === key) || null;

  // defOf is one item of GET /api/team (or of one group of GET /api/hq/teams)
  // read as the record the screen draws: the definition as one place reads it,
  // the team's own agreement document, and the number of problems the server
  // counted for the row (api.js lays the list out, so this screen reads no
  // answer field by hand). project is the group the row stands in: a listed
  // project's path, "global", or empty for the one this serve was started in.
  function defOf(item, project) {
    project = project || '';
    const places = item.places || {};
    const eff = item.effective || (places.project ? 'project' : 'global');
    const place = places[eff] || {};
    const doc = place.doc || {};
    const agree = doc.agreement || { path: '', hash: '', has: false, doc: null };
    const sum = item.summary || {};
    const dom = domOf(item.name || '', project);
    const key = dom + '.' + eff;
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
      name: item.name || '', dom, project, place: eff, places: PLACES.filter((p) => places[p]), key,
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
      // The list's own size and budget, the same pair the figure's check
      // answers (d-00a9, r-2f83). A file that does not read has no size and
      // draws 大きさ —.
      size: sum.size || 0, limit: sum.limit || 0,
      problems: item.problems || {}, report: null, check: null, req: 0, areq: 0,
    };
    recs.set(key, r);
    return r;
  }
  const byKey = (k) => recs.get(k);
  const byName = (n) => [...recs.values()].find((r) => r.dom === n);
  // byRef is the record one address names: the row of one group, or, with no
  // group, the row the screen wrote before the headquarters (its name alone).
  const byRef = (name, group) => (group ? byName(domOf(name, group)) : (byName(name) || byRealName(name)));
  // byRealName is the first row of a name, the project's own first: a link the
  // older screens wrote names a definition, not a group.
  const byRealName = (name) => [...recs.values()].find((r) => r.name === name && (r.project === CURRENT || r.project === ''))
    || [...recs.values()].find((r) => r.name === name);
  // groupOfRecord is the group a writable row belongs to: its own project, the
  // global layer, or, for a row that is not there yet, this serve's project.
  // Without projects: the screen writes to the one project, as it always did.
  const groupOfRecord = (r) => (GROUPED ? (r.project || (r.place === 'global' ? 'global' : CURRENT)) : '');
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
  function draftRecord(how, place, name, base, project) {
    const r = {
      dom: '~new' + (++uids), draftRow: true, how, base: base || null, project: project || '',
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
  async function addDraft(how, place, name, base, project) {
    const r = draftRecord(how, place, name, base, project);
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
    state.open.delete(r.dom); delete state.dtab[r.dom]; CANVAS.reset(dkey(r));
    delete state.confirm['del' + dkey(r)];
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
      return { sev: p.sev, field, node, worker, text: p.text, role: p.role || '', ruleID: p.rule_id || '' };
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
      else if (op.kind === 'moveWorker' && op.to === from && op.purpose === purpose) from = op.from;
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
      if (w.from !== o.from || w.purpose !== o.purpose || w.profile !== sw.profile) n++;
    });
    wasW.forEach((sw, k) => { if (!claimed.has(k)) n++; });
    const wasM = new Map((s.members || []).map((m) => [m.name, m]));
    const nowM = new Map((t.members || []).map((m) => [m.name, m]));
    nowM.forEach((m, name) => {
      const sm = wasM.get(name);
      if (!sm) n++;
      else if (sm.role !== m.role || sm.profile !== m.profile) n++;
    });
    wasM.forEach((m, name) => { if (!nowM.has(name)) n++; });
    return n;
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
    // A finding about a role's own rule opens that role at the rule: the name
    // the server put on the finding (d-8bbf). A finding that names no role
    // still opens the roles screen, the way it did.
    const roleHref = (m) => {
      if (!m.role) return '/roles';
      const q = `open=${encodeURIComponent(m.role)}`;
      return m.ruleID ? `/roles?${q}&amp;rule=${encodeURIComponent(m.ruleID)}` : `/roles?${q}`;
    };
    return `<div class="vhead"><span class="vt">validate</span>${issues.length ? `<span class="vcount">${counts(issues)}</span>` : '<span class="vclean">✓ 問題なし</span>'}</div>`
      + own.map((m) => `<button class="vitem" data-act="vpick" ${m.node ? `data-node="${esc(m.node)}"` : ''} ${m.worker != null && m.worker >= 0 ? `data-worker="${m.worker}"` : ''} ${m.field === 'name' ? 'data-name-field="1"' : ''}>${sevChip(m.sev)}<span class="lno">${esc(where(m))}</span><span class="msg">${esc(m.text)}</span></button>`).join('')
      + (teamOut.length ? `<div class="vsub">チームのルール</div>` + teamOut.map((m) => `<button class="vitem" data-act="vrule" data-name="${esc(r.dom)}" data-id="${esc(ruleID(m))}">${sevChip(m.sev)}<span class="lno">${file(m)}</span><span class="msg">${esc(m.text)}</span></button>`).join('') : '')
      + (out.length ? `<div class="vsub">ロールの文書</div>` + out.map((m) => `<a class="vitem" href="${roleHref(m)}">${sevChip(m.sev)}<span class="lno">${file(m)}</span><span class="msg">${esc(m.text)}</span></a>`).join('') : '');
  }

  // teamPanelHTML is the トポロジー tab: the canvas and its attribute column
  // (topocanvas.js), the check column under the canvas, and the save's band
  // (spec-teams B7). The column's 属性 / YAML tabs pick what stands on the right.
  function teamPanelHTML(r) {
    const k = dkey(r);
    const head = `<header class="ph"><code class="path">${esc(r.teamPath)}</code>
      <span class="phr">${(r.places || []).length > 1 ? `<span class="state ${r.place === 'global' ? 'off' : 'on'}">${r.place === 'global' ? '隠れている' : '効いている'}</span>` : ''}</span></header>`;
    // A team.yaml that does not read draws no figure and offers no shape save:
    // the check column alone says what stopped it, and the YAML tab is where the
    // text is put right (spec-teams 1.9).
    if (r.broken) return `<section class="panel team-p" data-key="${k}">
    ${head}
    <div class="gbody"><div class="vlist">${vlistHTML(r)}</div></div>
  </section>`;
    return `<section class="panel team-p" data-key="${k}">
    ${head}
    ${CANVAS.panel(r)}
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
    const marks = lineMarks(r), cur = CANVAS.sel(r), n = at.length;
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
    if (want < 0) { const cur = CANVAS.sel(r); if (cur) { for (let i = 0; i < at.length; i++) if (isCurLine(at[i], cur)) { want = i; break; } } }
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
  // nothing saved to go back to (spec-teams B2). The ルール tab draws that same
  // row's foot — either tab's 保存 writes the whole folder — so the press the
  // save carries and the key its 保存しました stands under are the caller's
  // (spec-teams B6, 監査 R9).
  function footHTML(r, o) {
    const n = changeCount(r), k = dkey(r);
    const okKey = (o && o.ok) || 't' + k, saveAct = (o && o.act) || 't-save';
    // 保存しました stays until the next change writes over it, so a check that
    // answers just after the save does not take the notice away.
    const ok = state.ok[okKey] ? '<span class="ok">保存しました</span>' : '';
    // The gate is the saved team.yaml's own errors (spec-teams 1.3, X3): a save
    // stops only when an error the file did not hold has appeared.
    const issues = teamIssues(r);
    const before = new Set((r.baseFindings || []).filter((m) => m.sev === 'err').map((m) => m.text));
    // A member with no turn and a definition no turn reaches the human from
    // still save (r-df77, d-9abb): they stay as the definition's problem and
    // stop up, but a save is where work in progress is kept. Those findings
    // stand on a node (field 'node'); every other new error stops the save.
    const grew = issues.some((m) => m.sev === 'err' && m.field !== 'node' && !before.has(m.text));
    if (r.draftRow) {
      // A row that is not there yet is saved only when its name is one the
      // server will take: the name's problems are the answer's own, and while
      // the first answer is still out nothing is written from the row
      // (spec-teams B2, N2).
      const named = r.seeded && !(r.nameIssues || []).length;
      return `<footer class="pf">
      <button class="btn primary" data-act="${saveAct}" ${grew || !named ? 'disabled' : ''}>保存</button><span class="dirty">新規</span>${ok}
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
    const open = `<section class="panel agree-p ${shadowed ? 'shadowed' : ''}" data-uid="${r.uid}" data-l="${r.place}" data-key="${dkey(r)}" data-mode="${mode}">
    <header class="ph"><code class="path">${esc(r.agreePath)}</code>
      <span class="phr">
        ${both ? `<span class="state ${shadowed ? 'off' : 'on'}">${shadowed ? '隠れている' : '効いている'}</span>` : ''}
        <span class="seg pview">${[['edit', '編集'], ['md', 'Markdown']].map(([v, label]) => `<button class="${mode === v ? 'on' : ''}" data-act="pmode" data-v="${v}" aria-pressed="${mode === v}">${label}</button>`).join('')}</span>
      </span></header>
    <div class="pmain">${RD.body(rowOfPanel(r))}</div>`;
    // A definition that is not there yet draws the shape tab's own foot here,
    // too: either tab's 保存 writes the whole folder, so the ルール tab carries
    // the same words — 新規 and the files the save would write — and the name is
    // what lets it be pressed (spec-teams B2, B6, 監査 R9).
    if (r.draftRow) return `${open}
    ${footHTML(r, { ok: 'a' + dkey(r), act: 'save' })}
  </section>`;
    const ok = state.ok['a' + dkey(r)] ? (delete state.ok['a' + dkey(r)], '<span class="ok">保存しました</span>') : '';
    return `${open}
    <footer class="pf">
      <button class="btn primary" data-act="save">保存</button><span class="errline doc" hidden></span><span class="dirty"></span><span class="vcount"></span>${ok}
      <span class="acts"><button class="btn" data-act="revert" ${agreeDiff(r) > 0 ? '' : 'hidden'}>元に戻す</button></span>
    </footer></section>`;
  }
  function rowOfPanel(r) {
    const e = r.agree;
    return {
      uid: r.uid, l: r.place, e, saved: e.saved, draft: e.draft, name: r.name,
      // newRow says the definition is not there yet: the rule document's shared
      // foot is not this row's own, so the part leaves it alone (監査 R9).
      newRow: !!r.draftRow,
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
    // (spec-teams B2, B3). A saved row says which project it is (spec 4 章).
    let pills;
    if (r.draftRow && r.how === 'copy') pills = `<span class="pill pending">${PLACE[r.place]}</span>`;
    else if (r.draftRow) pills = `<span class="seg" role="group" aria-label="置き場所">${PLACES.map((p) => `<button class="${p === r.place ? 'on' : ''}" data-act="pick-place" data-x="${p}" aria-pressed="${p === r.place}">${PLACE[p]}</button>`).join('')}</span>`;
    else pills = placePills(r);
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
    <span class="c-agree c-num">${rules.length}${outNote(outCount(rules))}</span>
    <span class="c-size">${broken || !r.limit ? '<span class="none">—</span>' : SIZE.html(r.size, r.limit)}</span>
    ${HQ && !r.draftRow ? `<span class="hq-runc">${runCellHTML(r)}</span>` : ''}`;
  }

  // placePills is the row's プロジェクト column (spec 4 章): the project's name,
  // or グローバル. A project that overrides a global name carries the crossed-out
  // グローバル, and the global row says which projects override it.
  function placePills(r) {
    // Without projects: the list stays one, and its プロジェクト column still
    // names the project or グローバル (d-8a45, r-5b96 audit 5).
    if (!GROUPED) return (r.places || []).map((p) => `<span class="pill ${p === r.place ? 'on' : 'shadow'}">${p === 'global' ? 'グローバル' : esc(currentProjectName())}</span>`).join('');
    const hq = window.hergyHQ;
    const label = r.project === 'global' ? 'グローバル' : (hq ? hq.projectName(r.project) : '');
    let out = `<span class="pill on">${esc(label)}</span>`;
    const globalGroup = (GROUPS || []).find((g) => g.global);
    const over = (globalGroup && globalGroup.overridden_by) || {};
    if (r.project && r.project !== 'global' && (over[r.name] || []).includes(r.project)) out += '<span class="pill shadow">グローバル</span>';
    if (r.project === 'global') out += (over[r.name] || []).map((p) => `<span class="pill hq-ov">${esc(hq ? hq.projectName(p) : p)} で上書き</span>`).join('');
    return out;
  }

  // runCellHTML is one row's 動き: the running teams, or 起動… (spec 5 章). The
  // headquarters holds the reading; the screen only puts it in the row.
  function runCellHTML(r) {
    const hq = window.hergyHQ;
    if (!hq) return '';
    // A project's team runs in that project: without projects:, it is the one
    // this serve was started in (r-5b96 audit 5).
    const project = r.place === 'global' ? '' : (r.project || CURRENT);
    return hq.runCell(r.name, r.place, project);
  }
  // currentProjectName is the name of the project serve was started in, as the
  // band's sheet calls it.
  function currentProjectName() {
    const p = (HQ && HQ.projects) || {};
    return (p.current && p.current.name) || '';
  }
  // groupHeaderHTML is one project's group's head (spec 4 章): its name, where it
  // is, and how many definitions and running teams it holds. The global layer
  // comes last, with the one sentence about being used from every project.
  function groupHeaderHTML(g, extra) {
    const key = groupKeyOf(g);
    const live = window.hergyHQ ? (window.hergyHQ.state.running.teams || []).filter((t) => t.project === key).length : 0;
    if (g.global) {
      return `<div class="hq-gh global" data-group="global"><span class="hq-gn">グローバル</span><span class="hq-gp">${esc(g.display || '')}</span><span class="hq-gc">どのプロジェクトからも使える。同じ名前があれば、そのプロジェクトのほうが使われる</span></div>`;
    }
    return `<div class="hq-gh" data-group="${esc(key)}"><span class="hq-gn">${esc(groupNameOf(g))}</span><span class="hq-gp">${esc(g.project ? g.project.display : '')}</span><span class="hq-gc">${extra !== undefined ? extra : `チーム定義 <b>${rowsIn(g).length}</b> · 動いているチーム <b>${live}</b>`}</span></div>`;
  }
  // rowsOfGroup is the rows one group draws, in the list's own order: a row that
  // is not there yet has no group yet and stands in this serve's project (or the
  // global layer, when its place is global).
  function rowsOfGroup(key) {
    return [...recs.values()].filter((r) => (r.project ? r.project === key : key === (r.place === 'global' ? 'global' : CURRENT)));
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
    if (!recs.size && !GROUPED) return `${toolbar}<div class="list"><p class="none">チーム定義はありません</p></div>`;
    const head = `<div class="head"><span></span><span>名前</span><span>使命</span><span>${HQ ? 'プロジェクト' : '置き場所'}</span><span>メンバー</span><span>ワーカー</span><span>ルール</span><span>大きさ</span>${HQ ? '<span class="hq-runh">動き</span>' : ''}</div>`;
    if (!GROUPED) return `${toolbar}<div class="list">${head}
    ${[...recs.values()].map(rowHTML).join('')}</div>`;
    // The rows stand in groups: every listed project in the config's order, then
    // the global layer once (spec 4 章).
    let body = '';
    let any = false;
    GROUPS.forEach((g) => {
      const rows = rowsOfGroup(groupKeyOf(g));
      body += groupHeaderHTML(g);
      if (rows.length) { any = true; body += rows.map(rowHTML).join(''); }
    });
    if (!any) body += '<p class="none">チーム定義はありません</p>';
    return `${toolbar}<div class="list hq-grouped">${head}${body}</div>`;
  }
  // refreshRunCells puts the headquarters reading in the 動き cells: only the
  // cells move, so a field a person is typing in keeps its place (spec 5 章).
  function refreshRunCells() {
    if (!HQ) return;
    $$('.tdef[data-key]').forEach((row) => {
      const r = byKey(row.dataset.key);
      const cell = row.querySelector(':scope > .sum > .hq-runc');
      if (!r || !cell) return;
      const html = runCellHTML(r);
      if (cell.innerHTML !== html) cell.innerHTML = html;
    });
    $$('.hq-gh[data-group]').forEach((head) => {
      const key = head.dataset.group;
      const g = GROUPS.find((x) => groupKeyOf(x) === key);
      const cell = head.querySelector('.hq-gc');
      if (!g || !cell) return;
      const live = ((window.hergyHQ && window.hergyHQ.state.running.teams) || []).filter((t) => t.project === key).length;
      if (g.global) return;
      cell.innerHTML = `チーム定義 <b>${rowsIn(g).length}</b> · 動いているチーム <b>${live}</b>`;
    });
  }
  document.addEventListener('hergy:hq', refreshRunCells);
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
    const mi = q('.in-mission'); if (mi && document.activeElement !== mi) mi.value = t.mission;
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
    label: PLACE[f.place] || '', path: f.path || '', hash: f.hash || '',
    text: f.text || '', hidden: !!f.hidden, problems: f.problems || [],
  }));
  const fileByKey = new Map(FILES.map((f) => [f.file, f]));
  const recFor = (f) => {
    const r = [...recs.values()].find((x) => x.name === f.name && x.place === f.place && (x.project === CURRENT || x.project === ''));
    return r || null;
  };
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
        return `<button class="file ${state.yfile === f.file ? 'on' : ''} ${f.hidden ? 'hid' : ''}" data-act="yfile" data-file="${esc(f.file)}" aria-pressed="${state.yfile === f.file}"><code>${esc(f.name)}/${f.kind === 'agreement' ? 'agreement.yaml' : 'team.yaml'}</code><span class="fs">${fileDirty(f.file) ? '<span class="dirty">●</span>' : ''}${counts(yIssues(f.file)) || n}</span><span class="ft">${f.hidden ? '隠れている' : esc(f.path)}</span></button>`;
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
  // With no team definition at all, the YAML tab has no file to show. It says
  // what is missing and the one thing that makes the files (n-f2ef).
  const yamlEmptyHTML = () => `<section class="yed yempty">
    <p>チーム定義はまだありません。</p>
    <p>一覧の『＋ チーム定義を追加』で作ると、ここに team.yaml と agreement.yaml が並びます。</p>
    <span><button class="btn" data-act="tab" data-t="list">一覧へ</button></span></section>`;
  const yamlTabHTML = () => (FILES.length ? `<div class="ylay">${filesHTML()}${yedHTML()}</div>` : yamlEmptyHTML());

  // applySizes puts the width a person computed on the bar measure.js draws.
  // The page's CSP refuses a style="…" written as markup, so it is set through
  // the stylesheet object after the row is drawn (the roles screen does the same).
  function applySizes(root) { $$('[data-w]', root).forEach((el) => { el.style.width = el.dataset.w + '%'; }); }

  // ---- 描く ------------------------------------------------------------
  function render() {
    $('#tabs').innerHTML = [['list', '一覧'], ['yaml', 'YAML']]
      .map(([t, label]) => `<button class="tab ${state.tab === t ? 'on' : ''}" data-act="tab" data-t="${t}" aria-pressed="${state.tab === t}">${label}</button>`).join('');
    view.innerHTML = state.tab === 'list' ? listHTML() : yamlTabHTML();
    applySizes(view);
    setStick();
    if (state.tab === 'list') {
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
    panel.outerHTML = teamPanelHTML(r);
    const fresh = $(`.panel.team-p[data-key="${CSS.escape(dkey(r))}"]`);
    if (fresh) {
      fitYbox(fresh);
      afterTPrev(r);
      if (sel) { const el = fresh.querySelector(sel); if (el) el.focus({ preventScroll: true }); }
    }
    refreshRow(r.dom);
  }
  // softTeam draws again what a person's typing moves without taking the field
  // they type in away (spec-teams 1.4.4, 監査 R7): the canvas, the validate
  // column, the YAML preview and the foot. The attribute column is left as it
  // is, so the cursor stays in its field.
  function softTeam(r) {
    const panel = $(`.panel.team-p[data-key="${CSS.escape(dkey(r))}"]`);
    if (!panel) { refreshRow(r.dom); return; }
    const cv = $('.cv-canvas', panel);
    if (cv) cv.innerHTML = CANVAS.canvas(r);
    const vl = $('.vlist', panel);
    if (vl) vl.innerHTML = vlistHTML(r);
    const pv = $('.tprev', panel);
    if (pv) pv.outerHTML = tprevHTML(r);
    const ft = $('footer.pf', panel);
    if (ft) ft.outerHTML = footHTML(r);
    // The mission stands in two places (the row and the canvas's 属性); the
    // field the person is not typing in follows the one they are.
    const cm = $('.cv-side [data-cv-f="mission"]', panel);
    if (cm && document.activeElement !== cm) cm.value = r.team.draft.mission;
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
    if (!state.open.has(r.dom)) { row.outerHTML = rowHTML(r); applySizes(view); return; }
    const team = $(`.panel.team-p`, row);
    if (team) team.outerHTML = teamPanelHTML(r);
    const ag = $(`.panel.agree-p`, row);
    if (ag) ag.outerHTML = agreePanelHTML(r);
    $$('.dtabs', row).forEach((n) => { n.innerHTML = dtabsNav(r); });
    $$('.panel.team-p', row).forEach(fitYbox);
    afterTPrev(r);
    $$('.panel.agree-p', row).forEach(markPanel);
    row.classList.toggle('unsaved', dirtyDef(r));
    refreshRow(r.dom);
  }
  function rerenderRow(r) {
    const row = $(`.tdef[data-name="${CSS.escape(r.dom)}"]`);
    if (row) { row.outerHTML = rowHTML(r); applySizes(view); }
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
    // Each member's prompt, cut into the pieces it was built from (r-4cf9),
    // follows the ops the check was sent with: the 属性 column shows it without
    // building a line of its own.
    // The check answers members only when hergy itself can build the prompts: a
    // definition with a problem is not given any, and the column says why
    // (d-20ac). The old lines are not kept.
    r.prompts = Array.isArray(json.members) ? json.members : [];
    r.promptsReady = !!(json.members && json.members.length);
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
    const typed = act && act.matches && (act.matches('.in-mission') || !!act.closest('.cv-side')) && act.closest('.tdef') === row;
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
    setProblems(json.items, r.project);
    state.ok['t' + dkey(r)] = 1;
    CANVAS.reset(dkey(r));
    cundo.delete(dkey(r));
    rerenderRow(r);
    tellDirty();
    API.saved();
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
    setProblems(json.items, r.project);
    state.ok['a' + dkey(r)] = 1;
    rerenderSides(r);
    tellDirty();
    API.saved();
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
    [...recs.values()].filter((x) => x.dom === dom).forEach((x) => {
      recs.delete(x.key); delete state.confirm['del' + x.key]; CANVAS.reset(x.key);
    });
    setProblems(json.items, groupOfRecord(r));
    const item = (json.items || []).find((it) => it.name === name);
    if (!item) { state.open.delete(dom); delete state.dtab[dom]; render(); return; }
    const nr = defOf(item, groupOfRecord(r));
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
    API.saved();
  }
  // doDelete takes one definition's folder out of one place (spec-teams B6): its
  // team.yaml and its agreement.yaml, and the directory itself when the two files
  // were all it held. Every record of that name is read again from the answer,
  // because the other place's reading may be the one in effect now.
  async function doDelete(r, panel) {
    const res = await API.del('team', r.name, r.place, r.hash, r.agreeHash, r.project === 'global' ? undefined : r.project);
    if (!res.ok) { API.refusal(panel, API.problemText(res, '削除できませんでした')); return; }
    const json = API.answer(res);
    const name = r.name;
    // The row keeps its place in the list: taking one place's folder away leaves
    // the definition where a person was looking at it (spec-teams B6, N7).
    const at = firstIndexOf((x) => x.dom === r.dom);
    [...recs.values()].filter((x) => x.dom === r.dom).forEach((x) => {
      recs.delete(x.key); delete state.confirm['del' + x.key]; CANVAS.reset(x.key);
    });
    setProblems(json.items, groupOfRecord(r));
    const item = (json.items || []).find((it) => it.name === name);
    if (item) {
      const nr = defOf(item, groupOfRecord(r));
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
    if (open) {
      const rows = window.hergyHQ ? window.hergyHQ.parseOpen(open) : open.split(',').filter(Boolean).map((name) => ({ name, group: '' }));
      rows.forEach(({ name, group }) => { const r = byRef(name, group); if (r) state.open.add(r.dom); });
    }
    const agree = p.get('agree');
    if (agree) agree.split(',').filter(Boolean).forEach((n) => { state.dtab[n] = 'agree'; });
    const rule = p.get('rule');
    state.wantRule = null;
    if (rule) rule.split(',').filter(Boolean).forEach((spec) => {
      const i = spec.indexOf('.');
      if (i < 0) return;
      const r = byRealName(spec.slice(0, i));
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
  function setProblems(items, project) {
    (items || []).forEach((item) => {
      const places = item.places || {};
      const eff = item.effective || (places.project ? 'project' : 'global');
      const r = recs.get(domOf(item.name || '', project || '') + '.' + eff);
      if (r) r.problems = item.problems || {};
    });
  }

  // ---- the canvas's own operations (topocanvas.js) ---------------------
  // A change the canvas asks for is one operation, however many ops it takes:
  // the line × removes both directions, and a member removed takes its turns
  // and workers with it. The undo the person gets is the operation before it
  // (d-9abb, the mock of 2026-10-23). A field's typing is one operation from the
  // first key to the moment the field is left.
  const cundo = new Map();
  let actSeq = 0;
  let ctyping = { key: null, field: null };
  let toastRow = null, toastTimer = null;
  const cloneOps = (ops) => JSON.parse(JSON.stringify(ops || []));
  function canvasUndoPoint(r, label) {
    const k = dkey(r), st = cundo.get(k) || [];
    st.push({ ops: cloneOps(r.ops), label });
    while (st.length > 80) st.shift();
    cundo.set(k, st);
  }
  function canvasToast(r, msg, withUndo) {
    let el = $('.cv-toast');
    if (!el) { el = document.createElement('div'); el.className = 'cv-toast'; document.body.appendChild(el); }
    toastRow = r;
    el.innerHTML = `<span>${esc(msg)}</span>${withUndo ? '<button type="button" data-act="cv-undo">取り消す</button>' : ''}`;
    el.hidden = false;
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => { el.hidden = true; toastRow = null; }, 4000);
  }
  document.addEventListener('click', (e) => {
    const b = e.target.closest && e.target.closest('[data-act="cv-undo"]');
    if (b && toastRow) canvasUndo(toastRow);
  });
  function canvasAct(r, ops, o) {
    o = o || {};
    r.lastAct = ++actSeq;
    r.promptsReady = undefined;
    delete state.ok['t' + dkey(r)]; delete state.ok['a' + dkey(r)];
    canvasUndoPoint(r, o.undo || '');
    (ops || []).forEach((op) => pushOp(r, op));
    ctyping = { key: null, field: null };
    r.team.draft = applyOps(r.team.saved, r.ops);
    rerenderSides(r);
    checkTeam(r);
    if (o.message) canvasToast(r, o.message, true);
    if (o.focus) setTimeout(() => {
      const p = $(`.panel.team-p[data-key="${CSS.escape(dkey(r))}"]`);
      const el = p && $(o.focus, p);
      if (el) { el.focus(); el.select && el.select(); }
    }, 0);
  }
  function canvasTyping(r, op, field) {
    const k = dkey(r);
    r.lastAct = ++actSeq;
    r.promptsReady = undefined;
    delete state.ok['t' + k]; delete state.ok['a' + k];
    if (ctyping.key !== k || ctyping.field !== field) {
      const label = { mission: '使命を戻した', 'm-name': '名前を戻した', 'w-purpose': '目的を戻した' }[field] || '元に戻した';
      canvasUndoPoint(r, label);
      ctyping = { key: k, field };
    }
    pushOp(r, op);
    r.team.draft = applyOps(r.team.saved, r.ops);
    softTeam(r);
    debounce('ct' + k, () => checkTeam(r));
  }
  function canvasEndTyping(r) {
    if (!r) return;
    if (ctyping.key === dkey(r)) { ctyping = { key: null, field: null }; checkTeam(r); }
  }
  function canvasUndo(r) {
    const k = dkey(r), st = cundo.get(k);
    r.promptsReady = undefined;
    delete state.ok['t' + k]; delete state.ok['a' + k];
    if (!st || !st.length) return;
    const point = st.pop();
    r.ops = cloneOps(point.ops);
    r.team.draft = applyOps(r.team.saved, r.ops);
    ctyping = { key: null, field: null };
    rerenderSides(r);
    checkTeam(r);
    canvasToast(r, point.label || '元に戻した', false);
  }
  // The worker's prompt is the one hire --dry builds (r-4cf9). Its name and task
  // are chosen at hire, so the screen asks with values that cannot be mistaken
  // for the purpose or another line, and shows <ワーカー名> / <依頼> in their
  // place. The values are the walk's own too, so it can put them back.
  const WRK_NAME = 'zz-worker-name-0', WRK_TASK = 'zzTaskPlaceholder0';
  function workerPrompt(r, i) {
    const w = r.team.draft.workers[i];
    if (!w || !w.purpose || !w.from) return null;
    const k = w.from + '\u0000' + w.purpose;
    r.wprompts = r.wprompts || {};
    if (r.wprompts[k]) return r.wprompts[k];
    r.wpromptReq = r.wpromptReq || {};
    if (r.wpromptReq[k]) return null;
    r.wpromptReq[k] = true;
    API.check('team', { place: r.place, name: r.name, part: 'worker', project: r.project && r.project !== 'global' ? r.project : undefined, doc: { name: WRK_NAME, purpose: w.purpose, task: WRK_TASK, from: w.from } }).then((res) => {
      delete r.wpromptReq[k];
      const ans = res.ok ? API.answer(res) : null;
      r.wprompts[k] = res.ok
        ? { segments: (ans && ans.prompt) || [], name: WRK_NAME, task: WRK_TASK }
        : { segments: [], name: WRK_NAME, task: WRK_TASK, err: API.problemText(res, 'プロンプトを読めませんでした') };
      const panel = $(`.panel.team-p[data-key="${CSS.escape(dkey(r))}"]`);
      if (panel) rerenderTeam(r);
    }).catch(() => { delete r.wpromptReq[k]; });
    return null;
  }
  // rolesFor and profilesFor are the documents one row's member 属性 offers: the
  // row's own project's and the global layer's, so another project's definition
  // chooses from its own names (spec 4 章). Without a headquarters they are the
  // one list the island carries.
  function rolesFor(r) {
    if (!HQ) return ROLES;
    const key = r ? groupOfRecord(r) : CURRENT;
    const all = key === 'global'
      ? Object.values(ROLES_BY).flat()
      : (ROLES_BY[key] || []).concat(ROLES_BY.global || []);
    const seen = new Set();
    return all.filter((x) => !seen.has(x.name) && seen.add(x.name));
  }
  function profilesFor(r) {
    if (!HQ) return PROFILES;
    const key = r ? groupOfRecord(r) : CURRENT;
    const all = key === 'global'
      ? Object.values(PROFILES_BY).flat()
      : (PROFILES_BY[key] || []).concat(PROFILES_BY.global || []);
    const seen = new Set();
    return all.filter((x) => !seen.has(x) && seen.add(x));
  }
  const CH = {
    key: dkey,
    row: (rowEl) => byKey(rowEl.dataset.key),
    draft: (r) => r.team.draft,
    saved: (r) => r.team.saved,
    findings: (r) => teamIssues(r),
    name: (r) => r.name,
    path: (r) => r.teamPath,
    roles: (r) => rolesFor(r),
    rolesIn: () => rolesIn,
    profiles: (r) => profilesFor(r),
    prompts: (r) => r.prompts || [],
    promptBlocked: (r) => r.promptsReady === false,
    workerPrompt,
    toAgree: (r) => {
      const row = $(`.tdef[data-name="${CSS.escape(r.dom)}"]`);
      const tab = row && $('.dtab[data-t=agree]', row);
      if (tab) tab.click();
    },
    validate: (r) => vlistHTML(r),
    yaml: (r) => tprevHTML(r),
    redraw: (r, sel) => rerenderTeam(r, sel),
    act: canvasAct,
    typing: canvasTyping,
    endTyping: canvasEndTyping,
    undo: canvasUndo,
    // The row the newest operation stands on: ⌘Z acts on it even when nothing
    // is picked (監査 1-1).
    undoRow: () => [...recs.values()].filter((x) => state.open.has(x.dom) && (cundo.get(dkey(x)) || []).length)
      .sort((a, b) => (b.lastAct || 0) - (a.lastAct || 0))[0] || null,
    say: (r, msg) => canvasToast(r, msg, false),
  };
  CANVAS.mount(view, CH);

  const state = {
    tab: view.dataset.tab === 'yaml' ? 'yaml' : 'list',
    open: new Set(), dtab: {}, confirm: {},
    ok: {}, yfile: null, ytext: {}, yc: {}, pmode: {}, wantRule: null,
    // The topology tab's preview bookkeeping: the lines drawn last, the lines one
    // edit just moved, and when they lit up (spec-teams B7).
    tlast: {}, tstep: {}, tglow: {}, tglowTimer: {},
  };
  if (GROUPED) GROUPS.forEach((g) => rowsIn(g).forEach((item) => defOf(item, groupKeyOf(g))));
  else (LIST.items || []).forEach((item) => defOf(item));
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
    // A button of the headquarters' 動き column (start, stop) acts on the
    // definition, not on the row: it must not open or close the row (r-5b96
    // audit 1).
    if (e.target.closest('[data-hq]')) return;
    if (e.target.closest('.in-mission, .in-name')) return;
    const b = e.target.closest('[data-act]');
    if (!b) return;
    const act = b.dataset.act, row = b.closest('.tdef'), n = row && row.dataset.name;
    if (act === 'cv-undo') { if (toastRow) canvasUndo(toastRow); return; }
    const tp = b.closest('.panel.team-p'), td = tp && byKey(tp.dataset.key), tk = td && dkey(td);
    const c = ctx(b);
    const re = (sel) => rerenderAgree(c.panel, sel);
    switch (act) {
      case 'tab': state.tab = b.dataset.t; render(); break;
      case 'toggle': if (state.open.has(n)) state.open.delete(n); else state.open.add(n); render(); break;
      // The list's own toolbar adds a definition from the scaffold, and each row
      // duplicates itself, copies itself into the other place, or goes away
      // (spec-teams B1-B5).
      case 'new': await addDraft('new', 'project', '', null, GROUPED ? CURRENT : ''); break;
      case 'dup': { const r = byName(n); if (r) await addDraft('dup', r.place, theName(r.name), { place: r.place, name: r.name }, r.project); break; }
      case 'copy': { const r = byName(n); if (r) await addDraft('copy', other(r.place), r.name, { place: r.place, name: r.name }, r.project); break; }
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
        CANVAS.reset(r.key);
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
      // A press on a line of the preview picks what the line stands for, the way
      // a press on the canvas's own point does (spec-teams B7). A mission line
      // puts the cursor in the mission field of the attribute column.
      case 'tyline': if (td) {
        if (b.dataset.f === 'mission') {
          CANVAS.select(td, null, 'attr');
          rerenderTeam(td);
          const el = $(`.panel.team-p[data-key="${CSS.escape(tk)}"] [data-cv-f="mission"]`);
          if (el) { el.focus({ preventScroll: true }); el.select && el.select(); }
          break;
        }
        if (b.dataset.node) CANVAS.select(td, { type: 'node', id: b.dataset.node }, 'attr');
        else if (b.dataset.worker != null) CANVAS.select(td, { type: 'worker', i: +b.dataset.worker }, 'attr');
        else if (b.dataset.a) CANVAS.select(td, { type: 'edge', k: pairKey(b.dataset.a, b.dataset.b) }, 'attr');
        else CANVAS.select(td, null, 'attr');
        rerenderTeam(td);
      } break;
      case 'vpick':
        if (b.dataset.nameField) { const el = $(`.tdef[data-name="${CSS.escape(n)}"] .in-name`); if (el) { el.focus({ preventScroll: true }); el.select && el.select(); } break; }
        if (b.dataset.node) CANVAS.select(td, { type: 'node', id: b.dataset.node }, 'attr');
        else if (b.dataset.worker != null) CANVAS.select(td, { type: 'worker', i: +b.dataset.worker }, 'attr');
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
      case 't-revert': td.ops = []; CANVAS.reset(tk); cundo.delete(tk); state.confirm = {}; syncTeam(td); rerenderRow(td); break;
      case 'to-yaml': state.tab = 'yaml'; state.yfile = `${b.dataset.key}.team`; render(); window.scrollTo(0, 0); break;
      case 'dtab': state.dtab[n] = b.dataset.t; { const det = $('.detail', row); if (det) det.outerHTML = detailHTML(byName(n)); if (b.dataset.t === 'agree') checkAgree(byName(n)); } $$(`.tdef[data-name="${CSS.escape(n)}"] .panel.team-p`).forEach(fitYbox); $$(`.tdef[data-name="${CSS.escape(n)}"] .panel.agree-p`).forEach(markPanel); refreshRow(n); break;
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
    // A point or a line of the canvas answers its own keys (topocanvas.js);
    // this screen answers the row, the list and the validate column.
    if ((e.key === 'Enter' || e.key === ' ') && !e.target.closest('.cv-canvas') && e.target.matches('.sum, .vitem, [role=button]')) { e.preventDefault(); e.target.dispatchEvent(new MouseEvent('click', { bubbles: true })); }
    if (e.key === 'Tab' && e.target.matches('.ytext') && !e.shiftKey) { e.preventDefault(); YAML.insertTab(e.target); }
    if (e.key === 'Enter' && e.target.matches('.in-mission, .in-name')) e.preventDefault();
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
      else { fitYboxes(); }
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
  // settled turns true once the reads that follow the first frame have answered
  // (the roles, the HQ's roles and profiles, the first checks): each of them
  // draws the screen again, and a member added before the roles came has no role.
  window.hergyPage = { screen: 'teams', dirty: false, settled: false, take, tellDirty };

  render();
  setStick();
  const later = [];
  // The member's 属性 offers the roles the role screen holds; the team answer
  // carries no role documents, so they are read once from GET /api/role. A panel
  // already drawn gets them when the answer comes (the first frame is quick).
  later.push(API.list('role').then((res) => {
    if (!res.ok) return;
    const list = API.answer(res) || {};
    ROLES = (list.items || []).map((it) => {
      const places = it.places || {}, eff = it.effective || (places.project ? 'project' : 'global');
      const doc = (places[eff] || {}).doc || {};
      return { name: it.name || '', title: doc.title || '' };
    }).filter((x) => x.name);
    if (!HQ) rolesIn = true;
    if ($('.panel.team-p')) render();
  }).catch(() => {}).finally(() => { if (!HQ) rolesCame(); }));
  // The roles and profiles a member's 属性 offers are read after the list: the
  // page itself carries only the definitions, so a row opens without waiting
  // for every project's roles and profiles (r-5b96, Enter lag).
  if (HQ) {
    const fill = (kind, map, toEntry) => API.call('GET', '/api/hq/' + kind).then((res) => {
      if (!res.ok || !res.json) return;
      (res.json.groups || []).forEach((g) => {
        map[groupKeyOf(g)] = rowsIn(g).map(toEntry).filter(Boolean);
      });
      if (map === ROLES_BY) rolesIn = true;
      if ($('.panel.team-p')) render();
    }).catch(() => {});
    later.push(fill('roles', ROLES_BY, (item) => {
      const places = item.places || {}, eff = item.effective || (places.project ? 'project' : 'global');
      const name = item.name || '';
      return name ? { name, title: ((places[eff] || {}).doc || {}).title || '' } : null;
    }).finally(rolesCame));
    later.push(fill('profiles', PROFILES_BY, (item) => item.name || null));
  }
  // Every definition is checked once at open, so the figure's problems and the
  // node badges are the server's own reading of the saved file, and the
  // agreement's saved problems are the baseline its save is held to
  // (spec-teams C2, X3).
  const first = [...recs.values()];
  const checks = first.flatMap((r) => [checkTeam(r), checkAgree(r)]);
  Promise.all(checks).then(() => {
    // A rule the address names opens with the cursor in its body, once the
    // panels the first checks redrew have settled (spec-teams 2, spec-roles
    // 1.2.4, 監査 R12).
    jumpWantRule();
  });
  Promise.allSettled([...later, ...checks]).then(() => { window.hergyPage.settled = true; });
})();