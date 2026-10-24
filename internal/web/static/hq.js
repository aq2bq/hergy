// hq.js — 本部の帯と欄（n-7f3c、モデルは d-73fc、画面は d-8a45）。
//
// この 1 本が持つもの:
//   - 帯の「プロジェクト N」と、その欄（並べる・外す・候補・パスで並べる）
//   - 帯の「探す」（/ キー）。並べたプロジェクトとグローバルのチーム・
//     ワークスペース・メンバー・ロール・規則・プロファイルを一度に探す
//   - チーム定義の一覧の「動き」の欄（起動…・動いている・止める）と、
//     起動・起動している・止めるの欄
//   - 動いているチームの読み。一覧と活動は、この読みから同じ状態を出す
//
// データはすべて本部の API（/api/hq/…）から読む。文はこの画面が持ち、状態の
// 名（answer、blocked、…）はサーバのものをそのまま使う。
(() => {
  'use strict';

  const API = window.hergyAPI;
  const $ = (sel, root = document) => root.querySelector(sel);
  const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));
  const esc = (s) => String(s == null ? '' : s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  const enc = (s) => encodeURIComponent(String(s == null ? '' : s));
  const el = (html) => { const t = document.createElement('template'); t.innerHTML = html.trim(); return t.content.firstElementChild; };
  const h = (html) => el(html);
  const POLL = 10000;

  // ---- 状態 -------------------------------------------------------------
  // groups は、プロジェクトごとの組の配列（API の groups と同じ形）。組は
  // 最後にグローバルが 1 つ。
  const st = {
    projects: null,
    running: { herdr: { ok: true, text: '' }, teams: [] },
    workspaces: [],
    groups: { teams: [], roles: [], profiles: [] },
    loaded: false,
  };

  const groupKey = (g) => (g && g.global ? 'global' : (g.project ? g.project.path : ''));
  const groupLabel = (g) => (g && g.global ? 'グローバル' : (g.project ? g.project.name : ''));
  const groupDisplay = (g) => (g && g.global ? g.display : (g.project ? g.project.display : ''));
  // projectOf is the key a row of one group carries in the URL and in the API:
  // the project's path, or "global".
  const projectOfGroup = (g) => (g && g.global ? 'global' : (g.project ? g.project.path : ''));

  // token joins a row's name and its group for the address: a name alone is the
  // one the screen was opened at (the old way), and a name with a group says
  // which project's row it is (spec 4 章).
  const token = (name, group) => (group ? name + '|' + group : name);
  function parseOpen(value) {
    return String(value || '').split(',').filter(Boolean).map((one) => {
      const i = one.lastIndexOf('|');
      if (i < 0) return { name: one, group: '' };
      return { name: one.slice(0, i), group: one.slice(i + 1) };
    });
  }

  const projectName = (path) => {
    if (!path) return '';
    const found = ((st.projects && st.projects.projects) || []).find((p) => p.path === path);
    if (found) return found.name;
    if (path === 'global') return 'グローバル';
    return String(path).split('/').filter(Boolean).pop() || path;
  };

  // ---- 読み -------------------------------------------------------------
  async function readJSON(url) {
    const res = await API.call('GET', url);
    if (!res.ok || !res.json) return null;
    return res.json;
  }

  function groupOf(kind) {
    return st.groups[kind] || [];
  }
  const itemGroup = (kind, name, group) => groupOf(kind).find((g) => groupKey(g) === group) || null;

  // runningFor is the running teams a team definition has: its own, of its place
  // and its project (spec 5 章).
  function runningFor(definition, place, project) {
    return (st.running.teams || []).filter((t) => t.definition === definition
      && (t.place || 'project') === (place || 'project')
      && (place === 'global' || !project || t.project === project));
  }
  const runningTeam = (name) => (st.running.teams || []).find((t) => t.team === name) || null;

  // lastWorkspaceOf is where a team definition ran last, as the launch record
  // wrote it (the workspace whose last_team is this definition). Empty when the
  // record has none.
  function lastWorkspaceOf(definition) {
    const ws = (st.workspaces || []).find((w) => w.last_team === definition);
    return ws ? ws.label : '';
  }

  // definitionOf is one team definition's document, from the groups the search
  // reads: the activity's figure uses its turns (spec 6 章).
  function definitionOf(project, definition) {
    const groups = groupOf('teams');
    for (const g of groups) {
      if (project && projectOfGroup(g) !== project && !g.global) continue;
      const item = (g.list ? g.list.items : []).find((x) => x.name === definition);
      if (!item) continue;
      const places = item.places || {};
      const eff = item.effective || (places.project ? 'project' : 'global');
      return (places[eff] || {}).doc || null;
    }
    return null;
  }

  // globalDefinition is one global team definition's own document, for the start
  // sheet of a global definition (its members do not depend on the project).
  function globalDefinition(definition) {
    const g = groupOf('teams').find((x) => x.global);
    if (!g) return null;
    const item = (g.list ? g.list.items : []).find((x) => x.name === definition);
    if (!item) return null;
    const places = item.places || {};
    return (places.global || places[item.effective] || {}).doc || null;
  }

  // definitionDoc is the definition's own reading for a start sheet: the
  // project's own for a project's definition, the global one otherwise.
  function definitionDoc(definition, project) {
    if (project) {
      const own = definitionOf(project, definition);
      if (own) return own;
    }
    return globalDefinition(definition) || definitionOf('', definition);
  }

  // ---- 帯 ---------------------------------------------------------------
  // bandActivity keeps the band's two numbers the reading's own: 活動 N（動いて
  // いるチーム）と 待ち N（人間を待っているメンバー）(spec 2 章, spec-activity 2.1).
  function bandActivity() {
    const nav = window.hergyNav;
    if (!nav || !nav.setActivity) return;
    const teams = st.running.teams || [];
    const waiting = teams.reduce((n, t) => n + (t.waiting || []).length, 0);
    nav.setActivity(teams.length, waiting, !!st.running.herdr.ok && teams.length > 0);
  }

  // bandCounts writes the projects reading's totals into the band, so the four
  // numbers are the listed projects' and the global layer's (spec 2 章). The
  // server writes them too; this keeps them right after a project is listed or
  // removed without a reload.
  function bandCounts() {
    const p = st.projects;
    if (!p || !window.hergyNav || !window.hergyNav.setCount) return;
    if (p.mode !== 'listed') return;
    const totals = { teams: 0, roles: 0, profiles: 0 };
    ((p.projects || []).filter((x) => x.listed)).forEach((x) => {
      totals.teams += x.definitions || 0;
      totals.roles += x.roles || 0;
      totals.profiles += x.profiles || 0;
    });
    totals.teams += (p.global && p.global.definitions) || 0;
    totals.roles += (p.global && p.global.roles) || 0;
    totals.profiles += (p.global && p.global.profiles) || 0;
    window.hergyNav.setCount('teams', totals.teams);
    window.hergyNav.setCount('roles', totals.roles);
    window.hergyNav.setCount('profiles', totals.profiles);
  }

  // ---- 読む・知らせる ---------------------------------------------------
  // readCore is what every screen needs at once: the projects, the running teams
  // and the workspaces. The groups a search reads are read when the search is
  // opened (readGroups), so a page that never searches does not make every
  // project's catalog be read.
  async function readCore() {
    const [projects, running, workspaces] = await Promise.all([
      readJSON('/api/hq/projects'),
      readJSON('/api/hq/running'),
      readJSON('/api/hq/workspaces'),
    ]);
    if (projects) st.projects = projects;
    if (running) st.running = running;
    if (workspaces) st.workspaces = workspaces.workspaces || [];
    st.loaded = true;
    bandActivity();
    bandCounts();
    document.dispatchEvent(new CustomEvent('hergy:hq', { detail: st }));
  }

  // readGroups reads the lists a search looks through: the team definitions,
  // roles and profiles of every listed project and the global layer.
  async function readGroups() {
    if (st.groupsLoaded) return;
    const [teams, roles, profiles] = await Promise.all([
      readJSON('/api/hq/teams'),
      readJSON('/api/hq/roles'),
      readJSON('/api/hq/profiles'),
    ]);
    st.groups.teams = (teams && teams.groups) || [];
    st.groups.roles = (roles && roles.groups) || [];
    st.groups.profiles = (profiles && profiles.groups) || [];
    st.groupsLoaded = true;
    index = buildIndex();
    document.dispatchEvent(new CustomEvent('hergy:hq', { detail: st }));
  }

  // readAll is both readings, in order: a start or a stop asks for the groups
  // too, because a project's row may have moved with it.
  async function readAll() {
    await readCore();
    await readGroups();
  }

  // readRunning is the ten-second reading: the teams and the workspaces, which
  // move without a reload. The groups and the projects stay as they were.
  async function readRunning() {
    const [running, workspaces] = await Promise.all([readJSON('/api/hq/running'), readJSON('/api/hq/workspaces')]);
    if (running) st.running = mergeRunning(running);
    if (workspaces) st.workspaces = workspaces.workspaces || [];
    bandActivity();
    document.dispatchEvent(new CustomEvent('hergy:hq', { detail: st }));
  }

  // sameTeam is the team a row is about: its name and its project. Two readings
  // of one team are one row, whether one of them is the provisional line a start
  // adds before herdr has a tab (r-5b96 audit S1).
  const sameTeam = (t) => t.team + '\u0000' + (t.project || '');
  function mergeRunning(running) {
    const real = (running.teams || []).slice();
    const seen = new Set(real.map(sameTeam));
    // A provisional row stands only for a team the real reading has not found
    // yet: once it is found, the provisional one goes at once.
    const provisional = (st.running.teams || []).filter((t) => t.provisional && !seen.has(sameTeam(t)));
    return Object.assign({}, running, { teams: real.concat(provisional) });
  }

  // noteStarting tells the list a team is starting now, before herdr has a tab
  // for it: the row says 起動の途中 at once, and the next reading replaces this
  // with the team as herdr sees it (r-5b96 audit R5).
  function noteStarting(launch) {
    const teams = (st.running.teams || []).filter((t) => sameTeam(t) !== sameTeam(launch));
    teams.push({
      team: launch.team, definition: launch.definition, place: launch.place,
      project: launch.project, project_name: launch.project_name,
      workspace: launch.workspace, tab: launch.tab,
      starting: true, launch: launch.id, members: [], workers: [], waiting: [],
      provisional: true,
    });
    st.running.teams = teams;
    bandActivity();
    document.dispatchEvent(new CustomEvent('hergy:hq', { detail: st }));
    // The tab is made within the first second; a few readings bring the team in
    // without waiting for the ten-second one, whether or not the sheet is open.
    [600, 1500, 3000, 6000].forEach((ms) => setTimeout(readRunning, ms));
  }

  // ---- 欄（プロジェクト・起動・停止） ------------------------------------
  let sheet = null;
  function closeSheet() { if (sheet) { sheet.remove(); sheet = null; } }
  function openSheet(html, side) {
    closeSheet();
    sheet = h(`<div class="hq-sheet ${side || 'right'}" role="dialog" aria-modal="false" tabindex="-1">${html}</div>`);
    document.body.appendChild(sheet);
    sheet.focus();
    sheet.addEventListener('click', (e) => {
      const x = e.target.closest('[data-hqx="close"]');
      if (x) { e.preventDefault(); closeSheet(); }
    });
    return sheet;
  }
  document.addEventListener('keydown', (e) => { if (e.key === 'Escape' && sheet) closeSheet(); });
  document.addEventListener('click', (e) => {
    if (sheet && !e.target.closest('.hq-sheet') && !e.target.closest('[data-hq]')) closeSheet();
  });

  const errBox = (s, text) => { const e = s.querySelector('.hq-err[data-role="error"]'); if (e) { e.textContent = text; e.hidden = false; } };
  const clearErr = (s) => { const e = s.querySelector('.hq-err[data-role="error"]'); if (e) { e.textContent = ''; e.hidden = true; } };

  // projectsSheet is the band's project switcher (spec 2 章): the listed ones, the
  // launch record's candidates, and a path to list one that never ran.
  function projectsSheet() {
    const p = st.projects;
    if (!p) { openSheet('<div class="hq-h"><b>本部に並べるプロジェクト</b><button type="button" class="hq-x" data-hqx="close" aria-label="閉じる">×</button></div><p class="hq-sub">読み込んでいます…</p>', 'left'); return; }
    const listed = (p.projects || []).filter((x) => x.listed);
    const candidates = (p.projects || []).filter((x) => !x.listed && x.source === 'launch');
    const current = p.current || {};
    const currentOnly = p.mode !== 'listed';
    const row = (project) => `<div class="hq-li"><span class="hq-pn">${esc(project.name)}</span><span class="hq-pp">${esc(project.display)}</span>
      <span class="hq-pc">チーム定義 ${project.definitions || 0} · 動いているチーム ${project.running || 0}</span>
      <span class="hq-act"><button type="button" class="hq-btn sm" data-hq="unlist" data-path="${esc(project.path)}">外す</button></span></div>`;
    const cand = (project) => `<div class="hq-li cand"><span class="hq-pn">${esc(project.name)}</span><span class="hq-pp">${esc(project.display)}</span>
      <span class="hq-pc">最後の起動 ${esc(mmdd(project.last_launch))} · チーム定義 ${project.definitions || 0}</span>
      <span class="hq-act"><button type="button" class="hq-btn sm" data-hq="list" data-path="${esc(project.path)}">並べる</button></span></div>`;
    // Without projects:, the project serve was started in is not listed yet. It
    // stands in the sheet with 並べる, and listing anything lists it first, so the
    // teams a person is looking at do not fall out of the screen (r-5b96 audit 7).
    const currentRow = currentOnly
      ? `<div class="hq-li"><span class="hq-pn">${esc(current.name)}</span><span class="hq-pp">${esc(current.display)}</span><span class="hq-pc">チーム定義 ${current.definitions || 0} · 動いているチーム ${current.running || 0}</span><span class="hq-act"><button type="button" class="hq-btn sm" data-hq="list" data-path="${esc(current.path)}">並べる</button></span></div>`
      : '';
    const unlisted = p.mode === 'listed' && !current.listed
      ? `<div class="hq-li cand"><span class="hq-pn">${esc(current.name)}</span><span class="hq-pp">${esc(current.display)}</span><span class="hq-pc">この serve を起こしたプロジェクト（並んでいない）</span><span class="hq-act"><button type="button" class="hq-btn sm" data-hq="list" data-path="${esc(current.path)}">並べる</button></span></div>`
      : '';
    const currentNote = currentOnly ? `並べると <code>config.yaml</code> に <code>projects:</code> を作ります。この serve を起こした ${esc(current.name)} も一緒に並べます。` : '';
    const s = openSheet(
      `<div class="hq-h"><b>本部に並べるプロジェクト</b><button type="button" class="hq-x" data-hqx="close" aria-label="閉じる">×</button></div>
      <p class="hq-sub">並べたプロジェクトのチーム定義・ロール・プロファイルと、動いているチームが、どの画面にも出ます。どこで hergy serve を起こしても、同じ並びです。</p>
      ${currentOnly
        ? `<div class="hq-sec"><span class="hq-lbl">この serve を起こしたプロジェクト（まだ並べていない）</span>${currentRow}</div>`
        : `<div class="hq-sec"><span class="hq-lbl">並べている ${listed.length}</span>${listed.map(row).join('') || '<p class="hq-sub">まだありません</p>'}</div>`}
      <div class="hq-sec"><span class="hq-lbl">起動の記録にあって、並べていないプロジェクト</span>${unlisted}${candidates.filter((x) => x.path !== current.path).map(cand).join('') || (unlisted ? '' : '<p class="hq-sub">ありません</p>')}</div>
      <p class="hq-note">並びは <code>~/.config/hergy/config.yaml</code> の <code>projects:</code> に書きます。外しても、そのプロジェクトのファイルは変わりません。${currentNote ? ` ${currentNote}` : ''}</p>
      <div class="hq-sec hq-foot"><span class="hq-lbl">パスで並べる（まだチームを起こしたことのないプロジェクト）</span><div class="hq-acts"><input id="hq-path" class="hq-sel" placeholder="~/src/…" autocomplete="off" aria-label="プロジェクトのパス"><button type="button" class="hq-btn sm" data-hq="add">並べる</button></div><p class="hq-err" data-role="error" hidden></p></div>`, 'left');
    return s;
  }

  const mmdd = (iso) => {
    if (!iso) return '';
    const d = new Date(iso);
    if (isNaN(d.getTime())) return '';
    const two = (n) => String(n).padStart(2, '0');
    return `${two(d.getMonth() + 1)}/${two(d.getDate())}`;
  };

  async function postProject(path) {
    const res = await API.call('POST', '/api/hq/projects', { path });
    if (!res.ok) { if (sheet) errBox(sheet, API.problemText(res, '並べられませんでした')); return false; }
    return true;
  }
  // listProject lists one project. Without projects:, the project serve was
  // started in is listed first, so listing another one does not drop the teams a
  // person is looking at (r-5b96 audit 7).
  async function listProject(path) {
    const p = st.projects;
    if (p && p.mode !== 'listed' && p.current && p.current.path && p.current.path !== path) {
      if (!await postProject(p.current.path)) return false;
    }
    return postProject(path);
  }
  async function unlistProject(path) {
    const res = await API.call('DELETE', '/api/hq/projects?path=' + enc(path));
    if (!res.ok) { if (sheet) errBox(sheet, API.problemText(res, '外せませんでした')); return false; }
    return true;
  }

  // ---- 起動の欄 ---------------------------------------------------------
  // startSheet is spec 5 章: the definition and its project, the members, the
  // name when one is taken, and the workspaces in the spec's order. A global
  // definition chooses its project first; its members and workspaces follow the
  // pick (r-5b96 audit 8).
  async function startSheet(definition, place, project) {
    // The definition's own members and the clash are read from the groups, which
    // a page reads when a search or a sheet asks for them.
    await readGroups();
    const q = new URLSearchParams({ definition, place: place || 'project' });
    if (project) q.set('project', project);
    const options = await readJSON('/api/hq/launch/options?' + q.toString());
    if (!options) { openSheet('<div class="hq-h"><b>起動する</b><code>' + esc(definition) + '</code><button type="button" class="hq-x" data-hqx="close" aria-label="閉じる">×</button></div><p class="hq-err">起動の選択肢を読めませんでした</p>'); return; }
    const isGlobal = (place || 'project') === 'global';
    let picked = options.last_workspace ? options.last_workspace.id : '';
    let chosenProject = options.project ? options.project.path : '';

    // membersOf is the people the definition starts: the answer's own list, or,
    // before a project is chosen for a global definition, the definition's own
    // members (they do not depend on the project).
    const membersOf = () => {
      if ((options.members || []).length) return options.members;
      const doc = definitionDoc(definition, chosenProject);
      return ((doc && doc.members) || []).map((m) => ({ name: m.name, role: m.role, profile: m.profile }));
    };
    const membersHTML = () => {
      const members = membersOf();
      return members.length ? `${members.length} 人: ${members.map((m) => `<code>${esc(m.name)}</code>`).join(' · ')}` : '—';
    };
    const projRow = (x) => (x.available
      ? `<option value="${esc(x.path)}"${x.path === chosenProject ? ' selected' : ''}>${esc(x.name)}</option>`
      : `<option value="" disabled>${esc(x.name)}（${esc(x.reason)}）</option>`);
    const projectsHTML = () => `<select id="hq-proj" class="hq-sel"><option value="">選んでください</option>${(options.projects || []).map(projRow).join('')}</select>`;
    const wsHTML = () => {
      if (isGlobal && !chosenProject) return '<p class="hq-sub">プロジェクトを選ぶと、ワークスペースが出ます</p>';
      return (options.workspaces || []).map((w) => {
        const why = w.why === 'last' ? `前に ${definition} が動いたワークスペース` : w.why === 'project' ? 'プロジェクトの名前で始まるワークスペース' : w.why === 'busy' ? `${w.team} が動いている（1 つのワークスペースにチームは 1 つ）` : 'hergy のチームはいない';
        if (!w.selectable) return `<label class="hq-wo no"><input type="radio" name="hq-ws" disabled><span class="hq-wn">${esc(w.label)}</span><span class="hq-wd">${esc(why)}</span></label>`;
        return `<label class="hq-wo"><input type="radio" name="hq-ws" value="${esc(w.id)}"${w.id === picked ? ' checked' : ''}><span class="hq-wn">${esc(w.label)}</span><span class="hq-wd">${esc(why)}</span></label>`;
      }).join('') + `<label class="hq-wo"><input type="radio" name="hq-ws" value="__new"${picked === '__new' ? ' checked' : ''}><span class="hq-wn">新しく作る</span><span class="hq-wd"><input id="hq-wsname" class="hq-in" value="${esc(options.new_workspace_name || definition)}" aria-label="新しいワークスペースの名前"></span></label>`;
    };
    // clashHTML is the name field: it appears and goes with the project picked,
    // because a team of the same name may be running in one project and not
    // another (r-5b96 audit 8).
    const clashHTML = () => {
      const clash = options.clash || [];
      if (!clash.length) return '';
      return `<dt>チームの名前</dt><dd><input id="hq-as" class="hq-sel" value="${esc(options.suggested_name || '')}"><p class="hq-sub">${esc(definition)} という名前のチームが ${esc(clash.map((c) => projectName(c.project)).join(' · '))} で動いています。この名前で起こします（hergy up --as と同じ）。</p></dd>`;
    };
    // sameHTML is the command the start stands for: when the name is taken, the
    // command carries --as and the name the field holds (r-5b96 audit R1).
    const sameHTML = (asName) => {
      const clash = (options.clash || []).length > 0;
      const as = clash && asName ? ` --as ${asName}` : '';
      return chosenProject
        ? `<code>${esc(projectName(chosenProject))}</code> で <code>hergy up ${esc(definition)}${esc(as)}</code> を打つのと同じ`
        : `選んだプロジェクトで <code>hergy up ${esc(definition)}${esc(as)}</code> を打つのと同じ`;
    };
    const s = openSheet(
      `<div class="hq-h"><b>起動する</b><code>${esc(definition)}</code><button type="button" class="hq-x" data-hqx="close" aria-label="閉じる">×</button></div>
      <dl class="hq-kv">
        <dt>チーム定義</dt><dd><code>${esc(definition)}</code>（${isGlobal ? 'グローバル' : esc(projectName(chosenProject))}）</dd>
        <dt>プロジェクト</dt><dd>${isGlobal ? `${projectsHTML()}<div class="hq-sub">グローバルのチーム定義は、どのプロジェクトで起こすかを選びます</div>` : `<code>${esc(projectName(chosenProject))}</code>`}</dd>
        <dt>メンバー</dt><dd data-role="members">${membersHTML()}</dd>
      </dl>
      <dl class="hq-kv" data-role="clash">${clashHTML()}</dl>
      <div class="hq-sec"><span class="hq-lbl">ワークスペース</span><div class="hq-ws" data-role="wslist">${wsHTML()}</div></div>
      <p class="hq-err" data-role="error" hidden>ワークスペースを選んでください</p>
      <div class="hq-acts"><button type="button" class="hq-btn primary" data-hq="go" data-def="${esc(definition)}" data-place="${esc(place || 'project')}">起動する</button><button type="button" class="hq-btn" data-hqx="close">やめる</button>
      <span class="hq-sub" data-role="same">${sameHTML(options.suggested_name)}</span></div>`);

    const projSel = s.querySelector('#hq-proj');
    if (projSel) {
      projSel.addEventListener('change', async () => {
        chosenProject = projSel.value;
        picked = '';
        // The refusal for a missing project is about the pick before this one:
        // it goes as soon as one is picked (r-5b96 audit R2).
        clearErr(s);
        if (!chosenProject) {
          options.members = [];
          options.clash = [];
          options.suggested_name = '';
          options.new_workspace_name = definition;
          options.workspaces = [];
        } else {
          const q2 = new URLSearchParams({ definition, place: 'global', project: chosenProject });
          const next = await readJSON('/api/hq/launch/options?' + q2.toString());
          if (!next) return;
          options.members = next.members || [];
          options.clash = next.clash || [];
          options.suggested_name = next.suggested_name || '';
          options.new_workspace_name = next.new_workspace_name || definition;
          options.workspaces = next.workspaces || [];
          picked = next.last_workspace ? next.last_workspace.id : '';
        }
        s.querySelector('[data-role="members"]').innerHTML = membersHTML();
        s.querySelector('[data-role="clash"]').innerHTML = clashHTML();
        s.querySelector('[data-role="wslist"]').innerHTML = wsHTML();
        s.querySelector('[data-role="same"]').innerHTML = sameHTML(options.suggested_name);
        const go = s.querySelector('[data-hq="go"]');
        if (go) go.dataset.project = chosenProject;
      });
    }
    s.querySelector('[data-hq="go"]').dataset.project = chosenProject;
    // The name field's own typing follows the command below it (r-5b96 audit R1).
    s.addEventListener('input', (e) => {
      if (e.target && e.target.id === 'hq-as') {
        s.querySelector('[data-role="same"]').innerHTML = sameHTML(e.target.value.trim());
      }
    });
    return s;
  }

  // runStart shows one start: its steps, the startup question, and its end. The
  // start goes on on the server after the answer (api.md 4 章): closing this
  // sheet does not stop it.
  function runStart(launch) {
    const s = openSheet(
      `<div class="hq-h"><b data-role="title">起動しています</b><code>${esc(launch.team)}</code><button type="button" class="hq-x" data-hqx="close" aria-label="閉じる">×</button></div>
      <p class="hq-sub" data-role="where">${esc(launch.project_name)} · ワークスペース <code>${esc(launch.workspace.label || launch.workspace.id)}</code>。この欄を閉じても、起動は続きます。</p>
      <ul class="hq-steps" data-role="steps"></ul>
      <div data-role="waitbox"></div>
      <div class="hq-acts" data-role="endacts"><button type="button" class="hq-btn danger" data-hq="abort" data-launch="${esc(launch.id)}">起動をやめる</button></div>`);
    drawLaunch(s, launch);
    watchLaunch(launch.id, s);
    return s;
  }

  function drawLaunch(s, launch) {
    // The title says what became of the start, the same as its last line
    // (r-5b96 audit R4).
    const over = launch.phase === 'done' || launch.phase === 'failed' || launch.phase === 'cancelled';
    const title = s.querySelector('[data-role="title"]');
    if (title) {
      title.textContent = launch.phase === 'done' ? '起動しました'
        : launch.phase === 'failed' ? '起動できませんでした'
          : launch.phase === 'cancelled' ? '起動をやめました' : '起動しています';
    }
    // A start that is over does not say the closing leaves it running: the line
    // under the title says only where it is (r-5b96 audit S3).
    const where = s.querySelector('[data-role="where"]');
    if (where && over) {
      where.textContent = `${launch.project_name} · ワークスペース ${launch.workspace.label || launch.workspace.id}`;
    }
    const steps = s.querySelector('[data-role="steps"]');
    if (steps) {
      const all = launch.steps || [];
      // A stage is the step in progress, not a finished one: the last stage
      // stands with … only while the start is starting, and is dropped when the
      // next line comes or the start waits or ends (r-5b96 audit 9).
      const lastStage = all.map((x, i) => (x.kind === 'stage' ? i : -1)).filter((i) => i >= 0).pop();
      const shown = all.filter((x, i) => x.kind !== 'stage' || (launch.phase === 'starting' && i === lastStage));
      steps.innerHTML = shown.map((step) => {
        const now = step.kind === 'stage';
        const mark = now ? 'now' : step.kind === 'done' || step.kind === 'member' ? 'done' : '';
        return `<li class="${mark}"><span>${esc(step.text)}</span></li>`;
      }).join('');
      steps.scrollTop = steps.scrollHeight;
    }
    const waitbox = s.querySelector('[data-role="waitbox"]');
    if (waitbox) {
      if (launch.waiting) {
        const w = launch.waiting;
        waitbox.innerHTML = `<div class="hq-wait"><div class="hq-wh"><span class="hq-on wait">許可待ち</span><b><code>${esc(w.herdr)}</code> が、起動の途中で確認に答えを待っています</b></div>
          <p class="hq-where">ワークスペース ${esc(w.workspace)} · タブ ${esc(w.tab)} · ペイン ${esc(w.pane)}</p>
          <p>エージェントが、このフォルダを信頼するか、などを聞いています。そのペインを開いて答えてください。確認は続けて出ることがあります。</p>
          <p>答え終わると、hergy は <code>${esc(w.herdr)}</code> に初期プロンプトを送って続きを進めます。</p></div>`;
      } else waitbox.replaceChildren();
    }
    const end = s.querySelector('[data-role="endacts"]');
    if (!end) return;
    if (launch.phase === 'done') {
      end.innerHTML = `<p class="hq-ok">起動しました: ${esc(launch.team)}（${esc(launch.project_name)} · ワークスペース ${esc(launch.workspace.label || launch.workspace.id)} · タブ ${esc(launch.tab.label)}）</p>
        <a class="hq-btn primary" href="/activity?open=${enc(token(launch.team, launch.project))}">活動で見る</a><button type="button" class="hq-btn" data-hqx="close">閉じる</button>`;
    } else if (launch.phase === 'failed') {
      end.innerHTML = `<p class="hq-err">${esc(launch.error || '起動できませんでした')}</p><button type="button" class="hq-btn" data-hqx="close">閉じる</button>`;
    } else if (launch.phase === 'cancelled') {
      end.innerHTML = `<p class="hq-sub">起動をやめました。作ったタブとペインは残っています（止めるのはチームを止める欄から）。</p><button type="button" class="hq-btn" data-hqx="close">閉じる</button>`;
    }
  }

  let watching = {};
  function watchLaunch(id, s) {
    if (watching[id]) return;
    watching[id] = setInterval(async () => {
      if (!document.contains(s)) { /* the start goes on: read it once more for the band */ }
      const answer = await readJSON('/api/hq/launches/' + enc(id));
      if (!answer || !answer.launch) { clearInterval(watching[id]); delete watching[id]; return; }
      const launch = answer.launch;
      if (document.contains(s)) drawLaunch(s, launch);
      if (launch.phase === 'done' || launch.phase === 'failed' || launch.phase === 'cancelled') {
        clearInterval(watching[id]); delete watching[id];
        // The row and the group's count move as the title does, rather than on
        // the next ten-second reading (r-5b96 audit S2).
        readRunning();
        setTimeout(readRunning, 800);
      }
    }, 900);
  }

  // stopSheet is spec 5 章's 止める欄: what is closed, and what stays. The same
  // sheet is the activity screen's チームを止める….
  function stopSheet(team) {
    const t = runningTeam(team);
    if (!t) {
      openSheet(`<div class="hq-h"><b>止める</b><code>${esc(team)}</code><button type="button" class="hq-x" data-hqx="close" aria-label="閉じる">×</button></div><p class="hq-sub">このチームは動いていません。読み直してから、もう一度お試しください。</p>`);
      readRunning();
      return;
    }
    const members = (t.members || []).filter((m) => m.live !== false).map((m) => m.name);
    const workers = (t.workers || []).length;
    openSheet(
      `<div class="hq-h"><b>止める</b><code>${esc(team)}</code><button type="button" class="hq-x" data-hqx="close" aria-label="閉じる">×</button></div>
      <dl class="hq-kv"><dt>プロジェクト</dt><dd><code>${esc(t.project_name || projectName(t.project))}</code></dd><dt>ワークスペース</dt><dd><code>${esc(t.workspace.label || t.workspace.id)}</code></dd>
      <dt>閉じるペイン</dt><dd>メンバー ${members.length} 人（${members.map((m) => `<code>${esc(m)}</code>`).join(' · ')}）${workers ? `と、${esc(team)} が追加したワーカー ${workers} 人のタブ` : ''}</dd></dl>
      <p class="hq-sub">エージェントを止めてペインを閉じます。会話は残りません。ワークスペース ${esc(t.workspace.label || t.workspace.id)} は閉じません。${esc(t.project_name || projectName(t.project))} で <code>hergy down ${esc(team)}</code> を打つのと同じです。</p>
      <p class="hq-err" data-role="error" hidden></p>
      <div class="hq-acts"><button type="button" class="hq-btn danger solid" data-hq="stopgo" data-team="${esc(team)}">止める</button><button type="button" class="hq-btn" data-hqx="close">やめる</button></div>`);
  }

  async function stopTeam(team) {
    const res = await API.call('POST', '/api/hq/stop', { team });
    if (!res.ok) { if (sheet) errBox(sheet, API.problemText(res, '止められませんでした')); return; }
    closeSheet();
    // The row goes at once, rather than waiting for the ten-second reading
    // (r-5b96 audit R5).
    await readCore();
  }

  // ---- 横断の検索 --------------------------------------------------------
  // KINDS is the order the result groups stand in (spec 3 章). A rule's group
  // shows six rows and the rest as ほか N 件; every other group shows five.
  const KINDS = [
    ['live', '動いているチーム'], ['def', 'チーム定義'], ['ws', 'ワークスペース'], ['member', 'メンバー'],
    ['role', 'ロール'], ['rule', '規則'], ['profile', 'プロファイル'],
  ];
  const CAP = (kind) => (kind === 'rule' ? 6 : 5);

  // index builds the rows the search reads: names, missions, rules' ids and
  // texts, kinds and args — the fields api.md 3.5 names (spec 3 章).
  function buildIndex() {
    const out = [];
    for (const g of groupOf('teams')) {
      const group = projectOfGroup(g), label = groupLabel(g);
      for (const item of (g.list ? g.list.items : [])) {
        const places = item.places || {};
        const eff = item.effective || (places.project ? 'project' : 'global');
        const doc = (places[eff] || {}).doc || {};
        const href = `/teams?open=${enc(token(item.name, group))}`;
        out.push({ kind: 'def', group, project: label, label: item.name, sub: doc.mission || '', text: `${item.name} ${doc.mission || ''}`, href });
        for (const m of doc.members || []) {
          out.push({ kind: 'member', group, project: label, label: `${item.name} の ${m.name}`, sub: `ロール ${m.role || ''} · プロファイル ${m.profile || ''}`, text: `${m.name} ${item.name}-${m.name} ${m.role || ''} ${m.profile || ''}`, href });
        }
        const agree = (doc.agreement && doc.agreement.doc) || {};
        for (const sec of agree.sections || []) {
          for (const rule of sec.rules || []) {
            out.push({ kind: 'rule', group, project: label, label: rule.id, sub: `${item.name} のルール`, text: `${rule.id} ${rule.text}`, body: rule.text, href });
          }
        }
      }
    }
    for (const g of groupOf('roles')) {
      const group = projectOfGroup(g), label = groupLabel(g);
      for (const item of (g.list ? g.list.items : [])) {
        const places = item.places || {};
        const eff = item.effective || (places.project ? 'project' : 'global');
        const doc = (places[eff] || {}).doc || {};
        const href = `/roles?open=${enc(token(item.name, group))}`;
        out.push({ kind: 'role', group, project: label, label: `${item.name} ${doc.title || ''}`.trim(), sub: '', text: `${item.name} ${doc.title || ''}`, href });
        for (const sec of doc.sections || []) {
          for (const rule of sec.rules || []) {
            out.push({ kind: 'rule', group, project: label, label: rule.id, sub: `ロール ${item.name}`, text: `${rule.id} ${rule.text}`, body: rule.text, href });
          }
        }
      }
    }
    for (const g of groupOf('profiles')) {
      const group = projectOfGroup(g), label = groupLabel(g);
      for (const item of (g.list ? g.list.items : [])) {
        const places = item.places || {};
        const eff = item.effective || (places.project ? 'project' : 'global');
        const doc = (places[eff] || {}).doc || {};
        const sum = item.summary || {};
        const kind = doc.kind || sum.kind || '';
        const args = (doc.args || []).join(' ') || sum.args || '';
        out.push({ kind: 'profile', group, project: label, label: item.name, sub: `${kind} ${args}`.trim(), text: `${item.name} ${kind} ${args}`, href: `/profiles?open=${enc(token(item.name, group))}` });
      }
    }
    return out;
  }

  function liveEntries() {
    const out = [];
    for (const t of st.running.teams || []) {
      const wait = (t.waiting || []).length;
      out.push({
        kind: 'live', group: t.project, project: t.project_name || projectName(t.project),
        label: t.team, sub: `ワークスペース ${t.workspace.label}${wait ? ` · 許可待ち（${t.waiting.join(' · ')}）` : t.starting ? ' · 起動の途中' : ' · 動いている'}`,
        text: `${t.team} ${t.definition} ${t.workspace.label} ${t.project_name || ''}`,
        href: `/activity?open=${enc(token(t.team, t.project))}`,
      });
    }
    for (const w of st.workspaces || []) {
      const last = (st.running.teams || []).find((t) => t.workspace && t.workspace.id === w.id);
      out.push({
        kind: 'ws', group: last ? last.project : '', project: last ? (last.project_name || projectName(last.project)) : '',
        label: w.label,
        sub: last ? `${last.team} が動いている` : `hergy のチームはいない${w.last_team ? `（前は ${w.last_team}）` : ''}`,
        text: `${w.label} ${last ? last.team : ''} ${w.last_team || ''}`,
        href: last ? `/activity?open=${enc(token(last.team, last.project))}` : '',
      });
    }
    return out;
  }

  const mark = (text, q) => {
    const i = String(text).toLowerCase().indexOf(q.toLowerCase());
    if (i < 0) return esc(text);
    return esc(String(text).slice(0, i)) + '<mark>' + esc(String(text).slice(i, i + q.length)) + '</mark>' + esc(String(text).slice(i + q.length));
  };
  const snippet = (text, q) => {
    const s = String(text || '');
    const i = s.toLowerCase().indexOf(q.toLowerCase());
    if (i < 0) return esc(s.slice(0, 60));
    const a = Math.max(0, i - 24), b = Math.min(s.length, i + q.length + 36);
    return (a > 0 ? '…' : '') + mark(s.slice(a, b), q) + (b < s.length ? '…' : '');
  };

  let index = null;
  let shown = [];
  let sel = 0;

  function resultsBox() {
    let box = $('.hq-sr');
    if (!box) { box = h('<div class="hq-sr" role="listbox" aria-label="探した結果"></div>'); document.body.appendChild(box); }
    return box;
  }

  function renderResults(q) {
    const box = $('.hq-sr');
    if (!q) { if (box) box.remove(); shown = []; return; }
    const out = resultsBox();
    if (!index) {
      out.innerHTML = '<p class="hq-sub">読み込んでいます…</p>';
      readGroups().then(() => { const v = ($('#hq-q') || {}).value || ''; if (v.trim()) renderResults(v.trim()); });
      return;
    }
    const low = q.toLowerCase();
    const all = liveEntries().concat(index).filter((e) => e.text.toLowerCase().includes(low));
    if (!all.length) {
      out.innerHTML = `<p class="hq-sub">「${esc(q)}」に合うものはありません。探すのは、並べたプロジェクトとグローバルの、チーム・ワークスペース・メンバー・ロール・規則・プロファイルです。</p>`;
      shown = [];
      return;
    }
    shown = [];
    let html = '';
    for (const [kind, title] of KINDS) {
      const items = all.filter((e) => e.kind === kind);
      if (!items.length) continue;
      html += `<div class="hq-srg"><div class="hq-srh">${title}<span class="n">${items.length}</span></div>`;
      for (const e of items.slice(0, CAP(kind))) {
        const i = shown.length;
        shown.push(e);
        const tag = e.href ? 'a' : 'div';
        html += `<${tag} class="hq-ri${i === sel ? ' sel' : ''}" data-i="${i}"${e.href ? ` href="${esc(e.href)}"` : ''} role="option">` +
          `<span class="hq-rp">${esc(e.project || '—')}</span>` +
          `<span class="hq-rl">${mark(e.label, q)}</span>` +
          `<span class="hq-rs">${e.body ? snippet(e.body, q) + ` <span class="hq-rw">${esc(e.sub)}</span>` : mark(e.sub, q)}</span></${tag}>`;
      }
      if (items.length > CAP(kind)) html += `<div class="hq-srm">ほか ${items.length - CAP(kind)} 件</div>`;
      html += '</div>';
    }
    out.innerHTML = html;
  }

  function moveSel(step) {
    if (!shown.length) return;
    sel = Math.max(0, Math.min(shown.length - 1, sel + step));
    renderResults($('#hq-q').value.trim());
    const cur = $('.hq-ri.sel');
    if (cur) cur.scrollIntoView({ block: 'nearest' });
  }

  // ---- 押す -------------------------------------------------------------
  document.addEventListener('click', async (e) => {
    const t = e.target.closest('[data-hq]');
    if (!t) return;
    const act = t.dataset.hq;
    if (['projects', 'unlist', 'list', 'add', 'start', 'go', 'abort', 'stop', 'stopgo'].indexOf(act) >= 0) e.preventDefault();
    if (act === 'projects') { projectsSheet(); return; }
    if (act === 'unlist') { if (await unlistProject(t.dataset.path)) location.reload(); return; }
    if (act === 'list') { if (await listProject(t.dataset.path)) location.reload(); return; }
    if (act === 'add') {
      const input = $('#hq-path');
      const path = input ? input.value.trim() : '';
      if (!path) { if (sheet) errBox(sheet, 'プロジェクトのディレクトリのパスを書いてください'); return; }
      if (await listProject(path)) location.reload();
      return;
    }
    if (act === 'start') { startSheet(t.dataset.def, t.dataset.place, t.dataset.project); return; }
    if (act === 'go') {
      const s = sheet;
      if (!s) return;
      const place = t.dataset.place || 'project';
      const proj = place === 'global' ? ((s.querySelector('#hq-proj') || {}).value || '') : t.dataset.project;
      if (!proj) { errBox(s, 'どのプロジェクトで起こすかを選んでください'); return; }
      const picked = s.querySelector('input[name="hq-ws"]:checked');
      if (!picked) { errBox(s, 'ワークスペースを選んでください'); return; }
      let workspace;
      if (picked.value === '__new') {
        const name = (s.querySelector('#hq-wsname') || {}).value || '';
        if (!name.trim()) { errBox(s, '新しいワークスペースの名前を書いてください'); return; }
        workspace = { new: name.trim() };
      } else workspace = { id: picked.value };
      const body = { project: proj, definition: t.dataset.def, place, workspace };
      const as = s.querySelector('#hq-as');
      if (as && as.value.trim()) body.name = as.value.trim();
      t.disabled = true;
      const res = await API.call('POST', '/api/hq/launch', body);
      t.disabled = false;
      if (!res.ok) { errBox(s, API.problemText(res, '起動できませんでした')); return; }
      const launch = (res.json || {}).launch;
      // The row says 起動の途中 at once, rather than waiting for the ten-second
      // reading (r-5b96 audit R5).
      if (launch) { noteStarting(launch); runStart(launch); }
      return;
    }
    if (act === 'abort') {
      const res = await API.call('DELETE', '/api/hq/launches/' + enc(t.dataset.launch));
      if (res.ok && res.json && res.json.launch && sheet) drawLaunch(sheet, res.json.launch);
      readRunning();
      return;
    }
    if (act === 'stop') { stopSheet(t.dataset.team); return; }
    if (act === 'stopgo') { t.disabled = true; await stopTeam(t.dataset.team); return; }
  });

  // The search's keys (spec 3 章): "/" from anywhere outside a field, Esc closes,
  // the arrows walk the rows, Enter goes to the chosen one.
  document.addEventListener('keydown', (e) => {
    const box = $('.hq-sr');
    if (e.key === 'Escape' && box) { const q = $('#hq-q'); if (q) { q.value = ''; renderResults(''); } box.remove(); return; }
    if (e.key === '/' && !e.target.closest('input, textarea, select, [contenteditable]')) {
      const q = $('#hq-q');
      if (q) { e.preventDefault(); q.focus(); }
      return;
    }
    if (!box || !e.target.closest('.hq-search')) return;
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') { e.preventDefault(); moveSel(e.key === 'ArrowDown' ? 1 : -1); }
    if (e.key === 'Enter') {
      const pick = shown[sel];
      if (pick && pick.href) location.href = pick.href;
    }
  });

  function mountSearch() {
    const input = $('#hq-q');
    if (!input) return;
    input.addEventListener('input', () => { sel = 0; renderResults(input.value.trim()); });
    input.addEventListener('focus', () => { if (input.value.trim()) renderResults(input.value.trim()); });
  }
  // A test hook, the way the other screens carry one: what the search has on
  // show, so a walk of the screens can tell a key that did not land from one
  // that did.
  window.hergyHQSearch = () => ({ shown: shown.map((x) => x.href), sel: sel, box: !!$('.hq-sr') });
  document.addEventListener('click', (e) => {
    if (!e.target.closest('.hq-sr, .hq-search')) { const box = $('.hq-sr'); if (box) box.remove(); }
  });

  // A click on a search row goes where it says; the anchor's own href is the
  // address (the rows are links), so nothing else is needed.

  // ---- 図（活動）の共通の部品 --------------------------------------------
  // teamSVG draws one team's figure from its definition's turns and the members'
  // states (spec 6 章): the human above, one row of members, workers below the
  // member that added them. The screen hands the rows and the turns; the drawing
  // is here so the tile and the detail cannot draw two shapes.
  function teamSVG(t, turns, W, H) {
    W = W || 440; H = H || 236;
    const crew = t.crew || [];
    const pos = { human: [W / 2, 30] };
    const n = crew.length;
    crew.forEach((m, i) => { pos[m.name] = [n === 1 ? W / 2 : 46 + (i * (W - 92)) / (n - 1), 124]; });
    const edgesSeen = new Set();
    let edges = '';
    for (const [a, b] of turns || []) {
      if (!pos[a] || !pos[b]) continue;
      const k = [a, b].sort().join('|');
      if (edgesSeen.has(k)) continue;
      edgesSeen.add(k);
      const [x1, y1] = pos[a], [x2, y2] = pos[b];
      if (y1 === y2) { const mx = (x1 + x2) / 2, my = y1 + 58 + Math.abs(x2 - x1) * 0.1; edges += `<path class="hq-e" d="M${x1} ${y1} Q${mx} ${my} ${x2} ${y2}"/>`; }
      else edges += `<line class="hq-e" x1="${x1}" y1="${y1}" x2="${x2}" y2="${y2}"/>`;
    }
    let calls = '';
    for (const m of crew) {
      if (m.state !== 'blocked' && m.state !== 'handed') continue;
      const [x, y] = pos[m.name];
      calls += `<line class="hq-call" x1="${x}" y1="${y - 14}" x2="${W / 2}" y2="44"/>`;
    }
    let nodes = `<g class="hq-n human"><rect x="${W / 2 - 26}" y="16" width="52" height="28" rx="6"/><text x="${W / 2}" y="35">人間</text></g>`;
    for (const m of crew) {
      const [x, y] = pos[m.name];
      nodes += `<g class="hq-n ${esc(m.state)}"><circle class="ring" cx="${x}" cy="${y}" r="15"/><circle class="dot" cx="${x}" cy="${y}" r="11"/>` +
        (m.state === 'blocked' ? `<text class="bang" x="${x}" y="${y + 4}">!</text>` : '') +
        `<text class="lbl" x="${x}" y="${y + 30}">${esc(m.name)}</text></g>`;
    }
    const byMember = {};
    const free = [];
    for (const w of t.workers || []) {
      if (!pos[w.from]) { free.push(w); continue; }
      (byMember[w.from] || (byMember[w.from] = [])).push(w);
    }
    for (const [from, ws] of Object.entries(byMember)) {
      const [x] = pos[from];
      ws.forEach((w, i) => {
        const wx = x + (i - (ws.length - 1) / 2) * 34, wy = 204;
        nodes += `<line class="hq-e wk" x1="${x}" y1="139" x2="${wx}" y2="${wy - 7}"/><g class="hq-n wkr ${esc(w.state)}"><circle class="dot" cx="${wx}" cy="${wy}" r="6"/><text class="lbl s" x="${wx}" y="${wy + 20}">${esc(w.name)}</text></g>`;
      });
    }
    // A worker whose member the reading does not name stands on the row below,
    // spread out and unconnected: the figure does not draw a line it cannot
    // stand behind.
    free.forEach((w, i) => {
      const wx = free.length === 1 ? W / 2 : 30 + (i * (W - 60)) / (free.length - 1), wy = 204;
      nodes += `<g class="hq-n wkr ${esc(w.state)}"><circle class="dot" cx="${wx}" cy="${wy}" r="6"/><text class="lbl s" x="${wx}" y="${wy + 20}">${esc(w.name)}</text></g>`;
    });
    return `<svg class="hq-svg" viewBox="0 0 ${W} ${H}" role="img" aria-label="${esc(t.name)} のメンバーとターン">${edges}${calls}${nodes}</svg>`;
  }

  // ---- 動きの欄（チーム定義の一覧） --------------------------------------
  // runCell is one definition's 動き cell (spec 5 章): the running teams it has,
  // with their workspace and 止める, and 起動… with 前は <workspace> when none is.
  // A global definition also lists its running teams by project, and always
  // offers 起動…, because it can still be started in another project.
  function runCell(definition, place, project) {
    const live = runningFor(definition, place, project);
    const parts = live.map((t) => {
      const label = t.starting ? '起動の途中 · 許可待ち' : (t.waiting || []).length ? '動いている · 許可待ち' : '動いている';
      const where = (place === 'global' ? `${t.project_name || projectName(t.project)} · ` : '') + (t.workspace.label || t.workspace.id);
      return `<span class="hq-on${t.starting || (t.waiting || []).length ? ' wait' : ''}">${esc(label)}</span><span class="hq-wsn">${esc(where)}</span><button type="button" class="hq-btn sm danger" data-hq="stop" data-team="${esc(t.team)}">止める</button>`;
    });
    if (!live.length || place === 'global') {
      const last = !live.length ? lastWorkspaceOf(definition) : '';
      // A global definition chooses its project in the start sheet, so its
      // button names none (designer's note).
      const startProject = place === 'global' ? '' : (project || '');
      parts.push(`<button type="button" class="hq-btn sm" data-hq="start" data-def="${esc(definition)}" data-place="${esc(place || 'project')}" data-project="${esc(startProject)}">起動…</button>${last ? `<span class="hq-last">前は ${esc(last)}</span>` : ''}`);
    }
    return parts.join('');
  }

  // ---- 外に出す ---------------------------------------------------------
  window.hergyHQ = {
    state: st,
    POLL,
    token,
    parseOpen,
    projectOfGroup,
    groupKey,
    groupLabel,
    groupDisplay,
    projectName,
    runningFor,
    runningTeam,
    lastWorkspaceOf,
    definitionOf,
    groupOf,
    itemGroup,
    teamSVG,
    runCell,
    readAll,
    readCore,
    readGroups,
    readRunning,
    closeSheet,
    startSheet,
    stopSheet,
    esc,
  };

  // The page's band is written by the server; the reading keeps its numbers
  // right (spec 2 章). The first reading runs now, the groups when a search is
  // opened (the activity screen needs them for the figures' turns), and the
  // ten-second one while the page is looked at.
  readCore().then(() => {
    const q = $('#hq-q');
    if (q && q.value.trim()) readGroups().then(() => renderResults(q.value.trim()));
    if (location.pathname.indexOf('/activity') === 0) readGroups();
  });
  setInterval(() => { if (document.visibilityState === 'visible') readRunning(); }, POLL);
  document.addEventListener('visibilitychange', () => { if (document.visibilityState === 'visible') readRunning(); });

  document.addEventListener('DOMContentLoaded', () => {
    mountSearch();
    const bar = $('.bar');
    if (bar) document.documentElement.style.setProperty('--stick', bar.offsetHeight + 'px');
  });
  window.addEventListener('resize', () => {
    const bar = $('.bar');
    if (bar) document.documentElement.style.setProperty('--stick', bar.offsetHeight + 'px');
  });
})();
