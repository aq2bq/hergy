// activity-v2.js — 活動の画面（spec-activity 2、spec 6 章、r-8a67、d-8a45）。
//
// 上から 3 段: 人間を待っている（許可待ち・人間の番）と止まっているチーム、
// 動いているチームの図（1 チーム 1 枚）、選んだチームの中。行の一覧は使わない。
// 読むのは /api/activity と /api/activity/{team}、それに本部の読み
// （/api/hq/running、チーム定義の turns）で、状態の判定はサーバの 1 つをそのまま
// 使う。長さ・時刻の言い方もサーバが作る（V1）。この画面は文を 1 つも作らない。
(() => {
  'use strict';

  const A = window.hergyHQ;
  const POLL = 10000;
  // The screen's fixed words (2.2, 2.5). The server's answer carries the state's
  // name (answer, idle, …) only; these are the letters a person reads.
  const ST = { answer: '許可待ち', turn: '人間の番', stalled: '止まっている', working: '仕事中' };
  const NS = { blocked: '許可待ち', handed: '人間に渡した', active: '仕事中', idle: '止まっている', absent: '動いていない' };
  const RANK = { answer: 0, turn: 1, stalled: 2, working: 3 };

  const list = document.getElementById('list');
  const words = JSON.parse((document.getElementById('activity-words') || {}).textContent || '{}');
  const esc = (s) => String(s == null ? '' : s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  const enc = (s) => encodeURIComponent(String(s == null ? '' : s));

  // The board's own state: the last reading, the opened parts, and the team a
  // person picked. The pick and the page's scroll are kept across the ten-second
  // reading (spec 6 章).
  let items = [];
  let note = { ok: true, text: '' };
  const docs = new Map();
  let teams = [];
  let selected = null;
  let reading = false;

  function openTeam() {
    if (!A) return { name: '', group: '' };
    const raw = (new URLSearchParams(location.search).get('open') || '');
    const one = A.parseOpen(raw)[0];
    return one || { name: '', group: '' };
  }

  // ---- reading ---------------------------------------------------------
  async function readJSON(url) {
    try {
      const res = await fetch(url, { credentials: 'same-origin', headers: { Accept: 'application/json' } });
      if (!res.ok) return null;
      return await res.json();
    } catch (e) { return null; }
  }

  function turnsOf(def) {
    return ((def && def.turns) || []).map((t) => (Array.isArray(t) ? [t[0], t[1]] : [t.from, t.to]));
  }
  function memberOrderOf(def, nodes) {
    const named = ((def && def.members) || []).map((m) => m.name);
    const rest = (nodes || []).map((n) => n.name).filter((n) => named.indexOf(n) < 0);
    return named.concat(rest);
  }
  function askingLine(lines) {
    const clean = (lines || []).map((s) => String(s).replace(/^[●\s]+/, '')).filter(Boolean);
    return clean.find((s) => /(よいか|ますか|ですか|\?|？)/.test(s)) || clean[0] || '';
  }
  function lastMoment(t) {
    const at = (t.crew || []).map((m) => m.at).filter(Boolean).sort();
    return at.length ? at[at.length - 1] : '';
  }

  // sheetOf reads one team as the board draws it: the members in the
  // definition's order with their live state, the workers under the member that
  // added them, the turns of team.yaml, and the sentence of the screen's last
  // line (spec 6 章).
  function sheetOf(item) {
    const doc = docs.get(item.name);
    const run = A ? A.runningTeam(item.name) : null;
    const defName = item.summary.definition || item.name;
    const def = (A && A.definitionOf(item.project, defName)) || null;
    const nodes = doc ? (doc.nodes || []) : ((run && run.members) || []).map((m) => ({ name: m.name, herdr: m.herdr, state: m.state, at: '' }));
    const order = memberOrderOf(def, nodes);
    const byName = new Map(nodes.map((n) => [n.name, n]));
    const crew = order.filter((n) => byName.has(n)).map((name) => {
      const n = byName.get(name);
      return { name, herdr: n.herdr || (item.name + '-' + name), state: n.state || 'idle', at: n.at || '', since: n.since_at || n.at || '', doing: n.doing || '', stale: !!n.stale };
    });
    const hired = doc ? (doc.hired || []) : ((run && run.workers) || []).map((w) => ({ name: w.name, state: w.live ? 'active' : 'idle' }));
    const workers = hired.map((w) => ({ name: w.name, state: w.state || 'idle', doing: w.doing || '', from: w.from || '' }));
    const lines = (doc && doc.tail && doc.tail.lines) || [];
    const state = item.summary.state || 'idle';
    const waitNode = crew.find((m) => m.state === 'blocked') || crew.find((m) => m.state === 'handed');
    return {
      item, name: item.name, project: item.project, projectName: item.project_name || (A ? A.projectName(item.project) : ''),
      workspace: (item.workspace && item.workspace.label) || (run && run.workspace && run.workspace.label) || '',
      definition: defName, def, state, since: item.summary.since || null, last: item.summary.last || '',
      crew, workers, turns: turnsOf(def), lines,
      wait: waitNode || null,
      starting: !!(run && run.starting),
      launch: (run && run.launch) || '',
      error: item.error || (doc && doc.error) || '',
      herdr: (doc && doc.herdr) || '',
      tail: lines,
      question: askingLine(lines),
    };
  }

  function sortTeams(rows) {
    return rows.slice().sort((a, b) => (RANK[a.state] - RANK[b.state]) || a.name.localeCompare(b.name));
  }

  async function readAll() {
    if (reading) return;
    reading = true;
    try {
      // The figures' turns are the team definitions' own, so the groups are read
      // with the activity (the other screens read them when their search opens).
      if (A && A.readGroups) await A.readGroups();
      const answer = await readJSON('/api/activity');
      if (answer) {
        items = answer.items || [];
        note = answer.herdr || { ok: true, text: '' };
        window.hergyNav && window.hergyNav.setActivity(answer.running || 0, answer.waiting || 0, !!(answer.herdr && answer.herdr.ok) && (answer.running || 0) > 0);
      }
      // The headquarters reading is the same one the list's 動き column uses, so
      // a team's state cannot read two ways (api.md 3.5).
      const running = await readJSON('/api/hq/running');
      if (running && A) A.state.running = running;
      const wanted = items.map((t) => t.name);
      await Promise.all(items.map(async (t) => {
        const one = await readJSON('/api/activity/' + enc(t.name));
        if (one && one.doc) docs.set(t.name, one.doc);
      }));
      for (const name of Array.from(docs.keys())) if (wanted.indexOf(name) < 0 && !items.some((t) => t.name === name)) docs.delete(name);
      teams = sortTeams(items.map(sheetOf));
      const pick = openTeam();
      if (!selected || !teams.some((t) => t.name === selected)) {
        const found = pick.name ? teams.find((t) => t.name === pick.name && (!pick.group || t.project === pick.group)) : null;
        selected = found ? found.name : (teams[0] ? teams[0].name : null);
      }
      draw();
    } finally {
      reading = false;
    }
  }

  // ---- drawing ---------------------------------------------------------
  // A workspace herdr did not name is left out whole, so no separator stands
  // without a word (r-5b96 audit 11).
  const whereText = (t, node) => [
    t.workspace ? `ワークスペース ${t.workspace}` : '',
    `タブ ${t.name}`,
    `ペイン ${(node && node.herdr) || t.name}`,
  ].filter(Boolean).join(' · ');
  const tabWhere = (t) => [t.workspace ? `ワークスペース ${t.workspace}` : '', `タブ ${t.name}`].filter(Boolean).join(' · ');

  // attention is the first tier: every member waiting for a human, with where to
  // answer, and every team none of whose members holds the turn (spec 6 章).
  function attention(rows) {
    const waiting = [];
    for (const t of rows) {
      for (const m of t.crew) {
        if (m.state !== 'blocked' && m.state !== 'handed') continue;
        const q = t.question || (t.starting ? '起動の途中で、このフォルダを信頼するかを聞いています' : '');
        waiting.push(`<button type="button" class="hq-at" data-pick="${esc(t.name)}"><span class="st ${m.state === 'blocked' ? 'answer' : 'turn'}">${m.state === 'blocked' ? '許可待ち' : '人間の番'}</span>
          <span class="hq-aw"><code>${esc(m.herdr || (t.name + '-' + m.name))}</code><span class="hq-pw">${esc(t.projectName)}</span></span>
          <span class="hq-aq">${esc(q)}</span>
          <span class="hq-where">${esc(whereText(t, m))}</span>
          <span class="hq-tl">${m.since ? esc(m.since) : ''}</span></button>`);
      }
    }
    const stalled = rows.filter((t) => t.state === 'stalled').map((t) => `<button type="button" class="hq-at stall" data-pick="${esc(t.name)}"><span class="st stalled">止まっている</span>
      <span class="hq-aw"><code>${esc(t.name)}</code><span class="hq-pw">${esc(t.projectName)}</span></span>
      <span class="hq-aq">誰もターンを持っていない</span>
      <span class="hq-where">${esc(tabWhere(t))}</span>
      <span class="hq-tl">${esc((t.since && t.since.len) || '')}</span></button>`);
    if (!waiting.length && !stalled.length) return `<section class="hq-att calm"><p>人間を待っているチームも、止まっているチームもありません。</p></section>`;
    return `<section class="hq-att"><div class="hq-ah"><b>人間を待っている</b><span class="n">${waiting.length}</span>${stalled.length ? `<b class="sp">止まっている</b><span class="n">${stalled.length}</span>` : ''}</div><div class="hq-al">${waiting.join('')}${stalled.join('')}</div></section>`;
  }

  function countsLine(t) {
    const n = (k) => t.crew.filter((m) => m.state === k).length;
    // The figure's counts add up to the crew: a member with no agent is
    // 動いていない, as the opened list calls it (r-5b96 audit K1).
    return `<div class="hq-tc"><span><i class="dot active"></i>仕事中 ${n('active')}</span><span><i class="dot idle"></i>止まっている ${n('idle')}</span>${n('blocked') ? `<span><i class="dot blocked"></i>許可待ち ${n('blocked')}</span>` : ''}${n('absent') ? `<span><i class="dot absent"></i>動いていない ${n('absent')}</span>` : ''}${t.workers.length ? `<span class="wk">ワーカー ${t.workers.length}</span>` : ''}</div>`;
  }

  function tile(t) {
    const sel = t.name === selected;
    const stall = t.state === 'stalled'
      ? `<p class="hq-why">誰もターンを持っていません（人間の番でもない）。最後に動いたのは ${esc(lastMoment(t) || (t.since && t.since.at) || '')}。lead に続きを渡すか、<code>hergy nudge</code> でうながす</p>`
      : '';
    return `<article class="hq-tile ${esc(t.state)}${sel ? ' sel' : ''}" data-team="${esc(t.name)}">
      <button type="button" class="hq-th" data-pick="${esc(t.name)}" aria-pressed="${sel}">
        <span class="hq-tn">${esc(t.name)}</span><span class="hq-pw">${esc(t.projectName)} · ${esc(t.workspace)}</span>
        <span class="st ${esc(t.state)}">${ST[t.state] || ''}</span>
        <span class="hq-tl">${t.since ? `<b>${esc(t.since.len)}</b> ${esc(t.since.at)}` : ''}</span>
      </button>
      ${A ? A.teamSVG(t, t.turns) : ''}
      ${stall}
      ${countsLine(t)}
    </article>`;
  }

  // detail is the third tier: the opened team's last screen, where to answer, and
  // every member and worker with its state (spec 6 章).
  function detail(t) {
    if (!t) return '';
    // The screen's last lines are drawn when there are some: a team whose pane
    // could not be read (a working one, often) has no heading and no empty box
    // (r-5b96 audit 10). The place to answer stands on its own then.
    const answerBox = t.wait ? `<div class="hq-ans"><span>答える場所</span><span class="hq-where">${esc(whereText(t, t.wait))}</span></div>` : '';
    const hasTail = t.tail.length > 0;
    const screenBox = hasTail ? `<div class="hq-dq"><div class="th">画面の最後（<code>${esc(t.wait ? (t.wait.herdr || (t.name + '-' + t.wait.name)) : t.last ? (t.name + '-' + t.last) : t.name)}</code>）</div>
      <pre class="term">${esc(t.tail.join('\n'))}</pre>${answerBox}</div>`
      : (answerBox ? `<div class="hq-dq">${answerBox}</div>` : '');
    const rows = t.crew.map((m) => `<li class="${esc(m.state)}"><i class="dot ${esc(m.state)}"></i><span class="nm">${esc(m.name)}</span><span class="sw">${NS[m.state] || ''}</span><span class="t">${esc(m.at || '')}</span>${m.doing ? `<span class="hq-doing">${esc(m.doing)}</span>` : ''}${m.stale ? '<span class="dlv-sub"><button class="dlv-badge" type="button" data-deliver-open title="伝えていない変更があります">定義が変わった</button></span>' : ''}</li>`).join('');
    const wk = t.workers.length ? `<li class="sub">ワーカー</li>` + t.workers.map((w) => `<li class="${esc(w.state)}"><i class="dot ${esc(w.state)}"></i><span class="nm wk">${esc(w.name)}</span><span class="sw">${NS[w.state] || ''}</span><span class="t">${esc(w.from ? w.from + ' が追加' : '')}</span>${w.doing ? `<span class="hq-doing">${esc(w.doing)}</span>` : ''}</li>`).join('') : '';
    const defHref = `/teams?open=${enc(A ? A.token(t.definition, t.project) : t.definition)}`;
    return `<section class="hq-detail" aria-label="${esc(t.name)} の中">
      <div class="hq-dh"><b>${esc(t.name)}</b><span class="hq-pw">${esc([t.projectName, t.workspace ? 'ワークスペース ' + t.workspace : ''].filter(Boolean).join(' · '))}</span><span class="st ${esc(t.state)}">${ST[t.state] || ''}</span>
        <span class="hq-dacts"><a class="hq-btn sm" href="${esc(defHref)}">チーム定義を開く</a><button type="button" class="hq-btn sm danger" data-hq="stop" data-team="${esc(t.name)}">チームを止める…</button></span></div>
      <div class="hq-db">${screenBox}<ul class="nodes">${rows}${wk}</ul></div></section>`;
  }

  let signature = '';
  function draw() {
    if (!list) return;
    if (!teams.length) {
      list.innerHTML = note.ok ? `<p class="gone">${esc(words.none || '並べたプロジェクトに、動いているチームはありません。チーム定義の画面の「起動…」で起こせます。')}</p>` : `<p class="gone">${esc(note.text)}</p>`;
      signature = '';
      return;
    }
    if (!note.ok) {
      list.innerHTML = `<p class="gone">${esc(note.text)}</p>`;
      signature = '';
      return;
    }
    const next = teams.map((t) => [t.name, t.state, t.crew.map((m) => m.name + m.state + m.at + (m.stale ? '!' : '') + (m.doing || '')), t.workers.map((w) => w.name + w.state + (w.from || '') + (w.doing || '')), (t.tail || []).slice(-1)[0], t.since && t.since.len]).join('|') + '#' + selected;
    if (next === signature) return;
    signature = next;
    const scroll = window.scrollY;
    const sel = teams.find((t) => t.name === selected) || teams[0];
    list.innerHTML = `<div class="hq-board">${attention(teams)}<div class="hq-tiles">${teams.map(tile).join('')}</div>${detail(sel)}</div>`;
    window.scrollTo(0, scroll);
  }

  // ---- pressing --------------------------------------------------------
  function pick(name) {
    selected = name;
    const t = teams.find((x) => x.name === name);
    if (t && A) {
      const q = new URLSearchParams(location.search);
      q.set('open', A.token(name, t.project));
      history.replaceState(null, '', location.pathname + '?' + q.toString());
    }
    draw();
  }

  if (list) {
    list.addEventListener('click', (e) => {
      const stop = e.target.closest('[data-hq="stop"]');
      if (stop && A) { A.stopSheet(stop.dataset.team); return; }
      const pickEl = e.target.closest('[data-pick]');
      if (pickEl) pick(pickEl.dataset.pick);
    });
    list.addEventListener('keydown', (e) => {
      if (e.key !== 'Enter' && e.key !== ' ') return;
      const el = e.target.closest('[data-pick]');
      if (!el) return;
      e.preventDefault();
      pick(el.dataset.pick);
    });
  }
  document.addEventListener('hergy:hq', () => { if (!reading) readAll(); });

  // ---- the ten-second reading ------------------------------------------
  readAll();
  setInterval(() => { if (document.visibilityState === 'visible') readAll(); }, POLL);
  document.addEventListener('visibilitychange', () => { if (document.visibilityState === 'visible') readAll(); });

  window.hergyPage = { screen: 'activity' };
})();
