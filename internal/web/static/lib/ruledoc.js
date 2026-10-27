// lib/ruledoc.js — the part a screen uses to edit one rule document and to show
// the Markdown a member reads (spec-teams 7), as a Preact component since the
// rewrite (webui-preact-design 段階 1-4). The role screen mounts it and the team
// screen mounts the same part: everything inside a panel that edits a document —
// the title and revision, the sections, the rules, the オン / オフ switch of a
// rule, the memo, the heading depth, the reordering (↑ ↓ and the ⋮ drag), and
// the Markdown box — lives here. What a screen keeps for itself is the panel's
// head and foot (where the document stands, 複製, 削除) and where the checks and
// writes go.
//
// The part draws the inside of two elements the screen made: `.pmain`'s `.pedit`
// (the edit) and `.pmain`'s `.yprev` (the preview), plus the one `.vcount` the
// screen's foot leaves empty. It draws them with Preact's render and writes no
// markup (webui-preact-design 5 章 1). The foot's other words — the places the
// document changed, whether 保存 can be pressed, the rename hint's box — stay
// the bridge's, as they were, until the two screens are modules.
//
// The presses on those elements come from the screen that owns the view: it
// hands the view to mount below, which listens once and reads the `data-act`
// the part put in its markup, so a press is handled once. A press that changes
// the document asks the screen to draw the panel again (host.rerender): the
// panel's head and foot are the screen's, and the address the open rules live in
// is the screen's too.
//
// While the screens are still classic scripts, they hand the view to
// window.hergyRuleDoc.mount and read the same names they always did: body (the
// skeleton a panel's .pmain starts from), mark (draw one written panel), state
// (the open rules, the confirmations, the field the caret is in), and the few
// words a screen draws inside its own strings (badge, counts, vmItem, sevChip).
import { html, render as draw, signal, useLayoutEffect, useRef } from '../vendor/preact.js';

const clone = (o) => JSON.parse(JSON.stringify(o));
const esc = (s) => String(s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
const reEsc = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
const num = (n) => Number(n || 0).toLocaleString('ja-JP');
const $ = (sel, root = document) => root.querySelector(sel);
const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));
let kseq = 0;

// ---- the document as the screen holds it -----------------------------
// fromJSON reads one document off the page's island into the draft a panel
// edits. Each rule keeps the key it was read with (_k) and each section the
// place it stood at (_s), so a rule or a section that is moved, or a value that
// is written back, is found on the file it came from (spec-roles 1.2.9,
// spec-teams C7). The numbering is the screen's too: it names an open rule in
// the address (`rule=lead.project.LEAD-1` is read back through _k), so the keys
// are handed out the same way they always were (nextKey, observeKeys).
function fromJSON(d) {
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
function toDoc(d, saved) {
  const at = new Map(), secAt = new Map();
  if (saved) {
    saved.sections.forEach((s, si) => { secAt.set(s._s, si); s.rules.forEach((x, ri) => at.set(x._k, [si, ri])); });
  }
  return {
    name: d.name, title: d.title, revision: d.revision,
    sections: (d.sections || []).map((s) => ({
      from: secAt.has(s._s) ? secAt.get(s._s) : null,
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

const rulesOf = (d) => d.sections.flatMap((s) => s.rules);
// nextID is the ID a rule the person just added carries (n-ec32). Its prefix is
// the one the document's own IDs use most — LEAD for a lead.yaml — or, in a
// document that has no such ID yet, the document's name in capital letters with
// everything but A-Z and 0-9 taken out (ui-impl -> UIIMPL; a name that is
// missing or starts with a digit gives R). The number is the one after the
// largest of that prefix, counting both the rules on the screen and the ones
// already saved, so the number of a rule just deleted is not used again.
const ID_RE = /^([A-Z][A-Z0-9]*)-(\d+)$/;
function nextID(row) {
  const onScreen = rulesOf(row.draft), saved = row.saved ? rulesOf(row.saved) : [];
  const count = (list) => {
    const n = new Map();
    list.forEach((x) => { const m = ID_RE.exec(String(x.id || '').trim()); if (m) n.set(m[1], (n.get(m[1]) || 0) + 1); });
    return n;
  };
  let uses = count(onScreen);
  if (!uses.size) uses = count(saved);
  let prefix = '';
  uses.forEach((n, p) => { if (!prefix || n > uses.get(prefix)) prefix = p; });
  if (!prefix) {
    prefix = String(row.name || (row.draft && row.draft.name) || '').toUpperCase().replace(/[^A-Z0-9]/g, '');
    if (!prefix || /^[0-9]/.test(prefix)) prefix = 'R';
  }
  let top = 0;
  onScreen.concat(saved).forEach((x) => { const m = ID_RE.exec(String(x.id || '').trim()); if (m && m[1] === prefix) top = Math.max(top, +m[2]); });
  return `${prefix}-${top + 1}`;
}
const findRule = (d, k) => { for (const [si, s] of d.sections.entries()) { const ri = s.rules.findIndex((x) => x._k === Number(k)); if (ri >= 0) return { si, ri, x: s.rules[ri] }; } return null; };
const savedOf = (row) => (row.e ? row.e.saved : row.saved);
const savedRule = (row, k) => { const s = savedOf(row); return s ? (findRule(s, k) || { x: null }).x : null; };
const ruleChanged = (row, x) => JSON.stringify(savedRule(row, x._k) || null) !== JSON.stringify(x);
const secKey = (row, si) => `${row.uid}${row.l}-s${si}`;
const rkey = (row, x) => `${row.uid}${row.l}:${x._k}`;
const inP = (x) => x.in_prompt !== false;
const outCount = (rules) => rules.filter((x) => !inP(x)).length;
// A rule switched off in this draft has no day yet: the server writes the day on
// the save, so the column says what the save will do. A rule the file already
// held off with no day is the file's own いつからか不明 (d-85dc).
const sinceText = (row, x) => {
  if (x.off_since) return `${String(x.off_since).slice(5).replace('-', '/')} から`;
  const sr = savedRule(row, x._k);
  return sr && !inP(sr) ? 'いつからか不明' : '保存すると今日から';
};
// The depth of a heading is shown the way it stands in the prompt: level 0 and 2
// are ##, 3 is ###, 4 is ####. There is no level — any more (A2, I6).
const lvlMark = (lv) => '#'.repeat(lv && lv > 2 ? lv : 2);

// ---- state the part owns --------------------------------------------
// The screens read and write this object themselves (roles keeps the rules it
// opened in state.ropen, and reads it back when it writes the address), so it
// stays one plain object for as long as they are classic scripts. `rev` is the
// number the part raises when it changes the object itself: the trees below read
// it, so a change only the part knows about is drawn without waiting for the
// screen to draw the panel again. A change a screen makes is drawn when the
// screen draws the panel (which is what those screens do after a change).
const S = { ropen: new Set(), introOpen: null, secPick: {}, confirm: {}, pfocus: {}, yscroll: {} };
const rev = signal(0);
const touched = () => { rev.value += 1; };

let host = {};
let root = null;
const rowOf = (panel) => (panel && host.row ? host.row(panel) : null);

// ---- the shape of a problem, as the mock draws it -------------------
// The string forms are what a classic screen puts inside its own markup; the
// components are the same words for the part's own drawing. They read the same
// five fields, and the string forms go when the last screen is a module.
const sevChip = (sev) => `<span class="sev ${sev === 'err' ? 'err' : 'warn'}">${sev === 'err' ? '問題' : '警告'}</span>`;
const vmItem = (m) => `<li>${sevChip(m.sev)}<span>${esc(m.msg)}</span></li>`;
const counts = (list) => {
  const e = list.filter((m) => m.sev === 'err').length, w = list.length - e;
  return (e ? `<span class="sev err">問題 ${e}</span>` : '') + (w ? `<span class="sev warn">警告 ${w}</span>` : '');
};
const badge = (list) => {
  const e = list.filter((m) => m.sev === 'err').length;
  return e ? `<span class="sev err">問題 ${e}</span>` : list.length ? `<span class="sev warn">警告 ${list.length}</span>` : '';
};
const outNote = (n) => (n ? `<span class="outn">オフ ${n}</span>` : '');

function Sev({ sev }) {
  return html`<span class=${'sev ' + (sev === 'err' ? 'err' : 'warn')}>${sev === 'err' ? '問題' : '警告'}</span>`;
}
function Vm({ list }) {
  return list.map((m) => html`<li><${Sev} sev=${m.sev} /><span>${m.msg}</span></li>`);
}
function Counts({ list }) {
  const e = list.filter((m) => m.sev === 'err').length, w = list.length - e;
  return html`${e ? html`<span class="sev err">問題 ${e}</span>` : ''}${w ? html`<span class="sev warn">警告 ${w}</span>` : ''}`;
}
function Badge({ list }) {
  const e = list.filter((m) => m.sev === 'err').length;
  if (e) return html`<span class="sev err">問題 ${e}</span>`;
  return list.length ? html`<span class="sev warn">警告 ${list.length}</span>` : null;
}
function OutNote({ n }) {
  return n ? html`<span class="outn">オフ ${n}</span>` : null;
}

// ---- decoration ------------------------------------------------------
// A rule's id and summary carry the words the prompt will bold and the arrows a
// person reads as a hand-off. The three wrappings are laid on in the order they
// always were — an arrow first, then a model's name, then what the search asks
// for — and never one inside another.
const ARROW_RE = /([A-Za-z][\w-]*|人間)\s*(→|⇄)\s*([A-Za-z][\w-]*|人間)/g;
const MODEL_RE = /\b(claude|codex|opencode|gemini|opus|sonnet|haiku|fable|gpt-[\w.]+|deepseek[\w.-]*|qwen[\w.-]*)\b/gi;
const arrowsOf = (text) => [...String(text).matchAll(ARROW_RE)].map((m) => ({ a: m[1], op: m[2], b: m[3], s: m[0] }));

// cut splits the pieces that are still plain text by one pattern, and wraps each
// match. A piece that is already a wrapped node is left alone, so nothing is
// wrapped twice.
function cut(pieces, re, wrap) {
  const out = [];
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
const flat = (pieces) => pieces.map((p) => (p.node ? p.node : p.text));
const started = (text) => (String(text) ? [{ text: String(text) }] : []);
const models = (text) => flat(cut(started(text), MODEL_RE, (m) => html`<span class="mw">${m}</span>`));

function deco(text) {
  let pieces = started(text);
  pieces = cut(pieces, ARROW_RE, (m) => html`<span class="arrow">${m}</span>`);
  pieces = cut(pieces, MODEL_RE, (m) => html`<span class="mw">${m}</span>`);
  const q = host.query ? host.query() : '';
  if (q) pieces = cut(pieces, new RegExp(reEsc(q), 'gi'), (m) => html`<mark>${m}</mark>`);
  return flat(pieces);
}
function firstPhrase(t) { const i = t.indexOf('。'); return i >= 0 && i < t.length - 1 ? t.slice(0, i + 1) : t; }
function snippet(t) {
  const f = firstPhrase(t), q = (host.query ? host.query() : '').toLowerCase();
  if (!q || f.toLowerCase().includes(q) || !t.toLowerCase().includes(q)) return f;
  const hit = t.split(/(?<=。)/).find((x) => x.toLowerCase().includes(q));
  return hit ? `${f} … ${hit}` : f;
}

// Field is an input or a textarea whose value the part keeps in the draft. The
// value goes in only when the field stands somewhere else than the draft says —
// a rule put back with 元に戻す, a section drawn anew — and never while a person
// types: the field itself holds what was typed, the draft was written from it,
// and writing the same words back is what moves a caret and ends an IME's
// composition (webui-preact-design 5 章 1, 引き継ぎ 4 章 1).
function Field({ as: Tag = 'input', value, ...rest }) {
  const el = useRef(null);
  useLayoutEffect(() => {
    const field = el.current;
    if (!field) return;
    const want = value === null || value === undefined ? '' : String(value);
    if (field.value !== want) field.value = want;
  });
  return html`<${Tag} ...${rest} ref=${el} />`;
}

// paint draws one piece of a screen's panel for the first time by taking what
// the screen put there (a skeleton, or nothing) out of the way, and draws it
// again and again after that on the same element — so the fields a person is in,
// and the box they scrolled, are the same elements afterwards.
const painted = new WeakSet();
function paint(el, vnode) {
  if (!el) return;
  if (!painted.has(el)) {
    el.replaceChildren();
    painted.add(el);
  }
  draw(vnode, el);
}

// ---- the edit --------------------------------------------------------
// Edit is the document's fields and rules; Preview is the Markdown beside it.
// Both read the row the screen holds — the bridge finds that row from the panel
// element, a screen that is a module hands it in.
function Edit({ row, issues, nameError }) {
  // Reading the part's own change count subscribes this tree to it (see S).
  rev.value;
  if (!row) return null;
  const d = row.draft, s = savedOf(row), key = row.uid + row.l;
  const doc = issues.filter((m) => m.k === 0);
  const nread = s && d.name && d.name !== s.name && host.readerCount ? host.readerCount(s.name) : 0;
  return html`<div class="dh">
      ${host.hasName === false ? '' : html`<div class="dhf"><label for=${'n' + key}>名前</label><${Field} id=${'n' + key} class=${'in-name' + (s && s.name === d.name ? '' : ' changed')} data-f="name" spellcheck=${false} autocomplete="off" value=${d.name} /><span class="errline name" hidden=${!nameError}>${nameError || ''}</span></div>`}
      <div class="dhf"><label for=${'t' + key}>タイトル</label><${Field} id=${'t' + key} class=${'in-title' + (s && s.title === d.title ? '' : ' changed')} data-f="title" autocomplete="off" value=${d.title} /></div>
      <div class="dhf"><label for=${'v' + key}>revision</label><${Field} id=${'v' + key} class=${'in-rev' + (s && s.revision === d.revision ? '' : ' changed')} data-f="revision" type="number" min="1" value=${d.revision} /></div>
      <label class="rename" hidden=${!nread}><input type="checkbox" id=${'rn' + key} defaultChecked=${true} /><span>${nread ? `読んでいる ${nread} 人の role も ${d.name || '—'} に書き換える` : ''}</span></label>
    </div>
    <ul class="vm dhvm"><${Vm} list=${doc} /></ul>
    ${d.sections.map((sec, si) => secHTML(row, si, issues))}
    <button class="add secadd" data-act="sec-add">＋ 節</button>`;
}

function secHTML(row, si, issues) {
  const d = row.draft, sec = d.sections[si], key = secKey(row, si), ck = `${row.uid}${row.l}s${si}`;
  const filtering = host.filtering ? host.filtering() : false;
  const rules = sec.rules.filter((x) => !filtering || host.shown(x, row.issues));
  if (filtering && !rules.length) return null;
  const a = (savedOf(row) || { sections: [] }).sections[si];
  const pick = S.secPick[ck];
  const opts = pick === 'lvl'
    ? [2, 3, 4].map((v) => {
      const on = lvlMark(sec.level) === lvlMark(v);
      return html`<button class=${'lvl' + (on ? ' on' : '')} data-act="sec-level" data-v=${v} aria-pressed=${on}>${`${lvlMark(v)} ${sec.heading || '見出し'}`}</button>`;
    })
    : null;
  return html`<section class="sec" data-si=${si}>
      <div class="sech">
      <button class=${'lvl' + (!a || lvlMark(a.level) !== lvlMark(sec.level) ? ' changed' : '')} data-f="level" data-act="sec-pick" data-w="lvl" aria-expanded=${pick === 'lvl'} aria-label="見出しの深さ">${lvlMark(sec.level)}</button>
      <${Field} id=${'h' + key} class=${'in-head' + (!a || a._s !== sec._s || a.heading !== sec.heading ? ' changed' : '')} data-f="heading" value=${sec.heading} placeholder="見出し" autocomplete="off" />
      <span class="n">${`${sec.rules.length} 規則`}<${OutNote} n=${outCount(sec.rules)} /></span>
      <span class="end"><button class="x" data-act="sec-up" disabled=${si === 0} aria-label="節を上へ">↑</button><button class="x" data-act="sec-down" disabled=${si === d.sections.length - 1} aria-label="節を下へ">↓</button>${secEnd(row, sec, si, ck)}</span>
    </div>
    ${opts ? html`<div class="pickrow">${opts}</div>` : ''}
    ${sec.intro !== '' || S.introOpen === key ? html`<${Field} as="textarea" id=${'in' + key} class=${'intro' + (!a || a.intro !== sec.intro ? ' changed' : '')} data-f="intro" rows="1" placeholder="前置き" value=${sec.intro} />` : ''}
    <div class="rules">${rules.map((x) => ruleHTML(row, x, issues))}</div>
    <button class="add secadd" data-act="rule-add">＋ 規則</button>
  </section>`;
}

// secEnd is the section's own end of the head: ＋ 前置き, and the × that asks
// before a section with its rules is taken out.
function secEnd(row, sec, si, ck) {
  const adding = sec.intro === '' && !S.confirm[ck] ? html`<button class="add sm" data-act="intro-add">＋ 前置き</button>` : '';
  const confirm = S.confirm[ck]
    ? html`<button class="btn danger solid sm" data-act="sec-del-yes">${`節を削除する（${sec.rules.length} 規則）`}</button><button class="btn sm" data-act="sec-del-no">やめる</button>`
    : html`<button class="x" data-act="sec-del" aria-label=${`節 ${sec.heading} を消す`}>×</button>`;
  return html`${adding}${confirm}`;
}

function ruleHTML(row, x, issues) {
  const open = S.ropen.has(rkey(row, x));
  const mine = issues.filter((m) => m.k === x._k);
  const cls = ['rule', open ? 'open' : '', ruleChanged(row, x) ? 'unsaved' : '', inP(x) ? '' : 'out'].filter(Boolean).join(' ');
  return html`<div class=${cls} data-k=${x._k}>
      <div class="rsum" role="button" tabindex="0" aria-expanded=${open} data-act="rtoggle">${rsumInner(row, x, mine)}</div>
      ${open ? ruleEditHTML(row, x, mine) : ''}
    </div>`;
}

function rsumInner(row, x, mine) {
  const id = String(x.id || '');
  return html`<span class="grip" aria-hidden="true">⋮</span>
      <span class="rid"><b>${id ? deco(id) : html`<span class="none">—</span>`}</b><${Badge} list=${mine} /></span>
      <span class="rtx">${deco(snippet(String(x.text || '')))}</span>
      <span class="marks">${x.note ? html`<span class="mk memo">メモ</span>` : ''}${inP(x) ? '' : html`<span class="since">${sinceText(row, x)}</span>`}<button class="onsw" type="button" role="switch" aria-checked=${inP(x)} data-act="rule-toggle" aria-label=${`${String(x.id || '')} をプロンプトに${inP(x) ? '入れている' : '入れていない'}`}>${inP(x) ? 'オン' : 'オフ'}</button></span>`;
}

function ruleEditHTML(row, x, mine) {
  const key = `${row.uid}${row.l}-${x._k}`, ck = 'r' + key;
  const sr = savedRule(row, x._k);
  const field = (name) => mine.filter((m) => (m.f.split('.')[0] || '') === name);
  const bad = (name, errOnly) => mine.some((m) => (m.f.split('.')[0] || '') === name && (!errOnly || m.sev === 'err'));
  const changed = (name) => !sr || sr[name] !== x[name];
  const id = String(x.id || '');
  return html`<div class="redit">
      ${inP(x) ? '' : html`<div class="offinfo">${`この規則はオフ（プロンプトに入れていない）。${sinceText(row, x)}。`}</div>`}
      <div class="field"><span class="fl">ID</span><div><span class="ridv" aria-label="ID">${id ? id : html`<span class="none">—</span>`}</span><ul class="vm" data-for="id"><${Vm} list=${field('id')} /></ul></div></div>
      <div class="field"><label for=${'tx' + key}>本文</label><div><${Field} as="textarea" id=${'tx' + key} class=${['ta', changed('text') ? 'changed' : '', bad('text', true) ? 'bad' : ''].filter(Boolean).join(' ')} data-f="text" rows="2" value=${x.text} /><ul class="vm" data-for="text"><${Vm} list=${field('text')} /></ul></div></div>
      <div class="field"><label for=${'nt' + key}>メモ</label><div><${Field} as="textarea" id=${'nt' + key} class=${['ta', changed('note') ? 'changed' : '', bad('note') ? 'bad' : ''].filter(Boolean).join(' ')} data-f="note" rows="2" value=${x.note} /><div class="notehint">なぜ書いたか・何を見て書いたかを残す。AI には渡さない。消すか迷ったときに読む。</div><ul class="vm" data-for="note"><${Vm} list=${field('note')} /></ul></div></div>
      <footer class="rf">
        <button class="btn sm" data-act="rule-up" aria-label="上へ">↑</button><button class="btn sm" data-act="rule-down" aria-label="下へ">↓</button>
        <span class="acts">
          <button class="btn sm" data-act="rule-revert" hidden=${!sr || !ruleChanged(row, x)}>元に戻す</button>
          ${S.confirm[ck] ? html`<button class="btn danger solid sm" data-act="rule-del-yes">規則を削除する</button><button class="btn sm" data-act="rule-del-no">やめる</button>`
          : html`<button class="btn danger sm" data-act="rule-del">削除</button>`}
        </span>
      </footer>
    </div>`;
}

// ---- the Markdown preview -------------------------------------------
function mdHL(t) {
  let m = t.match(/^(#+) (.*)$/);
  if (m) return { h: 0, node: [html`<span class="mm">${m[1]} </span>`, html`<span class=${'mh h' + m[1].length}>${m[2]}</span>`] };
  m = t.match(/^- \*\*(.*?)\*\* (.*)$/);
  if (m) return { h: 2, node: [html`<span class="mm">- **</span>`, html`<b>${m[1]}</b>`, html`<span class="mm">**</span>`, ' ', models(m[2])] };
  m = t.match(/^(- |\d+\. |   )(.*)$/);
  if (m) return { h: m[1].length, node: [html`<span class="mm">${m[1]}</span>`, models(m[2])] };
  return { h: 0, node: models(t) };
}

function ruleKeyAt(d, si, ri) {
  const s = d.sections[si];
  if (!s) return 0;
  const x = s.rules[ri];
  return x ? x._k : 0;
}

function Preview({ row, issues }) {
  const box = useRef(null);
  rev.value;
  const key = row ? row.uid + row.l : '';
  // The box keeps the place the person scrolled to, and the line they are in is
  // brought into it, after every draw of the preview — the same two moves the
  // part made after writing the box before the rewrite.
  useLayoutEffect(() => {
    const el = box.current;
    if (!el || !row) return;
    el.scrollTop = S.yscroll[key] || 0;
    follow(el, key);
  });
  if (!row) return null;
  const md = row.markdown || { lines: [], chars: 0 }, lines = md.lines || [];
  const df = row.diff || { added: [], removedBefore: [] };
  const fc0 = S.pfocus[key];
  const fc = fc0 && { k: fc0.k, f: ['id', 'text', 'note'].includes(fc0.f) ? 'text' : fc0.k ? '' : fc0.f };
  const added = new Set(df.added || []), removedBefore = new Set(df.removedBefore || []);
  let ri = -1;
  const rows = lines.map((z) => {
    const k = z.section >= 0 ? ruleKeyAt(row.draft, z.section, z.rule) : 0;
    if (z.part === 'excluded') {
      return html`<div class=${['yl ghost go', fc && fc.k && k === fc.k ? 'cur' : ''].filter(Boolean).join(' ')} data-act="yline" data-k=${k} data-part="rule"><span class="lno"></span><span class="yt" data-h="0" style=${{ '--h': '0' }}><span class="mk out">オフ</span><s>${z.text || ''}</s></span></div>`;
    }
    ri++;
    // A problem about the document as a whole (no rule of its own) is marked on
    // the first line alone; a problem about a rule is marked on the lines that
    // rule stands on (spec-roles 1.2.6). A heading or a blank line has no rule of
    // its own, so it is not one of a rule's lines.
    const ms = issues.filter((m) => (k ? m.k === k : m.k === 0 && ri === 0));
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
    const props = go ? { 'data-act': 'yline', 'data-k': k, 'data-si': z.section, 'data-part': z.part } : {};
    if (ms.length) props.title = ms.map((m) => m.msg).join('\n');
    return html`<div class=${cls} ...${props}><span class="lno">${ri + 1}</span><span class="yt" data-h=${h} style=${{ '--h': String(h) }}>${node && node.length ? node : ' '}</span></div>`;
  });
  const nadd = added.size, ndel = removedBefore.size;
  return html`<div class="yph"><span class="pkl">Markdown</span><span class="yinfo"><span class="vt">${`${ri + 1} 行 · ${num(md.chars)} 文字`}</span>${nadd || ndel ? html`<span class="dirty">${`+${nadd} −${ndel}`}</span>` : ''}<span class="yr">${issues.length ? html`<span class="vcount"><${Counts} list=${issues} /></span>` : html`<span class="vclean">✓ 問題なし</span>`}</span></span></div>
    <div class="ybox" ref=${box}>${rows}</div>`;
}

// follow brings the line the caret is on into the box: a line taller than the
// window keeps its head in it, the same as the rule the screen opens.
function follow(box, key) {
  const el = box.querySelector('.yl.curf') || box.querySelector('.yl.cur');
  if (!el || !box.clientHeight) return;
  const last = [...box.querySelectorAll(el.classList.contains('curf') ? '.yl.curf' : '.yl.cur')].pop();
  const top = el.offsetTop, bot = last.offsetTop + last.offsetHeight;
  if (top < box.scrollTop + 4 || bot > box.scrollTop + box.clientHeight - 4) box.scrollTop = Math.max(0, top - Math.round(box.clientHeight * 0.3));
  S.yscroll[key] = box.scrollTop;
}

function focusField(t) {
  const f = (t.closest('[data-f]') || {}).dataset ? t.closest('[data-f]').dataset.f : '';
  if (['id', 'text', 'note', 'title', 'revision'].includes(f)) return f;
  if (['heading', 'level'].includes(f)) { const se = t.closest('.sec'); return se ? `sec.${se.dataset.si}` : ''; }
  if (f === 'intro') { const se = t.closest('.sec'); return se ? `sec.${se.dataset.si}.intro` : ''; }
  return '';
}

// ---- marking a drawn panel ------------------------------------------
// diffCount is how many places the draft changed: a value written, a rule that
// left the section it stood in, a section whose own keys changed — and one for
// an order that only moved, whether one rule or one section (spec-roles I4, I5:
// 並びだけが替わったら 1).
function diffCount(e) {
  const s = e.saved, d = e.draft;
  if (!s) return 1;
  // A rule counts when its values changed, or when it left the section it stood
  // in. Section identity travels with a section (_s), so rules that travel with
  // their section are not counted one by one.
  const secOfSaved = new Map(), secOfDraft = new Map();
  s.sections.forEach((sec) => sec.rules.forEach((x) => secOfSaved.set(x._k, sec._s)));
  d.sections.forEach((sec) => sec.rules.forEach((x) => secOfDraft.set(x._k, sec._s)));
  let n = ['name', 'title', 'revision'].filter((f) => s[f] !== d[f]).length;
  const sr = new Map(rulesOf(s).map((x) => [x._k, JSON.stringify(x)])), dr = rulesOf(d);
  dr.forEach((x) => {
    if (sr.get(x._k) !== JSON.stringify(x) || secOfSaved.get(x._k) !== secOfDraft.get(x._k)) n++;
  });
  sr.forEach((v, k) => { if (!dr.some((x) => x._k === k)) n++; });
  // A section counts when its own keys changed or when it is new; a section that
  // was taken out counts once, below.
  const bySec = new Map(s.sections.map((sec) => [sec._s, sec]));
  const order = [];
  d.sections.forEach((sec) => {
    const a = bySec.get(sec._s);
    if (!a) { n++; return; }
    if (['heading', 'level', 'intro'].some((f) => a[f] !== sec[f])) n++;
    order.push(sec._s);
  });
  s.sections.forEach((sec) => { if (!d.sections.some((x) => x._s === sec._s)) n++; });
  const savedOrder = s.sections.map((sec) => sec._s).filter((k) => order.includes(k));
  if (order.join() !== savedOrder.join()) n++;
  // The order of the rules within the document, when nothing else changed.
  if (!n && sr.size && rulesOf(s).map((x) => x._k).join() !== dr.map((x) => x._k).join()) n++;
  return n;
}

function mark(panel, issues) {
  const row = rowOf(panel);
  if (!row) return;
  const all = issues || row.issues || [];
  paint($('.pedit', panel), html`<${Edit} row=${row} issues=${all} />`);
  paint($('.yprev', panel), html`<${Preview} row=${row} issues=${all} />`);
  if (!row.newRow) paint($('.pf .vcount', panel), html`<${Counts} list=${all} />`);
  markFoot(panel, row, all);
}

// markFoot says where the document stands, in the words the screen's own foot
// holds: the places it changed, whether 保存 can be pressed, and the count of
// what is wrong. The team screen's shape tab draws that row's foot while a
// definition is not there yet, and this part leaves it alone (監査 R9).
function markFoot(panel, row, all) {
  const foot = $('.pf', panel);
  if (!foot || row.newRow) return;
  const e = row.e, s = e && e.saved, n = e ? diffCount(e) : 0;
  const dirty = $('.dirty', foot);
  if (dirty) dirty.textContent = s && n ? `${n} か所` : '';
  const save = $('[data-act=save]', foot);
  if (save) {
    const before = new Set(s && row.savedIssues ? row.savedIssues.filter((m) => m.sev === 'err').map((m) => m.msg) : all.filter((m) => m.sev === 'err').map((m) => m.msg));
    save.disabled = !s ? all.some((m) => m.sev === 'err') : (all.some((m) => m.sev === 'err' && !before.has(m.msg)) || n === 0);
  }
}

// refreshPrev draws the preview alone — the part's own way in, kept for the
// scripts that call it (the caret moving to a line, the scroll of the box).
function refreshPrev(panel, issues) {
  const row = rowOf(panel);
  if (!row) return;
  paint($('.yprev', panel), html`<${Preview} row=${row} issues=${issues || row.issues || []} />`);
}

// ---- reordering ------------------------------------------------------
function moveRule(d, si, ri, dir) {
  const s = d.sections[si], removed = s.rules.splice(ri, 1), x = removed[0];
  if (ri + dir >= 0 && ri + dir <= s.rules.length) s.rules.splice(ri + dir, 0, x);
  else if (dir < 0 && si > 0) d.sections[si - 1].rules.push(x);
  else if (dir > 0 && si < d.sections.length - 1) d.sections[si + 1].rules.unshift(x);
  else s.rules.splice(ri, 0, x);
}

let drag = null;
const clearDrop = () => $$('.drop-b, .drop-a, .drop-end').forEach((el) => el.classList.remove('drop-b', 'drop-a', 'drop-end'));

// ---- the operations --------------------------------------------------
// The drag classes are put on the elements themselves while the pointer is over
// them: the panel is not drawn again until the drop, so nothing takes them away.
// The parts a press reads are the DOM's and the drafts', as they were.
function ctx(el) {
  const panel = el.closest('.panel');
  if (!panel) return {};
  const row = rowOf(panel);
  if (!row) return {};
  const d = row.draft;
  const ruleEl = el.closest('.rule'), f = ruleEl ? findRule(d, ruleEl.dataset.k) : null;
  const secEl = el.closest('.sec'), si = secEl ? +secEl.dataset.si : null;
  const oi = el.closest('.orow') ? +el.closest('.orow').dataset.oi : null;
  return { panel, row, d, x: f && f.x, si: f ? f.si : si, ri: f && f.ri, oi, key: `${row.uid}${row.l}-${f ? f.x._k : ''}` };
}

function rerender(panel, focusSel) {
  const row = rowOf(panel);
  if (!row) return panel;
  const box = $('.ybox', panel);
  if (box && box.clientHeight) S.yscroll[row.uid + row.l] = box.scrollTop;
  return host.rerender(panel, focusSel);
}

function re(c, sel) { return rerender(c.panel, sel); }

// goRule opens one rule of a document and puts the cursor in its body (spec-roles
// 1.2.4, 1.2.7, P15, 監査 R12): a screen that jumps to a rule from a check, or
// from the address, reads through this, so the rule stands open where the editor
// shows, brought into the window — never under the save's band — with the body
// ready to be typed in. A rule the document does not hold leaves the panel as it
// is.
function goRule(panel, k) {
  const row = rowOf(panel);
  if (!row || !k) return null;
  S.ropen.add(rkey(row, { _k: k }));
  if (host.mode) host.mode(panel, 'edit');
  S.pfocus[row.uid + row.l] = { k, f: 'text' };
  const fresh = rerender(panel, null);
  const el = fresh && fresh.querySelector(`.rule[data-k="${k}"] [data-f=text]`);
  if (el) {
    // The rule is brought into the middle of the window, so the band the screen
    // pins to the bottom never hides it; a rule taller than the window keeps its
    // head in it.
    el.scrollIntoView({ block: 'center' });
    const top = el.getBoundingClientRect().top;
    if (top < 0) window.scrollBy(0, top);
    el.focus({ preventScroll: true });
  }
  if (host.url) host.url();
  return fresh;
}

function handleClick(b) {
  const act = b.dataset.act, c = ctx(b);
  const row = c.row;
  if (!row) return false;
  const pkey = row.uid + row.l, ck = c.x ? 'r' + c.key : null;
  const sk = c.si != null ? `${row.uid}${row.l}s${c.si}` : null;
  switch (act) {
    case 'rtoggle': {
      const k = rkey(row, c.x);
      if (S.ropen.has(k)) S.ropen.delete(k); else S.ropen.add(k);
      re(c, `.rule[data-k="${c.x._k}"] .rsum`); if (host.url) host.url(); break;
    }
    case 'sec-pick': S.secPick[sk] = S.secPick[sk] === b.dataset.w ? null : b.dataset.w; re(c, `.sec[data-si="${c.si}"] [data-act=sec-pick][data-w=${b.dataset.w}]`); break;
    case 'sec-level': {
      const s0 = c.d.sections[c.si], v = +b.dataset.v;
      // Choosing ## for a section the file wrote no level for leaves it unwritten
      // (0), so the save adds no level: line (A2).
      s0.level = v === 2 && !s0.level ? 0 : v;
      S.secPick[sk] = null;
      if (s0.rules[0]) S.pfocus[pkey] = { k: s0.rules[0]._k, f: 'text' };
      re(c, `.sec[data-si="${c.si}"] [data-act=sec-pick][data-w=lvl]`); edited(c); break;
    }
    case 'sec-add': c.d.sections.push({ _s: null, heading: '', level: 0, intro: '', rules: [] }); re(c, `.sec[data-si="${c.d.sections.length - 1}"] [data-f=heading]`); edited(c); break;
    case 'sec-del': S.confirm[sk] = 1; re(c, `.sec[data-si="${c.si}"] [data-act=sec-del-yes]`); break;
    case 'sec-del-no': delete S.confirm[sk]; re(c); break;
    case 'sec-del-yes': delete S.confirm[sk]; c.d.sections.splice(c.si, 1); re(c); edited(c); break;
    case 'sec-up': case 'sec-down': {
      const j = c.si + (act === 'sec-up' ? -1 : 1), ss = c.d.sections;
      if (j < 0 || j >= ss.length) break;
      const at = ss[c.si]; ss[c.si] = ss[j]; ss[j] = at;
      S.secPick = {};
      Object.keys(S.confirm).filter((k2) => k2.startsWith(`${row.uid}${row.l}s`)).forEach((k2) => delete S.confirm[k2]);
      re(c, `.sec[data-si="${j}"] [data-act=${act}]`); edited(c); break;
    }
    case 'intro-add': S.introOpen = secKey(row, c.si); re(c, `.sec[data-si="${c.si}"] [data-f=intro]`); break;
    case 'rule-add': {
      const x = { _k: ++kseq, id: nextID(row), text: '', note: '', in_prompt: true, off_since: '' };
      c.d.sections[c.si].rules.push(x); S.ropen.add(rkey(row, x));
      re(c, `.rule[data-k="${x._k}"] [data-f=text]`); edited(c); break;
    }
    case 'rule-up': case 'rule-down': moveRule(c.d, c.si, c.ri, act === 'rule-up' ? -1 : 1); re(c, `.rule[data-k="${c.x._k}"] [data-act=${act}]`); edited(c); break;
    case 'rule-revert': { const sx = clone(savedRule(row, c.x._k)); if (sx) { sx._k = c.x._k; c.d.sections[c.si].rules[c.ri] = sx; } re(c, `.rule[data-k="${c.x._k}"] .rsum`); edited(c); break; }
    case 'rule-del': S.confirm[ck] = 1; re(c, `.rule[data-k="${c.x._k}"] [data-act=rule-del-yes]`); break;
    case 'rule-del-no': delete S.confirm[ck]; re(c); break;
    case 'rule-del-yes': delete S.confirm[ck]; c.d.sections[c.si].rules.splice(c.ri, 1); re(c); edited(c); break;
    case 'rule-toggle': {
      c.x.in_prompt = !inP(c.x);
      if (inP(c.x)) c.x.off_since = '';
      re(c, `.rule[data-k="${c.x._k}"] .onsw`);
      edited(c); break;
    }
    case 'yline': {
      const k = +b.dataset.k, si = b.dataset.si, part = b.dataset.part;
      if (k) S.ropen.add(`${row.uid}${row.l}:${k}`);
      if (host.mode) host.mode(c.panel, 'edit');
      S.pfocus[pkey] = k ? { k, f: 'text' } : part === 'title' ? { k: 0, f: 'title' } : part === 'intro' ? { k: 0, f: `sec.${si}.intro` } : { k: 0, f: `sec.${si}` };
      // The Markdown box follows to the section's first rule: the panel is drawn
      // again first, then the fresh box is scrolled (A2, P15).
      const fresh = host.rerender(c.panel, null);
      const target = k ? `.rule[data-k="${k}"] [data-f=text]` : part === 'title' ? '[data-f=title]' : `.sec[data-si="${si}"] [data-f=${part === 'intro' ? 'intro' : 'heading'}]`;
      if (fresh) { const el = fresh.querySelector(target); if (el) { el.scrollIntoView({ block: 'nearest' }); el.focus({ preventScroll: true }); } }
      if (host.url) host.url(); break;
    }
    default: return false;
  }
  return true;
}

function handleInput(t) {
  const c = ctx(t);
  if (!c.panel || !c.d || !t.dataset.f) return false;
  const v = t.value;
  switch (t.dataset.f) {
    case 'name': c.d.name = v; { const p = c.panel.querySelector('.path'); if (p) p.textContent = host.path ? host.path(c.row.l, c.d) : p.textContent; } break;
    case 'title': c.d.title = v; break;
    case 'revision': c.d.revision = v === '' ? null : +v; break;
    case 'heading': c.d.sections[c.si].heading = v; break;
    case 'intro': c.d.sections[c.si].intro = v; break;
    case 'id': case 'text': case 'note': c.x[t.dataset.f] = v; break;
    default: return false;
  }
  edited(c);
  return true;
}

function edited(c) { if (host.edited) host.edited(c); }

function attach(r, h) {
  root = r;
  host = h || {};
  root.addEventListener('click', (e) => {
    const b = e.target.closest('[data-act]');
    if (b && handleClick(b)) e.stopPropagation();
  });
  root.addEventListener('input', (e) => { if (handleInput(e.target)) { /* handled */ } });
  root.addEventListener('keydown', (e) => {
    if ((e.key === 'Enter' || e.key === ' ') && e.target.matches('.rsum')) { e.preventDefault(); e.target.click(); }
  });
  root.addEventListener('focusin', (e) => {
    const t = e.target;
    if (!t.closest('.pedit')) return;
    const c = ctx(t);
    if (!c.panel) return;
    const k = c.row.uid + c.row.l, fc = { k: c.x ? c.x._k : 0, f: focusField(t) };
    const was = S.pfocus[k];
    if (was && was.k === fc.k && was.f === fc.f) return;
    S.pfocus[k] = fc;
    // The preview marks the line the caret is in: the part says its own state
    // changed, and the preview is drawn from it (S).
    touched();
  });
  root.addEventListener('scroll', (e) => {
    if (e.target.matches?.('.ybox')) { const p = e.target.closest('.panel'); if (p) S.yscroll[p.dataset.uid + p.dataset.l] = e.target.scrollTop; }
  }, true);
  // A rule is dragged by its ⋮ handle: above or below another rule's row, or to
  // the end of a section (A5). The drop line is 2px of the accent, and the row
  // that is dragged goes faint.
  root.addEventListener('pointerdown', (e) => { const g = e.target.closest && e.target.closest('.grip'); if (g) g.closest('.rule').draggable = true; });
  root.addEventListener('dragstart', (e) => {
    const el = e.target.closest && e.target.closest('.rule[draggable=true]');
    if (!el) return;
    drag = { panel: el.closest('.panel'), k: +el.dataset.k, to: null };
    e.dataTransfer.effectAllowed = 'move';
    try { e.dataTransfer.setData('text/plain', el.dataset.k); } catch (err) { /* the move is inside this screen */ }
    el.classList.add('dragging');
  });
  root.addEventListener('dragover', (e) => {
    if (!drag) return;
    const sec = e.target.closest('.sec');
    if (!sec || sec.closest('.panel') !== drag.panel) return;
    e.preventDefault(); clearDrop();
    const el = e.target.closest('.rule');
    if (el) {
      const rc = el.getBoundingClientRect(), before = e.clientY < rc.top + rc.height / 2;
      el.classList.add(before ? 'drop-b' : 'drop-a');
      drag.to = { si: +sec.dataset.si, k: +el.dataset.k, before };
    } else {
      sec.classList.add('drop-end');
      drag.to = { si: +sec.dataset.si, k: null };
    }
  });
  root.addEventListener('drop', (e) => {
    if (!drag || !drag.to) return;
    e.preventDefault();
    const panel = drag.panel, row = rowOf(panel), d = row.draft, to = drag.to;
    const from = findRule(d, drag.k);
    if (from && to.k !== drag.k) {
      const [x] = d.sections[from.si].rules.splice(from.ri, 1), rs = d.sections[to.si].rules;
      rs.splice(to.k == null ? rs.length : rs.findIndex((y) => y._k === to.k) + (to.before ? 0 : 1), 0, x);
    }
    const k = drag.k;
    drag = null; clearDrop();
    const fresh = rerender(panel);
    const moved = fresh && fresh.querySelector(`.rule[data-k="${k}"]`);
    if (moved) moved.classList.add('flash');
    if (host.edited) host.edited({ panel, row, d });
  });
  root.addEventListener('dragend', () => { $$('.rule.dragging, .rule[draggable=true]').forEach((el) => { el.classList.remove('dragging'); el.draggable = false; }); clearDrop(); drag = null; });
}

// RuleDoc is the part as one component, for a screen that is a module: the two
// containers a panel's .pmain holds, and the trees inside them.
export function RuleDoc({ row, issues, nameError }) {
  return html`<div class="pedit"><${Edit} row=${row} issues=${issues} nameError=${nameError} /></div><aside class="yprev" aria-label="Markdown"><${Preview} row=${row} issues=${issues} /></aside>`;
}

// nextKey hands out the key of a rule a person adds, and observeKeys takes the
// keys a draft that came from elsewhere already holds: together they keep the
// keys of one document together across a save, a carried draft and a screen's
// own newrow (the address names an open rule by its key).
const nextKey = () => ++kseq;
const observeKeys = (rules) => { rules.forEach((x) => { if (x._k > kseq) kseq = x._k; }); };

// ---- the modules' way in (4 章) ----
// A screen that is a module imports the part rather than reading the bridge.
export {
  attach as mount, Counts, Badge, Vm, Sev, OutNote,
  S as state, rkey, rulesOf, findRule, fromJSON, toDoc, diffCount, inP,
  outNote, outCount, lvlMark, nextKey, observeKeys, esc, clone, num, goRule,
};

// body is the skeleton a panel's .pmain starts from: the two elements the part
// draws into, and nothing else — the part puts their insides in on the first
// mark, so no markup a person could type in is ever written here twice
// (webui-preact-design 5 章 1).
function body() {
  return '<div class="pedit"></div><aside class="yprev" aria-label="Markdown"></aside>';
}

// ---- the classic screens' way in (4 章, until the last of them is moved) ----
window.hergyRuleDoc = {
  mount: attach,
  body,
  mark,
  refreshPrev,
  rerender,
  goRule,
  state: S,
  rkey,
  rulesOf,
  findRule,
  fromJSON,
  toDoc,
  diffCount,
  inP,
  outNote,
  outCount,
  lvlMark,
  // The few words a screen draws too: the shape of a problem, the count of a
  // list of them, and the chip a row carries.
  sevChip, vmItem, counts, badge, esc, clone, num, arrowsOf,
  nextKey,
  observeKeys,
};
