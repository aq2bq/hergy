// teams/main.js — チーム定義の画面の入口（webui-preact-design 4 章）。
//
// mount と、住所・帯への約束だけを持つ。描くのは view.js（一覧・パネル・YAML）と
// canvas.js（トポロジー）、状態は state.js の signal、押し込みは部品が自分の
// onClick / onInput / onKeyDown から state.js の関数を直接呼ぶ（依頼書 2）。
// ここも DOM を書かない（5 章 1）。
import { html, effect, render as draw } from '../vendor/preact.js';
import { setCount, setDirty } from '../lib/nav.js';
import {
  view, tabsEl, state, boot, take, dirty, tellDirty, savedCount, setStick, readAddress,
  narrow, thin,
} from './state.js';
import { Tabs, App } from './view.js';
import { mountCanvas, fitYboxes } from './canvas.js';

if (view) {
  // The page says what it is through window.hergyPage (nav.js reads it, 5 章 7).
  // settled turns true once the reads that follow the first frame have answered
  // (the roles, the HQ's roles and profiles, the first checks).
  const page = { screen: 'teams', dirty, settled: false, take, tellDirty };
  window.hergyPage = page;

  // The two trees the page holds stand where the server drew them, and stay
  // where they are: a signal they read draws them again (5 章 2).
  draw(html`<${Tabs} />`, tabsEl);
  draw(html`<${App} />`, view);

  mountCanvas(view);

  // The band's number is the list's own — it counts the definitions that are
  // saved (spec-nav 1.1) — and its dot follows whether the files differ from
  // what the screen holds (nav.js 1.5).
  effect(() => { setCount('teams', savedCount()); });
  effect(() => { setDirty(dirty()); });

  let wasNarrow = narrow();
  let rt = null;
  window.addEventListener('resize', () => {
    clearTimeout(rt);
    rt = setTimeout(() => {
      setStick();
      if (narrow() !== wasNarrow) { wasNarrow = narrow(); thin.value = wasNarrow; }
      else { fitYboxes(); }
    }, 160);
  });
  // Where the save's band stands is a matter of where the page is, so the box
  // is capped again as the page moves (監査 R2). The work is left to one frame.
  let ft = false;
  window.addEventListener('scroll', () => {
    if (ft || state.value.tab !== 'list') return;
    ft = true;
    requestAnimationFrame(() => { ft = false; fitYboxes(); });
  }, { passive: true });
  // The back button comes back to the place the address names: the rows it
  // opens, the rule it opens and the file it chose are read again.
  window.addEventListener('popstate', () => { readAddress(); });

  boot().catch(() => {
    // A screen that could not read its documents draws what it has: the walk
    // says what is missing where it looks for it.
  }).finally(() => { page.settled = true; });
}
