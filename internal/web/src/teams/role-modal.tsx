import { transferTargets } from './prose-transfer.js';
import { documentID, documents } from '../lib/prose-transfer.js';
import type { TransferSource } from '../lib/prose-transfer.js';
import { signal, useEffect, useMemo, useRef } from '../lib/preact.js';
import { read, problemText } from '../lib/api.js';
import type { Schemas } from '../lib/api.js';
import { resource } from '../lib/resource.js';
import { roleEditor, roleProblems } from '../lib/role-editor.js';
import { RuleDoc, fromJSON, toDoc } from '../lib/ruledoc.js';
import { ProseIdentity, title, type ProseHeading } from '../lib/prose-editor.js';
import type { EditableDocument } from '../lib/ruledoc.js';
import { ConflictNotice, conflictOf } from '../lib/conflict.js';
import type { Conflict } from '../lib/conflict.js';
import { carryTo } from '../lib/nav.js';
import { projectOfGroup } from '../lib/copy.js';
import { CURRENT, groupOfRecord, refreshAfterRoleSave, checkTeam, pressDtab } from './state.js';
import type { Rec } from './state.js';

type Session = { id: number; r: Rec; name: string; expectedPlace?: string; trigger: HTMLElement | null };
type Entry = { saved: EditableDocument; draft: EditableDocument; hash: string; text: string };
let ids = 1000000;
const focusTarget = signal('');
export function RoleButton({ r, name, children = 'ロールを確認・編集' }: { r: Rec; name: string; expectedPlace?: string; children?: string }) {
  return <button class="btn sm" type="button" disabled={!name} onClick={() => {
    focusTarget.value = `prose-${r.uid}-${name}`;
    pressDtab(r, 'agree');
  }}>{children}</button>;
}
export function RoleColumn({ r, name }: { r: Rec; name: string }) {
  const session = useMemo<Session>(() => ({ id: ++ids, r, name, trigger: null }), [r, name]);
  return <Editor session={session} />;
}

function newModel(session: Session) {
    const global = session.r.place.value === 'global';
    const project = global ? undefined : (projectOfGroup(groupOfRecord(session.r)) || CURRENT);
    const query = new URLSearchParams({ context: global ? 'global' : 'project' });
    if (project) query.set('project', project);
    return {
      global, project, editor: roleEditor(), entry: signal<Entry | null>(null),
      baseline: signal<Set<string> | null>(null), error: signal(''), ok: signal(false),
      target: signal<{place: 'project' | 'global'; path: string} | null>(null),
      conflict: signal<{ c: Conflict; before?: Entry; undone?: boolean } | null>(null),
      context: resource(async (signal: AbortSignal) => read<Schemas['RoleContext']>(`/api/role/${encodeURIComponent(session.name)}/context?${query}`, { signal })),
    };
}

const models = new Map<string, ReturnType<typeof newModel>>();
export function roleDraftsDirty(): boolean {
  return [...models.values()].some((model) => {
    const e = model.entry.value;
    return !!e && e.draft.text !== e.saved.text;
  });
}

function Editor({ session }: { session: Session }) {
  const dialog = useRef<HTMLElement>(null);
  const model = useMemo(() => {
    const key = `${session.r.uid}:${session.r.place.value}:${session.name}`;
    let held = models.get(key);
    if (!held) { held = newModel(session); models.set(key, held); }
    return held;
  }, [session]);
  const { editor, entry } = model;
  const dirty = () => !!entry.value && JSON.stringify(entry.value.draft) !== JSON.stringify(entry.value.saved);
  const canDiscard = (message: string) => !editor.saving.value && (!dirty() || window.confirm(message));
  async function check(resetBaseline = false) {
    const e = entry.value, ctx = model.context.value.value;
    if (!e || !ctx) return;
    const answer = await editor.check.run({ project: model.project, place: model.target.value?.place || ctx.place, original: session.name, doc: toDoc(e.draft, e.saved) });
    if (answer?.json?.problems && resetBaseline) model.baseline.value = new Set(answer.json.problems.filter((p) => p.sev === 'err').map((p) => p.text));
  }
  async function load() {
    const ctx = await model.context.run();
    if (!ctx?.doc || ctx.state !== 'readable') return;
    if (session.expectedPlace && session.expectedPlace !== ctx.place) {
      await checkTeam(session.r);
      model.error.value = 'プロンプトの文書が変わったため、プロンプトを再検査しました。閉じて、もう一度開いてください';
      return;
    }
    const saved = fromJSON(ctx.doc);
    model.target.value = { place: ctx.place, path: ctx.path };
    entry.value = { saved, draft: structuredClone(saved), hash: ctx.hash, text: ctx.text };
    model.baseline.value = null;
    await check(true);
  }
  useEffect(() => {
    if (!dirty()) void load();
    else void check();
    return () => {
      model.context.abort(); editor.check.abort();
      session.trigger?.focus({ preventScroll: true });
    };
  }, [model]);

  async function save(over?: string) {
    const e = entry.value, ctx = model.context.value.value, target = model.target.value;
    if (!e || !ctx || !target || target.path !== ctx.path || editor.saving.value || model.context.loading.value || model.context.error.value || ctx.errors.length) return;
    // Saving disables the controls; keep focus on a stable, enabled element.
    dialog.current?.querySelector<HTMLElement>('h2')?.focus({ preventScroll: true });
    editor.check.abort(); model.error.value = ''; model.ok.value = false;
    try {
      const answer = await editor.save(session.name, { project: model.project, place: target.place,
        original: session.name, role_context: model.global ? 'global' : 'project',
        hash: over || e.hash, doc: toDoc(e.draft, e.saved),
      });
      if (!answer.json) throw new Error('保存の結果を読めませんでした。ファイルからリロードしてください');
      const saved = fromJSON(answer.json.doc);
      entry.value = { saved, draft: structuredClone(saved), hash: answer.json.hash, text: answer.json.text };
      model.conflict.value = null; model.ok.value = true;
      try { await check(true); await model.context.run(); } catch { model.error.value = '保存しましたが、文書の再検査に失敗しました。'; }
      void refreshAfterRoleSave().catch(() => { model.error.value = '保存しましたが、チーム定義を再検査できませんでした。'; });
      return true;
    } catch (err) {
      const conflict = conflictOf(err);
      if (conflict) model.conflict.value = { c: conflict };
      else model.error.value = problemText(err, 'ロールを保存できませんでした');
    }
  }

  const ctx = model.context.value.value, e = entry.value;
  useEffect(() => {
    if (!e || focusTarget.value !== `prose-${session.r.uid}-${session.name}`) return;
    const input = dialog.current?.querySelector('textarea');
    if (input) { input.focus(); input.scrollIntoView({ block: 'nearest', inline: 'nearest' }); focusTarget.value = ''; }
  }, [e, focusTarget.value]);
  const answer = editor.check.value.value?.json;
  const issues = e && answer?.problems ? roleProblems(e.draft, answer.problems) : [];
  const errors = issues.filter((p) => p.sev === 'err' && !model.baseline.value?.has(p.text));
  const unavailable = model.context.loading.value || !!model.context.error.value || !ctx || ctx.errors.length > 0 || model.target.value?.path !== ctx.path;
  const blocked = unavailable || !dirty() || !model.baseline.value || editor.check.loading.value || !!editor.check.error.value || !answer?.problems || errors.length > 0 || editor.saving.value;
  const other = ctx?.uses.filter((u) => !(u.name === session.r.name.value && u.place === session.r.place.value && (u.place === 'global' || u.project === model.project))) || [];
  const query = new URLSearchParams({ open: session.name + '|' + (ctx?.place === 'global' ? 'global' : (model.project || '')) });
  if (model.project) query.set('project', model.project);
  function leave(event: MouseEvent) { if (!canDiscard('ロールに保存していない変更があります。変更を破棄して閉じますか？')) event.preventDefault(); else carryTo('roles', model.project); }
  const transfer: TransferSource | undefined = e && ctx?.state === 'readable' ? {
    id: documentID('role', session.name, ctx.place, model.project || CURRENT),
    label: `ロール ${session.name}（${ctx.place === 'global' ? 'グローバル' : model.project || CURRENT}）`,
    text: () => entry.value!.draft.text,
    set: (text) => { const old = entry.value!; entry.value = { ...old, draft: { ...old.draft, text, title: title(text, session.name) } }; model.ok.value = false; void check(); },
    save: async () => !!await save(), targets: transferTargets,
  } : undefined;
  if (transfer) documents.set(transfer.id, transfer);
  const heading: ProseHeading = { kind: 'ロール', name: session.name, title: e?.draft.title,
    id: `role-title-${session.id}`, path: ctx ? `${ctx.place === 'global' ? 'グローバル' : 'プロジェクト'} · roles/${session.name}.md` : '',
    members: session.r.draft.value.members.filter((m) => m.role === session.name).map((m) => m.name).join('、'),
    state: ctx?.state === 'missing' ? '文書なし' : ctx?.state === 'unreadable' ? '読み込み不可' : dirty() ? '未保存' : model.ok.value ? '保存しました' : '保存済み' };
  return <section class={'prose-column role-column' + (ctx?.state === 'missing' ? ' missing-document' : '')} ref={dialog} id={`prose-${session.r.uid}-${session.name}`} aria-labelledby={`role-title-${session.id}`}>
    <div class="role-dialog-body">
    {!e || ctx?.state !== 'readable' ? <><header class="prose-head prose-head-unavailable"><ProseIdentity heading={heading} /></header><div class="prose-unavailable">
      <p role="alert">{ctx?.problem || (model.context.loading.value ? 'ロールの文書を読んでいます。' : 'ロールの文書を読み込めませんでした。')}</p>
      {ctx?.state === 'missing' ? <a class="btn" href={`/roles?${query}`} onClick={leave}>ロールの画面で作る</a> : null}
    </div></> : null}
    {ctx?.errors.map((text) => <p role="alert">{text}</p>)}
    {model.context.error.value ? <p role="alert">{problemText(model.context.error.value, 'ロールと使用先を読み込めませんでした')}</p> : null}
    {model.context.loading.value ? <p role="status">ロールと使用先を読んでいます。</p> : null}
    {model.target.value && ctx && model.target.value.path !== ctx.path ? <p role="alert">使われるロールの置き場所が変わりました。ファイルからリロードしてください。</p> : null}
    {e && ctx?.state === 'readable' ? <fieldset disabled={editor.saving.value || !model.baseline.value}><div class="pmain"><RuleDoc transfer={transfer} hasName={false} heading={heading}
      row={{ uid: session.id, l: ctx.place, saved: e.saved, draft: e.draft, markdown: answer?.preview || { lines: [], chars: 0 }, diff: answer?.effect?.diff || {}, savedIssues: [] }}
      issues={issues} onChange={() => { entry.value = { ...e, draft: structuredClone(e.draft) }; model.ok.value = false; void check(); }} /></div></fieldset> : null}
    {model.conflict.value && e ? <ConflictNotice c={model.conflict.value.c} mine={e.text} busy={editor.saving.value} undone={model.conflict.value.undone}
      onOverwrite={() => void save(model.conflict.value?.c.file.hash)}
      onRevert={() => { const hold = model.conflict.value!; const before = entry.value!; void load().then(() => { model.conflict.value = { c: hold.c, before, undone: true }; }); }}
      onUndo={() => { if (model.conflict.value?.before) { entry.value = model.conflict.value.before; model.conflict.value = { c: model.conflict.value.c }; void check(); } }} /> : null}
    {errors.map((error) => <p role="alert">{error.text}</p>)}
    {editor.check.error.value ? <p role="alert">{problemText(editor.check.error.value, 'ロールを検査できませんでした')}</p> : null}
    {model.error.value ? <p role="alert">{model.error.value}</p> : null}
    </div>
    {ctx?.state !== 'missing' ? <footer class="prose-foot"><div class="prose-save">
      <button class="btn primary" disabled={blocked} onClick={() => void save()}>保存</button>
    </div><div class="prose-actions">
      <a class="btn sm" title="ロールの画面で開く" aria-label="ロールの画面で開く" href={`/roles?${query}`} onClick={leave}><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M14 3h7v7M21 3 10 14M10 5H4v15h15v-6" /></svg><span>ロールの画面で開く</span></a>
      <button class="btn sm" title="ファイルからリロード" aria-label="ファイルからリロード" disabled={editor.saving.value} onClick={() => { if (canDiscard('保存していない変更を捨てて、ファイルからリロードしますか？')) void load(); }}><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M20 4v6h-6M20 10a8 8 0 1 0 0 6" /></svg><span>ファイルからリロード</span></button></div>
    {ctx ? <div class="role-usage">
      {ctx.scanned.length ? <div class="usage"><span class="lbl">確認できた範囲</span>{ctx.scanned.map((name) => <span class="uchip">{name}</span>)}</div> : null}
      {other.length ? <div class="usage"><span class="lbl">使っているチーム</span>{other.flatMap((u) => u.members.map((member) => <span class="uchip"><span class="g">{u.project_name}: {u.name}</span>{member}</span>))}</div> : null}
    </div> : null}
</footer> : null}
  </section>;
}
