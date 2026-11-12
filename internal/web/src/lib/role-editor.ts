import { check as apiCheck, save as apiSave } from './legacy-editing-api.js';
// One role file's check ordering and save state, shared by both editors.
import { signal } from './preact.js';
import { resource } from './resource.js';
import { Refusal } from './api.js';
import type { Answer, RawAnswer, Request, Schemas } from './api.js';
import type { components } from '../../../apicontract/api-types';
import type { EditableDocument, RuleIssue } from './ruledoc.js';

type RoleCheckData = Partial<Answer<'role', 'check'>>;
type RoleCheckReading = RawAnswer<RoleCheckData>;

function record(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isProblem(value: unknown): value is components['schemas']['Problem'] {
  if (!record(value)) return false;
  return (value.sev === 'err' || value.sev === 'warn')
    && typeof value.at === 'string'
    && typeof value.text === 'string'
    && (value.field === undefined || typeof value.field === 'string')
    && (value.line === undefined || typeof value.line === 'number')
    && (value.definition === undefined || typeof value.definition === 'string')
    && (value.role === undefined || typeof value.role === 'string');
}

function isProblemList(value: unknown): value is components['schemas']['Problem'][] {
  return Array.isArray(value) && value.every(isProblem);
}

function isRolePreviewLine(value: unknown): value is components['schemas']['RolePreviewLine'] {
  return record(value)
    && typeof value.text === 'string'
    && typeof value.part === 'string';
}

function isRolePreview(value: unknown): value is components['schemas']['RolePreview'] {
  return record(value)
    && typeof value.chars === 'number'
    && (value.lines === null || (Array.isArray(value.lines) && value.lines.every(isRolePreviewLine)));
}

function isRoleDiff(value: unknown): value is components['schemas']['RoleDiff'] {
  return record(value)
    && (value.added === null || (Array.isArray(value.added) && value.added.every((line) => typeof line === 'number')))
    && (value.removedBefore === null || (Array.isArray(value.removedBefore) && value.removedBefore.every((line) => typeof line === 'number')))
    && typeof value.removedAtEnd === 'boolean';
}

function isRoleReader(value: unknown): value is components['schemas']['RoleReader'] {
  return record(value) && typeof value.definition === 'string' && typeof value.member === 'string';
}

function isRoleEffect(value: unknown): value is components['schemas']['RoleEffect'] {
  return record(value)
    && isRoleDiff(value.diff);
}

function isRoleCheckData(value: unknown): value is RoleCheckData {
  if (!record(value)) return false;
  return (value.problems === undefined || isProblemList(value.problems))
    && (value.preview === undefined || isRolePreview(value.preview))
    && (value.effect === undefined || isRoleEffect(value.effect))
    && (value.readers === undefined || (Array.isArray(value.readers) && value.readers.every(isRoleReader)))
    && (value.size === undefined || typeof value.size === 'number')
    && (value.limit === undefined || typeof value.limit === 'number');
}

function roleCheckData(value: unknown): RoleCheckData {
  return isRoleCheckData(value) ? value : {};
}


export function roleEditor() {
  const saving = signal(false);
  const check = resource<RoleCheckReading, [Request<'role', 'check'>]>(async (signal, payload) => {
    try {
      const result = await apiCheck('role', payload, { signal });
      if (!Array.isArray(result.json?.problems)) throw new Error('ロールの検査結果を読めませんでした');
      return result;
    }
    catch (err) {
      if (err instanceof Refusal) {
        const json = roleCheckData(err.json);
        if (Array.isArray(json.problems)) return { ok: false, status: err.status, text: err.text, json };
      }
      throw err;
    }
  });
  async function save(name: string, payload: Schemas['RoleSaveRequest']) {
    if (saving.value) throw new Error('ロールを保存しています。完了をお待ちください');
    saving.value = true;
    try { return await apiSave('role', name, payload); }
    finally { saving.value = false; }
  }
  return { check, saving, save };
}

export function roleProblems(_doc: EditableDocument, list: Schemas['Problem'][]): RuleIssue[] {
  return list.map((p) => ({ sev: p.sev, k: 0,
    f: p.field || '', text: p.text, definition: p.definition || '' }));
}
