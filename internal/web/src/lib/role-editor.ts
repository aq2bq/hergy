import { check as apiCheck, save as apiSave } from './legacy-editing-api.js';
// F3 compatibility for role editing inside the team definition screen.
import { signal } from './preact.js';
import { resource } from './resource.js';
import { Refusal } from './api.js';
import type { RawAnswer, Request, Schemas } from './api.js';

import { roleCheckData, type RoleCheckData } from './role-check.js';
type RoleCheckReading = RawAnswer<RoleCheckData>;

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

export { roleProblems } from './role-check.js';
