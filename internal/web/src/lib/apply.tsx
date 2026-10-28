// lib/apply.tsx — 反映 (n-4b22, d-36ef)。module の入口。
//
// 動いているチームへの定義の変更の反映を、1 つの操作にする。帯は、まだ反映して
// いないメンバーの数（GET /api/apply/count）を出し、欄はチームごと・メンバーごとに
// 反映の仕方（送り直す・起動する・起動し直す・反映できない）と変わったことを出して、
// 選んだ人に POST /api/apply する。送ったあとの進み具合は、起動の欄が同じ形で出す。
//
// 帯のボタン・欄・下の知らせは Preact が描く（lib/apply/view.js）。状態と
// サーバーとの口は lib/apply/state.js。保存は欄を開かず、送りもしない:
// hergy:saved を聞いて、帯の数を読み直す。
//
// 器はこの 1 本が作る。帯のボタンの器は `.bar` の `.screens` / `.hq-search` の
// 直後に置き、`display: contents`（CSSOM。style 属性は CSP が拒む）で並びに
// 関わらせない。button そのものには付けない（支援技術から button と見えなく
// なるブラウザがある）。
import { render } from './preact.js';
import { BandButton, Layer } from './apply/view.js';
import { close, opened, readCount, readTeams, POLL } from './apply/state.js';

// ---- 器 -----------------------------------------------------------------
// The search field of a headquarters stands right of the screens (spec 2 章),
// and the apply button right of it: the two are one band's little row.
function mountBand(): HTMLDivElement | null {
  const after = document.querySelector('.bar .hq-search') || document.querySelector('.bar .screens');
  if (!after) return null;
  const host = document.createElement('div');
  host.className = 'apl-band';
  host.style.display = 'contents';
  after.insertAdjacentElement('afterend', host);
  render(<BandButton />, host);
  return host;
}

function mountLayer(): HTMLDivElement {
  const host = document.createElement('div');
  host.className = 'apl-layer';
  document.body.appendChild(host);
  render(<Layer />, host);
  return host;
}

// ---- 押す ----------------------------------------------------------------
// The mark on the activity screen's team and on the 動き column is those screens'
// own markup, and the screens answer it: they call openSheet with their team
// (activity/board.tsx, lib/hq/run.tsx). What stands on this part is its own: the
// band's button, the sheet, and the one key that closes it.
document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape' && opened.value) {
    e.preventDefault();
    close();
  }
});

// A save never opens the sheet and never sends: it only says the numbers may
// have moved.
document.addEventListener('hergy:saved', () => { readCount(); readTeams(); });

// ---- 始め ----------------------------------------------------------------
// The page's band is written by the server, and this part's button is put in it
// here (the band's links and numbers stay nav.js's).
mountBand();
mountLayer();
readCount();
// The screens that show one team's own number read the whole answer too: the
// 動き column of the team definitions' list, and the activity screen's team.
const showsTeams = /^\/(teams|activity)/.test(location.pathname);
if (showsTeams) readTeams();
setInterval(() => {
  if (document.visibilityState !== 'visible') return;
  readCount();
  if (showsTeams) readTeams();
}, POLL);
document.addEventListener('visibilitychange', () => {
  if (document.visibilityState === 'visible') { readCount(); if (showsTeams) readTeams(); }
});
