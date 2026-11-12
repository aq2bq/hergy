// The only owner of an edited resource's saved value, draft, file basis,
// validation and recovery. Definitions and request constructors are private;
// screens can choose an editing operation, but cannot supply a request builder.
import { signal, computed } from './preact.js';
import { resource } from './resource.js';
import { check as apiCheck, save as apiSave, del as apiDel, problemText, Refusal, type Kind, type Request, type Answer, type RawAnswer, type Schemas } from './api.js';
import { conflictOf, type Conflict } from './conflict.js';
import { PROFILE, ROLE_DOC, PROFILE_ASSIGN, YAML_TEXT, dirty, diff, lost, snapshot, profileFlags, assignDirty, type Shape, type ProfileWritten } from './changed.js';
import { roleCheckData, type RoleCheckData } from './role-check.js';
import { resourceIdentity, type ResourceIdentity, type ResourceKind, type ResourceName } from './resource-identity.js';

declare const requestBrand: unique symbol;
export type EditedRequest<K extends Kind, A extends 'check' | 'save'> = Readonly<{
  kind: K; name: string; body: Request<K, A>; [requestBrand]: A;
}>;
// Only this private constructor can attach the brand. No method exports it.
function request<K extends Kind, A extends 'check' | 'save'>(kind: K, name: string, body: Request<K, A>): EditedRequest<K, A> {
  return { kind, name, body } as EditedRequest<K, A>;
}
export type ReadonlyDeep<T> = T extends (...args: never[]) => unknown ? T : T extends object ? { readonly [P in keyof T]: ReadonlyDeep<T[P]> } : T;
type Problem = Schemas['Problem'];
export type ProfileDraft = ProfileWritten;
type Basis<D> = { saved: D | null; draft: D; hash: string; text?: string };
type Recovery<D> = { c: Conflict; undone?: boolean; before?: Basis<D> };
export type EditingState<D> = Basis<D> & {
  refusalAt?: 'name' | 'doc'; checked?: unknown; born: D; problems: Problem[]; baseProblems: Problem[]; refusal: string;
  conflict: Recovery<D> | null; rename: boolean; savedLine: boolean;
};
export type CarryTarget = { kind: ResourceKind; place: 'project' | 'global'; project?: string; name: string | null; draftId?: string };
export type Carry<D> = { hash: string; draft: D; target?: CarryTarget; born?: D; saved?: D | null; rename?: boolean };
export type CompleteCarry<D> = Carry<D> & { target: CarryTarget; born: D; saved: D | null };
let nextDraft = 0;
type Definition<K extends ResourceKind, D, W> = {
  origin?: (state: EditingState<D>) => string | null;
  busySave?: string;
  baseline?: (state: EditingState<D>) => D | null;
  identity: ResourceIdentity<K>; shape: Shape<W>; written: (draft: D) => W;
  check: (state: EditingState<D>) => Request<K, 'check'>;
  save: (state: EditingState<D>) => Request<K, 'save'>;
  name: (state: EditingState<D>) => string;
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
type Initial<D> = { saved: D | null; draft: D; hash: string; problems?: Problem[]; readable?: boolean; savedLine?: boolean; text?: string; born?: D; draftId?: string };

// Read the common fields of the concrete operation answers; keeping these
// outside the generic editor lets the generated operation types stay intact.
function checkedProblems(answer: unknown): Problem[] | undefined {
  if (typeof answer !== 'object' || answer === null || !('problems' in answer)) return undefined;
  return answer.problems === null ? [] : Array.isArray(answer.problems) ? answer.problems : undefined;
}
function savedHash(answer: { hash?: string } | null): string { return answer?.hash || ''; }

function savedText(answer: { text?: string } | null, fallback: string): string { return typeof answer?.text === 'string' ? answer.text : fallback; }
function refusalAt(error: unknown): 'name' | 'doc' {
  if (typeof error !== 'object' || error === null || !('json' in error)) return 'doc';
  const json = error.json;
  return typeof json === 'object' && json !== null && 'at' in json && json.at === 'name' ? 'name' : 'doc';
}

type Control<K extends ResourceKind, D, W> = Editing<K, D, W> & {
  version: () => number;
  accept: (saved: D, hash: string, text: string, version: number) => void;
  checkOriginal: () => Promise<void>;
};
function editing<K extends ResourceKind, D, W>(definition: Definition<K, D, W>, initial: Initial<D>): Control<K, D, W> {
  const state = signal<EditingState<D>>({ saved: snapshot(initial.saved), draft: snapshot(initial.draft), hash: initial.hash, text: initial.text || '', born: snapshot(initial.born ?? initial.draft), problems: snapshot(initial.problems || []),
    baseProblems: snapshot(initial.problems || []), refusal: '', conflict: null, rename: true, savedLine: !!initial.savedLine });
  const draftId = initial.draftId || `draft-${++nextDraft}`;
  const target = (): CarryTarget => {
    const name = definition.origin ? definition.origin(state.value) : definition.identity.name;
    return { kind: definition.identity.kind, place: definition.identity.place, project: definition.identity.project, name, ...(name === null ? { draftId } : {}) };
  };
  const matches = (carry: Carry<D>): boolean => {
    if (!carry.target) return true; // drafts written by the preceding UI version
    const expected = target(), got = carry.target;
    return expected.kind === got.kind && expected.place === got.place && expected.project === got.project && expected.name === got.name && (expected.name !== null || expected.draftId === got.draftId);
  };
  let alive = true;
  let revision = 0;
  let saving = 0;
  let pendingSaves = 0;
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
    patch({ checked: snapshot(answer.json), refusal: state.value.refusalAt === 'name' && !problems.some((p) => p.sev === 'err' && p.field === 'name') ? '' : state.value.refusal, problems: snapshot(problems), ...(dirty(definition.shape, original(), written()) ? {} : { baseProblems: snapshot(problems) }) });
  }
  async function save(over?: string): Promise<RawAnswer<Answer<K, 'save'>> | undefined> {
    if (definition.busySave && pendingSaves > 0) { patch({ refusal: definition.busySave, refusalAt: 'doc' }); return undefined; }
    pendingSaves++;
    const mine = ++saving;
    const sent = snapshot(state.value);
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
        refusal: '', refusalAt: undefined, text: savedText(answer.json, sent.text || ''), hash: savedHash(answer.json) || sent.hash, baseProblems: snapshot(sent.problems), conflict: null, savedLine: current === revision });
      return answer;
    } catch (error) {
      if (!alive || mine !== saving) return undefined;
      const c = conflictOf(error);
      patch(c ? { conflict: { c } } : { refusal: problemText(error, '保存できませんでした'), refusalAt: refusalAt(error) });
      return undefined;
    } finally { pendingSaves--; }
  }
  function readConflict(): void {
    const c = state.value.conflict?.c;
    if (!c) return;
    const draft = definition.recovered(c);
    if (draft === null) return;
    const before = snapshot({ saved: state.value.saved, draft: state.value.draft, hash: state.value.hash, text: state.value.text });
    forgetOriginal();
    change(draft);
    patch({ saved: snapshot(draft), hash: c.file.hash || '', text: c.file.text || '', refusal: '', conflict: { c, undone: true, before } });
  }
  return {
    version: () => revision,
    checkOriginal,
    accept: (saved, hash, text, version) => {
      saving++;
      reading.abort(); forgetOriginal();
      patch({ saved: snapshot(saved), draft: revision === version ? snapshot(saved) : state.value.draft, hash, text, conflict: null });
    },
    read: computed(() => snapshot(state.value) as ReadonlyDeep<EditingState<D>>),
    changes: computed(() => diff(definition.shape, original(), written(), readable)),
    dirty: computed(() => dirty(definition.shape, original(), written(), readable)),
    lost: computed(() => lost(definition.shape, original(), written(), readable)),
    bornLost: computed(() => lost(definition.shape, definition.written(state.value.born), written(), readable)),
    loading: reading.loading, checkingError: reading.error,
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
    undoConflict: () => { const before = state.value.conflict?.before; if (!before) return; forgetOriginal(); change(before.draft); patch({ saved: before.saved, hash: before.hash, text: before.text, conflict: null }); },
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
export type ProfileAssignments = Editing<'team', ProfilePick[], { picks: { member: string; role: string }[] }>;
export function editProfileAssignments(identity: ResourceIdentity<'team'>, picks: ProfilePick[], hash: string): ProfileAssignments {
  const doc = (s: EditingState<ProfilePick[]>) => ({ ops: s.draft.filter((row) => {
    const saved = s.saved?.find((one) => one.key === row.key);
    return !saved || assignDirty({ name: row.key, saved: saved.profile, draft: row.profile });
  }).map((row) => row.key.startsWith('m') ? { kind: 'memberProfile', member: row.name, profile: row.profile } : { kind: 'workerProfile', from: row.role, purpose: row.name, profile: row.profile }) });
  return editing({ identity, shape: PROFILE_ASSIGN, written: (rows) => ({ picks: rows.map((r) => ({ member: r.key, role: r.profile })) }),
    name: () => identity.name || '', check: (s) => ({ place: identity.place, project: identity.project, name: identity.name || '', doc: doc(s) }),
    save: (s) => ({ place: identity.place, project: identity.project, doc: doc(s), hash: s.hash }), recovered: () => null, saved: (_a, sent) => sent,
  }, { saved: picks, draft: picks, hash });
}

export type YamlEditing = Editing<ResourceKind, string, { text: string }>;
function textEditing(kind: ResourceKind, input: { place: string; name: string; project?: string; text: string; hash: string; problems?: Problem[]; problem?: string; part?: Schemas['TeamCheckRequest']['part'] }, nameOf = () => input.name): Control<ResourceKind, string, { text: string }> {
  const identity = resourceIdentity(kind, input);
  const body = (s: EditingState<string>) => ({ place: identity.place, project: identity.project,
    ...(kind === 'profile' ? {} : { name: nameOf() }), text: s.draft, ...(input.part ? { part: input.part } : {}) });
  return editing({ identity, origin: () => nameOf(), shape: YAML_TEXT, written: (text) => ({ text }), name: nameOf,
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
};
const roleDoc = (doc: ReadonlyDeep<RoleDraft>): RoleDraft => ({ name: doc.name, title: doc.title || '', text: doc.text });
export function editRole(identity: ResourceIdentity<'role'>, initial: Omit<Initial<RoleDraft>, 'saved'> & { saved: SavedRole | null; base?: ResourceIdentity<'role'> }): RoleEditing {
  let base = initial.base;
  const doc = editing<'role', RoleDraft, RoleDraft>({ identity, origin: (s) => s.saved?.name || null, busySave: 'ロールを保存しています。完了をお待ちください', shape: ROLE_DOC, written: roleDoc,
    baseline: (s) => s.saved || s.born,
    name: (s) => s.draft.name || s.saved?.name || '_',
    check: (s) => ({ place: identity.place, project: identity.project, original: s.saved?.name || null, doc: roleDoc(s.draft) }),
    save: (s) => ({ place: identity.place, project: identity.project, original: s.saved?.name || null, doc: roleDoc(s.draft), rename_users: s.rename, hash: s.hash,
      ...(base ? { base: { place: base.place, name: base.name || '', project: base.project } } : {}) }),
    remove: (s) => apiDel('role', s.saved?.name || '', identity.place, s.hash, undefined, identity.project),
    recovered: (c) => c.file.doc ? readRole(c.file.doc) : null,
    saved: (answer, sent) => { base = undefined; return answer?.doc ? readRole(answer.doc) : sent; },
  }, initial);
  const yaml = textEditing('role', { place: identity.place, project: identity.project, name: identity.name || initial.draft.name,
    text: initial.text || '', hash: initial.hash, problem: initial.readable === false ? 'unreadable' : '' }, () => doc.read.value.saved?.name || doc.read.value.draft.name);
  async function saveDocument(overwrite = false) {
    const version = yaml.version();
    const answer = await (overwrite ? doc.overwrite() : doc.save());
    if (answer?.json) yaml.accept(answer.json.text ?? '', answer.json.hash || doc.read.value.hash, answer.json.text ?? '', version);
    return answer;
  }
  async function saveText(overwrite = false) {
    const version = doc.version();
    const answer = await (overwrite ? yaml.overwrite() : yaml.save());
    const json = answer?.json;
    if (json && 'doc' in json && json.doc && 'text' in json.doc) { base = undefined; doc.accept(readRole(json.doc), json.hash || yaml.read.value.hash, json.text || '', version); }
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
    validation: computed(() => (doc.read.value.checked || {}) as RoleCheckData),
    grew,
    blocked: computed(() => doc.read.value.saved ? grew.value.length > 0 || !doc.dirty.value : doc.read.value.problems.some((p) => p.sev === 'err')),
    check: async () => { await Promise.all([doc.checkOriginal(), doc.check()]); },
    save: () => saveDocument(), overwrite: () => saveDocument(true),
    yaml: { ...yaml, save: () => saveText(), overwrite: () => saveText(true) },
    readConflict: () => { const version = yaml.dirty.value ? -1 : yaml.version(); doc.readConflict(); const s = doc.read.value; yaml.accept(s.text || '', s.hash, s.text || '', version); },
    undoConflict: () => { const version = yaml.dirty.value ? -1 : yaml.version(); doc.undoConflict(); const s = doc.read.value; yaml.accept(s.text || '', s.hash, s.text || '', version); },
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
export type RoleAssignments = Editing<'team', { name: string; role: string }[], { picks: { member: string; role: string }[] }> & { renameRole: (from: string, to: string) => void };
export function editRoleAssignments(identity: ResourceIdentity<'team'>, picks: RolePick[], hash: string): RoleAssignments {
  const doc = (s: EditingState<{ name: string; role: string }[]>) => ({ ops: s.draft.filter((row) => {
    const saved = s.saved?.find((one) => one.name === row.name);
    return !saved || assignDirty({ name: row.name, saved: saved.role, draft: row.role });
  }).map((row) => ({ kind: 'memberRole', member: row.name, role: row.role })) });
  const one = editing({ identity, shape: PROFILE_ASSIGN, written: (rows: { name: string; role: string }[]) => ({ picks: rows.map((r) => ({ member: r.name, role: r.role })) }),
    name: () => identity.name || '', check: (s) => ({ place: identity.place, project: identity.project, name: identity.name || '', doc: doc(s) }),
    save: (s) => ({ place: identity.place, part: 'team', hash: s.hash, doc: doc(s) }), recovered: () => null, saved: (_a, sent) => sent,
  }, { saved: picks.map((r) => ({ name: r.name, role: r.saved })), draft: picks.map((r) => ({ name: r.name, role: r.draft })), hash });
  return { ...one, renameRole: (from, to) => {
    const state = one.read.value;
    const map = (rows: readonly { name: string; role: string }[]) => rows.map((r) => ({ ...r, role: r.role === from ? to : r.role }));
    if (state.saved) one.accept(map(state.saved), state.hash, '', -1);
    one.change(map(state.draft));
  } };
}
