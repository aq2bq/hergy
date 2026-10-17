// The profile screen's behaviour (r-c683). The page comes from the server; this
// script keeps the drafts in the page, asks the server to check a draft while it
// is typed, and puts the server's answer in place of the row or card that was
// saved. Nothing is held anywhere but the page, and a save carries the hash of
// the text the page read.
(() => {
  'use strict';

  const view = document.getElementById('view');
  if (!view) return;
  const token = view.dataset.token;
  const tab = view.dataset.tab;
  const otherKinds = () => {
    try { return JSON.parse(view.dataset.otherKinds || '[]'); } catch (e) { return []; }
  };
  const BASE_KINDS = ['claude', 'codex', 'opencode', 'gemini'];

  const $ = (sel, root = document) => root.querySelector(sel);
  const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));
  const esc = (s) => String(s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  const classOf = (kind) => (BASE_KINDS.includes(kind) ? 'k-' + kind : '');
  const labelOf = (file) => (file === 'global' ? '共通' : 'プロジェクト');
  const pathOf = (file) => (file === 'global' ? view.dataset.pathShared : view.dataset.pathProject);
  const otherOf = (file) => (file === 'global' ? 'project' : 'global');

  // post sends one write: the token of this run and the form's fields.
  async function post(url, fields) {
    const body = new URLSearchParams();
    body.set('token', token);
    for (const [key, value] of Object.entries(fields)) body.set(key, value);
    const response = await fetch(url, {
      method: 'POST', credentials: 'same-origin',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded;charset=UTF-8' },
      body,
    });
    const text = await response.text();
    let json = null;
    if ((response.headers.get('content-type') || '').includes('application/json')) {
      try { json = JSON.parse(text); } catch (e) { json = null; }
    }
    return { ok: response.ok, status: response.status, html: text, json, headers: response.headers };
  }

  function updateCounts(headers) {
    const profiles = $$('#tabs .tab')[0];
    const assign = $$('#tabs .tab')[1];
    const p = headers.get('X-Hergy-Profiles');
    const a = headers.get('X-Hergy-Assign');
    if (p !== null && profiles) $('.n', profiles).textContent = p;
    if (a !== null && assign) $('.n', assign).textContent = a;
    const hp = headers.get('X-Hergy-Hash-Project');
    const hs = headers.get('X-Hergy-Hash-Shared');
    if (hp !== null) view.dataset.hashProject = hp;
    if (hs !== null) view.dataset.hashShared = hs;
  }

  const chip = (sev) => `<span class="sev ${sev === 'err' ? 'err' : 'warn'}">${sev === 'err' ? '問題' : '警告'}</span>`;
  const counts = (errs, warns) => `${errs ? `<span class="sev err">問題 ${errs}</span>` : ''}${warns ? `<span class="sev warn">警告 ${warns}</span>` : ''}`;

  // ---- the profile tab -------------------------------------------------

  function savedOf(panel) {
    if (panel.dataset.saved !== '1') return null;
    try {
      return {
        name: panel.dataset.savedName || '',
        kind: panel.dataset.savedKind || '',
        args: JSON.parse(panel.dataset.savedArgs || '[]'),
        env: JSON.parse(panel.dataset.savedEnv || '[]'),
      };
    } catch (e) { return null; }
  }

  function panelKind(panel) {
    return panel.dataset.draftKind || (savedOf(panel) || {}).kind || 'claude';
  }

  function panelValues(panel) {
    return {
      name: $('[data-f=name]', panel).value,
      kind: panelKind(panel),
      args: $$('[data-f=arg]', panel).map((input) => input.value),
      env: $$('.envrow', panel).map((row) => ({ key: $('[data-f=ek]', row).value, value: $('[data-f=ev]', row).value })),
    };
  }

  function changeCount(panel) {
    const saved = savedOf(panel);
    const draft = panelValues(panel);
    let n = 0;
    if (!saved || draft.name !== saved.name) n++;
    if (!saved || draft.kind !== saved.kind) n++;
    draft.args.forEach((arg, i) => { if (!saved || saved.args[i] !== arg) n++; });
    if (saved && saved.args.length > draft.args.length) n += saved.args.length - draft.args.length;
    draft.env.forEach((pair, i) => {
      const before = saved && saved.env[i];
      if (!before || before.key !== pair.key) n++;
      if (!before || before.value !== pair.value) n++;
    });
    if (saved && saved.env.length > draft.env.length) n += saved.env.length - draft.env.length;
    return n;
  }

  function setChanged(el, changed) {
    if (el) el.classList.toggle('changed', !!changed);
  }

  function mark(panel) {
    const saved = savedOf(panel);
    const draft = panelValues(panel);
    setChanged($('[data-f=name]', panel), !saved || draft.name !== saved.name);
    setChanged($('[data-f=kind]', panel), !saved || draft.kind !== saved.kind);
    $$('.args > .arg', panel).forEach((arg, i) => setChanged(arg, !saved || saved.args[i] !== $('input', arg).value));
    $$('.envrow', panel).forEach((row, i) => {
      const before = saved && saved.env[i];
      setChanged($('[data-f=ek]', row), !before || before.key !== $('[data-f=ek]', row).value);
      setChanged($('[data-f=ev]', row), !before || before.value !== $('[data-f=ev]', row).value);
    });
    const hint = $('.rename', panel);
    const users = Number(panel.dataset.renameUsers || 0);
    const show = saved && draft.name && draft.name !== saved.name && users > 0;
    hint.hidden = !show;
    if (show) $('span', hint).textContent = `使っている ${users} 人の指定も ${draft.name} に書き換える`;
    updateSave(panel);
  }

  function updateSave(panel) {
    const saved = savedOf(panel);
    const n = changeCount(panel);
    const name = $('[data-f=name]', panel).value;
    const save = $('[data-act=save]', panel);
    if (save) save.disabled = (panel._errors || 0) > 0 || (saved ? n === 0 : false) || name === '';
    const dirty = $('.dirty', panel);
    if (dirty) dirty.textContent = saved && n ? `${n} か所` : '';
    const row = panel.closest('.row');
    if (row) markRowDirty(row);
  }

  function markRowDirty(row) {
    const dirty = $$('.panel', row).some((panel) => savedOf(panel) === null || changeCount(panel) > 0);
    row.classList.toggle('unsaved', dirty);
  }

  function markBad(panel, field) {
    const parts = field.split('.');
    if (parts[0] === 'name') { $('[data-f=name]', panel).classList.add('bad'); return; }
    if (parts[0] === 'kind') { $('[data-f=kind]', panel).classList.add('bad'); return; }
    if (parts[0] === 'args') {
      const arg = $$('.args > .arg', panel)[Number(parts[1])];
      if (arg) arg.classList.add('bad');
      return;
    }
    if (parts[0] === 'env') {
      const row = $$('.envrow', panel)[Number(parts[1])];
      if (!row) return;
      const input = parts[2] === 'value' ? $('[data-f=ev]', row) : $('[data-f=ek]', row);
      if (input) input.classList.add('bad');
    }
  }

  function renderProblems(panel, problems) {
    $$('.bad', panel).forEach((el) => el.classList.remove('bad'));
    $$('.vm[data-for]', panel).forEach((ul) => { ul.innerHTML = ''; });
    const groups = {};
    for (const problem of problems) {
      const field = (problem.field || '').split('.')[0];
      (groups[field] = groups[field] || []).push(problem);
      markBad(panel, problem.field || '');
    }
    for (const [field, list] of Object.entries(groups)) {
      const ul = $(`.vm[data-for="${field}"]`, panel);
      if (ul) ul.innerHTML = list.map((p) => `<li>${chip(p.sev)}<span>${esc(p.text)}</span></li>`).join('');
    }
    const errs = problems.filter((p) => p.sev === 'err').length;
    const warns = problems.length - errs;
    const counter = $('.vcount', panel);
    if (counter) counter.innerHTML = counts(errs, warns);
  }

  function scheduleCheck(panel) {
    clearTimeout(panel._timer);
    panel._timer = setTimeout(() => check(panel), 300);
  }

  async function check(panel) {
    const saved = savedOf(panel);
    const draft = panelValues(panel);
    const seq = (panel._seq || 0) + 1;
    panel._seq = seq;
    const answer = await post('/profiles/check', {
      file: panel.dataset.file, original: saved ? saved.name : '', profile: JSON.stringify(draft),
    });
    if (seq !== panel._seq || !panel.isConnected) return;
    const problems = (answer.json && answer.json.problems) || [];
    renderProblems(panel, problems);
    panel._errors = problems.filter((p) => p.sev === 'err').length;
    updateSave(panel);
  }

  function initPanel(panel) {
    if (panel.dataset.ready === '1') { mark(panel); return; }
    panel.dataset.ready = '1';
    const saved = savedOf(panel);
    panel.dataset.draftKind = saved ? saved.kind : (panel.dataset.draftKind || 'claude');
    mark(panel);
    check(panel);
  }

  function initRow(row) {
    if (row.classList.contains('open')) $$('.panel', row).forEach(initPanel);
  }

  // replaceRows puts the list a save answered with in place of the rows, so a
  // rename that split one row into two shows both, in the list's order (N1).
  function replaceRows(html) {
    const rows = $('#rows');
    $$('.row', rows).forEach((row) => row.remove());
    rows.insertAdjacentHTML('beforeend', html || '');
    $$('.row', rows).forEach(initRow);
    afterRowsChanged();
  }

  function replaceRow(old, html) {
    const holder = document.createElement('div');
    holder.innerHTML = (html || '').trim();
    const row = holder.firstElementChild;
    if (!row) { old.remove(); afterRowsChanged(); return; }
    old.replaceWith(row);
    initRow(row);
    afterRowsChanged();
  }

  // afterRowsChanged keeps the toolbar's counts and the empty line true after a
  // row was added or taken away.
  function afterRowsChanged() {
    const rows = $$('#rows .row');
    const counts = {};
    rows.forEach((row) => { counts[row.dataset.kind] = (counts[row.dataset.kind] || 0) + 1; });
    $$('.fchip').forEach((chip) => {
      const kind = chip.dataset.k || '';
      const count = kind ? (counts[kind] || 0) : rows.length;
      const number = $('.n', chip);
      if (number) number.textContent = count;
    });
    const empty = $('#rows > p.none:not(#no-match)');
    if (rows.length === 0 && !empty) {
      const line = document.createElement('p');
      line.className = 'none';
      line.textContent = 'プロファイルはありません';
      $('#rows').insertBefore(line, $('#rows').firstChild);
    } else if (rows.length > 0 && empty) {
      empty.remove();
    }
    applyFilter();
    if (tab === 'profiles') updateProfilesURL();
  }

  function showRefusal(box, text) {
    $$('.refusal', box).forEach((el) => el.remove());
    const span = document.createElement('span');
    span.className = 'sev err refusal';
    span.textContent = text;
    const acts = $('.acts', box);
    if (acts) acts.parentNode.insertBefore(span, acts); else box.appendChild(span);
  }

  function kindButtons(kind) {
    return BASE_KINDS.map((k) => `<button class="${kind === k ? 'on' : ''}" data-act="set-kind" data-k="${k}" aria-pressed="${kind === k}"><i class="sw ${classOf(k)}"></i>${k}</button>`).join('');
  }

  function kindSelect(kind) {
    const rest = otherKinds().filter((k) => !BASE_KINDS.includes(k));
    if (kind && !BASE_KINDS.includes(kind) && !rest.includes(kind)) rest.push(kind);
    return `<span class="sel"><select data-f="kind-other" aria-label="ほかの kind"><option value="">ほか</option>${rest.map((k) => `<option value="${esc(k)}"${k === kind ? ' selected' : ''}>${esc(k)}</option>`).join('')}</select></span>`;
  }

  function argHTML(i, arg) {
    const size = Math.max(String(arg).length, 3);
    return `<span class="arg${String(arg).startsWith('-') ? ' flag' : ''}"><span class="grip" aria-hidden="true">⋮</span><input data-f="arg" data-i="${i}" value="${esc(arg)}" size="${size}" spellcheck="false" autocomplete="off"><button class="x" data-act="arg-del" data-i="${i}" aria-label="${esc(arg)} を消す">×</button></span>`;
  }

  function envHTML(i, key, value) {
    return `<div class="envrow"><input data-f="ek" data-i="${i}" value="${esc(key)}" spellcheck="false" autocomplete="off"><span class="eq">=</span><textarea data-f="ev" data-i="${i}" rows="1" spellcheck="false">${esc(value)}</textarea><button class="x" data-act="env-del" data-i="${i}" aria-label="${esc(key)} を消す">×</button></div>`;
  }

  function panelHTML(file, values, layerSwitch) {
    const head = layerSwitch
      ? `<div class="seg small">${['project', 'global'].map((x) => `<button class="${file === x ? 'on' : ''}" data-act="pick-layer" data-x="${x}">${labelOf(x)}</button>`).join('')}</div>`
      : `<span class="ln">${labelOf(file)}</span>`;
    return `<section class="panel" data-file="${file}" data-users="0" data-draft-kind="${esc(values.kind)}">
  <header class="ph">${head}<code class="path">${esc(pathOf(file))}</code></header>
  <div class="field"><label>名前</label><div>
    <input class="in-name" data-f="name" value="${esc(values.name)}" spellcheck="false" autocomplete="off">
    <label class="rename" hidden><input type="checkbox" checked><span></span></label>
    <ul class="vm" data-for="name"></ul>
  </div></div>
  <div class="field"><label>kind</label><div>
    <div class="seg" data-f="kind">${kindButtons(values.kind)}${kindSelect(values.kind)}</div>
    <ul class="vm" data-for="kind"></ul>
  </div></div>
  <div class="field"><label>引数</label><div>
    <div class="args"><span class="bin">${esc(values.kind)}</span>${values.args.map((arg, i) => argHTML(i, arg)).join('')}<button class="add" data-act="arg-add" aria-label="引数を足す">＋</button></div>
    <ul class="vm" data-for="args"></ul>
  </div></div>
  <div class="field"><label>環境変数</label><div>
    <div class="env">${values.env.map((pair, i) => envHTML(i, pair.key, pair.value)).join('')}<button class="add" data-act="env-add" aria-label="環境変数を足す">＋</button></div>
    <ul class="vm" data-for="env"></ul>
  </div></div>
  <footer class="pf">
    <button class="btn primary" data-act="save" disabled>保存</button><span class="dirty"></span><span class="vcount"></span>
    <span class="acts"><button class="btn danger" data-act="discard">取り消し</button></span>
  </footer>
</section>`;
  }

  function newRow(file) {
    const row = document.createElement('div');
    row.className = 'row open unsaved';
    row.dataset.name = '';
    row.dataset.kind = 'claude';
    row.innerHTML = `<div class="sum" role="button" tabindex="0" aria-expanded="true" data-act="toggle">
      <span class="chev"></span>
      <span class="c-name"><span class="nm"><span class="none">—</span></span><span class="tag">新規</span></span>
      <span class="c-kind"><i class="sw k-claude"></i>claude</span>
      <span class="c-place"><span class="pill pending">${labelOf(file)}</span></span>
      <span class="c-args"><span class="none">—</span></span>
      <span class="c-env"><span class="none">—</span></span>
      <span class="c-users"><span class="none">0</span></span>
    </div><div class="detail">${panelHTML(file, { name: '', kind: 'claude', args: ['--model', ''], env: [] }, true)}</div>`;
    return row;
  }

  function renumberArgs(panel) {
    $$('.args > .arg', panel).forEach((arg, i) => {
      $('input', arg).dataset.i = i;
      $('.x', arg).dataset.i = i;
    });
  }

  function renumberEnv(panel) {
    $$('.envrow', panel).forEach((row, i) => {
      $('[data-f=ek]', row).dataset.i = i;
      $('[data-f=ev]', row).dataset.i = i;
      $('.x', row).dataset.i = i;
    });
  }

  function activeKind() {
    const chip = $('#rows') && $$('.fchip.on')[0];
    return chip ? (chip.dataset.k || '') : '';
  }

  function applyFilter() {
    const kind = activeKind();
    const q = ($('#q') || {}).value || '';
    let shown = 0;
    $$('#rows .row').forEach((row) => {
      const ok = (!kind || row.dataset.kind === kind) && (!q || (row.dataset.name || '').includes(q));
      row.hidden = !ok;
      if (ok) shown++;
    });
    const empty = $$('#rows > .none').some((el) => el.id !== 'no-match' && !el.hidden);
    const noMatch = $('#no-match');
    if (noMatch) noMatch.hidden = empty || shown > 0;
  }

  function updateProfilesURL() {
    const params = new URLSearchParams();
    const kind = activeKind();
    if (kind) params.set('kind', kind);
    const q = ($('#q') || {}).value || '';
    if (q) params.set('q', q);
    const open = $$('#rows .row.open').map((row) => row.dataset.name).filter(Boolean);
    if (open.length) params.set('open', open.join(','));
    const query = params.toString();
    history.replaceState(null, '', location.pathname + (query ? '?' + query : ''));
  }

  function clearFilter() {
    $$('.fchip').forEach((chip) => { chip.classList.toggle('on', !chip.dataset.k); chip.setAttribute('aria-pressed', String(!chip.dataset.k)); });
    if ($('#q')) $('#q').value = '';
    applyFilter();
    updateProfilesURL();
  }

  function initProfiles() {
    $$('#rows .row').forEach(initRow);
    applyFilter();
  }

  // ---- the assign tab --------------------------------------------------

  function optionKind(select) {
    const option = select.options[select.selectedIndex];
    return option ? (option.dataset.kind || '') : '';
  }

  function assignChange(select) {
    const row = select.closest('.arow');
    const team = select.closest('.team');
    const saved = row.dataset.saved || '';
    const changed = select.value !== saved;
    row.classList.toggle('changed', changed);
    row.classList.remove('missing');
    $$('.vm', row).forEach((el) => el.remove());
    let was = $('s.was', row);
    if (changed) {
      if (!was) { was = document.createElement('s'); was.className = 'was'; was.textContent = saved; }
      $('.pick', row).insertBefore(was, $('.sw', row));
    } else if (was) { was.remove(); }
    const sw = $('.sw', row);
    if (sw) sw.className = 'sw ' + classOf(optionKind(select));
    updateTeam(team);
    updateKindbar();
  }

  function updateTeam(team) {
    const rows = $$('.arow', team);
    const dirty = rows.filter((row) => $('select', row).value !== (row.dataset.saved || '')).length;
    const missing = rows.filter((row) => row.classList.contains('missing')).length;
    const side = $('.tside', team);
    $$('.dirty, [data-act=g-save]', side).forEach((el) => el.remove());
    if (dirty > 0) {
      $$('.ok', side).forEach((el) => el.remove());
      const count = document.createElement('span');
      count.className = 'dirty';
      count.textContent = `${dirty} 人`;
      side.appendChild(count);
      const save = document.createElement('button');
      save.className = 'btn primary';
      save.dataset.act = 'g-save';
      save.textContent = '保存';
      save.disabled = missing > 0;
      side.appendChild(save);
    }
  }

  function updateKindbar() {
    const counts = {};
    $$('.arow select').forEach((select) => {
      const kind = optionKind(select);
      if (kind) counts[kind] = (counts[kind] || 0) + 1;
    });
    $$('.kseg').forEach((segment) => {
      const count = counts[segment.dataset.k] || 0;
      $('b', segment).textContent = count;
      const grow = Math.max(1, Math.min(20, count));
      segment.className = segment.className.replace(/\bg\d+\b/g, '').replace(/\s+/g, ' ').trim() + ' g' + grow;
    });
  }

  function initAssign() {
    $$('.team').forEach(updateTeam);
    updateKindbar();
    applyAssignKind();
    flashHash();
  }

  // applyAssignKind draws the kind a reloaded URL carries: the other segments
  // and people go thin, the chosen one stays plain (R6).
  function applyAssignKind() {
    const kind = new URLSearchParams(location.search).get('kind');
    $$('.kseg').forEach((segment) => {
      const on = kind !== null && segment.dataset.k === kind;
      segment.classList.toggle('dim', kind !== null && !on);
      segment.setAttribute('aria-pressed', String(on));
    });
    $$('.arow').forEach((row) => {
      const select = $('select', row);
      const dim = kind !== null && optionKind(select) !== kind;
      row.classList.toggle('dim', dim);
    });
  }

  // updateAssignURL keeps the chosen kind in the address, so a reload shows the
  // same thinned cards (仕様 2 節).
  function updateAssignURL(kind) {
    const params = new URLSearchParams(location.search);
    if (kind) params.set('kind', kind); else params.delete('kind');
    const query = params.toString();
    history.replaceState(null, '', location.pathname + (query ? '?' + query : ''));
  }

  function flashHash() {
    if (!location.hash) return;
    const id = location.hash.slice(1);
    const dot = id.lastIndexOf('.');
    if (dot < 0) return;
    const teamName = id.slice(0, dot);
    const key = id.slice(dot + 1);
    const team = $$('.team').find((el) => el.dataset.g === teamName);
    const row = team && $$('.arow', team).find((el) => el.dataset.k === key);
    if (!row) return;
    row.scrollIntoView({ block: 'center' });
    row.classList.add('flash');
    const select = $('select', row);
    if (select) select.focus({ preventScroll: true });
  }

  async function saveTeam(team) {
    const changes = [];
    $$('.arow select', team).forEach((select) => {
      const row = select.closest('.arow');
      if (select.value === (row.dataset.saved || '')) return;
      const key = row.dataset.k;
      if (key.startsWith('m')) {
        changes.push({ member: $('.who code', row).textContent, profile: select.value });
      } else {
        changes.push({
          from: $('.role code', row).textContent.replace(/^from /, ''),
          purpose: $('.purpose', row).textContent,
          profile: select.value,
        });
      }
    });
    const answer = await post('/profiles/assign/save', {
      definition: team.dataset.g, changes: JSON.stringify(changes), hash: team.dataset.hash,
    });
    if (!answer.ok) {
      const side = $('.tside', team);
      let refusal = $('.refusal', side);
      if (!refusal) { refusal = document.createElement('span'); refusal.className = 'sev err refusal'; side.appendChild(refusal); }
      refusal.textContent = (answer.json && answer.json.text) || '保存できませんでした';
      return;
    }
    const holder = document.createElement('div');
    holder.innerHTML = answer.html.trim();
    const card = holder.firstElementChild;
    if (card) { team.replaceWith(card); initAssign(); }
    updateCounts(answer.headers);
  }

  // ---- the YAML tab ----------------------------------------------------

  function autosize(textarea) {
    if (CSS.supports('field-sizing', 'content')) return;
    textarea.style.height = 'auto';
    textarea.style.height = (textarea.scrollHeight + 2) + 'px';
  }

  function yamlFoot(section, problems) {
    const textarea = $('.ytext', section);
    const dirty = textarea.value !== section._base;
    $('.dirty', section).textContent = dirty ? '変更あり' : '';
    $('[data-act=yaml-save]', section).disabled = !dirty || problems.some((p) => p.sev === 'err');
    $('[data-act=yaml-revert]', section).hidden = !dirty;
  }

  function renderYaml(section, problems) {
    const textarea = $('.ytext', section);
    const text = textarea.value;
    const lines = text.split('\n');
    const marks = {};
    problems.forEach((p) => { if (marks[p.line] !== 'err') marks[p.line] = p.sev; });
    $('.gutter', section).innerHTML = lines.map((_, i) => `<span class="${marks[i + 1] || ''}">${i + 1}</span>`).join('');
    const height = Math.min(Math.max(lines.length, 6), 28);
    const code = $('.code', section);
    code.className = code.className.replace(/\bh\d+\b/g, '').replace(/\s+/g, ' ').trim() + ' h' + height;
    const errs = problems.filter((p) => p.sev === 'err').length;
    const warns = problems.length - errs;
    $('.vlist', section).innerHTML = `<div class="vhead"><span class="vt">validate</span>${problems.length ? `<span class="vcount">${counts(errs, warns)}</span>` : '<span class="vclean">✓ 問題なし</span>'}</div>` +
      problems.map((p) => `<button class="vitem" data-act="yaml-jump" data-line="${p.line}">${chip(p.sev)}<span class="lno">${p.line} 行</span><span class="msg">${esc(p.text)}</span></button>`).join('');
    $('.gutter', section).scrollTop = textarea.scrollTop;
    yamlFoot(section, problems);
    section._problems = problems;
  }

  function scheduleYaml(section) {
    clearTimeout(section._timer);
    section._timer = setTimeout(() => checkYaml(section), 300);
  }

  async function checkYaml(section) {
    const textarea = $('.ytext', section);
    const answer = await post('/profiles/yaml/check', { file: section.dataset.file, text: textarea.value });
    const problems = (answer.json && answer.json.problems) || [];
    if (!section.isConnected) return;
    renderYaml(section, problems);
  }

  function initYaml() {
    $$('.yed').forEach((section) => {
      const textarea = $('.ytext', section);
      section._base = textarea.value;
      section._problems = [];
      renderYaml(section, []);
      checkYaml(section);
    });
  }

  function insertTab(textarea) {
    const start = textarea.selectionStart;
    textarea.setRangeText('  ', start, textarea.selectionEnd, 'end');
    textarea.dispatchEvent(new Event('input', { bubbles: true }));
  }

  // ---- the page's presses ----------------------------------------------

  function panelOf(target) { return target.closest('.panel'); }

  view.addEventListener('click', (event) => {
    const button = event.target.closest('[data-act]');
    if (!button) return;
    const act = button.dataset.act;
    const panel = panelOf(button);
    const row = button.closest('.row');
    const team = button.closest('.team');
    const section = button.closest('.yed');

    if (act === 'goto') return; // the link itself moves the page
    event.preventDefault();

    if (act === 'toggle' && row && button.classList.contains('sum')) {
      const open = !row.classList.contains('open');
      row.classList.toggle('open', open);
      const detail = $('.detail', row);
      if (detail) detail.hidden = !open;
      button.setAttribute('aria-expanded', String(open));
      if (open) {
        $$('.panel', row).forEach(initPanel);
        if (detail) detail.scrollIntoView({ block: 'nearest' });
      }
      button.focus({ preventScroll: true });
      updateProfilesURL();
      return;
    }

    if (act === 'kind-filter') {
      const chip = button.closest('.fchip');
      const on = !chip.classList.contains('on');
      $$('.fchip').forEach((el) => {
        el.classList.toggle('on', el === chip && on);
        el.setAttribute('aria-pressed', String(el === chip && on));
      });
      applyFilter();
      updateProfilesURL();
      return;
    }

    if (act === 'new') {
      clearFilter();
      const fresh = newRow('project');
      const rows = $('#rows');
      const first = $('.row', rows);
      if (first) rows.insertBefore(fresh, first); else rows.appendChild(fresh);
      $$('.panel', fresh).forEach(initPanel);
      const name = $('[data-f=name]', fresh);
      if (name) name.focus();
      return;
    }

    if (act === 'pick-layer' && panel) {
      const file = button.dataset.x;
      if (file !== panel.dataset.file) {
        panel.dataset.file = file;
        $('.path', panel).textContent = pathOf(file);
        $$('.seg.small button', panel).forEach((el) => el.classList.toggle('on', el.dataset.x === file));
        const pill = $('.pill.pending', row);
        if (pill) pill.textContent = labelOf(file);
        mark(panel);
      }
      return;
    }

    if (act === 'set-kind' && panel) {
      panel.dataset.draftKind = button.dataset.k;
      $$('.seg[data-f=kind] > button', panel).forEach((el) => {
        const on = el.dataset.k === button.dataset.k;
        el.classList.toggle('on', on);
        el.setAttribute('aria-pressed', String(on));
      });
      const select = $('[data-f=kind-other]', panel);
      if (select) { select.value = ''; select.classList.remove('on'); }
      const bin = $('.bin', panel);
      if (bin) bin.textContent = button.dataset.k;
      mark(panel);
      scheduleCheck(panel);
      return;
    }

    if (act === 'arg-add' && panel) {
      const args = $('.args', panel);
      const add = $('[data-act=arg-add]', panel);
      const holder = document.createElement('span');
      holder.innerHTML = argHTML(0, '');
      const arg = holder.firstElementChild;
      args.insertBefore(arg, add);
      renumberArgs(panel);
      mark(panel);
      scheduleCheck(panel);
      $('input', arg).focus();
      return;
    }

    if (act === 'arg-del' && panel) {
      const arg = button.closest('.arg');
      if (arg) arg.remove();
      renumberArgs(panel);
      mark(panel);
      scheduleCheck(panel);
      return;
    }

    if (act === 'env-add' && panel) {
      const env = $('.env', panel);
      const add = $('[data-act=env-add]', panel);
      const holder = document.createElement('div');
      holder.innerHTML = envHTML(0, '', '');
      const fresh = holder.firstElementChild;
      env.insertBefore(fresh, add);
      renumberEnv(panel);
      mark(panel);
      scheduleCheck(panel);
      $('[data-f=ek]', fresh).focus();
      return;
    }

    if (act === 'env-del' && panel) {
      const envRow = button.closest('.envrow');
      if (envRow) envRow.remove();
      renumberEnv(panel);
      mark(panel);
      scheduleCheck(panel);
      return;
    }

    if (act === 'save' && panel) { save(panel); return; }

    if (act === 'dup' && panel) {
      const saved = savedOf(panel);
      if (!saved) return;
      const names = $$('#rows .row').map((el) => el.dataset.name);
      let suffix = 2;
      while (names.includes(`${saved.name}-${suffix}`)) suffix++;
      const fresh = newRow(panel.dataset.file);
      fresh.dataset.name = `${saved.name}-${suffix}`;
      const detail = $('.detail', fresh);
      detail.innerHTML = panelHTML(panel.dataset.file, {
        name: `${saved.name}-${suffix}`, kind: saved.kind, args: saved.args.slice(), env: saved.env.map((pair) => ({ key: pair.key, value: pair.value })),
      }, false);
      row.after(fresh);
      initRow(fresh);
      const nm = $('.c-name .nm', fresh);
      if (nm) nm.textContent = `${saved.name}-${suffix}`;
      const name = $('[data-f=name]', fresh);
      if (name) { name.focus(); name.select(); }
      return;
    }

    if (act === 'copy' && panel) {
      const saved = savedOf(panel);
      if (!saved) return;
      const other = otherOf(panel.dataset.file);
      const panels = $('.panels', row);
      const holder = document.createElement('div');
      holder.innerHTML = panelHTML(other, { name: saved.name, kind: saved.kind, args: saved.args.slice(), env: saved.env.map((pair) => ({ key: pair.key, value: pair.value })) }, false);
      const fresh = holder.firstElementChild;
      panels.appendChild(fresh);
      $$('.panel', panels)
        .sort((a, b) => (a.dataset.file === 'project' ? 0 : 1) - (b.dataset.file === 'project' ? 0 : 1))
        .forEach((one) => panels.appendChild(one));
      panels.classList.toggle('two', $$('.panel', panels).length > 1);
      const pills = $('.c-place', row);
      if (pills && !$('.pill.pending', pills)) {
        const pill = document.createElement('span');
        pill.className = 'pill pending';
        pill.textContent = labelOf(other);
        pills.appendChild(pill);
      }
      initPanel(fresh);
      const name = $('[data-f=name]', fresh);
      if (name) { name.focus(); name.select(); }
      return;
    }

    if (act === 'discard' && panel) {
      panel.remove();
      if (row && !$('.panel', row)) { row.remove(); afterRowsChanged(); }
      else if (row) markRowDirty(row);
      return;
    }

    if (act === 'del' && panel) {
      const acts = $('.acts', panel);
      panel._acts = acts.innerHTML;
      const users = Number(panel.dataset.users || 0);
      acts.innerHTML = `<button class="btn danger solid" data-act="del-yes">削除する${users ? `（${users} 人が使っている）` : ''}</button><button class="btn" data-act="del-no">やめる</button>`;
      $('[data-act=del-yes]', acts).focus();
      return;
    }

    if (act === 'del-no' && panel) {
      if (panel._acts !== undefined) $('.acts', panel).innerHTML = panel._acts;
      return;
    }

    if (act === 'del-yes' && panel) {
      const saved = savedOf(panel);
      if (!saved) return;
      post('/profiles/delete', { file: panel.dataset.file, name: saved.name, hash: hashOf(panel.dataset.file) }).then((answer) => {
        if (!answer.ok) { showRefusal(panel, (answer.json && answer.json.text) || '削除できませんでした'); return; }
        updateCounts(answer.headers);
        replaceRow(row, answer.html);
      });
      return;
    }

    if (act === 'kind-hi') {
      const segment = button;
      const pressed = $('.kseg[aria-pressed="true"]');
      const current = pressed ? pressed.dataset.k : null;
      const active = current === segment.dataset.k ? null : segment.dataset.k;
      $$('.kseg').forEach((el) => {
        const dim = active !== null && el.dataset.k !== active;
        el.classList.toggle('dim', dim);
        el.setAttribute('aria-pressed', String(el === segment && active !== null));
      });
      $$('.arow').forEach((el) => {
        const select = $('select', el);
        const dim = active !== null && optionKind(select) !== active;
        el.classList.toggle('dim', dim);
      });
      updateAssignURL(active);
      return;
    }

    if (act === 'g-save' && team) { saveTeam(team); return; }

    if (act === 'yaml-jump' && section) {
      const textarea = $('.ytext', section);
      const line = Number(button.dataset.line);
      const lines = textarea.value.split('\n');
      const start = lines.slice(0, line - 1).reduce((n, s) => n + s.length + 1, 0);
      textarea.focus({ preventScroll: true });
      textarea.setSelectionRange(start, start + (lines[line - 1] || '').length);
      textarea.scrollTop = Math.max(0, (line - 4) * 20);
      textarea.scrollLeft = 0;
      return;
    }

    if (act === 'yaml-revert' && section) {
      const textarea = $('.ytext', section);
      textarea.value = section._base;
      renderYaml(section, section._problems || []);
      checkYaml(section);
      return;
    }

    if (act === 'yaml-save' && section) {
      saveYaml(section);
      return;
    }
  });

  function hashOf(file) {
    return file === 'global' ? view.dataset.hashShared : view.dataset.hashProject;
  }

  async function save(panel) {
    const saved = savedOf(panel);
    const draft = panelValues(panel);
    const hint = $('.rename input', panel);
    const answer = await post('/profiles/save', {
      file: panel.dataset.file,
      original: saved ? saved.name : '',
      profile: JSON.stringify(draft),
      renameUsers: hint && hint.checked ? '1' : '0',
      hash: hashOf(panel.dataset.file),
      open: $$('#rows .row.open').map((row) => row.dataset.name).filter(Boolean).join(','),
    });
    if (!answer.ok) { showRefusal(panel, (answer.json && answer.json.text) || '保存できませんでした'); return; }
    updateCounts(answer.headers);
    replaceRows(answer.html);
  }

  async function saveYaml(section) {
    const textarea = $('.ytext', section);
    const answer = await post('/profiles/yaml/save', { file: section.dataset.file, text: textarea.value, hash: hashOf(section.dataset.file) });
    if (!answer.ok) { showRefusal(section, (answer.json && answer.json.text) || '保存できませんでした'); return; }
    updateCounts(answer.headers);
    const holder = document.createElement('div');
    holder.innerHTML = answer.html.trim();
    const yeds = $('.yeds');
    if (yeds) {
      yeds.innerHTML = holder.innerHTML;
      initYaml();
    }
  }

  view.addEventListener('input', (event) => {
    const target = event.target;
    if (target.id === 'q') { applyFilter(); updateProfilesURL(); return; }

    if (target.classList.contains('ytext')) {
      const section = target.closest('.yed');
      section._dirty = target.value !== section._base;
      yamlFoot(section, section._problems || []);
      scheduleYaml(section);
      return;
    }

    const panel = target.closest('.panel');
    if (!panel || !target.dataset.f) return;
    const field = target.dataset.f;
    if (field === 'name') {
      const name = target.value;
      const row = panel.closest('.row');
      if (row) {
        row.dataset.name = savedOf(panel) ? row.dataset.name : name;
        const nm = $('.c-name .nm', row);
        if (nm && !savedOf(panel)) nm.innerHTML = name ? esc(name) : '<span class="none">—</span>';
      }
    }
    if (field === 'arg') {
      target.size = Math.max(target.value.length, 3);
      target.closest('.arg').classList.toggle('flag', target.value.startsWith('-'));
    }
    if (field === 'ev') autosize(target);
    mark(panel);
    scheduleCheck(panel);
  });

  view.addEventListener('change', (event) => {
    const target = event.target;
    if (target.dataset.f === 'kind-other') {
      const panel = target.closest('.panel');
      if (panel && target.value) {
        panel.dataset.draftKind = target.value;
        $$('.seg[data-f=kind] > button', panel).forEach((el) => { el.classList.remove('on'); el.setAttribute('aria-pressed', 'false'); });
        target.classList.add('on');
        const bin = $('.bin', panel);
        if (bin) bin.textContent = target.value;
        mark(panel);
        scheduleCheck(panel);
      }
      return;
    }
    if (target.matches('.arow select')) assignChange(target);
  });

  view.addEventListener('keydown', (event) => {
    const target = event.target;
    if (target.classList.contains('sum') && (event.key === 'Enter' || event.key === ' ')) {
      event.preventDefault();
      target.click();
      return;
    }
    if (target.classList.contains('ytext') && event.key === 'Tab' && !event.shiftKey) {
      event.preventDefault();
      insertTab(target);
    }
  });

  // A dragged argument's grip moves the whole chip; the chips' order is the
  // order the save writes.
  view.addEventListener('pointerdown', (event) => {
    const grip = event.target.closest('.grip');
    if (grip) grip.closest('.arg').setAttribute('draggable', 'true');
  });
  view.addEventListener('dragstart', (event) => {
    const arg = event.target.closest('.arg');
    if (arg) arg.classList.add('dragging');
  });
  view.addEventListener('dragover', (event) => {
    if (!event.target.closest('.arg')) return;
    event.preventDefault();
  });
  view.addEventListener('drop', (event) => {
    const target = event.target.closest('.arg');
    const source = $('.dragging');
    if (!target || !source || target === source) return;
    event.preventDefault();
    const rect = target.getBoundingClientRect();
    if (event.clientX < rect.left + rect.width / 2) target.before(source); else target.after(source);
    source.classList.remove('dragging');
    source.removeAttribute('draggable');
    const panel = source.closest('.panel');
    renumberArgs(panel);
    mark(panel);
    scheduleCheck(panel);
  });
  view.addEventListener('dragend', (event) => {
    const arg = event.target.closest('.arg');
    if (arg) { arg.classList.remove('dragging'); arg.removeAttribute('draggable'); }
  });

  view.addEventListener('scroll', (event) => {
    if (event.target.classList && event.target.classList.contains('ytext')) {
      const section = event.target.closest('.yed');
      if (section) $('.gutter', section).scrollTop = event.target.scrollTop;
    }
  }, true);

  if (tab === 'profiles') initProfiles();
  else if (tab === 'assign') initAssign();
  else initYaml();
})();
