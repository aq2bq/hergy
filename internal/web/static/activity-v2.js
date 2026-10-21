// activity-v2.js — 活動の画面（spec-activity 2、r-8a67）。行はサーバの答え
// （GET /api/activity）から描き、開いている行の中は GET /api/activity/{team} から
// 描く。長さ・時刻・何分前の言い方はサーバが作り、ここでは作らない（V1）。
// メンバーを名前で扱う操作（focus・nudge・stop・tell）は無い。
//
// 押せる所（行、`▸ 上に N 行`、`ほか N 件`）は #list への委譲で受ける。読み直しで
// 描き直すので、要素ごとに listener を付け直さない。
(() => {
  'use strict';

  // 画面の固定語（2.2、2.5）。サーバの答えは状態の名（answer、idle、…）だけを
  // 運び、字はここで決める。長さ・時刻の言い方はサーバのもので、ここには無い。
  const ST = { answer: '許可待ち', turn: '人間の番', stalled: '止まっている', working: '仕事中' };
  const NS = { blocked: '許可待ち', handed: '人間に渡した', active: '仕事中', idle: '止まっている', absent: '動いていない' };

  const POLL = 10000; // 読み直しの間（2.6）
  const CAP_Q = 3;    // 続く問いを 1 行ずつ出す数（2.4）

  const list = document.getElementById('list');
  const words = JSON.parse((document.getElementById('activity-words') || {}).textContent || '{}');

  // openNames は開いている行。null は「URL に open が無い」＝初めの形で、一度でも
  // 押したら、開いている名前の集まりになる（2.3）。
  let openNames = openFromURL();
  // ▸ 上に N 行 を伸ばした行と、ほか N 件 を出した行（2.4）。読み直しでも持ち越す。
  const tailOut = new Set();
  const moreOut = new Set();

  // 最後に読んだ答え。
  let items = [];
  let note = { ok: true, text: '' };
  const docs = new Map();
  // 読み直しが重ならないようにする（10 秒ごとと、見えるようになったとき）。
  let reading = false;

  const esc = (s) => String(s == null ? '' : s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));

  // ---- 開いている行 (2.3) --------------------------------------------------
  function openFromURL() {
    const q = new URLSearchParams(location.search);
    if (!q.has('open')) return null;
    return new Set((q.get('open') || '').split(',').map((s) => s.trim()).filter(Boolean));
  }

  // 初めの形: 許可待ち・人間の番・止まっているの行が開いている。仕事中は閉じている。
  function isOpen(name) {
    if (openNames) return openNames.has(name);
    const item = items.find((t) => t.name === name);
    return !!item && item.summary.state !== 'working';
  }

  function toggle(name) {
    if (!openNames) openNames = new Set(items.filter((t) => isOpen(t.name)).map((t) => t.name));
    if (openNames.has(name)) openNames.delete(name);
    else openNames.add(name);
    const q = new URLSearchParams(location.search);
    // 空でも open を残す: 何も開いていない、と「open が無い」は別の意味（2.3）。
    q.set('open', Array.from(openNames).join(','));
    history.replaceState(null, '', location.pathname + '?' + q.toString());
    draw();
    // 開いた行はその場で読む（2.3）。読み直しを待つと、仕事中のチームを開いた
    // 人の前に、次の 10 秒まで空の欄が残る。
    if (isOpen(name)) readDocs([name]).then(draw);
  }

  // ---- 一覧の行 (2.2) ------------------------------------------------------
  function who(t) {
    const s = t.summary;
    if (s.state === 'answer') return `<b>${esc(s.last)}</b>`;
    if (s.state === 'turn') return `<span class="lb">最後のターン</span><b>${esc(s.last)}</b><span class="ar">→</span><span class="hm">人間</span>`;
    if (s.state === 'stalled') return s.last ? `<span class="lb">最後のターン</span><b>${esc(s.last)}</b>` : '<span class="lb">—</span>';
    const w = s.working || { members: [], workers: 0 };
    const names = (w.members || []).map((n) => `<b>${esc(n)}</b>`).join('<span class="ar">・</span>');
    return names + (w.workers ? `<span class="lb">ワーカー ${w.workers}</span>` : '');
  }

  function head(t) {
    const s = t.summary;
    const def = s.definition && s.definition !== t.name ? `<span class="def">定義 <code>${esc(s.definition)}</code></span>` : '';
    // since.at はサーバの文（"13:34 から" / "09:47 より前"）をそのまま出す（V1）。
    const since = s.since ? `<span class="len">${esc(s.since.len)}</span><span class="at">${esc(s.since.at)}</span>` : '';
    return `<div class="ahead" id="row-${esc(t.name)}" role="button" tabindex="0" aria-expanded="${isOpen(t.name)}">
    <i class="chev"></i>
    <div class="c-name"><span class="nm">${esc(t.name)}</span>${def}</div>
    <div class="c-st"><span class="st ${esc(s.state)}">${ST[s.state] || ''}</span></div>
    <div class="c-who">${who(t)}</div>
    <div class="c-since">${since}</div>
  </div>`;
  }

  // ---- 開いた中の左: 何を決めてほしいか (2.4) ------------------------------
  const qrow = (r) => `<div class="mrow"><code>${esc(r.id)}</code><span>${esc(r.title)}</span><span class="ago">${esc(r.ago)}</span></div>`;

  function question(d) {
    const all = d.questions || [];
    const first = all[0];
    if (!first) return '';
    const rest = all.slice(1);
    const shown = moreOut.has(d.name) ? rest : rest.slice(0, CAP_Q);
    const left = rest.length - shown.length;
    const opts = (first.options || []).length ? `<ol class="opts">${first.options.map((o) => `<li>${esc(o)}</li>`).join('')}</ol>` : '';
    const more = shown.length
      ? `<div class="more">${shown.map(qrow).join('')}${left ? `<span class="rest" role="button" tabindex="0" id="more-${esc(d.name)}">ほか ${left} 件</span>` : ''}</div>`
      : '';
    return `<div class="q">
      <div class="qh"><span class="qid">${esc(first.id)}</span><span class="ago">${esc(first.ago)}</span></div>
      <p class="qt">${esc(first.title)}</p>
      ${opts}
      ${more}
    </div>`;
  }

  // The pane is herdr's own lines (2.4). The newline right under <pre> is
  // dropped by the HTML parser, so one is written there on purpose: a pane whose
  // first line is empty then keeps that empty line, and every line herdr sent
  // stands — as many as were sent, however the pane is extended (audit R3).
  function tail(d) {
    const lines = (d.tail && d.tail.lines) || [];
    if (!lines.length) return ''; // 読めないときはこの段が無い (2.4)
    const cap = (d.tail && d.tail.shown) || 6;
    const out = tailOut.has(d.name);
    const shown = out ? lines : lines.slice(-cap);
    const hidden = lines.length - shown.length;
    const rest = hidden > 0
      ? `<span class="rest" role="button" tabindex="0" id="tail-${esc(d.name)}">▸ 上に ${hidden} 行</span>`
      : out ? `<span class="rest" role="button" tabindex="0" id="tail-${esc(d.name)}">▾ 上の行をしまう</span>` : '';
    return `<div class="tail">
      <div class="th">画面の最後${rest}</div>
      <pre class="term">
${shown.map((l) => esc(l)).join('\n')}</pre>
    </div>`;
  }

  function ask(d) {
    if (d.error) {
      // チーム定義が読めない (2.2): その文を 問題 の札で 1 行と、チーム定義 の札。
      return `<section class="ask" aria-label="${esc(d.name)}">
      <div class="qh"><span class="st stalled">問題</span><span>${esc(d.error)}</span></div>
      <div class="qh"><a class="st stalled" href="/teams?open=${encodeURIComponent(d.definition)}">チーム定義</a></div>
    </section>`;
    }
    if (!d.herdr) return ''; // 仕事中のチームは右の欄だけ (2.5)
    const q = d.gy && d.gy.ok === false
      ? `<div class="q"><div class="qh"><span class="st stalled">警告</span><span>${esc(d.gy.text)}</span></div></div>`
      : question(d);
    return `<section class="ask" aria-label="${esc(d.herdr)}">
    <div class="who2"><code>${esc(d.herdr)}</code><span class="rl">ロール ${esc(d.role)}</span></div>
    ${q}
    ${tail(d)}
  </section>`;
  }

  // ---- 開いた中の右: どうして止まっているか (2.5) --------------------------
  function crew(d) {
    const row = (n) => `<li class="${esc(n.state)}${n.name === d.last ? ' last' : ''}"><i class="dot ${esc(n.state)}"></i><span class="nm">${esc(n.name)}</span><span class="sw">${NS[n.state] || ''}</span><span class="t">${esc(n.at || '')}</span></li>`;
    // ワーカーの行は名前だけ（X1）: 目的と呼んだメンバーは herdr にも起動の記録にも無い。
    const wrow = (n) => `<li class="${esc(n.state)}"><i class="dot ${esc(n.state)}"></i><span class="nm wk">${esc(n.name)}</span><span class="sw">${NS[n.state] || ''}</span><span class="t">${esc(n.at || '')}</span></li>`;
    const hired = (d.hired || []).length ? `<li class="sub">ワーカー</li>${d.hired.map(wrow).join('')}` : '';
    return `<aside class="crew" aria-label="${esc(d.name)} のメンバー">
    <ul class="nodes">${(d.nodes || []).map(row).join('')}${hired}</ul>
  </aside>`;
  }

  function body(t) {
    const d = docs.get(t.name);
    if (!d) return ''; // まだ読めていない行は、中の箱だけが無い
    return `${ask(d)}${crew(d)}`;
  }

  // ---- 描く ----------------------------------------------------------------
  // 描き直しでフォーカスを落とさない (2.6): 押していた所の id を覚えて、描いた後に
  // 戻す。開いている行と伸ばした箱は、DOM ではなくこの先の状態が持っている。
  function draw() {
    const active = document.activeElement;
    const id = active && active.id ? active.id : '';
    const rows = items.map((t) => {
      const open = isOpen(t.name);
      return `<article class="act ${open ? 'open' : ''}" data-team="${esc(t.name)}">${head(t)}${open ? `<div class="abody">${body(t)}</div>` : ''}</article>`;
    });
    const lines = [];
    if (!note.ok) lines.push(`<p class="gone">${esc(note.text)}</p>`);
    else if (!items.length) lines.push(`<p class="gone">${esc(words.none || '')}</p>`);
    list.innerHTML = rows.join('') + lines.join('');
    if (id) {
      const again = document.getElementById(id);
      if (again) again.focus();
    }
  }

  // ---- 読む ----------------------------------------------------------------
  async function readJSON(url) {
    const res = await fetch(url, { credentials: 'same-origin', headers: { Accept: 'application/json' } });
    if (!res.ok) return null;
    return res.json();
  }

  // readDocs reads the given rows' opened parts and puts them in place; the
// caller draws. A row that does not answer keeps what it had, so a moment of
// trouble does not empty it, and a row that is no longer open is forgotten.
async function readDocs(names) {
    if (!names.length) return;
    await Promise.all(names.map(async (name) => {
      let answer = null;
      try {
        answer = await readJSON('/api/activity/' + encodeURIComponent(name));
      } catch (e) {
        // 読めなかった行は、前に読んだ中身をそのまま置いておく: 一瞬で空にしない。
      }
      if (answer && answer.doc) docs.set(name, answer.doc);
    }));
    for (const name of Array.from(docs.keys())) {
      if (!isOpen(name)) docs.delete(name);
    }
  }

  // openList is the rows that stand open, in the list's own order (2.3).
  function openList() {
    return items.filter((t) => isOpen(t.name)).map((t) => t.name);
  }

  // band hands the reading's two numbers to the band (2.1): the screen's own
  // re-reading keeps 活動 N and 待ち N the same as the list. The numbers are
  // left off while herdr cannot be read and while no team runs (2.7).
  function band(answer) {
    const nav = window.hergyNav;
    if (!nav || !nav.setActivity) return;
    const running = answer.running || 0;
    const waiting = answer.waiting || 0;
    nav.setActivity(running, waiting, !!(answer.herdr && answer.herdr.ok) && running > 0);
  }

  async function readAll() {
    if (reading) return;
    reading = true;
    try {
      const answer = await readJSON('/api/activity');
      if (!answer) return; // 読めないときは、前の画面のままにする
      items = answer.items || [];
      note = answer.herdr || { ok: true, text: '' };
      await readDocs(openList());
      draw();
      band(answer);
    } catch (e) {
      // 読めないときは、前の画面のままにする。
    } finally {
      reading = false;
    }
  }

  // ---- 押す (2.3、2.4) -----------------------------------------------------
  // 押す所は増える所と減る所があるので、id で見分ける。伸ばした・出したは、行ごとに
  // 覚えて、次に描いたときも同じ形で出す。
  function press(rest) {
    const act = rest.closest('.act');
    const name = act && act.dataset.team;
    if (!name) return;
    const set = rest.id === 'more-' + name ? moreOut : tailOut;
    if (set.has(name)) set.delete(name);
    else set.add(name);
    draw();
  }

  list.addEventListener('click', (e) => {
    const rest = e.target.closest('.rest');
    if (rest) {
      press(rest);
      return;
    }
    const ahead = e.target.closest('.ahead');
    const act = ahead && ahead.closest('.act');
    if (act) toggle(act.dataset.team);
  });

  // Enter と Space でも押せる（role="button" の要素は、自分では押せない）。
  list.addEventListener('keydown', (e) => {
    if (e.key !== 'Enter' && e.key !== ' ') return;
    const el = e.target.closest('.rest, .ahead');
    if (!el) return;
    e.preventDefault();
    el.click();
  });

  // ---- 読み直し (2.6) ------------------------------------------------------
  readAll();
  setInterval(() => {
    if (document.visibilityState === 'visible') readAll();
  }, POLL);
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'visible') readAll();
  });

  window.hergyPage = { screen: 'activity' };
})();