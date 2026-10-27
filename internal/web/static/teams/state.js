// teams/state.js — チーム定義の画面の状態と、サーバーとのやり取り
// （webui-preact-design 4 章・5 章）。
//
// 定義の記録（recs）、下書き（ops と draft）、チェックの答え、URL、帯への約束を
// ここが持つ。描くのは view.js / canvas.js / yaml.js で、どれもここを読むだけ。
// 状態は 1 つのオブジェクトと Map のまま（部品 ruledoc が下書きをその場で書き換え、
// 画面も同じ下書きを読む）。変えたら render() を呼ぶ。render() は同期に描く:
// 部品が描き直しの直後に自分の要素を読む（規則を開いて、その本文に caret を置く）
// ので、マイクロタスク待ちにすると要素がまだ無い。
//
// 検査の文はすべてサーバ（check.Team / check.Agreement）が作る。この画面は文を
// 作らない（spec-boundary 3）。
import { signal } from '../vendor/preact.js';
import {
  check as apiCheck, save as apiSave, del as apiDel, saved as apiSaved,
  list as apiList, call as apiCall, problemText as apiProblemText,
} from '../lib/api.js';
import {
  fromJSON, toDoc, rulesOf, diffCount, clone, state as ruleState, observeKeys, goRule,
} from '../lib/ruledoc.js';
import { parseOpen, projectName as hqProjectName, state as hqState } from '../lib/hq.js';
import { setCount, takeDraft } from '../lib/nav.js';

export const view = document.getElementById('view');
export const tabsEl = document.getElementById('tabs');
export const rev = signal(0);

const DATA = JSON.parse((document.getElementById('teams-data') || {}).textContent || '{}');
export const PLACES = ['project', 'global'];
export const PLACE = { project: 'プロジェクト', global: 'グローバル' };
export const DIR = DATA.dirs || { project: '', global: '' };
export const PROFILES = DATA.profiles || [];
// HQ is the headquarters lists (d-73fc): the projects' groups of definitions,
// roles and profiles. Without it (a server with no headquarters reading) the
// screen is the one project's, as it always was.
export const HQ = DATA.hq || null;
// GROUPED is the headquarters with projects: listed: the rows stand in groups
// and carry their project. Without projects: the screen is the one project
// the way it always was (d-73fc: the old use is not broken).
export const GROUPED = !!(HQ && HQ.projects && HQ.projects.mode === 'listed');
export const GROUPS = GROUPED ? (HQ.teams || []) : null;
// currentGroup is the key of the project this serve was started in: a row the
// screen adds goes there, and a row without a group is its own.
export const CURRENT = HQ ? ((HQ.projects && HQ.projects.current && HQ.projects.current.path) || '') : '';
export const rowsIn = (g) => ((g.list && g.list.items) || []);
export const groupKeyOf = (g) => (g.global ? 'global' : (g.project ? g.project.path : ''));
export const groupNameOf = (g) => (g.global ? 'グローバル' : (g.project ? g.project.name : ''));
if (HQ) document.body.classList.add('hq-teams');

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
// ROLES is the role documents the member's 属性 offers: the same list the
// role screen shows, with the title each document carries. It is read once
// from GET /api/role; the team answer itself carries no role documents.
export let ROLES = [];
// rolesIn turns true once the roles a new member is named after have answered:
// until then ＋メンバー and the blank space's メンバーを足す cannot be pressed,
// since a member added before them has no role (d-c5fe). A read that fails lets
// them be pressed as before.
export let rolesIn = false;
export const rolesCame = () => { if (rolesIn) return; rolesIn = true; render(); };
// The island is the same list answer GET /api/team gives (spec-boundary 5):
// the items, and the files the YAML tab can open.
const LIST = DATA.team || { items: [], files: [] };

export const $ = (s, root = document) => root.querySelector(s);
export const $$ = (s, root = document) => Array.from(root.querySelectorAll(s));
export const nodeName = (x) => (x === 'human' ? '人間' : x);
const other = (p) => (p === 'project' ? 'global' : 'project');
export const pairKey = (a, b) => [a, b].sort().join('|');
const hasTurn = (t, a, b) => (t.turns || []).some(([p, q]) => p === a && q === b);
const memberOf = (t, n) => (t.members || []).find((m) => m.name === n);
const turnLabel = (d, t, b) => (b === 'human'
  ? '人間（ターンを渡したら、止まって人の答えを待つ）'
  : `${d.name}-${b}（ロール ${(memberOf(t, b) || {}).role || '?'}）`);
export const narrow = () => window.innerWidth <= 760;

// ---- 状態 ---------------------------------------------------------------
export const state = {
  tab: (view && view.dataset.tab) === 'yaml' ? 'yaml' : 'list',
  open: new Set(), dtab: {}, confirm: {},
  ok: {}, yfile: null, ytext: {}, yc: {}, pmode: {}, wantRule: null,
  // The topology tab's preview bookkeeping: the lines drawn last, the lines one
  // edit just moved, and when they lit up (spec-teams B7).
  tlast: {}, tstep: {}, tglow: {}, tglowTimer: {},
  // refusal is the server's sentence on one row's foot (the shape tab's or the
  // rules tab's), and toast the words the canvas says at the bottom.
  refusal: {}, toast: null,
  // focusAsk is the one field a person's press should land in after the draw
  // (5 章 5): a member just added, a rule just opened.
  focusAsk: null,
};

// render is the screen's draw: the tabs and the view are drawn again from this
// state, synchronously, because the shared part reads its fresh element at once.
let paint = () => {};
export const onDraw = (fn) => { paint = fn; };
export function render() {
  rev.value += 1;
  paint();
  applyFocusAsk();
  setBandCount();
  tellDirty();
}
// applyFocusAsk puts the cursor in the one field a person's press asked for,
// right after the draw (5 章 5). Nothing here puts the focus back after a
// redraw: the element is the same one, so there is nothing to put back.
function applyFocusAsk() {
  const a = state.focusAsk;
  if (!a) return;
  state.focusAsk = null;
  const row = document.querySelector(`.tdef[data-name="${CSS.escape(a.dom)}"]`);
  const el = row && row.querySelector(a.sel);
  if (!el) return;
  el.focus({ preventScroll: true });
  if (el.select) el.select();
}
const debounceTimers = {};
export const debounce = (k, fn, ms = 260) => { clearTimeout(debounceTimers[k]); debounceTimers[k] = setTimeout(fn, ms); };

// dkey is the key the screen's own state and DOM stand on: the definition's
// saved name, or, for a definition that is not there yet (its name is what the
// person is typing), a stable id of its own (spec-teams B2).
export const dkey = (d) => d.dom + '.' + d.place;
export const domOf = (name, group) => (GROUPED ? name + '|' + (group || '') : name);
const groupByKey = (key) => (GROUPS || []).find((g) => groupKeyOf(g) === key) || null;

// ---- サーバとのやり取りの形 ---------------------------------------------
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

// ---- 下書き（姿） ------------------------------------------------------
export const cloneTeam = (t) => ({
  mission: (t && t.mission) || '',
  members: ((t && t.members) || []).map((m) => ({ name: m.name, role: m.role, profile: m.profile })),
  workers: ((t && t.workers) || []).map((w) => ({ from: w.from, purpose: w.purpose, profile: w.profile })),
  turns: ((t && t.turns) || []).map((x) => (Array.isArray(x) ? [x[0], x[1]] : [x.from, x.to])),
});
export function applyOps(saved, ops) {
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

// ---- 記録（1 定義ごと） ------------------------------------------------
const recs = new Map();
let uids = 0;

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
    saved = fromJSON(agree.doc);
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
      draft: saved ? clone(saved) : fromJSON({ name: '', title: (item.name || '') + ' チームのルール', revision: 1, sections: [] }),
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
export const allRecs = () => [...recs.values()];
export const byKey = (k) => recs.get(k);
export const byName = (n) => [...recs.values()].find((r) => r.dom === n);
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
export const groupOfRecord = (r) => (GROUPED ? (r.project || (r.place === 'global' ? 'global' : CURRENT)) : '');
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
export const rowsOfGroup = (key) => [...recs.values()].filter((r) => (r.project ? r.project === key : key === (r.place === 'global' ? 'global' : CURRENT)));

// ---- まだ無い定義（B2, B3） --------------------------------------------
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
      draft: fromJSON({ name: '', title: 'チームのルール', revision: 1, sections: [] }),
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
  if (agree.doc) { const d = fromJSON(agree.doc); observeKeys(rulesOf(d)); d.__path = agree.path; r.agree.saved = d; r.agree.draft = clone(d); }
}
// theName is the name a duplicate takes: the source's with -2, -3 … until one
// is free, so the row starts without the name problem the same name would raise
// (spec-teams B3).
export function theName(base) {
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
export async function addDraft(how, place, name, base, project) {
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
  // The name a copy already holds is chosen, so the first letter a person
  // types replaces it rather than landing beside it (監査 R8).
  state.focusAsk = { dom: r.dom, sel: '.in-name' };
  render();
  return r;
}
// dropDraft takes a row that is not there yet away without writing anything
// (spec-teams B2).
export function dropDraft(r) {
  recs.delete(dkey(r));
  state.open.delete(r.dom); delete state.dtab[r.dom]; canvasReset(dkey(r));
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
export const agreeIssues = (r) => (r.check ? r.check.issues : []);
export const agreeDiff = (r) => diffCount(r.agree);
export const teamIssues = (r) => r.findings;
const pcountList = (c) => [...Array((c && c.err) || 0)].map(() => ({ sev: 'err' })).concat([...Array((c && c.warn) || 0)].map(() => ({ sev: 'warn' })));
// rowIssues is the problems a row's own chip shows: the server's count for a
// saved definition, and the findings the server answered for a definition that
// is not there yet (spec-teams B2).
export const rowIssues = (r) => (r.draftRow ? teamIssues(r) : pcountList(r.problems));
// A definition that is not there yet counts as unsaved whatever it holds: it is
// written only when 保存 is pressed (spec-teams B2).
export const dirtyDef = (r) => r.draftRow || changeCount(r) > 0 || (r.agree.saved ? agreeDiff(r) > 0 : false);

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
export function changeCount(r) {
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
export const vlistParts = (r) => {
  const issues = teamIssues(r);
  const at = (m) => m.field && m.field.startsWith('rule:');
  const out = issues.filter((m) => at(m) && !ownAgreement(r, m));
  const teamOut = issues.filter((m) => at(m) && ownAgreement(r, m));
  const own = issues.filter((m) => !at(m));
  const where = (m) => (m.node ? nodeName(m.node) : m.worker != null && m.worker >= 0 ? ((r.team.draft.workers[m.worker] || {}).purpose || 'ワーカー') : m.field === 'mission' ? '使命' : m.field === 'name' ? '名前' : '');
  const file = (m) => (m.field.split('#')[0] || '').replace(/^rule:/, '').split('/').pop();
  // A finding about a role's own rule opens that role at the rule: the name
  // the server put on the finding (d-8bbf). A finding that names no role
  // still opens the roles screen, the way it did.
  const roleHref = (m) => {
    if (!m.role) return '/roles';
    const q = `open=${encodeURIComponent(m.role)}`;
    return m.ruleID ? `/roles?${q}&rule=${encodeURIComponent(m.ruleID)}` : `/roles?${q}`;
  };
  return { issues, own, teamOut, out, where, file, roleHref, ruleID };
};

// ---- 検査と保存 --------------------------------------------------------
async function checkOf(payload) {
  return apiCall('POST', '/api/team/check', payload);
}
export async function checkTeam(r) {
  const id = ++r.req;
  // A check sent with no ops reads the saved file, so its findings are the
  // baseline the save gate is held to (spec-teams X3). It is taken from the
  // request rather than from the answer's turn, and kept even when a later
  // check has made this answer too old to draw: the file it read is the one a
  // save would write over, and a person who writes before the first answer
  // has come back still gets the file's own problems as the baseline.
  const based = !r.ops.length;
  const res = await checkOf(teamIn(r, { doc: { ops: r.ops } }));
  if (!res.ok) return;
  const json = (res && res.json) || {};
  // The first answer for a definition that is not there yet is the base it
  // stands on, and it is read once (spec-teams B2).
  if (r.draftRow && !r.seeded) seedDraft(r, json);
  const findings = figFindings(r, json.problems || []);
  if (based) r.baseFindings = findings;
  if (id !== r.req) return;
  r.findings = findings;
  // Each member's prompt, cut into the pieces it was built from (r-4cf9),
  // follows the ops the check was sent with: the 属性 column shows it without
  // building a line of its own. A definition with a problem is not given any,
  // and the column says why (d-20ac).
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
  render();
}
// afterAgree re-marks the panel the check answered for: the shared part reads
// the Markdown, the diff and the problems through host.row, so it draws the
// answer without this screen rebuilding the panel (spec-boundary 7).
export async function checkAgree(r) {
  const id = ++r.areq;
  const doc = toDoc(r.agree.draft, r.agree.saved);
  const res = await checkOf(agreeIn(r, { doc: { ops: r.ops, doc } }));
  if (id !== r.areq || !res.ok) return;
  r.check = buildCheck(r.agree.draft, (res && res.json) || {});
  if (!agreeDiff(r)) r.agree.savedIssues = r.check.issues;
  render();
}
export const scheduleAgree = (r) => debounce('a' + r.key, () => syncAgree(r));

export function syncTeam(r) {
  r.team.draft = applyOps(r.team.saved, r.ops);
  render();
  checkTeam(r);
}
export function syncAgree(r) {
  render();
  checkAgree(r);
}
export function pushOp(r, op) { r.ops.push(op); }

// saveTeam writes the team.yaml's ops (spec-teams B6). The refusal is the
// screen's own state: the panel's foot draws it (5 章 2).
export async function saveTeam(r) {
  if (r.draftRow) return saveFolder(r);
  const res = await apiCall('PUT', `/api/team/${encodeURIComponent(r.name)}`, teamIn(r, { hash: r.hash, doc: { ops: r.ops } }));
  if (!res.ok) return refuse(r, 't', apiProblemText(res, '保存できませんでした'));
  const json = (res && res.json) || {};
  r.hash = json.hash || r.hash;
  r.team.saved = cloneTeam(r.team.draft);
  r.savedTurns = cloneTeam(r.team.draft).turns;
  r.ops = [];
  r.baseFindings = r.findings;
  setProblems(json.items, r.project);
  state.ok['t' + dkey(r)] = 1;
  canvasReset(dkey(r));
  cundo.delete(dkey(r));
  render();
  apiSaved();
}
export async function saveAgree(r) {
  if (r.draftRow) return saveFolder(r);
  const doc = toDoc(r.agree.draft, r.agree.saved);
  const res = await apiCall('PUT', `/api/team/${encodeURIComponent(r.name)}`, agreeIn(r, { hash: r.agreeHash, doc: { ops: r.ops, doc } }));
  if (!res.ok) return refuse(r, 'a', apiProblemText(res, '保存できませんでした'));
  const json = (res && res.json) || {};
  r.agreeHash = json.hash || r.agreeHash;
  r.hasAgree = true;
  if (json.doc) { const d2 = fromJSON(json.doc); observeKeys(rulesOf(d2)); d2.__path = r.agreePath; r.agree.saved = d2; r.agree.draft = clone(d2); }
  setProblems(json.items, r.project);
  state.ok['a' + dkey(r)] = 1;
  render();
  apiSaved();
  // The saved text's own problems are the new baseline, and the check the
  // panel shows is the one it now holds.
  checkAgree(r);
}
// saveFolder writes a definition that is not there yet, whole (spec-teams B2,
// B6, C8): its team.yaml and its agreement.yaml, and, for one made from the
// scaffold, the layer's role documents and the starter profile. Either tab's
// 保存 writes the whole folder. The row becomes the saved definition in its
// place, so the person keeps working on it.
async function saveFolder(r) {
  const doc = toDoc(r.agree.draft, r.agree.saved);
  const body = Object.assign(teamIn(r), {
    hash: r.hash, agreement_hash: r.agreeHash, part: 'folder', doc: { ops: r.ops, doc },
  });
  const res = await apiCall('PUT', `/api/team/${encodeURIComponent(r.name)}`, body);
  if (!res.ok) return refuse(r, 't', apiProblemText(res, '保存できませんでした'));
  const json = (res && res.json) || {};
  const name = r.name, dom = r.dom, dtab = state.dtab[dom] || 'shape', opens = state.open.has(dom);
  // The row stands where the row a person was looking at stood: the first
  // reading of this name keeps its place, and a copy's save joins into it
  // (spec-teams B1-B4).
  const at = firstIndexOf((x) => x.dom === dom || x.name === name);
  // Every reading of this name is taken again from the answer: a copy's save
  // joins the two places into one row (spec-teams B4).
  [...recs.values()].filter((x) => x.dom === dom).forEach((x) => {
    recs.delete(x.key); delete state.confirm['del' + x.key]; canvasReset(x.key);
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
  state.ok[(state.dtab[nr.dom] === 'agree' ? 'a' : 't') + dkey(nr)] = 1;
  render();
  syncURL();
  apiSaved();
}
// doDelete takes one definition's folder out of one place (spec-teams B6): its
// team.yaml and its agreement.yaml, and the directory itself when the two files
// were all it held. Every record of that name is read again from the answer,
// because the other place's reading may be the one in effect now.
export async function doDelete(r) {
  const res = await apiCall('DELETE', delURL(r));
  if (!res.ok) return refuse(r, 't', apiProblemText(res, '削除できませんでした'));
  const json = (res && res.json) || {};
  const name = r.name;
  // The row keeps its place in the list: taking one place's folder away leaves
  // the definition where a person was looking at it (spec-teams B6, N7).
  const at = firstIndexOf((x) => x.dom === r.dom);
  [...recs.values()].filter((x) => x.dom === r.dom).forEach((x) => {
    recs.delete(x.key); delete state.confirm['del' + x.key]; canvasReset(x.key);
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
const delURL = (r) => {
  const qs = [`place=${encodeURIComponent(r.place)}`, `hash=${encodeURIComponent(r.hash)}`];
  if (r.agreeHash) qs.push(`agreement_hash=${encodeURIComponent(r.agreeHash)}`);
  if (r.project && r.project !== 'global') qs.push(`project=${encodeURIComponent(r.project)}`);
  return `/api/team/${encodeURIComponent(r.name)}?${qs.join('&')}`;
};
// refuse holds the server's sentence on the row's foot (the shape tab's, or the
// rules tab's) and draws it; a screen builds no sentence of its own.
export function refuse(r, which, text) {
  state.refusal[which + dkey(r)] = text || '保存できませんでした';
  render();
}
export const refusalOf = (r, which) => state.refusal[which + dkey(r)] || '';
export const okOf = (key) => !!state.ok[key];
// okShown takes the notice away when the words it stood for change: a save's
// 保存しました stays until the next change writes over it (1.4.6).
export function clearOk(r) { delete state.ok['t' + dkey(r)]; delete state.ok['a' + dkey(r)]; }
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

// ---- the canvas's own operations --------------------------------------
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
export function canvasToast(r, msg, withUndo) {
  toastRow = r;
  state.toast = { msg, undo: !!withUndo };
  render();
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { state.toast = null; toastRow = null; render(); }, 4000);
}
export function canvasUndoFromToast() { if (toastRow) canvasUndo(toastRow); }
// forgetUndo drops the operations a row could take back (a save, a revert).
export function forgetUndo(r) { cundo.delete(dkey(r)); }
export function canvasAct(r, ops, o) {
  o = o || {};
  r.lastAct = ++actSeq;
  r.promptsReady = undefined;
  clearOk(r);
  canvasUndoPoint(r, o.undo || '');
  (ops || []).forEach((op) => pushOp(r, op));
  ctyping = { key: null, field: null };
  r.team.draft = applyOps(r.team.saved, r.ops);
  if (o.focus) state.focusAsk = { dom: r.dom, sel: o.focus };
  render();
  checkTeam(r);
  if (o.message) canvasToast(r, o.message, true);
}
export function canvasTyping(r, op, field) {
  const k = dkey(r);
  r.lastAct = ++actSeq;
  r.promptsReady = undefined;
  clearOk(r);
  if (ctyping.key !== k || ctyping.field !== field) {
    const label = { mission: '使命を戻した', 'm-name': '名前を戻した', 'w-purpose': '目的を戻した' }[field] || '元に戻した';
    canvasUndoPoint(r, label);
    ctyping = { key: k, field };
  }
  pushOp(r, op);
  r.team.draft = applyOps(r.team.saved, r.ops);
  render();
  debounce('ct' + k, () => checkTeam(r));
}
export function canvasEndTyping(r) {
  if (!r) return;
  if (ctyping.key === dkey(r)) { ctyping = { key: null, field: null }; checkTeam(r); }
}
export function canvasUndo(r) {
  const k = dkey(r), st = cundo.get(k);
  r.promptsReady = undefined;
  clearOk(r);
  if (!st || !st.length) return;
  const point = st.pop();
  r.ops = cloneOps(point.ops);
  r.team.draft = applyOps(r.team.saved, r.ops);
  ctyping = { key: null, field: null };
  render();
  checkTeam(r);
  canvasToast(r, point.label || '元に戻した', false);
}
// The row the newest operation stands on: ⌘Z acts on it even when nothing is
// picked (監査 1-1).
export function undoRow() {
  return [...recs.values()].filter((x) => state.open.has(x.dom) && (cundo.get(dkey(x)) || []).length)
    .sort((a, b) => (b.lastAct || 0) - (a.lastAct || 0))[0] || null;
}
// canvasReset forgets the canvas's picks and places for one row (a save, a
// delete: the figure lays out afresh, d-a575).
let canvasReset = () => {};
export const onCanvasReset = (fn) => { canvasReset = fn; };

// The worker's prompt is the one hire --dry builds (r-4cf9). Its name and task
// are chosen at hire, so the screen asks with values that cannot be mistaken
// for the purpose or another line, and shows <ワーカー名> / <依頼> in their
// place. The values are the walk's own too, so it can put them back.
const WRK_NAME = 'zz-worker-name-0', WRK_TASK = 'zzTaskPlaceholder0';
export function workerPrompt(r, i) {
  const w = r.team.draft.workers[i];
  if (!w || !w.purpose || !w.from) return null;
  const k = w.from + '\u0000' + w.purpose;
  r.wprompts = r.wprompts || {};
  if (r.wprompts[k]) return r.wprompts[k];
  r.wpromptReq = r.wpromptReq || {};
  if (r.wpromptReq[k]) return null;
  r.wpromptReq[k] = true;
  apiCall('POST', '/api/team/check', { place: r.place, name: r.name, part: 'worker', project: r.project && r.project !== 'global' ? r.project : undefined, doc: { name: WRK_NAME, purpose: w.purpose, task: WRK_TASK, from: w.from } }).then((res) => {
    delete r.wpromptReq[k];
    const ans = res.ok ? (res.json || {}) : null;
    r.wprompts[k] = res.ok
      ? { segments: (ans && ans.prompt) || [], name: WRK_NAME, task: WRK_TASK }
      : { segments: [], name: WRK_NAME, task: WRK_TASK, err: apiProblemText(res, 'プロンプトを読めませんでした') };
    render();
  }).catch(() => { delete r.wpromptReq[k]; });
  return null;
}
// rolesFor and profilesFor are the documents one row's member 属性 offers: the
// row's own project's and the global layer's, so another project's definition
// chooses from its own names (spec 4 章). Without a headquarters they are the
// one list the island carries.
export function rolesFor(r) {
  if (!HQ) return ROLES;
  const key = r ? groupOfRecord(r) : CURRENT;
  const all = key === 'global'
    ? Object.values(ROLES_BY).flat()
    : (ROLES_BY[key] || []).concat(ROLES_BY.global || []);
  const seen = new Set();
  return all.filter((x) => !seen.has(x.name) && seen.add(x.name));
}
export function profilesFor(r) {
  if (!HQ) return PROFILES;
  const key = r ? groupOfRecord(r) : CURRENT;
  const all = key === 'global'
    ? Object.values(PROFILES_BY).flat()
    : (PROFILES_BY[key] || []).concat(PROFILES_BY.global || []);
  const seen = new Set();
  return all.filter((x) => !seen.has(x) && seen.add(x));
}

// ---- YAML のタブ ------------------------------------------------------
export const FILES = (LIST.files || []).map((f) => ({
  file: `${f.name}.${f.place}.${f.kind}`, place: f.place, name: f.name, kind: f.kind,
  label: PLACE[f.place] || '', path: f.path || '', hash: f.hash || '',
  text: f.text || '', hidden: !!f.hidden, problems: f.problems || [],
}));
export const fileByKey = new Map(FILES.map((f) => [f.file, f]));
export const fileText = (file) => (state.ytext[file] !== undefined ? state.ytext[file] : ((fileByKey.get(file) || {}).text || ''));
export const fileDirty = (file) => state.ytext[file] !== undefined && state.ytext[file] !== ((fileByKey.get(file) || {}).text || '');
export const yIssues = (file) => state.yc[file] || (fileByKey.get(file) || {}).problems || [];
export const recForFile = (f) => [...recs.values()].find((x) => x.name === f.name && x.place === f.place && (x.project === CURRENT || x.project === '')) || null;
// The file nav chooses one file when the tab opens: the first the places hold.
export function pickFile() {
  const keys = PLACES.flatMap((p) => FILES.filter((f) => f.place === p).map((f) => f.file));
  if (!state.yfile || !keys.includes(state.yfile)) state.yfile = keys[0] || null;
  return state.yfile;
}

// promptsOf is each member's prompt, cut into the pieces it was built from
// (r-4cf9): the check answers it, and the column shows it without building a
// line of its own. A definition with a problem is not given any (d-20ac).
export const promptsOf = (r) => r.prompts || [];
export const promptBlockedOf = (r) => r.promptsReady === false;

// ---- 描くための小さな問い ----------------------------------------------
export const dtabOf = (r) => state.dtab[r.dom] || 'shape';
export const teamLabel = (r) => r.name || '新しいチーム定義';
export const currentProjectName = () => {
  const p = (HQ && HQ.projects) || {};
  return (p.current && p.current.name) || '';
};
export const groupLabelOf = (g) => (g.global ? 'グローバル' : hqProjectName(groupKeyOf(g)));
export const liveRunning = (key) => ((hqState.running && hqState.running.teams) || []).filter((t) => t.project === key).length;

// ---- 住所 --------------------------------------------------------------
export function syncURL() {
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
      ruleState.ropen.forEach((kk) => {
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
export function readURL() {
  const p = new URLSearchParams(location.search);
  const open = p.get('open');
  if (open) {
    const rows = parseOpen(open) || [];
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
      ruleState.ropen.add(`${r.uid}${r.place}:${hit._k}`);
      state.wantRule = { name: r.dom, k: hit._k };
    }
  });
  const file = p.get('file');
  if (file) state.yfile = file;
  if (location.pathname === '/teams/yaml') state.tab = 'yaml';
}

// ---- 共有の部品（ruledoc）の受け口 ------------------------------------
export const rowOfPanel = (r) => {
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
};
export const host = {
  // A team's rules document has no name of its own to edit: the definition's
  // name is the row's, so the part leaves the 名前 field out (監査 R9).
  hasName: false,
  row: (panel) => {
    if (!panel || !panel.classList.contains('agree-p')) return null;
    const r = byKey(panel.dataset.key);
    return r ? rowOfPanel(r) : null;
  },
  // rerender draws the screen again and answers the fresh element: the part
  // reads it right after (to put the caret in the rule it opened).
  rerender: (panel, sel) => {
    render();
    const fresh = panel && document.contains(panel) ? panel
      : (panel && view && view.querySelector(`.panel.agree-p[data-key="${CSS.escape(panel.dataset.key)}"]`)) || panel;
    if (sel && fresh) { const el = fresh.querySelector(sel); if (el) { el.focus(); el.select?.(); } }
    return fresh;
  },
  edited: (c) => {
    const panel = c && c.panel;
    const r = panel && byKey(panel.dataset.key);
    if (!r) return;
    render();
    scheduleAgree(r);
  },
  readerCount: () => 0,
  filtering: () => false,
  shown: () => true,
  query: () => '',
  path: (l, d) => (d && d.__path) || '',
  mode: (panel, v) => { const r = panel && byKey(panel.dataset.key); if (r) state.pmode[dkey(r)] = v; },
  url: () => syncURL(),
};

// ---- 帯と、ページの外への約束 ------------------------------------------
// setBandCount keeps the band's number the same as the list's own: the band
// counts the definitions, which are the rows that are saved — a row that is
// not there yet is not one (spec-nav 1.1, 監査 R7).
export function setBandCount() {
  let n = 0;
  for (const r of recs.values()) if (!r.draftRow) n++;
  setCount('teams', n);
}
// dirty is whether the screen holds anything the files do not: a row's ops, a
// rules draft, a YAML text. The band asks it through window.hergyPage (nav.js
// 1.5), the same way it asks the other screens.
export function dirty() {
  for (const r of recs.values()) if (dirtyDef(r)) return true;
  for (const f of FILES) if (fileDirty(f.file)) return true;
  return false;
}
// tellDirty says it again after a change, so the band's mark follows the field
// a person is in (nav.js 1.5).
export function tellDirty() {
  document.dispatchEvent(new CustomEvent('hergy:dirty', { detail: dirty() }));
}
// restore puts back what the band carried from this screen (spec-nav 1.5): the
// ops of a row that was not saved, the rules draft, and the YAML text. A file
// that moved outside since is not put back (its hash is not the one the draft
// was read from).
export function restore(carried) {
  (carried.teams || []).forEach((c) => {
    const r = [...recs.values()].find((x) => x.name === c.name && x.place === c.place && !x.draftRow);
    if (!r || (c.hash && r.hash !== c.hash)) return;
    r.ops = clone(c.ops);
    r.team.draft = applyOps(r.team.saved, r.ops);
    checkTeam(r);
  });
  (carried.agrees || []).forEach((c) => {
    const r = [...recs.values()].find((x) => x.name === c.name && x.place === c.place && !x.draftRow);
    if (!r || !r.agree.saved || (c.hash && r.agreeHash !== c.hash)) return;
    const d = fromJSON(c.doc);
    d.__path = r.agreePath;
    r.agree.draft = d;
    checkAgree(r);
  });
  (carried.files || []).forEach((c) => {
    const f = fileByKey.get(c.file);
    if (!f || (c.hash && f.hash !== c.hash)) return;
    state.ytext[c.file] = c.text;
  });
  render();
}

export function take() {
  const teams = [];
  for (const r of recs.values()) if (!r.draftRow && changeCount(r) > 0) teams.push({ place: r.place, name: r.name, hash: r.hash, ops: r.ops });
  const agrees = [];
  for (const r of recs.values()) if (!r.draftRow && r.agree.saved && agreeDiff(r)) agrees.push({ place: r.place, name: r.name, hash: r.agreeHash, doc: toDoc(r.agree.draft, r.agree.saved) });
  const files = FILES.filter((f) => fileDirty(f.file)).map((f) => ({ file: f.file, hash: f.hash, text: state.ytext[f.file] }));
  return { teams, agrees, files };
}

// ---- 立ち上げ ----------------------------------------------------------
export function boot() {
  if (GROUPED) GROUPS.forEach((g) => rowsIn(g).forEach((item) => defOf(item, groupKeyOf(g))));
  else (LIST.items || []).forEach((item) => defOf(item));
  readURL();
  pickFile();
  // What the band carried here from this screen is put back once, before the
  // first draw (spec-nav 1.5). A draft it did not carry is spent there.
  const carried = takeDraft('teams');
  if (carried) restore(carried);
  render();
  setStick();
  const later = [];
  // The member's 属性 offers the roles the role screen holds; the team answer
  // carries no role documents, so they are read once from GET /api/role. A panel
  // already drawn gets them when the answer comes (the first frame is quick).
  later.push(apiList('role').then((list) => {
    ROLES = (list.items || []).map((it) => {
      const places = it.places || {}, eff = it.effective || (places.project ? 'project' : 'global');
      const doc = (places[eff] || {}).doc || {};
      return { name: it.name || '', title: doc.title || '' };
    }).filter((x) => x.name);
    if (!HQ) rolesIn = true;
    render();
  }).catch(() => {}).finally(() => { if (!HQ) rolesCame(); }));
  // The roles and profiles a member's 属性 offers are read after the list: the
  // page itself carries only the definitions, so a row opens without waiting
  // for every project's roles and profiles (r-5b96, Enter lag).
  if (HQ) {
    const fill = (kind, map, toEntry) => apiCall('GET', '/api/hq/' + kind).then((res) => {
      if (!res.ok || !res.json) return;
      (res.json.groups || []).forEach((g) => {
        map[groupKeyOf(g)] = rowsIn(g).map(toEntry).filter(Boolean);
      });
      if (map === ROLES_BY) rolesIn = true;
      render();
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
  Promise.allSettled([...later, ...checks]).then(() => { if (window.hergyPage) window.hergyPage.settled = true; });
}
// jumpToRule opens one rule of a team's own agreement and puts the cursor in
// its body (spec-teams 1.4.5, spec-roles 1.2.4, 監査 R12). The check's column
// reads through this, and so does the address.
export function jumpToRule(r, k) {
  state.focusAsk = null;
  const p = $(`.panel.agree-p[data-key="${CSS.escape(dkey(r))}"]`);
  if (p) goRule(p, k);
}
export function jumpWantRule() {
  const w = state.wantRule;
  state.wantRule = null;
  if (!w) return;
  const r = byName(w.name);
  if (r) jumpToRule(r, w.k);
}
export const setStick = () => {
  const bar = $('.bar');
  document.documentElement.style.setProperty('--stick', (bar ? bar.offsetHeight : 48) + 'px');
};
// The view needs these to read one row's parts.
export { hasTurn, memberOf, turnLabel, other, firstIndexOf, orderRows, byRef, recs };
