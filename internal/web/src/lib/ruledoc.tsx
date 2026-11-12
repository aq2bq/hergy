// Shared document state and prose editing for role and team screens.
import type { ReadonlyDeep } from './editing.js';
import type { TransferSource } from './prose-transfer.js';
import { Field } from './field.js';
import { ProseEditor, title, type ProseHeading } from './prose-editor.js';

export type EditableDocument = { name: string; title: string; text: string; __path?: string };
export type RuleIssue = { sev: string; k: number; f: string; text: string; definition?: string };
export type RulePanel = ReadonlyDeep<{ saved: EditableDocument | null; draft: EditableDocument }>;
export type RuleDocProps = {
 transfer?: TransferSource; row: RulePanel; issues: RuleIssue[]; nameError?: string; heading: ProseHeading;
 onDraft: (draft: EditableDocument) => void;
} & ({ hasName: false; rename?: never; nameChanged?: never } | { hasName?: true; rename: { value: boolean; change: (on: boolean) => void }; nameChanged: boolean });
export const clone = <T,>(value: T): T => structuredClone(value);
export function fromJSON(value: unknown): EditableDocument {
 const d = value && typeof value === 'object' ? value as Record<string, unknown> : {};
 return { name: typeof d.name === 'string' ? d.name : '', title: typeof d.title === 'string' ? d.title : '', text: typeof d.text === 'string' ? d.text : '' };
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
export function RuleDoc({ row, hasName = true, heading, nameError, issues, onDraft, rename, nameChanged, transfer }: RuleDocProps) {
 const change = onDraft;
 return <>{hasName ? <label class="prose-name">ロール名<Field class="in-name" data-f="name" value={row.draft.name} onInput={(event: Event) => { if (event.currentTarget instanceof HTMLInputElement) { change({ ...row.draft, name: event.currentTarget.value }); } }} />{nameError ? <span class="errline name" role="alert">{nameError}</span> : null}</label> : null}
 {hasName && nameChanged ? <label class="rename"><input type="checkbox" checked={rename!.value} onChange={(event) => { rename!.change(event.currentTarget.checked); }} />使っているチームのロール名も変更する</label> : null}
 <ProseEditor transfer={transfer} heading={heading} text={row.draft.text} onChange={(text) => { change({ ...row.draft, text, title: title(text, row.draft.name) }); }} />
 <Vm list={issues.filter((p) => p.f !== 'name' && p.f !== 'size')} /></>;
}
