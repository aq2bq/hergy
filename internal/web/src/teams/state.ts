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
import { signal, computed } from '../lib/preact.js';
import { resource } from '../lib/resource';
import { saved as apiSaved, list as apiList, check as apiCheck, save as apiSave, del as apiDel, hqList, problemText as apiProblemText, bases } from '../lib/api';
import type { Answer, ApiAnswer, Request, Schemas } from '../lib/api';
import { conflictOf } from '../lib/conflict.js';
import type { Conflict } from '../lib/conflict.js';

// CheckAnswer is what POST /api/team/check answers with, read from the operation
// (the saved definition's answer, or the one for a definition not there yet).
export type CheckAnswer = ApiAnswer<'checkTeam'>;

// TeamSaveBody and TeamCheckBody are what a save and a check take, read from
// the operations: the screen builds one and the port names its type.
type TeamSaveBody = Request<'team', 'save'>;
type TeamCheckBody = Request<'team', 'check'>;
import {
  fromJSON, toDoc, rulesOf, diffCount, clone, openRules, openRuleKey, observeKeys, openRule,
} from '../lib/ruledoc.js';
import { parseOpen } from '../lib/hq.js';
import { copyGroupOf, groupPlace } from '../lib/copy';
import { takeDraft, setDirty } from '../lib/nav.js';
import { place } from './place';
import type { Positions } from './place';
import type { CanvasMenu, CanvasTyped } from './canvas/shared';
import type { Reading } from '../lib/resource';

// ---- 画面が読む形 -------------------------------------------------------
// Member, Worker and Team are one definition as a row draws and saves it: the
// same fields the team.yaml holds (api-types' TeamDocument answers them, and
// cloneTeam makes this from it).
export type Member = { name: string; role: string; profile: string };
export type Worker = { from: string; purpose: string; profile: string };
export type Team = { mission: string; members: Member[]; workers: Worker[]; turns: [string, string][] };

// Op is one edit the row holds until it is saved: the kinds applyOps knows, and
// the words each kind carries (the API's TeamOperation takes the same fields).
export type Op = { kind: string } & Record<string, string>;

// HQIsland is the headquarters reading the page carries: the projects, and the
// groups of definitions, roles and profiles (d-73fc).
export type HQIsland = {
  projects?: Schemas['HQProjects'] | null;
  teams?: Schemas['HQTeamGroup'][] | null;
  roles?: Schemas['HQRoleGroup'][] | null;
  profiles?: Schemas['HQProfileGroup'][] | null;
  /** Why the headquarters' lists could not be read; a screen says it. */
  error?: string;
};

// Island is the JSON the page carries (teams_v2.go's t2Island).
type Island = {
  dirs?: Record<string, string>;
  profiles?: string[];
  hq?: HQIsland | null;
  team?: Answer<'team', 'list'>;
};

export const view = document.getElementById('view');
export const tabsEl = document.getElementById('tabs');

// Boundary: the page's embedded JSON is read here and nowhere else.
const DATA: Island = JSON.parse((document.getElementById('teams-data') || {}).textContent || '{}') as Island;
export type Place = 'project' | 'global';
export const PLACES: readonly Place[] = ['project', 'global'];
export const PLACE: Record<Place, string> = { project: 'プロジェクト', global: 'グローバル' };
export const DIR = DATA.dirs || { project: '', global: '' };
export const PROFILES: string[] = DATA.profiles || [];
// HQ is the headquarters lists (d-73fc): the projects' groups of definitions,
// roles and profiles. Without it (a server with no headquarters reading) the
// screen is the one project's, as it always was.
export const HQ: HQIsland | null = DATA.hq || null;
// GROUPED is the headquarters with projects: listed: the rows stand in groups
// and carry their project. Without projects: the screen is the one project
// the way it always was (d-73fc: the old use is not broken).
export const GROUPED = !!(HQ && HQ.projects && HQ.projects.mode === 'listed');
export const GROUPS: Schemas['HQTeamGroup'][] | null = GROUPED && HQ ? (HQ.teams || []) : null;

// TeamGroup is one place the list draws a head for (r-410c, d-e77c): the
// project's own place (its key is "" without projects: listed), the global
// layer's, or another listed project's. The one-project headquarters' groups
// are read with the same shape.
export type TeamGroup = { key: string; global: boolean; name: string; display: string; count: number };
export const placeGroups = (): TeamGroup[] => {
  if (HQ && HQ.teams && HQ.teams.length) {
    return HQ.teams.map((g) => {
      const key = GROUPED ? groupKeyOf(g) : (g.global ? 'global' : '');
      return { key, global: !!g.global, name: groupNameOf(g), display: g.display || '', count: 0 };
    });
  }
  return [
    { key: '', global: false, name: currentProjectName() || 'プロジェクト', display: DIR.project, count: 0 },
    { key: 'global', global: true, name: 'グローバル', display: DIR.global, count: 0 },
  ];
};
// currentGroup is the key of the project this serve was started in: a row the
// screen adds goes there, and a row without a group is its own.
export const CURRENT = HQ && HQ.projects && HQ.projects.current ? (HQ.projects.current.path || '') : '';
export const rowsIn = (g: Schemas['HQTeamGroup']): Schemas['TeamItem'][] => (g.list && g.list.items) || [];
export const groupKeyOf = (g: Schemas['HQTeamGroup'] | Schemas['HQRoleGroup'] | Schemas['HQProfileGroup']): string => (g.global ? 'global' : (g.project ? g.project.path : ''));
export const groupNameOf = (g: Schemas['HQTeamGroup'] | Schemas['HQRoleGroup'] | Schemas['HQProfileGroup']): string => (g.global ? 'グローバル' : (g.project ? (g.project.name || '') : ''));
if (HQ) document.body.classList.add('hq-teams');

// ROLES_BY and PROFILES_BY are the names each project's member 属性 offers:
// another project's definition picks from that project's own documents and
// the global layer's (spec 4 章). They are filled after the first frame, and
// ROLES_REV is the number that says they changed, so the columns that read
// them are drawn again without the screen asking.
export type RoleName = { name: string; title: string };
const ROLES_BY: Record<string, RoleName[]> = {};
const PROFILES_BY: Record<string, string[]> = {};
const ROLES_REV = signal(0);
if (HQ) {
  (HQ.roles || []).forEach((g) => {
    ROLES_BY[groupKeyOf(g)] = ((g.list && g.list.items) || []).map((item): RoleName => {
      const places = item.places || {}, eff = item.effective || (places.project ? 'project' : 'global');
      const doc = places[eff]?.doc;
      return { name: item.name || '', title: (doc && doc.title) || '' };
    }).filter((x) => !!x.name);
  });
  (HQ.profiles || []).forEach((g) => { PROFILES_BY[groupKeyOf(g)] = ((g.list && g.list.items) || []).map((item) => item.name).filter(Boolean); });
}
// ROLES is the role documents the member's 属性 offers: the same list the
// role screen shows, with the title each document carries. It is read once
// from GET /api/role; the team answer itself carries no role documents.
const ROLES = signal<RoleName[]>([]);
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

export const $ = <T extends Element = HTMLElement>(s: string, root: ParentNode = document): T | null => root.querySelector<T>(s);
export const nodeName = (x: string): string => (x === 'human' ? '人間' : x);
export const other = (p: Place): Place => (p === 'project' ? 'global' : 'project');
const hasTurn = (t: Team, a: string, b: string): boolean => (t.turns || []).some(([p, q]) => p === a && q === b);
const memberOf = (t: Team, n: string): Member | undefined => (t.members || []).find((m) => m.name === n);
export const narrow = () => window.innerWidth <= 760;
// thin is narrow as the figure reads it: a page that grows or shrinks draws the
// figure again through this (main.js puts the width in).
export const thin = signal(narrow());

// ---- 画面の状態 ---------------------------------------------------------
// state is what the screen knows about itself: the tab, the open rows, the
// files' own drafts and the notices. Every change puts a new value in (依頼書 1),
// so nothing here is written in place.
// State is what the screen knows about itself. Every map is keyed by the row or
// the file it is about.
export type State = {
  tab: 'list' | 'yaml';
  open: Set<string>;
  dtab: Record<string, string>;
  confirm: Record<string, number>;
  ok: Record<string, number>;
  yfile: string | null;
  ytext: Record<string, string | undefined>;
  yc: Record<string, Schemas['Problem'][] | undefined>;
  pmode: Record<string, string>;
  wantRule: { name: string; k: number } | null;
  refusal: Record<string, string>;
  // conflicts stand where a save was refused because the file moved: the file as
  // the answer showed it, and the row as it stood before 元に戻す (r-3c77).
  conflicts: Record<string, ConflictHold>;
  toast: { msg: string; undo: boolean } | null;
};
// ConflictHold is one refused save: the answer's file, whether the person has
// read the file's own text in (undone), and the row to put back with 取り消す.
type RowBefore = { saved: Team; ops: Op[]; agreeSaved: RuleDoc | null; agreeDraft: RuleDoc; hash: string; agreeHash: string };
type ConflictHold = { c: Conflict; undone?: boolean; before?: RowBefore };
const put = (patch: Partial<State>) => { state.value = { ...state.value, ...patch }; };
export const state = signal<State>({
  tab: (view && view.dataset.tab) === 'yaml' ? 'yaml' : 'list',
  open: new Set(), dtab: {}, confirm: {},
  ok: {}, yfile: null, ytext: {}, yc: {}, pmode: {}, wantRule: null,
  // refusal is the server's sentence on one row's foot (the shape tab's or the
  // rules tab's), and toast the words the canvas says at the bottom.
  refusal: {}, conflicts: {}, toast: null,
});
// focusAsk is the one field a person's press should land in after the draw
// (5 章 5): a member just added, a rule just opened. The row it names puts the
// cursor there itself.
export const focusAsk = signal<{ dom: string; sel: string } | null>(null);
// The screen's maps are written by the setter of the map they belong to: each
// copies the map it is about, puts the entry in, and keeps the entry's own type.
function setOk(key: string, on: boolean): void {
  const m = { ...state.value.ok };
  if (on) m[key] = 1; else delete m[key];
  put({ ok: m });
}
function setRefusal(key: string, text: string): void {
  const m = { ...state.value.refusal };
  if (text) m[key] = text; else delete m[key];
  put({ refusal: m });
}
function setYtext(key: string, text: string | undefined): void {
  const m = { ...state.value.ytext };
  if (text === undefined) delete m[key]; else m[key] = text;
  put({ ytext: m });
}
function setYc(key: string, list: Schemas['Problem'][] | undefined): void {
  const m = { ...state.value.yc };
  if (list === undefined) delete m[key]; else m[key] = list;
  put({ yc: m });
}
const withOpen = (dom: string, on: boolean) => {
  const s = new Set(state.value.open);
  if (on) s.add(dom); else s.delete(dom);
  put({ open: s });
};
export const isOpen = (dom: string): boolean => state.value.open.has(dom);
export const dtabOf = (r: Rec): string => state.value.dtab[r.dom] || 'shape';
export const pmodeOf = (r: Rec): string => state.value.pmode[dkey(r)] || 'edit';
export const okOf = (key: string): boolean => !!state.value.ok[key];
export const refusalOf = (r: Rec, which: string): string => state.value.refusal[which + dkey(r)] || '';
export const conflictAt = (r: Rec, which: string): ConflictHold | null => state.value.conflicts[which + dkey(r)] || null;
function setConflict(r: Rec, which: string, hold: ConflictHold | null): void {
  const m = { ...state.value.conflicts };
  const key = which + dkey(r);
  if (hold) m[key] = hold; else delete m[key];
  put({ conflicts: m });
}
const setDtab = (dom: string, t: string) => { put({ dtab: { ...state.value.dtab, [dom]: t } }); };
const dropDtab = (dom: string) => { const m = { ...state.value.dtab }; delete m[dom]; put({ dtab: m }); };
const setConfirm = (key: string, on: boolean) => { const m: Record<string, number> = { ...state.value.confirm }; if (on) m[key] = 1; else delete m[key]; put({ confirm: m }); };
const debounceTimers: Record<string, ReturnType<typeof setTimeout> | undefined> = {};
export const debounce = (k: string, fn: () => void, ms = 260) => { clearTimeout(debounceTimers[k]); debounceTimers[k] = setTimeout(fn, ms); };

// ---- 記録の形 -----------------------------------------------------------
// Issue is one problem as the rules panel draws it: the rule's own key (0 when
// the finding is not about a rule), the field it stands on, and the sentence.
export type Issue = { sev: string; k: number; f: string; text: string; definition: string };
// Finding is the figure's problem: the node, the worker or the field it stands
// beside, and the server's sentence (the role and rule it names, when it does).
export type Finding = { sev: string; field: string; node: string | null; worker: number | null; text: string; role: string; ruleID: string; ruleKey?: number };
export type CheckDiff = Schemas['TeamDiff'];
export type CheckTurn = Schemas['TeamRuleTurn'];
// TeamPreview and RuleMarkdown are the check's two previews, as checkTeam answers
// them: the team.yaml's lines and the diff (TeamPreview), and the agreement's
// Markdown with the section and rule each line stands for (TeamRuleMarkdown).
// They are read from the operation, not written here (n-e3de).
export type TeamPreview = Schemas['TeamPreview'];
export type RuleMarkdown = Schemas['TeamRuleMarkdown'];
export type Check = { issues: Issue[]; markdown: RuleMarkdown; diff: CheckDiff; turns: CheckTurn[]; readers: Schemas['TeamRuleReader'][] };
// Pick is what the canvas has picked: a point of the figure, a worker, or a
// line between two points.
export type CanvasPick = { type: 'node'; id: string } | { type: 'worker'; i: number } | { type: 'edge'; k: string };
export type WorkerPrompt = { segments: PromptSegment[]; name: string; task: string; err?: string };
export type PromptSegment = Schemas['TeamPromptSegment'];
export type MemberPrompt = { name: string; segments?: PromptSegment[] | null };
// RuleDoc is the rules document the panel edits (lib/ruledoc makes it from the
// server's answer): the sections, their rules, and the path it came from.
export type Rule = { id?: string; _k: number; text?: string; note?: string; in_prompt?: boolean };
export type RuleSection = { heading?: string; level?: number; intro?: string; rules: Rule[] };
export type RuleDoc = { name?: string; title?: string; revision?: number | null; sections: RuleSection[]; __path?: string };

// Sig is one field of a record: a signal, so a part that reads it draws again
// when it changes (依頼文 1).
type Sig<T> = { value: T };

// Rec is one definition the screen holds: the fields it was made with (uid,
// dom, project, places, draftRow, how, base), the fields that change (each a
// signal of its own), and the two the screen computes: the draft (saved + ops)
// and its key.
export type Rec = {
  uid: number; dom: string; project: string; places: Place[];
  draftRow: boolean; how: string; base: TeamBase | null;
  name: Sig<string>; place: Sig<Place>;
  teamPath: Sig<string>; agreePath: Sig<string>;
  hash: Sig<string>; agreeHash: Sig<string>; broken: Sig<boolean>;
  hidden: Sig<{ place: string; path?: string; hash?: string }[]>;
  ops: Sig<Op[]>; saved: Sig<Team>;
  agreeSaved: Sig<RuleDoc | null>; agreeDraft: Sig<RuleDoc>; agreeSavedIssues: Sig<Issue[] | null>;
  findings: Sig<Finding[]>; problems: Sig<Schemas['TeamCounts']>; check: Sig<Check | null>;
  size: Sig<number>; limit: Sig<number>;
  drawMark: Sig<number>; seeded: Sig<boolean>;
  writes: Sig<string[]>; nameIssues: Sig<Schemas['Problem'][]>;
  hasRoles: Sig<boolean>; hasProfiles: Sig<boolean>;
  starterRoles: Sig<boolean>; starterProfile: Sig<boolean>;
  preview: Sig<TeamPreview>; baseFindings: Sig<Finding[]>;
  prompts: Sig<MemberPrompt[]>; promptsReady: Sig<boolean | undefined>;
  sel: Sig<CanvasPick | null>; side: Sig<string>; menu: Sig<CanvasMenu | null>; typed: Sig<CanvasTyped | null>; flash: Sig<string | null>;
  lastAct: Sig<number>;
  wprompts: Sig<Record<string, WorkerPrompt>>; wpromptReq: Record<string, boolean>;
  placed: Sig<Positions>; layout: Sig<Positions>;
  // wide is the figure laid out in rows rather than round the human: a ring too
  // tall for the screen is drawn this way (r-e39c). moved is the points a person
  // put down by hand, so a new shape leaves them where they are (not a signal:
  // it is read while the places are laid out).
  wide: Sig<boolean>; moved: Set<string>;
  draft: Sig<Team>; key: Sig<string>;
};
// TeamBase is the definition a duplicate or a copy stands on.
export type TeamBase = { place: Place; name: string };

// dkey is the key the screen's own state and DOM stand on: the definition's
// saved name, or, for a definition that is not there yet (its name is what the
// person is typing), a stable id of its own (spec-teams B2).
export const dkey = (d: Rec): string => d.dom + '.' + d.place.value;
export const domOf = (name: string, group?: string): string => (GROUPED ? name + '|' + (group || '') : name);

// ---- サーバとのやり取りの形 ---------------------------------------------
// A definition that is not there yet carries how it was made: the starter's own
// files (new), or the definition it stands on (base). A row made from the
// scaffold also carries whether the person asked for the starter roles and
// profile: the server writes them only when asked (r-54b8). The server reads the
// base from there and checks the row as if it were saved (spec-teams B2, B3).
const draftIn = (r: Rec): { new?: boolean; starter_roles?: boolean; starter_profile?: boolean; base?: TeamBase | null } => (!r.draftRow ? {} : r.how === 'new' ? { new: true, starter_roles: r.starterRoles.value, starter_profile: r.starterProfile.value } : { base: r.base });
// projectIn is the project a write names: a listed project's path, and nothing
// for the global layer (which the server writes without being told) or for this
// serve's own project.
const projectIn = (project: string): { project?: string } => (project && project !== 'global' ? { project } : {});
const teamIn = (r: Rec, extra: Partial<TeamSaveBody> = {}): TeamSaveBody => ({ place: r.place.value, name: r.name.value, part: 'team', ...projectIn(r.project), ...draftIn(r), ...extra });
const agreeIn = (r: Rec, extra: Partial<TeamSaveBody> = {}): TeamSaveBody => ({ place: r.place.value, name: r.name.value, part: 'agreement', ...projectIn(r.project), ...draftIn(r), ...extra });

// ---- 下書き（姿） ------------------------------------------------------
// TeamLike is a definition as either entrance answers it: a saved document
// (the project's own or the headquarters'), or the row's own draft.
type TeamLike = {
  mission?: string;
  members?: { name: string; role: string; profile: string }[] | null;
  workers?: { from: string; purpose: string; profile: string }[] | null;
  turns?: ([string, string] | { from: string; to: string })[] | null;
};
export const cloneTeam = (t: TeamLike | null | undefined): Team => ({
  mission: (t && t.mission) || '',
  members: ((t && t.members) || []).map((m) => ({ name: m.name, role: m.role, profile: m.profile })),
  workers: ((t && t.workers) || []).map((w) => ({ from: w.from, purpose: w.purpose, profile: w.profile })),
  turns: ((t && t.turns) || []).map((x): [string, string] => ('from' in x ? [x.from, x.to] : [x[0], x[1]])),
});
export function applyOps(saved: Team, ops: Op[]): Team {
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

// emptyAgreement is the rules document a team without one opens with: no
// sections, and the title its first save would write. A row with no
// agreement.yaml keeps this as its draft, so 元に戻す has a document to go back
// to and does not put null where a document belongs (n-e3de).
export function emptyAgreement(name: string): RuleDoc {
  return fromJSON({
    name: '', title: name ? `${name} チームのルール` : 'チームのルール', revision: 1, sections: [],
  });
}

// ---- 記録（1 定義ごと） ------------------------------------------------
// RecInit is a record as it is made: every field a value, the fields that never
// change and the ones recOf turns into signals of their own.
type RecInit = {
  uid?: number; dom: string; project: string; places?: Place[];
  draftRow: boolean; how: string; base: TeamBase | null;
  name: string; place: Place; teamPath: string; agreePath: string;
  hash: string; agreeHash: string; broken: boolean;
  hidden: { place: string; path?: string; hash?: string }[];
  ops: Op[]; saved: Team;
  agreeSaved: RuleDoc | null; agreeDraft: RuleDoc; agreeSavedIssues: Issue[] | null;
  findings: Finding[]; problems: Schemas['TeamCounts']; check: Check | null;
  size: number; limit: number; drawMark: number; seeded: boolean;
  writes: string[]; nameIssues: Schemas['Problem'][];
  hasRoles: boolean; hasProfiles: boolean; starterRoles: boolean; starterProfile: boolean;
  preview: TeamPreview; baseFindings: Finding[];
  prompts: MemberPrompt[]; promptsReady?: boolean;
  sel: CanvasPick | null; side: string; menu: CanvasMenu | null; typed: CanvasTyped | null; flash: string | null;
  lastAct: number;
};
// recOf puts one record together: every field is written where it stands, so a
// field the type gains is one the compiler asks for here (依頼書 1). draft and
// key are computed from the record itself, so they are made last.
function recOf(init: RecInit): Rec {
  const r: Rec = {
    // the fields a record is made with, read as they are
    uid: init.uid || 0, dom: init.dom, project: init.project, places: init.places || [],
    draftRow: init.draftRow, how: init.how, base: init.base,
    // the fields a change moves: one signal apiece
    name: signal(init.name), place: signal(init.place),
    teamPath: signal(init.teamPath), agreePath: signal(init.agreePath),
    hash: signal(init.hash), agreeHash: signal(init.agreeHash), broken: signal(init.broken),
    hidden: signal(init.hidden),
    ops: signal(init.ops), saved: signal(init.saved),
    agreeSaved: signal(init.agreeSaved), agreeDraft: signal(init.agreeDraft), agreeSavedIssues: signal(init.agreeSavedIssues),
    findings: signal(init.findings), problems: signal(init.problems), check: signal(init.check),
    size: signal(init.size), limit: signal(init.limit), drawMark: signal(init.drawMark), seeded: signal(init.seeded),
    writes: signal(init.writes), nameIssues: signal(init.nameIssues),
    hasRoles: signal(init.hasRoles), hasProfiles: signal(init.hasProfiles),
    starterRoles: signal(init.starterRoles), starterProfile: signal(init.starterProfile),
    preview: signal(init.preview), baseFindings: signal(init.baseFindings),
    prompts: signal(init.prompts), promptsReady: signal(init.promptsReady),
    sel: signal(init.sel), side: signal(init.side), menu: signal(init.menu), typed: signal(init.typed), flash: signal(init.flash),
    lastAct: signal(init.lastAct),
    // wprompts is the worker prompts the server answered, per worker; the request
    // that is out is the drawing's own bookkeeping and is not a signal.
    wprompts: signal<Record<string, WorkerPrompt>>({}), wpromptReq: {},
    // placed is where a person put a point down (a double press's menu, or a drag
    // that was let go), and layout is where the figure draws every point: the
    // places above and an automatic place for the rest. The drawing reads layout
    // and writes nothing (webui-components 1 章).
    placed: signal<Positions>({}),
    wide: signal(false), moved: new Set<string>(),
    draft: computed(() => applyOps(r.saved.value, r.ops.value)),
    key: computed(() => dkey(r)),
    layout: computed(() => place(r.draft.value, r.placed.value, thin.value, r.wide.value)),
  };
  return r;
}
// seedPlaces puts every point of the draft into the record's places, once: when
// the row is made, and after each change of the draft. The places are the
// screen's own state, decided by the operation that changes the draft, so the
// figure draws what stands there and writes nothing, and a point that was laid
// out does not move when another is added (webui-components 1 章).
export function seedPlaces(r: Rec): void {
  r.placed.value = place(r.draft.value, r.placed.value, thin.value, r.wide.value);
}
// setWide lays the figure out in rows, for a ring too tall to draw at the
// smallest size (r-e39c): every automatic place is taken again in that shape,
// and the points a person put down stay where they are. The shape stays while
// the row is open, so a window dragged near the edge does not swing between the
// two shapes.
export function setWide(r: Rec): void {
  if (r.wide.value) return;
  r.wide.value = true;
  const kept: Positions = {};
  r.moved.forEach((id) => { const q = r.placed.value[id]; if (q) kept[id] = [q[0], q[1]]; });
  r.placed.value = place(r.draft.value, kept, thin.value, true);
}
export const recs = signal<Rec[]>([]);
let uids = 0;

// defOf is one item of GET /api/team (or of one group of GET /api/hq/teams)
// read as the record the screen draws: the definition as one place reads it,
// the team's own agreement document, and the number of problems the server
// counted for the row (api.js lays the list out, so this screen reads no
// answer field by hand). project is the group the row stands in: a listed
// project's path, "global", or empty for the one this serve was started in.
// DefItem is one row of either team list: the project's own (GET /api/team) or
// one group of the headquarters' (GET /api/hq/teams).
export type DefItem = NonNullable<Answer<'team', 'list'>['items']>[number];

// NO_AGREEMENT is the agreement a definition without one is read as: no file,
// no hash, no document, and no rule (n-e3de).
const NO_AGREEMENT: Schemas['TeamAgreement'] = { path: '', hash: '', has: false, rules: 0 };

function defOf(item: DefItem, project?: string, only?: Place): Rec {
  const group = project || '';
  const places = item.places || {};
  // only keeps one side of a name: a list draws a head for each place, so a
  // definition that stands in both is two rows (r-410c).
  const eff: Place = only || item.effective || (places.project ? 'project' : 'global');
  const place = places[eff];
  const doc = place?.doc;
  const agree = doc?.agreement || NO_AGREEMENT;
  const sum = item.summary;
  const dom = domOf(item.name || '', group);
  const had = byKey(dom + '.' + eff);
  if (had) return had;
  let saved: RuleDoc | null = null;
  if (agree.doc) {
    saved = fromJSON(agree.doc);
    saved.__path = agree.path;
  }
  const hidden = (only ? [] : (item.hidden || [])).filter((p) => places[p]).map((p) => ({
    place: p, path: places[p]?.path, hash: places[p]?.hash,
  }));
  const r = recOf({
    name: item.name || '', dom, project: group, place: eff, places: only ? [only] : PLACES.filter((p) => places[p]),
    teamPath: place?.path || '', agreePath: agree.path || '',
    hash: place?.hash || '', agreeHash: agree.hash || '', broken: !!doc?.broken,
    hidden, ops: [],
    saved: cloneTeam(doc),
    agreeSaved: saved,
    agreeDraft: saved ? clone(saved) : emptyAgreement(item.name || ''),
    // The saved document's own problems are the agreement save's baseline:
    // a save stops only when an error the saved text did not hold has
    // appeared (spec-teams X3).
    agreeSavedIssues: null,
    findings: [],
    // The list's own size and budget, the same pair the figure's check
    // answers (d-00a9, r-2f83). A file that does not read has no size and
    // draws 大きさ —.
    size: (sum && sum.size) || 0, limit: (sum && sum.limit) || 0,
    problems: item.problems || { err: 0, warn: 0 }, check: null, drawMark: 0,
    draftRow: false, how: '', base: null, seeded: true, writes: [], nameIssues: [],
    hasRoles: false, hasProfiles: false, starterRoles: false, starterProfile: false,
    preview: { lines: [], diff: { added: [], removedBefore: [], removedAtEnd: false } }, baseFindings: [], prompts: [], promptsReady: undefined,
    // The canvas's own picks for this row (webui-components 3 章): what is
    // picked, which column stands on the right, the blank space's menu, the
    // member name being typed, and the line that just lit up.
    sel: null, side: 'attr', menu: null, typed: null, flash: null,
    lastAct: 0,
  });
  r.uid = ++uids;
  seedPlaces(r);
  recs.value = [...recs.value, r];
  return r;
}
export const allRecs = () => recs.value;
export const byKey = (k: string): Rec | null => recs.value.find((r: Rec) => r.key.value === k) || null;
export const byName = (n: string): Rec | null => recs.value.find((r: Rec) => r.dom === n) || null;
// byRef is the record one address names: the row of one group, or, with no
// group, the row the screen wrote before the headquarters (its name alone).
const byRef = (name: string, group?: string): Rec | null => (group ? byName(domOf(name, group)) : (byName(name) || byRealName(name)));
// byRealName is the first row of a name, the project's own first: a link the
// older screens wrote names a definition, not a group.
const byRealName = (name: string): Rec | null => recs.value.find((r: Rec) => r.name.value === name && (r.project === CURRENT || r.project === ''))
  || recs.value.find((r: Rec) => r.name.value === name) || null;
// groupOfRecord is the group a writable row belongs to: its own project, the
// global layer, or, for a row that is not there yet, this serve's project.
// Without projects: the screen writes to the one project, as it always did.
export const groupOfRecord = (r: Rec) => (r.project || (r.place.value === 'global' ? 'global' : (GROUPED ? CURRENT : '')));
// The list is drawn in the order the records stand in, so a row that is added
// or read again is put back where a person was looking: firstIndexOf is the
// place of the first record one rule names, and orderRows puts one row there
// (spec-teams B1-B6).
export const firstIndexOf = (want: (r: Rec) => boolean): number => recs.value.findIndex((x) => want(x));
export function orderRows(r: Rec, at: number): void {
  const all = recs.value.filter((x) => x !== r);
  all.splice(Math.max(0, Math.min(at < 0 ? 0 : at, all.length)), 0, r);
  recs.value = all;
}
export const rowsOfGroup = (key: string) => recs.value.filter((r: Rec) => groupOfRecord(r) === key);

// ---- まだ無い定義（B2, B3） --------------------------------------------
// A definition that is not there yet is one row the screen holds rather than a
// saved file: the name the person is typing, the place it would go to, the
// definition it stands on, and what the server answered about it. The screen
// builds no document of its own; it draws the check's own answer (spec-teams
// B2, spec-boundary 5).
function draftRecord(how: string, place: Place, name: string, base: TeamBase | null | undefined, project?: string): Rec {
  const r = recOf({
    dom: '~new' + (++uids), draftRow: true, how, base: base || null, project: project || '',
    name: name || '', place: place || 'project', places: [],
    teamPath: '', agreePath: '', hash: '', agreeHash: '', broken: false,
    hidden: [], ops: [],
    saved: cloneTeam(null),
    agreeSaved: null,
    agreeDraft: emptyAgreement(''),
    agreeSavedIssues: null,
    findings: [], problems: { err: 0, warn: 0 }, check: null,
    size: 0, limit: 0,
    drawMark: 0, seeded: false, writes: [], nameIssues: [],
    hasRoles: false, hasProfiles: false, starterRoles: false, starterProfile: false,
    preview: { lines: [], diff: { added: [], removedBefore: [], removedAtEnd: false } }, baseFindings: [], prompts: [], promptsReady: undefined,
    sel: null, side: 'attr', menu: null, typed: null, flash: null,
    lastAct: 0,
  });
  seedPlaces(r);
  recs.value = [...recs.value, r];
  return r;
}
// seedDraft reads the first check of a definition that is not there yet into
// the row: the base text its figure and its rules stand on, and where a save
// would write it (spec-teams B2, B3).
function seedDraft(r: Rec, json: CheckAnswer): void {
  // seedDraft is the answer for a definition that is not there yet: that branch
  // is the one that carries the document a save would write, in doc, and the
  // row it would make, in draft.
  const draft = json.draft, doc = json.doc;
  if (!draft || !doc) return;
  const agree = doc.agreement || NO_AGREEMENT;
  r.seeded.value = true;
  // The base the row stands on is a draft of its own: its points are laid out
  // afresh, not kept from the empty one the row was made with.
  r.placed.value = {};
  r.moved.clear();
  r.saved.value = cloneTeam(doc);
  r.broken.value = !!doc.broken;
  r.teamPath.value = draft.path || '';
  r.agreePath.value = agree.path || '';
  r.hash.value = draft.hash || '';
  r.agreeHash.value = agree.hash || '';
  if (agree.doc) {
    const d = fromJSON(agree.doc);
    observeKeys(rulesOf(d));
    d.__path = agree.path;
    r.agreeSaved.value = d;
    r.agreeDraft.value = clone(d);
  }
  seedPlaces(r);
}
// theName is the name a duplicate takes: the source's with -2, -3 … until one
// is free, so the row starts without the name problem the same name would raise
// (spec-teams B3).
export function theName(base: string): string {
  const taken = new Set(recs.value.map((r: Rec) => r.name.value));
  for (let i = 2; i < 1000; i++) {
    const n = `${base}-${i}`;
    if (!taken.has(n) && n.length <= 25) return n;
  }
  return base;
}
// groupStart is where a row added under one place's head stands: the first row
// of that head, or before the next head's rows when it holds none.
function groupStart(group: string): number {
  const order = placeGroups().map((g) => g.key);
  const rank = order.indexOf(group);
  const i = recs.value.findIndex((r) => groupOfRecord(r) === group);
  if (i >= 0) return i;
  const j = recs.value.findIndex((r) => order.indexOf(groupOfRecord(r)) > rank);
  return j >= 0 ? j : recs.value.length;
}

// addDraft makes a row for a definition that is not there yet, checks it once,
// and draws it: its figure, its rules and its preview are the server's own
// reading of the base (spec-teams B2).
export async function addDraft(how: string, place: Place, name: string, base: TeamBase | null, project?: string): Promise<Rec> {
  const r = draftRecord(how, place, name, base, project);
  await checkTeam(r);
  // The row stands where the mock draws it: one added from the scaffold stands
  // first, and a duplicate or a copy stands right under the row it came from
  // (spec-teams B1, B3, B4).
  // A row stands at the top of the head it was added under; a duplicate stands
  // right under the row it came from (r-410c, r-f781).
  let at = groupStart(project || (place === 'global' ? 'global' : ''));
  if (base && how !== 'copy') {
    const i = firstIndexOf((x) => x.name.value === base.name);
    at = i >= 0 ? i + 1 : at;
  }
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
export function dropDraft(r: Rec): void {
  recs.value = recs.value.filter((x) => x !== r);
  withOpen(r.dom, false);
  dropDtab(r.dom);
  setConfirm('del' + dkey(r), false);
  resetCanvas(r);
}

function ruleKeyAt(doc: RuleDoc, si: number, ri: number): number {
  const s = doc && doc.sections[si];
  if (!s) return 0;
  const x = s.rules[ri];
  return x ? x._k : 0;
}
const convProblems = (doc: RuleDoc, list: Schemas['Problem'][] | null | undefined): Issue[] => (list || []).map((p) => ({
  sev: p.sev, k: p.section >= 0 ? ruleKeyAt(doc, p.section, p.rule) : 0,
  f: p.field || '', text: p.text || '', definition: p.definition || '',
}));
const buildCheck = (doc: RuleDoc, json: CheckAnswer): Check => {
  // The saved definition's answer carries the effect and the readers; the answer
  // for one that is not there yet carries its document instead (api-types). The
  // agreement's effect is the one this panel draws: its diff and its turns.
  const effect = json.effect && 'turns' in json.effect ? json.effect : null;
  return {
    issues: convProblems(doc, json.problems || []),
    // The agreement's check answers the rule Markdown; the team part answers the
    // team.yaml preview instead, which is not this panel's.
    markdown: json.preview && !('diff' in json.preview) ? json.preview : { lines: [], chars: 0 },
    diff: (effect && effect.diff) || { added: [], removedBefore: [], removedAtEnd: false },
    turns: (effect && effect.turns) || [], readers: json.readers || [],
  };
};

// figFindings is the figure's problems as the panel draws them: the field the
// server names ("mission", "members[1].profile", "workers[0]", "turns[2]", a
// rule) becomes the node, the worker or the word a row stands by. The server
// writes every sentence; this only says where it goes (spec-teams X1).
function figFindings(r: Rec, problems: Schemas['Problem'][] | null | undefined): Finding[] {
  const t = r.draft.value;
  return (problems || []).map((p) => {
    const at = p.at || p.field || '';
    let node: string | null = null, worker: number | null = null, field = at, ruleKey = 0, m;
    if ((m = /^members\[(\d+)\]/.exec(at))) { const mem = t.members[Number(m[1])]; node = mem ? mem.name : null; field = 'members'; }
    else if ((m = /^workers\[(\d+)\]/.exec(at))) { worker = Number(m[1]); field = 'workers'; }
    // A sentence about a node ("node:analyst") stands beside that point: the
    // graph's own problems reach the card and the figure that way (監査 R9).
    else if (at.startsWith('node:')) { node = at.slice('node:'.length); field = 'node'; }
    else if (at === 'mission') { field = 'mission'; }
    else if (at === 'name') { field = 'name'; }
    else if (at.startsWith('rule:')) {
      field = at;
      const path = at.slice('rule:'.length).split('#')[0];
      if (p.section >= 0 && path.endsWith(`/teams/${r.name.value}/agreement.yaml`)) {
        ruleKey = ruleKeyAt(r.agreeDraft.value, p.section, p.rule);
      }
    }
    else { field = ''; }
    return { sev: p.sev, field, node, worker, text: p.text, role: p.role || '', ruleID: p.rule_id || '', ruleKey };
  });
}
export const agreeIssues = (r: Rec): Issue[] => (r.check.value ? r.check.value.issues : []);
export const agreeDiff = (r: Rec): number => diffCount({ saved: r.agreeSaved.value, draft: r.agreeDraft.value });
export const teamIssues = (r: Rec): Finding[] => r.findings.value;
const pcountList = (c: Schemas['TeamCounts'] | null | undefined): { sev: string }[] => [...Array((c && c.err) || 0)].map(() => ({ sev: 'err' })).concat([...Array((c && c.warn) || 0)].map(() => ({ sev: 'warn' })));
// rowIssues is the problems a row's own chip shows: the server's count for a
// saved definition, and the findings the server answered for a definition that
// is not there yet (spec-teams B2).
export const rowIssues = (r: Rec) => (r.draftRow ? teamIssues(r) : pcountList(r.problems.value));
// A definition that is not there yet counts as unsaved whatever it holds: it is
// written only when 保存 is pressed (spec-teams B2).
export const dirtyDef = (r: Rec) => r.draftRow || changeCount(r) > 0 || (r.agreeSaved.value ? agreeDiff(r) > 0 : false);

// workerOrigin is the worker of the saved file a draft worker stands in, read
// back through the ops that changed its purpose or profile, or null when the
// draft added it: it is how a purpose typed one letter at a time still counts
// as one worker (spec-teams 1.4.6).
function workerOrigin(w: Worker, ops: Op[]): Worker | null {
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
export function changeCount(r: Rec): number {
  const t = r.draft.value, s = r.saved.value, ops = r.ops.value || [];
  let n = t.mission !== s.mission ? 1 : 0;
  const key = (a: string, b: string): string => a + '\x00' + b;
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
  wasW.forEach((_sw, k) => { if (!claimed.has(k)) n++; });
  const wasM = new Map((s.members || []).map((m) => [m.name, m]));
  const nowM = new Map((t.members || []).map((m) => [m.name, m]));
  nowM.forEach((m, name) => {
    const sm = wasM.get(name);
    if (!sm) n++;
    else if (sm.role !== m.role || sm.profile !== m.profile) n++;
  });
  wasM.forEach((_m, name) => { if (!nowM.has(name)) n++; });
  return n;
}

// ownAgreement is whether a rule finding is about the team's own agreement:
// those are the team's promises and stand under チームのルール, while the rules of
// the role documents its members read stand under ロールの文書 (spec-teams 1.4.5,
// 1.7, 監査 R9).
const ownAgreement = (r: Rec, m: Finding): boolean => {
  const path = (m.field || '').replace(/^rule:/, '').split('#')[0];
  const name = r.name.value;
  return !!r.agreePath.value && path.endsWith('/teams/' + name + '/agreement.yaml');
};
const ruleID = (m: Finding): string => ((m.field || '').split('#')[1] || '');
export const vlistParts = (r: Rec) => {
  const issues = teamIssues(r);
  const at = (m: Finding): boolean => !!m.field && m.field.startsWith('rule:');
  const out = issues.filter((m) => at(m) && !ownAgreement(r, m));
  const teamOut = issues.filter((m) => at(m) && ownAgreement(r, m));
  const own = issues.filter((m) => !at(m));
  const purposeOf = (i: number): string => {
    const w = r.draft.value.workers[i];
    return (w && w.purpose) || 'ワーカー';
  };
  const where = (m: Finding): string => {
    if (m.node) return nodeName(m.node);
    if (m.worker != null && m.worker >= 0) return purposeOf(m.worker);
    if (m.field === 'mission') return '使命';
    if (m.field === 'name') return 'チーム定義名';
    return '';
  };
  const file = (m: Finding): string | undefined => (m.field.split('#')[0] || '').replace(/^rule:/, '').split('/').pop();
  // A finding about a role's own rule opens that role at the rule: the name
  // the server put on the finding (d-8bbf). A finding that names no role
  // still opens the roles screen, the way it did.
  const roleHref = (m: Finding): string => {
    if (!m.role) return '/roles';
    const q = `open=${encodeURIComponent(m.role)}`;
    return m.ruleID ? `/roles?${q}&rule=${encodeURIComponent(m.ruleID)}` : `/roles?${q}`;
  };
  return { issues, own, teamOut, out, where, file, roleHref, ruleID };
};

// ---- 検査と保存 --------------------------------------------------------
// A check is a reading like any other (5 章 3), so each goes through
// lib/resource.ts: the newest request is the one whose answer is kept and the
// one before it is dropped. A row keeps two of them, because one reading is not
// like the others: the check sent with no ops reads the saved file, and its
// findings are the baseline the save gate is held to (spec-teams X3). That
// reading is a request of its own, so a person who starts typing before it comes
// back still gets the file's own problems as the baseline.
type Checkers = {
  draft: Reading<CheckAnswer | null, [TeamCheckBody]>;
  base: Reading<CheckAnswer | null, [TeamCheckBody]>;
  agree: Reading<CheckAnswer | null, [TeamCheckBody]>;
};
const teamChecks = new Map<string, Checkers>();
function checkersFor(r: Rec): Checkers {
  const k = dkey(r);
  const had = teamChecks.get(k);
  if (had) return had;
  const load = async (signal: AbortSignal, payload: TeamCheckBody): Promise<CheckAnswer | null> => {
    try {
      const res = await apiCheck('team', payload, { signal });
      return res.json;
    } catch {
      // A refused draft answers nothing here: the screen draws no findings of its
      // own, and the refusal itself is the rules tab's to show (spec-teams X1).
      return null;
    }
  };
  const one: Checkers = { draft: resource(load), base: resource(load), agree: resource(load) };
  teamChecks.set(k, one);
  return one;
}
export function forgetChecks(r: Rec): void { teamChecks.delete(dkey(r)); }

// applyTeamAnswer draws one check's answer into the row.
function applyTeamAnswer(r: Rec, json: CheckAnswer, findings: Finding[]): void {
  r.findings.value = findings;
  // Each member's prompt, cut into the pieces it was built from (r-4cf9),
  // follows the ops the check was sent with: the 属性 column shows it without
  // building a line of its own. A definition with a problem is not given any,
  // and the column says why (d-20ac).
  r.prompts.value = Array.isArray(json.members) ? json.members : [];
  r.promptsReady.value = !!(json.members && json.members.length);
  // Both a saved definition and one that is not there yet get the topology
  // tab's own preview from the check (spec-teams B7).
  // The topology tab draws the team part's preview: the team.yaml's lines and the
  // diff. The agreement's answer carries the rule Markdown instead (n-e3de).
  const pv = json.preview && 'diff' in json.preview ? json.preview : r.preview.value;
  r.preview.value = pv;
  const draft = json.draft;
  if (r.draftRow && draft) {
    r.writes.value = draft.writes || r.writes.value;
    // What the places hold says which starter roles and profile the person has
    // to choose from rather than write (r-54b8).
    r.hasRoles.value = !!draft.has_roles;
    r.hasProfiles.value = !!draft.has_profiles;
    r.nameIssues.value = (json.problems || []).filter((p) => p.at === 'name' || p.field === 'name');
    // The paths the row would be written to are the server's, and they move
    // with the name a person is typing: both panels' heads follow the check
    // (spec-teams B2, 監査 R5).
    if (draft.path) r.teamPath.value = draft.path;
    const ag = (json.doc && json.doc.agreement) || NO_AGREEMENT;
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

export async function checkTeam(r: Rec) {
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
export async function checkAgree(r: Rec) {
  const one = checkersFor(r);
  const doc = toDoc(r.agreeDraft.value, r.agreeSaved.value);
  const json = await one.agree.run(agreeIn(r, { doc: { ops: r.ops.value, doc } }));
  if (!json) return;
  r.check.value = buildCheck(r.agreeDraft.value, json);
  if (!agreeDiff(r)) r.agreeSavedIssues.value = r.check.value.issues;
}
export const scheduleAgree = (r: Rec) => debounce('a' + dkey(r), () => syncAgree(r));

export function syncTeam(r: Rec) {
  checkTeam(r);
}
export function syncAgree(r: Rec) {
  checkAgree(r);
}
export function pushOp(r: Rec, op: Op) {
  r.ops.value = [...r.ops.value, op];
  seedPlaces(r);
}
// setOps puts a whole list of operations in: a revert drops them all at once.
export function setOps(r: Rec, ops: Op[]) {
  r.ops.value = ops;
  seedPlaces(r);
}

// saveTeam writes the team.yaml's ops (spec-teams B6). The refusal is the
// screen's own state: the panel's foot draws it (5 章 2).
export async function saveTeam(r: Rec, over?: string) {
  if (r.draftRow) return saveFolder(r);
  const json = await answered(r, 't', apiSave('team', r.name.value, teamIn(r, { hash: over === undefined ? r.hash.value : over, doc: { ops: r.ops.value } })), '保存できませんでした');
  if (!json) return;
  setConflict(r, 't', null);
  rebase(json);
  r.hash.value = json.hash || r.hash.value;
  r.saved.value = cloneTeam(r.draft.value);
  r.ops.value = [];
  r.baseFindings.value = r.findings.value;
  setProblems(json.items, r.project);
  setOk('t' + dkey(r), true);
  resetCanvas(r);
  cundo.delete(dkey(r));
  apiSaved();
}
// rebase puts the files a save's answer read again under the rows' feet
// (r-b41c). A save gives the rules their ids after it writes (d-4aab), so the
// agreement a person did not touch can move while another file is saved: the row
// that kept the hash it read before would be refused at its own next save as a
// change another place made. The draft a person is writing in stays where it is.
function rebase(answer: Answer<'team', 'save'> | null): void {
  for (const b of bases(answer)) {
    if (!b.name) continue;
    for (const r of recs.value) {
      if (r.draftRow || b.place !== r.place.value || b.name !== r.name.value) continue;
      if (b.kind === 'agreement') r.agreeHash.value = b.hash;
      else if (b.kind === 'team' || b.kind === '') r.hash.value = b.hash;
    }
  }
}
export async function saveAgree(r: Rec, over?: string) {
  if (r.draftRow) return saveFolder(r);
  const doc = toDoc(r.agreeDraft.value, r.agreeSaved.value);
  const json = await answered(r, 'a', apiSave('team', r.name.value, agreeIn(r, { hash: over === undefined ? r.agreeHash.value : over, doc: { ops: r.ops.value, doc } })), '保存できませんでした');
  if (!json) return;
  setConflict(r, 'a', null);
  rebase(json);
  r.agreeHash.value = json.hash || r.agreeHash.value;
  if (json.doc) {
    const d2 = fromJSON(json.doc);
    observeKeys(rulesOf(d2));
    d2.__path = r.agreePath.value;
    r.agreeSaved.value = d2;
    r.agreeDraft.value = clone(d2);
  }
  setProblems(json.items, r.project);
  setOk('a' + dkey(r), true);
  apiSaved();
  // The saved text's own problems are the new baseline, and the check the
  // panel shows is the one it now holds.
  checkAgree(r);
}
// itemInGroup is one item of a list answer as a grouped screen's group reads
// it: only the place that group stands in, the way the headquarters filters a
// group's list (web/hq.go's filterPlace). A list a save answers with holds both
// places, and the side a person is looking at is the one the row must keep
// (r-f781). Without projects listed the row keeps both places.
function itemInGroup(item: DefItem, group: string): DefItem {
  const place = groupPlace(group);
  const one = item.places && item.places[place];
  if (!one) return { ...item, places: {}, effective: place, hidden: [] };
  return { ...item, places: { [place]: one }, effective: place, hidden: [] };
}

// saveFolder writes a definition that is not there yet, whole (spec-teams B2,
// B6, C8): its team.yaml and its agreement.yaml, and, for one made from the
// scaffold, the layer's role documents and the starter profile. Either tab's
// 保存 writes the whole folder. The row becomes the saved definition in its
// place, so the person keeps working on it.
async function saveFolder(r: Rec) {
  const doc = toDoc(r.agreeDraft.value, r.agreeSaved.value);
  const body = Object.assign(teamIn(r), {
    hash: r.hash.value, agreement_hash: r.agreeHash.value, part: 'folder', doc: { ops: r.ops.value, doc },
  });
  const json = await answered(r, 't', apiSave('team', r.name.value, body), '保存できませんでした');
  if (!json) return;
  const name = r.name.value, dom = r.dom, dtab = dtabOf(r), opens = isOpen(dom);
  const group = groupOfRecord(r);
  // The row stands where the row a person was looking at stood: the first
  // reading of this name keeps its place, and a copy's save stays on the side
  // it was copied to (spec-teams B1-B4, r-f781).
  const at = firstIndexOf((x) => x === r);
  // Every reading of this name is taken again from the answer: a copy's save
  // joins the two places into one row (spec-teams B4).
  const gone = recs.value.filter((x) => x.dom === dom);
  recs.value = recs.value.filter((x) => x.dom !== dom);
  gone.forEach((x) => { setConfirm('del' + dkey(x), false); resetCanvas(x); });
  setProblems(json.items, group);
  const raw = (json.items || []).find((it) => it.name === name);
  if (!raw) { withOpen(dom, false); dropDtab(dom); return; }
  const nr = defOf(itemInGroup(raw, group), group);
  orderRows(nr, at);
  if (opens) withOpen(nr.dom, true);
  setDtab(nr.dom, dtab);
  // The definition the save wrote holds its own checks now: the figure's and
  // the rules' are read again from the file it wrote (spec-boundary 5). The
  // 保存しました stands on the row's own foot after that reading, so the
  // check's own draw does not take it away (1.4.6, 監査 R7).
  await checkTeam(nr);
  if (nr.agreeSaved.value) await checkAgree(nr);
  setOk((dtabOf(nr) === 'agree' ? 'a' : 't') + dkey(nr), true);
  syncURL();
  apiSaved();
}
// doDelete takes one definition's folder out of one place (spec-teams B6): its
// team.yaml and its agreement.yaml, and the directory itself when the two files
// were all it held. Every record of that name is read again from the answer,
// because the other place's reading may be the one in effect now.
export async function doDelete(r: Rec) {
  const json = await answered(r, 't', apiDel('team', r.name.value, r.place.value, r.hash.value, r.agreeHash.value, r.project && r.project !== 'global' ? r.project : undefined), '削除できませんでした');
  if (!json) return;
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
// answered waits for a save's or a delete's answer and answers its document, or,
// when the server refused it, puts the server's sentence on the row's foot and
// answers null (spec-boundary 3): a screen builds no sentence of its own.
async function answered<T>(r: Rec, which: string, one: Promise<{ json: T | null }>, fallback: string): Promise<T | null> {
  try {
    const res = await one;
    return res.json;
  } catch (e) {
    const c = conflictOf(e);
    if (c) {
      // The file moved: the row keeps its draft and the notice stands above the
      // save band (r-3c77, d-a34a).
      setConflict(r, which, { c });
      return null;
    }
    refuse(r, which, apiProblemText(e, fallback));
    return null;
  }
}
// refuse holds the server's sentence on the row's foot (the shape tab's, or the
// rules tab's) and draws it; a screen builds no sentence of its own.
export function refuse(r: Rec, which: string, text: string) {
  setRefusal(which + dkey(r), text || '保存できませんでした');
}
// clearOk takes the notice away when the words it stood for change: a save's
// 保存しました stays until the next change writes over it (1.4.6).
export function clearOk(r: Rec) {
  if (state.value.ok['t' + dkey(r)] || state.value.ok['a' + dkey(r)]) {
    const m = { ...state.value.ok };
    delete m['t' + dkey(r)];
    delete m['a' + dkey(r)];
    put({ ok: m });
  }
}
// setProblems takes the counts a save answered with back into the rows, so the
// row's 問題/警告 chip is the server's own count again (spec-boundary 5).
type TeamItems = NonNullable<Answer<'team', 'save'>['items']>;
function setProblems(items: TeamItems | null | undefined, project?: string) {
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
type UndoPoint = { ops: Op[]; label: string };
const cundo = new Map<string, UndoPoint[]>();
let actSeq = 0;
let ctyping: { key: string | null; field: string | null } = { key: null, field: null };
let toastRow: Rec | null = null, toastTimer: ReturnType<typeof setTimeout> | undefined;
const cloneOps = (ops: Op[] | null | undefined): Op[] => clone(ops || []);
function canvasUndoPoint(r: Rec, label: string) {
  const k = dkey(r), st = cundo.get(k) || [];
  st.push({ ops: cloneOps(r.ops.value), label });
  while (st.length > 80) st.shift();
  cundo.set(k, st);
}
export function canvasToast(r: Rec, msg: string, withUndo?: boolean): void {
  toastRow = r;
  put({ toast: { msg, undo: !!withUndo } });
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { put({ toast: null }); toastRow = null; }, 4000);
}
export function canvasUndoFromToast(): void { if (toastRow) canvasUndo(toastRow); }
// forgetUndo drops the operations a row could take back (a save, a revert).
export function forgetUndo(r: Rec) { cundo.delete(dkey(r)); }
export function canvasAct(r: Rec, ops: Op[], o?: { undo?: string; focus?: string; message?: string }) {
  o = o || {};
  r.lastAct.value = ++actSeq;
  r.promptsReady.value = undefined;
  clearOk(r);
  canvasUndoPoint(r, o.undo || '');
  (ops || []).forEach((op) => pushOp(r, op));
  ctyping = { key: null, field: null };
  if (o.focus) focusAsk.value = { dom: r.dom, sel: o.focus };
  checkTeam(r);
  if (o.message) canvasToast(r, o.message, true);
}
export function canvasTyping(r: Rec, op: Op, field: string) {
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
export function canvasEndTyping(r: Rec | null): void {
  if (!r) return;
  if (ctyping.key === dkey(r)) { ctyping = { key: null, field: null }; checkTeam(r); }
}
export function canvasUndo(r: Rec): void {
  const k = dkey(r), st = cundo.get(k);
  r.promptsReady.value = undefined;
  clearOk(r);
  if (!st || !st.length) return;
  const point = st.pop();
  if (!point) return;
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
export function resetCanvas(r: Rec): void {
  if (!r) return;
  r.sel.value = null;
  r.side.value = 'attr';
  r.menu.value = null;
  r.typed.value = null;
  r.flash.value = null;
  r.placed.value = {};
  r.moved.clear();
  seedPlaces(r);
}

// The worker's prompt is the one hire --dry builds (r-4cf9). Its name and task
// are chosen at hire, so the screen asks with values that cannot be mistaken
// for the purpose or another line, and shows <ワーカー名> / <依頼> in their
// place. The values are the walk's own too, so it can put them back.
const WRK_NAME = 'zz-worker-name-0', WRK_TASK = 'zzTaskPlaceholder0';
export function workerPrompt(r: Rec, i: number) {
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
  apiCheck('team', { place: r.place.value, name: r.name.value, part: 'worker', ...(r.project && r.project !== 'global' ? { project: r.project } : {}), doc: { name: WRK_NAME, purpose: w.purpose, task: WRK_TASK, from: w.from } }).then((res) => {
    delete r.wpromptReq[k];
    const ans = res.json;
    r.wprompts.value = {
      ...r.wprompts.value,
      [k]: { segments: (ans && ans.prompt) || [], name: WRK_NAME, task: WRK_TASK },
    };
  }).catch((e) => {
    delete r.wpromptReq[k];
    r.wprompts.value = {
      ...r.wprompts.value,
      [k]: { segments: [], name: WRK_NAME, task: WRK_TASK, err: apiProblemText(e, 'プロンプトを読めませんでした') },
    };
  });
  return null;
}
// rolesFor and profilesFor are the documents one row's member 属性 offers: the
// row's own project's and the global layer's, so another project's definition
// chooses from its own names (spec 4 章). Without a headquarters they are the
// one list the island carries. A row that is not there yet stands in the place
// it would be written to, so it offers that place's names too (r-54b8). ROLES_REV
// is read so a list that comes after the first frame draws the columns again.
// rowGroup is the group a row's names are read from: a listed project's own, the
// global layer's, or this project's.
const rowGroup = (r: Rec | null): string => (r ? (r.project || (r.place.value === 'global' ? 'global' : CURRENT)) : CURRENT);
export function rolesFor(r: Rec) {
  ROLES_REV.value;
  if (!HQ) return ROLES.value;
  const key = rowGroup(r);
  const all = key === 'global'
    ? Object.values(ROLES_BY).flat()
    : (ROLES_BY[key] || []).concat(ROLES_BY.global || []);
  const seen = new Set();
  return all.filter((x) => !seen.has(x.name) && seen.add(x.name));
}
export function profilesFor(r: Rec) {
  ROLES_REV.value;
  if (!HQ) return PROFILES;
  const key = rowGroup(r);
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
// YFile is one file the YAML tab draws: the answer's file, with the key the tab
// opens it by.
export type YFile = {
  file: string; place: Place; name: string; kind: string; label: string;
  path: string; hash: string; text: string; hidden: boolean; problems: Schemas['Problem'][];
};
export const yfiles = signal<YFile[]>((LIST.files || []).map((f): YFile => ({
  file: `${f.name}.${f.place}.${f.kind}`, place: f.place, name: f.name || '', kind: f.kind || '',
  label: PLACE[f.place] || '', path: f.path || '', hash: f.hash || '',
  text: f.text || '', hidden: !!f.hidden, problems: f.problems || [],
})));
export const fileOf = (file: string): YFile | null => yfiles.value.find((f) => f.file === file) || null;
export const fileKeys = (): string[] => PLACES.flatMap((p) => yfiles.value.filter((f) => f.place === p).map((f) => f.file));
export const fileText = (file: string): string => (state.value.ytext[file] !== undefined ? state.value.ytext[file] : ((fileOf(file) || {}).text || ''));
export const fileDirty = (file: string): boolean => state.value.ytext[file] !== undefined && state.value.ytext[file] !== ((fileOf(file) || {}).text || '');
export const yIssues = (file: string): Schemas['Problem'][] => state.value.yc[file] || (fileOf(file) || {}).problems || [];
export const recForFile = (f: YFile) => recs.value.find((x) => x.name.value === f.name && x.place.value === f.place && (x.project === CURRENT || x.project === '')) || null;
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
export function pressYfile(file: string) {
  put({ yfile: file });
  syncURL();
}
// yamltab's own state, kept by the screen: the text a person typed, and the
// problems the part's check answered.
export function yamlState(file: string, st: { text: string }) {
  const f = fileOf(file);
  setYtext(file, f && st.text === f.text ? undefined : st.text);
}
export function yamlProblems(file: string, list: Schemas['Problem'][]) {
  setYc(file, list);
}
export function yamlRevert(file: string) {
  setYtext(file, undefined);
}
export function yamlSaved(file: string, answer: { json?: { text?: string; hash?: string } | null } | null): void {
  const json: { text?: string; hash?: string } = (answer && answer.json) || {};
  const typed = state.value.ytext[file];
  const f = fileOf(file);
  if (!f) return;
  const text = typeof json.text === 'string' ? json.text : (typed !== undefined ? typed : f.text);
  yfiles.value = yfiles.value.map((x) => (x.file === file ? { ...x, hash: json.hash || x.hash, text } : x));
  setYtext(file, undefined);
  setYc(file, undefined);
}

// promptsOf is each member's prompt, cut into the pieces it was built from
// (r-4cf9): the check answers it, and the column shows it without building a
// line of its own. A definition with a problem is not given any (d-20ac).
export const promptsOf = (r: Rec) => r.prompts.value;
export const promptBlockedOf = (r: Rec) => r.promptsReady.value === false;

// ---- 描くための小さな問い ----------------------------------------------
export const teamLabel = (r: Rec) => r.name.value || '新しいチーム定義';
export const currentProjectName = (): string => {
  const p: Schemas['HQProjects'] | null = (HQ && HQ.projects) || null;
  return (p && p.current && p.current.name) || '';
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
    const rules: string[] = [];
    opens.forEach((n) => {
      const r = byName(n);
      if (!r) return;
      openRules().forEach((kk) => {
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
      openRuleKey(`${r.uid}${r.place.value}:${hit._k}`);
      put({ wantRule: { name: r.dom, k: hit._k } });
    }
  });
  const file = p.get('file');
  if (file) put({ yfile: file });
  if (location.pathname === '/teams/yaml') put({ tab: 'yaml' });
}

// ---- 共有の部品（ruledoc）の受け口 ------------------------------------
export const rowOfPanel = (r: Rec) => {
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

// ---- 帯と、ページの外への受け渡し ------------------------------------------
// savedCount is the number the band shows: the definitions, which are the rows
// that are saved — a row that is not there yet is not one (spec-nav 1.1,
// 監査 R7). main.js keeps the band's number on it.
export const savedCount = () => recs.value.filter((r: Rec) => !r.draftRow).length;
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
// Carried is what the band hands back to this screen: the rows' ops, the rules
// drafts and the YAML texts (spec-nav 1.5).
export type Carried = {
  teams?: { place: Place; name: string; hash: string; ops: Op[] }[];
  agrees?: { place: Place; name: string; hash: string; doc: unknown }[];
  files?: { file: string; hash: string; text: string | undefined }[];
};
export function restore(carried: Carried) {
  (carried.teams || []).forEach((c) => {
    const r = recs.value.find((x) => x.name.value === c.name && x.place.value === c.place && !x.draftRow);
    if (!r || (c.hash && r.hash.value !== c.hash)) return;
    setOps(r, clone(c.ops));
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
    setYtext(c.file, c.text);
  });
}

export function take() {
  const teams = [];
  for (const r of recs.value) if (!r.draftRow && changeCount(r) > 0) teams.push({ place: r.place.value, name: r.name.value, hash: r.hash.value, ops: r.ops.value });
  const agrees = [];
  for (const r of recs.value) if (!r.draftRow && r.agreeSaved.value && agreeDiff(r)) agrees.push({ place: r.place.value, name: r.name.value, hash: r.agreeHash.value, doc: toDoc(r.agreeDraft.value, r.agreeSaved.value) });
  const files = yfiles.value.filter((f: YFile) => fileDirty(f.file)).map((f: YFile) => ({ file: f.file, hash: f.hash, text: state.value.ytext[f.file] }));
  return { teams, agrees, files };
}

// ---- 押し込み（部品が呼ぶ） --------------------------------------------
export function pressTab(t: 'list' | 'yaml') {
  if (state.value.tab === t) return;
  put({ tab: t });
  syncURL();
}
export function pressToggle(r: Rec) {
  withOpen(r.dom, !isOpen(r.dom));
  syncURL();
}
export async function pressNew(group?: string) {
  // The head a person pressed says the place the row stands in (r-410c): the
  // row is that group's, and there is no switch inside it to move it.
  const key = group === undefined ? placeGroups()[0].key : group;
  await addDraft('new', key === 'global' ? 'global' : 'project', '', null, key);
}
// pressStarterRoles asks for the starter role documents, or drops the ask: the
// server writes them only when the person pressed this, and only the ones no
// place holds (r-54b8). The check is asked again, so the files a save would
// write follow the press.
export function pressStarterRoles(r: Rec) {
  r.starterRoles.value = !r.starterRoles.value;
  checkTeam(r);
}
// pressStarterProfile is pressStarterRoles for the starter profile.
export function pressStarterProfile(r: Rec) {
  r.starterProfile.value = !r.starterProfile.value;
  checkTeam(r);
}
export async function pressDup(r: Rec) {
  await addDraft('dup', r.place.value, theName(r.name.value), { place: r.place.value, name: r.name.value }, r.project);
}
export async function pressCopy(r: Rec) {
  const to = other(r.place.value);
  // A grouped screen draws each side as rows under its own head: the copy is a
  // row of the side it was copied to (r-f781).
  const group = copyGroupOf(to, groupOfRecord(r), GROUPED ? CURRENT : '');
  await addDraft('copy', to, r.name.value, { place: r.place.value, name: r.name.value }, group);
}
// pressPlace moves a definition that is not there yet to the other place. The
// row keeps its place in the list while its place changes (spec-teams B1, N4).
export async function pressPlace(r: Rec, place: Place) {
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
export function pressDiscard(r: Rec) {
  if (r) dropDraft(r);
}
export function pressDel(r: Rec) { setConfirm('del' + dkey(r), true); }
export function pressDelNo(r: Rec) { setConfirm('del' + dkey(r), false); }
export async function pressDelYes(r: Rec) { if (r) await doDelete(r); }
// pressName is a definition that is not there yet being named: the name is
// checked, and the files a save would write are read again: the server owns
// both (spec-teams B2). The field keeps its place and its cursor while the
// person types.
export function pressName(r: Rec, value: string) {
  r.name.value = value;
  debounce('n' + r.dom, () => checkTeam(r));
}
// pressMission is the row's own mission field: it writes one operation and asks
// the check again (spec-teams 1.4.6).
export function pressMission(r: Rec, value: string) {
  const text = value.replace(/\n/g, ' ');
  r.ops.value = r.ops.value.filter((o) => o.kind !== 'mission');
  if (text !== r.saved.value.mission) pushOp(r, { kind: 'mission', text });
  checkTeam(r);
}
export function pressDtab(r: Rec, t: string) {
  setDtab(r.dom, t);
  syncURL();
  if (t === 'agree') checkAgree(r);
}
export function pressPmode(r: Rec, v: string) {
  put({ pmode: { ...state.value.pmode, [dkey(r)]: v } });
}
export function pressToYaml(key: string) {
  put({ tab: 'yaml', yfile: `${key}.team` });
  syncURL();
  window.scrollTo(0, 0);
}
export function pressTSave(r: Rec) { saveTeam(r); }
// ---- a save refused because the file moved (r-3c77, d-a34a) ---------------
// pressOverwrite writes what the person wrote over the file the answer showed:
// the save carries the hash of that file, so a file that moved again is refused
// the same way instead of being written over unseen.
export function pressOverwrite(r: Rec, which: string) {
  const hold = conflictAt(r, which);
  if (!hold) return;
  if (which === 'a') saveAgree(r, hold.c.file.hash || '');
  else saveTeam(r, hold.c.file.hash || '');
}
// pressRevertConflict reads the file the answer carried into the row: the row
// then stands on the file as it is, and the row before the read is kept for
// 取り消す.
export function pressRevertConflict(r: Rec, which: string) {
  const hold = conflictAt(r, which);
  if (!hold || !hold.c.file.doc) return;
  const before: RowBefore = {
    saved: cloneTeam(r.saved.value), ops: cloneOps(r.ops.value),
    agreeSaved: r.agreeSaved.value ? clone(r.agreeSaved.value) : null, agreeDraft: clone(r.agreeDraft.value),
    hash: r.hash.value, agreeHash: r.agreeHash.value,
  };
  if (which === 'a') {
    const doc = fromJSON(hold.c.file.doc);
    r.agreeSaved.value = clone(doc);
    r.agreeDraft.value = clone(doc);
    r.agreeHash.value = hold.c.file.hash || '';
  } else {
    r.saved.value = cloneTeam(hold.c.file.doc as Team);
    r.ops.value = [];
    r.hash.value = hold.c.file.hash || '';
    if (hold.c.file.path) r.teamPath.value = hold.c.file.path;
    r.baseFindings.value = r.findings.value;
    seedPlaces(r);
  }
  setConflict(r, which, { c: hold.c, undone: true, before });
  if (which === 'a') checkAgree(r); else checkTeam(r);
}
// pressUndoConflict puts the row back the way the person had it, over the file
// the row has just read: their draft stands as a change to that file.
export function pressUndoConflict(r: Rec, which: string) {
  const hold = conflictAt(r, which);
  const before = hold && hold.before;
  if (!hold || !before) return;
  r.saved.value = cloneTeam(before.saved);
  r.ops.value = cloneOps(before.ops);
  r.agreeSaved.value = before.agreeSaved ? clone(before.agreeSaved) : null;
  r.agreeDraft.value = clone(before.agreeDraft);
  r.hash.value = before.hash;
  r.agreeHash.value = before.agreeHash;
  seedPlaces(r);
  setConflict(r, which, null);
  if (which === 'a') checkAgree(r); else checkTeam(r);
}
export function pressTRevert(r: Rec) {
  setOps(r, []);
  resetCanvas(r);
  forgetUndo(r);
  put({ confirm: {} });
  syncTeam(r);
}
export function pressSave(r: Rec) { saveAgree(r); }
export function pressRevert(r: Rec): void {
  // 元に戻す puts the document back to the one the row opened with: the saved
  // agreement, or, for a team that has none, the empty document it started from
  // (n-e3de).
  const saved = r.agreeSaved.value;
  r.agreeDraft.value = saved ? clone(saved) : emptyAgreement(r.name.value);
  checkAgree(r);
}
export function pressToastUndo() { canvasUndoFromToast(); }
// pressVrule opens the team's own rule a finding names (spec-teams 1.4.5,
// 監査 R9) in the ルール tab, with the cursor in its body (監査 R12).
export function pressVrule(r: Rec, key: string | number) {
  if (!r) return;
  withOpen(r.dom, true);
  setDtab(r.dom, 'agree');
  const hit = typeof key === 'number'
    ? rulesOf(r.agreeDraft.value).find((x) => x._k === key)
    : rulesOf(r.agreeDraft.value).find((x) => String(x.id) === key);
  checkAgree(r);
  syncURL();
  if (hit) jumpToRule(r, hit._k);
}
// pressVpick puts the cursor in a finding's own field, or picks what it names on
// the figure.
export function pressVpick(r: Rec, m: Finding & { nameField?: boolean }) {
  if (m.nameField) { focusAsk.value = { dom: r.dom, sel: '.in-name' }; return; }
  if (m.node) selectPick(r, { type: 'node', id: m.node }, 'attr');
  else if (m.worker != null) selectPick(r, { type: 'worker', i: m.worker }, 'attr');
}
// pressLine is a press on one line of the topology tab's YAML preview: it picks
// what the line stands for, the way a press on the canvas's own point does
// (spec-teams B7). A mission line puts the cursor in the mission field.
export type LinePress = { f?: string; node?: string; worker?: number; a?: string; b?: string };
export function pressLine(r: Rec, p: LinePress) {
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
export function selectPick(r: Rec, sel: CanvasPick | null, side?: string) {
  r.typed.value = null;
  r.sel.value = sel;
  if (side) r.side.value = side;
}

// ---- 立ち上げ ----------------------------------------------------------
export function boot() {
  if (HQ && HQ.teams && HQ.teams.length) {
    HQ.teams.forEach((g) => {
      const key = GROUPED ? groupKeyOf(g) : (g.global ? 'global' : '');
      rowsIn(g).forEach((item) => defOf(item, key, g.global ? 'global' : 'project'));
    });
  } else {
    (LIST.items || []).forEach((item) => {
      for (const place of PLACES) if (item.places && item.places[place]) defOf(item, place === 'global' ? 'global' : '', place);
    });
  }
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
    later.push(hqList('roles').then((answer) => {
      (answer.groups || []).forEach((g) => {
        ROLES_BY[groupKeyOf(g)] = ((g.list && g.list.items) || []).map((item) => {
          const places = item.places || {}, eff = item.effective || (places.project ? 'project' : 'global');
          const name = item.name || '';
          const doc = places[eff]?.doc;
          return name ? { name, title: (doc && doc.title) || '' } : null;
        }).filter((x): x is RoleName => x !== null);
      });
      ROLES_IN.value = true;
      bumpRoles();
    }).catch(() => {}).finally(rolesCame));
    later.push(hqList('profiles').then((answer) => {
      (answer.groups || []).forEach((g) => {
        PROFILES_BY[groupKeyOf(g)] = ((g.list && g.list.items) || []).map((item) => item.name || null).filter((x): x is string => x !== null);
      });
      bumpRoles();
    }).catch(() => {}));
  }
  // Every definition is checked once at open, so the figure's problems and the
  // node badges are the server's own reading of the saved file, and the
  // agreement's saved problems are the baseline its save is held to
  // (spec-teams C2, X3).
  const first = recs.value.slice();
  const checks = first.flatMap((r: Rec) => [checkTeam(r), checkAgree(r)]);
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
export function openAgree(r: Rec) {
  withOpen(r.dom, true);
  setDtab(r.dom, 'agree');
  checkAgree(r);
  syncURL();
}

// jumpToRule opens one rule of a team's own agreement and puts the cursor in
// its body (spec-teams 1.4.5, spec-roles 1.2.4, 監査 R12). The check's column
// reads through this, and so does the address.
export function jumpToRule(r: Rec, k: number) {
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
