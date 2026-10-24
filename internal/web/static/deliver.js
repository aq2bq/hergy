// deliver.js — 変更を伝える (r-a8fa). Every screen carries this part, because
// the band is on every screen. The band counts the running members whose prompt
// is not the one hergy last handed them (GET /api/delivery/count), and the sheet
// hands the current prompt to the ones a person picks (POST /api/delivery).
//
// The summary of what changes for one member is the server's (deliver.Compare):
// this file lays those sentences out and builds none of its own. A save never
// opens the sheet and never sends — it only says hergy:saved, and the band reads
// its number again.
(() => {
  'use strict';

  const API = window.hergyAPI;
  if (!API) return;
  const $ = API.$;
  const $$ = API.$$;
  const esc = (s) => String(s == null ? '' : s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));

  // 状態の字（brief 2）。サーバは状態の名だけを運び、字はここで決める。
  const STATE = { idle: '止まっている', handed: '人間に渡した', active: '仕事中', blocked: '許可待ち', absent: '動いていない' };
  const POLL = 15000; // 帯の数の読み直し（画面が見えている間だけ）

  let shown = 0;
  let sheet = null;
  let answer = null;
  let opening = false;
  let sending = false;
  let opener = null;

  // ---- 帯のボタン ----------------------------------------------------------
  // 画面の切り替えの右隣に、独立したボタン。数が見える所を、そのまま押す所に
  // する（試作 3 回目）。0 のときは出さない。
  const bar = () => $('.bar');
  // The search field of a headquarters stands right of the screens (spec 2 章),
  // and the delivery button right of it: the two are one band's little row.
  const screens = () => $('.bar .hq-search') || $('.bar .screens');

  function bandButton() {
    const el = $('.bar .dlv-bandbtn');
    if (el) return el;
    const after = screens();
    if (!after) return null;
    const b = document.createElement('button');
    b.type = 'button';
    b.className = 'dlv-bandbtn';
    b.setAttribute('data-dlv', 'open');
    b.setAttribute('aria-haspopup', 'dialog');
    b.setAttribute('title', '定義を直したあと、まだ伝えていない人');
    b.innerHTML = '変更を伝えていない人 <b>0</b>';
    after.insertAdjacentElement('afterend', b);
    return b;
  }

  function setCount(n) {
    shown = n;
    const b = bandButton();
    if (!b) return;
    const num = b.querySelector('b');
    if (num) num.textContent = String(n);
    b.hidden = n <= 0;
    b.setAttribute('aria-label', `変更を伝えていない人 ${n}`);
  }

  async function readCount() {
    try {
      const res = await API.deliveryCount();
      if (!res.ok) return; // 読めないときは、前の数をそのまま置く
      setCount(Number(API.answer(res).count) || 0);
    } catch (e) {
      // 読めないときは、前の数をそのまま置く
    }
  }

  // afterSave reads the number now: the server clears the count's five-second
  // answer on a save or a delete, so this reading is the new one (r-a8fa).
  function afterSave() {
    readCount();
  }

  // ---- 変わる所の行き先 ----------------------------------------------------
  // The server names each change's document by the path a screen shows
  // (".hergy/roles/builder.yaml", "~/.config/hergy/roles/scout.yaml",
  // ".hergy/teams/core/agreement.yaml"). A role's rule opens on the role
  // screen, the team's own rules on its rules tab, and everything else a member
  // reads opens on its team definition (brief 2).
  const baseName = (p) => String(p == null ? '' : p).replace(/\\/g, '/').split('/').pop();
  function roleOf(path) {
    const m = /(^|\/)roles\/([^/]+)$/.exec(String(path == null ? '' : path).replace(/\\/g, '/'));
    return m ? m[2].replace(/\.(ya?ml|md)$/, '') : '';
  }
  function teamOf(path) {
    const m = /(^|\/)teams\/([^/]+)\/[^/]+$/.exec(String(path == null ? '' : path).replace(/\\/g, '/'));
    return m ? m[2] : '';
  }
  const enc = (s) => encodeURIComponent(String(s == null ? '' : s));

  function linkFor(member, change, group) {
    const kind = change.kind || '';
    if (kind === 'turn' || kind === 'worker' || kind === 'roster' || kind === 'template' || kind === 'prompt') {
      return `/teams?open=${enc(group.definition || group.name)}`;
    }
    if (kind === 'rule' || kind === 'document') {
      const role = roleOf(change.document);
      if (role) return `/roles?open=${enc(role)}${kind === 'rule' && change.rule ? `&rule=${enc(change.rule)}` : ''}`;
      const team = teamOf(change.document) || group.definition || group.name;
      const tab = kind === 'rule' || /agreement\.ya?ml$/.test(baseName(change.document)) ? `&agree=${enc(team)}` : '';
      return `/teams?open=${enc(team)}${tab}`;
    }
    return '';
  }

  // ---- シート --------------------------------------------------------------
  const INTRO = 'チェックした人に、初期プロンプトを送り直します（それまでの会話は残ります）。仕事中と許可待ちの人には送れません。';

  function groupHTML(group) {
    const def = group.definition && group.definition !== group.name
      ? `（定義 <code>${esc(group.definition)}</code>）`
      : '';
    const members = (group.members || []).map((m) => memberHTML(m, group)).join('');
    return `<div class="dlv-team">
      <div class="dlv-th">チーム <code>${esc(group.name)}</code>${def}</div>
      <div class="dlv-ms">${members}</div>
    </div>`;
  }

  function memberHTML(m, group) {
    const can = !!m.can_send;
    const changes = (m.changes || []).map((c) => {
      const href = linkFor(m, c, group);
      const text = esc(c.text || '');
      return `<span>${href ? `<a class="dlv-chg" href="${esc(href)}">${text}</a>` : text}</span>`;
    }).join('');
    return `<label class="dlv-mm${can ? '' : ' no'}">
      <input type="checkbox" data-dlv-name="${esc(m.name)}"${can ? ' checked' : ' disabled'}>
      <span class="dlv-who"><code>${esc(m.name)}</code><span class="dlv-st">${esc(STATE[m.state] || m.state)}</span>${can ? '' : '<span class="dlv-wait">仕事が終わると伝えられます</span>'}</span>
      <span class="dlv-what">${changes}</span>
    </label>`;
  }

  function sheetHTML(a, note) {
    const groups = (a.teams || []).map((g) => groupHTML(g)).join('');
    return `<section class="dlv-sheet" role="dialog" aria-label="変更を伝える" tabindex="-1">
      <div class="dlv-h"><b>変更を伝える</b><button class="dlv-x" type="button" data-dlv="close" aria-label="閉じる">×</button></div>
      <div class="dlv-sub2">${INTRO}</div>
      <p class="dlv-note err" data-dlv-note${note ? '' : ' hidden'}>${esc(note)}</p>
      ${groups}
      <div class="dlv-acts"><button class="dlv-btn dlv-primary" type="button" data-dlv="send" disabled>伝える人を選んでください</button><button class="dlv-btn" type="button" data-dlv="close">やめる</button></div>
    </section>`;
  }

  function showNote(text) {
    if (!sheet) return;
    const el = $('[data-dlv-note]', sheet);
    if (!el) return;
    el.textContent = text;
    el.hidden = !text;
  }

  function syncSend() {
    if (!sheet) return;
    const checked = $$('input[data-dlv-name]:checked', sheet).length;
    const blocked = !!(answer && answer.herdr && answer.herdr.ok === false);
    const b = $('[data-dlv=send]', sheet);
    if (!b) return;
    b.disabled = checked === 0 || blocked || sending;
    b.textContent = checked ? `選んだ ${checked} 人に伝える` : '伝える人を選んでください';
  }

  function drawSheet(a, note) {
    close(false);
    answer = a;
    opener = document.activeElement;
    document.body.insertAdjacentHTML('beforeend', sheetHTML(a, note));
    sheet = $('.dlv-sheet');
    if (sheet) sheet.focus({ preventScroll: true });
    syncSend();
  }

  async function openSheet() {
    if (opening) return;
    opening = true;
    try {
      const res = await API.delivery();
      if (!res.ok) return; // 読めないときは、シートを開かない
      const a = API.answer(res);
      drawSheet(a, a.herdr && a.herdr.ok === false ? a.herdr.text || '' : '');
    } catch (e) {
      // 読めないときは、シートを開かない
    } finally {
      opening = false;
    }
  }

  function close(restore = true) {
    $$('.dlv-sheet').forEach((el) => el.remove());
    sheet = null;
    answer = null;
    if (restore && opener && opener.isConnected && typeof opener.focus === 'function') {
      opener.focus({ preventScroll: true });
    }
    opener = null;
  }

  async function send() {
    if (!sheet || sending) return;
    const names = $$('input[data-dlv-name]:checked', sheet).map((el) => el.dataset.dlvName);
    if (!names.length) return;
    sending = true;
    syncSend();
    try {
      const res = await API.deliver(names);
      if (res.status === 422) {
        // 1 人でも拒めば、誰にも送っていない (brief 2)。シートは閉じない。
        showNote(`誰にも送っていません。${API.problemText(res, '')}`);
        return;
      }
      if (!res.ok) {
        showNote(`誰にも送っていません。${API.problemText(res, res.text || '')}`);
        return;
      }
      close();
      toast(`${names.join('・')} に伝えました`);
      readCount();
    } catch (e) {
      showNote('送れませんでした。');
    } finally {
      sending = false;
      syncSend();
    }
  }

  let toastTimer = 0;
  function toast(text) {
    let el = $('.dlv-toast');
    if (!el) {
      el = document.createElement('div');
      el.className = 'dlv-toast';
      el.setAttribute('role', 'status');
      document.body.appendChild(el);
    }
    el.textContent = text;
    el.hidden = false;
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => { el.hidden = true; }, 4000);
  }

  // ---- 押す ----------------------------------------------------------------
  document.addEventListener('click', (e) => {
    const badge = e.target.closest('[data-deliver-open]');
    if (badge) {
      e.preventDefault();
      openSheet();
      return;
    }
    const b = e.target.closest('[data-dlv]');
    if (!b || !b.closest('.dlv-sheet, .bar')) return;
    const act = b.dataset.dlv;
    if (act === 'open') {
      e.preventDefault();
      openSheet();
    } else if (act === 'close') {
      e.preventDefault();
      close();
    } else if (act === 'send') {
      e.preventDefault();
      send();
    }
  }, true);

  document.addEventListener('change', (e) => {
    if (e.target.matches('.dlv-sheet input[data-dlv-name]')) syncSend();
  });

  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && sheet) {
      e.preventDefault();
      close();
    }
  });

  // A save never opens the sheet and never sends: it only says the band's number
  // may have moved.
  document.addEventListener('hergy:saved', afterSave);

  // ---- 始め ----------------------------------------------------------------
  function start() {
    bandButton();
    readCount();
    setInterval(() => {
      if (document.visibilityState === 'visible') readCount();
    }, POLL);
    document.addEventListener('visibilitychange', () => {
      if (document.visibilityState === 'visible') readCount();
    });
  }

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', start);
  else start();
})();
