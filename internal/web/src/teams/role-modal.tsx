import { editRole, type RoleEditing, type RoleCarry } from '../lib/editing.js';
import { resourceIdentity } from '../lib/resource-identity.js';
import { runDanger } from '../lib/danger.js';
import { showOnly } from './rule-selection.js';
import { transferTargets } from './prose-transfer.js';
import { documentID, documents } from '../lib/prose-transfer.js';
import type { TransferSource } from '../lib/prose-transfer.js';
import { signal, useEffect, useMemo, useRef } from '../lib/preact.js';
import { read, problemText } from '../lib/api.js';
import type { Schemas } from '../lib/api.js';
import { resource } from '../lib/resource.js';
import { roleProblems } from '../lib/role-check.js';
import { RuleDoc, fromJSON } from '../lib/ruledoc.js';
import { ProseIdentity, title, type ProseHeading } from '../lib/prose-editor.js';
import { ConflictNotice } from '../lib/conflict.js';
import { carryTo } from '../lib/nav.js';
import { projectOfGroup } from '../lib/copy.js';
import { CURRENT, groupOfRecord, refreshAfterRoleSave, checkTeam, pressDtab } from './state.js';
import type { Rec } from './state.js';

type Session = { id: number; r: Rec; name: string; expectedPlace?: string; trigger: HTMLElement | null };
let ids = 1000000;
const focusTarget = signal('');
export function RoleButton({ r, name, children = 'ロールを確認・編集' }: { r: Rec; name: string; expectedPlace?: string; children?: string }) {
  return <button class="btn sm" type="button" disabled={!name} onClick={() => {
    showOnly(r.uid, 'role:' + name);
    focusTarget.value = `prose-${r.uid}-${name}`;
    pressDtab(r, 'agree');
  }}>{children}</button>;
}
type Scanned = (name: string, places: string[] | null) => void;
export function RoleColumn({ r, name, onScanned, visible = true }: { r: Rec; name: string; onScanned: Scanned; visible?: boolean }) {
  const session = useMemo<Session>(() => ({ id: ++ids, r, name, trigger: null }), [r, name]);
  return <Editor session={session} onScanned={onScanned} visible={visible} />;
}

function newModel(session: Session) {
    const global = session.r.place.value === 'global';
    const project = global ? undefined : (projectOfGroup(groupOfRecord(session.r)) || CURRENT);
    const query = new URLSearchParams({ context: global ? 'global' : 'project' });
    if (project) query.set('project', project);
    return {
      global, project, editor: signal<RoleEditing | null>(null), ready: signal(false), error: signal(''),
      target: signal<{place: 'project' | 'global'; path: string} | null>(null),
      context: resource(async (signal: AbortSignal) => read<Schemas['RoleContext']>(`/api/role/${encodeURIComponent(session.name)}/context?${query}`, { signal })),
    };
}

const models = new Map<string, ReturnType<typeof newModel>>();
let carriedRoles: RoleCarry[] = [];
export const roleEntries = (): RoleEditing[] => [...models.values()].flatMap((m) => m.editor.value ? [m.editor.value] : []);
export const pendingRoleChanges = () => carriedRoles.length > 0;
export const takeRoleEdits = (): RoleCarry[] => [...carriedRoles, ...roleEntries().filter((e) => e.dirty.value).map((e) => e.take())];
export function restoreRoleEdits(carries: RoleCarry[]): void { carriedRoles = carries; }

export function roleSummary(r: Rec, name: string): string {
 const model = models.get(`${r.uid}:${r.place.value}:${name}`);
 if (!model) return '読み込み中';
 const editor = model.editor.value;
 const context = model.context.value.value;
 const problems = editor?.read.value.problems || [];
 const status = editor?.dirty.value ? '未保存' : '保存済み';
 if (model.context.loading.value) return '読み込み中';
 if (model.context.error.value || context?.errors.length || context?.state !== 'readable' || model.error.value || editor?.checkingError.value) return `${status} · 読み込み・保存の確認が必要`;
 return `${status}${problems.length ? ` · 問題・警告 ${problems.length}` : ''}`;
}

function Editor({ session, onScanned, visible }: { session: Session; onScanned: Scanned; visible: boolean }) {
  const dialog = useRef<HTMLElement>(null);
  const model = useMemo(() => {
    const key = `${session.r.uid}:${session.r.place.value}:${session.name}`;
    let held = models.get(key);
    if (!held) { held = newModel(session); models.set(key, held); }
    return held;
  }, [session]);
  const editor = model.editor.value;
  const dirty = () => !!model.editor.value?.dirty.value;
  const saving = () => !!model.editor.value?.saving.value;
  // askDiscard asks before an edit goes (d-021e): the words are changed's
  // answer for this document, not the screen's. A clean document goes at once.
  const askDiscard = (message: string, label: string, run: () => void) => {
    if (saving()) return;
    if (!dirty()) { run(); return; }
    const cur = model.editor.value;
    runDanger('discardRoleEdit', {
      run,
      lost: cur ? cur.lost.value : [],
      confirm: { title: message, label },
    });
  };
  async function check() {
    const editor = model.editor.value;
    if (!editor) return;
    await editor.check();
    if (!editor.checkingError.value && Array.isArray(editor.validation.value.problems)) model.ready.value = true;
  }
  async function load() {
    const ctx = await model.context.run();
    if (!ctx?.doc || ctx.state !== 'readable') return;
    if (session.expectedPlace && session.expectedPlace !== ctx.place) {
      await checkTeam(session.r);
      model.error.value = 'プロンプトの文書が変わったため、プロンプトを再検査しました。閉じて、もう一度開いてください';
      return;
    }
    const saved = fromJSON(ctx.doc), identity = resourceIdentity('role', { place: ctx.place, project: model.project, name: session.name });
    model.target.value = { place: ctx.place, path: ctx.path };
    model.editor.value?.dispose();
    const next = editRole(identity, { saved: { ...saved, name: identity.name! }, draft: saved, hash: ctx.hash, text: ctx.text, roleContext: model.global ? 'global' : 'project' });
    model.editor.value = next;
    const carried = carriedRoles.find((carry) => next.matches(carry));
    if (carried) { next.restore(carried); carriedRoles = carriedRoles.filter((one) => one !== carried); }
    model.ready.value = false;
    await check();
  }

  useEffect(() => {
    if (!dirty()) void load();
    else void check();
    return () => {
      model.context.abort();
      session.trigger?.focus({ preventScroll: true });
    };
  }, [model]);

  async function save(overwrite = false) {
    const editor = model.editor.value, ctx = model.context.value.value, target = model.target.value;
    if (!editor || !ctx || !target || target.path !== ctx.path || saving() || model.context.loading.value || model.context.error.value || ctx.errors.length) return;
    dialog.current?.querySelector<HTMLElement>('h2')?.focus({ preventScroll: true });
    model.error.value = '';
    const answer = await (overwrite ? editor.overwrite() : editor.save());
    if (!answer?.json) return false;
    try { await check(); await model.context.run(); } catch { model.error.value = '保存しましたが、文書の再検査に失敗しました。'; }
    void refreshAfterRoleSave().catch(() => { model.error.value = '保存しましたが、チーム定義を再検査できませんでした。'; });
    return true;
  }

  const ctx = model.context.value.value, e = editor?.read.value;
  useEffect(() => {
    onScanned(session.name, ctx?.scanned || []);
    return () => onScanned(session.name, null);
  }, [ctx?.scanned, onScanned]);
  useEffect(() => {
    if (!e || focusTarget.value !== `prose-${session.r.uid}-${session.name}`) return;
    const input = dialog.current?.querySelector('textarea');
    if (input) { input.focus({ preventScroll: true }); input.scrollIntoView({ block: 'nearest', inline: 'nearest' }); focusTarget.value = ''; }
  }, [e, focusTarget.value]);
  const answer = editor?.validation.value;
  const issues = e && answer?.problems ? roleProblems(e.draft, answer.problems) : [];
  const errors = editor?.grew.value || [];
  const unavailable = model.context.loading.value || !!model.context.error.value || !ctx || ctx.errors.length > 0 || model.target.value?.path !== ctx.path;
  const blocked = unavailable || !dirty() || !model.ready.value || editor?.loading.value || !!editor?.checkingError.value || !answer?.problems || errors.length > 0 || saving();
  const other = ctx?.uses.filter((u) => !(u.name === session.r.name.value && u.place === session.r.place.value && (u.place === 'global' || u.project === model.project))) || [];
  const query = new URLSearchParams({ open: session.name + '|' + (ctx?.place === 'global' ? 'global' : (model.project || '')) });
  if (model.project) query.set('project', model.project);
  function leave(event: MouseEvent) {
    if (saving()) { event.preventDefault(); return; }
    if (!dirty()) { carryTo('roles', model.project); return; }
    // The link goes with the page: hold it until the person answers (d-021e).
    event.preventDefault();
    const href = (event.currentTarget as HTMLAnchorElement).href;
    askDiscard('ロールに保存していない変更があります。変更を破棄して閉じますか？', '破棄して閉じる', () => {
      carryTo('roles', model.project);
      location.assign(href);
    });
  }
  const transfer: TransferSource | undefined = e && ctx?.state === 'readable' ? {
    id: documentID('role', session.name, ctx.place, model.project || CURRENT),
    label: `ロール ${session.name}（${ctx.place === 'global' ? 'グローバル' : model.project || CURRENT}）`,
    text: () => model.editor.value!.read.value.draft.text,
    set: (text) => { const editor = model.editor.value!; editor.change({ ...editor.read.value.draft, text, title: title(text, session.name) }); void check(); },
    save: async () => !!await save(), targets: transferTargets,
  } : undefined;
  if (transfer) documents.set(transfer.id, transfer);
  const heading: ProseHeading = { kind: 'ロール', name: session.name, title: e?.draft.title,
    id: `role-title-${session.id}`, path: ctx ? `${ctx.place === 'global' ? 'グローバル' : 'プロジェクト'} · roles/${session.name}.md` : '',
    members: session.r.draft.value.members.filter((m) => m.role === session.name).map((m) => m.name).join('、'),
    otherTeams: other.map((u) => `${u.project_name} ${u.name}（${u.members.join('、')}）`).join('、'),
    state: ctx?.state === 'missing' ? '文書なし' : ctx?.state === 'unreadable' ? '読み込み不可' : dirty() ? '未保存' : editor?.read.value.savedLine ? '保存しました' : '保存済み' };
  return <section hidden={!visible} class={'prose-column role-column' + (ctx?.state === 'missing' ? ' missing-document' : '')} ref={dialog} id={`prose-${session.r.uid}-${session.name}`} aria-labelledby={`role-title-${session.id}`}>
    <div class="role-dialog-body">
    {!e || ctx?.state !== 'readable' ? <><header class="prose-head prose-head-unavailable"><ProseIdentity heading={heading} /></header><div class="prose-unavailable">
      <p role="alert">{ctx?.problem || (model.context.loading.value ? 'ロールの文書を読んでいます。' : 'ロールの文書を読み込めませんでした。')}</p>
      {ctx?.state === 'missing' ? <a class="btn" href={`/roles?${query}`} onClick={leave}>ロールの画面で作る</a> : null}
    </div></> : null}
    {ctx?.errors.map((text) => <p role="alert">{text}</p>)}
    {model.context.error.value ? <p role="alert">{problemText(model.context.error.value, 'ロールと使用先を読み込めませんでした')}</p> : null}
    {model.context.loading.value ? <p role="status">ロールと使用先を読んでいます。</p> : null}
    {model.target.value && ctx && model.target.value.path !== ctx.path ? <p role="alert">使われるロールの置き場所が変わりました。ファイルからリロードしてください。</p> : null}
    {e && ctx?.state === 'readable' ? <fieldset disabled={saving() || !model.ready.value}><div class="pmain"><RuleDoc transfer={transfer} hasName={false} heading={heading}
      row={{ saved: e.saved, draft: e.draft }}
      issues={issues} onDraft={(draft) => { editor!.change(draft); void check(); }} /></div></fieldset> : null}
    {editor?.read.value.conflict && e ? <ConflictNotice c={editor.read.value.conflict.c} mine={e.text || ''} busy={saving()} undone={editor.read.value.conflict.undone}
      onOverwrite={() => void save(true)} onRevert={() => { editor.readConflict(); void check(); }} onUndo={() => { editor.undoConflict(); void check(); }} /> : null}
    {errors.map((error) => <p role="alert">{error.text}</p>)}
    {editor?.checkingError.value ? <p role="alert">{problemText(editor.checkingError.value, 'ロールを検査できませんでした')}</p> : null}
    {editor?.read.value.refusal ? <p role="alert">{editor.read.value.refusal}</p> : null}
    {model.error.value ? <p role="alert">{model.error.value}</p> : null}
    </div>
    {ctx?.state !== 'missing' ? <footer class="prose-foot"><div class="prose-save">
      <button class="btn primary" disabled={blocked} onClick={() => void save()}>保存</button>
    </div><div class="prose-actions">
      <a class="btn sm" title="ロールの画面で開く" aria-label="ロールの画面で開く" href={`/roles?${query}`} onClick={leave}><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M14 3h7v7M21 3 10 14M10 5H4v15h15v-6" /></svg><span>ロールの画面で開く</span></a>
      <button class="btn sm" title="ファイルからリロード" aria-label="ファイルからリロード" disabled={saving()} onClick={() => askDiscard('保存していない変更を捨てて、ファイルからリロードしますか？', '破棄して読み直す', () => { void load(); })}><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M20 4v6h-6M20 10a8 8 0 1 0 0 6" /></svg><span>ファイルからリロード</span></button></div>

</footer> : null}
  </section>;
}
