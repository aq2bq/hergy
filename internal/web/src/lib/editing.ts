// The only owner of an edited resource's saved value, draft, file basis,
// validation and recovery. Definitions and request constructors are private;
// screens can choose an editing operation, but cannot supply a request builder.
import { signal, computed } from './preact.js';
import { resource } from './resource.js';
import { check as apiCheck, save as apiSave, del as apiDel, problemText, Refusal, type Kind, type Request, type Answer, type RawAnswer, type Schemas } from './api.js';
import { conflictOf, type Conflict } from './conflict.js';
import { roleCheckData, type RoleCheckData } from './role-check.js';
import { resourceIdentity, type ResourceIdentity, type ResourceKind, type ResourceName } from './resource-identity.js';
type Field<T> = { key: string; label: string; get: (v: T) => unknown };
type Shape<T> = { fields: Field<T>[] };

// same is the one comparison: primitives by Object.is, arrays element by
// element, plain objects by their own keys. Nothing else is compared.
function same(a: unknown, b: unknown): boolean {
  if (Object.is(a, b)) return true;
  if (Array.isArray(a) || Array.isArray(b)) {
    if (!Array.isArray(a) || !Array.isArray(b) || a.length !== b.length) return false;
    return a.every((one, i) => same(one, b[i]));
  }
  if (typeof a !== 'object' || typeof b !== 'object' || a === null || b === null) return false;
  // A field whose value is undefined is no field at all: the value is handed to
  // the API as JSON, where undefined and missing are the same (r-1e9a).
  const ka = Object.keys(a).filter((key) => (a as Record<string, unknown>)[key] !== undefined);
  const kb = Object.keys(b).filter((key) => (b as Record<string, unknown>)[key] !== undefined);
  if (ka.length !== kb.length) return false;
  return ka.every((key) => key in (b as object) && same((a as Record<string, unknown>)[key], (b as Record<string, unknown>)[key]));
}

// snapshot is a value's copy, made when a thing is created or saved.
function snapshot<T>(v: T): T {
  if (Array.isArray(v)) return v.map((one) => snapshot(one)) as unknown as T;
  if (v && typeof v === 'object') {
    const out: Record<string, unknown> = {};
    for (const [key, one] of Object.entries(v as Record<string, unknown>)) out[key] = snapshot(one);
    return out as T;
  }
  return v;
}

// changed is whether the written value differs from the one held.
function dirty<T>(_shape: Shape<T>, from: T | null, to: T, readable = true): boolean {
  // A document that cannot be read is never a change: there is nothing it would
  // write, so a broken row carries no ● and no 未保存 (r-1e9a).
  if (!readable) return false;
  if (from === null || from === undefined) return to !== null && to !== undefined;
  return !same(from, to);
}

// diff is the fields that differ, in the table's order, with one 「そのほかの
// 変更」 when the value differs and no field in the table does.
function diff<T>(shape: Shape<T>, from: T | null, to: T, readable = true): Field<T>[] {
  if (!dirty(shape, from, to, readable)) return [];
  if (from === null || from === undefined) {
    const out = shape.fields.filter((f) => !empty(f.get(to)));
    return out.length ? out : [{ key: '*', label: 'そのほかの変更', get: () => undefined }];
  }
  const out = shape.fields.filter((f) => !same(f.get(from), f.get(to)));
  if (out.length === 0) return [{ key: '*', label: 'そのほかの変更', get: () => undefined }];
  return out;
}

// lost is what a revert or a discard would drop, in words.
function lost<T>(shape: Shape<T>, from: T | null, to: T, readable = true): string[] {
  return diff(shape, from, to, readable).map((f) => f.label);
}

function empty(v: unknown): boolean {
  if (v === null || v === undefined || v === '') return true;
  if (Array.isArray(v)) return v.length === 0;
  if (typeof v === 'object') return Object.keys(v as object).length === 0;
  return false;
}

// The shapes the screens keep. A field's label is what a person reads when the
// change is lost; it is not what decides the change.
type RoleDoc = { name: string; title: string; text: string };
const ROLE_DOC: Shape<RoleDoc> = { fields: [
  { key: 'name', label: '名前（保存先のファイルも変わる）', get: (v) => v.name },
  { key: 'title', label: '題名', get: (v) => v.title },
  { key: 'text', label: '本文', get: (v) => v.text },
] };

type TeamWritten = {
  description: string;
  turns: [string, string][];
  members: { name: string; role: string; profile: string }[];
  workers: { from: string; purpose: string; profile: string }[];
};
const TEAM: Shape<TeamWritten> = { fields: [
  { key: 'description', label: '説明', get: (v) => v.description },
  { key: 'turns', label: 'ターン', get: (v) => v.turns },
  { key: 'members', label: 'メンバー', get: (v) => v.members.map((m) => ({ name: m.name, role: m.role, profile: m.profile })) },
  { key: 'workers', label: 'ワーカー', get: (v) => v.workers.map((w) => ({ from: w.from, purpose: w.purpose, profile: w.profile })) },
] };

type Agreement = { text: string };
const TEAM_AGREEMENT: Shape<Agreement> = { fields: [
  { key: 'text', label: 'チームの規則', get: (v) => v.text },
] };

type ProfileWritten = { name: string; kind: string; args: string[]; env: { key: string; value: string }[] };
const PROFILE: Shape<ProfileWritten> = { fields: [
  { key: 'name', label: '名前（保存先のファイルも変わる）', get: (v) => v.name },
  { key: 'kind', label: 'kind', get: (v) => v.kind },
  { key: 'args', label: '引数', get: (v) => v.args },
  { key: 'env', label: '環境変数', get: (v) => v.env },
] };

type AssignPicks = { member: string; role: string; profile?: string };
type AssignWritten = { picks: AssignPicks[] };
const PROFILE_ASSIGN: Shape<AssignWritten> = { fields: [
  { key: 'picks', label: '割り当て', get: (v) => v.picks },
] };

type YamlWritten = { text: string };
const YAML_TEXT: Shape<YamlWritten> = { fields: [
  { key: 'text', label: '本文', get: (v) => v.text },
] };

// profileFlags is which fields of a profile differ, index by index for args and
// env, from the same value comparison the change test makes (r-1e9a). The count
// is the owner's diff.
type ProfileFlags = { name: boolean; kind: boolean; args: boolean[]; env: { key: boolean; value: boolean }[]; count: number };
function profileFlags(saved: ProfileWritten | null, draft: ProfileWritten, readable = true): ProfileFlags {
  const count = diff(PROFILE, saved, draft, readable).length;
  const out: ProfileFlags = { name: false, kind: false, args: [], env: [], count };
  if (!readable) return out;
  out.name = !saved || !same(saved.name, draft.name);
  out.kind = !saved || !same(saved.kind, draft.kind);
  draft.args.forEach((arg, i) => { out.args[i] = !saved || i >= saved.args.length || !same(saved.args[i], arg); });
  out.env = draft.env.map((pair, i) => ({
    key: !saved || i >= saved.env.length || !same(saved.env[i].key, pair.key),
    value: !saved || i >= saved.env.length || !same(saved.env[i].value, pair.value),
  }));
  return out;
}


declare const requestBrand: unique symbol;
export type EditedRequest<K extends Kind, A extends 'check' | 'save'> = Readonly<{
  kind: K; name: string; body: Request<K, A>; [requestBrand]: A;
}>;
// Only this private constructor can attach the brand. No method exports it.
function request<K extends Kind, A extends 'check' | 'save'>(kind: K, name: string, body: Request<K, A>): EditedRequest<K, A> {
  return { kind, name, body } as EditedRequest<K, A>;
}
export type ReadonlyDeep<T> = T extends string | number | boolean | bigint | symbol | null | undefined ? T : T extends (...args: never[]) => unknown ? T : T extends object ? { readonly [P in keyof T]: ReadonlyDeep<T[P]> } : T;
type Problem = Schemas['Problem'];
export type ProfileDraft = ProfileWritten;
type Basis<D> = { saved: D | null; draft: D; hash: string; text?: string; path?: string };
type Recovery<D> = { c: Conflict; undone?: boolean; before?: Basis<D> };
export type EditingState<D> = Basis<D> & {
  refusalAt?: 'name' | 'doc'; checked?: unknown; born: D; problems: Problem[]; baseProblems: Problem[]; refusal: string;
  conflict: Recovery<D> | null; rename: boolean; savedLine: boolean;
};
export type CarryTarget = { kind: ResourceKind; place: 'project' | 'global'; project?: string; name: string | null; part?: string; draftId?: string };
export type Carry<D> = { hash: string; draft: D; target?: CarryTarget; born?: D; saved?: D | null; rename?: boolean };
export type CompleteCarry<D> = Carry<D> & { target: CarryTarget; born: D; saved: D | null };
let nextDraft = 0;
// A carry outlives the page. Keep its sequence in the band's session storage
// so an editor created on the next page cannot reuse the carried editor's ID.
function newDraftId(restored?: string): string {
  const key = 'hergy.editing.draft-sequence';
  const number = (value: string | null): number => {
    const n = Number(value);
    return Number.isSafeInteger(n) && n > 0 ? n : 0;
  };
  try { nextDraft = Math.max(nextDraft, number(window.sessionStorage.getItem(key))); } catch { /* the band cannot carry drafts without storage either */ }
  if (restored?.startsWith('draft-')) nextDraft = Math.max(nextDraft, number(restored.slice(6)));
  const id = restored || `draft-${++nextDraft}`;
  try { window.sessionStorage.setItem(key, String(nextDraft)); } catch { /* IDs within this page still differ */ }
  return id;
}
type Definition<K extends ResourceKind, D, W> = {
  part?: string;
  origin?: (state: EditingState<D>) => string | null;
  busySave?: string;
  baseline?: (state: EditingState<D>) => D | null;
  identity: ResourceIdentity<K>; shape: Shape<W>; written: (draft: D) => W;
  check: (state: EditingState<D>) => Request<K, 'check'>;
  save: (state: EditingState<D>) => Request<K, 'save'>;
  name: (state: EditingState<D>) => string;
  // A profile's URL names the saved resource; a name refusal concerns the
  // proposed name in its draft instead.
  inputName?: (state: EditingState<D>) => string;
  recovered: (conflict: Conflict) => D | null;
  remove?: (state: EditingState<D>) => Promise<RawAnswer<Answer<K, 'del'>>>;
  saved: (answer: Answer<K, 'save'> | null, sent: D) => D;
};
export type Editing<K extends ResourceKind, D, W> = {
  readonly read: { readonly value: ReadonlyDeep<EditingState<D>> };
  readonly changes: { readonly value: ReturnType<typeof diff<W>> };
  readonly dirty: { readonly value: boolean };
  readonly lost: { readonly value: string[] };
  readonly bornLost: { readonly value: string[] };
  readonly loading: { readonly value: boolean };
  readonly saving: { readonly value: boolean };
  readonly checkingError: { readonly value: unknown };
  change: (draft: D | ((draft: D) => D)) => void;
  dismissSaved: () => void;
  renameUsers: (on: boolean) => void;
  check: () => Promise<void>;
  save: () => Promise<RawAnswer<Answer<K, 'save'>> | undefined>;
  remove: () => Promise<RawAnswer<Answer<K, 'del'>> | undefined>;
  overwrite: () => Promise<RawAnswer<Answer<K, 'save'>> | undefined>;
  discard: () => void;
  readConflict: () => void;
  undoConflict: () => void;
  take: () => CompleteCarry<D>;
  matches: (carry: Carry<D>) => boolean;
  restore: (carry: Carry<D>) => boolean;
  rebase: (hash: string) => void;
  dispose: () => void;
};
type Initial<D> = { saved: D | null; draft: D; hash: string; problems?: Problem[]; readable?: boolean; savedLine?: boolean; text?: string; path?: string; born?: D; draftId?: string };

// Read the common fields of the concrete operation answers; keeping these
// outside the generic editor lets the generated operation types stay intact.
function checkedProblems(answer: unknown): Problem[] | undefined {
  if (typeof answer !== 'object' || answer === null || !('problems' in answer)) return undefined;
  return answer.problems === null ? [] : Array.isArray(answer.problems) ? answer.problems : undefined;
}
function savedHash(answer: { hash?: string } | null): string { return answer?.hash || ''; }

function savedPath(answer: unknown, fallback?: string): string | undefined {
  return typeof answer === 'object' && answer !== null && 'path' in answer && typeof answer.path === 'string' ? answer.path : fallback;
}
function savedText(answer: { text?: string } | null, fallback: string): string { return typeof answer?.text === 'string' ? answer.text : fallback; }
function refusalAt(error: unknown): 'name' | 'doc' {
  if (typeof error !== 'object' || error === null || !('json' in error)) return 'doc';
  const json = error.json;
  return typeof json === 'object' && json !== null && 'at' in json && json.at === 'name' ? 'name' : 'doc';
}

type Control<K extends ResourceKind, D, W> = Editing<K, D, W> & {
  version: () => number;
  accept: (saved: D, hash: string, text: string, version: number, path?: string) => void;
  checkOriginal: () => Promise<void>;
  initialize: (saved: D, draft: D, hash: string, born: D) => void;
  notice: () => void;
  invalidateOriginal: () => void;
};
function editing<K extends ResourceKind, D, W>(definition: Definition<K, D, W>, initial: Initial<D>): Control<K, D, W> {
  const state = signal<EditingState<D>>({ saved: snapshot(initial.saved), draft: snapshot(initial.draft), hash: initial.hash, text: initial.text || '', path: initial.path || '', born: snapshot(initial.born ?? initial.draft), problems: snapshot(initial.problems || []),
    baseProblems: snapshot(initial.problems || []), refusal: '', conflict: null, rename: true, savedLine: !!initial.savedLine });
  const draftId = (definition.origin ? definition.origin(state.value) === null : initial.saved === null) ? newDraftId(initial.draftId) : '';
  const target = (): CarryTarget => {
    const name = definition.origin ? definition.origin(state.value) : definition.identity.name;
    return { kind: definition.identity.kind, place: definition.identity.place, project: definition.identity.project, name, ...(definition.part ? { part: definition.part } : {}), ...(name === null ? { draftId } : {}) };
  };
  const matches = (carry: Carry<D>): boolean => {
    if (!carry.target) return true; // drafts written by the preceding UI version
    const expected = target(), got = carry.target;
    return expected.kind === got.kind && expected.place === got.place && expected.project === got.project && expected.name === got.name && expected.part === got.part && (expected.name !== null || expected.draftId === got.draftId);
  };
  let alive = true;
  let revision = 0;
  let saving = 0;
  let refusedName: string | undefined;
  let checkedName: string | undefined;
  const inputName = (s: EditingState<D>) => (definition.inputName || definition.name)(s);
  const pendingSaves = signal(0);
  const patch = (next: Partial<EditingState<D>>) => { state.value = { ...state.value, ...next }; };
  const written = () => definition.written(state.value.draft);
  const original = () => { const basis = definition.baseline ? definition.baseline(state.value) : state.value.saved; return basis === null ? null : definition.written(basis); };
  const readable = initial.readable !== false;
  const loadCheck = async (signal: AbortSignal, input: EditedRequest<K, 'check'>) => {
    try { return { json: (await apiCheck<K>(input.kind, input, { signal })).json as unknown }; }
    catch (error) {
      if (input.kind === 'role' && error instanceof Refusal) {
        const json = roleCheckData(error.json);
        if (Array.isArray(json.problems)) return { json };
      }
      throw error;
    }
  };
  const reading = resource(loadCheck);
  const baseReading = resource(loadCheck);
  let baseHash: string | undefined;
  let basePending: Promise<void> | undefined;
  let originalEpoch = 0;
  function forgetOriginal(): void {
    originalEpoch++;
    baseReading.abort(); baseHash = undefined; basePending = undefined;
  }
  async function checkOriginal(): Promise<void> {
    if (state.value.saved === null) return;
    if (baseHash === state.value.hash && basePending) return basePending;
    baseHash = state.value.hash;
    const epoch = originalEpoch;
    const basis = snapshot(state.value);
    basis.draft = snapshot(basis.saved!);
    basePending = (async () => {
      const answer = await baseReading.run(request<K, 'check'>(definition.identity.kind, definition.name(basis), definition.check(basis)));
      if (!alive || epoch !== originalEpoch || state.value.hash !== basis.hash) return;
      const problems = checkedProblems(answer?.json);
      if (problems !== undefined) patch({ baseProblems: snapshot(problems) });
      else { baseHash = undefined; basePending = undefined; }
    })();
    return basePending;
  }
  const change = (draft: D | ((draft: D) => D)) => {
    reading.abort();
    revision++;
    const next = typeof draft === 'function' ? (draft as (draft: D) => D)(snapshot(state.value.draft)) : draft;
    patch({ draft: snapshot(next), savedLine: false });
  };
  async function check(): Promise<void> {
    const current = revision;
    const input = request<K, 'check'>(definition.identity.kind, definition.name(state.value), definition.check(state.value));
    const answer = await reading.run(input);
    if (!alive || current !== revision || !answer?.json) return;
    const problems = checkedProblems(answer.json);
    if (problems === undefined) return;
    // Validation of an unchanged name cannot undo a write refusal (for
    // example, a destination occupied after copying). Only a checked change
    // of that name resolves the preceding name's refusal.
    checkedName = problems.some((p) => p.sev === 'err' && (p.field === 'name' || p.at === 'name')) ? undefined : inputName(state.value);
    const resolvedName = state.value.refusalAt === 'name' && checkedName !== undefined && checkedName !== refusedName;
    patch({ checked: snapshot(answer.json), ...(resolvedName ? { refusal: '', refusalAt: undefined } : {}), problems: snapshot(problems), ...(dirty(definition.shape, original(), written()) ? {} : { baseProblems: snapshot(problems) }) });
  }
  async function save(over?: string): Promise<RawAnswer<Answer<K, 'save'>> | undefined> {
    if (definition.busySave && pendingSaves.value > 0) { patch({ refusal: definition.busySave, refusalAt: 'doc' }); return undefined; }
    pendingSaves.value++;
    const mine = ++saving;
    const sent = snapshot(state.value);
    const sentName = inputName(sent);
    const current = revision;
    if (over !== undefined) sent.hash = over;
    patch({ refusal: '', refusalAt: undefined });
    try {
      const answer = await apiSave<K>(definition.identity.kind, request<K, 'save'>(definition.identity.kind, definition.name(sent), definition.save(sent)));
      if (!alive || mine !== saving) return undefined;
      forgetOriginal();
      const saved = definition.saved(answer.json, sent.draft);
      reading.abort();
      patch({ saved: snapshot(saved), draft: current === revision ? snapshot(saved) : state.value.draft,
        refusal: '', refusalAt: undefined, text: savedText(answer.json, sent.text || ''), hash: savedHash(answer.json) || sent.hash, path: savedPath(answer.json, sent.path), baseProblems: snapshot(sent.problems), conflict: null, savedLine: current === revision });
      return answer;
    } catch (error) {
      if (!alive || mine !== saving) return undefined;
      const c = conflictOf(error);
      // The person may already have checked a different name while this write
      // was pending. A refusal of the preceding name does not invalidate it.
      if (!c && refusalAt(error) === 'name' && inputName(state.value) !== sentName && checkedName === inputName(state.value)) return undefined;
      refusedName = sentName;
      patch(c ? { conflict: { c } } : { refusal: problemText(error, '保存できませんでした'), refusalAt: refusalAt(error) });
      return undefined;
    } finally { pendingSaves.value--; }
  }
  function readConflict(): void {
    const c = state.value.conflict?.c;
    if (!c) return;
    const draft = definition.recovered(c);
    if (draft === null) return;
    const before = snapshot({ saved: state.value.saved, draft: state.value.draft, hash: state.value.hash, text: state.value.text, path: state.value.path });
    forgetOriginal();
    change(draft);
    patch({ saved: snapshot(draft), hash: c.file.hash || '', text: c.file.text || '', path: c.file.path || state.value.path, refusal: '', conflict: { c, undone: true, before } });
  }
  return {
    invalidateOriginal: forgetOriginal,
    notice: () => patch({ savedLine: true }),
    initialize: (saved, draft, hash, born) => { forgetOriginal(); patch({ saved: snapshot(saved), draft: snapshot(draft), hash, born: snapshot(born) }); },
    version: () => revision,
    checkOriginal,
    accept: (saved, hash, text, version, path) => {
      saving++;
      reading.abort(); forgetOriginal();
      patch({ saved: snapshot(saved), draft: revision === version ? snapshot(saved) : state.value.draft, hash, text, path: path ?? state.value.path, conflict: null });
    },
    read: computed(() => snapshot(state.value) as ReadonlyDeep<EditingState<D>>),
    changes: computed(() => diff(definition.shape, original(), written(), readable)),
    dirty: computed(() => dirty(definition.shape, original(), written(), readable)),
    lost: computed(() => lost(definition.shape, original(), written(), readable)),
    bornLost: computed(() => lost(definition.shape, definition.written(state.value.born), written(), readable)),
    loading: computed(() => reading.loading.value || baseReading.loading.value), saving: computed(() => pendingSaves.value > 0), checkingError: computed(() => reading.error.value || baseReading.error.value),
    remove: async () => {
      if (!definition.remove) return undefined;
      patch({ refusal: '', refusalAt: undefined });
      const mine = ++saving;
      try {
        const answer = await definition.remove(snapshot(state.value));
        return alive && mine === saving ? answer : undefined;
      } catch (e) {
        if (alive && mine === saving) patch({ refusal: problemText(e, '削除できませんでした'), refusalAt: refusalAt(e) });
        return undefined;
      }
    },
    change, check, save: () => save(), overwrite: () => state.value.conflict ? save(state.value.conflict.c.file.hash || '') : Promise.resolve(undefined),
    dismissSaved: () => patch({ savedLine: false }),
    renameUsers: (rename) => patch({ rename }),
    discard: () => { change(state.value.saved ?? state.value.born); patch({ refusal: '', conflict: null }); },
    readConflict,
    undoConflict: () => { const before = state.value.conflict?.before; if (!before) return; forgetOriginal(); change(before.draft); patch({ saved: before.saved, hash: before.hash, text: before.text, path: before.path, conflict: null }); },
    matches,
    take: () => snapshot({ hash: state.value.hash, draft: state.value.draft, saved: state.value.saved, born: state.value.born, rename: state.value.rename, target: target() }),
    restore: (carry) => { if (!matches(carry) || (carry.hash && carry.hash !== state.value.hash)) return false; change(carry.draft); if (carry.rename !== undefined) patch({ rename: carry.rename }); if (state.value.saved === null && carry.born) patch({ born: snapshot(carry.born) }); return true; },
    rebase: (hash) => { if (hash !== state.value.hash) forgetOriginal(); patch({ hash }); },
    dispose: () => { alive = false; saving++; reading.abort(); baseReading.abort(); },
  };
}

const profileDoc = (d: ReadonlyDeep<ProfileDraft>): ProfileDraft => ({ name: d.name || '', kind: d.kind || '', args: [...(d.args || [])], env: (d.env || []).map((p) => ({ ...p })) });
export type ProfileEditing = Editing<'profile', ProfileDraft, ProfileDraft> & { readonly flags: { readonly value: ReturnType<typeof profileFlags> } };
export type SavedProfile = Omit<ProfileDraft, 'name'> & { name: ResourceName<'profile'> };
type ProfileInitial = Omit<Initial<ProfileDraft>, 'saved'> & { saved: SavedProfile | null };
export function editProfile(identity: ResourceIdentity<'profile'>, initial: ProfileInitial): ProfileEditing {
  const definition: Definition<'profile', ProfileDraft, ProfileDraft> = {
    identity, origin: (s) => s.saved?.name || null, shape: PROFILE, written: profileDoc,
    name: (s) => s.saved?.name || s.draft.name || '_',
    inputName: (s) => s.draft.name,
    check: (s) => ({ place: identity.place, project: identity.project, original: s.saved?.name || '', doc: profileDoc(s.draft) }),
    save: (s) => ({ place: identity.place, project: identity.project, original: s.saved?.name || '', doc: profileDoc(s.draft), hash: s.hash, rename_users: s.rename }),
    remove: (s) => apiDel('profile', s.saved?.name || s.draft.name, identity.place, s.hash, undefined, identity.project),
    recovered: (c) => c.file.doc ? profileDoc(c.file.doc as ProfileDraft) : null,
    saved: (_a, sent) => sent,
  };
  const one = editing(definition, initial);
  return { ...one, flags: computed(() => { const s = one.read.value; return profileFlags(s.saved ? profileDoc(s.saved) : null, profileDoc(s.draft), initial.readable !== false); }) };
}

export type ProfilePick = { key: string; name: string; role: string; profile: string };
export type ProfileAssignments = Editing<'team', ProfilePick[], { picks: { member: string; role: string }[] }> & { readonly rows: { readonly value: readonly Readonly<ProfilePick & { saved: string; changed: boolean }>[] } };
export function editProfileAssignments(identity: ResourceIdentity<'team'>, picks: ProfilePick[], hash: string): ProfileAssignments {
  const doc = (s: EditingState<ProfilePick[]>) => ({ ops: s.draft.filter((row) => {
    const saved = s.saved?.find((one) => one.key === row.key);
    return !saved || !same(saved.profile, row.profile);
  }).map((row) => row.key.startsWith('m') ? assignmentAction('memberProfile', row.name, row.profile) : assignmentAction('workerProfile', row.role, row.profile, row.name)) });
  const one = editing({ identity, shape: PROFILE_ASSIGN, written: (rows) => ({ picks: rows.map((r) => ({ member: r.key, role: r.profile })) }),
    name: () => identity.name || '', check: (s) => ({ place: identity.place, project: identity.project, name: identity.name || '', doc: doc(s) }),
    save: (s) => ({ place: identity.place, project: identity.project, doc: doc(s), hash: s.hash }), recovered: () => null, saved: (_a, sent) => sent,
  }, { saved: picks, draft: picks, hash });
  return { ...one, rows: computed(() => one.read.value.draft.map((row) => {
    const saved = one.read.value.saved?.find((r) => r.key === row.key)?.profile || '';
    return { ...row, saved, changed: !same(saved, row.profile) };
  })) };
}

export type YamlEditing = Editing<ResourceKind, string, { text: string }>;
function textEditing(kind: ResourceKind, input: { place: string; name: string; project?: string; text: string; hash: string; problems?: Problem[]; problem?: string; part?: Schemas['TeamCheckRequest']['part'] }, nameOf = () => input.name): Control<ResourceKind, string, { text: string }> {
  const identity = resourceIdentity(kind, input);
  const body = (s: EditingState<string>) => ({ place: identity.place, project: identity.project,
    ...(kind === 'profile' ? {} : { name: nameOf() }), text: s.draft, ...(input.part ? { part: input.part } : {}) });
  return editing({ identity, part: input.part, origin: () => nameOf(), shape: YAML_TEXT, written: (text) => ({ text }), name: nameOf,
    check: body, save: (s) => ({ ...body(s), hash: s.hash }), recovered: (c) => c.file.text || '', saved: (_a, sent) => sent,
  }, { saved: input.text, draft: input.text, hash: input.hash, problems: input.problems, readable: !input.problem });
}

export function editYaml(kind: ResourceKind, input: Parameters<typeof textEditing>[1]): YamlEditing { return textEditing(kind, input); }

// A profiles.yaml file is shared by every profile in its place, including new
// drafts. Bases are indexed by the typed place/project identity, never a label.
export function profileFiles() {
  const hashes = signal<Record<string, string>>({});
  const key = (id: ResourceIdentity<'profile'>) => id.place === 'global' ? 'global' : `project:${id.project || ''}`;
  return {
    hash: (id: ResourceIdentity<'profile'>): string => hashes.value[key(id)] || '',
    learn: (id: ResourceIdentity<'profile'>, hash: string): void => { if (hash) hashes.value = { ...hashes.value, [key(id)]: hash }; },
  };
}

export type RoleDraft = { name: string; title: string; text: string; __path?: string };
export type SavedRole = Omit<RoleDraft, 'name'> & { name: ResourceName<'role'> };
export type RoleCarry = CompleteCarry<RoleDraft> & { yaml?: Carry<string>; base?: ResourceIdentity<'role'> };
export type RoleEditing = Omit<Editing<'role', RoleDraft, RoleDraft>, 'take'> & {
  take: () => RoleCarry;
  readonly validation: { readonly value: ReadonlyDeep<RoleCheckData> };
  readonly yaml: YamlEditing;
  readonly blocked: { readonly value: boolean };
  readonly grew: { readonly value: readonly Problem[] };
  readonly nameChanged: { readonly value: boolean };
};
const roleDoc = (doc: ReadonlyDeep<RoleDraft>): RoleDraft => ({ name: doc.name, title: doc.title || '', text: doc.text });
export function editRole(identity: ResourceIdentity<'role'>, initial: Omit<Initial<RoleDraft>, 'saved'> & { saved: SavedRole | null; base?: ResourceIdentity<'role'>; roleContext?: 'project' | 'global' }): RoleEditing {
  let base = initial.base;
  const doc = editing<'role', RoleDraft, RoleDraft>({ identity, origin: (s) => s.saved?.name || null, busySave: 'ロールを保存しています。完了をお待ちください', shape: ROLE_DOC, written: roleDoc,
    baseline: (s) => s.saved || s.born,
    name: (s) => s.draft.name || s.saved?.name || '_',
    check: (s) => ({ place: identity.place, project: identity.project, original: s.saved?.name || null, doc: roleDoc(s.draft) }),
    save: (s) => ({ place: identity.place, project: identity.project, original: s.saved?.name || null, doc: roleDoc(s.draft), rename_users: s.rename, hash: s.hash,
      ...(base ? { base: { place: base.place, name: base.name || '', project: base.project } } : {}), ...(initial.roleContext ? { role_context: initial.roleContext } : {}) }),
    remove: (s) => apiDel('role', s.saved?.name || '', identity.place, s.hash, undefined, identity.project),
    recovered: (c) => c.file.doc ? readRole(c.file.doc) : null,
    saved: (answer, sent) => { base = undefined; return answer?.doc ? readRole(answer.doc) : sent; },
  }, initial);
  const yaml = textEditing('role', { place: identity.place, project: identity.project, name: identity.name || initial.draft.name,
    text: initial.text || '', hash: initial.hash, problem: initial.readable === false ? 'unreadable' : '' }, () => doc.read.value.saved?.name || doc.read.value.draft.name);
  async function saveDocument(overwrite = false) {
    const version = yaml.version();
    const answer = await (overwrite ? doc.overwrite() : doc.save());
    if (answer?.json) yaml.accept(answer.json.text ?? '', answer.json.hash || doc.read.value.hash, answer.json.text ?? '', version, answer.json.path);
    return answer;
  }
  async function saveText(overwrite = false) {
    const version = doc.version();
    const answer = await (overwrite ? yaml.overwrite() : yaml.save());
    const json = answer?.json;
    if (json && 'doc' in json && json.doc && 'text' in json.doc) { base = undefined; doc.accept(readRole(json.doc), json.hash || yaml.read.value.hash, json.text || '', version, json.path); }
    return answer;
  }
  const grew = computed(() => {
    const state = doc.read.value;
    const before = new Set(state.baseProblems.filter((p) => p.sev === 'err').map((p) => p.text));
    return state.problems.filter((p) => p.sev === 'err' && !before.has(p.text));
  });
  return { ...doc,
    take: () => ({ ...doc.take(), yaml: yaml.take(), ...(base ? { base: snapshot(base) } : {}) }),
    restore: (carry: RoleCarry | Carry<RoleDraft>) => {
      const restored = doc.restore(carry);
      const raw = 'yaml' in carry && carry.yaml ? yaml.restore(carry.yaml) : false;
      if (restored && 'base' in carry && carry.base) base = resourceIdentity('role', { place: carry.base.place, project: carry.base.project, name: carry.base.name || undefined });
      return restored || raw;
    },
    nameChanged: computed(() => !!doc.read.value.saved && doc.read.value.saved.name !== doc.read.value.draft.name),
    validation: computed(() => (doc.read.value.checked || {}) as RoleCheckData),
    grew,
    blocked: computed(() => doc.read.value.saved ? grew.value.length > 0 || !doc.dirty.value : doc.read.value.problems.some((p) => p.sev === 'err')),
    check: async () => { await Promise.all([doc.checkOriginal(), doc.check()]); },
    save: () => saveDocument(), overwrite: () => saveDocument(true),
    yaml: { ...yaml, save: () => saveText(), overwrite: () => saveText(true) },
    readConflict: () => { const version = yaml.dirty.value ? -1 : yaml.version(); doc.readConflict(); const s = doc.read.value; yaml.accept(s.text || '', s.hash, s.text || '', version, s.path); },
    undoConflict: () => { const version = yaml.dirty.value ? -1 : yaml.version(); doc.undoConflict(); const s = doc.read.value; yaml.accept(s.text || '', s.hash, s.text || '', version, s.path); },
    rebase: (hash) => { doc.rebase(hash); yaml.rebase(hash); },
    dispose: () => { doc.dispose(); yaml.dispose(); },
  };
}
export function restoreRole(identity: ResourceIdentity<'role'>, carry: RoleCarry): RoleEditing | undefined {
  if (carry.saved !== null || carry.target.name !== null || carry.hash !== '' || carry.target.kind !== 'role' || carry.target.place !== identity.place || carry.target.project !== identity.project || identity.name !== null) return undefined;
  const editing = editRole(identity, { saved: null, draft: carry.born, born: carry.born, draftId: carry.target.draftId, hash: '', text: '' });
  if (!editing.restore(carry)) { editing.dispose(); return undefined; }
  return editing;
}
function readRole(value: unknown): RoleDraft {
  if (typeof value !== 'object' || value === null) return { name: '', title: '', text: '' };
  return { name: 'name' in value && typeof value.name === 'string' ? value.name : '', title: 'title' in value && typeof value.title === 'string' ? value.title : '', text: 'text' in value && typeof value.text === 'string' ? value.text : '' };
}
export type RolePick = { name: string; saved: string; draft: string };
export type RoleAssignments = Editing<'team', { name: string; role: string }[], { picks: { member: string; role: string }[] }> & { readonly rows: { readonly value: readonly Readonly<RolePick & { changed: boolean }>[] }; renameRole: (from: string, to: string) => void };
export function editRoleAssignments(identity: ResourceIdentity<'team'>, picks: RolePick[], hash: string): RoleAssignments {
  const doc = (s: EditingState<{ name: string; role: string }[]>) => ({ ops: s.draft.filter((row) => {
    const saved = s.saved?.find((one) => one.name === row.name);
    return !saved || !same(saved.role, row.role);
  }).map((row) => (assignmentAction('memberRole', row.name, row.role))) });
  const one = editing({ identity, shape: PROFILE_ASSIGN, written: (rows: { name: string; role: string }[]) => ({ picks: rows.map((r) => ({ member: r.name, role: r.role })) }),
    name: () => identity.name || '', check: (s) => ({ place: identity.place, project: identity.project, name: identity.name || '', doc: doc(s) }),
    save: (s) => ({ place: identity.place, part: 'team', hash: s.hash, doc: doc(s) }), recovered: () => null, saved: (_a, sent) => sent,
  }, { saved: picks.map((r) => ({ name: r.name, role: r.saved })), draft: picks.map((r) => ({ name: r.name, role: r.draft })), hash });
  return { ...one, rows: computed(() => one.read.value.draft.map((row) => {
    const saved = one.read.value.saved?.find((r) => r.name === row.name)?.role || '';
    return { name: row.name, saved, draft: row.role, changed: !same(saved, row.role) };
  })), renameRole: (from, to) => {
    const state = one.read.value;
    const map = (rows: readonly { name: string; role: string }[]) => rows.map((r) => ({ ...r, role: r.role === from ? to : r.role }));
    if (state.saved) one.accept(map(state.saved), state.hash, '', -1);
    one.change(map(state.draft));
  } };
}

// A team's figure edits a document. These actions are intentions, not API
// payloads; only the definition below turns their resulting value into ops.
declare const actionBrand: unique symbol;
type TeamActionKind = 'description' | 'addTurn' | 'removeTurn' | 'addWorker' | 'removeWorker' | 'workerProfile' | 'workerPurpose' | 'memberRole' | 'memberProfile' | 'addMember' | 'removeMember' | 'renameMember' | 'moveWorker';
export type TeamAction = Readonly<{ kind: TeamActionKind } & Record<string, string>> & { readonly [actionBrand]: true };
function teamAction(kind: TeamActionKind, fields: Record<string, string>): TeamAction { return { kind, ...fields } as TeamAction; }
// These constructors are shared by the figure and both assignment screens.
export const teamActions = {
  description: (text: string) => teamAction('description', { text }),
  addTurn: (from: string, to: string) => teamAction('addTurn', { from, to }),
  removeTurn: (from: string, to: string) => teamAction('removeTurn', { from, to }),
  addMember: (member: string, role: string, profile: string) => teamAction('addMember', { member, role, profile }),
  removeMember: (member: string) => teamAction('removeMember', { member }),
  renameMember: (member: string, next: string) => teamAction('renameMember', { member, new: next }),
  memberRole: (member: string, role: string) => teamAction('memberRole', { member, role }),
  memberProfile: (member: string, profile: string) => teamAction('memberProfile', { member, profile }),
  addWorker: (from: string, purpose: string, profile: string) => teamAction('addWorker', { from, purpose, profile }),
  removeWorker: (from: string, purpose: string) => teamAction('removeWorker', { from, purpose }),
  workerProfile: (from: string, purpose: string, profile: string) => teamAction('workerProfile', { from, purpose, profile }),
  workerPurpose: (from: string, old: string, next: string) => teamAction('workerPurpose', { from, old, new: next }),
  moveWorker: (from: string, purpose: string, to: string) => teamAction('moveWorker', { from, purpose, to }),
};
export type TeamDocument = TeamWritten;
type TeamInput = { description?: string; members?: ReadonlyDeep<TeamDocument['members']> | null; workers?: ReadonlyDeep<TeamDocument['workers']> | null; turns?: readonly (readonly [string, string] | { readonly from: string; readonly to: string })[] | null };
export function teamDocument(t: TeamInput | null | undefined): TeamDocument {
  return { description: t?.description || '', members: (t?.members || []).map((m) => ({ ...m })), workers: (t?.workers || []).map((w) => ({ ...w })),
    turns: (t?.turns || []).map((x) => 'from' in x ? [x.from, x.to] : [x[0], x[1]]) };
}
function applyTeamActions(saved: ReadonlyDeep<TeamDocument>, actions: readonly TeamAction[]): TeamDocument {
  const t = teamDocument(saved);
  const hasTurn = (from: string, to: string) => t.turns.some(([a, b]) => a === from && b === to);
  for (const op of actions) switch (op.kind) {
    case 'description': t.description = op.text; break;
    case 'addTurn': if (!hasTurn(op.from, op.to)) t.turns.push([op.from, op.to]); break;
    case 'removeTurn': t.turns = t.turns.filter(([a, b]) => a !== op.from || b !== op.to); break;
    case 'addWorker': t.workers.push({ from: op.from, purpose: op.purpose, profile: op.profile }); break;
    case 'removeWorker': t.workers = t.workers.filter((w) => w.from !== op.from || w.purpose !== op.purpose); break;
    case 'workerProfile': { const w = t.workers.find((w) => w.from === op.from && w.purpose === op.purpose); if (w) w.profile = op.profile; break; }
    case 'workerPurpose': { const w = t.workers.find((w) => w.from === op.from && w.purpose === op.old); if (w) w.purpose = op.new; break; }
    case 'memberRole': { const m = t.members.find((m) => m.name === op.member); if (m) m.role = op.role; break; }
    case 'memberProfile': { const m = t.members.find((m) => m.name === op.member); if (m) m.profile = op.profile; break; }
    case 'addMember': if (!t.members.some((m) => m.name === op.member)) t.members.push({ name: op.member, role: op.role, profile: op.profile }); break;
    case 'removeMember': t.members = t.members.filter((m) => m.name !== op.member); t.turns = t.turns.filter(([a, b]) => a !== op.member && b !== op.member); t.workers = t.workers.filter((w) => w.from !== op.member); break;
    case 'renameMember': {
      const m = t.members.find((m) => m.name === op.member); if (m) m.name = op.new;
      t.turns = t.turns.map(([a, b]) => [a === op.member ? op.new : a, b === op.member ? op.new : b]);
      t.workers.forEach((w) => { if (w.from === op.member) w.from = op.new; }); break;
    }
    case 'moveWorker': { const w = t.workers.find((w) => w.from === op.from && w.purpose === op.purpose); if (w) w.from = op.to; break; }
  }
  return t;
}
// All three assignment views and the figure use this one operation definition.
function assignmentAction(kind: 'memberRole' | 'memberProfile' | 'workerProfile', target: string, value: string, purpose = ''): TeamAction {
  if (kind === 'memberRole') return teamActions.memberRole(target, value);
  if (kind === 'memberProfile') return teamActions.memberProfile(target, value);
  return teamActions.workerProfile(target, purpose, value);
}
function teamOperations(before: ReadonlyDeep<TeamDocument>, after: ReadonlyDeep<TeamDocument>): TeamAction[] {
  const out: TeamAction[] = [];
  if (before.description !== after.description) out.push(teamActions.description(after.description));
  // Membership/order changes rebuild the figure; no transient intermediate
  // state is checked or saved, and the result preserves the person's order.
  const rebuild = !same(before.members.map((m) => m.name), after.members.map((m) => m.name));
  if (rebuild) {
    before.turns.forEach(([from, to]) => out.push(teamActions.removeTurn(from, to)));
    before.workers.forEach((w) => out.push(teamActions.removeWorker(w.from, w.purpose)));
    before.members.forEach((m) => out.push(teamActions.removeMember(m.name)));
    after.members.forEach((m) => out.push(teamActions.addMember(m.name, m.role, m.profile)));
    after.workers.forEach((w) => out.push(teamActions.addWorker(w.from, w.purpose, w.profile)));
    after.turns.forEach(([from, to]) => out.push(teamActions.addTurn(from, to)));
    return out;
  }
  after.members.forEach((m, i) => {
    if (before.members[i].role !== m.role) out.push(assignmentAction('memberRole', m.name, m.role));
    if (before.members[i].profile !== m.profile) out.push(assignmentAction('memberProfile', m.name, m.profile));
  });
  const sameWorkers = same(before.workers.map((w) => [w.from, w.purpose]), after.workers.map((w) => [w.from, w.purpose]));
  if (sameWorkers) after.workers.forEach((w, i) => { if (before.workers[i].profile !== w.profile) out.push(assignmentAction('workerProfile', w.from, w.profile, w.purpose)); });
  else { before.workers.forEach((w) => out.push(teamActions.removeWorker(w.from, w.purpose))); after.workers.forEach((w) => out.push(teamActions.addWorker(w.from, w.purpose, w.profile))); }
  // Rebuild turns when their order changes too: order is part of team.yaml.
  if (!same(before.turns, after.turns)) {
    before.turns.forEach(([from, to]) => out.push(teamActions.removeTurn(from, to)));
    after.turns.forEach(([from, to]) => out.push(teamActions.addTurn(from, to)));
  }
  return out;
}
type TeamOptions = { name: string; fresh: boolean; how: string; base: ResourceIdentity<'team'> | null; seeded: boolean;
  starterRoles: boolean; starterProfile: boolean; carry: { roles: string[]; profiles: string[]; chosenRoles: string[]; chosenProfiles: string[] } | null;
  writes: string[]; hasRoles: boolean; hasProfiles: boolean; teamPath: string; agreementPath: string; renameDraft: string | null;
  born: { name: string; team: TeamDocument; agreement: string } | null };
export type TeamCarry = { actions: TeamAction[]; folder: TeamOptions; team: CompleteCarry<TeamDocument>; agreement: CompleteCarry<RoleDraft>; teamYaml: CompleteCarry<string>; agreementYaml: CompleteCarry<string> };
type TeamInitial = { name: string; team: TeamInput; hash: string; agreement: RoleDraft | null; agreementHash: string; teamText?: string; agreementText?: string;
  teamPath?: string; agreementPath?: string; how?: string; base?: ResourceIdentity<'team'> | null; seeded?: boolean; readable?: boolean; draftId?: string; agreementDraftId?: string };
export type TeamChanges = ReadonlyDeep<{
  members: Record<string, '' | 'new' | 'changed'>;
  workers: { from: string; purpose: string; added: boolean }[];
  turns: { from: string; to: string; added: boolean }[];
  list: ['a' | 'c' | 'd', string][];
}>;
function teamChanges(s: ReadonlyDeep<TeamDocument>, t: ReadonlyDeep<TeamDocument>): TeamChanges {
  const out: ['a' | 'c' | 'd', string][] = [];
  const hasTurn = (team: ReadonlyDeep<TeamDocument>, from: string, to: string) => team.turns.some(([a, b]) => a === from && b === to);
  const nodeName = (name: string) => name === 'human' ? '人間' : name;
  const sm = new Map(s.members.map((m) => [m.name, m])), cm = new Map(t.members.map((m) => [m.name, m]));
  if (s.description !== t.description) out.push(['c', `説明: ${t.description}`]);
  cm.forEach((m, n) => {
    if (!sm.has(n)) out.push(['a', `メンバー ${n}（${m.role} · ${m.profile}）`]);
    else {
      const o = sm.get(n);
      if (o && o.role !== m.role) out.push(['c', `${n} のロール ${o.role} → ${m.role}`]);
      if (o && o.profile !== m.profile) out.push(['c', `${n} のプロファイル ${o.profile} → ${m.profile}`]);
    }
  });
  sm.forEach((_m, n) => { if (!cm.has(n)) out.push(['d', `メンバー ${n}`]); });
  const sw = new Map(s.workers.map((w) => [w.from + '\u0000' + w.purpose, w])), cw = new Map(t.workers.map((w) => [w.from + '\u0000' + w.purpose, w]));
  cw.forEach((w, k) => {
    if (!sw.has(k)) out.push(['a', `ワーカー ${w.purpose || '（目的）'}（${w.from} · ${w.profile}）`]);
    else {
      const o = sw.get(k);
      if (o && (o.purpose !== w.purpose || o.profile !== w.profile || o.from !== w.from)) out.push(['c', `ワーカー ${w.purpose || '（目的）'}（${w.from} · ${w.profile}）`]);
    }
  });
  sw.forEach((w, k) => { if (!cw.has(k)) out.push(['d', `ワーカー ${w.purpose || '（目的）'}`]); });
  t.turns.forEach(([a, b]) => { if (!hasTurn(s, a, b)) out.push(['a', `ターン ${nodeName(a)} → ${nodeName(b)}`]); });
  s.turns.forEach(([a, b]) => { if (!hasTurn(t, a, b)) out.push(['d', `ターン ${nodeName(a)} → ${nodeName(b)}`]); });
  return {
    members: Object.fromEntries([...new Set(t.members.map((m) => m.name))].map((name) => {
      const m = t.members.find((one) => one.name === name)!;
      const before = s.members.find((one) => one.name === name);
      return [m.name, !before ? 'new' : before.role !== m.role || before.profile !== m.profile ? 'changed' : ''];
    })),
    workers: t.workers.map((w) => ({ from: w.from, purpose: w.purpose, added: !s.workers.some((one) => one.from === w.from && one.purpose === w.purpose) })),
    turns: t.turns.map(([from, to]) => ({ from, to, added: !hasTurn(s, from, to) })),
    list: out,
  };
}
export type TeamEditing = {
  readonly changes: { readonly value: TeamChanges };
  readonly read: { readonly value: ReadonlyDeep<TeamOptions> };
  readonly team: Editing<'team', TeamDocument, TeamDocument>;
  readonly agreement: Editing<'team', RoleDraft, { text: string }>;
  readonly teamYaml: YamlEditing; readonly agreementYaml: YamlEditing;
  readonly operations: { readonly value: readonly TeamAction[] };
  readonly validation: { readonly value: ReadonlyDeep<Answer<'team', 'check'>> };
  readonly agreementValidation: { readonly value: ReadonlyDeep<Answer<'team', 'check'>> };
  readonly grew: { readonly value: readonly Problem[] }; readonly agreementBlocked: { readonly value: boolean };
  readonly bornTouched: { readonly value: boolean }; readonly bornLost: { readonly value: string[] };
  act: (actions: readonly TeamAction[]) => void; replaceOperations: (actions: readonly TeamAction[]) => void;
  moveTo: (identity: ResourceIdentity<'team'>) => TeamEditing;
  changeName: (name: string) => void; renameInput: (name: string | null) => void;
  starter: (kind: 'role' | 'profile') => void; chooseCarry: (kind: 'role' | 'profile', name: string) => void;
  refresh: () => Promise<void>;
  checkTeam: () => Promise<void>; checkAgreement: () => Promise<void>;
  save: (part: 'team' | 'agreement', overwrite?: boolean) => Promise<RawAnswer<Answer<'team', 'save'>> | undefined>;
  checkRename: (name: string, signal: AbortSignal) => Promise<Schemas['TeamRenamePreview'] | null>;
  workerPreview: (worker: { from: string; purpose: string; name: string; task: string; runningTeam: string }, signal: AbortSignal) => Promise<Answer<'team', 'check'> | null>;
  promptInput: () => Omit<Schemas['PromptComparisonRequest'], 'member'>;
  take: () => TeamCarry; restore: (carry: TeamCarry) => boolean; matches: (carry: TeamCarry) => boolean; dispose: () => void;
};
export function editTeam(identity: ResourceIdentity<'team'>, input: TeamInitial): TeamEditing {
  const meta = signal<TeamOptions>({ name: input.name, fresh: !!input.how, how: input.how || '', base: input.base || null, seeded: input.seeded ?? !input.how,
    starterRoles: false, starterProfile: false, carry: null, writes: [], hasRoles: false, hasProfiles: false, teamPath: input.teamPath || '', agreementPath: input.agreementPath || '', renameDraft: null, born: null });
  const set = (next: Partial<TeamOptions>) => { meta.value = { ...meta.value, ...next }; };
  const scope = () => ({ place: identity.place, project: identity.project, name: meta.value.name });
  const creation = (): Pick<Request<'team', 'save'>, 'new' | 'starter_roles' | 'starter_profile' | 'base' | 'carry_roles' | 'carry_profiles'> => {
    const s = meta.value;
    if (!s.fresh) return {};
    if (s.how === 'new') return { new: true, starter_roles: s.starterRoles, starter_profile: s.starterProfile };
    return { base: s.base ? { place: s.base.place, name: s.base.name || '', project: s.base.project } : null,
      ...(s.carry ? { carry_roles: s.carry.chosenRoles, carry_profiles: s.carry.chosenProfiles } : {}) };
  };
  const origin = () => meta.value.fresh ? null : meta.value.name;
  const initialTeam = teamDocument(input.team);
  // Keep operation provenance as well as the written value. Rebuilding an
  // unchanged YAML node would lose its comments; ordinary edits keep the exact
  // semantic actions the person made. A replaced baseline uses a value delta.
  const actions = signal<TeamAction[]>([]);
  const operations = (s: ReadonlyDeep<EditingState<TeamDocument>>) => same(applyTeamActions(s.saved || initialTeam, actions.value), s.draft) ? snapshot(actions.value) : teamOperations(s.saved || initialTeam, s.draft);
  let agreement: Control<'team', RoleDraft, { text: string }>;
  const body = (s: EditingState<TeamDocument>): Request<'team', 'save'> => ({ ...scope(), ...creation(), part: meta.value.fresh ? 'folder' : 'team', hash: s.hash,
    ...(meta.value.fresh ? { agreement_hash: agreement.read.value.hash } : {}), doc: { ops: operations(s), ...(meta.value.fresh ? { doc: roleDoc(agreement.read.value.draft) } : {}) } });
  const team = editing<'team', TeamDocument, TeamDocument>({ identity, part: 'team', origin, shape: TEAM, written: snapshot, name: () => meta.value.name,
    check: (s) => ({ ...scope(), ...creation(), part: 'team', doc: { ops: operations(s) } }), save: body,
    remove: (s) => apiDel('team', meta.value.name, identity.place, s.hash, agreement.read.value.hash, identity.project),
    recovered: (c) => c.file.doc ? teamDocument(c.file.doc as TeamInput) : null,
    saved: (_answer, sent) => sent,
  }, { saved: initialTeam, draft: initialTeam, hash: input.hash, text: input.teamText, path: input.teamPath, readable: input.readable, draftId: input.draftId });
  const empty = { name: '', title: input.name, text: '' };
  agreement = editing<'team', RoleDraft, { text: string }>({ identity, part: 'agreement', origin, shape: TEAM_AGREEMENT, written: (d) => ({ text: d.text }), name: () => meta.value.name,
    check: (s) => ({ ...scope(), ...creation(), part: 'agreement', doc: { ops: operations(team.read.value), doc: roleDoc(s.draft) } }),
    save: (s) => ({ ...scope(), part: 'agreement', hash: s.hash, doc: { ops: operations(team.read.value), doc: roleDoc(s.draft) } }),
    recovered: (c) => c.file.doc ? readRole(c.file.doc) : null, saved: (answer, sent) => answer?.doc && 'text' in answer.doc ? readRole(answer.doc) : sent,
  }, { saved: input.agreement, draft: input.agreement || empty, born: input.agreement || empty, hash: input.agreementHash, text: input.agreementText, path: input.agreementPath, draftId: input.agreementDraftId });
  const teamYaml = textEditing('team', { ...scope(), text: input.teamText || '', hash: input.hash, part: 'team', problem: input.readable === false ? 'unreadable' : '' }, () => meta.value.name);
  const agreementYaml = textEditing('team', { ...scope(), text: input.agreementText ?? input.agreement?.text ?? '', hash: input.agreementHash, part: 'agreement' }, () => meta.value.name);
  const bornValue = () => ({ name: meta.value.name, team: teamDocument(team.read.value.draft), agreement: agreement.read.value.draft.text });
  const bornShape: Shape<ReturnType<typeof bornValue>> = { fields: [ { key: 'name', label: '名前（保存先のファイルも変わる）', get: (v) => v.name }, { key: 'team', label: 'チーム定義', get: (v) => v.team }, { key: 'agreement', label: 'チームの規則', get: (v) => v.agreement } ] };
  if (meta.value.seeded) set({ born: bornValue() });
  function changedOptions(next: Partial<TeamOptions>): void { set(next); team.change(teamDocument(team.read.value.draft)); }
  async function checkTeam(): Promise<void> {
    const version = team.version(), name = meta.value.name;
    await Promise.all([team.checkOriginal(), team.check()]);
    const json = team.read.value.checked as Answer<'team', 'check'> | undefined;
    if (!json || version !== team.version() || name !== meta.value.name || team.checkingError.value) return;
    if (meta.value.fresh && !meta.value.seeded && json.draft && json.doc) {
      const seed = teamDocument(json.doc), ag = json.doc.agreement;
      team.initialize(seed, applyTeamActions(seed, teamOperations(initialTeam, team.read.value.draft)), json.draft.hash || '', seed);
      if (ag?.doc) { const d = readRole(ag.doc); d.__path = ag.path; agreement.initialize(d, agreement.version() > 0 ? snapshot(agreement.read.value.draft) : d, ag.hash || '', d); }
      set({ seeded: true, born: meta.value.born || { name: meta.value.name, team: seed, agreement: ag?.doc?.text || '' } });
    }
    if (meta.value.fresh && json.draft) {
      const d = json.draft, previous = meta.value.carry, carry = d.carry;
      set({ writes: d.writes || [], hasRoles: !!d.has_roles, hasProfiles: !!d.has_profiles,
        teamPath: d.path || meta.value.teamPath, agreementPath: json.doc?.agreement?.path || meta.value.agreementPath,
        carry: carry ? { roles: carry.roles || [], profiles: carry.profiles || [], chosenRoles: (previous?.chosenRoles || []).filter((n) => carry.roles?.includes(n)), chosenProfiles: (previous?.chosenProfiles || []).filter((n) => carry.profiles?.includes(n)) } : null });
    }
  }
  async function save(part: 'team' | 'agreement', overwrite = false) {
    const sentActions = operations(team.read.value);
    const fresh = meta.value.fresh, sentName = meta.value.name, sentAgreement = snapshot(agreement.read.value.draft);
    const agVersion = agreement.version(), rawVersion = teamYaml.version(), agRawVersion = agreementYaml.version();
    const one = fresh || part === 'team' ? team : agreement;
    const answer = await (overwrite ? one.overwrite() : one.save());
    if (!answer?.json) return answer;
    const json = answer.json;
    if (fresh || part === 'team') actions.value = same(actions.value.slice(0, sentActions.length), sentActions) ? actions.value.slice(sentActions.length) : teamOperations(team.read.value.saved || initialTeam, team.read.value.draft);
    if (fresh) {
      const name = json.name || sentName;
      const place = json.items?.find((item) => item.name === name)?.places?.[identity.place];
      const ag = place?.doc?.agreement;
      agreement.accept(ag?.doc ? readRole(ag.doc) : sentAgreement, ag?.hash || '', ag?.doc?.text || sentAgreement.text, agVersion);
      agreementYaml.accept(ag?.doc?.text || sentAgreement.text, ag?.hash || '', ag?.doc?.text || sentAgreement.text, agRawVersion);
      if (part === 'agreement') { team.dismissSaved(); if (agreement.version() === agVersion) agreement.notice(); }
      set({ fresh: false, name, renameDraft: meta.value.name !== sentName ? meta.value.name : null, base: null, how: '', teamPath: place?.path || json.path || meta.value.teamPath, agreementPath: ag?.path || meta.value.agreementPath });
    } else set(part === 'team' ? { teamPath: json.path || meta.value.teamPath } : { agreementPath: json.path || meta.value.agreementPath });
    if (fresh || part === 'team') teamYaml.accept(json.text || '', team.read.value.hash, json.text || '', rawVersion);
    else agreementYaml.accept(json.text || '', agreement.read.value.hash, json.text || '', agRawVersion);
    return answer;
  }
  function rawEditor(raw: Control<ResourceKind, string, { text: string }>, part: 'team' | 'agreement'): YamlEditing {
    const one = part === 'team' ? team : agreement;
    const save = async (overwrite = false) => {
      const version = one.version(); const answer = await (overwrite ? raw.overwrite() : raw.save()); const json = answer?.json;
      if (json && 'doc' in json) {
        if (part === 'team' && json.doc && 'members' in json.doc) team.accept(teamDocument(json.doc), json.hash || '', json.text || '', version, json.path);
        if (part === 'agreement' && json.doc && 'text' in json.doc) agreement.accept(readRole(json.doc), json.hash || '', json.text || '', version, json.path);
      }
      return answer;
    };
    return { ...raw, save: () => save(), overwrite: () => save(true) };
  }
  function recover(part: 'team' | 'agreement', undo = false): void {
    const one = part === 'team' ? team : agreement, raw = part === 'team' ? teamYaml : agreementYaml;
    if (!one.read.value.conflict) return;
    const version = raw.dirty.value ? -1 : raw.version();
    if (undo) one.undoConflict(); else one.readConflict();
    const s = one.read.value;
    raw.accept(s.text || '', s.hash, s.text || '', version, s.path);
    if (s.path) set(part === 'team' ? { teamPath: s.path } : { agreementPath: s.path });
  }
  const grew = computed(() => {
    const before = new Set(team.read.value.baseProblems.filter((p) => p.sev === 'err').map((p) => p.text));
    return team.read.value.problems.filter((p) => p.sev === 'err' && !(p.at || p.field || '').startsWith('node:') && !before.has(p.text));
  });
  const take = (): TeamCarry => snapshot({ actions: operations(team.read.value), folder: meta.value, team: team.take(), agreement: agreement.take(), teamYaml: teamYaml.take(), agreementYaml: agreementYaml.take() });
  return { changes: computed(() => teamChanges(team.read.value.saved || initialTeam, team.read.value.draft)), read: computed(() => snapshot(meta.value)), team: { ...team, save: () => save('team'), overwrite: () => save('team', true), readConflict: () => recover('team'), undoConflict: () => recover('team', true) }, agreement: { ...agreement, save: () => save('agreement'), overwrite: () => save('agreement', true), readConflict: () => recover('agreement'), undoConflict: () => recover('agreement', true) }, teamYaml: rawEditor(teamYaml, 'team'), agreementYaml: rawEditor(agreementYaml, 'agreement'),
    operations: computed(() => operations(team.read.value)),
    validation: computed(() => (team.read.value.checked || {}) as Answer<'team', 'check'>), agreementValidation: computed(() => (agreement.read.value.checked || {}) as Answer<'team', 'check'>),
    grew, agreementBlocked: computed(() => { const s = agreement.read.value; const before = new Set(s.baseProblems.filter((p) => p.sev === 'err').map((p) => p.text)); return !s.saved && !s.draft.text || !agreement.dirty.value || s.problems.some((p) => p.sev === 'err' && (!s.saved || !before.has(p.text))); }),
    bornTouched: computed(() => meta.value.seeded ? !meta.value.born || dirty(bornShape, meta.value.born, bornValue()) : meta.value.name !== input.name || team.dirty.value || agreement.version() > 0), bornLost: computed(() => lost(bornShape, meta.value.born, bornValue())),
    act: (next) => { actions.value = [...operations(team.read.value), ...snapshot(next)]; team.change(applyTeamActions(team.read.value.draft, next)); },
    replaceOperations: (next) => { actions.value = snapshot([...next]); team.change(applyTeamActions(team.read.value.saved || initialTeam, next)); },
    moveTo: (target) => {
      if (!meta.value.fresh || target.name !== null) throw new Error('保存前のチーム定義だけ置き場所を変えられます');
      const next = editTeam(target, { name: meta.value.name, team: teamDocument(null), hash: '', agreement: null, agreementHash: '', how: meta.value.how, base: meta.value.base, seeded: false });
      const carry = next.take();
      carry.folder = { ...snapshot(meta.value), seeded: false, teamPath: '', agreementPath: '' };
      carry.actions = operations(team.read.value);
      carry.team.draft = applyTeamActions(teamDocument(null), carry.actions);
      carry.agreement.draft = snapshot(agreement.read.value.draft);
      next.restore(carry);
      return next;
    },
    changeName: (name) => changedOptions({ name }), renameInput: (renameDraft) => set({ renameDraft }),
    starter: (kind) => changedOptions(kind === 'role' ? { starterRoles: !meta.value.starterRoles } : { starterProfile: !meta.value.starterProfile }),
    chooseCarry: (kind, name) => { const carry = meta.value.carry; if (!carry) return; const key = kind === 'role' ? 'chosenRoles' : 'chosenProfiles'; changedOptions({ carry: { ...carry, [key]: carry[key].includes(name) ? carry[key].filter((n) => n !== name) : [...carry[key], name] } }); },
    refresh: async () => { team.invalidateOriginal(); agreement.invalidateOriginal(); await checkTeam(); await Promise.all([agreement.checkOriginal(), agreement.check()]); },
    checkTeam, checkAgreement: async () => { if (!agreement.read.value.saved && !agreement.read.value.draft.text) return; await Promise.all([agreement.checkOriginal(), agreement.check()]); }, save,
    checkRename: async (name, signal) => (await apiCheck('team', request('team', meta.value.name, { ...scope(), rename_to: name }), { signal })).json?.rename || null,
    workerPreview: async (worker, signal) => (await apiCheck('team', request('team', meta.value.name, { ...scope(), ...creation(), part: 'worker', hash: team.read.value.hash, running_team: worker.runningTeam,
      doc: { name: worker.name, purpose: worker.purpose, task: worker.task, from: worker.from, preview: true, ops: operations(team.read.value) } }), { signal })).json,
    promptInput: () => ({ ...scope(), ...creation(), part: 'team', hash: team.read.value.hash, doc: { ops: operations(team.read.value) } }),
    take, matches: (carry) => team.matches(carry.team),
    restore: (carry) => {
      if (!team.matches(carry.team)) return false;
      actions.value = snapshot(carry.actions || []);
      const restored = team.restore(carry.team), rules = agreement.restore(carry.agreement);
      teamYaml.restore(carry.teamYaml); agreementYaml.restore(carry.agreementYaml);
      if (restored) {
        if (meta.value.fresh) {
          const folder = snapshot(carry.folder);
          folder.base = folder.base ? resourceIdentity('team', { name: folder.base.name || undefined, place: folder.base.place, project: folder.base.project }) : null;
          set(folder);
        } else set({ renameDraft: carry.folder.renameDraft });
      }
      return restored || rules;
    },
    dispose: () => { team.dispose(); agreement.dispose(); teamYaml.dispose(); agreementYaml.dispose(); },
  };
}
export function restoreTeam(identity: ResourceIdentity<'team'>, carry: TeamCarry): TeamEditing | undefined {
  if (!carry.folder.fresh || identity.name !== null || carry.team.target.kind !== 'team' || carry.team.target.place !== identity.place || carry.team.target.project !== identity.project || carry.team.target.name !== null) return undefined;
  const e = editTeam(identity, { name: carry.folder.name, team: carry.team.saved || carry.team.born, hash: carry.team.hash, agreement: carry.agreement.saved, agreementHash: carry.agreement.hash,
    how: carry.folder.how, seeded: carry.folder.seeded, draftId: carry.team.target.draftId, agreementDraftId: carry.agreement.target.draftId });
  if (!e.restore(carry)) { e.dispose(); return undefined; }
  return e;
}
