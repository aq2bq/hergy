// lib/deliver.js — 変更を伝える (r-a8fa)。module の入口。
//
// Every screen carries this part, because the band is on every screen. The band
// counts the running members whose prompt is not the one hergy last handed them
// (GET /api/delivery/count), and the sheet hands the current prompt to the ones
// a person picks (POST /api/delivery).
//
// 帯のボタン・シート・下の知らせは Preact が描く（lib/deliver/view.js）。状態と
// サーバーとの口は lib/deliver/state.js。保存はシートを開かず、送りもしない:
// hergy:saved を聞いて、帯の数だけを読み直す。
//
// 器はこの 1 本が作る。帯のボタンの器は `.bar` の `.screens` / `.hq-search` の
// 直後に置き、`display: contents`（CSSOM。style 属性は CSP が拒む）で並びに
// 関わらせない。button そのものには付けない（支援技術から button と見えなく
// なるブラウザがある）。
import { html, render } from './preact.js';
import { BandButton, Layer } from './deliver/view.js';
import { close, opened, readCount, POLL } from './deliver/state.js';

// ---- 器 -----------------------------------------------------------------
// The search field of a headquarters stands right of the screens (spec 2 章),
// and the delivery button right of it: the two are one band's little row.
function mountBand() {
  const after = document.querySelector('.bar .hq-search') || document.querySelector('.bar .screens');
  if (!after) return null;
  const host = document.createElement('div');
  host.className = 'dlv-band';
  host.style.display = 'contents';
  after.insertAdjacentElement('afterend', host);
  render(html`<${BandButton} />`, host);
  return host;
}

function mountLayer() {
  const host = document.createElement('div');
  host.className = 'dlv-layer';
  document.body.appendChild(host);
  render(html`<${Layer} />`, host);
  return host;
}

// ---- 押す ----------------------------------------------------------------
// The mark on the activity screen's member row is the screen's own markup, and
// the screen answers it: activity/board.js calls openSheet (webui-components
// 2 章). What stands on this part is its own: the band's button, the sheet, and
// the one key that closes it.
document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape' && opened.value) {
    e.preventDefault();
    close();
  }
});

// A save never opens the sheet and never sends: it only says the band's number
// may have moved.
document.addEventListener('hergy:saved', () => readCount());

// ---- 始め ----------------------------------------------------------------
// The page's band is written by the server, and this part's button is put in it
// here (the band's links and numbers stay nav.js's).
mountBand();
mountLayer();
readCount();
setInterval(() => {
  if (document.visibilityState === 'visible') readCount();
}, POLL);
document.addEventListener('visibilitychange', () => {
  if (document.visibilityState === 'visible') readCount();
});
