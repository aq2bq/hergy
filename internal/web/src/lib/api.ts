// lib/api.ts — the one way a screen talks to the server (spec-boundary 7, r-2cc2).
//
// Every kind of thing a screen reads and writes answers under /api/ in one shape
// (spec-boundary 5): a list, one document, a check, a save, a delete. This file
// holds the call, the token of this run, the shape each answer is read as, and
// the refusal a write can meet. A screen keeps its own look and draws a refusal
// as its own state — nothing here writes the page (webui-preact-design 5 章 1).
//
// Every screen reads the server through this file. `call` answers
// {ok, status, json, text}; the module's own `call` throws Refusal instead, so a
// screen can hold the refusal as a resource's error and draw it (5 章 2・3).
// The page carries this run's token wherever it writes; the activity screen
// holds it on its list, the others on #view (r-a8fa).
//
// The shapes are the API's written promise, not this file's: the answers and
// bodies come from internal/apicontract/api-types.d.ts, made from openapi.yaml
// (n-e3de, r-5a03), so a screen reads an answer's fields through the type the
// server is held to. Nothing here writes a field's name on its own.
import type { components, operations } from '../../../apicontract/api-types';

// Schemas is the shapes the API answers and takes, by their names in the promise.
export type Schemas = components['schemas'];

export const token = (document.querySelector('[data-token]') as HTMLElement | null)?.dataset?.token || '';
const enc = (s: string | number | null | undefined): string => encodeURIComponent(String(s == null ? '' : s));

// Place is where a document stands: the project's own layer or the shared one.
export type Place = 'project' | 'global';

// RawAnswer is one answer as it came: whether the server took it, its status,
// the JSON it wrote (null when it wrote none) and its text.
export type RawAnswer<T = unknown> = { ok: boolean; status: number; json: T | null; text: string };

// CallOptions is what a request may carry: the abort signal of the reading that
// asked for it, so a read that is no longer the screen's is dropped.
export type CallOptions = { signal?: AbortSignal };

// Refusal is the server saying it will not take this: the status it answered,
// the sentence it wrote (its text body, or json.text), and the answer behind it.
// A screen draws the sentence; it builds none of its own (spec-boundary 3).
export class Refusal extends Error {
  status: number;
  text: string;
  json: unknown;

  constructor(status: number, text: string, json: unknown) {
    super(text || `HTTP ${status}`);
    this.name = 'Refusal';
    this.status = status;
    this.text = text;
    this.json = json;
  }
}

// raw sends one request with the token of this run and answers what came back,
// whatever the status. A write carries its body as JSON, and the token rides in
// the header, so POST, PUT and DELETE are one shape (a DELETE carries no body at
// all). Nothing here decides what a screen does with a non-OK answer.
export async function raw<T = unknown>(method: string, url: string, payload?: unknown, { signal: stop }: CallOptions = {}): Promise<RawAnswer<T>> {
  const headers: Record<string, string> = { Accept: 'application/json' };
  if (token) headers['X-Hergy-Token'] = token;
  const opts: RequestInit = { method, credentials: 'same-origin', headers, signal: stop };
  if (payload !== undefined) {
    headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(payload);
  }
  const response = await fetch(url, opts);
  const text = await response.text();
  let json: T | null = null;
  if ((response.headers.get('content-type') || '').includes('application/json')) {
    try { json = JSON.parse(text) as T; } catch { json = null; }
  }
  return { ok: response.ok, status: response.status, json, text };
}

// call is one request whose refusal is an error: {status, json, text} when the
// server took it, a Refusal (with the sentence in it) when it did not.
export async function call<T = unknown>(method: string, url: string, payload?: unknown, opts?: CallOptions): Promise<RawAnswer<T>> {
  const a = await raw<T>(method, url, payload, opts);
  const refused = a.json as { text?: string } | null;
  if (!a.ok) throw new Refusal(a.status, (refused && refused.text) || a.text, a.json);
  return a;
}

// ---- 書き込みと読み込みの形（spec-boundary 5）------------------------------
// One place writes the addresses, so a screen asks for what it wants and not for
// an address. Each takes the request's AbortSignal last, so a resource can drop
// a read that is no longer the screen's (webui-preact-design 5 章 3).
const urls = {
  list: (kind: string) => `/api/${kind}`,
  // get is GET /api/{kind}/{name}?place=&project=: one document. project names
  // the listed project it stands in (d-73fc), empty for this one.
  get: (kind: string, name: string, place: string, project: string) => `/api/${kind}/${enc(name)}?${place ? `place=${enc(place)}&` : ''}${project ? `project=${enc(project)}` : ''}`,
  // del is DELETE /api/{kind}/{name}?place=&hash=&agreement_hash=&project=: take
  // one document away. The team kind takes a whole folder, so it carries the
  // second file's hash too (spec-teams B6).
  del: (kind: string, name: string, place: string, hash: string, agreementHash: string, project: string) => `/api/${kind}/${enc(name)}?place=${enc(place)}&hash=${enc(hash)}${agreementHash ? `&agreement_hash=${enc(agreementHash)}` : ''}${project ? `&project=${enc(project)}` : ''}`,
};

// What one operation answers and takes, read from the operation itself: a
// schema picked by hand can be the wrong one (deliveryCount answered the note
// beside the count), so every answer here comes from the operation that sends it.
type JsonOf<T> = T extends { content: { 'application/json': infer R } } ? R : never;
export type ApiAnswer<K extends keyof operations> = operations[K] extends { responses: { 200: infer R } } ? JsonOf<R> : never;
export type ApiBody<K extends keyof operations> = operations[K] extends { requestBody: infer B } ? JsonOf<B> : never;

// KindOps is the kind a screen names and the operations the promise writes for
// it, so `list('team')` answers the team list's own type.
type KindOps = {
  team: { list: 'listTeams'; get: 'getTeam'; check: 'checkTeam'; save: 'saveTeam'; del: 'deleteTeam' };
  role: { list: 'listRoles'; get: 'getRole'; check: 'checkRole'; save: 'saveRole'; del: 'deleteRole' };
  profile: { list: 'listProfiles'; get: 'getProfile'; check: 'checkProfile'; save: 'saveProfile'; del: 'deleteProfile' };
  activity: { list: 'listActivity'; get: 'getActivity'; check: 'checkActivity'; save: 'saveActivity'; del: 'deleteActivity' };
};
export type Kind = keyof KindOps;
type Action = 'list' | 'get' | 'check' | 'save' | 'del';
type Op<K extends Kind, A extends Action> = KindOps[K][A] & keyof operations;

// Answer is what one kind's operation answers with; Request is what it takes.
export type Answer<K extends Kind, A extends Action> = ApiAnswer<Op<K, A>>;
export type Request<K extends Kind, A extends Action> = ApiBody<Op<K, A>>;

// ListItem is one row of a kind's list answer, so a model reads a row by the
// operation that sent it and not by a schema picked here.
type ListItem<K extends Kind> = NonNullable<Answer<K, 'list'>['items']>[number];

// read is a GET whose json is the answer: a list or one document.
export async function read<T>(url: string, opts?: CallOptions): Promise<T> {
  const a = await call<T>('GET', url, undefined, opts);
  return a.json as T;
}

export const list = <K extends Kind>(kind: K, opts?: CallOptions): Promise<Answer<K, 'list'>> => read<Answer<K, 'list'>>(urls.list(kind), opts);
export const get = <K extends Kind>(kind: K, name: string, place?: Place | '', project?: string, opts?: CallOptions): Promise<Answer<K, 'get'>> =>
  read<Answer<K, 'get'>>(urls.get(kind, name, place || '', project || ''), opts);
// check is POST /api/{kind}/check: what is wrong with a draft or a text.
export const check = <K extends Kind>(kind: K, payload: Request<K, 'check'>, opts?: CallOptions): Promise<RawAnswer<Answer<K, 'check'>>> =>
  call<Answer<K, 'check'>>('POST', `/api/${kind}/check`, payload, opts);
// save is PUT /api/{kind}/{name}: write one document.
export const save = <K extends Kind>(kind: K, name: string, payload: Request<K, 'save'>, opts?: CallOptions): Promise<RawAnswer<Answer<K, 'save'>>> =>
  call<Answer<K, 'save'>>('PUT', `/api/${kind}/${enc(name)}`, payload, opts);
export const del = <K extends Kind>(kind: K, name: string, place: Place, hash: string, agreementHash?: string, project?: string, opts?: CallOptions): Promise<RawAnswer<Answer<K, 'del'>>> =>
  call<Answer<K, 'del'>>('DELETE', urls.del(kind, name, place, hash, agreementHash || '', project || ''), undefined, opts);

// ---- 変更を伝える (r-a8fa) -------------------------------------------------
// delivery is GET /api/delivery: the running members whose prompt is not the one
// hergy last handed them, by running team.
export const delivery = (opts?: CallOptions): Promise<ApiAnswer<'listDelivery'>> => read('/api/delivery', opts);
// deliveryCount is GET /api/delivery/count: the band's number alone.
export const deliveryCount = (opts?: CallOptions): Promise<ApiAnswer<'countDelivery'>> => read('/api/delivery/count', opts);
// deliver is POST /api/delivery: hand the current prompt to the named herdr
// members. A refusal comes back 422 with the one sentence in text.
export const deliver = (members: string[], opts?: CallOptions): Promise<RawAnswer<ApiAnswer<'deliver'>>> =>
  call<ApiAnswer<'deliver'>>('POST', '/api/delivery', { members } satisfies ApiBody<'deliver'>, opts);
// saved says one save wrote. Every screen's successful save says it, and the
// band reads its number again (deliver.js).
export const saved = (): boolean => document.dispatchEvent(new CustomEvent('hergy:saved'));

// problemText is the sentence one refusal carries, or the fallback word. The
// refusal is what call threw, and a screen holds it as a resource's error.
export const problemText = (refusal: unknown, fallback: string): string => {
  const one = refusal as { text?: string; json?: { text?: string } } | null;
  return (one && (one.text || (one.json && one.json.text))) || fallback;
};

// ---- reading an answer's shape (spec-boundary 5) ---------------------
// One place reads what the server answers; a screen asks for its kind's model
// and draws that. No screen digs into the fields of an answer itself.

const PLACES = ['project', 'global'] as const;
const PLACE_LABEL: Record<Place, string> = { project: 'プロジェクト', global: 'グローバル' };
const countOf = (c: { err?: number | null; warn?: number | null } | null | undefined): { err: number; warn: number } => ({ err: (c && c.err) || 0, warn: (c && c.warn) || 0 });
const jsonOf = (v: unknown): string => JSON.stringify(v || []);

// profileModel is GET /api/profile read as the profile screen's model: the rows
// of the list, the two files the YAML tab draws, and the files' hashes.
export function profileModel(list: Answer<'profile', 'list'> | null, openNames?: string[]) {
  const open = openNames || [];
  return {
    rows: ((list && list.items) || []).map((item) => profileRow(item, open.indexOf(item.name) >= 0)),
    files: ((list && list.files) || []).map((f) => ({
      file: f.place, label: PLACE_LABEL[f.place as Place] || f.label, path: f.path, text: f.text, hash: f.hash,
      problems: f.problems || [],
    })),
    hashes: (list && list.hashes) || {},
  };
}

// ProfilePanel is one place's profile as the row draws it: the document the
// place holds, what it saved, and the counts beside it.
export type ProfilePanel = {
  file: Place; label: string; path: string; saved: boolean;
  shadowed: boolean; both: boolean; other: Place; otherLabel: string;
  name: string; kind: string;
  args: string[]; env: Schemas['ProfileEnvPair'][];
  argsJSON: string; envJSON: string;
  savedName: string; savedKind: string; savedArgs: string; savedEnv: string;
  hash: string; users: number; renameUsers: number; err: number; warn: number;
  savedLine: boolean;
};

// ProfileRow is one item read as the row the list draws.
export type ProfileRow = {
  name: string; kind: string; kindClass: string;
  new: boolean; open: boolean;
  places: { label: string; state: string }[];
  args: string; env: string[]; more: number;
  users: number; roles: string; err: number; warn: number;
  chips: Schemas['ProfileChip'][]; panels: ProfilePanel[];
};

// profileRow is one item read as the row the list draws: each place becomes a
// panel, and the item's summary its columns.
function profileRow(item: ListItem<'profile'>, isOpen: boolean): ProfileRow {
  const places = item.places || {};
  const hidden = (item.hidden || []) as string[];
  const s = item.summary;
  const names = PLACES.filter((f) => places[f]);
  const both = names.length > 1;
  const panels = names.map((file): ProfilePanel => {
    const place = places[file] || {}, doc = place.doc || {};
    const other: Place = file === 'project' ? 'global' : 'project';
    const c = countOf(place.problems);
    return {
      file, label: PLACE_LABEL[file], path: place.path || '', saved: true,
      shadowed: hidden.indexOf(file) >= 0, both, other, otherLabel: PLACE_LABEL[other],
      name: doc.name || '', kind: doc.kind || '',
      args: doc.args || [], env: doc.env || [],
      argsJSON: jsonOf(doc.args), envJSON: jsonOf(doc.env),
      savedName: doc.name || '', savedKind: doc.kind || '',
      savedArgs: jsonOf(doc.args), savedEnv: jsonOf(doc.env),
      hash: place.hash || '', users: both ? 0 : (s.users || 0),
      renameUsers: place.readers || 0, err: c.err, warn: c.warn,
      savedLine: !!place.savedLine,
    };
  });
  const c = countOf(item.problems);
  return {
    name: item.name || '', kind: s.kind || '', kindClass: s.kindClass || '',
    new: false, open: !!isOpen,
    places: names.map((file) => ({ label: PLACE_LABEL[file], state: hidden.indexOf(file) >= 0 ? 'shadow' : 'on' })),
    args: s.args || '', env: s.env || [], more: s.more || 0,
    users: s.users || 0, roles: s.roles || '', err: c.err, warn: c.warn,
    chips: s.chips || [], panels,
  };
}

// HQRoleList is one group of the headquarters' role list (listHQRoles): the roles
// of one listed project, or of the shared layer.
export type HQRoleList = NonNullable<ApiAnswer<'listHQRoles'>['groups']>[number]['list'];

// RoleListLike is a role list as either entrance answers it: the project's own
// list (GET /api/role) and one group of the headquarters' reading. The two answer
// different documents (RoleDoc and HQRoleDocument) for the same rows, so the model
// reads the fields both hold.
export type RoleListLike = Answer<'role', 'list'> | HQRoleList;

// RolePlaceDoc is one place's role document as the screen holds it: the document
// the server answered and the file it came from.
export type RolePlaceDoc = (Schemas['RoleDoc'] | Schemas['HQRoleDocument']) & { hash: string; text: string; path: string };

// RoleRow is one role of the list: its two places, one of them the effective
// one, or the Markdown a role with no document is drawn from.
export type RoleRow = {
  uid: number;
  project: RolePlaceDoc | null;
  global: RolePlaceDoc | null;
  markdown?: string;
  name?: string;
  noDoc?: boolean;
  isNew?: boolean;
};

// roleModel is GET /api/role read as the role screen's model: the rows, each
// place holding the document behind it.
export function roleModel(list: RoleListLike | null): { roles: RoleRow[] } {
  return { roles: ((list && list.items) || []).map(roleRow) };
}

// RoleListRow is one row of either role list. The headquarters' list is one
// schema for three kinds, so its document is named as the union the promise
// writes; a role group's row holds a role document (see the cast below).
type RoleListRow = ListItem<'role'> | HQRoleList['items'][number];

function roleRow(item: RoleListRow, i: number): RoleRow {
  const places = item.places || {};
  // The headquarters answers one list shape for three kinds, so its row's
  // summary is the union of theirs; a roles group's row holds a role's.
  const s = item.summary as Schemas['RoleSummary'] | Schemas['HQRoleSummary'];
  const row: RoleRow = { uid: i + 1, project: null, global: null };
  if (s.markdown) {
    row.markdown = s.markdown;
    row.name = item.name || '';
    return row;
  }
  PLACES.forEach((l) => {
    const place = (places as Record<string, Schemas['RolePlace'] | Schemas['HQPlace'] | undefined>)[l];
    row[l] = (place && place.doc)
      ? Object.assign({}, place.doc as Schemas['RoleDoc'] | Schemas['HQRoleDocument'], { hash: place.hash || '', text: place.text || '', path: place.path || '' })
      : null;
  });
  if (s.noDoc) row.noDoc = true;
  if (s.isNew) row.isNew = true;
  return row;
}

// TeamCard is one team definition as the 割り当て tab holds it: the place its
// save writes, that file's hash, and the members' roles.
export type TeamCard = {
  id: string;
  hash: string;
  place: Place;
  turns: [string, string][];
  members: { name: string; saved: string; draft: string }[];
};

// teamModel is GET /api/team read as the role screen's 割り当て model: one team
// per definition, with the members' roles and the place it stands in. The hash
// a save carries is the file of that place: a global definition is saved in the
// global file, and every project reading it changes (d-0f32, r-f8b9).
export function teamModel(list: Answer<'team', 'list'> | null): { teams: TeamCard[] } {
  return {
    teams: ((list && list.items) || []).map((item) => {
      const places = item.places || {};
      const s = item.summary;
      const eff: Place = item.effective || (places.project ? 'project' : 'global');
      const place = places[eff] || {};
      const doc = place.doc;
      return {
        id: item.name || '', hash: place.hash || '', place: eff,
        turns: ((doc && doc.turns) || []).map((t): [string, string] => [t.from, t.to]),
        members: ((s && s.members) || []).map((m) => ({ name: m.name, saved: m.role || '', draft: m.role || '' })),
      };
    }),
  };
}

// CheckLine is one line of a preview: the words a person reads, where the line
// stands, and whether the change holds it.
export type CheckLine = { text: string; at?: string; part?: string; held?: string };

// CheckDiff is how a change moved the file's lines.
export type CheckDiff = { added: number[] | null; removedBefore: number[] | null; removedAtEnd?: boolean };

// CheckPreview is the preview a check answers with: the lines and the diff for
// team.yaml, the lines and the character count for a role (the shapes differ by
// kind — n-e3de's wobble, kept as it is).
export type CheckPreview = { lines?: CheckLine[] | null; diff?: CheckDiff; chars?: number };

// CheckEffectLine is one line of a prompt a change would move.
export type CheckEffectLine = { member?: string; part?: string; removed?: string[] | null; added?: string[] | null };

// CheckTurn is one turn a change would move.
export type CheckTurn = { from?: string; to?: string };

// CheckReader is one person a change reaches.
export type CheckReader = { definition?: string; member?: string; rules?: number; chars?: number; over?: boolean };

// CheckAnswer is a check's whole answer: the problems, the Markdown a person
// reads, how the change moved it, and the people it named.
export type CheckAnswer = {
  problems: Schemas['Problem'][];
  preview: CheckPreview | null;
  prompt: unknown | null;
  diff: CheckDiff | null;
  turns: CheckTurn[] | null;
  readers: CheckReader[];
  size: number;
  limit: number;
};

export function checkAnswer(a: RawAnswer<unknown> | null): CheckAnswer {
  const json = (a && a.json ? a.json : {}) as {
    problems?: Schemas['Problem'][];
    preview?: unknown;
    prompt?: unknown;
    effect?: { diff?: unknown; turns?: unknown };
    readers?: unknown;
    size?: number;
    limit?: number;
  };
  const effect = json.effect || {};
  return {
    problems: json.problems || [], preview: (json.preview as CheckPreview) || null, prompt: json.prompt || null,
    diff: (effect.diff as CheckDiff) || null, turns: (effect.turns as CheckTurn[]) || null, readers: (json.readers as CheckReader[]) || [],
    size: json.size || 0, limit: json.limit || 0,
  };
}
