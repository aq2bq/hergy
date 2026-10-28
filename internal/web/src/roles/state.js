// roles/state.js — ロールの画面の state（webui-preact-design 4 章・5 章）。
//
// 一覧の行、割り当てのチーム、YAML のファイル、そしてサーバーのチェックの答えは、
// すべてここに signal で置く（5 章 2）。部品（view.js / assign.js / yaml.js）は
// signal を読んで描き、値が変われば Preact が自分で描き直す。だから描き直しを
// 呼ぶ関数は無い（依頼書 1）。行の下書きは共通部品（ruledoc）が書き換え、変える
// たびに onChange で知らせるので、画面はそのたびに新しい行（新しい下書きの写し）を
// signal に入れる。
import { signal } from '../lib/preact.js';
import { resource } from '../lib/resource.js';
import { takeDraft, setCount, setDirty } from '../lib/nav.js';
import { parseOpen, token as hqToken } from '../lib/hq.js';
import {
  check as apiCheck, save as apiSave, del as apiDel, saved as apiSaved,
  roleModel, teamModel, problemText, checkAnswer,
} from '../lib/api.js';
import {
  fromJSON, toDoc, rulesOf, inP, nextKey, observeKeys, openRules, openRuleKey, renameCarries,
} from '../lib/ruledoc.js';

export const view = document.getElementById('view');
export const tabsEl = document.getElementById('tabs');

const ISLAND = (() => {
  const el = document.getElementById('roles-data');
  if (!el) return {};
  try { return JSON.parse(el.textContent || '{}'); } catch (e) { return {}; }
})();

export const PLACES = ['project', 'global'];
export const DIR = Object.assign({ project: '.hergy/roles/', global: '~/.config/hergy/roles/' }, ISLAND.places || {});
export const TEXT = ISLAND.texts || {};
// fill is the server's sentence with the screen's numbers in it: the words stay
// the server's (the same ones hergy validate prints).
export const fill = (s, ...a) => String(s || '').replace(/%[sd]/g, () => (a.length ? a.shift() : ''));
export const LNAME = { project: 'プロジェクト', global: 'グローバル' };
export const clone = (o) => JSON.parse(JSON.stringify(o));
const other = (l) => (l === 'project' ? 'global' : 'project');
let uid = 0;

// HQ is the headquarters lists (d-73fc): every listed project's roles and team
// definitions, then the global layer's. Without it the screen is the one
// project's, as it always was.
export const HQ = ISLAND.hq || null;
// GROUPED is a headquarters with projects: listed: every row stands in a group
// and carries its project. Without projects: the screen is the one project's,
// the way it always was (d-73fc).
export const GROUPED = !!(HQ && HQ.projects && HQ.projects.mode === 'listed');
export const CURRENT = GROUPED ? ((HQ.projects.current && HQ.projects.current.path) || '') : '';
export const groupKeyOf = (g) => (g.global ? 'global' : (g.project ? g.project.path : ''));
export const groupNameOf = (g) => (g.global ? 'グローバル' : (g.project ? g.project.name : ''));
export const currentGroup = () => (GROUPED ? CURRENT : '');
// currentProjectName is the name the プロジェクト column gives this serve's
// project, whether or not it is listed (d-8a45, r-5b96 audit 5).
export const currentProjectName = () => {
  if (!HQ) return LNAME.project;
  const p = HQ.projects || {};
  return (p.current && p.current.name) || '';
};
export const currentGroupName = () => {
  if (!HQ) return '';
  const p = HQ.projects || {};
  if (p.mode === 'current') return (p.current && p.current.name) || '';
  const found = (p.projects || []).find((x) => x.path === CURRENT);
  return found ? found.name : ((p.current && p.current.name) || '');
};

// ---- the model -------------------------------------------------------
// rowsOf is every row the list draws: each group's answer read with the same
// api.js model as the one project's, and every row given its project's key and
// its own uid, so two projects' rows never share one.
function rowsOf(model, group) {
  return model.roles.map((r) => {
    const row = { uid: r.uid, group };
    if (r.markdown) { row.markdown = r.markdown; row.name = r.name || ''; row.project = null; row.global = null; return row; }
    row.paths = {};
    for (const l of PLACES) {
      if (r[l]) {
        const draft = fromJSON(r[l]);
        // A role no layer holds a document for opens as a new one (spec-roles
        // 1.2.10): nothing is saved yet, so the first 保存 writes the file, and
        // the row counts no rules until it is saved.
        const saved = r.noDoc ? null : draft;
        row[l] = { saved, draft: clone(draft), hash: r[l].hash || '', text: r[l].text || '' };
        if (r[l].path) row.paths[l] = r[l].path;
      } else { row[l] = null; }
    }
    if (r.noDoc) row.noDoc = true;
    if (r.isNew) row.isNew = true;
    return row;
  });
}
function initialRoles() {
  if (!GROUPED) return rowsOf(roleModel(ISLAND.role), '');
  const out = [];
  (HQ.roles || []).forEach((g) => { rowsOf(roleModel(g.list), groupKeyOf(g)).forEach((r) => out.push(r)); });
  out.forEach((r) => { r.uid = ++uid; });
  return out;
}

// ---- the screen's state ---------------------------------------------
// roles is every row; state is what the screen knows about itself; checks and
// ychecks are the server's answers; teams is the assign tab's cards. Each is a
// signal, and every change puts a new value in (5 章 2).
const FIRST = initialRoles();
export const roles = signal(FIRST);
export const state = signal({
  tab: 'roles', open: new Set(), filter: null, q: '', roleHi: null,
  confirm: {}, ok: {}, yfile: null, pmode: {}, refusals: {}, teamErrors: {},
});
export const checks = signal({});
export const ychecks = signal({});
// okShowing is the 保存しました the current drawing shows: a save's answer shows
// it once, until the next change (the part that says a document is saved).
export const okShowing = signal({});
// focusAsk and gotoAsk are where the caret goes after a press: which row's which
// field (5 章 5), and which member of the assign tab a chip sent it to (P11).
export const focusAsk = signal(null);
export const gotoAsk = signal(null);

const patch = (part) => { state.value = { ...state.value, ...part }; };
export const askFocus = (id, sel) => {
  focusAsk.value = { uid: Number(id), sel, seq: ((focusAsk.value && focusAsk.value.seq) || 0) + 1 };
};
export const askGoto = (team, member, i) => {
  gotoAsk.value = { team, member, i, seq: ((gotoAsk.value && gotoAsk.value.seq) || 0) + 1 };
};

const TEAMS = teamModel(ISLAND.team).teams.map((t) => ({
  id: t.id, hash: t.hash || '', place: t.place || 'project', turns: t.turns || [],
  members: (t.members || []).map((m) => ({ name: m.name, saved: m.saved, draft: m.draft })),
  group: currentGroup(),
}));
// The definitions that read the roles: the current project's for the 割り当て
// tab, and every group's for the 使っているチーム column.
export const teams = signal(TEAMS);
export const hqTeams = [];
if (GROUPED) {
  (HQ.teams || []).forEach((g) => {
    const key = groupKeyOf(g);
    ((g.list && g.list.items) || []).forEach((item) => {
      const sum = item.summary || {};
      hqTeams.push({
        id: item.name || '', group: key,
        members: (sum.members || []).map((m) => ({ name: m.name, saved: m.role || '', draft: m.role || '' })),
      });
    });
  });
}
// A row a person adds gets an id of its own: start past the ones the island
// carries, so a new row never takes an opened row's place.
uid = FIRST.reduce((m, r) => Math.max(m, r.uid || 0), uid);
export const nextUid = () => { uid += 1; return uid; };

// ---- lookups ---------------------------------------------------------
export const byUid = (id) => roles.value.find((r) => r.uid === Number(id));
export const live = (r, l) => !!(r[l] && r[l].saved);
export const eff = (r) => (live(r, 'project') ? 'project' : live(r, 'global') ? 'global' : (r.project ? 'project' : 'global'));
export const cur = (r) => r[eff(r)].draft;
export const curName = (r) => (r.markdown ? r.name || '' : (r[eff(r)].saved || r[eff(r)].draft).name);
export const savedRoles = () => roles.value.filter((r) => live(r, 'project') || live(r, 'global'));
export const roleByName = (n) => savedRoles().find((r) => curName(r) === n);
// whoReads is the members that name a role: the row's own project's teams, and
// for a global row every project's (spec 4 章). Each answer says which project
// the team belongs to.
export const whoReads = (n, group) => {
  const pool = GROUPED ? hqTeams.filter((t) => (group === 'global' ? true : t.group === group)) : teams.value;
  return pool.flatMap((t) => t.members.filter((m) => m.saved === n).map((m) => ({ t, m })));
};
export const projectNameOf = (key) => {
  if (key === 'global') return 'グローバル';
  if (key === '' || key === CURRENT) return currentGroupName();
  const found = HQ && ((HQ.projects || {}).projects || []).find((x) => x.path === key);
  return found ? found.name : String(key || '').split('/').filter(Boolean).pop() || '';
};
export const pathOf = (l, d) => DIR[l] + (d.name || '…') + '.yaml';
// pathOfRow is the file one panel stands behind: the answer's own path when the
// row is another project's, and the one this project's places build.
export const pathOfRow = (r, l, d) => (r.paths && r.paths[l]) || pathOf(l, d);

// ---- the rows, changed ----------------------------------------------
// withRow puts a new row in place of one: the array is new, so every part that
// reads roles is drawn again, and the row itself is the new one (5 章 2).
export const withRow = (id, fn) => {
  const one = Number(id);
  roles.value = roles.value.map((r) => (r.uid === one ? fn(r) : r));
};
// withEntry puts a new place's entry in place of one row's, and the row with it.
export const withEntry = (id, l, fn) => withRow(id, (r) => (r[l] ? { ...r, [l]: fn(r[l]) } : r));

// touch is what a change that the rows do not show by themselves goes through:
// the 保存しました that stands for one drawing, the band's dot and its number.
export function touch() {
  okShowing.value = { ...state.value.ok };
  if (Object.keys(state.value.ok).length) patch({ ok: {} });
  roles.value = roles.value.slice();
  tellDirty();
  setCount('roles', savedRoles().length);
}

// ---- server checks ---------------------------------------------------
// checkers are one resource per document place: a check is a reading like any
// other, so the newest request is the one whose answer is kept, and the one
// before it is dropped (5 章 3). The screen keeps no numbering of its own.
const checkers = new Map();
function checkerFor(k) {
  let one = checkers.get(k);
  if (one) return one;
  one = resource(async (signal, payload) => {
    try {
      return await apiCheck('role', payload, { signal });
    } catch (err) {
      // A refusal about the text still carries what is wrong with it.
      if (err && err.json) return { json: err.json };
      throw err;
    }
  });
  checkers.set(k, one);
  return one;
}
const timers = {};
const keyOf = (r, l) => r.uid + l;
const ruleKeyAt = (d, si, ri) => { const s = d.sections[si]; if (!s) return 0; const x = s.rules[ri]; return x ? x._k : 0; };

function convertProblems(d, list) {
  return (list || []).map((p) => ({
    sev: p.sev, k: p.section >= 0 ? ruleKeyAt(d, p.section, p.rule) : 0,
    f: p.field || '', msg: p.text || '', definition: p.definition || '',
  }));
}

export function issuesOf(r, l) { const c = checks.value[keyOf(r, l)]; return c ? c.issues : []; }

// rowOf is one panel as the shared part reads it: the draft and the saved
// document, and the check the server last answered (its problems, the Markdown,
// the diff, the arrows).
export function rowOfPanel(r, l) {
  const e = r[l], c = checks.value[keyOf(r, l)];
  return {
    uid: r.uid, l, e, saved: e.saved, draft: e.draft,
    issues: c ? c.issues : [], markdown: c ? c.markdown : { lines: [], chars: 0 },
    diff: c ? c.diff : { added: [], removedBefore: [], removedAtEnd: false }, turns: c ? c.turns : [],
    savedIssues: e.__savedIssues,
  };
}

export async function askCheck(r, l, cb) {
  const e = r[l];
  if (!e) return;
  const k = keyOf(r, l);
  // A check that a newer one has replaced answers nothing (the resource dropped
  // it), so only the newest reading reaches the panel.
  const answer = await checkerFor(k).run({
    place: l, original: e.saved ? e.saved.name : null,
    project: r.group && r.group !== 'global' ? r.group : undefined,
    doc: toDoc(e.draft, e.saved || e.base),
  });
  if (!answer || !answer.json) return;
  const c = checkAnswer(answer);
  const issues = convertProblems(e.draft, c.problems);
  checks.value = {
    ...checks.value,
    [k]: {
      issues,
      markdown: c.preview || { lines: [], chars: 0 },
      diff: c.diff || { added: [], removedBefore: [], removedAtEnd: false },
      turns: c.turns || [], readers: c.readers, size: c.size, limit: c.limit,
    },
  };
  // A save's refusal under ロール名 is the same answer, older: the check speaks
  // for the name field now, so a refusal it no longer holds goes (d-841e).
  const shown = state.value.refusals[k];
  if (shown && shown.at === 'name' && !issues.some((m) => m.f === 'name')) {
    const next = { ...state.value.refusals };
    delete next[k];
    patch({ refusals: next });
  }
  if (cb) cb();
}

export function scheduleCheck(r, l) {
  const k = keyOf(r, l);
  clearTimeout(timers[k]);
  timers[k] = setTimeout(() => askCheck(r, l, () => { updateRowSummary(r.uid); touch(); }), 300);
}
export const clearTimer = (r, l) => clearTimeout(timers[keyOf(r, l)]);

// updateRowSummary is what a check's answer changes about the row itself: the
// saved document's own problems are what a save is held to (X3).
function updateRowSummary(id) {
  const r = byUid(id);
  if (!r) return;
  const entries = {};
  PLACES.forEach((l) => {
    if (!r[l] || !r[l].saved) return;
    const c = checks.value[keyOf(r, l)];
    if (c) entries[l] = { ...r[l], __savedIssues: c.issues };
  });
  if (Object.keys(entries).length) withRow(id, (old) => ({ ...old, ...entries }));
}

export const okOf = (k) => !!okShowing.value[k];
export const refusalOf = (k) => state.value.refusals[k] || null;
export const teamErrorOf = (id) => state.value.teamErrors[id] || '';

// ---- URL -------------------------------------------------------------
// openNewPanel opens the row a screen shows for a role no document holds: its
// one empty rule stands open and its check runs once, the way ＋ ロールを追加
// does (spec-roles 1.2.4, 1.2.10). Until the check comes back the preview would
// say 0 行 · 0 文字, which is not what the row holds.
export function openNewPanel(r) {
  const l = eff(r), e = r[l];
  const first = e && e.draft.sections[0] && e.draft.sections[0].rules[0];
  if (!first) return;
  openRuleKey(`${r.uid}${l}:${first._k}`);
  if (!checks.value[keyOf(r, l)]) askCheck(r, l, () => touch());
}

export function parseURL() {
  const path = location.pathname;
  const tab = /\/roles\/assign$/.test(path) ? 'assign' : /\/roles\/yaml$/.test(path) ? 'yaml' : 'roles';
  const qs = new URLSearchParams(location.search);
  const next = { tab };
  if (tab === 'roles') {
    const f = qs.get('filter');
    next.filter = (f === 'off' || f === 'issues') ? f : null;
    next.q = qs.get('q') || '';
    const openedRaw = (qs.get('open') || '');
    const openedRows = parseOpen(openedRaw) || [];
    const open = new Set(state.value.open);
    openedRows.forEach(({ name, group: g }) => {
      // A row is found by the name it shows and, from a search result, by the
      // project it stands in: a saved name, a written one, or the name of a
      // document only Markdown holds (spec-roles 2 節, spec 3 章).
      const r = roles.value.find((x) => curName(x) === name && (!g || x.group === g)) || roles.value.find((x) => curName(x) === name);
      if (!r) return;
      open.add(r.uid);
      if (r.noDoc) openNewPanel(r);
    });
    next.open = open;
    // A rule is given as <role>.<place>.<id>, or as its id alone when the
    // opening names the role (the team screen links so, d-8bbf).
    (qs.get('rule') || '').split(',').filter(Boolean).forEach((spec) => {
      const m = /^(.*)\.(project|global)\.(.+)$/.exec(spec);
      const wanted = m ? [{ name: m[1], place: m[2], id: m[3] }] : openedRows.map(({ name }) => ({ name, place: null, id: spec }));
      wanted.forEach(({ name, place, id }) => {
        const r = roleByName(name);
        if (!r) return;
        const places = place ? [place] : [eff(r)];
        places.forEach((l) => {
          if (!r[l] || !r[l].saved) return;
          const hit = rulesOf(r[l].saved).find((x) => x.id === id);
          if (hit) { open.add(r.uid); openRuleKey(`${r.uid}${l}:${hit._k}`); }
        });
      });
    });
    next.open = open;
  } else if (tab === 'assign') {
    next.roleHi = qs.get('role') || null;
  } else {
    const f = qs.get('file');
    if (f) {
      const m = /^(project|global)\.(.+)$/.exec(f);
      if (m) {
        const r = roleByName(m[2]);
        if (r && r[m[1]] && r[m[1]].saved && eff(r) === m[1]) next.yfile = r.uid + m[1];
      }
    }
  }
  patch(next);
}

export function syncURL(push) {
  const s = state.value;
  let path = '/roles';
  const q = new URLSearchParams();
  if (s.tab === 'assign') {
    path = '/roles/assign';
    if (s.roleHi) q.set('role', s.roleHi);
  } else if (s.tab === 'yaml') {
    path = '/roles/yaml';
    if (s.yfile) { const m = /^(\d+)(project|global)$/.exec(s.yfile); if (m) { const r = byUid(m[1]); if (r) q.set('file', m[2] + '.' + curName(r)); } }
  } else {
    if (s.filter) q.set('filter', s.filter);
    if (s.q) q.set('q', s.q);
    const open = [...s.open].map((u) => { const r = byUid(u); return r ? (GROUPED ? hqToken(curName(r), r.group) : curName(r)) : ''; }).filter(Boolean);
    if (open.length) q.set('open', open.join(','));
    const rule = [...openRules()].map((kk) => {
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

// ---- the list's filters ---------------------------------------------
export const FILTERS = [[null, 'すべて'], ['off', 'オフがある'], ['issues', '問題・警告']];
export const allEntries = () => roles.value.flatMap((r) => PLACES.filter((l) => r[l]).map((l) => ({ r, l, e: r[l] })));
export function ruleShown(x, issues) {
  const s = state.value;
  if (s.filter === 'off' && inP(x)) return false;
  if (s.filter === 'issues' && !issues.some((m) => m.k === x._k)) return false;
  if (s.q) { const q = s.q.toLowerCase(); if (![x.id, x.text, x.note].some((v) => String(v).toLowerCase().includes(q))) return false; }
  return true;
}
export const filtering = () => !!(state.value.filter || state.value.q);
export const rowIssues = (r) => PLACES.filter((l) => r[l]).flatMap((l) => issuesOf(r, l));

export function filterCount(f) {
  if (f === 'off') return roles.value.filter((r) => PLACES.some((l) => r[l] && rulesOf(r[l].draft).some((x) => !inP(x)))).length;
  return allEntries().reduce((c, { r, l, e }) => {
    const is = issuesOf(r, l);
    return c + rulesOf(e.draft).filter((x) => f === null || is.some((m) => m.k === x._k)).length;
  }, 0);
}

// ---- the YAML tab's own state ---------------------------------------
// A file's problems are what the file nav marks a file with; the part checks the
// text and hands them back (yamltab.js).
export const ykey = (place, name) => place + '.' + name;
export const ychecksOf = (place, name) => ychecks.value[ykey(place, name)];
export const setYchecks = (place, name, list) => { ychecks.value = { ...ychecks.value, [ykey(place, name)]: list }; };
// filesList is the YAML tab's files: this serve's project and the global layer,
// which are the two places the tab writes to. Another project's rows are read and
// written in the list, not here.
export const filesList = () => allEntries().filter(({ r, l }) => live(r, l) && (!GROUPED || r.group === CURRENT || r.group === 'global'));
export const docText = (r, l) => { const e = r[l]; return e.ytext !== undefined ? e.ytext : (e.text || ''); };

// ---- operations ------------------------------------------------------
// A refusal stands where its field does: a name's refusal under the name field,
// the document's in the panel's footer (spec-roles 1.2.9).
export function showRefusal(key, at, text) {
  patch({ refusals: { ...state.value.refusals, [key]: { at: at === 'name' ? 'name' : 'doc', text: text || '保存できませんでした' } } });
  touch();
}
export function clearRefusal(key) {
  if (!state.value.refusals[key]) return;
  const next = { ...state.value.refusals };
  delete next[key];
  patch({ refusals: next });
  touch();
}

// pressToggle opens or closes a row. A row a person presses into opens in front
// of them; the caret goes back to the line they pressed (5 章 5).
export function pressToggle(idPressed) {
  const r = byUid(idPressed);
  if (!r) return;
  const open = new Set(state.value.open);
  if (open.has(r.uid)) open.delete(r.uid);
  else { open.add(r.uid); if (r.noDoc) openNewPanel(r); }
  patch({ open });
  askFocus(r.uid, '.sum');
  touch();
  syncURL(false);
}

export function pressFilter(f) {
  patch({ filter: f || null });
  touch();
  syncURL(false);
}

export function pressNew() {
  const d = { name: '', title: '', revision: 1, sections: [{ _s: 0, heading: '使命', level: 0, intro: '', rules: [{ _k: nextKey(), id: '', text: '', note: '', in_prompt: true, off_since: '' }] }] };
  const nr = { uid: nextUid(), isNew: true, group: currentGroup(), project: { saved: null, draft: d, hash: '', text: '' }, global: null };
  roles.value = [nr, ...roles.value];
  const open = new Set(state.value.open);
  open.add(nr.uid);
  patch({ open, filter: null, q: '' });
  openRuleKey(`${nr.uid}project:${d.sections[0].rules[0]._k}`);
  touch();
  askCheck(nr, 'project', () => { askFocus(nr.uid, '[data-f=name]'); touch(); });
}

export function pressPickPlace(id, l, x) {
  if (x === l) return;
  withRow(id, (r) => ({ ...r, [x]: r[l], [l]: null }));
  touch();
}

export function pressPmode(id, l, v) {
  // The switch's button is the one the person pressed: it keeps the focus
  // through the draw, so nothing is put back (5 章 5).
  patch({ pmode: { ...state.value.pmode, [id + l]: v } });
  touch();
}

export function pressDup(id, l) {
  // The copy keeps the places its rules stood in (their _k), so the server
  // writes the new file from the original's own lines (spec-roles 1.2.9).
  const r = byUid(id);
  const src = r && r[l] && r[l].saved;
  if (!src) return;
  const d = clone(src);
  d.name += '-2';
  const nr = { uid: nextUid(), isNew: true, group: r.group, [l]: { saved: null, draft: d, base: clone(src), basePlace: { place: l, name: src.name }, hash: '', text: '' } };
  nr.project = nr.project || null;
  nr.global = nr.global || null;
  const at = roles.value.indexOf(r);
  roles.value = [...roles.value.slice(0, at + 1), nr, ...roles.value.slice(at + 1)];
  const open = new Set(state.value.open);
  open.add(nr.uid);
  patch({ open });
  touch();
  askCheck(nr, l, () => { askFocus(nr.uid, '[data-f=name]'); touch(); });
}

export function pressCopy(id, l) {
  const r = byUid(id);
  if (!r || !r[l] || !r[l].saved) return;
  const e = { saved: null, draft: clone(r[l].saved), base: clone(r[l].saved), basePlace: { place: l, name: r[l].saved.name }, hash: '', text: '' };
  withRow(id, (old) => ({ ...old, [other(l)]: e }));
  touch();
  askCheck(byUid(id), other(l), () => touch());
}

export function pressDel(id, l) {
  patch({ confirm: { ...state.value.confirm, [id + l]: 1 } });
  askFocus(id, '[data-act=del-yes]');
  touch();
}

export function pressDelNo(id, l) {
  const next = { ...state.value.confirm };
  delete next[id + l];
  patch({ confirm: next });
  touch();
}

export function pressDiscard(id, l) {
  const next = { ...state.value.confirm };
  delete next[id + l];
  patch({ confirm: next });
  const r = byUid(id);
  if (!r) return;
  if (r[l] && r[l].saved) {
    withEntry(id, l, (e) => ({ ...e, draft: clone(e.saved) }));
  } else {
    withRow(id, (old) => ({ ...old, [l]: null }));
  }
  const after = byUid(id);
  if (after && !after.project && !after.global) {
    roles.value = roles.value.filter((x) => x.uid !== id);
    const open = new Set(state.value.open);
    open.delete(Number(id));
    patch({ open });
  }
  touch();
}

// pressSearch is the list's search: the letters go into the state the list is
// drawn from. The field is the same element after the draw, so the cursor stays
// where the person put it (5 章 5).
export function pressSearch(value) {
  patch({ q: value });
  touch();
  syncURL(false);
}

export function pressTab(t) {
  patch({ tab: t });
  touch();
  syncURL(true);
}

export function pressRoleHi(name) {
  patch({ roleHi: state.value.roleHi === name ? null : name });
  touch();
  syncURL(false);
}

// pressGoto walks to the assign tab and puts the cursor on the member the row
// named (spec-roles 2「割り当てのタブ」).
export function pressGoto(team, member, i) {
  patch({ tab: 'assign' });
  touch();
  syncURL(true);
  askGoto(team, member, i);
  history.replaceState(null, '', `${location.pathname}${location.search}#${team}.m-${i}`);
}

// pickRole is one member's role picked in the assign tab: the pick is the
// screen's draft until the team's 保存 writes it.
export function pickRole(id, i, value) {
  const t = teams.value.find((x) => x.id === id);
  if (!t) return;
  // The select is the same element after the draw, so the cursor stays in it:
  // nothing puts it back (5 章 5).
  teams.value = teams.value.map((one) => (one.id !== id
    ? one
    : { ...one, members: one.members.map((m, j) => (j === Number(i) ? { ...m, draft: value } : m)) }));
  touch();
}

export function pressYfile(key) {
  patch({ yfile: key });
  touch();
  syncURL(false);
}

// setYfile is the file the tab shows when the address names none: the first one.
export function setYfile(key) {
  patch({ yfile: key });
}

export function pressYamlRevert(r, l) {
  // The text goes back to the saved one, and the shared part checks the text it
  // now shows: a result for the thrown-away text must not stay.
  if (r && r[l]) withEntry(r.uid, l, (e) => { const next = { ...e }; delete next.ytext; return next; });
}

// ---- writes ----------------------------------------------------------
export async function doSave(id, l) {
  const r = byUid(id);
  const e = r && r[l];
  if (!e) return;
  const key = id + l;
  // Whether the rename carries into the teams that read the role is the rule
  // document's state, not a box the screen reads back (5 章 2).
  const keepRename = renameCarries(key);
  const payload = { place: l, original: e.saved ? e.saved.name : null, doc: toDoc(e.draft, e.saved || e.base), rename_users: keepRename, hash: e.hash || '' };
  if (r.group && r.group !== 'global') payload.project = r.group;
  if (e.basePlace) payload.base = e.basePlace;
  let res;
  try {
    res = await apiSave('role', e.draft.name || (e.saved ? e.saved.name : '_'), payload);
  } catch (err) {
    showRefusal(key, err && err.json && err.json.at, problemText(err, '保存できませんでした'));
    return;
  }
  clearRefusal(key);
  const json = res.json || {};
  const now = byUid(id);
  const cur0 = now && now[l];
  if (!cur0) return;
  const old = cur0.saved;
  const newName = cur0.draft.name;
  // The written document comes back with the answer: hergy wrote the day a rule
  // was switched off (off_since), so the screen takes the document the server
  // made rather than the one it sent (d-85dc).
  const saved = json.doc ? fromJSON(json.doc) : clone(cur0.draft);
  withEntry(id, l, (one) => {
    const next = { ...one, hash: json.hash || one.hash, saved, draft: clone(saved) };
    // The written text comes back with the answer, so the YAML tab shows the file
    // this save just made (a moved line, a rule the write re-emitted).
    if (typeof json.text === 'string') next.text = json.text;
    delete next.base;
    delete next.basePlace;
    delete next.ytext;
    return next;
  });
  withRow(id, (one) => {
    const next = { ...one };
    delete next.isNew;
    delete next.noDoc;
    return next;
  });
  if (old && old.name !== newName && keepRename) {
    teams.value = teams.value.map((t) => ({
      ...t,
      members: t.members.map((m) => ({ ...m, saved: m.saved === old.name ? newName : m.saved, draft: m.draft === old.name ? newName : m.draft })),
    }));
    const o = other(l);
    if (!GROUPED && now[o]) {
      const split = { uid: nextUid(), project: null, global: null, group: now.group };
      split[o] = now[o];
      const at = roles.value.findIndex((x) => x.uid === id);
      roles.value = [...roles.value.slice(0, at + 1), split, ...roles.value.slice(at + 1)];
      withRow(id, (one) => ({ ...one, [o]: null }));
    }
  }
  setYchecks(l, newName, undefined);
  patch({ ok: { ...state.value.ok, [key]: 1 } });
  await askCheck(byUid(id), l, null);
  touch();
  // A rename changes the name the address opens the row by, so the address
  // follows the save (spec-roles 2 節).
  syncURL(false);
  apiSaved();
}

export async function doDelete(id, l) {
  const r = byUid(id);
  const e = r && r[l];
  if (!e || !e.saved) return;
  try {
    await apiDel('role', e.saved.name, l, e.hash || '', undefined, r.group && r.group !== 'global' ? r.group : undefined);
  } catch (err) {
    showRefusal(id + l, err && err.json && err.json.at, problemText(err, '削除できませんでした'));
    return;
  }
  setYchecks(l, e.saved.name, undefined);
  withRow(id, (one) => ({ ...one, [l]: null }));
  const after = byUid(id);
  if (!after) return;
  if (!after.project && !after.global) {
    roles.value = roles.value.filter((x) => x.uid !== Number(id));
    const open = new Set(state.value.open);
    open.delete(Number(id));
    patch({ open, tab: 'roles' });
  } else {
    patch({ tab: 'roles' });
  }
  touch();
  syncURL(false);
}

export async function doAssignSave(id) {
  const t = teams.value.find((x) => x.id === id);
  if (!t) return;
  const changes = t.members.filter((m) => m.draft !== m.saved).map((m) => ({ member: m.name, role: m.draft }));
  if (!changes.length) return;
  // The pick travels as the team kind's own op, so the save writes one line of
  // team.yaml and no other (spec-boundary 6, spec-teams 1.3). The place is the
  // card's own: a global definition is saved in the global file, and every
  // project reading it changes (d-0f32, r-f8b9).
  let res;
  try {
    res = await apiSave('team', id, { place: t.place || 'project', part: 'team', hash: t.hash || '', doc: { ops: changes.map((m) => ({ kind: 'memberRole', member: m.member, role: m.role })) } });
  } catch (err) {
    patch({ teamErrors: { ...state.value.teamErrors, [id]: problemText(err, '保存できませんでした') } });
    touch();
    return;
  }
  const json = res.json || {};
  const nextErrors = { ...state.value.teamErrors };
  delete nextErrors[id];
  patch({ teamErrors: nextErrors, ok: { ...state.value.ok, ['t' + id]: 1 } });
  teams.value = teams.value.map((one) => (one.id !== id ? one : {
    ...one,
    hash: json.hash || one.hash,
    members: one.members.map((m) => ({ ...m, saved: m.draft })),
  }));
  for (const r of savedRoles()) await askCheck(r, eff(r), null);
  touch();
  apiSaved();
}

// ---- 帯への報告と持ち越し (spec-nav.md 1.4, 1.5) ---------------------
// dirty is whether the screen holds something no save wrote yet: a row's draft,
// a row a person added, a pick in the assign tab, or a YAML text.
function rowDirty(r) {
  return PLACES.some((l) => r[l] && (!r[l].saved || JSON.stringify(r[l].draft) !== JSON.stringify(r[l].saved)));
}

export function dirty() {
  const rows = roles.value.some(rowDirty);
  const members = teams.value.some((t) => t.members.some((m) => m.draft !== m.saved));
  const ytexts = allEntries().some(({ r, l }) => r[l].ytext !== undefined && r[l].ytext !== (r[l].text || ''));
  return rows || members || ytexts;
}

export function tellDirty() {
  setDirty(dirty());
}

// take is the draft the band puts aside: every row's draft that is not what the
// file holds, every row a person added, every pick that was made, every YAML
// text that was typed — each with the hash of the file it was read from, so a
// file that moved outside is not written over by a page that read another one.
export function take() {
  const rows = [], ytexts = [], members = [];
  roles.value.forEach((r) => {
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
  teams.value.forEach((t) => t.members.forEach((m) => {
    if (m.draft !== m.saved) members.push({ team: t.id, hash: t.hash || '', name: m.name, saved: m.saved, draft: m.draft });
  }));
  return { rows, ytexts, members };
}

// restore puts the carried draft back, skipping every part whose file moved
// outside since the draft was made (spec-nav.md 1.5).
export function restore(carried) {
  (carried.rows || []).forEach((c) => {
    const r = c.saved
      ? roles.value.find((x) => x[c.place] && x[c.place].saved && x[c.place].saved.name === c.saved)
      : roles.value.find((x) => x[c.place] && !x[c.place].saved && (x.noDoc || x.isNew || x[c.place].draft.name === c.name));
    if (!r || !r[c.place]) return;
    const e = r[c.place];
    if (c.hash && e.hash !== c.hash) return;
    observeKeys(rulesOf(c.draft));
    withEntry(r.uid, c.place, (one) => ({ ...one, draft: clone(c.draft) }));
    if (c.isNew) withRow(r.uid, (one) => ({ ...one, isNew: true }));
    clearTimer(r, c.place);
    askCheck(byUid(r.uid), c.place, null);
  });
  (carried.ytexts || []).forEach((c) => {
    const r = roles.value.find((x) => x[c.place] && (x[c.place].saved || x[c.place].draft).name === c.name);
    if (!r) return;
    const e = r[c.place];
    if (c.hash && e.hash !== c.hash) return;
    withEntry(r.uid, c.place, (one) => ({ ...one, ytext: c.text }));
  });
  (carried.members || []).forEach((c) => {
    const t = teams.value.find((x) => x.id === c.team);
    if (!t || (c.hash && t.hash !== c.hash)) return;
    const at = t.members.findIndex((x) => x.name === c.name && x.saved === c.saved);
    if (at >= 0) {
      teams.value = teams.value.map((one) => (one.id !== t.id ? one : { ...one, members: one.members.map((m, i) => (i === at ? { ...m, draft: c.draft } : m)) }));
    }
  });
  touch();
}

// boot reads the address, asks every saved document's check, and keeps what the
// band carried from the screen before this one (spec-nav.md 1.5).
export async function boot() {
  parseURL();
  await Promise.all(roles.value.flatMap((r) => PLACES.filter((l) => r[l] && r[l].saved).map((l) => askCheck(r, l, () => { updateRowSummary(r.uid); }))));
  roles.value.forEach((r) => updateRowSummary(r.uid));
  const carried = takeDraft('roles');
  if (carried) restore(carried);
  touch();
}
