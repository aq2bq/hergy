import { signal, useEffect, useMemo, useRef } from '../lib/preact.js';
import { read, problemText } from '../lib/api.js';
import type { Schemas } from '../lib/api.js';
import { resource } from '../lib/resource.js';
import { roleEditor, roleProblems } from '../lib/role-editor.js';
import { RuleDoc, fromJSON, toDoc } from '../lib/ruledoc.js';
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
  const canClose = () => !editor.saving.value && (!dirty() || window.confirm('ロールに保存していない変更があります。変更を破棄して閉じますか？'));
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
      if (!answer.json) throw new Error('保存の結果を読めませんでした。文書を読み直してください');
      const saved = fromJSON(answer.json.doc);
      entry.value = { saved, draft: structuredClone(saved), hash: answer.json.hash, text: answer.json.text };
      model.conflict.value = null; model.ok.value = true;
      await check(true);
      await model.context.run();
      await refreshAfterRoleSave();
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
  function leave(event: MouseEvent) { if (!canClose()) event.preventDefault(); else carryTo('roles', model.project); }
  return <section class="prose-column role-column" ref={dialog} id={`prose-${session.r.uid}-${session.name}`} aria-labelledby={`role-title-${session.id}`}>
    <header><h2 id={`role-title-${session.id}`} tabIndex={-1}>{e?.draft.title || session.name}（{session.name}）</h2>{ctx ? <code class="path" title={ctx.path}>{ctx.place === 'global' ? 'グローバル' : 'プロジェクト'} · roles/{session.name}.md</code> : null}</header>
    {ctx ? <details class="role-usage"><summary>保存先と影響するチーム</summary>
      <p>{ctx.place === 'global' ? 'グローバル' : 'プロジェクト'} · <code>{ctx.path}</code></p>
      {ctx.place === 'global' ? <p>ほかのプロジェクトからも使える文書です。実行するプロジェクトに同名のロールがあれば、その文書が使われます。</p> : null}
      <p>このチーム定義で編集中: {session.r.name.value}。確認できた範囲: {ctx.scanned.join('、')}</p>
      {other.length ? <ul>{other.map((u) => <li>{u.project_name} / {u.name}（{u.place === 'global' ? 'グローバル' : 'プロジェクト'}）: {u.members.join('、')}</li>)}</ul> : <p>確認できた範囲では、ほかに使うチーム定義はありません。</p>}
      {ctx.errors.map((text) => <p role="alert">{text}</p>)}
      {ctx.state !== 'readable' ? <p role="alert">{ctx.problem}</p> : null}
    </details> : null}
    <div class="role-dialog-body">
    {model.context.error.value ? <p role="alert">{problemText(model.context.error.value, 'ロールと使用先を読み込めませんでした')}</p> : null}
    {model.context.loading.value ? <p role="status">ロールと使用先を読んでいます。</p> : null}
    {model.target.value && ctx && model.target.value.path !== ctx.path ? <p role="alert">使われるロールの置き場所が変わりました。文書を読み直してください。</p> : null}
    {e && ctx?.state === 'readable' ? <fieldset disabled={editor.saving.value || !model.baseline.value}><div class="pmain"><RuleDoc hasName={false}
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
    <footer><div class="prose-actions"><a class="btn sm" href={`/roles?${query}`} onClick={leave}>ロールの画面で開く</a>{ctx?.state === 'unreadable' ? <a class="btn sm" href={`/roles/yaml?${query}&file=${ctx.place}.${encodeURIComponent(session.name)}`} onClick={leave}>ロールの Markdown で直す</a> : null}
    <button class="btn sm" disabled={editor.saving.value} onClick={() => { if (canClose()) void load(); }}>文書を読み直す</button></div>
{e ? <button class="btn primary" disabled={blocked} onClick={() => void save()}>ロールを保存</button> : null}{model.ok.value ? <span role="status">ロールを保存しました</span> : null}</footer>
  </section>;
}
