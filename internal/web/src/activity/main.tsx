// activity/main.tsx — アクティビティの画面の入口（webui-preact-design 4 章）。
//
// mount と、ページの外への受け渡しだけを持つ。描くのは board.js、状態は
// state.js。押し込みは部品が持つ（board.js の onClick）。ここも DOM を書かない
// （5 章 1）。
import { render } from '../lib/preact.js';
import { Board } from './board.js';
import { reading, refresh, POLL } from './state.js';

const list = document.getElementById('list');

// The page says what it is through window.hergyPage (nav.js reads it, 5 章 7).
// settled is true once the first reading has answered (5 章 4): the e2e walks
// wait for it, and it stays true — a ten-second re-reading does not put the
// screen back to loading, or every walk would stop once a poll.
const page = { screen: 'activity', settled: false };
window.hergyPage = page;

if (list) {
  render(<Board />, list);

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
