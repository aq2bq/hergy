// lib/yamltab.js — the YAML tab as one shared part (spec-boundary 7, r-2cc2),
// as a Preact component since the rewrite (webui-preact-design 段階 1-2).
//
// One section stands for one file: the line numbers down its left side, the
// check the person's typing rides on (300ms after it stops), the save that
// writes the whole text with the hash the page read, and the jump a problem's
// line makes. The server writes every sentence a problem shows; this file only
// lays them out (spec-boundary 3).
//
// The part draws one section's contents, from the header down to the 保存
// footer, and nothing here writes markup outside Preact's render
// (webui-preact-design 5 章 1). The presses on those buttons are the part's own
// (the problem's line, 元に戻す, 保存, and the Tab that writes two spaces), so a
// press is handled once: the screen draws the section's element, the part draws
// what stands inside it, and the screen passes the file and the ways to tell it
// things as props (依頼書 4).
import { runDanger } from './danger.js';
import { useEffect, useRef, useState } from './preact.js';
import { saved as apiSaved } from './api.js';
import { editYaml, type YamlEditing } from './editing.js';
import type { RawAnswer } from './api.js';
import { ConflictNotice } from './conflict.js';
import type { components, operations } from '../../../apicontract/api-types';

type Place = operations['getTeam']['parameters']['query']['place'];
type YamlKind = 'role' | 'team' | 'profile';
type Problem = components['schemas']['Problem'];
type SavedJson = components['schemas']['RoleSaved']
  | components['schemas']['TeamSaved']
  | components['schemas']['ProfileSaved'];
type ApiAnswer<T> = RawAnswer<T>;
type YamlState = { base: string; text: string; hash: string };

export type YamlTabProps = {
  editing?: YamlEditing;
  label: string;
  path: string;
  text: string;
  hash: string;
  kind: YamlKind;
  place: Place | string;
  name: string;
  problems?: Problem[];
  // problem is why the file cannot be read: the section says it and offers no
  // save over what it could not read (r-ea17, d-0bf1).
  problem?: string;
  badge?: { cls: string; text: string } | null;
  file?: string;
  extra?: Pick<components['schemas']['TeamCheckRequest'], 'part'>;
  carried?: string | null;
  onProblems?: (problems: Problem[]) => void;
  onRevert?: () => void;
  onState?: (state: YamlState) => void;
  onSaved?: (answer: ApiAnswer<SavedJson>) => void;
};

// tellDirty lets the page mark itself as carrying unsaved work, the same way the
// panels do (the browser's beforeunload reads it).
export function tellDirty() {
  if (window.hergyPage && window.hergyPage.tellDirty) window.hergyPage.tellDirty();
}

export function autosize(textarea: HTMLTextAreaElement): void {
  if (CSS.supports('field-sizing', 'content')) return;
  textarea.style.height = 'auto';
  textarea.style.height = `${textarea.scrollHeight + 2}px`;
}

// insertTab puts two spaces where the caret is and says the text changed, the
// way typing does: the part's own check rides on that.
export function insertTab(textarea: HTMLTextAreaElement): void {
  textarea.setRangeText('  ', textarea.selectionStart, textarea.selectionEnd, 'end');
  textarea.dispatchEvent(new Event('input', { bubbles: true }));
}

const errTexts = (problems: readonly Problem[] | null | undefined): string[] => (problems || []).filter((p) => p.sev === 'err').map((p) => p.text);
const heightOf = (lines: number): number => Math.min(Math.max(lines, 6), 34);
const marksOf = (problems: readonly Problem[] | null | undefined): Record<number, Problem['sev']> => {
  const marks: Record<number, Problem['sev']> = {};
  (problems || []).forEach((p) => {
    if (p.line !== undefined && marks[p.line] !== 'err') marks[p.line] = p.sev;
  });
  return marks;
};

// YamlTab renders one editor. Input updates editing synchronously; a render
// never writes the textarea again when its value already matches the draft.
// laid is only the last text used to draw the gutter.
//
// What the part reads and who it tells (依頼書 4):
//   file, label, path, badge, text, hash, kind, place, name, extra, carried
//   problems   the problems the last check named
//   onProblems(list)      the check answered: the file's own problems
//   onSaved(answer)       the save wrote the file: the screen keeps its hash
//   onState({base,text,hash})  what the part holds, for the band's draft
//   onRevert()            the person pressed 元に戻す, before the text goes back
export function YamlTab(props: YamlTabProps) {
  const { label, path, badge, file, text, hash, kind, place, name, extra, problems, problem, onProblems, onSaved, onState, onRevert, carried } = props;
  const ta = useRef<HTMLTextAreaElement | null>(null);
  const gutter = useRef<HTMLPreElement | null>(null);
  const timer = useRef<number>(0);
  const alive = useRef(true);
  const [editing] = useState(() => props.editing || editYaml(kind, { place, name, text, hash, problems, problem, part: extra?.part,
    project: new URLSearchParams(location.search).get('project') || undefined }));
  const state = editing.read.value;
  const initial = useRef(text);
  const [laid, setLaid] = useState(text);
  const now = { text: state.draft, problems: state.problems, baseProblems: state.baseProblems };
  const conflict = state.conflict?.c || null;
  const undone = !!state.conflict?.undone;
  const saved = state.savedLine;
  const refusal = state.refusal;
  const dirty = editing.dirty.value;
  const errs = now.problems.filter((p) => p.sev === 'err').length;
  const warns = now.problems.length - errs;
  const grew = now.problems.some((p) => p.sev === 'err' && errTexts(now.baseProblems).indexOf(p.text) < 0);

  function report() {
    const s = editing.read.value;
    onState?.({ base: s.saved || '', text: s.draft, hash: s.hash });
  }
  function foot() {
    const el = ta.current;
    if (!el) return;
    if (el.value !== editing.read.value.draft) editing.change(el.value);
    report();
    tellDirty();
  }
  function layout() {
    if (!alive.current) return;
    setLaid(editing.read.value.draft);
    onProblems?.(editing.read.value.problems.map((p) => ({ ...p })));
    report();
    tellDirty();
  }
  async function check() {
    await editing.check();
    layout();
  }
  function schedule() {
    clearTimeout(timer.current);
    timer.current = window.setTimeout(() => { check(); }, 300);
  }
  async function save(overwrite = false) {
    foot();
    const answer = await (overwrite ? editing.overwrite() : editing.save());
    if (!alive.current || !answer) return;
    onSaved?.(answer);
    layout();
    check();
    report();
    apiSaved();
  }
  function overwrite() { save(true); }
  function syncField() {
    const el = ta.current;
    if (!el) return;
    el.value = editing.read.value.draft;
    autosize(el);
    layout();
  }
  function readIn() { editing.readConflict(); syncField(); check(); }
  function undoRead() { editing.undoConflict(); syncField(); check(); }
  function revert() {
    const el = ta.current;
    if (!el) return;
    const was = editing.take();
    const host = (el.closest('.panel') as HTMLElement | null) || el.parentElement;
    runDanger('revertDraft', {
      run: () => { onRevert?.(); editing.discard(); syncField(); check(); },
      restore: () => { editing.restore(was); syncField(); check(); },
      said: '保存した内容に戻しました', host,
    });
  }

  // jump puts the caret on the line a problem names, and looks at it: a person
  // pressed the problem just now (5 章 5).
  function jump(line: number | undefined) {
    const el = ta.current;
    if (!el) return;
    const lines = el.value.split('\n');
    const targetLine = line ?? Number.NaN;
    const start = lines.slice(0, targetLine - 1).reduce((n: number, s: string) => n + s.length + 1, 0);
    el.focus({ preventScroll: true });
    el.setSelectionRange(start, start + (lines[targetLine - 1] || '').length);
    el.scrollTop = Math.max(0, (targetLine - 4) * 20);
    el.scrollLeft = 0;
  }

  // The part asks the server what is wrong with the text as it came, once (mount
  // did this before it drew anything), and stops the wait when the screen goes.
  useEffect(() => {
    alive.current = true;
    check();
    return () => {
      alive.current = false;
      clearTimeout(timer.current);
      editing.dispose();
    };
  }, []);

  // A text the band carried here is the field's, not the file's: it is put in
  // once and the part checks it as if it had been typed (spec-nav 1.5). What the
  // part reports is the screen's own text, not a draft to put in again — a
  // re-render that landed after a key would write the key's text back into the
  // field (r-1e9a). The screen carries the draft apart from the typed text.
  useEffect(() => {
    if (carried == null) return;
    const el = ta.current;
    if (!el || el.value === carried) return;
    editing.change(carried);
    syncField();
    check();
  }, [carried]);

  // The gutter follows the text when it is laid out again, as it did then.
  useEffect(() => {
    if (gutter.current && ta.current) gutter.current.scrollTop = ta.current.scrollTop;
  }, [laid, now.problems]);

  // The hash the page read is the one a save carries: the answer's own hash
  // travels through the screen, and comes back as this prop (5 章 2).
  useEffect(() => {
    if (hash) editing.rebase(hash);
  }, [hash]);

  useEffect(() => { if (ta.current && ta.current.value !== state.draft) { syncField(); check(); } }, [state.draft]);

  const lines = laid.split('\n');
  const marks = marksOf(now.problems);
  return <><header class="ph"><span class="ln">{label}</span><code class="path">{path}</code>{badge ? <span class={badge.cls}>{badge.text}</span> : ''}</header>
  <div class="ybody">
    <div class={'code h' + heightOf(lines.length)}><pre class="gutter" aria-hidden="true" ref={gutter}>{lines.map((_: string, i: number) => <span class={marks[i + 1] || ''}>{i + 1}</span>)}</pre><textarea class="ytext" data-file={file || undefined} wrap="off" spellcheck={false} autocomplete="off" aria-label={path} ref={ta} onInput={() => { foot(); schedule(); }} onScroll={() => { if (gutter.current && ta.current) gutter.current.scrollTop = ta.current.scrollTop; }} onKeyDown={(e: KeyboardEvent) => { if (e.key === 'Tab' && !e.shiftKey) { const target = e.target; if (target instanceof HTMLTextAreaElement) { e.preventDefault(); insertTab(target); } } }}>{initial.current}</textarea></div>
    <div class="vlist"><div class="vhead"><span class="vt">設定の検査</span>{now.problems.length ? <span class="vcount">{errs ? <span class="sev err">問題 {errs}</span> : ''}{warns ? <span class="sev warn">警告 {warns}</span> : ''}</span> : <span class="vclean">✓ 問題なし</span>}</div>{now.problems.map((p: Problem) => <button class="vitem" data-act="yaml-jump" data-line={p.line} onClick={() => jump(p.line)}><span class={'sev ' + (p.sev === 'err' ? 'err' : 'warn')}>{p.sev === 'err' ? '問題' : '警告'}</span><span class="lno">{p.line} 行</span><span class="msg">{p.text}</span></button>)}</div>
  </div>
  {conflict ? <ConflictNotice c={conflict} mine={state.saved || ''} undone={undone} onOverwrite={overwrite} onRevert={readIn} onUndo={undoRead} /> : null}
  <footer class="pf">
    <button class="btn primary" data-act="yaml-save" disabled={!dirty || grew || !!problem} onClick={() => save()}>保存</button><span class="dirty">{dirty ? '変更あり' : ''}</span>
    {problem ? <span class="sev err refusal" role="alert">{problem}</span> : ''}{refusal ? <span class="sev err refusal">{refusal}</span> : ''}{saved ? <span class="ok">保存しました</span> : ''}
    <span class="acts"><button class="btn" data-act="yaml-revert" data-danger data-danger-act="revertDraft" hidden={!dirty} onClick={revert}>元に戻す</button></span>
  </footer></>;
}


