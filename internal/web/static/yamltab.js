// yamltab.js — the YAML tab as one shared part (spec-boundary 7, r-2cc2).
// One section stands for one file: the line numbers down its left side, the
// check the person's typing rides on (300ms after it stops), the save that
// writes the whole text with the hash the page read, and the jump a problem's
// line makes. The server writes every sentence a problem shows; this file only
// lays them out (spec-boundary 3). A screen draws the sections themselves and
// hands each one here.
window.hergyYaml = (() => {
  'use strict';

  const API = window.hergyAPI;
  const $ = API.$;
  const $$ = API.$$;
  const esc = (s) => String(s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  const sevChip = (sev) => `<span class="sev ${sev === 'err' ? 'err' : 'warn'}">${sev === 'err' ? '問題' : '警告'}</span>`;
  const countHTML = (errs, warns) =>
    `${errs ? `<span class="sev err">問題 ${errs}</span>` : ''}${warns ? `<span class="sev warn">警告 ${warns}</span>` : ''}`;

  // checkText asks the server what is wrong with one file's text. The kind says
  // which files these are, and extra carries the kind's own word for which one
  // (a team definition's team.yaml and agreement.yaml share one kind); the same
  // words answer a panel's draft.
  async function checkText(kind, place, name, text, extra) {
    const answer = await API.check(kind, Object.assign({ place, name, text }, extra || {}));
    return (answer.json && answer.json.problems) || [];
  }

  // saveText writes one file's whole text. The answer carries the new hash. The
  // name travels in the payload as well as in the URL: a kind that reads the
  // file's name from the body rather than from the path (a role's file is the
  // row's own name) writes the file it was handed (r-8e85).
  async function saveText(kind, name, place, text, hash, extra) {
    return API.save(kind, name, Object.assign({ place, name, text, hash: hash || '' }, extra || {}));
  }

  // tellDirty lets the page mark itself as carrying unsaved work, the same way
  // the panels do (the browser's beforeunload reads it).
  function tellDirty() {
    if (window.hergyPage && window.hergyPage.tellDirty) window.hergyPage.tellDirty();
  }

  function autosize(textarea) {
    if (CSS.supports('field-sizing', 'content')) return;
    textarea.style.height = 'auto';
    textarea.style.height = `${textarea.scrollHeight + 2}px`;
  }

  // errTexts is the sentences of the errors of one reading, the way a save gate
  // compares a text against the one that is written (spec-roles X3).
  const errTexts = (problems) => (problems || []).filter((p) => p.sev === 'err').map((p) => p.text);

  // foot keeps the section's 保存 / 元に戻す in step with the text and the
  // problems: a text that has grown an error the written file did not have
  // cannot be saved (the server refuses it anyway, but the button says so
  // first). An error the file already had does not stop a save (spec-roles X3).
  function foot(section) {
    const textarea = $('.ytext', section);
    const dirty = textarea.value !== section._base;
    const problems = section._problems || [];
    const before = errTexts(section._baseProblems);
    const grew = problems.some((p) => p.sev === 'err' && before.indexOf(p.text) < 0);
    if (dirty) $$('.ok', section).forEach((el) => el.remove());
    $('.dirty', section).textContent = dirty ? '変更あり' : '';
    $('[data-act=yaml-save]', section).disabled = !dirty || grew;
    $('[data-act=yaml-revert]', section).hidden = !dirty;
    tellDirty();
  }

  // note is the part's own 保存しました, the one every screen shows after a save
  // (the designer put this in the part, r-0f8c).
  function note(section) {
    $$('.ok', section).forEach((el) => el.remove());
    const span = document.createElement('span');
    span.className = 'ok';
    span.textContent = '保存しました';
    const acts = $('.acts', section);
    if (acts) acts.parentNode.insertBefore(span, acts); else section.appendChild(span);
  }

  // render lays out what the server said about the text: the numbered gutter
  // with a mark on each bad line, the height the file needs, and the list of
  // problems under the text.
  function render(section, problems) {
    const textarea = $('.ytext', section);
    const lines = textarea.value.split('\n');
    const marks = {};
    problems.forEach((p) => { if (marks[p.line] !== 'err') marks[p.line] = p.sev; });
    $('.gutter', section).innerHTML = lines.map((_, i) => `<span class="${marks[i + 1] || ''}">${i + 1}</span>`).join('');
    const height = Math.min(Math.max(lines.length, 6), 34);
    const code = $('.code', section);
    code.className = `${code.className.replace(/\bh\d+\b/g, '').replace(/\s+/g, ' ').trim()} h${height}`;
    const errs = problems.filter((p) => p.sev === 'err').length;
    const warns = problems.length - errs;
    $('.vlist', section).innerHTML = `<div class="vhead"><span class="vt">validate</span>${problems.length ? `<span class="vcount">${countHTML(errs, warns)}</span>` : '<span class="vclean">✓ 問題なし</span>'}</div>` +
      problems.map((p) => `<button class="vitem" data-act="yaml-jump" data-line="${p.line}">${sevChip(p.sev)}<span class="lno">${p.line} 行</span><span class="msg">${esc(p.text)}</span></button>`).join('');
    $('.gutter', section).scrollTop = textarea.scrollTop;
    section._problems = problems;
    if (textarea.value === section._base) section._baseProblems = problems;
    if (section._onProblems) section._onProblems(section, problems);
    foot(section);
  }

  function schedule(section) {
    API.after(section, 300, () => check(section));
  }

  async function check(section) {
    const textarea = $('.ytext', section);
    const problems = await checkText(section._kind, section._place, section._name, textarea.value, section._extra);
    if (!section.isConnected) return;
    render(section, problems);
  }

  async function save(section) {
    const textarea = $('.ytext', section);
    const answer = await saveText(section._kind, section._name, section._place, textarea.value, section.dataset.hash || '', section._extra);
    if (!answer.ok) {
      API.refusal(section, API.problemText(answer, '保存できませんでした'));
      return;
    }
    section._base = textarea.value;
    if (answer.json && answer.json.hash) section.dataset.hash = answer.json.hash;
    if (section._onSaved) section._onSaved(section, answer);
    render(section, section._problems || []);
    check(section);
    note(section);
    API.saved();
  }

  function revert(section) {
    const textarea = $('.ytext', section);
    textarea.value = section._base;
    $$('.ok', section).forEach((el) => el.remove());
    render(section, section._problems || []);
    check(section);
  }

  function jump(section, line) {
    const textarea = $('.ytext', section);
    const lines = textarea.value.split('\n');
    const start = lines.slice(0, line - 1).reduce((n, s) => n + s.length + 1, 0);
    textarea.focus({ preventScroll: true });
    textarea.setSelectionRange(start, start + (lines[line - 1] || '').length);
    textarea.scrollTop = Math.max(0, (line - 4) * 20);
    textarea.scrollLeft = 0;
  }

  function insertTab(textarea) {
    textarea.setRangeText('  ', textarea.selectionStart, textarea.selectionEnd, 'end');
    textarea.dispatchEvent(new Event('input', { bubbles: true }));
  }

  // mount takes over one section the screen has already drawn: it remembers the
  // text the page read as the base, marks the section with its kind, place and
  // name, lays out the problems the page came with, and asks the server what is
  // wrong with it now.
  function mount(section, opts) {
    section._kind = opts.kind;
    section._place = opts.place;
    section._name = opts.name;
    section._extra = opts.extra || null;
    section._onProblems = opts.onProblems || null;
    section._onSaved = opts.onSaved || null;
    const textarea = $('.ytext', section);
    section._base = textarea.value;
    section._problems = [];
    section._baseProblems = [];
    render(section, opts.problems || []);
    check(section);
  }

  return { mount, schedule, check, render, foot, save, revert, jump, insertTab, autosize, checkText, saveText, tellDirty };
})();