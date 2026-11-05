// changed.ts — 保存していない変更があるかを決める、画面をまたぐ 1 つの持ち主
// （r-1e9a / ac-a058 / d-92f6）。
//
// 比べるのは「保存がそのまま API へ渡す値」（written）だけ。欄を手で選んで
// 比べない（lead の条件、バージョン 2）。したがって、保存が書く欄が増えても判定から
// 漏れない。値は snapshot で写し、same(a, b) という 1 つの比べ方で比べる。
//
// Field<T> の表は「何を失うか」の言葉と「N か所」の内訳にだけ使う。表に
// 無い欄で差が出たら「そのほかの変更」と 1 つ数え、合計は必ず written の差と
// 一致させる。名前の変更は「変えた」に含める（保存先のファイルも変わる）。

export type Field<T> = { key: string; label: string; get: (v: T) => unknown };
export type Shape<T> = { fields: Field<T>[] };

// same is the one comparison: primitives by Object.is, arrays element by
// element, plain objects by their own keys. Nothing else is compared.
export function same(a: unknown, b: unknown): boolean {
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
export function snapshot<T>(v: T): T {
  if (Array.isArray(v)) return v.map((one) => snapshot(one)) as unknown as T;
  if (v && typeof v === 'object') {
    const out: Record<string, unknown> = {};
    for (const [key, one] of Object.entries(v as Record<string, unknown>)) out[key] = snapshot(one);
    return out as T;
  }
  return v;
}

// changed is whether the written value differs from the one held.
export function dirty<T>(_shape: Shape<T>, from: T | null, to: T, readable = true): boolean {
  // A document that cannot be read is never a change: there is nothing it would
  // write, so a broken row carries no ● and no 未保存 (r-1e9a).
  if (!readable) return false;
  if (from === null || from === undefined) return to !== null && to !== undefined;
  return !same(from, to);
}

// diff is the fields that differ, in the table's order, with one 「そのほかの
// 変更」 when the value differs and no field in the table does.
export function diff<T>(shape: Shape<T>, from: T | null, to: T, readable = true): Field<T>[] {
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
export function lost<T>(shape: Shape<T>, from: T | null, to: T, readable = true): string[] {
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
export type RoleDoc = { name: string; title: string; text: string };
export const ROLE_DOC: Shape<RoleDoc> = { fields: [
  { key: 'name', label: '名前（保存先のファイルも変わる）', get: (v) => v.name },
  { key: 'title', label: '題名', get: (v) => v.title },
  { key: 'text', label: '本文', get: (v) => v.text },
] };

export type TeamWritten = {
  description: string;
  turns: [string, string][];
  members: { name: string; role: string; profile: string }[];
  workers: { from: string; purpose: string; profile: string }[];
};
export const TEAM: Shape<TeamWritten> = { fields: [
  { key: 'description', label: '説明', get: (v) => v.description },
  { key: 'turns', label: 'ターン', get: (v) => v.turns },
  { key: 'members', label: 'メンバー', get: (v) => v.members.map((m) => ({ name: m.name, role: m.role, profile: m.profile })) },
  { key: 'workers', label: 'ワーカー', get: (v) => v.workers.map((w) => ({ from: w.from, purpose: w.purpose, profile: w.profile })) },
] };

export type Agreement = { text: string };
export const TEAM_AGREEMENT: Shape<Agreement> = { fields: [
  { key: 'text', label: 'チームの規則', get: (v) => v.text },
] };

export type ProfileWritten = { name: string; kind: string; args: string[]; env: { key: string; value: string }[] };
export const PROFILE: Shape<ProfileWritten> = { fields: [
  { key: 'name', label: '名前（保存先のファイルも変わる）', get: (v) => v.name },
  { key: 'kind', label: 'kind', get: (v) => v.kind },
  { key: 'args', label: '引数', get: (v) => v.args },
  { key: 'env', label: '環境変数', get: (v) => v.env },
] };

export type AssignPicks = { member: string; role: string; profile?: string };
export type AssignWritten = { picks: AssignPicks[] };
export const PROFILE_ASSIGN: Shape<AssignWritten> = { fields: [
  { key: 'picks', label: '割り当て', get: (v) => v.picks },
] };

export type YamlWritten = { text: string };
export const YAML_TEXT: Shape<YamlWritten> = { fields: [
  { key: 'text', label: '本文', get: (v) => v.text },
] };

// teamNow is a worker as the saved file holds it, read back through the ops
// that changed its purpose or profile, or null when the draft added it: it is
// how a purpose typed one letter at a time still counts as one worker
// (spec-teams 1.4.6).
export type TeamOp = { kind: string } & Record<string, string>;
export type TeamWorker = { from: string; purpose: string; profile: string };
export function teamNow(w: TeamWorker, ops: TeamOp[]): TeamWorker | null {
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

// RoleEntryLike is one place's role document as the screens hold it: what the
// file saved, the draft the fields edit, and why the file cannot be read.
export type RoleEntryLike = { saved: RoleDoc | null; draft: RoleDoc; problem?: string };
const docOf = (d: RoleDoc): RoleDoc => ({ name: d.name || '', title: d.title || '', text: d.text || '' });
// roleDirty is whether the role's written document differs from the file's. A
// document that cannot be read is never a change (r-1e9a).
export function roleDirty(e: RoleEntryLike): boolean {
  return dirty(ROLE_DOC, e.saved ? docOf(e.saved) : null, docOf(e.draft), !e.problem);
}

// assignDirty is whether one member's role pick differs from the file's.
export function assignDirty(m: { name: string; saved: string; draft: string }): boolean {
  return dirty(PROFILE_ASSIGN, { picks: [{ member: m.name, role: m.saved }] }, { picks: [{ member: m.name, role: m.draft }] });
}
// yamlDirty is whether a YAML tab's text differs from the file's.
export function yamlDirty(text: string, typed: string | undefined): boolean {
  return dirty(YAML_TEXT, { text }, { text: typed === undefined ? text : typed });
}
