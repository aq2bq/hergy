import { editTeam, restoreTeam, teamActions, editYaml, teamDocument, type TeamEditing, type TeamCarry, type YamlEditing, type TeamAction } from '../lib/editing.js';
import { resourceIdentity } from '../lib/resource-identity.js';
// Team rows own display state only: the selection, addresses and positions.
// Resource values, validation, file bases and operation provenance are read
// through editing. Screens cannot construct a save/check request themselves.
import { signal, computed } from '../lib/preact.js';
import { saved as apiSaved, list as apiList, hqList, problemText as apiProblemText, bases } from '../lib/api';
import type { Answer, ApiAnswer, Schemas } from '../lib/api';
import { select, startSelection } from './selection.js';
import type { Side } from './selection-table.js';

// CheckAnswer is what POST /api/team/check answers with, read from the operation
// (the saved definition's answer, or the one for a definition not there yet).
export type CheckAnswer = ApiAnswer<'checkTeam'>;

import {
  fromJSON, clone,
} from '../lib/ruledoc.js';
import { parseOpen, readOpen, rowsFor, token, writeOpen } from '../lib/address.js';
import { copyGroupOf, groupPlace, projectOfGroup } from '../lib/copy';
import { roleEntries, pendingRoleChanges, takeRoleEdits, restoreRoleEdits } from './role-modal.js';
import { runDanger } from '../lib/danger.js';
import { dropUndo } from '../lib/undo.js';
import { takeDraft, setDirty } from '../lib/nav.js';
import { place } from './place';
import type { Positions } from './place';
import type { CanvasMenu, CanvasTyped } from './canvas/shared';

// ---- 画面が読む形 -------------------------------------------------------
// Member, Worker and Team are editing's readonly document as a row draws it.
// teamDocument converts the API reading at the boundary.
export type Team = import('../lib/editing.js').TeamDocument;
export type Member = Team['members'][number];
export type Worker = Team['workers'][number];

// Op is an action constructed by editing's named operations. The row can keep
// its history for Undo, but cannot construct a save body from it.
export type Op = TeamAction;

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
const hasTurn = (t: Team, a: string, b: string): boolean => t.turns.some(([p, q]) => p === a && q === b);
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
  yfile: string | null;
  pmode: Record<string, string>;
  wantRule: { name: string; k: number } | null;
  toast: { msg: string; undo: boolean } | null;
};
const put = (patch: Partial<State>) => { state.value = { ...state.value, ...patch }; };
export const state = signal<State>({
  tab: (view && view.dataset.tab) === 'yaml' ? 'yaml' : 'list',
  open: new Set(), dtab: {},
  yfile: null, pmode: {}, wantRule: null,
  // refusal is the server's sentence on one row's foot (the shape tab's or the
  // rules tab's), and toast the words the canvas says at the bottom.
  toast: null,
});
// focusAsk is the one field a person's press should land in after the draw
// (5 章 5): a member just added, a rule just opened. The row it names puts the
// cursor there itself. A copy also carries the row into the screen (d-022a).
export const focusAsk = signal<{ dom: string; sel: string; scroll?: boolean } | null>(null);
// The screen's maps are written by the setter of the map they belong to: each
// copies the map it is about, puts the entry in, and keeps the entry's own type.
// urlGroup is the place a row is written in: the project's path (or `project`)
// on the side the row stands in, and `global` for the global layer.
const urlGroup = (r: Rec): string => (GROUPED ? groupOfRecord(r) : r.place.value);
// here is the place this screen writes for its own project, and rowRef reads a
// row the way the address writes it (⑥-2, d-a143).
const here = CURRENT;
const rowRef = (r: Rec): { name: string; group: string } => ({ name: r.name.value, group: urlGroup(r) });
// withOpen opens or closes one row by the address it stands at: every spelling
// that names the row goes (the old name-only one included), and the qualified
// address is put in its place (d-a143).
const withOpen = (r: Rec, on: boolean) => {
  const s = new Set(state.value.open);
  s.forEach((one) => { const ref = parseOpen(one)[0]; if (ref && rowsFor(ref, [r], rowRef, here).length) s.delete(one); });
  if (on) s.add(token(r.name.value, urlGroup(r)));
  put({ open: s });
};
export const isOpenRow = (r: Rec): boolean => [...state.value.open]
  .some((one) => { const ref = parseOpen(one)[0]; return !!ref && rowsFor(ref, [r], rowRef, here).length > 0; });
export const dtabOf = (r: Rec): string => state.value.dtab[r.dom] || 'shape';
export const pmodeOf = (r: Rec): string => state.value.pmode[dkey(r)] || 'edit';
const editorOf = (r: Rec, which: string) => which === 'a' ? r.editing.agreement : r.editing.team;
export const okOf = (key: string): boolean => { const r = recs.value.find((r) => key.slice(1) === dkey(r)); return !!r && editorOf(r, key[0]).read.value.savedLine; };
export const refusalOf = (r: Rec, which: string): string => editorOf(r, which).read.value.refusal || (r.draftRow && !r.seeded.value && r.editing.team.checkingError.value ? apiProblemText(r.editing.team.checkingError.value, '元にしたチーム定義を読めませんでした') : '');
export const conflictAt = (r: Rec, which: string) => editorOf(r, which).read.value.conflict;
const setDtab = (dom: string, t: string) => { put({ dtab: { ...state.value.dtab, [dom]: t } }); };
const dropDtab = (dom: string) => { const m = { ...state.value.dtab }; delete m[dom]; put({ dtab: m }); };
const debounceTimers: Record<string, ReturnType<typeof setTimeout> | undefined> = {};
export const debounce = (k: string, fn: () => void, ms = 260) => { clearTimeout(debounceTimers[k]); debounceTimers[k] = setTimeout(fn, ms); };

// ---- 記録の形 -----------------------------------------------------------
// Issue is one problem as the rules panel draws it: the rule's own key (0 when
// the finding is not about a rule), the field it stands on, and the sentence.
export type Issue = { sev: string; k: number; f: string; text: string; definition: string };
// Finding is the figure's problem: the node, the worker or the field it stands
// beside, and the server's sentence (the role and rule it names, when it does).
export type Finding = { sev: string; field: string; node: string | null; worker: number | null; text: string; role: string;  };
export type CheckDiff = Schemas['TeamDiff'];
// TeamPreview and RuleMarkdown are the check's two previews, as checkTeam answers
// them: the team.yaml's lines and the diff (TeamPreview), and the agreement's
// Raw Markdown preview and its character count (TeamProseMarkdown).
// They are read from the operation, not written here (n-e3de).
export type TeamPreview = Schemas['TeamPreview'];
export type RuleMarkdown = Schemas['TeamProseMarkdown'];
export type Check = { templateSize: number; issues: Issue[]; markdown: RuleMarkdown; diff: CheckDiff; readers: Schemas['TeamProseReader'][] };
// Pick is what the canvas has picked: a point of the figure, a worker, or a
// line between two points.
export type CanvasPick = { type: 'node'; id: string } | { type: 'worker'; i: number } | { type: 'edge'; k: string };
export type PromptSegment = Schemas['TeamPromptSegment'];
// RuleDoc is the rules document the panel edits (lib/ruledoc makes it from the
// server's answer): the sections, their rules, and the path it came from.
export type Rule = { id?: string; _k: number; text?: string; note?: string; in_prompt?: boolean };
export type RuleSection = { heading?: string; level?: number; intro?: string; rules: Rule[] };
export type RuleDoc = import('../lib/ruledoc.js').EditableDocument;

// Sig is one field of a record: a signal, so a part that reads it draws again
// when it changes (依頼文 1).
type Sig<T> = { value: T };
type Read<T> = { readonly value: T };

// Resource fields below are computed projections. Signals belong only to
// server list metadata and the screen's selection/layout, never to a draft.
export type Rec = {
  editing: TeamEditing;
  uid: number; dom: string; project: string; places: Place[];
  draftRow: boolean; how: string; base: TeamBase | null;
  name: Read<string>; place: Read<Place>;
  teamPath: Read<string>; agreePath: Read<string>;
  hash: Read<string>; agreeHash: Read<string>; broken: Sig<boolean>;
  // problem is why the definition's team.yaml cannot be read: the row stands and
  // says so (r-ea17, d-17ab).
  problem: Sig<string>;
  // mark is the owner's word for a name both layers hold (r-ea17).
  mark: Sig<string>;
  hidden: Sig<{ place: string; path?: string; hash?: string }[]>;
  ops: Read<Op[]>; saved: Read<Team>;
  agreeSaved: Read<RuleDoc | null>; agreeDraft: Read<RuleDoc>; agreeSavedIssues: Read<Issue[] | null>;
  findings: Read<Finding[]>; problems: Sig<Schemas['TeamCounts']>; check: Read<Check | null>;
  size: Sig<number>; limit: Sig<number>;
  drawMark: Sig<number>; seeded: Read<boolean>;
  // born is the row's content when it was made (new, duplicate or copy), so 取り消し
  // can tell a row nothing was put into from one the person changed (r-d480).
  writes: Read<string[]>; nameIssues: Read<Schemas['Problem'][]>;
  hasRoles: Read<boolean>; hasProfiles: Read<boolean>;
  starterRoles: Read<boolean>; starterProfile: Read<boolean>;
  carry: Read<Carry | null>;
  preview: Read<TeamPreview>; baseFindings: Read<Finding[]>;
  sel: Sig<CanvasPick | null>; side: Sig<Side>; menu: Sig<CanvasMenu | null>; typed: Sig<CanvasTyped | null>; flash: Sig<string | null>;
  lastAct: Sig<number>;
  renameNotes: Sig<string[]>;
  placed: Sig<Positions>; layout: Sig<Positions>;
  // wide is the figure laid out in rows rather than round the human: a ring too
  // tall for the screen is drawn this way (r-e39c). moved is the points a person
  // put down by hand, so a new shape leaves them where they are (not a signal:
  // it is read while the places are laid out).
  wide: Sig<boolean>; moved: Set<string>;
  draft: Read<Team>; key: Sig<string>;
};
// Carry is what a copy into the global layer brings with it (r-7e4a, d-dc81):
// the role documents and profiles the global layer does not hold and the project
// the row copies from does, and the names the person picked to write with it. A
// row that is not a project's definition copied into the global layer carries
// nothing: its carry is null.
export type Carry = { roles: string[]; profiles: string[]; chosenRoles: string[]; chosenProfiles: string[] };

// TeamBase is the definition a duplicate or a copy stands on. Project names the
// listed project the base stands in: a row under a project's head is that
// project's file, and a copy of it must read that file rather than the one this
// serve was started in (r-a028).
export type TeamBase = { place: Place; name: string; project?: string };

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
export const promptComparisonInput = (r: Rec): Omit<Schemas['PromptComparisonRequest'], 'member'> => r.editing.promptInput();
export const cloneTeam = teamDocument;

// ---- 記録（1 定義ごと） ------------------------------------------------
// RecInit is a record as it is made: every field a value, the fields that never
// change and the ones recOf turns into signals of their own.
type RecInit = {
  uid?: number; dom: string; project: string; places?: Place[]; draftRow: boolean; how: string; base: TeamBase | null;
  name: string; place: Place; teamPath: string; agreePath: string; hash: string; agreeHash: string;
  broken: boolean; problem: string; mark: string; hidden: { place: string; path?: string; hash?: string }[];
  saved: Team; agreeSaved: RuleDoc | null; problems: Schemas['TeamCounts']; size: number; limit: number; drawMark: number; seeded: boolean;
  sel: CanvasPick | null; side: Side; menu: CanvasMenu | null; typed: CanvasTyped | null; flash: string | null; lastAct: number;
};
// recOf puts one record together: every field is written where it stands, so a
// field the type gains is one the compiler asks for here (依頼書 1). draft and
// key are computed from the record itself, so they are made last.
function recOf(init: RecInit, held?: TeamEditing): Rec {
  const editing = held || editTeam(resourceIdentity('team', { name: init.draftRow ? undefined : init.name, place: init.place, project: projectOfGroup(init.place === 'global' ? 'global' : init.project || CURRENT) }), {
    name: init.name, team: init.saved, hash: init.hash, agreement: init.agreeSaved, agreementHash: init.agreeHash,
    teamPath: init.teamPath, agreementPath: init.agreePath, teamText: LIST.files?.find((f) => f.name === init.name && f.place === init.place && f.kind === 'team')?.text || '', agreementText: init.agreeSaved?.text || '', how: init.draftRow ? init.how : undefined,
    base: init.base ? resourceIdentity('team', init.base) : null, seeded: init.seeded, readable: !(init.broken || init.problem),
  });
  const read = <T>(get: () => T): Read<T> => computed(get);
  const r: Rec = {
    editing,
    uid: init.uid || 0, dom: init.dom, project: init.project, places: init.places || [],
    get draftRow() { return r.editing.read.value.fresh; }, get how() { return r.editing.read.value.how; }, get base() { const b = r.editing.read.value.base; return b ? { place: b.place, name: b.name || '', project: b.project } : null; },
    name: read(() => r.editing.read.value.name), place: read(() => r.editing.team.take().target.place),
    teamPath: read(() => r.editing.read.value.teamPath), agreePath: read(() => r.editing.read.value.agreementPath),
    hash: read(() => r.editing.team.read.value.hash), agreeHash: read(() => r.editing.agreement.read.value.hash), broken: signal(init.broken), problem: signal(init.problem), mark: signal(init.mark), hidden: signal(init.hidden),
    ops: read(() => clone([...r.editing.operations.value])), saved: read(() => teamDocument(r.editing.team.read.value.saved)), draft: read(() => teamDocument(r.editing.team.read.value.draft)),
    agreeSaved: read(() => r.editing.agreement.read.value.saved ? clone(r.editing.agreement.read.value.saved) : null), agreeDraft: read(() => clone(r.editing.agreement.read.value.draft)),
    agreeSavedIssues: read(() => convProblems(r.editing.agreement.read.value.baseProblems)),
    findings: read(() => figFindings(r, r.editing.team.read.value.problems)), baseFindings: read(() => figFindings(r, r.editing.team.read.value.baseProblems)),
    check: read(() => r.editing.agreement.read.value.checked ? buildCheck(clone(r.editing.agreementValidation.value) as CheckAnswer) : null), problems: signal(init.problems),
    size: signal(init.size), limit: signal(init.limit), drawMark: signal(init.drawMark), seeded: read(() => r.editing.read.value.seeded),
    writes: read(() => [...r.editing.read.value.writes]), nameIssues: read(() => r.editing.team.read.value.problems.filter((p) => p.at === 'name' || p.field === 'name')),
    hasRoles: read(() => r.editing.read.value.hasRoles), hasProfiles: read(() => r.editing.read.value.hasProfiles), starterRoles: read(() => r.editing.read.value.starterRoles), starterProfile: read(() => r.editing.read.value.starterProfile),
    carry: read(() => { const c = r.editing.read.value.carry; return c ? { roles: [...c.roles], profiles: [...c.profiles], chosenRoles: [...c.chosenRoles], chosenProfiles: [...c.chosenProfiles] } : null; }), preview: read(() => { const p = r.editing.validation.value.preview; return p && 'diff' in p ? clone(p) as TeamPreview : { lines: [], diff: { added: [], removedBefore: [], removedAtEnd: false } }; }),
    sel: signal(init.sel), side: signal(init.side), menu: signal(init.menu), typed: signal(init.typed), flash: signal(init.flash),
    lastAct: signal(init.lastAct), renameNotes: signal<string[]>([]),
    // placed is where a person put a point down (a double press's menu, or a drag
    // that was let go), and layout is where the figure draws every point: the
    // places above and an automatic place for the rest. The drawing reads layout
    // and writes nothing (webui-components 1 章).
    placed: signal<Positions>({}),
    wide: signal(false), moved: new Set<string>(),
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
export const missingOpen = signal<string[]>([]);
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
const NO_AGREEMENT: Schemas['TeamAgreement'] = { path: '', hash: '', has: false };

function defOf(item: DefItem, project?: string): Rec {
  const group = project || '';
  const places = item.places || {};
  // The answer already holds one place per row (r-ea17): a definition that
  // stands in both places is two items, one row each.
  const eff: Place = item.effective || (places.project ? 'project' : 'global');
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
  const hidden = (item.hidden || []).map((p) => ({
    place: p, path: places[p]?.path, hash: places[p]?.hash,
  }));
  const r = recOf({
    name: item.name || '', dom, project: group, place: eff, places: PLACES.filter((p) => places[p]),
    teamPath: place?.path || '', agreePath: agree.path || '',
    hash: place?.hash || '', agreeHash: agree.hash || '', broken: !!doc?.broken, problem: place?.problem || '', mark: item.mark || '',
    hidden,
    saved: cloneTeam(doc),
    agreeSaved: saved,
    // The saved document's own problems are the agreement save's baseline:
    // a save stops only when an error the saved text did not hold has
    // appeared (spec-teams X3).
    // The list's own size and budget, the same pair the figure's check
    // answers (d-00a9, r-2f83). A file that does not read has no size and
    // draws 大きさ —.
    size: (sum && sum.size) || 0, limit: (sum && sum.limit) || 0,
    problems: item.problems || { err: 0, warn: 0 }, drawMark: 0,
    draftRow: false, how: '', base: null, seeded: true,
    // The canvas's own picks for this row (webui-components 3 章): what is
    // picked, which column stands on the right, the blank space's menu, the
    // member name being typed, and the line that just lit up.
    ...startSelection(), menu: null, typed: null, flash: null,
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
// recordOfRef is the first row an address names: what the rules tab and the
// missing-row notice need, one row at a time.
const recordOfRef = (ref: string): Rec | null => {
  const one = parseOpen(ref)[0];
  return one ? rowsFor(one, recs.value, rowRef, here)[0] || null : null;
};
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
// groupRecs is the records the group draws: one per row of its own list answer
// (r-ea17).
export const groupRecs = (key: string) => recs.value.filter((r: Rec) => groupOfRecord(r) === key);

// rowsInGroup is the group's rows as its own list answer holds them: a group
// head counts the rows the answer drew, not the records the screen rebuilt
// (r-ea17, lead's answer 1).
export const rowsInGroup = (key: string): number => {
  if (HQ && HQ.teams && HQ.teams.length) {
    const g = HQ.teams.find((one) => (GROUPED ? groupKeyOf(one) : (one.global ? 'global' : '')) === key);
    return ((g && g.list && g.list.items) || []).length;
  }
  return (LIST.items || []).filter((item) => (item.effective === 'global' ? 'global' : '') === key || key === '' && item.effective !== 'global').length;
};

// ---- まだ無い定義（B2, B3） --------------------------------------------
// A definition that is not there yet is one row the screen holds rather than a
// saved file: the name the person is typing, the place it would go to, the
// definition it stands on, and what the server answered about it. The screen
// builds no document of its own; it draws the check's own answer (spec-teams
// B2, spec-boundary 5).
function draftRecord(how: string, place: Place, name: string, base: TeamBase | null | undefined, project?: string, held?: TeamEditing): Rec {
  const r = recOf({
    dom: '~new' + (++uids), draftRow: true, how, base: base || null, project: project || '',
    name: name || '', place: place || 'project', places: [],
    teamPath: '', agreePath: '', hash: '', agreeHash: '', broken: false, problem: '', mark: '',
    hidden: [],
    saved: cloneTeam(null),
    agreeSaved: null,
    problems: { err: 0, warn: 0 },
    size: 0, limit: 0,
    drawMark: 0, seeded: false,
    ...startSelection(), menu: null, typed: null, flash: null,
    lastAct: 0,
  }, held);
  seedPlaces(r);
  recs.value = [...recs.value, r];
  return r;
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

// Born is what 取り消し compares with: the name, the whole document a save would
// write, and the team's rules, as one snapshot (r-d480).
export const bornTouched = (r: Rec): boolean => r.editing.bornTouched.value;

// addDraft makes a row for a definition that is not there yet, checks it once,
// and draws it: its figure, its rules and its preview are the server's own
// reading of the base (spec-teams B2).
export async function addDraft(how: string, place: Place, name: string, base: TeamBase | null, project?: string): Promise<Rec> {
  const r = draftRecord(how, place, name, base, project);
  await checkTeam(r);
  if (!recs.value.includes(r)) return r;
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
  withOpen(r, true);
  setDtab(r.dom, 'shape');
  // The name a copy already holds is chosen, so the first letter a person
  // types replaces it rather than landing beside it (監査 R8).
  focusAsk.value = { dom: r.dom, sel: '.in-name' };
  return r;
}
// dropDraft takes a row that is not there yet away without writing anything
// (spec-teams B2).
export function dropDraft(r: Rec): void {
  r.editing.dispose();
  recs.value = recs.value.filter((x) => x !== r);
  withOpen(r, false);
  dropDtab(r.dom);
  resetCanvas(r);
}

const convProblems = (list: readonly Schemas['Problem'][] | null | undefined): Issue[] => (list || []).map((p) => ({
  sev: p.sev, k: 0,
  f: p.field || '', text: p.text || '', definition: p.definition || '',
}));
const buildCheck = (json: CheckAnswer): Check => {
  // The saved definition's answer carries the effect and the readers; the answer
  // for one that is not there yet carries its document instead (api-types). The
  // agreement's effect is the one this panel draws: its diff and its turns.
  const effect = json.effect && 'diff' in json.effect ? json.effect : null;
  return {
    templateSize: json.template_size || 0,
    issues: convProblems(json.problems || []),
    // The agreement's check answers the rule Markdown; the team part answers the
    // team.yaml preview instead, which is not this panel's.
    markdown: json.preview && !('diff' in json.preview) ? json.preview : { lines: [], chars: 0 },
    diff: (effect && effect.diff) || { added: [], removedBefore: [], removedAtEnd: false },
    readers: json.readers || [],
  };
};

// figFindings is the figure's problems as the panel draws them: the field the
// server names ("description", "members[1].profile", "workers[0]", "turns[2]", a
// rule) becomes the node, the worker or the word a row stands by. The server
// writes every sentence; this only says where it goes (spec-teams X1).
function figFindings(r: Rec, problems: readonly Schemas['Problem'][] | null | undefined): Finding[] {
  const t = r.draft.value;
  return (problems || []).map((p) => {
    const at = p.at || p.field || '';
    let node: string | null = null, worker: number | null = null, field = at, m;
    if ((m = /^members\[(\d+)\]/.exec(at))) { const mem = t.members[Number(m[1])]; node = mem ? mem.name : null; field = 'members'; }
    else if ((m = /^workers\[(\d+)\]/.exec(at))) { worker = Number(m[1]); field = 'workers'; }
    // A sentence about a node ("node:analyst") stands beside that point: the
    // graph's own problems reach the card and the figure that way (監査 R9).
    else if (at.startsWith('node:')) { node = at.slice('node:'.length); field = 'node'; }
    else if (at === 'description') { field = 'description'; }
    else if (at === 'name') { field = 'name'; }
    else { field = ''; }
    return { sev: p.sev, field, node, worker, text: p.text, role: p.role || '' };
  });
}
export const agreeIssues = (r: Rec): Issue[] => (r.check.value ? r.check.value.issues : []);
export const teamIssues = (r: Rec): Finding[] => r.findings.value;
const pcountList = (c: Schemas['TeamCounts'] | null | undefined): { sev: string }[] => [...Array((c && c.err) || 0)].map(() => ({ sev: 'err' })).concat([...Array((c && c.warn) || 0)].map(() => ({ sev: 'warn' })));
// rowIssues is the problems a row's own chip shows: the server's count for a
// saved definition, and the findings the server answered for a definition that
// is not there yet (spec-teams B2).
export const rowIssues = (r: Rec) => (r.draftRow ? teamIssues(r) : pcountList(r.problems.value));
// A definition that is not there yet counts as unsaved whatever it holds: it is
// written only when 保存 is pressed (spec-teams B2).
// A row whose team.yaml cannot be read is never a change: there is no document
// it would write (r-1e9a; seen when the list draws a broken row with ●).
export const dirtyDef = (r: Rec) => r.draftRow || r.editing.team.dirty.value || agreementChanged(r);
export const teamChanges = (r: Rec): number => r.editing.team.changes.value.length;
export const agreementChanged = (r: Rec): boolean => r.editing.agreement.read.value.saved ? r.editing.agreement.dirty.value : !!r.editing.agreement.read.value.draft.text;

// ownAgreement is whether a rule finding is about the team's own agreement:
// those are the team's promises and stand under チームの規則, while the rules of
// the role documents its members read stand under ロールの文書 (spec-teams 1.4.5,
// 1.7, 監査 R9).
export const vlistParts = (r: Rec) => {
  const issues = teamIssues(r);
  const out = issues.filter((m) => !!m.role);
  const own = issues.filter((m) => !m.role);
  const purposeOf = (i: number): string => {
    const w = r.draft.value.workers[i];
    return (w && w.purpose) || 'ワーカー';
  };
  const where = (m: Finding): string => {
    if (m.node) return nodeName(m.node);
    if (m.worker != null && m.worker >= 0) return purposeOf(m.worker);
    if (m.field === 'description') return '説明';
    if (m.field === 'name') return 'チーム定義名';
    return '';
  };
  const file = (m: Finding): string => m.role + '.md';
  // Role findings open the document in the same project as the team.
  const roleHref = (m: Finding): string => {
    if (!m.role) return '/roles';
    void ROLES_REV.value;
    const project = r.project && r.project !== 'global' ? r.project : CURRENT;
    const group = (ROLES_BY[project] || []).some((role) => role.name === m.role) ? project : 'global';
    const q = new URLSearchParams({ open: token(m.role, group) });
    if (project) q.set('project', project);
    return `/roles?${q}`;
  };
  return { issues, own, out, where, file, roleHref };
};

// ---- 検査と保存: delegate to the resource owner ------------------------
export function forgetChecks(r: Rec): void { r.editing.dispose(); }
export async function checkTeam(r: Rec) { await r.editing.checkTeam(); seedPlaces(r); r.drawMark.value++; }
export async function checkAgree(r: Rec) { await r.editing.checkAgreement(); }
export const scheduleAgree = (r: Rec) => { r.editing.agreement.dismissSaved(); debounce('a' + dkey(r), () => syncAgree(r)); };

export function syncTeam(r: Rec) {
  checkTeam(r);
}
export function syncAgree(r: Rec) {
  checkAgree(r);
}

// A role save changes the inputs of both the saved and draft team checks.
// Keep the canvas records and operations while refreshing their baselines.
export async function refreshAfterRoleSave(): Promise<void> {
  apiSaved();
  await Promise.all(recs.value.map(async (r) => {
    await r.editing.refresh();
    if (r.editing.team.checkingError.value || r.editing.agreement.checkingError.value) throw new Error('ロールは保存しましたが、チーム定義を再検査できませんでした。文書を読み直してください');
  }));
  const roleList = await apiList('role');
  ROLES.value = (roleList.items || []).map((item) => {
    const place = item.effective || 'project';
    return { name: item.name, title: item.places[place]?.doc?.title || '' };
  });
  if (HQ) {
    const groups = await hqList('roles');
    for (const g of groups.groups) ROLES_BY[groupKeyOf(g)] = (g.list?.items || []).map((item) => ({ name: item.name, title: item.places[item.effective || 'project']?.doc?.title || '' }));
  }
  bumpRoles();
}
export function pushOp(r: Rec, op: Op) { r.editing.act([op]); seedPlaces(r); }
export function setOps(r: Rec, ops: Op[]) { r.editing.replaceOperations(ops); seedPlaces(r); }
export async function saveTeam(r: Rec, overwrite = false) { return savePart(r, 'team', overwrite); }
async function savePart(r: Rec, part: 'team' | 'agreement', overwrite = false) {
  const fresh = r.draftRow, oldDom = r.dom, oldName = r.name.value, tab = dtabOf(r), opened = isOpenRow(r), group = groupOfRecord(r);
  const answer = await r.editing.save(part, overwrite);
  if (!answer?.json) return;
  const json = answer.json;
  if (fresh) {
    r.dom = domOf(r.name.value, r.project);
    r.places = [r.place.value];
    if (opened) { const open = new Set(state.value.open); for (const value of open) if (parseOpen(value)[0]?.name === oldName) open.delete(value); put({ open }); withOpen(r, true); }
    dropDtab(oldDom); setDtab(r.dom, tab);
    seedFiles(json.files || [], group);
    syncURL();
  }
  rebase(json, group); setProblems(json.items, r.project);
  if (part === 'team' || fresh) { resetCanvas(r); forgetUndo(r); }
  apiSaved();
  if (part === 'agreement') void checkAgree(r);
}
// rebase puts the files a save's answer read again under the rows' feet
// (r-b41c). A save gives the rules their ids after it writes (d-4aab), so the
// agreement a person did not touch can move while another file is saved: the row
// that kept the hash it read before would be refused at its own next save as a
// change another place made. The draft a person is writing in stays where it is.
//
// The answer names one project's files, and a definition of the same name can
// stand in another project: that row keeps its own file's hash, which the answer
// did not read (r-57d8).
function rebase(answer: Answer<'team', 'save'> | null, group: string): void {
  for (const b of bases(answer)) {
    if (!b.name) continue;
    for (const r of recs.value) {
      if (r.draftRow || b.place !== r.place.value || b.name !== r.name.value) continue;
      if (groupOfRecord(r) !== group) continue;
      if (b.kind === 'agreement') { r.editing.agreement.rebase(b.hash); r.editing.agreementYaml.rebase(b.hash); }
      else if (b.kind === 'team' || b.kind === '') { r.editing.team.rebase(b.hash); r.editing.teamYaml.rebase(b.hash); }
    }
  }
}
export async function saveAgree(r: Rec, overwrite = false) { return savePart(r, 'agreement', overwrite); }
// itemInGroup is one item of a list answer as a grouped screen's group reads
// it: the answer already holds one place per row (r-ea17), so this only checks
// that the item stands in the group's place and leaves it as it is.
function itemInGroup(item: DefItem, group: string): DefItem {
  const place = groupPlace(group);
  if (item.places && item.places[place]) return item;
  return { ...item, places: {}, effective: place, hidden: [] };
}

// doDelete takes one definition's folder out of one place (spec-teams B6): its
// team.yaml and its agreement.md, and the directory itself when the two files
// were all it held. Every record of that name is read again from the answer,
// because the other place's reading may be the one in effect now.
export async function doDelete(r: Rec) {
  const json = (await r.editing.team.remove())?.json;
  if (!json) return;
  const name = r.name.value;
  // The row keeps its place in the list: taking one place's folder away leaves
  // the definition where a person was looking at it (spec-teams B6, N7).
  const at = firstIndexOf((x) => x.dom === r.dom);
  const gone = recs.value.filter((x) => x.dom === r.dom);
  recs.value = recs.value.filter((x) => x.dom !== r.dom);
  gone.forEach((x) => { resetCanvas(x); x.editing.dispose(); });
  setProblems(json.items, groupOfRecord(r));
  const item = (json.items || []).find((it) => it.name === name);
  if (item) {
    const nr = defOf(item, groupOfRecord(r));
    orderRows(nr, at);
    if (!state.value.dtab[nr.dom]) setDtab(nr.dom, 'shape');
  }
  syncURL();
}

// renamed replaces only the saved target; other rows keep their drafts.
export async function renamed(r: Rec, name: string, json: Schemas['TeamSaved']): Promise<void> {
  const old = r.name.value, group = groupOfRecord(r), tab = dtabOf(r), at = firstIndexOf((x) => x === r);
  forgetChecks(r);
  resetCanvas(r);
  withOpen(r, false);
  recs.value = recs.value.filter((x) => x !== r);
  const item = (json.items || []).find((x) => x.name === name);
  if (!item) throw new Error('保存したチーム定義を読めませんでした。画面を読み直してください');
  const next = defOf(itemInGroup(item, group), group);
  next.renameNotes.value = json.rename?.notes || [];
  if (GROUPS) {
    try {
      const fresh = await hqList('teams');
      for (const g of GROUPS) {
        const item = fresh.groups.find((x) => !!x.global === !!g.global && x.project?.path === g.project?.path);
        if (item) { g.overridden_by = item.overridden_by; g.list = item.list; }
      }
      put({});
    } catch { next.renameNotes.value = [...next.renameNotes.value, '置き場所の表示を更新できませんでした。画面を読み直してください']; }
  }

  orderRows(next, at);
  withOpen(next, true);
  setDtab(next.dom, tab);
  yfiles.value = yfiles.value.filter((f) => !(f.place === r.place.value && f.name === old));
  seedFiles((json.files || []).filter((f) => f.place === r.place.value && f.name === name), group);
  await checkTeam(next);
  if (next.agreeSaved.value) await checkAgree(next);
  syncURL();
  apiSaved();
}

export const renameDirty = (r: Rec): boolean => dirtyDef(r) || yfiles.value.some((f) => f.name === r.name.value && f.place === r.place.value && yamlChanged(f.file));
export function clearOk(r: Rec) { r.editing.team.dismissSaved(); r.editing.agreement.dismissSaved(); }
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
  // A newer operation makes the notice's 取り消す stale: it would take back this
  // one, not the one the notice names (d-aeeb).
  dropUndo();
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
  clearOk(r);
  if (ctyping.key !== k || ctyping.field !== field) {
    const label = { description: '説明を戻しました', 'm-name': 'メンバー名を戻しました', 'w-purpose': '目的を戻しました' }[field] || '元に戻しました';
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
  clearOk(r);
  if (!st || !st.length) return;
  const point = st.pop();
  if (!point) return;
  setOps(r, cloneOps(point.ops));
  ctyping = { key: null, field: null };
  checkTeam(r);
  // The notice offered this operation; it has now been taken back (⌘Z and the
  // notice's 取り消す are the same restore), so it closes (d-aeeb).
  dropUndo();
  canvasToast(r, point.label || '元に戻しました', false);
}
// The row the newest operation stands on: ⌘Z acts on it even when nothing is
// picked (監査 1-1).
export function undoRow() {
  return recs.value.filter((x) => isOpenRow(x) && (cundo.get(dkey(x)) || []).length)
    .sort((a, b) => (b.lastAct.value || 0) - (a.lastAct.value || 0))[0] || null;
}
// resetCanvas forgets the canvas's picks and places for one row (a save, a
// delete: the figure lays out afresh, d-a575).
export function resetCanvas(r: Rec): void {
  if (!r) return;
  select(r, { how: 'reset' });
  r.menu.value = null;
  r.flash.value = null;
  r.placed.value = {};
  r.moved.clear();
  seedPlaces(r);
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
export type YFile = { file: string; place: Place; name: string; kind: string; label: string; path: string; readonly hash: string; readonly text: string; editing: YamlEditing; problem?: string; mark?: string };
function fileView(f: Schemas['TeamFile'], group = ''): YFile {
  const r = recs.value.find((r) => r.name.value === f.name && r.place.value === f.place && (!group || groupOfRecord(r) === group));
  const editing = r ? f.kind === 'agreement' ? r.editing.agreementYaml : r.editing.teamYaml : editYaml('team', { name: f.name || '', place: f.place, project: projectOfGroup(group || (f.place === 'global' ? 'global' : CURRENT)), text: f.text || '', hash: f.hash || '', part: f.kind, problems: f.problems || [] });
  return { file: `${f.name}.${f.place}.${f.kind}`, place: f.place, name: f.name || '', kind: f.kind || '', label: PLACE[f.place], path: f.path || '', editing,
    get hash() { return editing.read.value.hash; }, get text() { return editing.read.value.saved || ''; }, problem: (f as { problem?: string }).problem || '', mark: f.mark || '' };
}
export const yfiles = signal<YFile[]>([]);
function seedFiles(files: Schemas['TeamFile'][], group = ''): void {
  for (const f of files) { const next = fileView(f, group); const at = yfiles.value.findIndex((x) => x.file === next.file); yfiles.value = at < 0 ? [...yfiles.value, next] : yfiles.value.map((x, i) => i === at ? next : x); }
}
export const fileOf = (file: string): YFile | null => yfiles.value.find((f) => f.file === file) || null;
export const fileKeys = (): string[] => PLACES.flatMap((p) => yfiles.value.filter((f) => f.place === p).map((f) => f.file));
export const fileText = (file: string): string => fileOf(file)?.editing.read.value.draft || '';
export const yamlChanged = (file: string): boolean => fileOf(file)?.editing.dirty.value || false;
export const yIssues = (file: string): Schemas['Problem'][] => (fileOf(file)?.editing.read.value.problems || []).map((p) => ({ ...p }));
export function pickFile() { const keys = fileKeys(); if (!state.value.yfile || !keys.includes(state.value.yfile)) put({ yfile: keys[0] || null }); return state.value.yfile; }
export const chosenFile = () => { const keys = fileKeys(), y = state.value.yfile; return y && keys.includes(y) ? y : keys[0] || null; };
export function pressYfile(file: string) { put({ yfile: file }); syncURL(); }

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
    // The rows the screen has open, as the address names them: the row's name
    // and the place it stands in, never a bare name (d-a143).
    const rows = [...s.open]
      .map((one) => rowsFor(parseOpen(one)[0], recs.value, rowRef, here).find((r) => !r.draftRow))
      .filter((r): r is Rec => !!r);
    writeOpen(params, rows.map((r) => ({ name: r.name.value, group: urlGroup(r) })));
    const agrees = rows.filter((r) => dtabOf(r) === 'agree').map((r) => token(r.name.value, urlGroup(r)));
    if (agrees.length) params.set('agree', agrees.join(','));
  }
  const path = s.tab === 'yaml' ? '/teams/yaml' : '/teams';
  const q = params.toString();
  history.replaceState({}, '', q ? `${path}?${q}` : path);
}
export function readURL() {
  const p = new URLSearchParams(location.search);
  const refs = new Set(state.value.open);
  const missing: string[] = [];
  readOpen(p).forEach((ref) => {
    const rows = rowsFor(ref, recs.value, rowRef, here);
    if (!rows.length) missing.push(ref.name);
    // An address with no place opens both rows of that name: the old link names
    // a definition, and the screen cannot tell which side it meant (d-a143).
    rows.forEach((r) => refs.add(token(r.name.value, urlGroup(r))));
  });
  missingOpen.value = missing;
  put({ open: refs });
  const agree = p.get('agree');
  if (agree) {
    const m = { ...state.value.dtab };
    agree.split(',').filter(Boolean).forEach((ref) => {
      const r = recordOfRef(ref);
      m[!GROUPED && r ? r.dom : ref] = 'agree';
    });
    put({ dtab: m });
  }
  const rule = p.get('rule');
  put({ wantRule: null });
  if (rule) rule.split(',').filter(Boolean).forEach((spec) => {
    const i = spec.indexOf('.');
    if (i < 0) return;
    const r = byRealName(spec.slice(0, i));
    if (!r) return;
    withOpen(r, true);
    setDtab(r.dom, 'agree');
  });
  const file = p.get('file');
  if (file) put({ yfile: file });
  if (location.pathname === '/teams/yaml') put({ tab: 'yaml' });
}

// ---- 共有の部品（ruledoc）の受け口 ------------------------------------
export const rowOfPanel = (r: Rec) => ({ saved: r.agreeSaved.value, draft: r.agreeDraft.value });

// ---- 帯と、ページの外への受け渡し ------------------------------------------
// dirty is whether the screen holds anything the files do not: a row's ops, a
// rules draft, a YAML text. The band asks it through window.hergyPage (nav.js
// 1.5), the same way it asks the other screens. main.js keeps the band's dot on
// it.
export function dirty() {
  if (pendingRoleChanges() || roleEntries().some((e) => e.dirty.value)) return true;
  for (const r of recs.value) if (dirtyDef(r) || r.editing.read.value.renameDraft !== null) return true;
  for (const f of yfiles.value) if (yamlChanged(f.file)) return true;
  return false;
}
// lost names what leaving the page now would throw away: changed's words for
// each row that differs, in the order the screen holds them (r-1e9a, d-021e).
export function lost(): string[] {
  const out = roleEntries().flatMap((e) => e.dirty.value ? e.lost.value : []);
  for (const r of recs.value) { if (r.editing.read.value.renameDraft !== null) out.push('名前（保存先のファイルも変わる）'); if (r.editing.team.dirty.value) out.push(...r.editing.team.lost.value); if (agreementChanged(r)) out.push(...r.editing.agreement.lost.value); }
  for (const f of yfiles.value) if (f.editing.dirty.value) out.push(...f.editing.lost.value);
  return [...new Set(out)];
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
export type Carried = { roles?: import('../lib/editing.js').RoleCarry[]; resources?: TeamCarry[]; textFiles?: { file: string; carry: import('../lib/editing.js').CompleteCarry<string> }[]; teams?: { place: Place; group?: string; name: string; hash: string; ops: Op[] }[]; agrees?: { place: Place; group?: string; name: string; hash: string; doc: unknown }[]; files?: { file: string; hash: string; text: string | undefined }[] };
export function restore(carried: Carried) {
  restoreRoleEdits(carried.roles || []);
  for (const carry of carried.resources || []) {
    let r = recs.value.find((r) => r.editing.matches(carry));
    if (r) { r.editing.restore(carry); continue; }
    const target = carry.team.target, group = target.place === 'global' ? 'global' : target.project || '';
    if (!placeGroups().some((g) => (g.key || CURRENT) === (group || CURRENT))) continue;
    const editing = restoreTeam(resourceIdentity('team', { place: target.place, project: target.project }), carry);
    if (!editing) continue;
    r = draftRecord(carry.folder.how, target.place, carry.folder.name, null, GROUPED ? group : target.place === 'global' ? 'global' : '', editing);
    orderRows(r, groupStart(group)); withOpen(r, true);
  }
  for (const c of carried.teams || []) { const r = recs.value.find((r) => !r.draftRow && r.name.value === c.name && r.place.value === c.place && groupOfRecord(r) === (c.group || '')); if (r && (!c.hash || r.hash.value === c.hash)) setOps(r, c.ops); }
  for (const c of carried.agrees || []) { const r = recs.value.find((r) => !r.draftRow && r.name.value === c.name && r.place.value === c.place && groupOfRecord(r) === (c.group || '')); if (r && (!c.hash || r.agreeHash.value === c.hash)) r.editing.agreement.change(fromJSON(c.doc)); }
  for (const c of carried.textFiles || []) fileOf(c.file)?.editing.restore(c.carry);
  for (const c of carried.files || []) { const f = fileOf(c.file); if (f && (!c.hash || f.hash === c.hash) && c.text !== undefined) f.editing.change(c.text); }
}
export function take(): Carried { return { roles: takeRoleEdits(), resources: recs.value.filter((r) => dirtyDef(r) || r.editing.read.value.renameDraft !== null || r.editing.teamYaml.dirty.value || r.editing.agreementYaml.dirty.value).map((r) => r.editing.take()), textFiles: yfiles.value.filter((f) => f.editing.dirty.value && !recs.value.some((r) => r.editing.teamYaml === f.editing || r.editing.agreementYaml === f.editing)).map((f) => ({ file: f.file, carry: f.editing.take() })) }; }

// ---- 押し込み（部品が呼ぶ） --------------------------------------------
export function pressTab(t: 'list' | 'yaml') {
  if (state.value.tab === t) return;
  put({ tab: t });
  syncURL();
}
export function pressToggle(r: Rec) {
  withOpen(r, !isOpenRow(r));
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
  r.editing.starter('role');
  checkTeam(r);
}
// pressStarterProfile is pressStarterRoles for the starter profile.
export function pressStarterProfile(r: Rec) {
  r.editing.starter('profile');
  checkTeam(r);
}
// pressCarry asks for one of the names a copy can bring, or drops the ask: only
// the chosen ones are written with the definition (r-7e4a, d-dc81). The check is
// asked again, so the sentence about a name the global layer does not hold goes
// once the person chose it.
export function pressCarry(r: Rec, kind: 'role' | 'profile', name: string) {
  r.editing.chooseCarry(kind, name);
  checkTeam(r);
}
// baseOf is the definition a duplicate or a copy stands on, named the way the
// server reads it: the place it stands in, its name, and, for a row under a
// listed project's head, that project. Without the project the server would
// read the file of the project it was started in, and a copy of another
// project's row would have no base (r-a028).
const baseOf = (r: Rec, place: Place): TeamBase => {
  const group = groupOfRecord(r);
  return { place, name: r.name.value, project: projectOfGroup(place === 'global' ? 'global' : group || CURRENT) };
};
export async function pressDup(r: Rec) {
  await addDraft('dup', r.place.value, theName(r.name.value), baseOf(r, r.place.value), r.project);
}
export async function pressCopy(r: Rec) {
  const to = other(r.place.value);
  // A grouped screen draws each side as rows under its own head: the copy is a
  // row of the side it was copied to (r-f781).
  const group = copyGroupOf(to, groupOfRecord(r), GROUPED ? CURRENT : '');
  // The row the person pressed closes: the copy under the other place's head is
  // the one open row they read (d-022a).
  withOpen(r, false);
  const copy = await addDraft('copy', to, r.name.value, baseOf(r, r.place.value), group);
  // The copy is off the screen when the other place's head is below it: the row
  // comes to the band with its name chosen (d-022a).
  focusAsk.value = { dom: copy.dom, sel: '.in-name', scroll: true };
}
// pressPlace moves a definition that is not there yet to the other place. The
// row keeps its place in the list while its place changes (spec-teams B1, N4).
export async function pressPlace(r: Rec, target: Place) {
  if (!r.draftRow || r.place.value === target) return;
  const open = isOpenRow(r); withOpen(r, false);
  const old = r.editing;
  r.editing = old.moveTo(resourceIdentity('team', { place: target, project: projectOfGroup(target === 'global' ? 'global' : r.project) }));
  old.dispose();
  if (open) withOpen(r, true);
  resetCanvas(r); await checkTeam(r);
}
export function pressDiscard(r: Rec) {
  if (r) dropDraft(r);
}
export async function pressDelYes(r: Rec) { if (r) await doDelete(r); }
// pressName is a definition that is not there yet being named: the name is
// checked, and the files a save would write are read again: the server owns
// both (spec-teams B2). The field keeps its place and its cursor while the
// person types.
export function pressName(r: Rec, value: string) {
  // A row that is not there yet is addressed by the name it shows: the address
  // follows the letters a person types (⑥-2).
  const open = isOpenRow(r);
  withOpen(r, false);
  r.editing.changeName(value);
  if (open) withOpen(r, true);
  debounce('n' + r.dom, () => checkTeam(r));
}
// pressDescription is the row's own description field: it writes one operation and asks
// the check again (spec-teams 1.4.6).
export function pressDescription(r: Rec, value: string) {
  const text = value.replace(/\n/g, ' ');
  r.editing.act([teamActions.description(text)]);
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
export function pressTSave(r: Rec) { saveTeam(r); }
// ---- a save refused because the file moved (r-3c77, d-a34a) ---------------
// pressOverwrite writes what the person wrote over the file the answer showed:
// the save carries the hash of that file, so a file that moved again is refused
// the same way instead of being written over unseen.
export function pressOverwrite(r: Rec, which: string) { if (which === 'a') void saveAgree(r, true); else void saveTeam(r, true); }
export function pressRevertConflict(r: Rec, which: string) { editorOf(r, which).readConflict(); seedPlaces(r); if (which === 'a') void checkAgree(r); else void checkTeam(r); }
export function pressUndoConflict(r: Rec, which: string) { editorOf(r, which).undoConflict(); seedPlaces(r); if (which === 'a') void checkAgree(r); else void checkTeam(r); }
export function pressTRevert(r: Rec, host?: HTMLElement | null) {
  // 元に戻す goes at once and leaves its 取り消す in place (d-79e3 の A).
  const ops = clone(r.ops.value) as typeof r.ops.value;
  runDanger('revertDraft', {
    run: () => { setOps(r, []); resetCanvas(r); forgetUndo(r); syncTeam(r); tellDirty(); },
    restore: () => { setOps(r, ops); syncTeam(r); tellDirty(); },
    said: '保存した内容に戻しました',
    host,
  });
}
export function pressSave(r: Rec) { saveAgree(r); }
export function pressRevert(r: Rec, host?: HTMLElement | null): void {
  // 元に戻す puts the document back to the one the row opened with: the saved
  // agreement, or, for a team that has none, the empty document it started from
  // (n-e3de).
  const before = clone(r.agreeDraft.value) as typeof r.agreeDraft.value;
  runDanger('revertDraft', {
    run: () => { r.editing.agreement.discard(); checkAgree(r); tellDirty(); },
    restore: () => { r.editing.agreement.change(before); checkAgree(r); tellDirty(); },
    said: '保存した内容に戻しました',
    host,
  });
}
export function pressToastUndo() { canvasUndoFromToast(); }
// pressVpick puts the cursor in a finding's own field, or picks what it names on
// the figure.
export function pressVpick(r: Rec, m: Finding & { nameField?: boolean }) {
  if (m.nameField) { focusAsk.value = { dom: r.dom, sel: '.in-name' }; return; }
  if (m.node) select(r, { how: 'field', what: { type: 'node', id: m.node } });
  else if (m.worker != null) select(r, { how: 'field', what: { type: 'worker', i: m.worker } });
}
// pressLine is a press on one line of the topology tab's YAML preview: it picks
// what the line stands for, the way a press on the canvas's own point does
// (spec-teams B7). A description line puts the cursor in the description field.
export type LinePress = { f?: string; node?: string; worker?: number; a?: string; b?: string };
export function pressLine(r: Rec, p: LinePress) {
  if (p.f === 'description') {
    select(r, { how: 'field', what: null });
    focusAsk.value = { dom: r.dom, sel: '[data-cv-f="description"]' };
    return;
  }
  if (p.node) select(r, { how: 'field', what: { type: 'node', id: p.node } });
  else if (p.worker != null) select(r, { how: 'field', what: { type: 'worker', i: p.worker } });
  else if (p.a) select(r, { how: 'field', what: { type: 'edge', k: [p.a, p.b].sort().join('|') } });
  else select(r, { how: 'field', what: null });
}

// ---- 立ち上げ ----------------------------------------------------------
export function boot() {
  if (HQ && HQ.teams && HQ.teams.length) {
    HQ.teams.forEach((g) => {
      const key = GROUPED ? groupKeyOf(g) : (g.global ? 'global' : '');
      rowsIn(g).forEach((item) => defOf(item, key));
    });
  } else {
    (LIST.items || []).forEach((item) => defOf(item, item.effective === 'global' ? 'global' : ''));
  }
  seedFiles(LIST.files || []);
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
// openAgree puts a row on its 規則 tab and asks the rules' check: the same
// step the tab's press takes, for a part that names the tab without a press.
export function openAgree(r: Rec) {
  withOpen(r, true);
  setDtab(r.dom, 'agree');
  checkAgree(r);
  syncURL();
}

// jumpToRule opens one rule of a team's own agreement and puts the cursor in
// its body (spec-teams 1.4.5, spec-roles 1.2.4, 監査 R12). The check's column
// reads through this, and so does the address.
export function jumpToRule(r: Rec, _k: number) {
  focusAsk.value = null;
  pressPmode(r, 'edit');
  focusAsk.value = { dom: r.dom, sel: '.prose-editor textarea' };
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
