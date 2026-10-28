// roles/main.js — ロールの画面の入口（webui-preact-design 4 章）。
//
// mount と、住所・帯への約束だけを持つ。描くのは view.js / assign.js / yaml.js、
// 状態は state.js の signal、押し込みは部品が自分の onClick / onInput から
// state.js の関数を直接呼ぶ。ここも DOM を書かない（5 章 1）。
import { html, effect, render } from '../vendor/preact.js';
import { setCount } from '../lib/nav.js';
import { view, tabsEl, dirty, take, savedRoles, boot } from './state.js';
import { Tabs, App } from './view.js';

if (view) {
  // The page says what it is through window.hergyPage (nav.js reads it, 5 章 7).
  // settled is true once the first reading is drawn; every saved document's check
  // is part of that reading.
  const page = { screen: 'roles', dirty, take, settled: false };
  window.hergyPage = page;

  // The two trees the page holds stand where the server drew them, and stay
  // where they are: a signal they read draws them again (5 章 2).
  render(html`<${Tabs} />`, tabsEl);
  render(html`<${App} />`, view);

  // The band's number is the list's own: it counts the definitions that are saved
  // (spec-nav 1.1).
  effect(() => { setCount('roles', savedRoles().length); });

  // boot reads the address and the saved documents; the band's promise (what the
  // screen before this one left unsaved) is part of it.
  boot().catch(() => {
    // A screen that could not read its documents draws what it has: the walk
    // says what is missing where it looks for it.
  }).finally(() => { page.settled = true; });
}
