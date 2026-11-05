import { signal } from '../lib/preact.js';

// View state only: hidden documents keep their mounted editors and drafts.
// A definition nobody picked documents for shows every document it has (r-e9a2),
// so the stored lists hold only the picks the person made.
export const ruleSelection = signal<Record<number, string[]>>({});
export const selectedRules = (uid: number, available: string[]): string[] => {
 const chosen = ruleSelection.value[uid];
 if (!chosen) return available;
 return chosen.filter((name) => available.includes(name));
};
export function selectRule(uid: number, name: string, available: string[], add = false): void {
 const current = selectedRules(uid, available);
 const next = add ? current.includes(name) ? current.filter(item => item !== name) : [...current, name] : [name];
 ruleSelection.value = { ...ruleSelection.value, [uid]: next.length ? next : ['agreement'] };
}
// showOnly is a name the person pointed at from elsewhere (the role column's
// ロールを確認・編集): it reads that one document, not the definition's default.
export function showOnly(uid: number, name: string): void {
 ruleSelection.value = { ...ruleSelection.value, [uid]: [name] };
}
