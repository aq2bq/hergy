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
import { useEffect, useRef, useState } from './preact.js';
import { check as apiCheck, save as apiSave, problemText, saved as apiSaved } from './api.js';
import type { RawAnswer } from './api.js';
import type { components, operations } from '../../../apicontract/api-types';

type Place = operations['getTeam']['parameters']['query']['place'];
type YamlKind = 'role' | 'team' | 'profile';
type Problem = components['schemas']['Problem'];
type CheckRequest = components['schemas']['RoleCheckRequest']
  | components['schemas']['TeamCheckRequest']
  | components['schemas']['ProfileCheckRequest'];
type SaveRequest = components['schemas']['RoleSaveRequest']
  | components['schemas']['TeamSaveRequest']
  | components['schemas']['ProfileSaveRequest'];
type SavedJson = components['schemas']['RoleSaved']
  | components['schemas']['TeamSaved']
  | components['schemas']['ProfileSaved'];
type ApiAnswer<T> = RawAnswer<T>;
type YamlState = { base: string; text: string; hash: string };
type YamlWorkingState = { text: string; problems: Problem[]; baseProblems: Problem[] };

export type YamlTabProps = {
  label: string;
  path: string;
  text: string;
  hash: string;
  kind: YamlKind;
  place: Place | string;
  name: string;
  problems?: Problem[];
  badge?: { cls: string; text: string } | null;
  file?: string;
  extra?: Pick<components['schemas']['TeamCheckRequest'], 'part'>;
  carried?: string | null;
  onProblems?: (problems: Problem[]) => void;
  onRevert?: () => void;
  onState?: (state: YamlState) => void;
  onSaved?: (answer: ApiAnswer<SavedJson>) => void;
};

function isProblem(value: unknown): value is Problem {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return false;
  const problem = Object.fromEntries(Object.entries(value));
  return (problem.sev === 'err' || problem.sev === 'warn')
    && typeof problem.at === 'string'
    && typeof problem.text === 'string'
    && typeof problem.section === 'number'
    && typeof problem.rule === 'number'
    && (problem.field === undefined || typeof problem.field === 'string')
    && (problem.line === undefined || typeof problem.line === 'number')
    && (problem.definition === undefined || typeof problem.definition === 'string')
    && (problem.role === undefined || typeof problem.role === 'string')
    && (problem.rule_id === undefined || typeof problem.rule_id === 'string');
}

function isProblemList(value: unknown): value is Problem[] {
  return Array.isArray(value) && value.every(isProblem);
}


// checkText asks the server what is wrong with one file's text. The kind says
// which files these are, and extra carries the kind's own word for which one
// (a team definition's team.yaml and agreement.yaml share one kind); the same
// words answer a panel's draft.
export async function checkText(
  kind: YamlKind,
  place: Place | string,
  name: string,
  text: string,
  extra?: YamlTabProps['extra'],
): Promise<Problem[]> {
  try {
    // A profile's file is known from its place; only a role's file and a team
    // definition's read a name from the body (n-3f6c).
    const request: CheckRequest = { place, ...(kind === 'profile' ? {} : { name }), text, ...extra };
    const answer = await apiCheck(kind, request);
    return (answer.json && answer.json.problems) || [];
  } catch (e) {
    // A refusal still carries what the server said about the text; a reading
    // that never arrived leaves no problems, as it always did.
    if (typeof e !== 'object' || e === null || !('json' in e)) return [];
    const json: unknown = e.json;
    if (typeof json !== 'object' || json === null || !('problems' in json)) return [];
    const problems = json.problems;
    return isProblemList(problems) ? problems : [];
  }
}

// saveText writes one file's whole text. The answer carries the new hash. The
// name travels in the payload as well as in the URL: a kind that reads the
// file's name from the body rather than from the path (a role's file is the
// row's own name) writes the file it was handed (r-8e85).
export function saveText(
  kind: YamlKind,
  name: string,
  place: Place | string,
  text: string,
  hash: string,
  extra?: YamlTabProps['extra'],
): Promise<ApiAnswer<SavedJson>> {
  const request: SaveRequest = { place, ...(kind === 'profile' ? {} : { name }), text, hash: hash || '', ...extra };
  return apiSave(kind, name, request);
}

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

const errTexts = (problems: Problem[] | null | undefined): string[] => (problems || []).filter((p) => p.sev === 'err').map((p) => p.text);
const heightOf = (lines: number): number => Math.min(Math.max(lines, 6), 34);
const marksOf = (problems: Problem[] | null | undefined): Record<number, Problem['sev']> => {
  const marks: Record<number, Problem['sev']> = {};
  (problems || []).forEach((p) => {
    if (p.line !== undefined && marks[p.line] !== 'err') marks[p.line] = p.sev;
  });
  return marks;
};

// YamlTab is one file's section contents. text is the text the page came with
// and does not change under the part: it is the textarea's first content, not a
// value the part keeps putting back — the person's typing lives in the field
// itself, so no render moves the caret or undoes a change. laid is the text the
// gutter was last drawn from: the same moment the old part laid it out, when the
// server's problems came back.
//
// What the part reads and who it tells (依頼書 4):
//   file, label, path, badge, text, hash, kind, place, name, extra, carried
//   problems   the problems the last check named
//   onProblems(list)      the check answered: the file's own problems
//   onSaved(answer)       the save wrote the file: the screen keeps its hash
//   onState({base,text,hash})  what the part holds, for the band's draft
//   onRevert()            the person pressed 元に戻す, before the text goes back
export function YamlTab(props: YamlTabProps) {
  const { label, path, badge, file, text, hash, kind, place, name, extra, problems, onProblems, onSaved, onState, onRevert, carried } = props;
  const ta = useRef<HTMLTextAreaElement | null>(null);
  const gutter = useRef<HTMLPreElement | null>(null);
  const timer = useRef<number>(0);
  const alive = useRef(true);
  const seq = useRef(0);
  const base = useRef(text);
  const mine = useRef(hash || '');

  const [now, setNow] = useState<YamlWorkingState>({ text, problems: problems || [], baseProblems: problems || [] });
  const [laid, setLaid] = useState(text);
  const [saved, setSaved] = useState(false);
  const [refusal, setRefusal] = useState('');

  const dirty = now.text !== base.current;
  const errs = now.problems.filter((p: Problem) => p.sev === 'err').length;
  const warns = now.problems.length - errs;
  const grew = now.problems.some((p: Problem) => p.sev === 'err' && errTexts(now.baseProblems).indexOf(p.text) < 0);

  // foot keeps the 保存 / 元に戻す in step with the text and the problems: a text
  // that has grown an error the written file did not have cannot be saved (the
  // server refuses it anyway, but the button says so first). An error the file
  // already had does not stop a save (spec-roles X3).
  function foot() {
    const el = ta.current;
    if (!el) return;
    const value = el.value;
    setNow((was: YamlWorkingState) => (was.text === value ? was : { ...was, text: value }));
    if (value !== base.current) setSaved(false);
    tellDirty();
    report();
  }

  // report says what the part holds to a screen that keeps it as its own state.
  function report() {
    const el = ta.current;
    if (!el || !onState) return;
    onState({ base: base.current, text: el.value, hash: mine.current });
  }

  // layout draws what the server said about the text: the numbered gutter with a
  // mark on each bad line, the height the file needs, and the list of problems
  // under the text.
  function layout(next: Problem[]) {
    const el = ta.current;
    if (!el) return;
    const value = el.value;
    setLaid(value);
    setNow((was: YamlWorkingState) => ({ text: value, problems: next, baseProblems: value === base.current ? next : was.baseProblems }));
    if (onProblems) onProblems(next);
    foot();
  }

  async function check() {
    const el = ta.current;
    if (!el) return;
    const mine = (seq.current += 1);
    const found = await checkText(kind, place, name, el.value, extra);
    // Only the newest reading's answer is laid out: the check the mount started
    // can answer after the one the typing started, and the older text's problems
    // must not stand over the newer text (5 章 3).
    if (!alive.current || mine !== seq.current) return;
    layout(found);
  }

  function schedule() {
    clearTimeout(timer.current);
    timer.current = window.setTimeout(() => { check(); }, 300);
  }

  async function save() {
    const el = ta.current;
    if (!el) return;
    setRefusal('');
    try {
      const answer = await saveText(kind, name, place, el.value, mine.current, extra);
      if (!alive.current) return;
      base.current = el.value;
      if (answer.json && answer.json.hash) mine.current = answer.json.hash;
      if (onSaved) onSaved({ ok: true, status: answer.status, json: answer.json, text: answer.text });
      layout(now.problems);
      check();
      setSaved(true);
      report();
      apiSaved();
    } catch (e) {
      if (!alive.current) return;
      setRefusal(problemText(e, '保存できませんでした'));
    }
  }

  function revert() {
    const el = ta.current;
    if (!el) return;
    if (onRevert) onRevert();
    el.value = base.current;
    setRefusal('');
    layout(now.problems);
    check();
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
    };
  }, []);

  // A text the band carried here is the field's, not the file's: it is put in
  // once, and the part checks it as if it had been typed (spec-nav 1.5).
  useEffect(() => {
    if (carried == null) return;
    const el = ta.current;
    if (!el || el.value === carried) return;
    el.value = carried;
    autosize(el);
    layout(now.problems);
    check();
  }, [carried]);

  // The gutter follows the text when it is laid out again, as it did then.
  useEffect(() => {
    if (gutter.current && ta.current) gutter.current.scrollTop = ta.current.scrollTop;
  }, [laid, now.problems]);

  // The hash the page read is the one a save carries: the answer's own hash
  // travels through the screen, and comes back as this prop (5 章 2).
  useEffect(() => {
    if (hash) mine.current = hash;
  }, [hash]);

  const lines = laid.split('\n');
  const marks = marksOf(now.problems);
  return <><header class="ph"><span class="ln">{label}</span><code class="path">{path}</code>{badge ? <span class={badge.cls}>{badge.text}</span> : ''}</header>
  <div class="ybody">
    <div class={'code h' + heightOf(lines.length)}><pre class="gutter" aria-hidden="true" ref={gutter}>{lines.map((_: string, i: number) => <span class={marks[i + 1] || ''}>{i + 1}</span>)}</pre><textarea class="ytext" data-file={file || undefined} wrap="off" spellcheck={false} autocomplete="off" aria-label={path} ref={ta} onInput={() => { foot(); schedule(); }} onScroll={() => { if (gutter.current && ta.current) gutter.current.scrollTop = ta.current.scrollTop; }} onKeyDown={(e: KeyboardEvent) => { if (e.key === 'Tab' && !e.shiftKey) { const target = e.target; if (target instanceof HTMLTextAreaElement) { e.preventDefault(); insertTab(target); } } }}>{text}</textarea></div>
    <div class="vlist"><div class="vhead"><span class="vt">validate</span>{now.problems.length ? <span class="vcount">{errs ? <span class="sev err">問題 {errs}</span> : ''}{warns ? <span class="sev warn">警告 {warns}</span> : ''}</span> : <span class="vclean">✓ 問題なし</span>}</div>{now.problems.map((p: Problem) => <button class="vitem" data-act="yaml-jump" data-line={p.line} onClick={() => jump(p.line)}><span class={'sev ' + (p.sev === 'err' ? 'err' : 'warn')}>{p.sev === 'err' ? '問題' : '警告'}</span><span class="lno">{p.line} 行</span><span class="msg">{p.text}</span></button>)}</div>
  </div>
  <footer class="pf">
    <button class="btn primary" data-act="yaml-save" disabled={!dirty || grew} onClick={save}>保存</button><span class="dirty">{dirty ? '変更あり' : ''}</span>
    {refusal ? <span class="sev err refusal">{refusal}</span> : ''}{saved ? <span class="ok">保存しました</span> : ''}
    <span class="acts"><button class="btn" data-act="yaml-revert" hidden={!dirty} onClick={revert}>元に戻す</button></span>
  </footer></>;
}


