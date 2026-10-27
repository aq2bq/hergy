// activity/main.js — 活動の画面の入口（webui-preact-design 4 章）。
//
// mount と、ページの外への約束だけを持つ。描くのは board.js、状態は
// state.js。ここも DOM を書かない（5 章 1）。
import { html, render } from '../vendor/preact.js';
import { Board } from './board.js';
import { reading, refresh, pick, stopTeam, POLL } from './state.js';

const list = document.getElementById('list');

// The page says what it is through window.hergyPage (nav.js reads it, 5 章 7).
// settled is true once the first reading has answered (5 章 4): the e2e walks
// wait for it, and it stays true — a ten-second re-reading does not put the
// screen back to loading, or every walk would stop once a poll.
const page = { screen: 'activity', settled: false };
window.hergyPage = page;

if (list) {
  list.addEventListener('click', (e) => {
    const stop = e.target.closest('[data-hq="stop"]');
    if (stop) { stopTeam(stop.dataset.team); return; }
    const pickEl = e.target.closest('[data-pick]');
    if (pickEl) pick(pickEl.dataset.pick);
  });
  // The rows are buttons, so a press with the keyboard is the button's own
  // click; a row that is not one (the figure's tile) still answers Enter and
  // space here, as it did before the rewrite.
  list.addEventListener('keydown', (e) => {
    if (e.key !== 'Enter' && e.key !== ' ') return;
    const el = e.target.closest('[data-pick]');
    if (!el) return;
    e.preventDefault();
    pick(el.dataset.pick);
  });

  render(html`<${Board} />`, list);

  // hq.js announces every reading of its own (hergy:hq); one that arrives while
  // this screen is reading is left for the next tick, as before.
  document.addEventListener('hergy:hq', () => { refresh(); });

  // ---- the ten-second reading ------------------------------------------
  const first = refresh();
  Promise.resolve(first).finally(() => { page.settled = true; });
  setInterval(() => { if (document.visibilityState === 'visible') refresh(); }, POLL);
  document.addEventListener('visibilitychange', () => { if (document.visibilityState === 'visible') refresh(); });

  // Leaving the page drops the reading that is out (5 章 3): its answer is not
  // this screen's any more.
  window.addEventListener('pagehide', () => { reading.abort(); });
}
