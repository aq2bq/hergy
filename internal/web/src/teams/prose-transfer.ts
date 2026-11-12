import { term } from '../lib/terms.generated.js';
import { editRole } from '../lib/editing.js';
import { resourceIdentity } from '../lib/resource-identity.js';
import { get, hqList, read, saved, problemText } from '../lib/api.js';
import type { Schemas } from '../lib/api.js';
import { documents, documentID } from '../lib/prose-transfer.js';
import type { TransferDocument, TransferSource, TransferTarget } from '../lib/prose-transfer.js';
import { projectOfGroup } from '../lib/copy.js';
import { title } from '../lib/prose-editor.js';
import { recs, CURRENT, groupOfRecord, placeGroups, currentProjectName, saveAgree, scheduleAgree, refreshAfterRoleSave } from './state.js';
import type { Rec } from './state.js';

export function teamTransfer(r: Rec): TransferSource {
 return {
  id: documentID('team', r.name.value, r.place.value, projectOfGroup(groupOfRecord(r)) || CURRENT),
  label: `${term('team-definition')} ${r.name.value}（${r.place.value === 'global' ? term('global') : placeGroups().find((g) => g.key === groupOfRecord(r))?.name || currentProjectName() || CURRENT}）`,
  text: () => r.agreeDraft.value.text,
  set: (text) => { r.editing.agreement.change({ ...r.agreeDraft.value, text, title: title(text) }); scheduleAgree(r); },
  save: async () => { const text = r.agreeDraft.value.text; await saveAgree(r); return r.agreeSaved.value?.text === text; },
  targets: transferTargets,
 };
}
export async function transferTargets(): Promise<TransferTarget[]> {
 const out: TransferTarget[] = recs.value.filter((r) => !r.draftRow).map((r) => { const doc = teamTransfer(r); return { id: doc.id, label: doc.label, kind: 'team', group: r.place.value === 'global' ? term('global') : placeGroups().find((g) => g.key === groupOfRecord(r))?.name || currentProjectName(), otherProject: r.place.value !== 'global' && (projectOfGroup(groupOfRecord(r)) || CURRENT) !== CURRENT, load: async () => doc }; });
 const groups = await hqList('roles');
 for (const group of groups.groups) for (const item of group.list?.items || []) {
  for (const place of ['project', 'global'] as const) {
   if (!item.places[place]?.doc) continue;
   const project = group.project?.path || CURRENT;
   const id = documentID('role', item.name, place, project);
   if (out.some((x) => x.id === id)) continue;
   const label = `${term('role')} ${item.name}（${place === 'global' ? term('global') : group.project?.name || project}）`;
   out.push({ id, label, kind: 'role', group: place === 'global' ? term('global') : group.project?.name || project, otherProject: place !== 'global' && project !== CURRENT, load: async () => {
    const held = documents.get(id);
    if (held) return held;
    const file = await get('role', item.name, place, project);
    const identity = resourceIdentity('role', { name: item.name, place, project });
    const editing = editRole(identity, { saved: { ...file.doc, name: identity.name! }, draft: file.doc, hash: file.hash, text: file.text, roleContext: place });
    const doc: TransferDocument = { id, label, text: () => editing.read.value.draft.text, set: (text) => editing.change({ ...editing.read.value.draft, text, title: title(text) }), save: async () => {
     const query = new URLSearchParams({ context: place, project });
     const context = await read<Schemas['RoleContext']>(`/api/role/${encodeURIComponent(item.name)}/context?${query}`);
     if (context.state !== 'readable' || context.place !== place || context.errors.length) throw new Error(context.problem || context.errors.join('、') || `${term('role')}の保存先と使用先を確認できませんでした`);
     try {
      const answer = await editing.save();
      if (!answer?.json) { if (editing.read.value.refusal) throw new Error(editing.read.value.refusal); return false; }
      saved();
     } catch (err) { throw new Error(problemText(err, `${term('role')}を保存できませんでした`)); }
     // A successful file write stays successful even if refreshing the list fails.
     void refreshAfterRoleSave().catch(() => {});
     return true;
    } };
    return doc;
   } });
  }
 }
 return out;
}
