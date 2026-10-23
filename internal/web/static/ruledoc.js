// The part a screen uses to edit one rule document and to show the Markdown a
// member reads (spec-teams 7). The role screen mounts it today and the team
// screen mounts the same part tomorrow: everything inside a panel that edits a
// document — the title and revision, the sections, the rules, the
// 含める / 含めない switch of a rule and of its reason, the heading depth, the
// reordering (↑ ↓ and the ⋮ drag), and the Markdown box — lives here. What a
// screen keeps for itself is the panel's head and foot (where the document
// stands, 複製, 削除) and where the checks and writes go.
//
// A screen mounts once, on the element its panels stand in:
//
//   hergyRuleDoc.mount(root, {
//     row(panel) -> { uid, l, saved, draft, issues, markdown, diff },
//     rerender(panel, focusSel) -> the fresh element,
//     edited(panel),        // a value changed: mark the panel and ask for a check
//     readerCount(name),    // how many definitions read a role name
//     filtering(),          // whether the screen hides some rules
//     shown(rule, issues),  // whether one rule is shown while filtering
//     query(),              // the screen's search text, for the marks
//   });
//
// The part answers with the panel body (body), the marking of a drawn panel
// (mark), the state a screen reads (state), and the few words a screen draws
// too (badge, counts, vmItem, sevChip). No write of its own: a save stays the
// screen's, so the part never knows where a file goes.
(() => {
  'use strict';

  const PLACES = ['project', 'global'];
  const KIND = { incident: 'インシデント', instruction: '指示', derived: '導出' };
  const LIMIT = { rules: 20, chars: 6000 };
  const clone = (o) => JSON.parse(JSON.stringify(o));
  const esc = (s) => String(s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
  const reEsc = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const num = (n) => Number(n || 0).toLocaleString('ja-JP');
  const $ = (sel, root = document) => root.querySelector(sel);
  const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));
  const other = (l) => (l === 'project' ? 'global' : 'project');
  let kseq = 0;

  // ---- the document as the screen holds it -----------------------------
  // fromJSON reads one document off the page's island into the draft a panel
  // edits. Each rule keeps the key it was read with (_k) and each section the
  // place it stood at (_s), so a rule or a section that is moved, or a value
  // that is written back, is found on the file it came from (spec-roles 1.2.9,
  // spec-teams C7).
  function fromJSON(d) {
    return {
      name: d.name || '', title: d.title || '',
      revision: d.revision === null || d.revision === undefined ? null : d.revision,
      sections: (d.sections || []).map((s, si) => ({
        _s: si, heading: s.heading || '', level: s.level || 0, intro: s.intro || '',
        rules: (s.rules || []).map((x) => ({
          _k: ++kseq, id: x.id || '', text: x.text || '', why: x.why || '',
          send_why: x.send_why !== false, in_prompt: x.in_prompt !== false,
          note: x.note || '', report: x.report || '', tests: (x.tests || []).slice(),
          origin: (x.origin || []).map((o) => ({ kind: o.kind || 'incident', date: o.date || '', refs: (o.refs || []).slice(), note: o.note || '' })),
        })),
      })),
    };
  }

  // toDoc is the draft as the server reads it. Each rule and each section
  // carries where it stood in the saved document (its from), so the server can
  // write the changed lines and carry a moved rule or section whole.
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
            from: f ? { section: f[0], rule: f[1] } : null, id: x.id, text: x.text, why: x.why,
            send_why: x.send_why, in_prompt: x.in_prompt,
            note: x.note, report: x.report, tests: x.tests, origin: x.origin,
          };
        }),
      })),
    };
  }

  const rulesOf = (d) => d.sections.flatMap((s) => s.rules);
  // nextID is the ID a rule the person just added carries (n-ec32). Its prefix
  // is the one the document's own IDs use most — LEAD for a lead.yaml — or, in a
  // document that has no such ID yet, the document's name in capital letters
  // with everything but A-Z and 0-9 taken out (ui-impl -> UIIMPL; a name that is
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
  const savedRule = (e, k) => (e.saved ? (findRule(e.saved, k) || { x: null }).x : null);
  const ruleChanged = (e, x) => JSON.stringify(savedRule(e, x._k) || null) !== JSON.stringify(x);
  const secKey = (row, si) => `${row.uid}${row.l}-s${si}`;
  const rkey = (row, x) => `${row.uid}${row.l}:${x._k}`;
  const inP = (x) => x.in_prompt !== false;
  const outCount = (rules) => rules.filter((x) => !inP(x)).length;
  const outNote = (n) => (n ? `<span class="outn">含めない ${n}</span>` : '');
  // The depth of a heading is shown the way it stands in the prompt: level 0
  // and 2 are ##, 3 is ###, 4 is ####. There is no level — any more (A2, I6).
  const lvlMark = (lv) => '#'.repeat(lv && lv > 2 ? lv : 2);
  const promptSeg = (f, act, v) => `<div class="seg" data-f="${f}">${[[true, 'プロンプトに含める'], [false, '含めない']].map(([b, label]) => `<button class="${v === b ? 'on' : ''}" data-act="${act}" data-v="${b}" aria-pressed="${v === b}">${label}</button>`).join('')}</div>`;

  // ---- state the part owns --------------------------------------------
  const S = { ropen: new Set(), introOpen: null, secPick: {}, confirm: {}, pfocus: {}, yscroll: {} };

  let host = {};
  const rowOf = (panel) => (panel ? host.row(panel) : null);

  // ---- the shape of a problem, as the mock draws it -------------------
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

  // ---- decoration ------------------------------------------------------
  const ARROW_RE = /([A-Za-z][\w-]*|人間)\s*(→|⇄)\s*([A-Za-z][\w-]*|人間)/g;
  const MODEL_RE = /\b(claude|codex|opencode|gemini|opus|sonnet|haiku|fable|gpt-[\w.]+|deepseek[\w.-]*|qwen[\w.-]*)\b/gi;
  const arrowsOf = (text) => [...String(text).matchAll(ARROW_RE)].map((m) => ({ a: m[1], op: m[2], b: m[3], s: m[0] }));

  function deco(text) {
    const q = host.query ? host.query() : '';
    const marks = q ? [new RegExp(reEsc(q), 'gi')] : [];
    return esc(text)
      .replace(ARROW_RE, (m) => `<span class="arrow">${m}</span>`)
      .replace(/(<[^>]+>)|([^<]+)/g, (m, tag, txt) => tag || txt.replace(MODEL_RE, (w) => `<span class="mw">${w}</span>`))
      .replace(/(<[^>]+>)|([^<]+)/g, (m, tag, txt) => tag || marks.reduce((s, re) => s.replace(re, (w) => `<mark>${w}</mark>`), txt));
  }
  function firstPhrase(t) { const i = t.indexOf('。'); return i >= 0 && i < t.length - 1 ? t.slice(0, i + 1) : t; }
  function snippet(t) {
    const f = firstPhrase(t), q = (host.query ? host.query() : '').toLowerCase();
    if (!q || f.toLowerCase().includes(q) || !t.toLowerCase().includes(q)) return f;
    const hit = t.split(/(?<=。)/).find((x) => x.toLowerCase().includes(q));
    return hit ? `${f} … ${hit}` : f;
  }

  // ---- the panel body --------------------------------------------------
  function body(row) {
    return `${'<div class="pedit">'}${editHTML(row)}</div><aside class="yprev" aria-label="Markdown"><div class="yph"><span class="pkl">Markdown</span><span class="yinfo"></span></div><div class="ybox"></div></aside>`;
  }

  function editHTML(row) {
    const d = row.draft, key = row.uid + row.l;
    return `<div class="dh">
      ${host.hasName === false ? '' : `<div class="dhf"><label for="n${key}">名前</label><input id="n${key}" class="in-name" data-f="name" value="${esc(d.name)}" spellcheck="false" autocomplete="off"><span class="errline name" hidden></span></div>`}
      <div class="dhf"><label for="t${key}">タイトル</label><input id="t${key}" class="in-title" data-f="title" value="${esc(d.title)}" autocomplete="off"></div>
      <div class="dhf"><label for="v${key}">revision</label><input id="v${key}" class="in-rev" data-f="revision" type="number" min="1" value="${d.revision === null ? '' : d.revision}"></div>
      <label class="rename" hidden><input type="checkbox" id="rn${key}" checked><span></span></label>
    </div>
    <ul class="vm dhvm"></ul>
    ${d.sections.map((sec, si) => secHTML(row, si)).join('')}
    <button class="add secadd" data-act="sec-add">＋ 節</button>`;
  }

  function sechHTML(row, sec, si) {
    const key = secKey(row, si), ck = `${row.uid}${row.l}s${si}`;
    const pick = S.secPick[ck];
    const opts = pick === 'lvl' ? [2, 3, 4].map((v) => `<button class="lvl ${lvlMark(sec.level) === lvlMark(v) ? 'on' : ''}" data-act="sec-level" data-v="${v}" aria-pressed="${lvlMark(sec.level) === lvlMark(v)}">${lvlMark(v)} ${esc(sec.heading || '見出し')}</button>`).join('') : '';
    const extraEnd = `${sec.intro === '' && !S.confirm[ck] ? `<button class="add sm" data-act="intro-add">＋ 前置き</button>` : ''}
        ${S.confirm[ck] ? `<button class="btn danger solid sm" data-act="sec-del-yes">節を削除する（${sec.rules.length} 規則）</button><button class="btn sm" data-act="sec-del-no">やめる</button>`
        : `<button class="x" data-act="sec-del" aria-label="節 ${esc(sec.heading)} を消す">×</button>`}`;
    return `<div class="sech">
      <button class="lvl" data-f="level" data-act="sec-pick" data-w="lvl" aria-expanded="${pick === 'lvl'}" aria-label="見出しの深さ">${lvlMark(sec.level)}</button>
      <input id="h${key}" class="in-head" data-f="heading" value="${esc(sec.heading)}" placeholder="見出し" autocomplete="off">
      <span class="n">${sec.rules.length} 規則${outNote(outCount(sec.rules))}</span>
      <span class="end"><button class="x" data-act="sec-up" ${si === 0 ? 'disabled' : ''} aria-label="節を上へ">↑</button><button class="x" data-act="sec-down" ${si === row.draft.sections.length - 1 ? 'disabled' : ''} aria-label="節を下へ">↓</button>${extraEnd}</span>
    </div>
    ${opts ? `<div class="pickrow">${opts}</div>` : ''}`;
  }

  function secHTML(row, si) {
    const d = row.draft, sec = d.sections[si], key = secKey(row, si), ck = `${row.uid}${row.l}s${si}`;
    const filtering = host.filtering ? host.filtering() : false;
    const rules = sec.rules.filter((x) => !filtering || host.shown(x, row.issues));
    if (filtering && !rules.length) return '';
    return `<section class="sec" data-si="${si}">
      ${sechHTML(row, sec, si)}
      ${sec.intro !== '' || S.introOpen === key ? `<textarea id="in${key}" class="intro" data-f="intro" rows="1" placeholder="前置き">${esc(sec.intro)}</textarea>` : ''}
      <div class="rules">${rules.map((x) => ruleHTML(row, x)).join('')}</div>
      <button class="add secadd" data-act="rule-add">＋ 規則</button>
    </section>`;
  }

  function rsumInner(row, x, mine) {
    const o = x.origin[0];
    const org = o
      ? `<span class="org"><i class="sw o-${o.kind}"></i><span class="lbl">${KIND[o.kind] || esc(o.kind)}</span>${o.kind === 'derived' ? `<span class="dt">${esc(o.refs.join(' '))}</span>` : `<span class="dt">${esc(String(o.date).slice(5))}</span>`}${x.origin.length > 1 ? `<span class="more">+${x.origin.length - 1}</span>` : ''}</span>`
      : `<span class="org empty">—</span>`;
    return `<span class="grip" aria-hidden="true">⋮</span>
      <span class="rid"><b>${deco(x.id) || '<span class="none">—</span>'}</b>${badge(mine)}</span>
      <span class="rtx">${deco(snippet(x.text))}</span>
      <span class="marks">${org}<span>${x.why ? `<span class="mk why ${x.send_why ? '' : 'held'}">なぜ</span>` : ''}</span><span>${x.tests.length || x.report ? '<span class="mk tst">確かめ</span>' : ''}</span><span>${inP(x) ? '' : '<span class="mk out">含めない</span>'}</span></span>`;
  }

  function ruleHTML(row, x) {
    const open = S.ropen.has(rkey(row, x));
    return `<div class="rule ${open ? 'open' : ''} ${ruleChanged(row.e, x) ? 'unsaved' : ''} ${inP(x) ? '' : 'out'}" data-k="${x._k}">
      <div class="rsum" role="button" tabindex="0" aria-expanded="${open}" data-act="rtoggle">${rsumInner(row, x, row.issues.filter((m) => m.k === x._k))}</div>
      ${open ? ruleEditHTML(row, x) : ''}
    </div>`;
  }

  function ruleEditHTML(row, x) {
    const key = `${row.uid}${row.l}-${x._k}`, ck = 'r' + key;
    return `<div class="redit">
      <div class="field scope"><span class="fl">規則</span><div>${promptSeg('in_prompt', 'in-prompt', inP(x))}</div></div>
      <div class="field"><span class="fl">ID</span><div><span class="ridv" aria-label="ID">${esc(x.id) || '<span class="none">—</span>'}</span><ul class="vm" data-for="id"></ul></div></div>
      <div class="field"><label for="tx${key}">本文</label><div><textarea id="tx${key}" class="ta" data-f="text" rows="2">${esc(x.text)}</textarea><ul class="vm" data-for="text"></ul></div></div>
      <div class="field"><label for="wy${key}">なぜ</label><div><textarea id="wy${key}" class="ta ${x.why && !x.send_why ? 'held-why' : ''}" data-f="why" rows="1">${esc(x.why)}</textarea>
        <div class="whyrow">${promptSeg('send_why', 'send-why', x.send_why)}</div></div></div>
      <div class="field"><span class="fl">由来</span><div><div class="origins">${x.origin.map((o, oi) => `<div class="orow" data-oi="${oi}">
          <div class="seg" data-f="o-kind">${Object.entries(KIND).map(([k, label]) => `<button class="${o.kind === k ? 'on' : ''}" data-act="o-kind" data-v="${k}" aria-pressed="${o.kind === k}"><i class="sw o-${k}"></i>${label}</button>`).join('')}</div>
          <input id="od${key}-${oi}" data-f="o-date" value="${esc(o.date)}" placeholder="YYYY-MM-DD" spellcheck="false" autocomplete="off" aria-label="日付">
          <div class="chips">${o.refs.map((ref, j) => `<span class="chip"><input id="rf${key}-${oi}-${j}" data-f="ref" data-j="${j}" value="${esc(ref)}" size="${Math.max(String(ref).length, 5)}" spellcheck="false" autocomplete="off" aria-label="参照"><button class="x" data-act="ref-del" data-j="${j}" aria-label="${esc(ref)} を消す">×</button></span>`).join('')}<button class="add sm" data-act="ref-add" aria-label="参照を足す">＋</button></div>
          <input id="on${key}-${oi}" data-f="o-note" value="${esc(o.note)}" placeholder="メモ" autocomplete="off">
          <button class="x" data-act="o-del" aria-label="由来を消す">×</button>
        </div>`).join('')}<button class="add sm" data-act="o-add" aria-label="由来を足す">＋</button></div><ul class="vm" data-for="origin"></ul></div></div>
      <div class="field"><span class="fl">確かめ</span><div class="ckrow">
        <span class="chips">${x.tests.map((tst, j) => `<span class="chip"><input id="ts${key}-${j}" data-f="test" data-j="${j}" value="${esc(tst)}" size="${Math.max(String(tst).length, 6)}" spellcheck="false" autocomplete="off" aria-label="テスト"><button class="x" data-act="test-del" data-j="${j}" aria-label="${esc(tst)} を消す">×</button></span>`).join('')}<button class="add sm" data-act="test-add" aria-label="テストを足す">＋ テスト</button></span>
        <span class="lb">報告</span><input id="rp${key}" class="in-report" data-f="report" value="${esc(x.report)}" placeholder="report.field" spellcheck="false" autocomplete="off">
      </div></div>
      <div class="field"><label for="nt${key}">メモ</label><div><input id="nt${key}" class="in-note" data-f="note" value="${esc(x.note)}" autocomplete="off"></div></div>
      <footer class="rf">
        <button class="btn sm" data-act="rule-up" aria-label="上へ">↑</button><button class="btn sm" data-act="rule-down" aria-label="下へ">↓</button>
        <span class="acts">
          <button class="btn sm" data-act="rule-revert" hidden>元に戻す</button>
          ${S.confirm[ck] ? `<button class="btn danger solid sm" data-act="rule-del-yes">規則を削除する</button><button class="btn sm" data-act="rule-del-no">やめる</button>`
          : `<button class="btn danger sm" data-act="rule-del">削除</button>`}
        </span>
      </footer>
    </div>`;
  }

  // ---- the Markdown preview -------------------------------------------
  function mdHL(t, held) {
    const heldHTML = held ? `<span class="held" aria-hidden="true"><span class="mk why held">なぜ</span><s>${esc(held)}</s></span>` : '';
    const body = (x) => esc(x).replace(MODEL_RE, (w) => `<span class="mw">${w}</span>`);
    let m = t.match(/^(#+) (.*)$/);
    if (m) return { h: 0, html: `<span class="mm">${m[1]} </span><span class="mh h${m[1].length}">${esc(m[2])}</span>${heldHTML}` };
    m = t.match(/^- \*\*(.*?)\*\* (.*)$/);
    if (m) return { h: 2, html: `<span class="mm">- **</span><b>${esc(m[1])}</b><span class="mm">**</span> ${body(m[2])}${heldHTML}` };
    m = t.match(/^(- |\d+\. |   )(.*)$/);
    if (m) return { h: m[1].length, html: `<span class="mm">${m[1]}</span>${body(m[2])}${heldHTML}` };
    return { h: 0, html: body(t) + heldHTML };
  }

  function refreshPrev(panel, issues) {
    const row = rowOf(panel);
    if (!row) return;
    const e = row.e, all = issues || row.issues;
    const aside = panel.querySelector('.yprev');
    if (!aside) return;
    const md = row.markdown, lines = md.lines || [], df = row.diff || { added: [], removedBefore: [] };
    let ri = -1;
    const fc0 = S.pfocus[row.uid + row.l];
    const fc = fc0 && { k: fc0.k, f: ['id', 'text', 'why'].includes(fc0.f) ? 'text' : fc0.k ? '' : fc0.f };
    const added = new Set(df.added || []), removedBefore = new Set(df.removedBefore || []);
    const box = aside.querySelector('.ybox');
    box.innerHTML = lines.map((z) => {
      const k = z.section >= 0 ? ruleKeyAt(e.draft, z.section, z.rule) : 0;
      if (z.part === 'excluded') {
        return `<div class="yl ghost go ${fc && fc.k && k === fc.k ? 'cur' : ''}" data-act="yline" data-k="${k}" data-part="rule"><span class="lno"></span><span class="yt" data-h="0"><span class="mk out">含めない</span><s>${esc(z.text || '')}</s></span></div>`;
      }
      ri++;
      // A problem about the document as a whole (no rule of its own) is marked
      // on the first line alone; a problem about a rule is marked on the lines
      // that rule stands on (spec-roles 1.2.6). A heading or a blank line has no
      // rule of its own, so it is not one of a rule's lines.
      const ms = all.filter((m) => (k ? m.k === k : m.k === 0 && ri === 0));
      const sev = ms.some((m) => m.sev === 'err') ? 'err' : ms.length ? 'warn' : '';
      const { h, html } = mdHL(z.text || '', z.held || '');
      const go = k || z.section >= 0 || z.part === 'title';
      const here = fc && fc.k && k === fc.k;
      const around = here && fc.f === 'text';
      const partNear = fc && !fc.k && (z.part === 'title' || z.part === 'heading' || z.part === 'intro')
        && ((z.part === 'title' && fc.f === 'title') || (z.part !== 'title' && fc.f.startsWith('sec.')));
      const cls = [sev, added.has(ri) && 'ins', removedBefore.has(ri) && 'del', here && 'cur', (around || partNear) && 'curf', go && 'go'].filter(Boolean).join(' ');
      return `<div class="yl ${cls}"${go ? ` data-act="yline" data-k="${k}" data-si="${z.section}" data-part="${z.part}"` : ''}${ms.length ? ` title="${esc(ms.map((m) => m.msg).join('\n'))}"` : ''}><span class="lno">${ri + 1}</span><span class="yt" data-h="${h}">${html || ' '}</span></div>`;
    }).join('');
    const nadd = added.size, ndel = removedBefore.size;
    aside.querySelector('.yinfo').innerHTML = `<span class="vt">${ri + 1} 行 · ${num(md.chars)} 文字</span>${nadd || ndel ? `<span class="dirty">+${nadd} −${ndel}</span>` : ''}<span class="yr">${all.length ? `<span class="vcount">${counts(all)}</span>` : '<span class="vclean">✓ 問題なし</span>'}</span>`;
    box.scrollTop = S.yscroll[row.uid + row.l] || 0;
    follow(box, row.uid + row.l);
    applyStyles(aside);
  }

  function follow(box, key) {
    const el = box.querySelector('.yl.curf') || box.querySelector('.yl.cur');
    if (!el || !box.clientHeight) return;
    const last = [...box.querySelectorAll(el.classList.contains('curf') ? '.yl.curf' : '.yl.cur')].pop();
    const top = el.offsetTop, bot = last.offsetTop + last.offsetHeight;
    if (top < box.scrollTop + 4 || bot > box.scrollTop + box.clientHeight - 4) box.scrollTop = Math.max(0, top - Math.round(box.clientHeight * 0.3));
    S.yscroll[key] = box.scrollTop;
  }

  function applyStyles(root) {
    $$('[data-h]', root).forEach((el) => { el.style.setProperty('--h', el.dataset.h); });
  }

  function ruleKeyAt(d, si, ri) {
    const s = d.sections[si];
    if (!s) return 0;
    const x = s.rules[ri];
    return x ? x._k : 0;
  }

  function focusField(t) {
    const f = (t.closest('[data-f]') || {}).dataset ? t.closest('[data-f]').dataset.f : '';
    const oi = t.closest('.orow') ? t.closest('.orow').dataset.oi : null;
    if (oi != null) return `origin.${oi}` + ({ 'o-date': '.date', 'o-note': '.note', ref: '.refs' }[f] || '');
    if (['id', 'text', 'why', 'note', 'report', 'title', 'revision'].includes(f)) return f;
    if (f === 'send_why') return 'why';
    if (f === 'test' || t.closest('.ckrow')) return 'tests';
    if (['heading', 'level'].includes(f)) { const se = t.closest('.sec'); return se ? `sec.${se.dataset.si}` : ''; }
    if (f === 'intro') { const se = t.closest('.sec'); return se ? `sec.${se.dataset.si}.intro` : ''; }
    return '';
  }

  // ---- marking a drawn panel ------------------------------------------
  // diffCount is how many places the draft changed: a value written, a rule that
  // left the section it stood in, a section whose own keys changed — and one for
  // an order that only moved, whether one rule or one section (spec-roles I4,
  // I5: 並びだけが替わったら 1).
  function diffCount(e) {
    const s = e.saved, d = e.draft;
    if (!s) return 1;
    // A rule counts when its values changed, or when it left the section it
    // stood in. Section identity travels with a section (_s), so rules that
    // travel with their section are not counted one by one.
    const secOfSaved = new Map(), secOfDraft = new Map();
    s.sections.forEach((sec) => sec.rules.forEach((x) => secOfSaved.set(x._k, sec._s)));
    d.sections.forEach((sec) => sec.rules.forEach((x) => secOfDraft.set(x._k, sec._s)));
    let n = ['name', 'title', 'revision'].filter((f) => s[f] !== d[f]).length;
    const sr = new Map(rulesOf(s).map((x) => [x._k, JSON.stringify(x)])), dr = rulesOf(d);
    dr.forEach((x) => {
      if (sr.get(x._k) !== JSON.stringify(x) || secOfSaved.get(x._k) !== secOfDraft.get(x._k)) n++;
    });
    sr.forEach((v, k) => { if (!dr.some((x) => x._k === k)) n++; });
    // A section counts when its own keys changed or when it is new; a section
    // that was taken out counts once, below.
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
    const e = row.e, s = e.saved, d = e.draft, all = issues || row.issues;
    const marked = (el, ch) => el && el.classList.toggle('changed', !!ch);
    marked(panel.querySelector('[data-f=name]'), !s || s.name !== d.name);
    marked(panel.querySelector('[data-f=title]'), !s || s.title !== d.title);
    marked(panel.querySelector('[data-f=revision]'), !s || s.revision !== d.revision);
    const dhvm = panel.querySelector('.dhvm');
    if (dhvm) dhvm.innerHTML = all.filter((m) => m.k === 0).map(vmItem).join('');
    panel.querySelectorAll('.sec').forEach((el) => {
      const si = +el.dataset.si, a = s && s.sections[si], b = d.sections[si];
      if (!b) return;
      marked(el.querySelector('[data-f=heading]'), !a || a._s !== b._s || a.heading !== b.heading);
      marked(el.querySelector('.sech [data-f=level]'), !a || lvlMark(a.level) !== lvlMark(b.level));
      const cnt = el.querySelector('.sech .n');
      if (cnt) cnt.innerHTML = `${b.rules.length} 規則${outNote(outCount(b.rules))}`;
      marked(el.querySelector('[data-f=intro]'), !a || a.intro !== b.intro);
    });
    panel.querySelectorAll('.rule').forEach((el) => {
      const f = findRule(d, el.dataset.k);
      if (!f) return;
      const mine = all.filter((m) => m.k === f.x._k);
      el.querySelector('.rsum').innerHTML = rsumInner(row, f.x, mine);
      el.classList.toggle('unsaved', ruleChanged(e, f.x));
      el.classList.toggle('out', !inP(f.x));
      const ed = el.querySelector('.redit');
      if (ed) markEdit(ed, row, f.x, mine);
    });
    const hint = panel.querySelector('.rename');
    if (hint) {
      const nread = s && d.name && d.name !== s.name && host.readerCount ? host.readerCount(s.name) : 0;
      hint.hidden = !nread;
      if (nread) hint.querySelector('span').textContent = `読んでいる ${nread} 人の role も ${d.name || '—'} に書き換える`;
    }
    const n = diffCount(e);
    const foot = panel.querySelector('.pf');
    // The foot's own words are the calling screen's while a definition is not
    // there yet: the team screen draws that row the shape tab's foot, which says
    // 新規 and stops on the name rather than on this document's diff (監査 R9).
    if (foot && !row.newRow) {
      const vc = foot.querySelector('.vcount');
      if (vc) vc.innerHTML = counts(all);
      const dd = foot.querySelector('.dirty');
      if (dd) dd.textContent = s && n ? `${n} か所` : '';
      const saveBtn = foot.querySelector('[data-act=save]');
      if (saveBtn) {
        const before = new Set(s && row.savedIssues ? row.savedIssues.filter((m) => m.sev === 'err').map((m) => m.msg) : all.filter((m) => m.sev === 'err').map((m) => m.msg));
        saveBtn.disabled = !s ? all.some((m) => m.sev === 'err') : (all.some((m) => m.sev === 'err' && !before.has(m.msg)) || n === 0);
      }
    }
    refreshPrev(panel, all);
  }

  function markEdit(ed, row, x, issues) {
    const sr = savedRule(row.e, x._k);
    const marked = (el, ch) => el && el.classList.toggle('changed', !!ch);
    ['id', 'text', 'why', 'note', 'report'].forEach((f) => { const el = ed.querySelector(`[data-f=${f}]`); marked(el, !sr || sr[f] !== x[f]); });
    marked(ed.querySelector('[data-f=send_why]'), sr && sr.send_why !== x.send_why);
    marked(ed.querySelector('[data-f=in_prompt]'), sr && inP(sr) !== inP(x));
    marked(ed.querySelector('[data-f=why]'), sr && sr.why !== x.why);
    ed.querySelectorAll('.orow').forEach((r) => {
      const oi = +r.dataset.oi, so = sr && sr.origin[oi], o = x.origin[oi];
      if (!o) return;
      marked(r.querySelector('[data-f=o-kind]'), !so || so.kind !== o.kind);
      marked(r.querySelector('[data-f=o-date]'), !so || so.date !== o.date);
      marked(r.querySelector('[data-f=o-note]'), !so || so.note !== o.note);
      r.querySelectorAll('[data-f=ref]').forEach((el) => marked(el.closest('.chip'), !so || so.refs[+el.dataset.j] !== o.refs[+el.dataset.j]));
    });
    ed.querySelectorAll('[data-f=test]').forEach((el) => marked(el.closest('.chip'), !sr || sr.tests[+el.dataset.j] !== x.tests[+el.dataset.j]));
    ed.querySelectorAll('.bad').forEach((el) => el.classList.remove('bad'));
    ed.querySelectorAll('.vm[data-for]').forEach((u) => { u.innerHTML = issues.filter((m) => (m.f.split('.')[0] || '') === u.dataset.for).map(vmItem).join(''); });
    issues.forEach((m) => {
      const parts = m.f.split('.'), f = parts[0], oi = parts[1];
      let el = null;
      if (f === 'id') el = ed.querySelector('[data-f=id]');
      else if (f === 'text' && m.sev === 'err') el = ed.querySelector('[data-f=text]');
      else if (f === 'origin') {
        el = parts[2] === 'refs' ? (ed.querySelector(`.orow[data-oi="${oi}"] [data-f=ref][data-j="${parts[3]}"]`) || {}).closest?.('.chip')
          : ed.querySelector(`.orow[data-oi="${oi}"] [data-f=o-date]`);
      }
      if (el) el.classList.add('bad');
    });
    const rev = ed.querySelector('[data-act=rule-revert]');
    if (rev) rev.hidden = !sr || !ruleChanged(row.e, x);
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
  let root = null;

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
    const box = panel.querySelector('.ybox');
    if (box && box.clientHeight) S.yscroll[row.uid + row.l] = box.scrollTop;
    return host.rerender(panel, focusSel);
  }

  function re(c, sel) { return rerender(c.panel, sel); }

  // goRule opens one rule of a document and puts the cursor in its body
  // (spec-roles 1.2.4, 1.2.7, P15, 監査 R12): a screen that jumps to a rule from
  // a check, or from the address, reads through this, so the rule stands open
  // where the editor shows, brought into the window — never under the save's band
  // — with the body ready to be typed in. A rule the document does not hold
  // leaves the panel as it is.
  function goRule(panel, k) {
    const row = rowOf(panel);
    if (!row || !k) return null;
    S.ropen.add(rkey(row, { _k: k }));
    if (host.mode) host.mode(panel, 'edit');
    S.pfocus[row.uid + row.l] = { k, f: 'text' };
    const fresh = rerender(panel, null);
    const el = fresh && fresh.querySelector(`.rule[data-k="${k}"] [data-f=text]`);
    if (el) {
      // The rule is brought into the middle of the window, so the band the
      // screen pins to the bottom never hides it; a rule taller than the window
      // keeps its head in it.
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
        re(c, `.rule[data-k="${c.x._k}"] .rsum`); host.url && host.url(); break;
      }
      case 'sec-pick': S.secPick[sk] = S.secPick[sk] === b.dataset.w ? null : b.dataset.w; re(c, `.sec[data-si="${c.si}"] [data-act=sec-pick][data-w=${b.dataset.w}]`); break;
      case 'sec-level': {
        const s0 = c.d.sections[c.si], v = +b.dataset.v;
        // Choosing ## for a section the file wrote no level for leaves it
        // unwritten (0), so the save adds no level: line (A2).
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
        const x = { _k: ++kseq, id: nextID(row), text: '', why: '', send_why: true, in_prompt: true, note: '', report: '', tests: [], origin: [] };
        c.d.sections[c.si].rules.push(x); S.ropen.add(rkey(row, x));
        re(c, `.rule[data-k="${x._k}"] [data-f=text]`); edited(c); break;
      }
      case 'rule-up': case 'rule-down': moveRule(c.d, c.si, c.ri, act === 'rule-up' ? -1 : 1); re(c, `.rule[data-k="${c.x._k}"] [data-act=${act}]`); edited(c); break;
      case 'rule-revert': { const sx = clone(savedRule(row.e, c.x._k)); if (sx) { sx._k = c.x._k; c.d.sections[c.si].rules[c.ri] = sx; } re(c, `.rule[data-k="${c.x._k}"] .rsum`); edited(c); break; }
      case 'rule-del': S.confirm[ck] = 1; re(c, `.rule[data-k="${c.x._k}"] [data-act=rule-del-yes]`); break;
      case 'rule-del-no': delete S.confirm[ck]; re(c); break;
      case 'rule-del-yes': delete S.confirm[ck]; c.d.sections[c.si].rules.splice(c.ri, 1); re(c); edited(c); break;
      case 'in-prompt': c.x.in_prompt = b.dataset.v === 'true'; S.pfocus[pkey] = { k: c.x._k, f: 'text' }; re(c, `.rule[data-k="${c.x._k}"] [data-act=in-prompt].on`); edited(c); break;
      case 'send-why': c.x.send_why = b.dataset.v === 'true'; re(c, `.rule[data-k="${c.x._k}"] [data-act=send-why].on`); edited(c); break;
      case 'o-kind': c.x.origin[c.oi].kind = b.dataset.v; re(c, `.rule[data-k="${c.x._k}"] .orow[data-oi="${c.oi}"] [data-act=o-kind].on`); edited(c); break;
      case 'o-add': c.x.origin.push({ kind: 'incident', date: '', refs: [], note: '' }); re(c, `.rule[data-k="${c.x._k}"] .orow[data-oi="${c.x.origin.length - 1}"] [data-f=o-date]`); edited(c); break;
      case 'o-del': c.x.origin.splice(c.oi, 1); re(c); edited(c); break;
      case 'ref-add': c.x.origin[c.oi].refs.push(''); re(c, `.rule[data-k="${c.x._k}"] .orow[data-oi="${c.oi}"] [data-f=ref][data-j="${c.x.origin[c.oi].refs.length - 1}"]`); break;
      case 'ref-del': c.x.origin[c.oi].refs.splice(+b.dataset.j, 1); re(c); edited(c); break;
      case 'test-add': c.x.tests.push(''); re(c, `.rule[data-k="${c.x._k}"] [data-f=test][data-j="${c.x.tests.length - 1}"]`); break;
      case 'test-del': c.x.tests.splice(+b.dataset.j, 1); re(c); edited(c); break;
      case 'yline': {
        const k = +b.dataset.k, si = b.dataset.si, part = b.dataset.part;
        if (k) S.ropen.add(`${row.uid}${row.l}:${k}`);
        host.mode && host.mode(c.panel, 'edit');
        S.pfocus[pkey] = k ? { k, f: 'text' } : part === 'title' ? { k: 0, f: 'title' } : part === 'intro' ? { k: 0, f: `sec.${si}.intro` } : { k: 0, f: `sec.${si}` };
        // The Markdown box follows to the section's first rule: the panel is
        // redrawn first, then the fresh box is scrolled (A2, P15).
        const fresh = host.rerender(c.panel, null);
        const target = k ? `.rule[data-k="${k}"] [data-f=text]` : part === 'title' ? '[data-f=title]' : `.sec[data-si="${si}"] [data-f=${part === 'intro' ? 'intro' : 'heading'}]`;
        if (fresh) { const el = fresh.querySelector(target); if (el) { el.scrollIntoView({ block: 'nearest' }); el.focus({ preventScroll: true }); } }
        host.url && host.url(); break;
      }
      default: return false;
    }
    return true;
  }

  function handleInput(t) {
    const c = ctx(t);
    if (!c.panel || !c.d || !t.dataset.f) return false;
    const v = t.value, j = +t.dataset.j;
    switch (t.dataset.f) {
      case 'name': c.d.name = v; { const p = c.panel.querySelector('.path'); if (p) p.textContent = host.path ? host.path(c.row.l, c.d) : p.textContent; } break;
      case 'title': c.d.title = v; break;
      case 'revision': c.d.revision = v === '' ? null : +v; break;
      case 'heading': c.d.sections[c.si].heading = v; break;
      case 'intro': c.d.sections[c.si].intro = v; break;
      case 'id': case 'text': case 'why': case 'note': case 'report': c.x[t.dataset.f] = v; break;
      case 'o-date': c.x.origin[c.oi].date = v; break;
      case 'o-note': c.x.origin[c.oi].note = v; break;
      case 'ref': c.x.origin[c.oi].refs[j] = v; t.size = Math.max(v.length, 5); break;
      case 'test': c.x.tests[j] = v; t.size = Math.max(v.length, 6); break;
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
      refreshPrev(c.panel);
    });
    root.addEventListener('scroll', (e) => {
      if (e.target.matches?.('.ybox')) { const p = e.target.closest('.panel'); if (p) S.yscroll[p.dataset.uid + p.dataset.l] = e.target.scrollTop; }
    }, true);
    // A rule is dragged by its ⋮ handle: above or below another rule's row, or
    // to the end of a section (A5). The drop line is 2px of the accent, and the
    // row that is dragged goes faint.
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
    nextKey: () => ++kseq,
    observeKeys: (rules) => { rules.forEach((x) => { if (x._k > kseq) kseq = x._k; }); },
  };
})();