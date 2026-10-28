// lib/ruledoc.js — the part a screen uses to edit one rule document and to show
// the Markdown a member reads (spec-teams 7), as a Preact component since the
// rewrite (webui-preact-design 段階 1-4). The role screen draws it and the team
// screen draws the same part: everything inside a panel that edits a document —
// the title and revision, the sections, the rules, the オン / オフ switch of a
// rule, the memo, the heading depth, the reordering (↑ ↓ and the ⋮ drag), and
// the Markdown box — lives here. What a screen keeps for itself is the panel's
// head and foot (where the document stands, 複製, 削除) and where the checks and
// writes go.
//
// The part takes what it reads and who it tells, as props (依頼書 4):
//
//   row          the document this panel edits: { uid, l, draft, saved, markdown,
//                diff, issues, e } — the screen's own reading of it
//   issues       the problems the last check named
//   nameError    the sentence a save refused the name with
//   hasName      whether the document has a name of its own to edit (a team's
//                rules document has not; the definition's name is the row's)
//   filtering    whether the list is showing only some rules
//   shown        which rule the filter keeps: (rule, issues) => boolean
//   query        the search the person typed: the part marks it in the text
//   readerCount  how many people name the document: (name) => number
//   onChange     the document changed: the screen draws its head and foot again
//                and asks the server what is wrong with the draft
//   onMode       the person asked for the edit ('edit') or the Markdown ('md')
//   onURL        the open rules and the caret's rule ride in the screen's address
//
// The part writes no markup outside Preact's render (webui-preact-design 5 章 1)
// and never reaches into a screen's state: a press changes the draft it was
// handed and says so through onChange.
import type { components, operations } from '../../../apicontract/api-types';
import { signal, useLayoutEffect, useRef } from './preact.js';

type ApiRuleDocument = components['schemas']['RoleDoc'] | components['schemas']['HQRoleDocument'] | components['schemas']['TeamRuleDocument'];

export type EditableRule = {
  _k: number;
  id: string;
  text: string;
  note: string;
  in_prompt: boolean;
  off_since: string;
};

export type EditableSection = {
  _s: number | null;
  heading: string;
  level: number;
  intro: string;
  rules: EditableRule[];
};

export type EditableDocument = {
  name: string;
  title: string;
  revision: number | null;
  sections: EditableSection[];
  __path?: string;
};

export type RuleIssue = {
  sev: components['schemas']['Problem']['sev'];
  k: number;
  f: string;
  msg: string;
  definition?: string;
};

export type RulePanel = {
  uid: number;
  l: operations['getTeam']['parameters']['query']['place'];
  e?: { saved: EditableDocument | null };
  saved: EditableDocument | null;
  draft: EditableDocument;
  name?: string;
  markdown: components['schemas']['RolePreview'] | components['schemas']['TeamRuleMarkdown'];
  diff: components['schemas']['RoleDiff'] | components['schemas']['TeamDiff'];
  issues?: RuleIssue[];
  savedIssues?: RuleIssue[];
  turns?: components['schemas']['RoleTurn'][] | components['schemas']['TeamRuleTurn'][] | null;
};

export type RuleDocProps = {
  row: RulePanel;
  issues: RuleIssue[];
  nameError?: string;
  hasName?: boolean;
  filtering?: boolean;
  shown?: (rule: EditableRule, issues: RuleIssue[]) => boolean;
  query?: string;
  readerCount?: (name: string) => number;
  onChange?: (row: RulePanel) => void;
  onMode?: (mode: 'edit' | 'md') => void;
  onURL?: () => void;
};

type RuleLocation = { si: number; ri: number; x: EditableRule };
type FocusPosition = { k: number; f: string };
type AskPosition = { sel: string; how: 'focus' | 'center' | 'nearest'; seq: number };
type FlashPosition = { k: number; seq: number };
type Message = Pick<RuleIssue, 'sev' | 'msg'>;
type Piece = { text?: string; node?: unknown };
type EditProps = { row: RulePanel; issues: RuleIssue[]; nameError?: string; h: RuleDocProps };
type PreviewProps = { row: RulePanel; issues: RuleIssue[] };
type FieldProps = { as?: 'input' | 'textarea'; value?: string | number | null; [key: string]: unknown };
type DragTarget = { si: number; k: number | null; before?: boolean };
type DragState = { row: RulePanel; key: string; k: number; to: DragTarget | null };

const clone = <T,>(o: T): T => JSON.parse(JSON.stringify(o)) as T;
const reEsc = (s: string): string => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
const num = (n: number): string => Number(n || 0).toLocaleString('ja-JP');
const $$ = (sel: string, root: ParentNode = document): Element[] => Array.from(root.querySelectorAll(sel));
let kseq = 0;

// ---- the document as the screen holds it -----------------------------
// fromJSON reads one document off the page's island into the draft a panel
// edits. Each rule keeps the key it was read with (_k) and each section the
// place it stood at (_s), so a rule or a section that is moved, or a value that
// is written back, is found on the file it came from (spec-roles 1.2.9,
// spec-teams C7). The numbering is the screen's too: it names an open rule in
// the address (`rule=lead.project.LEAD-1` is read back through _k), so the keys
// are handed out the same way they always were (nextKey, observeKeys).
function fromJSON(d: ApiRuleDocument): EditableDocument {
  return {
    name: d.name || '', title: d.title || '',
    revision: d.revision === null || d.revision === undefined ? null : d.revision,
    sections: (d.sections || []).map((s, si) => ({
      _s: si, heading: s.heading || '', level: s.level || 0, intro: s.intro || '',
      rules: (s.rules || []).map((x) => ({
        _k: ++kseq, id: x.id || '', text: x.text || '', note: x.note || '',
        in_prompt: x.in_prompt !== false, off_since: x.off_since || '',
      })),
    })),
  };
}

// toDoc is the draft as the server reads it. Each rule and each section carries
// where it stood in the saved document (its from), so the server can write the
// changed lines and carry a moved rule or section whole.
function toDoc(d: EditableDocument, saved: EditableDocument | null): components['schemas']['RoleDraft'] {
  const at = new Map<number, [number, number]>(), secAt = new Map<number, number>();
  if (saved) {
    saved.sections.forEach((s, si) => { if (s._s !== null) secAt.set(s._s, si); s.rules.forEach((x, ri) => at.set(x._k, [si, ri])); });
  }
  return {
    name: d.name, title: d.title, revision: d.revision,
    sections: (d.sections || []).map((s) => ({
      from: s._s !== null && secAt.has(s._s) ? secAt.get(s._s) : null,
      heading: s.heading, level: s.level, intro: s.intro,
      rules: (s.rules || []).map((x) => {
        const f = at.get(x._k);
        return {
          from: f ? { section: f[0], rule: f[1] } : null, id: x.id, text: x.text,
          note: x.note, in_prompt: x.in_prompt, off_since: x.off_since || '',
        };
      }),
    })),
  };
}

const rulesOf = (d: EditableDocument): EditableRule[] => d.sections.flatMap((s) => s.rules);
// nextID is the ID a rule the person just added carries (n-ec32). Its prefix is
// the one the document's own IDs use most — LEAD for a lead.yaml — or, in a
// document that has no such ID yet, the document's name in capital letters with
// everything but A-Z and 0-9 taken out (ui-impl -> UIIMPL; a name that is
// missing or starts with a digit gives R). The number is the one after the
// largest of that prefix, counting both the rules on the screen and the ones
// already saved, so the number of a rule just deleted is not used again.
const ID_RE = /^([A-Z][A-Z0-9]*)-(\d+)$/;
function nextID(row: RulePanel): string {
  const onScreen = rulesOf(row.draft), saved = row.saved ? rulesOf(row.saved) : [];
  const count = (list: EditableRule[]): Map<string, number> => {
    const n = new Map<string, number>();
    list.forEach((x) => { const m = ID_RE.exec(String(x.id || '').trim()); if (m) n.set(m[1], (n.get(m[1]) || 0) + 1); });
    return n;
  };
  let uses = count(onScreen);
  if (!uses.size) uses = count(saved);
  let prefix = '';
  uses.forEach((n, p) => { if (!prefix || n > (uses.get(prefix) || 0)) prefix = p; });
  if (!prefix) {
    prefix = String(row.name || (row.draft && row.draft.name) || '').toUpperCase().replace(/[^A-Z0-9]/g, '');
    if (!prefix || /^[0-9]/.test(prefix)) prefix = 'R';
  }
  let top = 0;
  onScreen.concat(saved).forEach((x) => { const m = ID_RE.exec(String(x.id || '').trim()); if (m && m[1] === prefix) top = Math.max(top, +m[2]); });
  return `${prefix}-${top + 1}`;
}
const findRule = (d: EditableDocument, k: string | number): RuleLocation | null => {
  for (const [si, s] of d.sections.entries()) {
    const ri = s.rules.findIndex((x) => x._k === Number(k));
    if (ri >= 0) return { si, ri, x: s.rules[ri] };
  }
  return null;
};
const savedOf = (row: RulePanel): EditableDocument | null => (row.e ? row.e.saved : row.saved);
const savedRule = (row: RulePanel, k: number): EditableRule | null => {
  const s = savedOf(row);
  return s ? (findRule(s, k)?.x || null) : null;
};
const ruleChanged = (row: RulePanel, x: EditableRule): boolean => JSON.stringify(savedRule(row, x._k) || null) !== JSON.stringify(x);
const secKey = (row: RulePanel, si: number): string => `${row.uid}${row.l}-s${si}`;
const rkey = (row: RulePanel, x: Pick<EditableRule, '_k'>): string => `${row.uid}${row.l}:${x._k}`;
const inP = (x: Pick<EditableRule, 'in_prompt'>): boolean => x.in_prompt !== false;
const outCount = (rules: EditableRule[]): number => rules.filter((x) => !inP(x)).length;
// A rule switched off in this draft has no day yet: the server writes the day on
// the save, so the column says what the save will do. A rule the file already
// held off with no day is the file's own いつからか不明 (d-85dc).
const sinceText = (row: RulePanel, x: EditableRule): string => {
  if (x.off_since) return `${String(x.off_since).slice(5).replace('-', '/')} から`;
  const sr = savedRule(row, x._k);
  return sr && !inP(sr) ? 'いつからか不明' : '保存すると今日から';
};
// The depth of a heading is shown the way it stands in the prompt: level 0 and 2
// are ##, 3 is ###, 4 is ####. There is no level — any more (A2, I6).
const lvlMark = (lv: number): string => '#'.repeat(lv && lv > 2 ? lv : 2);

// ---- state the part owns --------------------------------------------
// What the part knows about itself: which rules stand open, which section's
// heading depth is being picked, which delete is being confirmed, where the
// caret was, whether a rename carries, and where the caret is to go next. Each
// field is a signal of its own, and a change puts a new value in: the trees that
// read one are drawn again, and no one raises a number to say so
// (webui-components 1 章). The screens read two of them (roles keeps the open
// rules in the address, and reads the rename box when it saves).
const ropen = signal(new Set<string>());
const introOpen = signal<string | null>(null);
const secPick = signal<Record<string, 'lvl' | null>>({});
const confirm = signal<Record<string, 1>>({});
const pfocus = signal<Record<string, FocusPosition>>({});
const renameBox = signal<Record<string, boolean>>({});
const askAt = signal<AskPosition | null>(null);
const flashAt = signal<FlashPosition | null>(null);
let askSeq = 0;
// scrolled is the preview's own memory: where the box stood when it was drawn
// last. It is read while drawing to put the box back where a person left it, and
// written while the person scrolls. It never says "draw again", so it is not a
// signal — the box's own scrollTop is the same kind of thing.
const scrolled: Record<string, number> = {};

// The screens' way in: the rules that stand open, and whether a rename carries.
export const openRules = () => ropen.value;
export function openRuleKey(key: string): void {
  const next = new Set(ropen.value);
  next.add(key);
  ropen.value = next;
}
export const renameCarries = (key: string): boolean => renameBox.value[key] !== false;

// confirmAt and clearConfirms keep the delete's own confirmation: one key per
// section or rule, and every key of a row when its sections move.
const confirmAt = (key: string, on: boolean): void => {
  const next = { ...confirm.value };
  if (on) next[key] = 1; else delete next[key];
  confirm.value = next;
};
const clearConfirms = (head: string): void => {
  const next: Record<string, 1> = {};
  Object.keys(confirm.value).forEach((k) => { if (!k.startsWith(head)) next[k] = confirm.value[k]; });
  confirm.value = next;
};
const focusAt = (key: string, one: FocusPosition): void => { pfocus.value = { ...pfocus.value, [key]: one }; };

// ask says where the caret goes once the press has been drawn: the field the
// person's press named, in the panel that press came from (5 章 5). It is put
// there after the draw, in the part's own tree — never by looking for an element
// the screen drew.
function ask(sel?: string, how: AskPosition['how'] = 'focus'): void {
  if (!sel) return;
  askSeq += 1;
  askAt.value = { sel, how, seq: askSeq };
}

// applyAsk puts the caret where the last press asked for it, once the part has
// drawn. It runs after every draw and acts only on a new ask, so a redraw that
// nobody asked for never moves the caret.
function applyAsk(root: HTMLElement | null, last: { current: number }): void {
  const one = askAt.value;
  if (!one || !root || one.seq <= last.current) return;
  const el = root.querySelector<HTMLElement>(one.sel);
  if (!el) return;
  last.current = one.seq;
  if (one.how === 'center') {
    // The rule is brought into the middle of the window, so the band the screen
    // pins to the bottom never hides it; a rule taller than the window keeps its
    // head in it.
    el.scrollIntoView({ block: 'center' });
    const top = el.getBoundingClientRect().top;
    if (top < 0) window.scrollBy(0, top);
  } else if (one.how === 'nearest') {
    el.scrollIntoView({ block: 'nearest' });
  }
  el.focus({ preventScroll: true });
  if (el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement) el.select();
}

// applyFlash marks the rule a drag just moved: the class stands on the element
// until the part draws it again, the way it always did.
function applyFlash(root: HTMLElement | null, last: { current: number }): void {
  const one = flashAt.value;
  if (!one || !root || one.seq <= last.current) return;
  const el = root.querySelector<HTMLElement>(`.rule[data-k="${one.k}"]`);
  if (!el) return;
  last.current = one.seq;
  el.classList.add('flash');
}

// ---- the shape of a problem, as the mock draws it -------------------
// The screens draw a problem's chip, a list of them, and the オフ count with
// these; each reads the same five fields the strings did.
function Sev({ sev }: { sev: Message['sev'] }) {
  return <span class={'sev ' + (sev === 'err' ? 'err' : 'warn')}>{sev === 'err' ? '問題' : '警告'}</span>;
}
function Vm({ list }: { list: Message[] }) {
  return list.map((m) => <li><Sev sev={m.sev} /><span>{m.msg}</span></li>);
}
function Counts({ list }: { list: Message[] }) {
  const e = list.filter((m) => m.sev === 'err').length, w = list.length - e;
  return <>{e ? <span class="sev err">問題 {e}</span> : ''}{w ? <span class="sev warn">警告 {w}</span> : ''}</>;
}
function Badge({ list }: { list: Message[] }) {
  const e = list.filter((m) => m.sev === 'err').length;
  if (e) return <span class="sev err">問題 {e}</span>;
  return list.length ? <span class="sev warn">警告 {list.length}</span> : null;
}
function OutNote({ n }: { n: number }) {
  return n ? <span class="outn">オフ {n}</span> : null;
}

// ---- decoration ------------------------------------------------------
// A rule's id and summary carry the words the prompt will bold and the arrows a
// person reads as a hand-off. The three wrappings are laid on in the order they
// always were — an arrow first, then a model's name, then what the search asks
// for — and never one inside another.
const ARROW_RE = /([A-Za-z][\w-]*|人間)\s*(→|⇄)\s*([A-Za-z][\w-]*|人間)/g;
const MODEL_RE = /\b(claude|codex|opencode|gemini|opus|sonnet|haiku|fable|gpt-[\w.]+|deepseek[\w.-]*|qwen[\w.-]*)\b/gi;

// cut splits the pieces that are still plain text by one pattern, and wraps each
// match. A piece that is already a wrapped node is left alone, so nothing is
// wrapped twice.
function cut(pieces: Piece[], re: RegExp, wrap: (match: string) => unknown): Piece[] {
  const out: Piece[] = [];
  pieces.forEach((piece) => {
    if (piece.node || !piece.text) { out.push(piece); return; }
    let at = 0;
    for (const m of piece.text.matchAll(re)) {
      if (m.index > at) out.push({ text: piece.text.slice(at, m.index) });
      out.push({ node: wrap(m[0]) });
      at = m.index + m[0].length;
    }
    if (at < piece.text.length) out.push({ text: piece.text.slice(at) });
  });
  return out;
}
const flat = (pieces: Piece[]): unknown[] => pieces.map((p) => (p.node ? p.node : p.text));
const started = (text: string): Piece[] => (String(text) ? [{ text: String(text) }] : []);
const models = (text: string): unknown[] => flat(cut(started(text), MODEL_RE, (m) => <span class="mw">{m}</span>));

function deco(text: string, q: string): unknown[] {
  let pieces = started(text);
  pieces = cut(pieces, ARROW_RE, (m) => <span class="arrow">{m}</span>);
  pieces = cut(pieces, MODEL_RE, (m) => <span class="mw">{m}</span>);
  if (q) pieces = cut(pieces, new RegExp(reEsc(q), 'gi'), (m) => <mark>{m}</mark>);
  return flat(pieces);
}
function firstPhrase(t: string): string { const i = t.indexOf('。'); return i >= 0 && i < t.length - 1 ? t.slice(0, i + 1) : t; }
function snippet(t: string, q: string): string {
  const f = firstPhrase(t), want = (q || '').toLowerCase();
  if (!want || f.toLowerCase().includes(want) || !t.toLowerCase().includes(want)) return f;
  const hit = t.split(/(?<=。)/).find((x) => x.toLowerCase().includes(want));
  return hit ? `${f} … ${hit}` : f;
}

// Field is an input or a textarea whose value the part keeps in the draft. The
// value goes in only when the field stands somewhere else than the draft says —
// a rule put back with 元に戻す, a section drawn anew — and never while a person
// types: the field itself holds what was typed, the draft was written from it,
// and writing the same words back is what moves a caret and ends an IME's
// composition (webui-preact-design 5 章 1, 引き継ぎ 4 章 1).
function Field({ as: Tag = 'input', value, ...rest }: FieldProps) {
  const input = useRef<HTMLInputElement | null>(null);
  const textarea = useRef<HTMLTextAreaElement | null>(null);
  useLayoutEffect(() => {
    const field = Tag === 'textarea' ? textarea.current : input.current;
    if (!field) return;
    const want = value === null || value === undefined ? '' : String(value);
    if (field.value !== want) field.value = want;
  });
  return Tag === 'textarea' ? <textarea {...rest} ref={textarea} /> : <input {...rest} ref={input} />;
}


// ---- the edit --------------------------------------------------------
// Edit is the document's fields and rules; Preview is the Markdown beside it.
// Both read the row the screen hands them.
function Edit({ row, issues, nameError, h }: EditProps) {
  // The part's own state is read below (the open rules, the section being
  // picked, the confirmation, the rename box), so a change draws this tree.
  if (!row) return null;
  const d = row.draft, s = savedOf(row), key = row.uid + row.l;
  // The name's own problem stands under the name field, not in this list a
  // second time (d-841e).
  const doc = issues.filter((m) => m.k === 0 && m.f !== 'name');
  const nread = s && d.name && d.name !== s.name && h.readerCount ? h.readerCount(s.name) : 0;
  return <><div class="dh">
      {h.hasName === false ? '' : <div class="dhf"><label for={'n' + key}>ロール名</label><Field id={'n' + key} class={'in-name' + (s && s.name === d.name ? '' : ' changed')} data-f="name" spellcheck={false} autocomplete="off" value={d.name} /><span class="errline name" hidden={!nameError}>{nameError || ''}</span></div>}
      <div class="dhf"><label for={'t' + key}>タイトル</label><Field id={'t' + key} class={'in-title' + (s && s.title === d.title ? '' : ' changed')} data-f="title" autocomplete="off" value={d.title} /></div>
      <div class="dhf"><label for={'v' + key}>revision</label><Field id={'v' + key} class={'in-rev' + (s && s.revision === d.revision ? '' : ' changed')} data-f="revision" type="number" min="1" value={d.revision} /></div>
      <label class="rename" hidden={!nread}><input type="checkbox" id={'rn' + key} checked={renameBox.value[key] !== false} /><span>{nread ? `読んでいる ${nread} 人の role も ${d.name || '—'} に書き換える` : ''}</span></label>
    </div>
    <ul class="vm dhvm"><Vm list={doc} /></ul>
    {d.sections.map((_, si) => secHTML(row, si, issues, h))}
    <button class="add secadd" data-act="sec-add">＋ 節</button></>;
}

function secHTML(row: RulePanel, si: number, issues: RuleIssue[], h: RuleDocProps) {
  const d = row.draft, sec = d.sections[si], key = secKey(row, si), ck = `${row.uid}${row.l}s${si}`;
  const filtering = !!h.filtering;
  const rules = sec.rules.filter((x) => !filtering || !h.shown || h.shown(x, issues));
  if (filtering && !rules.length) return null;
  const a = (savedOf(row) || { sections: [] }).sections[si];
  const pick = secPick.value[ck];
  const opts = pick === 'lvl'
    ? [2, 3, 4].map((v) => {
      const on = lvlMark(sec.level) === lvlMark(v);
      return <button class={'lvl' + (on ? ' on' : '')} data-act="sec-level" data-v={v} aria-pressed={on}>{`${lvlMark(v)} ${sec.heading || '見出し'}`}</button>;
    })
    : null;
  return <section class="sec" data-si={si}>
      <div class="sech">
      <button class={'lvl' + (!a || lvlMark(a.level) !== lvlMark(sec.level) ? ' changed' : '')} data-f="level" data-act="sec-pick" data-w="lvl" aria-expanded={pick === 'lvl'} aria-label="見出しの深さ">{lvlMark(sec.level)}</button>
      <Field id={'h' + key} class={'in-head' + (!a || a._s !== sec._s || a.heading !== sec.heading ? ' changed' : '')} data-f="heading" value={sec.heading} placeholder="見出し" autocomplete="off" />
      <span class="n">{`${sec.rules.length} 規則`}<OutNote n={outCount(sec.rules)} /></span>
      <span class="end"><button class="x" data-act="sec-up" disabled={si === 0} aria-label="節を上へ">↑</button><button class="x" data-act="sec-down" disabled={si === d.sections.length - 1} aria-label="節を下へ">↓</button>{secEnd(sec, ck)}</span>
    </div>
    {opts ? <div class="pickrow">{opts}</div> : ''}
    {sec.intro !== '' || introOpen.value === key ? <Field as="textarea" id={'in' + key} class={'intro' + (!a || a.intro !== sec.intro ? ' changed' : '')} data-f="intro" rows="1" placeholder="前置き" value={sec.intro} /> : ''}
    <div class="rules">{rules.map((x) => ruleHTML(row, x, issues, h))}</div>
    <button class="add secadd" data-act="rule-add">＋ 規則</button>
  </section>;
}

// secEnd is the section's own end of the head: ＋ 前置き, and the × that asks
// before a section with its rules is taken out.
function secEnd(sec: EditableSection, ck: string) {
  const adding = sec.intro === '' && !confirm.value[ck] ? <button class="add sm" data-act="intro-add">＋ 前置き</button> : '';
  const confirming = confirm.value[ck]
    ? <><button class="btn danger solid sm" data-act="sec-del-yes">{`節を削除する（${sec.rules.length} 規則）`}</button><button class="btn sm" data-act="sec-del-no">やめる</button></>
    : <button class="x" data-act="sec-del" aria-label={`節 ${sec.heading} を消す`}>×</button>;
  return <>{adding}{confirming}</>;
}

function ruleHTML(row: RulePanel, x: EditableRule, issues: RuleIssue[], h: RuleDocProps) {
  const open = ropen.value.has(rkey(row, x));
  const mine = issues.filter((m) => m.k === x._k);
  const cls = ['rule', open ? 'open' : '', ruleChanged(row, x) ? 'unsaved' : '', inP(x) ? '' : 'out'].filter(Boolean).join(' ');
  return <div class={cls} data-k={x._k}>
      <div class="rsum" role="button" tabIndex={0} aria-expanded={open} data-act="rtoggle">{rsumInner(row, x, mine, h)}</div>
      {open ? ruleEditHTML(row, x, mine) : ''}
    </div>;
}

function rsumInner(row: RulePanel, x: EditableRule, mine: RuleIssue[], h: RuleDocProps) {
  const id = String(x.id || '');
  return <><span class="grip" aria-hidden="true">⋮</span>
      <span class="rid"><b>{id ? deco(id, h.query || '') : <span class="none">—</span>}</b><Badge list={mine} /></span>
      <span class="rtx">{deco(snippet(String(x.text || ''), h.query || ''), h.query || '')}</span>
      <span class="marks">{x.note ? <span class="mk memo">メモ</span> : ''}{inP(x) ? '' : <span class="since">{sinceText(row, x)}</span>}<button class="onsw" type="button" role="switch" aria-checked={inP(x)} data-act="rule-toggle" aria-label={`${String(x.id || '')} をプロンプトに${inP(x) ? '入れている' : '入れていない'}`}>{inP(x) ? 'オン' : 'オフ'}</button></span></>;
}

function ruleEditHTML(row: RulePanel, x: EditableRule, mine: RuleIssue[]) {
  const key = `${row.uid}${row.l}-${x._k}`, ck = 'r' + key;
  const sr = savedRule(row, x._k);
  const field = (name: string): RuleIssue[] => mine.filter((m) => (m.f.split('.')[0] || '') === name);
  const bad = (name: string, errOnly = false): boolean => mine.some((m) => (m.f.split('.')[0] || '') === name && (!errOnly || m.sev === 'err'));
  const changed = (name: 'text' | 'note'): boolean => !sr || sr[name] !== x[name];
  const id = String(x.id || '');
  return <div class="redit">
      {inP(x) ? '' : <div class="offinfo">{`この規則はオフ（プロンプトに入れていない）。${sinceText(row, x)}。`}</div>}
      <div class="field"><span class="fl">ID</span><div><span class="ridv" aria-label="ID">{id ? id : <span class="none">—</span>}</span><ul class="vm" data-for="id"><Vm list={field('id')} /></ul></div></div>
      <div class="field"><label for={'tx' + key}>本文</label><div><Field as="textarea" id={'tx' + key} class={['ta', changed('text') ? 'changed' : '', bad('text', true) ? 'bad' : ''].filter(Boolean).join(' ')} data-f="text" rows="2" value={x.text} /><ul class="vm" data-for="text"><Vm list={field('text')} /></ul></div></div>
      <div class="field"><label for={'nt' + key}>メモ</label><div><Field as="textarea" id={'nt' + key} class={['ta', changed('note') ? 'changed' : '', bad('note') ? 'bad' : ''].filter(Boolean).join(' ')} data-f="note" rows="2" value={x.note} /><div class="notehint">なぜ書いたか・何を見て書いたかを残す。AI には送らない。消すか迷ったときに読む。</div><ul class="vm" data-for="note"><Vm list={field('note')} /></ul></div></div>
      <footer class="rf">
        <button class="btn sm" data-act="rule-up" aria-label="上へ">↑</button><button class="btn sm" data-act="rule-down" aria-label="下へ">↓</button>
        <span class="acts">
          <button class="btn sm" data-act="rule-revert" hidden={!sr || !ruleChanged(row, x)}>元に戻す</button>
          {confirm.value[ck] ? <><button class="btn danger solid sm" data-act="rule-del-yes">規則を削除する</button><button class="btn sm" data-act="rule-del-no">やめる</button></>
          : <button class="btn danger sm" data-act="rule-del">削除</button>}
        </span>
      </footer>
    </div>;
}

// ---- the Markdown preview -------------------------------------------
function mdHL(t: string): { h: number; node: unknown[] } {
  let m = t.match(/^(#+) (.*)$/);
  if (m) return { h: 0, node: [<span class="mm">{m[1]} </span>, <span class={'mh h' + m[1].length}>{m[2]}</span>] };
  m = t.match(/^- \*\*(.*?)\*\* (.*)$/);
  if (m) return { h: 2, node: [<span class="mm">- **</span>, <b>{m[1]}</b>, <span class="mm">**</span>, ' ', models(m[2])] };
  m = t.match(/^(- |\d+\. |   )(.*)$/);
  if (m) return { h: m[1].length, node: [<span class="mm">{m[1]}</span>, models(m[2])] };
  return { h: 0, node: models(t) };
}

function ruleKeyAt(d: EditableDocument, si: number, ri: number): number {
  const s = d.sections[si];
  if (!s) return 0;
  const x = s.rules[ri];
  return x ? x._k : 0;
}

function Preview({ row, issues }: PreviewProps) {
  const box = useRef<HTMLDivElement | null>(null);
  // The caret's own place is read below: a press that moves it draws this tree.
  const key = row ? row.uid + row.l : '';
  // The box keeps the place the person scrolled to, and the line they are in is
  // brought into it, after every draw of the preview — the same two moves the
  // part made after writing the box before the rewrite.
  useLayoutEffect(() => {
    const el = box.current;
    if (!el || !row) return;
    el.scrollTop = scrolled[key] || 0;
    follow(el, key);
  });
  if (!row) return null;
  const md = row.markdown || { lines: [], chars: 0 }, lines = md.lines || [];
  const df = row.diff || { added: [], removedBefore: [] };
  const fc0 = pfocus.value[key];
  const fc = fc0 && { k: fc0.k, f: ['id', 'text', 'note'].includes(fc0.f) ? 'text' : fc0.k ? '' : fc0.f };
  const added = new Set(df.added || []), removedBefore = new Set(df.removedBefore || []);
  let ri = -1;
  const rows = lines.map((z) => {
    const k = z.section >= 0 ? ruleKeyAt(row.draft, z.section, z.rule) : 0;
    if (z.part === 'excluded') {
      return <div class={['yl ghost go', fc && fc.k && k === fc.k ? 'cur' : ''].filter(Boolean).join(' ')} data-act="yline" data-k={k} data-part="rule"><span class="lno"></span><span class="yt" data-h="0" style={{ '--h': '0' }}><span class="mk out">オフ</span><s>{z.text || ''}</s></span></div>;
    }
    ri++;
    // A problem about the document as a whole (no rule of its own) is marked on
    // the first line alone; a problem about a rule is marked on the lines that
    // rule stands on (spec-roles 1.2.6). A heading or a blank line has no rule
    // of its own, so it is not one of a rule's lines.
    const ms = issues.filter((m) => m.f !== 'name' && (k ? m.k === k : m.k === 0 && ri === 0));
    const sev = ms.some((m) => m.sev === 'err') ? 'err' : ms.length ? 'warn' : '';
    const { h, node } = mdHL(String(z.text || ''));
    const go = k || z.section >= 0 || z.part === 'title';
    const here = fc && fc.k && k === fc.k;
    const around = here && fc.f === 'text';
    const near = fc && !fc.k && (z.part === 'title' || z.part === 'heading' || z.part === 'intro')
      && ((z.part === 'title' && fc.f === 'title') || (z.part !== 'title' && (fc.f || '').startsWith('sec.')));
    const cls = ['yl', sev, added.has(ri) ? 'ins' : '', removedBefore.has(ri) ? 'del' : '', here ? 'cur' : '', around || near ? 'curf' : '', go ? 'go' : ''].filter(Boolean).join(' ');
    // Only a line a person can go to carries the act and the place it goes to;
    // some lines carry the problems that stand on them as their title.
    const props: Record<string, string | number> = go ? { 'data-act': 'yline', 'data-k': k, 'data-si': z.section, 'data-part': z.part } : {};
    if (ms.length) props.title = ms.map((m) => m.msg).join('\n');
    return <div class={cls} {...props}><span class="lno">{ri + 1}</span><span class="yt" data-h={h} style={{ '--h': String(h) }}>{node && node.length ? node : ' '}</span></div>;
  });
  const nadd = added.size, ndel = removedBefore.size;
  return <><div class="yph"><span class="pkl">Markdown</span><span class="yinfo"><span class="vt">{`${ri + 1} 行 · ${num(md.chars)} 文字`}</span>{nadd || ndel ? <span class="dirty">{`+${nadd} −${ndel}`}</span> : ''}<span class="yr">{issues.length ? <span class="vcount"><Counts list={issues} /></span> : <span class="vclean">✓ 問題なし</span>}</span></span></div>
    <div class="ybox" ref={box} onScroll={() => { if (box.current && row) scrolled[key] = box.current.scrollTop; }}>{rows}</div></>;
}

// follow brings the line the caret is on into the box: a line taller than the
// window keeps its head in it, the same as the rule the screen opens.
function follow(box: HTMLDivElement, key: string): void {
  const el = box.querySelector<HTMLElement>('.yl.curf') || box.querySelector<HTMLElement>('.yl.cur');
  if (!el || !box.clientHeight) return;
  const last = [...box.querySelectorAll<HTMLElement>(el.classList.contains('curf') ? '.yl.curf' : '.yl.cur')].pop();
  if (!last) return;
  const top = el.offsetTop, bot = last.offsetTop + last.offsetHeight;
  if (top < box.scrollTop + 4 || bot > box.scrollTop + box.clientHeight - 4) box.scrollTop = Math.max(0, top - Math.round(box.clientHeight * 0.3));
  scrolled[key] = box.scrollTop;
}

function focusField(t: Element): string {
  const f = t.closest<HTMLElement>('[data-f]')?.dataset.f || '';
  if (['id', 'text', 'note', 'title', 'revision'].includes(f)) return f;
  if (['heading', 'level'].includes(f)) { const se = t.closest<HTMLElement>('.sec'); return se ? `sec.${se.dataset.si || ''}` : ''; }
  if (f === 'intro') { const se = t.closest<HTMLElement>('.sec'); return se ? `sec.${se.dataset.si || ''}.intro` : ''; }
  return '';
}

// ---- marking a drawn panel ------------------------------------------
// diffCount is how many places the draft changed: a value written, a rule that
// left the section it stood in, a section whose own keys changed — and one for
// an order that only moved, whether one rule or one section (spec-roles I4, I5:
// 並びだけが替わったら 1).
function diffCount(e: Pick<RulePanel, 'saved' | 'draft'>): number {
  const s = e.saved, d = e.draft;
  if (!s) return 1;
  // A rule counts when its values changed, or when it left the section it stood
  // in. Section identity travels with a section (_s), so rules that travel with
  // their section are not counted one by one.
  const secOfSaved = new Map<number, number | null>(), secOfDraft = new Map<number, number | null>();
  s.sections.forEach((sec) => sec.rules.forEach((x) => secOfSaved.set(x._k, sec._s)));
  d.sections.forEach((sec) => sec.rules.forEach((x) => secOfDraft.set(x._k, sec._s)));
  let n = Number(s.name !== d.name) + Number(s.title !== d.title) + Number(s.revision !== d.revision);
  const sr = new Map<number, string>(rulesOf(s).map((x) => [x._k, JSON.stringify(x)])), dr = rulesOf(d);
  dr.forEach((x) => {
    if (sr.get(x._k) !== JSON.stringify(x) || secOfSaved.get(x._k) !== secOfDraft.get(x._k)) n++;
  });
  sr.forEach((_, k) => { if (!dr.some((x) => x._k === k)) n++; });
  // A section counts when its own keys changed or when it is new; a section that
  // was taken out counts once, below.
  const bySec = new Map<number | null, EditableSection>(s.sections.map((sec) => [sec._s, sec]));
  const order: Array<number | null> = [];
  d.sections.forEach((sec) => {
    const a = bySec.get(sec._s);
    if (!a) { n++; return; }
    if (a.heading !== sec.heading || a.level !== sec.level || a.intro !== sec.intro) n++;
    order.push(sec._s);
  });
  s.sections.forEach((sec) => { if (!d.sections.some((x) => x._s === sec._s)) n++; });
  const savedOrder = s.sections.map((sec) => sec._s).filter((k) => order.includes(k));
  if (order.join() !== savedOrder.join()) n++;
  // The order of the rules within the document, when nothing else changed.
  if (!n && sr.size && rulesOf(s).map((x) => x._k).join() !== dr.map((x) => x._k).join()) n++;
  return n;
}

// ---- reordering ------------------------------------------------------
function moveRule(d: EditableDocument, si: number, ri: number, dir: -1 | 1): void {
  const s = d.sections[si], removed = s.rules.splice(ri, 1), x = removed[0];
  if (ri + dir >= 0 && ri + dir <= s.rules.length) s.rules.splice(ri + dir, 0, x);
  else if (dir < 0 && si > 0) d.sections[si - 1].rules.push(x);
  else if (dir > 0 && si < d.sections.length - 1) d.sections[si + 1].rules.unshift(x);
  else s.rules.splice(ri, 0, x);
}

let drag: DragState | null = null;
const clearDrop = (): void => $$('.drop-b, .drop-a, .drop-end').forEach((el) => el.classList.remove('drop-b', 'drop-a', 'drop-end'));

// ---- the operations --------------------------------------------------
// The drag classes are put on the elements themselves while the pointer is over
// them: the panel is not drawn again until the drop, so nothing takes them away.
// The parts a press reads are the DOM's and the drafts', as they were.
type RuleContext = { row: RulePanel; d: EditableDocument; x: EditableRule | null; si: number; ri: number; oi: number | null; key: string };
function ctx(row: RulePanel, el: Element): RuleContext {
  const d = row.draft;
  const ruleEl = el.closest<HTMLElement>('.rule'), f = ruleEl ? findRule(d, ruleEl.dataset.k || '') : null;
  const secEl = el.closest<HTMLElement>('.sec'), si = secEl ? +(secEl.dataset.si || 0) : null;
  const orderEl = el.closest<HTMLElement>('.orow'), oi = orderEl ? +(orderEl.dataset.oi || 0) : null;
  return {
    row, d, x: f?.x || null, si: f ? f.si : (si ?? 0), ri: f ? f.ri : 0, oi,
    key: `${row.uid}${row.l}-${f ? f.x._k : ''}`,
  };
}

// edited says the draft changed: the screen draws its head and foot again and
// asks the server what is wrong with the document.
function edited(h: RuleDocProps, row: RulePanel): void { if (h.onChange) h.onChange(row); }
function url(h: RuleDocProps): void { if (h.onURL) h.onURL(); }
function mode(h: RuleDocProps, v: 'edit' | 'md'): void { if (h.onMode) h.onMode(v); }

// openRule opens one rule of a document and puts the cursor in its body
// (spec-roles 1.2.4, 1.2.7, P15, 監査 R12): a screen that jumps to a rule from a
// check, or from the address, reads through this, so the rule stands open where
// the editor shows, brought into the window — never under the save's band — with
// the body ready to be typed in. A rule the document does not hold leaves the
// panel as it is.
export function openRule(row: RulePanel, k: number): void {
  if (!row || !k) return;
  ropen.value = new Set(ropen.value).add(rkey(row, { _k: k }));
  focusAt(row.uid + row.l, { k, f: 'text' });
  ask(`.rule[data-k="${k}"] [data-f=text]`, 'center');
}

function handleClick(h: RuleDocProps, b: HTMLElement): boolean {
  const act = b.dataset.act, c = ctx(h.row, b), row = c.row;
  if (!row) return false;
  const pkey = row.uid + row.l, ck = c.x ? 'r' + c.key : '';
  const sk = c.si != null ? `${row.uid}${row.l}s${c.si}` : '';
  switch (act) {
    case 'rtoggle': {
      if (!c.x) break;
      const k = rkey(row, c.x);
      {
        const next = new Set(ropen.value);
        if (next.has(k)) next.delete(k); else next.add(k);
        ropen.value = next;
      }
      ask(`.rule[data-k="${c.x._k}"] .rsum`); url(h); break;
    }
    case 'sec-pick': {
      const w = b.dataset.w === 'lvl' ? 'lvl' : null;
      secPick.value = { ...secPick.value, [sk]: secPick.value[sk] === w ? null : w };
      ask(`.sec[data-si="${c.si}"] [data-act=sec-pick][data-w=${b.dataset.w}]`); break;
    }
    case 'sec-level': {
      const s0 = c.d.sections[c.si], v = +(b.dataset.v || 0);
      // Choosing ## for a section the file wrote no level for leaves it unwritten
      // (0), so the save adds no level: line (A2).
      s0.level = v === 2 && !s0.level ? 0 : v;
      secPick.value = { ...secPick.value, [sk]: null };
      if (s0.rules[0]) focusAt(pkey, { k: s0.rules[0]._k, f: 'text' });
      ask(`.sec[data-si="${c.si}"] [data-act=sec-pick][data-w=lvl]`); edited(h, row); break;
    }
    case 'sec-add': c.d.sections.push({ _s: null, heading: '', level: 0, intro: '', rules: [] }); ask(`.sec[data-si="${c.d.sections.length - 1}"] [data-f=heading]`); edited(h, row); break;
    case 'sec-del': confirmAt(sk, true); ask(`.sec[data-si="${c.si}"] [data-act=sec-del-yes]`); break;
    case 'sec-del-no': confirmAt(sk, false); ask(); break;
    case 'sec-del-yes': confirmAt(sk, false); c.d.sections.splice(c.si, 1); ask(); edited(h, row); break;
    case 'sec-up': case 'sec-down': {
      const j = c.si + (act === 'sec-up' ? -1 : 1), ss = c.d.sections;
      if (j < 0 || j >= ss.length) break;
      const at = ss[c.si]; ss[c.si] = ss[j]; ss[j] = at;
      secPick.value = {};
      clearConfirms(`${row.uid}${row.l}s`);
      ask(`.sec[data-si="${j}"] [data-act=${act}]`); edited(h, row); break;
    }
    case 'intro-add': introOpen.value = secKey(row, c.si); ask(`.sec[data-si="${c.si}"] [data-f=intro]`); break;
    case 'rule-add': {
      const x = { _k: ++kseq, id: nextID(row), text: '', note: '', in_prompt: true, off_since: '' };
      c.d.sections[c.si].rules.push(x);
      ropen.value = new Set(ropen.value).add(rkey(row, x));
      ask(`.rule[data-k="${x._k}"] [data-f=text]`); edited(h, row); break;
    }
    case 'rule-up': case 'rule-down': {
      if (!c.x) break;
      moveRule(c.d, c.si, c.ri, act === 'rule-up' ? -1 : 1);
      ask(`.rule[data-k="${c.x._k}"] [data-act=${act}]`); edited(h, row); break;
    }
    case 'rule-revert': {
      if (!c.x) break;
      const sx = clone(savedRule(row, c.x._k));
      if (sx) { sx._k = c.x._k; c.d.sections[c.si].rules[c.ri] = sx; }
      ask(`.rule[data-k="${c.x._k}"] .rsum`); edited(h, row); break;
    }
    case 'rule-del': {
      if (!c.x) break;
      confirmAt(ck, true); ask(`.rule[data-k="${c.x._k}"] [data-act=rule-del-yes]`); break;
    }
    case 'rule-del-no': confirmAt(ck, false); ask(); break;
    case 'rule-del-yes': {
      if (!c.x) break;
      confirmAt(ck, false); c.d.sections[c.si].rules.splice(c.ri, 1); ask(); edited(h, row); break;
    }
    case 'rule-toggle': {
      if (!c.x) break;
      c.x.in_prompt = !inP(c.x);
      if (inP(c.x)) c.x.off_since = '';
      ask(`.rule[data-k="${c.x._k}"] .onsw`);
      edited(h, row); break;
    }
    case 'yline': {
      const k = +(b.dataset.k || 0), si = b.dataset.si || '', part = b.dataset.part || '';
      if (k) ropen.value = new Set(ropen.value).add(`${row.uid}${row.l}:${k}`);
      mode(h, 'edit');
      focusAt(pkey, k ? { k, f: 'text' } : part === 'title' ? { k: 0, f: 'title' } : part === 'intro' ? { k: 0, f: `sec.${si}.intro` } : { k: 0, f: `sec.${si}` });
      // The Markdown box follows to the section's first rule: the panel is drawn
      // again first, then the caret is put in the field the press named (A2, P15).
      const target = k ? `.rule[data-k="${k}"] [data-f=text]` : part === 'title' ? '[data-f=title]' : `.sec[data-si="${si}"] [data-f=${part === 'intro' ? 'intro' : 'heading'}]`;
      ask(target, 'nearest');
      url(h); break;
    }
    default: return false;
  }
  return true;
}

function handleInput(h: RuleDocProps, t: HTMLInputElement | HTMLTextAreaElement): boolean {
  const row = h.row;
  if (!row) return false;
  const c = ctx(row, t);
  if (!c.d || !t.dataset.f) return false;
  const v = t.value;
  switch (t.dataset.f) {
    // The panel's head draws the path from the draft the press changes, so the
    // screen is told the draft moved.
    case 'name': c.d.name = v; if (h.onChange) h.onChange(row); break;
    case 'title': c.d.title = v; break;
    case 'revision': c.d.revision = v === '' ? null : +v; break;
    case 'heading': c.d.sections[c.si].heading = v; break;
    case 'intro': c.d.sections[c.si].intro = v; break;
    case 'id': if (c.x) c.x.id = v; break;
    case 'text': if (c.x) c.x.text = v; break;
    case 'note': if (c.x) c.x.note = v; break;
    default: return false;
  }
  edited(h, row);
  return true;
}

// ---- the part as one component --------------------------------------
// RuleDoc draws the two containers a panel's .pmain holds — the edit and the
// preview — and answers every press, typing and drag inside them. The screen
// hands it the document and the ways to tell it things (the header above).
export function RuleDoc(props: RuleDocProps) {
  const edit = useRef<HTMLDivElement | null>(null);
  const lastAsk = useRef(0), lastFlash = useRef(0);
  const h = props;
  // The caret and the flash a press asked for are this component's to put in
  // place after the draw, so it reads them itself: a press that asks for either
  // draws this component even when the screen drew nothing.
  askAt.value;
  flashAt.value;
  useLayoutEffect(() => {
    applyAsk(edit.current, lastAsk);
    applyFlash(edit.current, lastFlash);
  });
  return <><div class="pedit" ref={edit}
      onClick={(e: MouseEvent) => { const target = e.target; const b = target instanceof Element ? target.closest<HTMLElement>('[data-act]') : null; if (b) handleClick(h, b); }}
      onInput={(e: Event) => { const target = e.target; if (target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement) handleInput(h, target); }}
      onChange={(e: Event) => {
        const target = e.target;
        const cb = target instanceof Element ? target.closest<HTMLInputElement>('.rename input[type=checkbox]') : null;
        if (!cb) return;
        renameBox.value = { ...renameBox.value, [`${h.row.uid}${h.row.l}`]: cb.checked };
      }}
      onKeyDown={(e: KeyboardEvent) => { const target = e.target; if ((e.key === 'Enter' || e.key === ' ') && target instanceof HTMLElement && target.matches('.rsum')) { e.preventDefault(); target.click(); } }}
      onFocusIn={(e: FocusEvent) => {
        const t = e.target;
        if (!(t instanceof Element) || !t.closest('.pedit')) return;
        const c = ctx(h.row, t);
        const k = h.row.uid + h.row.l, fc = { k: c.x ? c.x._k : 0, f: focusField(t) };
        const was = pfocus.value[k];
        if (was && was.k === fc.k && was.f === fc.f) return;
        // The preview marks the line the caret is in: its own state changed, so
        // the preview is drawn from it.
        focusAt(k, fc);
      }}
      onPointerDown={(e: PointerEvent) => {
        const target = e.target;
        const grip = target instanceof Element ? target.closest<HTMLElement>('.grip') : null;
        const rule = grip?.closest<HTMLElement>('.rule');
        if (rule) rule.draggable = true;
      }}
      onDragStart={(e: DragEvent) => {
        const target = e.target;
        const el = target instanceof Element ? target.closest<HTMLElement>('.rule[draggable=true]') : null;
        if (!el) return;
        drag = { row: h.row, key: h.row.uid + h.row.l, k: +(el.dataset.k || 0), to: null };
        if (e.dataTransfer) {
          e.dataTransfer.effectAllowed = 'move';
          try { e.dataTransfer.setData('text/plain', el.dataset.k || ''); } catch (err) { /* the move is inside this screen */ }
        }
        el.classList.add('dragging');
      }}
      onDragOver={(e: DragEvent) => {
        // A rule moves inside its own panel: a drag over another panel's rules is
        // not a drop this panel takes.
        const active = drag;
        if (!active || active.key !== h.row.uid + h.row.l) return;
        const target = e.target;
        const sec = target instanceof Element ? target.closest<HTMLElement>('.sec') : null;
        if (!sec) return;
        e.preventDefault(); clearDrop();
        const el = target instanceof Element ? target.closest<HTMLElement>('.rule') : null;
        if (el) {
          const rc = el.getBoundingClientRect(), before = e.clientY < rc.top + rc.height / 2;
          el.classList.add(before ? 'drop-b' : 'drop-a');
          active.to = { si: +(sec.dataset.si || 0), k: +(el.dataset.k || 0), before };
        } else {
          sec.classList.add('drop-end');
          active.to = { si: +(sec.dataset.si || 0), k: null };
        }
      }}
      onDrop={(e: DragEvent) => {
        const active = drag;
        if (!active || !active.to || active.key !== h.row.uid + h.row.l) return;
        e.preventDefault();
        const row = active.row, d = row.draft, to = active.to;
        const from = findRule(d, active.k);
        if (from && to.k !== active.k) {
          const [x] = d.sections[from.si].rules.splice(from.ri, 1), rs = d.sections[to.si].rules;
          if (x) rs.splice(to.k == null ? rs.length : rs.findIndex((y) => y._k === to.k) + (to.before ? 0 : 1), 0, x);
        }
        const k = active.k;
        drag = null; clearDrop();
        flashAt.value = { k, seq: (flashAt.value ? flashAt.value.seq : 0) + 1 };
        edited(h, row);
      }}
      onDragEnd={() => { $$('.rule.dragging, .rule[draggable=true]').forEach((el) => { el.classList.remove('dragging'); if (el instanceof HTMLElement) el.draggable = false; }); clearDrop(); drag = null; }}
      ><Edit row={h.row} issues={h.issues} nameError={h.nameError} h={h} /></div><aside class="yprev" aria-label="Markdown" onClick={(e: MouseEvent) => { const target = e.target; const b = target instanceof Element ? target.closest<HTMLElement>('[data-act]') : null; if (b) handleClick(h, b); }}><Preview row={h.row} issues={h.issues} /></aside></>;
}

// nextKey hands out the key of a rule a person adds, and observeKeys takes the
// keys a draft that came from elsewhere already holds: together they keep the
// keys of one document together across a save, a carried draft and a screen's
// own newrow (the address names an open rule by its key).
const nextKey = (): number => ++kseq;
const observeKeys = (rules: EditableRule[]): void => { rules.forEach((x) => { if (x._k > kseq) kseq = x._k; }); };

// ---- the screens' way in (4 章) ----
// A screen imports the part's names rather than reading them off a global.
export {
  Counts, Badge, Vm, Sev, OutNote,
  rulesOf, findRule, fromJSON, toDoc, diffCount, inP,
  outCount, nextKey, observeKeys, clone,
};
