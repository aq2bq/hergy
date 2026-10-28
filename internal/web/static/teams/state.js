// teams/state.js — チーム定義の画面の状態と、サーバーとのやり取り
// （webui-preact-design 4 章・5 章、webui-components 1・2 章）。
//
// 定義の記録（recs）と画面の状態は signals に置く。1 つの記録は欄ごとの signal
// の束で、姿（draft）は保存した定義と ops から作る computed にする。だから
// 記録を書き換えたら描き直す、という関数は無い: 部品は signal を読み、値が変われ
// ば Preact が自分で描き直す（依頼書 1）。押し込みは部品の onClick / onInput /
// onKeyDown から、ここの関数を直接呼ぶ（依頼書 2。ひとまず (a) では main.js の
// 委譲がここを呼ぶ）。
//
// 検査の文はすべてサーバ（check.Team / check.Agreement）が作る。この画面は文を
// 作らない（spec-boundary 3）。
import { signal, computed } from '../vendor/preact.js';
import { resource } from '../lib/resource.js';
import { saved as apiSaved, list as apiList, call as apiCall, problemText as apiProblemText } from '../lib/api.js';
import {
  fromJSON, toDoc, rulesOf, diffCount, clone, state as ruleState, observeKeys, openRule,
} from '../lib/ruledoc.js';
import { parseOpen } from '../lib/hq.js';
import { takeDraft, setDirty } from '../lib/nav.js';
import { place } from './place.js';

export const view = document.getElementById('view');
export const tabsEl = document.getElementById('tabs');

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
// the global layer's (spec 4 章). They are filled after the first frame, and
// ROLES_REV is the number that says they changed, so the columns that read
// them are drawn again without the screen asking.
const ROLES_BY = {};
const PROFILES_BY = {};
const ROLES_REV = signal(0);
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
const ROLES = signal([]);
// rolesIn turns true once the roles a new member is named after have answered:
// until then ＋メンバー and the blank space's メンバーを足す cannot be pressed,
// since a member added before them has no role (d-c5fe). A read that fails lets
// them be pressed as before.
const ROLES_IN = signal(false);
export const rolesIn = () => ROLES_IN.value;
export const rolesCame = () => { ROLES_IN.value = true; };
// The island is the same list answer GET /api/team gives (spec-boundary 5):
// the items, and the files the YAML tab can open.
const LIST = DATA.team || { items: [], files: [] };

export const $ = (s, root = document) => root.querySelector(s);
export const nodeName = (x) => (x === 'human' ? '人間' : x);
export const other = (p) => (p === 'project' ? 'global' : 'project');
const hasTurn = (t, a, b) => (t.turns || []).some(([p, q]) => p === a && q === b);
const memberOf = (t, n) => (t.members || []).find((m) => m.name === n);
export const narrow = () => window.innerWidth <= 760;
// thin is narrow as the figure reads it: a page that grows or shrinks draws the
// figure again through this (main.js puts the width in).
export const thin = signal(narrow());

// ---- 画面の状態 ---------------------------------------------------------
// state is what the screen knows about itself: the tab, the open rows, the
// files' own drafts and the notices. Every change puts a new value in (依頼書 1),
// so nothing here is written in place.
const put = (patch) => { state.value = { ...state.value, ...patch }; };
export const state = signal({
  tab: (view && view.dataset.tab) === 'yaml' ? 'yaml' : 'list',
  open: new Set(), dtab: {}, confirm: {},
  ok: {}, yfile: null, ytext: {}, yc: {}, pmode: {}, wantRule: null,
  // refusal is the server's sentence on one row's foot (the shape tab's or the
  // rules tab's), and toast the words the canvas says at the bottom.
  refusal: {}, toast: null,
});
// focusAsk is the one field a person's press should land in after the draw
// (5 章 5): a member just added, a rule just opened. The row it names puts the
// cursor there itself.
export const focusAsk = signal(null);
// withKey puts one entry of a map the screen keeps (a text, a check, a notice).
const withKey = (name, key, value) => {
  const m = { ...state.value[name] };
  if (value === undefined) delete m[key];
  else m[key] = value;
  put({ [name]: m });
};
const withOpen = (dom, on) => {
  const s = new Set(state.value.open);
  if (on) s.add(dom); else s.delete(dom);
  put({ open: s });
};
export const isOpen = (dom) => state.value.open.has(dom);
export const dtabOf = (r) => state.value.dtab[r.dom] || 'shape';
export const pmodeOf = (r) => state.value.pmode[dkey(r)] || 'edit';
export const okOf = (key) => !!state.value.ok[key];
export const refusalOf = (r, which) => state.value.refusal[which + dkey(r)] || '';
const setDtab = (dom, t) => { put({ dtab: { ...state.value.dtab, [dom]: t } }); };
const dropDtab = (dom) => { const m = { ...state.value.dtab }; delete m[dom]; put({ dtab: m }); };
const setConfirm = (key, on) => { const m = { ...state.value.confirm }; if (on) m[key] = 1; else delete m[key]; put({ confirm: m }); };
const debounceTimers = {};
export const debounce = (k, fn, ms = 260) => { clearTimeout(debounceTimers[k]); debounceTimers[k] = setTimeout(fn, ms); };

// dkey is the key the screen's own state and DOM stand on: the definition's
// saved name, or, for a definition that is not there yet (its name is what the
// person is typing), a stable id of its own (spec-teams B2).
export const dkey = (d) => d.dom + '.' + d.place.value;
export const domOf = (name, group) => (GROUPED ? name + '|' + (group || '') : name);

// ---- サーバとのやり取りの形 ---------------------------------------------
// A definition that is not there yet carries how it was made: the starter's own
// files (new), or the definition it stands on (base). The server reads the base
// from there and checks the row as if it were saved (spec-teams B2, B3).
const draftIn = (r) => (!r.draftRow ? {} : r.how === 'new' ? { new: true } : { base: r.base });
// projectIn is the project a write names: a listed project's path, and nothing
// for the global layer (which the server writes without being told) or for this
// serve's own project.
const projectIn = (project) => (project && project !== 'global' ? { project } : {});
const teamIn = (r, extra) => Object.assign({ place: r.place.value, name: r.name.value, part: 'team' }, projectIn(r.project), draftIn(r), extra || {});
const agreeIn = (r, extra) => Object.assign({ place: r.place.value, name: r.name.value, part: 'agreement' }, projectIn(r.project), draftIn(r), extra || {});

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
// Still are the fields a record is made with and never changes: they are read
// as they are. Every other field is a signal of its own, so a change draws the
// parts that read that field and nothing else (依頼書 1). ops and saved make
// draft, a computed: no one keeps the two in step by hand any more.
const STILL = ['uid', 'dom', 'project', 'places', 'draftRow', 'how', 'base'];
function recOf(init) {
  const r = {};
  Object.keys(init).forEach((k) => { r[k] = STILL.includes(k) ? init[k] : signal(init[k]); });
  r.draft = computed(() => applyOps(r.saved.value, r.ops.value));
  r.key = computed(() => dkey(r));
  // wprompts is the worker prompts the server answered, per worker; the request
  // that is out is the drawing's own bookkeeping and is not a signal.
  r.wprompts = signal({});
  r.wpromptReq = {};
  // placed is where a person put a point down (a double press's menu, or a drag
  // that was let go), and layout is where the figure draws every point: the
  // places above and an automatic place for the rest. The drawing reads layout
  // and writes nothing (webui-components 1 章).
  r.placed = signal({});
  r.layout = computed(() => place(r.draft.value, r.placed.value, thin.value));
  return r;
}
export const recs = signal([]);
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
  const had = byKey(dom + '.' + eff);
  if (had) return had;
  let saved = null;
  if (agree.doc) {
    saved = fromJSON(agree.doc);
    saved.__path = agree.path;
  }
  const hidden = (item.hidden || []).filter((p) => places[p]).map((p) => ({
    place: p, path: (places[p] || {}).path, hash: (places[p] || {}).hash,
  }));
  const r = recOf({
    name: item.name || '', dom, project, place: eff, places: PLACES.filter((p) => places[p]),
    teamPath: place.path || '', agreePath: agree.path || '',
    hash: place.hash || '', agreeHash: agree.hash || '', broken: !!doc.broken,
    hidden, ops: [],
    saved: cloneTeam(doc),
    agreeSaved: saved,
    agreeDraft: saved ? clone(saved) : fromJSON({ name: '', title: (item.name || '') + ' チームのルール', revision: 1, sections: [] }),
    // The saved document's own problems are the agreement save's baseline:
    // a save stops only when an error the saved text did not hold has
    // appeared (spec-teams X3).
    agreeSavedIssues: null,
    findings: [],
    // The list's own size and budget, the same pair the figure's check
    // answers (d-00a9, r-2f83). A file that does not read has no size and
    // draws 大きさ —.
    size: sum.size || 0, limit: sum.limit || 0,
    problems: item.problems || {}, check: null, drawMark: 0,
    draftRow: false, how: '', base: null, seeded: true, writes: [], nameIssues: [],
    preview: { lines: [], diff: {} }, baseFindings: [], prompts: [], promptsReady: undefined,
    // The canvas's own picks for this row (webui-components 3 章): what is
    // picked, which column stands on the right, the blank space's menu, the
    // member name being typed, and the line that just lit up.
    sel: null, side: 'attr', menu: null, typed: null, flash: null,
    lastAct: 0,
  });
  r.uid = ++uids;
  recs.value = [...recs.value, r];
  return r;
}
export const allRecs = () => recs.value;
export const byKey = (k) => recs.value.find((r) => r.key.value === k) || null;
export const byName = (n) => recs.value.find((r) => r.dom === n) || null;
// byRef is the record one address names: the row of one group, or, with no
// group, the row the screen wrote before the headquarters (its name alone).
const byRef = (name, group) => (group ? byName(domOf(name, group)) : (byName(name) || byRealName(name)));
// byRealName is the first row of a name, the project's own first: a link the
// older screens wrote names a definition, not a group.
const byRealName = (name) => recs.value.find((r) => r.name.value === name && (r.project === CURRENT || r.project === ''))
  || recs.value.find((r) => r.name.value === name) || null;
// groupOfRecord is the group a writable row belongs to: its own project, the
// global layer, or, for a row that is not there yet, this serve's project.
// Without projects: the screen writes to the one project, as it always did.
export const groupOfRecord = (r) => (GROUPED ? (r.project || (r.place.value === 'global' ? 'global' : CURRENT)) : '');
// The list is drawn in the order the records stand in, so a row that is added
// or read again is put back where a person was looking: firstIndexOf is the
// place of the first record one rule names, and orderRows puts one row there
// (spec-teams B1-B6).
export const firstIndexOf = (want) => recs.value.findIndex((x) => want(x));
export function orderRows(r, at) {
  const all = recs.value.filter((x) => x !== r);
  all.splice(Math.max(0, Math.min(at < 0 ? 0 : at, all.length)), 0, r);
  recs.value = all;
}
export const rowsOfGroup = (key) => recs.value.filter((r) => (r.project ? r.project === key : key === (r.place.value === 'global' ? 'global' : CURRENT)));

// ---- まだ無い定義（B2, B3） --------------------------------------------
// A definition that is not there yet is one row the screen holds rather than a
// saved file: the name the person is typing, the place it would go to, the
// definition it stands on, and what the server answered about it. The screen
// builds no document of its own; it draws the check's own answer (spec-teams
// B2, spec-boundary 5).
function draftRecord(how, place, name, base, project) {
  const r = recOf({
    dom: '~new' + (++uids), draftRow: true, how, base: base || null, project: project || '',
    name: name || '', place: place || 'project', places: [],
    teamPath: '', agreePath: '', hash: '', agreeHash: '', broken: false,
    hidden: [], ops: [],
    saved: cloneTeam(null),
    agreeSaved: null,
    agreeDraft: fromJSON({ name: '', title: 'チームのルール', revision: 1, sections: [] }),
    agreeSavedIssues: null,
    findings: [], problems: {}, check: null,
    size: 0, limit: 0,
    drawMark: 0, seeded: false, writes: [], nameIssues: [],
    preview: { lines: [], diff: {} }, baseFindings: [], prompts: [], promptsReady: undefined,
    sel: null, side: 'attr', menu: null, typed: null, flash: null,
    lastAct: 0,
  });
  recs.value = [...recs.value, r];
  return r;
}
// seedDraft reads the first check of a definition that is not there yet into
// the row: the base text its figure and its rules stand on, and where a save
// would write it (spec-teams B2, B3).
function seedDraft(r, json) {
  const doc = json.doc || {}, agree = doc.agreement || {};
  r.seeded.value = true;
  r.saved.value = cloneTeam(doc);
  r.broken.value = !!doc.broken;
  r.teamPath.value = json.path || '';
  r.agreePath.value = agree.path || '';
  r.hash.value = json.hash || '';
  r.agreeHash.value = agree.hash || '';
  if (agree.doc) {
    const d = fromJSON(agree.doc);
    observeKeys(rulesOf(d));
    d.__path = agree.path;
    r.agreeSaved.value = d;
    r.agreeDraft.value = clone(d);
  }
}
// theName is the name a duplicate takes: the source's with -2, -3 … until one
// is free, so the row starts without the name problem the same name would raise
// (spec-teams B3).
export function theName(base) {
  const taken = new Set(recs.value.map((r) => r.name.value));
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
  if (base) { const i = firstIndexOf((x) => x.name.value === base.name); at = i >= 0 ? i + 1 : 0; }
  orderRows(r, at);
  withOpen(r.dom, true);
  setDtab(r.dom, 'shape');
  // The name a copy already holds is chosen, so the first letter a person
  // types replaces it rather than landing beside it (監査 R8).
  focusAsk.value = { dom: r.dom, sel: '.in-name' };
  return r;
}
// dropDraft takes a row that is not there yet away without writing anything
// (spec-teams B2).
export function dropDraft(r) {
  recs.value = recs.value.filter((x) => x !== r);
  withOpen(r.dom, false);
  dropDtab(r.dom);
  setConfirm('del' + dkey(r), false);
  resetCanvas(r);
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
  const t = r.draft.value;
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
export const agreeIssues = (r) => (r.check.value ? r.check.value.issues : []);
export const agreeDiff = (r) => diffCount({ saved: r.agreeSaved.value, draft: r.agreeDraft.value });
export const teamIssues = (r) => r.findings.value;
const pcountList = (c) => [...Array((c && c.err) || 0)].map(() => ({ sev: 'err' })).concat([...Array((c && c.warn) || 0)].map(() => ({ sev: 'warn' })));
// rowIssues is the problems a row's own chip shows: the server's count for a
// saved definition, and the findings the server answered for a definition that
// is not there yet (spec-teams B2).
export const rowIssues = (r) => (r.draftRow ? teamIssues(r) : pcountList(r.problems.value));
// A definition that is not there yet counts as unsaved whatever it holds: it is
// written only when 保存 is pressed (spec-teams B2).
export const dirtyDef = (r) => r.draftRow || changeCount(r) > 0 || (r.agreeSaved.value ? agreeDiff(r) > 0 : false);

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
  const t = r.draft.value, s = r.saved.value, ops = r.ops.value || [];
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
  const name = r.name.value;
  return !!r.agreePath.value && path.endsWith('/teams/' + name + '/agreement.yaml');
};
const ruleID = (m) => ((m.field || '').split('#')[1] || '');
export const vlistParts = (r) => {
  const issues = teamIssues(r);
  const at = (m) => m.field && m.field.startsWith('rule:');
  const out = issues.filter((m) => at(m) && !ownAgreement(r, m));
  const teamOut = issues.filter((m) => at(m) && ownAgreement(r, m));
  const own = issues.filter((m) => !at(m));
  const where = (m) => (m.node ? nodeName(m.node) : m.worker != null && m.worker >= 0 ? ((r.draft.value.workers[m.worker] || {}).purpose || 'ワーカー') : m.field === 'mission' ? '使命' : m.field === 'name' ? 'チーム定義名' : '');
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
// A check is a reading like any other (5 章 3), so each goes through
// lib/resource.js: the newest request is the one whose answer is kept and the
// one before it is dropped. A row keeps two of them, because one reading is not
// like the others: the check sent with no ops reads the saved file, and its
// findings are the baseline the save gate is held to (spec-teams X3). That
// reading is a request of its own, so a person who starts typing before it comes
// back still gets the file's own problems as the baseline.
const teamChecks = new Map();
function checkersFor(r) {
  const k = dkey(r);
  let one = teamChecks.get(k);
  if (one) return one;
  const load = async (signal, payload) => {
    const res = await apiCall('POST', '/api/team/check', payload, { signal });
    // A refused draft answers nothing here: the screen draws no findings of its
    // own, and the refusal itself is the rules tab's to show (spec-teams X1).
    return res.ok ? (res.json || {}) : null;
  };
  one = { draft: resource(load), base: resource(load), agree: resource(load) };
  teamChecks.set(k, one);
  return one;
}
export function forgetChecks(r) { teamChecks.delete(dkey(r)); }

// applyTeamAnswer draws one check's answer into the row.
function applyTeamAnswer(r, json, findings) {
  r.findings.value = findings;
  // Each member's prompt, cut into the pieces it was built from (r-4cf9),
  // follows the ops the check was sent with: the 属性 column shows it without
  // building a line of its own. A definition with a problem is not given any,
  // and the column says why (d-20ac).
  r.prompts.value = Array.isArray(json.members) ? json.members : [];
  r.promptsReady.value = !!(json.members && json.members.length);
  // Both a saved definition and one that is not there yet get the topology
  // tab's own preview from the check (spec-teams B7).
  const pv = json.preview || r.preview.value;
  r.preview.value = pv;
  if (r.draftRow) {
    r.writes.value = json.writes || r.writes.value;
    r.nameIssues.value = (json.problems || []).filter((p) => p.at === 'name' || p.field === 'name');
    // The paths the row would be written to are the server's, and they move
    // with the name a person is typing: both panels' heads follow the check
    // (spec-teams B2, 監査 R5).
    if (json.path) r.teamPath.value = json.path;
    const ag = (json.doc || {}).agreement || {};
    if (ag.path) {
      r.agreePath.value = ag.path;
      const saved = r.agreeSaved.value;
      const draft = r.agreeDraft.value;
      if (saved) saved.__path = ag.path;
      if (draft) draft.__path = ag.path;
    }
  }
  r.drawMark.value = (r.drawMark.value || 0) + 1;
}

export async function checkTeam(r) {
  const one = checkersFor(r);
  const payload = teamIn(r, { doc: { ops: r.ops.value } });
  if (!r.ops.value.length) {
    // The op-less reading: the base a definition that is not there yet stands
    // on (spec-teams B2), and the file's own problems as the save's baseline.
    const at = r.drawMark.value || 0;
    const json = await one.base.run(payload);
    if (!json) return;
    if (r.draftRow && !r.seeded.value) seedDraft(r, json);
    const findings = figFindings(r, json.problems || []);
    // The baseline is kept even when it is too old to draw (the guard below),
    // but it does not draw over a newer answer (spec-teams X3). The foot is
    // drawn again all the same: its save is held to the baseline, and an
    // answer that came last would otherwise leave the save shut on an error
    // the file already held. The foot reads baseFindings, so putting it in
    // draws it.
    r.baseFindings.value = findings;
    if ((r.drawMark.value || 0) !== at) return;
    applyTeamAnswer(r, json, findings);
    return;
  }
  const json = await one.draft.run(payload);
  if (!json) return;
  applyTeamAnswer(r, json, figFindings(r, json.problems || []));
}
// checkAgree asks what is wrong with the rules draft: the newest answer is the
// one the panel shows, and a draft that has not moved is the baseline its save
// is held to (spec-teams X3).
export async function checkAgree(r) {
  const one = checkersFor(r);
  const doc = toDoc(r.agreeDraft.value, r.agreeSaved.value);
  const json = await one.agree.run(agreeIn(r, { doc: { ops: r.ops.value, doc } }));
  if (!json) return;
  r.check.value = buildCheck(r.agreeDraft.value, json);
  if (!agreeDiff(r)) r.agreeSavedIssues.value = r.check.value.issues;
}
export const scheduleAgree = (r) => debounce('a' + dkey(r), () => syncAgree(r));

export function syncTeam(r) {
  checkTeam(r);
}
export function syncAgree(r) {
  checkAgree(r);
}
export function pushOp(r, op) { r.ops.value = [...r.ops.value, op]; }
// setOps puts a whole list of operations in: a revert drops them all at once.
export function setOps(r, ops) { r.ops.value = ops; }

// saveTeam writes the team.yaml's ops (spec-teams B6). The refusal is the
// screen's own state: the panel's foot draws it (5 章 2).
export async function saveTeam(r) {
  if (r.draftRow) return saveFolder(r);
  const res = await apiCall('PUT', `/api/team/${encodeURIComponent(r.name.value)}`, teamIn(r, { hash: r.hash.value, doc: { ops: r.ops.value } }));
  if (!res.ok) return refuse(r, 't', apiProblemText(res, '保存できませんでした'));
  const json = (res && res.json) || {};
  r.hash.value = json.hash || r.hash.value;
  r.saved.value = cloneTeam(r.draft.value);
  r.ops.value = [];
  r.baseFindings.value = r.findings.value;
  setProblems(json.items, r.project);
  withKey('ok', 't' + dkey(r), 1);
  resetCanvas(r);
  cundo.delete(dkey(r));
  apiSaved();
}
export async function saveAgree(r) {
  if (r.draftRow) return saveFolder(r);
  const doc = toDoc(r.agreeDraft.value, r.agreeSaved.value);
  const res = await apiCall('PUT', `/api/team/${encodeURIComponent(r.name.value)}`, agreeIn(r, { hash: r.agreeHash.value, doc: { ops: r.ops.value, doc } }));
  if (!res.ok) return refuse(r, 'a', apiProblemText(res, '保存できませんでした'));
  const json = (res && res.json) || {};
  r.agreeHash.value = json.hash || r.agreeHash.value;
  if (json.doc) {
    const d2 = fromJSON(json.doc);
    observeKeys(rulesOf(d2));
    d2.__path = r.agreePath.value;
    r.agreeSaved.value = d2;
    r.agreeDraft.value = clone(d2);
  }
  setProblems(json.items, r.project);
  withKey('ok', 'a' + dkey(r), 1);
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
  const doc = toDoc(r.agreeDraft.value, r.agreeSaved.value);
  const body = Object.assign(teamIn(r), {
    hash: r.hash.value, agreement_hash: r.agreeHash.value, part: 'folder', doc: { ops: r.ops.value, doc },
  });
  const res = await apiCall('PUT', `/api/team/${encodeURIComponent(r.name.value)}`, body);
  if (!res.ok) return refuse(r, 't', apiProblemText(res, '保存できませんでした'));
  const json = (res && res.json) || {};
  const name = r.name.value, dom = r.dom, dtab = dtabOf(r), opens = isOpen(dom);
  // The row stands where the row a person was looking at stood: the first
  // reading of this name keeps its place, and a copy's save joins into it
  // (spec-teams B1-B4).
  const at = firstIndexOf((x) => x.dom === dom || x.name.value === name);
  // Every reading of this name is taken again from the answer: a copy's save
  // joins the two places into one row (spec-teams B4).
  const gone = recs.value.filter((x) => x.dom === dom);
  recs.value = recs.value.filter((x) => x.dom !== dom);
  gone.forEach((x) => { setConfirm('del' + dkey(x), false); resetCanvas(x); });
  setProblems(json.items, groupOfRecord(r));
  const item = (json.items || []).find((it) => it.name === name);
  if (!item) { withOpen(dom, false); dropDtab(dom); return; }
  const nr = defOf(item, groupOfRecord(r));
  orderRows(nr, at);
  if (opens) withOpen(nr.dom, true);
  setDtab(nr.dom, dtab);
  // The definition the save wrote holds its own checks now: the figure's and
  // the rules' are read again from the file it wrote (spec-boundary 5). The
  // 保存しました stands on the row's own foot after that reading, so the
  // check's own draw does not take it away (1.4.6, 監査 R7).
  await checkTeam(nr);
  if (nr.agreeSaved.value) await checkAgree(nr);
  withKey('ok', (dtabOf(nr) === 'agree' ? 'a' : 't') + dkey(nr), 1);
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
  const name = r.name.value;
  // The row keeps its place in the list: taking one place's folder away leaves
  // the definition where a person was looking at it (spec-teams B6, N7).
  const at = firstIndexOf((x) => x.dom === r.dom);
  const gone = recs.value.filter((x) => x.dom === r.dom);
  recs.value = recs.value.filter((x) => x.dom !== r.dom);
  gone.forEach((x) => { setConfirm('del' + dkey(x), false); resetCanvas(x); });
  setProblems(json.items, groupOfRecord(r));
  const item = (json.items || []).find((it) => it.name === name);
  if (item) {
    const nr = defOf(item, groupOfRecord(r));
    orderRows(nr, at);
    if (!state.value.dtab[nr.dom]) setDtab(nr.dom, 'shape');
  }
  syncURL();
}
const delURL = (r) => {
  const qs = [`place=${encodeURIComponent(r.place.value)}`, `hash=${encodeURIComponent(r.hash.value)}`];
  if (r.agreeHash.value) qs.push(`agreement_hash=${encodeURIComponent(r.agreeHash.value)}`);
  if (r.project && r.project !== 'global') qs.push(`project=${encodeURIComponent(r.project)}`);
  return `/api/team/${encodeURIComponent(r.name.value)}?${qs.join('&')}`;
};
// refuse holds the server's sentence on the row's foot (the shape tab's, or the
// rules tab's) and draws it; a screen builds no sentence of its own.
export function refuse(r, which, text) {
  withKey('refusal', which + dkey(r), text || '保存できませんでした');
}
// clearOk takes the notice away when the words it stood for change: a save's
// 保存しました stays until the next change writes over it (1.4.6).
export function clearOk(r) {
  if (state.value.ok['t' + dkey(r)] || state.value.ok['a' + dkey(r)]) {
    const m = { ...state.value.ok };
    delete m['t' + dkey(r)];
    delete m['a' + dkey(r)];
    put({ ok: m });
  }
}
// setProblems takes the counts a save answered with back into the rows, so the
// row's 問題/警告 chip is the server's own count again (spec-boundary 5).
function setProblems(items, project) {
  (items || []).forEach((item) => {
    const places = item.places || {};
    const eff = item.effective || (places.project ? 'project' : 'global');
    const r = byKey(domOf(item.name || '', project || '') + '.' + eff);
    if (r) r.problems.value = item.problems || {};
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
  st.push({ ops: cloneOps(r.ops.value), label });
  while (st.length > 80) st.shift();
  cundo.set(k, st);
}
export function canvasToast(r, msg, withUndo) {
  toastRow = r;
  put({ toast: { msg, undo: !!withUndo } });
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { put({ toast: null }); toastRow = null; }, 4000);
}
export function canvasUndoFromToast() { if (toastRow) canvasUndo(toastRow); }
// forgetUndo drops the operations a row could take back (a save, a revert).
export function forgetUndo(r) { cundo.delete(dkey(r)); }
export function canvasAct(r, ops, o) {
  o = o || {};
  r.lastAct.value = ++actSeq;
  r.promptsReady.value = undefined;
  clearOk(r);
  canvasUndoPoint(r, o.undo || '');
  r.ops.value = [...r.ops.value, ...(ops || [])];
  ctyping = { key: null, field: null };
  if (o.focus) focusAsk.value = { dom: r.dom, sel: o.focus };
  checkTeam(r);
  if (o.message) canvasToast(r, o.message, true);
}
export function canvasTyping(r, op, field) {
  const k = dkey(r);
  r.lastAct.value = ++actSeq;
  r.promptsReady.value = undefined;
  clearOk(r);
  if (ctyping.key !== k || ctyping.field !== field) {
    const label = { mission: '使命を戻した', 'm-name': 'メンバー名を戻した', 'w-purpose': '目的を戻した' }[field] || '元に戻した';
    canvasUndoPoint(r, label);
    ctyping = { key: k, field };
  }
  pushOp(r, op);
  debounce('ct' + k, () => checkTeam(r));
}
export function canvasEndTyping(r) {
  if (!r) return;
  if (ctyping.key === dkey(r)) { ctyping = { key: null, field: null }; checkTeam(r); }
}
export function canvasUndo(r) {
  const k = dkey(r), st = cundo.get(k);
  r.promptsReady.value = undefined;
  clearOk(r);
  if (!st || !st.length) return;
  const point = st.pop();
  setOps(r, cloneOps(point.ops));
  ctyping = { key: null, field: null };
  checkTeam(r);
  canvasToast(r, point.label || '元に戻した', false);
}
// The row the newest operation stands on: ⌘Z acts on it even when nothing is
// picked (監査 1-1).
export function undoRow() {
  return recs.value.filter((x) => isOpen(x.dom) && (cundo.get(dkey(x)) || []).length)
    .sort((a, b) => (b.lastAct.value || 0) - (a.lastAct.value || 0))[0] || null;
}
// resetCanvas forgets the canvas's picks and places for one row (a save, a
// delete: the figure lays out afresh, d-a575).
export function resetCanvas(r) {
  if (!r) return;
  r.sel.value = null;
  r.side.value = 'attr';
  r.menu.value = null;
  r.typed.value = null;
  r.flash.value = null;
  r.placed.value = {};
}

// The worker's prompt is the one hire --dry builds (r-4cf9). Its name and task
// are chosen at hire, so the screen asks with values that cannot be mistaken
// for the purpose or another line, and shows <ワーカー名> / <依頼> in their
// place. The values are the walk's own too, so it can put them back.
const WRK_NAME = 'zz-worker-name-0', WRK_TASK = 'zzTaskPlaceholder0';
export function workerPrompt(r, i) {
  const w = r.draft.value.workers[i];
  if (!w || !w.purpose || !w.from) return null;
  const k = w.from + '\u0000' + w.purpose;
  const got = r.wprompts.value[k];
  if (got) return got;
  // The request that is out is the drawing's own bookkeeping, not something a
  // part reads: it stands beside the drawing so asking again while drawing does
  // not write a signal (5 章 1).
  if (r.wpromptReq[k]) return null;
  r.wpromptReq[k] = true;
  apiCall('POST', '/api/team/check', { place: r.place.value, name: r.name.value, part: 'worker', project: r.project && r.project !== 'global' ? r.project : undefined, doc: { name: WRK_NAME, purpose: w.purpose, task: WRK_TASK, from: w.from } }).then((res) => {
    delete r.wpromptReq[k];
    const ans = res.ok ? (res.json || {}) : null;
    r.wprompts.value = {
      ...r.wprompts.value,
      [k]: res.ok
        ? { segments: (ans && ans.prompt) || [], name: WRK_NAME, task: WRK_TASK }
        : { segments: [], name: WRK_NAME, task: WRK_TASK, err: apiProblemText(res, 'プロンプトを読めませんでした') },
    };
  }).catch(() => { delete r.wpromptReq[k]; });
  return null;
}
// rolesFor and profilesFor are the documents one row's member 属性 offers: the
// row's own project's and the global layer's, so another project's definition
// chooses from its own names (spec 4 章). Without a headquarters they are the
// one list the island carries. ROLES_REV is read so a list that comes after the
// first frame draws the columns again.
export function rolesFor(r) {
  ROLES_REV.value;
  if (!HQ) return ROLES.value;
  const key = r ? groupOfRecord(r) : CURRENT;
  const all = key === 'global'
    ? Object.values(ROLES_BY).flat()
    : (ROLES_BY[key] || []).concat(ROLES_BY.global || []);
  const seen = new Set();
  return all.filter((x) => !seen.has(x.name) && seen.add(x.name));
}
export function profilesFor(r) {
  ROLES_REV.value;
  if (!HQ) return PROFILES;
  const key = r ? groupOfRecord(r) : CURRENT;
  const all = key === 'global'
    ? Object.values(PROFILES_BY).flat()
    : (PROFILES_BY[key] || []).concat(PROFILES_BY.global || []);
  const seen = new Set();
  return all.filter((x) => !seen.has(x) && seen.add(x));
}
const bumpRoles = () => { ROLES_REV.value += 1; };

// ---- YAML のタブ ------------------------------------------------------
// yfiles is the answer's own file list. A save replaces the file it wrote with
// a new one (the text and the hash are the server's), so the tab draws the
// file as it now stands (5 章 2).
export const yfiles = signal((LIST.files || []).map((f) => ({
  file: `${f.name}.${f.place}.${f.kind}`, place: f.place, name: f.name, kind: f.kind,
  label: PLACE[f.place] || '', path: f.path || '', hash: f.hash || '',
  text: f.text || '', hidden: !!f.hidden, problems: f.problems || [],
})));
export const fileOf = (file) => yfiles.value.find((f) => f.file === file) || null;
export const fileKeys = () => PLACES.flatMap((p) => yfiles.value.filter((f) => f.place === p).map((f) => f.file));
export const fileText = (file) => (state.value.ytext[file] !== undefined ? state.value.ytext[file] : ((fileOf(file) || {}).text || ''));
export const fileDirty = (file) => state.value.ytext[file] !== undefined && state.value.ytext[file] !== ((fileOf(file) || {}).text || '');
export const yIssues = (file) => state.value.yc[file] || (fileOf(file) || {}).problems || [];
export const recForFile = (f) => recs.value.find((x) => x.name.value === f.name && x.place.value === f.place && (x.project === CURRENT || x.project === '')) || null;
// pickFile chooses one file for the address when the tab opens: the first the
// places hold. The drawing itself asks chosenFile, which writes nothing.
export function pickFile() {
  const keys = fileKeys();
  if (!state.value.yfile || !keys.includes(state.value.yfile)) put({ yfile: keys[0] || null });
  return state.value.yfile;
}
export const chosenFile = () => {
  const keys = fileKeys();
  const y = state.value.yfile;
  return y && keys.includes(y) ? y : (keys[0] || null);
};
export function pressYfile(file) {
  put({ yfile: file });
  syncURL();
}
// yamltab's own state, kept by the screen: the text a person typed, and the
// problems the part's check answered.
export function yamlState(file, st) {
  const f = fileOf(file);
  withKey('ytext', file, f && st.text === f.text ? undefined : st.text);
}
export function yamlProblems(file, list) {
  withKey('yc', file, list);
}
export function yamlRevert(file) {
  withKey('ytext', file, undefined);
}
export function yamlSaved(file, answer) {
  const json = (answer && answer.json) || {};
  const typed = state.value.ytext[file];
  const f = fileOf(file);
  if (!f) return;
  const text = typeof json.text === 'string' ? json.text : (typed !== undefined ? typed : f.text);
  yfiles.value = yfiles.value.map((x) => (x.file === file ? { ...x, hash: json.hash || x.hash, text } : x));
  withKey('ytext', file, undefined);
  withKey('yc', file, undefined);
}

// promptsOf is each member's prompt, cut into the pieces it was built from
// (r-4cf9): the check answers it, and the column shows it without building a
// line of its own. A definition with a problem is not given any (d-20ac).
export const promptsOf = (r) => r.prompts.value;
export const promptBlockedOf = (r) => r.promptsReady.value === false;

// ---- 描くための小さな問い ----------------------------------------------
export const teamLabel = (r) => r.name.value || '新しいチーム定義';
export const currentProjectName = () => {
  const p = (HQ && HQ.projects) || {};
  return (p.current && p.current.name) || '';
};

// ---- 住所 --------------------------------------------------------------
export function syncURL() {
  const s = state.value;
  const params = new URLSearchParams();
  if (s.tab === 'yaml') { if (s.yfile) params.set('file', s.yfile); }
  else {
    const opens = [...s.open].filter((n) => { const r = byName(n); return r && !r.draftRow; });
    if (opens.length) params.set('open', opens.join(','));
    const agrees = opens.filter((n) => { const r = byName(n); return r && dtabOf(r) === 'agree'; });
    if (agrees.length) params.set('agree', agrees.join(','));
    const rules = [];
    opens.forEach((n) => {
      const r = byName(n);
      if (!r) return;
      ruleState.ropen.forEach((kk) => {
        const head = `${r.uid}${r.place.value}:`;
        if (!kk.startsWith(head)) return;
        const hit = rulesOf(r.agreeDraft.value).find((x) => x._k === Number(kk.slice(head.length)));
        if (hit && hit.id) rules.push(`${r.name.value}.${hit.id}`);
      });
    });
    if (rules.length) params.set('rule', rules.join(','));
  }
  const path = s.tab === 'yaml' ? '/teams/yaml' : '/teams';
  const q = params.toString();
  history.replaceState({}, '', q ? `${path}?${q}` : path);
}
export function readURL() {
  const p = new URLSearchParams(location.search);
  const open = p.get('open');
  if (open) {
    const rows = parseOpen(open) || [];
    rows.forEach(({ name, group }) => { const r = byRef(name, group); if (r) withOpen(r.dom, true); });
  }
  const agree = p.get('agree');
  if (agree) {
    const m = { ...state.value.dtab };
    agree.split(',').filter(Boolean).forEach((n) => { m[n] = 'agree'; });
    put({ dtab: m });
  }
  const rule = p.get('rule');
  put({ wantRule: null });
  if (rule) rule.split(',').filter(Boolean).forEach((spec) => {
    const i = spec.indexOf('.');
    if (i < 0) return;
    const r = byRealName(spec.slice(0, i));
    if (!r) return;
    const hit = rulesOf(r.agreeDraft.value).find((x) => x.id === spec.slice(i + 1));
    if (hit) {
      withOpen(r.dom, true);
      setDtab(r.dom, 'agree');
      ruleState.ropen.add(`${r.uid}${r.place.value}:${hit._k}`);
      put({ wantRule: { name: r.dom, k: hit._k } });
    }
  });
  const file = p.get('file');
  if (file) put({ yfile: file });
  if (location.pathname === '/teams/yaml') put({ tab: 'yaml' });
}

// ---- 共有の部品（ruledoc）の受け口 ------------------------------------
export const rowOfPanel = (r) => {
  const saved = r.agreeSaved.value, draft = r.agreeDraft.value;
  const check = r.check.value;
  return {
    uid: r.uid, l: r.place.value, e: { saved, draft, savedIssues: r.agreeSavedIssues.value },
    saved, draft, name: r.name.value,
    // newRow says the definition is not there yet: the rule document's shared
    // foot is not this row's own, so the part leaves it alone (監査 R9).
    newRow: !!r.draftRow,
    issues: agreeIssues(r),
    savedIssues: r.agreeSavedIssues.value,
    markdown: check ? check.markdown : { lines: [], chars: 0 },
    diff: check ? check.diff : { added: [], removedBefore: [], removedAtEnd: false },
    turns: check ? check.turns : [],
  };
};

// ---- 帯と、ページの外への約束 ------------------------------------------
// savedCount is the number the band shows: the definitions, which are the rows
// that are saved — a row that is not there yet is not one (spec-nav 1.1,
// 監査 R7). main.js keeps the band's number on it.
export const savedCount = () => recs.value.filter((r) => !r.draftRow).length;
// dirty is whether the screen holds anything the files do not: a row's ops, a
// rules draft, a YAML text. The band asks it through window.hergyPage (nav.js
// 1.5), the same way it asks the other screens. main.js keeps the band's dot on
// it.
export function dirty() {
  for (const r of recs.value) if (dirtyDef(r)) return true;
  for (const f of yfiles.value) if (fileDirty(f.file)) return true;
  return false;
}
// tellDirty says it again now, for a part that changed something without a
// value the screen can watch (lib/yamltab.js asks for it).
export function tellDirty() {
  setDirty(dirty());
}
// restore puts back what the band carried from this screen (spec-nav 1.5): the
// ops of a row that was not saved, the rules draft, and the YAML text. A file
// that moved outside since is not put back (its hash is not the one the draft
// was read from).
export function restore(carried) {
  (carried.teams || []).forEach((c) => {
    const r = recs.value.find((x) => x.name.value === c.name && x.place.value === c.place && !x.draftRow);
    if (!r || (c.hash && r.hash.value !== c.hash)) return;
    r.ops.value = clone(c.ops);
    checkTeam(r);
  });
  (carried.agrees || []).forEach((c) => {
    const r = recs.value.find((x) => x.name.value === c.name && x.place.value === c.place && !x.draftRow);
    if (!r || !r.agreeSaved.value || (c.hash && r.agreeHash.value !== c.hash)) return;
    const d = fromJSON(c.doc);
    d.__path = r.agreePath.value;
    r.agreeDraft.value = d;
    checkAgree(r);
  });
  (carried.files || []).forEach((c) => {
    const f = fileOf(c.file);
    if (!f || (c.hash && f.hash !== c.hash)) return;
    withKey('ytext', c.file, c.text);
  });
}

export function take() {
  const teams = [];
  for (const r of recs.value) if (!r.draftRow && changeCount(r) > 0) teams.push({ place: r.place.value, name: r.name.value, hash: r.hash.value, ops: r.ops.value });
  const agrees = [];
  for (const r of recs.value) if (!r.draftRow && r.agreeSaved.value && agreeDiff(r)) agrees.push({ place: r.place.value, name: r.name.value, hash: r.agreeHash.value, doc: toDoc(r.agreeDraft.value, r.agreeSaved.value) });
  const files = yfiles.value.filter((f) => fileDirty(f.file)).map((f) => ({ file: f.file, hash: f.hash, text: state.value.ytext[f.file] }));
  return { teams, agrees, files };
}

// ---- 押し込み（部品が呼ぶ） --------------------------------------------
export function pressTab(t) {
  if (state.value.tab === t) return;
  put({ tab: t });
  syncURL();
}
export function pressToggle(r) {
  withOpen(r.dom, !isOpen(r.dom));
  syncURL();
}
export async function pressNew() {
  await addDraft('new', 'project', '', null, GROUPED ? CURRENT : '');
}
export async function pressDup(r) {
  await addDraft('dup', r.place.value, theName(r.name.value), { place: r.place.value, name: r.name.value }, r.project);
}
export async function pressCopy(r) {
  await addDraft('copy', other(r.place.value), r.name.value, { place: r.place.value, name: r.name.value }, r.project);
}
// pressPlace moves a definition that is not there yet to the other place. The
// row keeps its place in the list while its place changes (spec-teams B1, N4).
export async function pressPlace(r, place) {
  if (!r || !r.draftRow || r.place.value === place) return;
  const at = firstIndexOf((x) => x === r);
  r.place.value = place;
  orderRows(r, at);
  resetCanvas(r);
  // The base is read again: the other place's role documents and profile are
  // not the ones the row stood on (spec-teams B2).
  r.seeded.value = false;
  r.saved.value = cloneTeam(null);
  r.agreeSaved.value = null;
  await checkTeam(r);
  focusAsk.value = { dom: r.dom, sel: `[data-act=pick-place][data-x="${place}"]` };
}
export function pressDiscard(r) {
  if (r) dropDraft(r);
}
export function pressDel(r) { setConfirm('del' + dkey(r), true); }
export function pressDelNo(r) { setConfirm('del' + dkey(r), false); }
export async function pressDelYes(r) { if (r) await doDelete(r); }
// pressName is a definition that is not there yet being named: the name is
// checked, and the files a save would write are read again: the server owns
// both (spec-teams B2). The field keeps its place and its cursor while the
// person types.
export function pressName(r, value) {
  r.name.value = value;
  debounce('n' + r.dom, () => checkTeam(r));
}
// pressMission is the row's own mission field: it writes one operation and asks
// the check again (spec-teams 1.4.6).
export function pressMission(r, value) {
  const text = value.replace(/\n/g, ' ');
  r.ops.value = r.ops.value.filter((o) => o.kind !== 'mission');
  if (text !== r.saved.value.mission) pushOp(r, { kind: 'mission', text });
  checkTeam(r);
}
export function pressDtab(r, t) {
  setDtab(r.dom, t);
  syncURL();
  if (t === 'agree') checkAgree(r);
}
export function pressPmode(r, v) {
  put({ pmode: { ...state.value.pmode, [dkey(r)]: v } });
}
export function pressToYaml(key) {
  put({ tab: 'yaml', yfile: `${key}.team` });
  syncURL();
  window.scrollTo(0, 0);
}
export function pressTSave(r) { saveTeam(r); }
export function pressTRevert(r) {
  setOps(r, []);
  resetCanvas(r);
  forgetUndo(r);
  put({ confirm: {} });
  syncTeam(r);
}
export function pressSave(r) { saveAgree(r); }
export function pressRevert(r) {
  r.agreeDraft.value = clone(r.agreeSaved.value);
  checkAgree(r);
}
export function pressToastUndo() { canvasUndoFromToast(); }
// pressVrule opens the team's own rule a finding names (spec-teams 1.4.5,
// 監査 R9) in the ルール tab, with the cursor in its body (監査 R12).
export function pressVrule(r, id) {
  if (!r) return;
  withOpen(r.dom, true);
  setDtab(r.dom, 'agree');
  const hit = rulesOf(r.agreeDraft.value).find((x) => String(x.id) === id);
  checkAgree(r);
  syncURL();
  if (hit) jumpToRule(r, hit._k);
}
// pressVpick puts the cursor in a finding's own field, or picks what it names on
// the figure.
export function pressVpick(r, m) {
  if (m.nameField) { focusAsk.value = { dom: r.dom, sel: '.in-name' }; return; }
  if (m.node) selectPick(r, { type: 'node', id: m.node }, 'attr');
  else if (m.worker != null) selectPick(r, { type: 'worker', i: m.worker }, 'attr');
}
// pressLine is a press on one line of the topology tab's YAML preview: it picks
// what the line stands for, the way a press on the canvas's own point does
// (spec-teams B7). A mission line puts the cursor in the mission field.
export function pressLine(r, p) {
  if (p.f === 'mission') {
    selectPick(r, null, 'attr');
    focusAsk.value = { dom: r.dom, sel: '[data-cv-f="mission"]' };
    return;
  }
  if (p.node) selectPick(r, { type: 'node', id: p.node }, 'attr');
  else if (p.worker != null) selectPick(r, { type: 'worker', i: p.worker }, 'attr');
  else if (p.a) selectPick(r, { type: 'edge', k: [p.a, p.b].sort().join('|') }, 'attr');
  else selectPick(r, null, 'attr');
}
// selectPick is the canvas's own pick, for a part that picks without a press on
// the figure (canvas.js keeps the drawing of it).
export function selectPick(r, sel, side) {
  r.typed.value = null;
  r.sel.value = sel;
  if (side) r.side.value = side;
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
  setStick();
  const later = [];
  // The member's 属性 offers the roles the role screen holds; the team answer
  // carries no role documents, so they are read once from GET /api/role. A panel
  // already drawn gets them when the answer comes (the first frame is quick).
  later.push(apiList('role').then((list) => {
    ROLES.value = (list.items || []).map((it) => {
      const places = it.places || {}, eff = it.effective || (places.project ? 'project' : 'global');
      const doc = (places[eff] || {}).doc || {};
      return { name: it.name || '', title: doc.title || '' };
    }).filter((x) => x.name);
    if (!HQ) ROLES_IN.value = true;
    bumpRoles();
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
      if (map === ROLES_BY) ROLES_IN.value = true;
      bumpRoles();
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
  const first = recs.value.slice();
  const checks = first.flatMap((r) => [checkTeam(r), checkAgree(r)]);
  Promise.all(checks).then(() => {
    // A rule the address names opens with the cursor in its body, once the
    // panels the first checks redrew have settled (spec-teams 2, spec-roles
    // 1.2.4, 監査 R12).
    jumpWantRule();
  });
  return Promise.allSettled([...later, ...checks]);
}
// openAgree puts a row on its ルール tab and asks the rules' check: the same
// step the tab's press takes, for a part that names the tab without a press.
export function openAgree(r) {
  withOpen(r.dom, true);
  setDtab(r.dom, 'agree');
  checkAgree(r);
  syncURL();
}

// jumpToRule opens one rule of a team's own agreement and puts the cursor in
// its body (spec-teams 1.4.5, spec-roles 1.2.4, 監査 R12). The check's column
// reads through this, and so does the address.
export function jumpToRule(r, k) {
  focusAsk.value = null;
  pressPmode(r, 'edit');
  openRule(rowOfPanel(r), k);
  syncURL();
}
export function jumpWantRule() {
  const w = state.value.wantRule;
  if (!w) return;
  put({ wantRule: null });
  const r = byName(w.name);
  if (r) jumpToRule(r, w.k);
}
export const setStick = () => {
  const bar = $('.bar');
  document.documentElement.style.setProperty('--stick', (bar ? bar.offsetHeight : 48) + 'px');
};
// readAddress is the back button's way back: the rows the address opens, the
// rule it opens and the file it chose are read again.
export function readAddress() {
  put({ open: new Set() });
  readURL();
  jumpWantRule();
}
// The view needs these to read one row's parts.
export { hasTurn, memberOf };
