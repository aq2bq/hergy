// The only owner of an edited resource's saved value, draft, file basis,
// validation and recovery. Definitions and request constructors are private;
// screens can choose an editing operation, but cannot supply a request builder.
import { signal, computed } from './preact.js';
import { resource } from './resource.js';
import { check as apiCheck, save as apiSave, del as apiDel, problemText, type Kind, type Request, type Answer, type RawAnswer, type Schemas } from './api.js';
import { conflictOf, type Conflict } from './conflict.js';
import { PROFILE, PROFILE_ASSIGN, YAML_TEXT, dirty, diff, lost, snapshot, profileFlags, assignDirty, type Shape, type ProfileWritten } from './changed.js';
import { resourceIdentity, type ResourceIdentity, type ResourceKind } from './resource-identity.js';

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
type Basis<D> = { saved: D | null; draft: D; hash: string };
type Recovery<D> = { c: Conflict; undone?: boolean; before?: Basis<D> };
export type EditingState<D> = Basis<D> & {
  born: D; problems: Problem[]; baseProblems: Problem[]; refusal: string;
  conflict: Recovery<D> | null; rename: boolean; savedLine: boolean;
};
export type Carry<D> = { hash: string; draft: D };
type Definition<K extends ResourceKind, D, W> = {
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
  renameUsers: (on: boolean) => void;
  check: () => Promise<void>;
  save: () => Promise<RawAnswer<Answer<K, 'save'>> | undefined>;
  remove: () => Promise<RawAnswer<Answer<K, 'del'>> | undefined>;
  overwrite: () => Promise<RawAnswer<Answer<K, 'save'>> | undefined>;
  discard: () => void;
  readConflict: () => void;
  undoConflict: () => void;
  take: () => Carry<D>;
  restore: (carry: Carry<D>) => boolean;
  rebase: (hash: string) => void;
  rememberBorn: () => void;
  dispose: () => void;
};
type Initial<D> = { saved: D | null; draft: D; hash: string; problems?: Problem[]; readable?: boolean; savedLine?: boolean };

// Read the common fields of the concrete operation answers; keeping these
// outside the generic editor lets the generated operation types stay intact.
function checkedProblems(answer: { problems: Problem[] | null } | null): Problem[] { return answer?.problems || []; }
function savedHash(answer: { hash?: string } | null): string { return answer?.hash || ''; }

function editing<K extends ResourceKind, D, W>(definition: Definition<K, D, W>, initial: Initial<D>): Editing<K, D, W> {
  const state = signal<EditingState<D>>({ ...snapshot(initial), born: snapshot(initial.draft), problems: snapshot(initial.problems || []),
    baseProblems: snapshot(initial.problems || []), refusal: '', conflict: null, rename: true, savedLine: !!initial.savedLine });
  let alive = true;
  let revision = 0;
  let saving = 0;
  const patch = (next: Partial<EditingState<D>>) => { state.value = { ...state.value, ...next }; };
  const written = () => definition.written(state.value.draft);
  const original = () => state.value.saved === null ? null : definition.written(state.value.saved);
  const readable = initial.readable !== false;
  const reading = resource(async (signal, input: EditedRequest<K, 'check'>) => apiCheck<K>(input.kind, input, { signal }));
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
    patch({ problems: snapshot(problems), ...(dirty(definition.shape, original(), written()) ? {} : { baseProblems: snapshot(problems) }) });
  }
  async function save(over?: string): Promise<RawAnswer<Answer<K, 'save'>> | undefined> {
    const mine = ++saving;
    const sent = snapshot(state.value);
    const current = revision;
    if (over !== undefined) sent.hash = over;
    patch({ refusal: '' });
    try {
      const answer = await apiSave<K>(definition.identity.kind, request<K, 'save'>(definition.identity.kind, definition.name(sent), definition.save(sent)));
      if (!alive || mine !== saving) return undefined;
      const saved = definition.saved(answer.json, sent.draft);
      reading.abort();
      patch({ saved: snapshot(saved), draft: current === revision ? snapshot(saved) : state.value.draft,
        hash: savedHash(answer.json) || sent.hash, conflict: null, savedLine: current === revision });
      return answer;
    } catch (error) {
      if (!alive || mine !== saving) return undefined;
      const c = conflictOf(error);
      patch(c ? { conflict: { c } } : { refusal: problemText(error, '保存できませんでした') });
      return undefined;
    }
  }
  function readConflict(): void {
    const c = state.value.conflict?.c;
    if (!c) return;
    const draft = definition.recovered(c);
    if (draft === null) return;
    const before = snapshot({ saved: state.value.saved, draft: state.value.draft, hash: state.value.hash });
    change(draft);
    patch({ saved: snapshot(draft), hash: c.file.hash || '', refusal: '', conflict: { c, undone: true, before } });
  }
  return {
    read: computed(() => snapshot(state.value) as ReadonlyDeep<EditingState<D>>),
    changes: computed(() => diff(definition.shape, original(), written(), readable)),
    dirty: computed(() => dirty(definition.shape, original(), written(), readable)),
    lost: computed(() => lost(definition.shape, original(), written(), readable)),
    bornLost: computed(() => lost(definition.shape, definition.written(state.value.born), written(), readable)),
    loading: reading.loading, checkingError: reading.error,
    remove: async () => {
      if (!definition.remove) return undefined;
      patch({ refusal: '' });
      try { return await definition.remove(snapshot(state.value)); }
      catch (e) { patch({ refusal: problemText(e, '削除できませんでした') }); return undefined; }
    },
    change, check, save: () => save(), overwrite: () => state.value.conflict ? save(state.value.conflict.c.file.hash || '') : Promise.resolve(undefined),
    renameUsers: (rename) => patch({ rename }),
    discard: () => { change(state.value.saved ?? state.value.born); patch({ refusal: '', conflict: null }); },
    readConflict,
    undoConflict: () => { const before = state.value.conflict?.before; if (!before) return; change(before.draft); patch({ saved: before.saved, hash: before.hash, conflict: null }); },
    take: () => snapshot({ hash: state.value.hash, draft: state.value.draft }),
    restore: (carry) => { if (carry.hash && carry.hash !== state.value.hash) return false; change(carry.draft); return true; },
    rebase: (hash) => patch({ hash }),
    rememberBorn: () => patch({ born: snapshot(state.value.draft) }),
    dispose: () => { alive = false; saving++; reading.abort(); },
  };
}

const profileDoc = (d: ReadonlyDeep<ProfileDraft>): ProfileDraft => ({ name: d.name || '', kind: d.kind || '', args: [...(d.args || [])], env: (d.env || []).map((p) => ({ ...p })) });
export type ProfileEditing = Editing<'profile', ProfileDraft, ProfileDraft> & { readonly flags: { readonly value: ReturnType<typeof profileFlags> } };
export function editProfile(identity: ResourceIdentity<'profile'>, initial: Initial<ProfileDraft>): ProfileEditing {
  const definition: Definition<'profile', ProfileDraft, ProfileDraft> = {
    identity, shape: PROFILE, written: profileDoc,
    name: (s) => identity.name || s.draft.name || '_',
    check: (s) => ({ place: identity.place, project: identity.project, original: identity.name || '', doc: profileDoc(s.draft) }),
    save: (s) => ({ place: identity.place, project: identity.project, original: identity.name || '', doc: profileDoc(s.draft), hash: s.hash, rename_users: s.rename }),
    remove: (s) => apiDel('profile', identity.name || s.draft.name, identity.place, s.hash, undefined, identity.project),
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
export function editYaml(kind: ResourceKind, input: { place: string; name: string; project?: string; text: string; hash: string; problems?: Problem[]; problem?: string; part?: Schemas['TeamCheckRequest']['part'] }): YamlEditing {
  const identity = resourceIdentity(kind, input);
  const body = (s: EditingState<string>) => ({ place: identity.place, project: identity.project,
    ...(kind === 'profile' ? {} : { name: identity.name || input.name }), text: s.draft, ...(input.part ? { part: input.part } : {}) });
  return editing({ identity, shape: YAML_TEXT, written: (text) => ({ text }), name: () => input.name,
    check: body, save: (s) => ({ ...body(s), hash: s.hash }), recovered: (c) => c.file.text || '', saved: (_a, sent) => sent,
  }, { saved: input.text, draft: input.text, hash: input.hash, problems: input.problems, readable: !input.problem });
}

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
