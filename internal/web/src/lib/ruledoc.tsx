// Shared document state and prose editing for role and team screens.
import type { components, operations } from '../../../apicontract/api-types';
import { roleDirty } from './changed.js';
import type { RoleEntryLike } from './changed.js';
import { signal } from './preact.js';
import type { TransferSource } from './prose-transfer.js';
import { Field } from './field.js';
import { ProseEditor, title, type ProseHeading } from './prose-editor.js';

export type EditableDocument = { name: string; title: string; text: string; __path?: string };
export type RulePanelDocument = EditableDocument;
export type RuleIssue = { sev: string; k: number; f: string; text: string; definition?: string };
export type RulePanel = {
 uid: number; l: operations['getTeam']['parameters']['query']['place'];
 saved: EditableDocument | null; draft: EditableDocument;
 e?: { saved: EditableDocument | null }; name?: string;
 markdown: unknown; diff: unknown; turns?: unknown; savedIssues?: RuleIssue[] | null; issues?: RuleIssue[];
};
export type RulePanelInput = RulePanel & { newRow?: boolean };
export type RuleDocProps = {
 transfer?: TransferSource; row: RulePanelInput; issues: RuleIssue[]; nameError?: string; hasName?: boolean; heading?: ProseHeading;
 filtering?: boolean; query?: string; readerCount?: (name: string) => number;
 onDraft?: (draft: EditableDocument) => void; rename?: { value: boolean; change: (on: boolean) => void };
 onChange?: (row: RulePanel) => void; onMode?: (mode: 'edit' | 'md') => void; onURL?: () => void;
};
const renameBox = signal<Record<string, boolean>>({});
export const renameCarries = (key: string): boolean => renameBox.value[key] !== false;
export const clone = <T,>(value: T): T => structuredClone(value);
export function fromJSON(value: unknown): EditableDocument {
 const d = value && typeof value === 'object' ? value as Record<string, unknown> : {};
 return { name: typeof d.name === 'string' ? d.name : '', title: typeof d.title === 'string' ? d.title : '', text: typeof d.text === 'string' ? d.text : '' };
}
export function toDoc(draft: EditableDocument, _saved: EditableDocument | null): components['schemas']['RoleDoc'] {
 return { name: draft.name, title: draft.title, text: draft.text };
}

export function Sev({ sev }: { sev: string }) {return <span class={'sev ' + sev}>{sev === 'err' ? '問題' : '警告'}</span>;}
export function Counts({ list }: { list: Pick<RuleIssue, 'sev'>[] }) {
 const errors = list.filter((p) => p.sev === 'err').length;
 const warnings = list.length - errors;
 return <>{errors ? <span class="sev err">問題 {errors}</span> : null}{warnings ? <span class="sev warn">警告 {warnings}</span> : null}</>;
}
export function Badge({ list }: { list: Pick<RuleIssue, 'sev'>[] }) { return <Counts list={list} />; }
export function Vm({ list }: { list: Pick<RuleIssue, 'sev' | 'text'>[] }) {
 return <ul class="vm">{list.map((p) => <li><Sev sev={p.sev} /><span>{p.text}</span></li>)}</ul>;
}
export function RuleDoc({ row, hasName = true, heading, nameError, issues, onChange, onDraft, rename, transfer }: RuleDocProps) {
 function change(draft: EditableDocument) { if (onDraft) onDraft(draft); else { Object.assign(row.draft, draft); onChange?.(row); } }
 return <>{hasName ? <label class="prose-name">ロール名<Field class="in-name" data-f="name" value={row.draft.name} onInput={(event: Event) => { if (event.currentTarget instanceof HTMLInputElement) { change({ ...row.draft, name: event.currentTarget.value }); } }} />{nameError ? <span class="errline name" role="alert">{nameError}</span> : null}</label> : null}
 {hasName && row.saved && row.saved.name !== row.draft.name ? <label class="rename"><input type="checkbox" checked={rename ? rename.value : renameCarries(row.uid + row.l)} onChange={(event) => { if (rename) rename.change(event.currentTarget.checked); else renameBox.value = { ...renameBox.value, [row.uid + row.l]: event.currentTarget.checked }; }} />使っているチームのロール名も変更する</label> : null}
 <ProseEditor transfer={transfer} heading={heading || { kind: 'ロール', name: row.draft.name || '新しいロール', title: row.draft.title, path: row.draft.__path, state: roleDirty(row as unknown as RoleEntryLike) ? '未保存' : '保存済み' }} text={row.draft.text} onChange={(text) => { change({ ...row.draft, text, title: title(text, row.draft.name) }); }} />
 <Vm list={issues.filter((p) => p.f !== 'name' && p.f !== 'size')} /></>;
}
