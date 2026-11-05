import { signal } from '../lib/preact.js';

// View state only: hidden documents keep their mounted editors and drafts.
export const ruleSelection = signal<Record<number, string[]>>({});
export const selectedRules = (uid: number): string[] => ruleSelection.value[uid] || ['agreement'];
export function selectRule(uid: number, name: string, add = false): void {
 const current = selectedRules(uid);
 const next = add ? current.includes(name) ? current.filter(item => item !== name) : [...current, name] : [name];
 ruleSelection.value = { ...ruleSelection.value, [uid]: next.length ? next : ['agreement'] };
}
