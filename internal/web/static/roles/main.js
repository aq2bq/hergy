// roles/main.js — ロールの画面の入口（webui-preact-design 4 章）。
//
// mount と、押し込み・住所・帯への約束だけを持つ。描くのは view.js / assign.js /
// yaml.js、状態と書き込みは state.js。ここも DOM を書かない（5 章 1）。規則の
// 文書は共通部品（lib/ruledoc.js）が、その押し込みは部品が自分の器で受ける。
import { html, render as draw } from '../vendor/preact.js';
import { insertTab } from '../lib/yamltab.js';
import { mount } from '../lib/ruledoc.js';
import {
  view, tabsEl, byUid, host, onDraw, boot, dirty, take,
  pressToggle, pressFilter, pressNew, pressPickPlace, pressPmode, pressDup, pressCopy,
  pressDel, pressDelNo, pressDiscard, pressGoto, pressRoleHi, pressTab, pressYfile,
  pressYamlRevert, pressSearch, pickRole, doSave, doDelete, doAssignSave,
} from './state.js';
import { Tabs, App } from './view.js';

if (view) {
  // The page says what it is through window.hergyPage (nav.js reads it, 5 章 7).
  // settled is true once the first reading is drawn; every saved document's check
  // is part of that reading.
  const page = { screen: 'roles', dirty, take, settled: false };
  window.hergyPage = page;

  onDraw(() => {
    draw(html`<${Tabs} />`, tabsEl);
    draw(html`<${App} />`, view);
  });

  // The shared part listens on the view first: its presses are its own (the
  // rules, the sections, the preview), and what it does not answer comes on to
  // this screen's list below.
  mount(view, host);

  tabsEl.addEventListener('click', (e) => {
    const b = e.target.closest('[data-act=tab]');
    if (b) pressTab(b.dataset.t);
  });

  view.addEventListener('keydown', (e) => {
    if ((e.key === 'Enter' || e.key === ' ') && e.target.matches('.sum')) { e.preventDefault(); e.target.click(); }
    if (e.key === 'Tab' && e.target.matches('.ytext') && !e.shiftKey) {
      e.preventDefault();
      insertTab(e.target);
    }
  });

  view.addEventListener('input', (e) => {
    if (e.target.id === 'q') pressSearch(e.target.value);
  });

  view.addEventListener('change', (e) => {
    const t = e.target;
    if (t.matches('.pickrole')) pickRole(t.dataset.t, t.dataset.i, t.value);
  });

  view.addEventListener('click', async (e) => {
    const b = e.target.closest('[data-act]');
    if (!b) return;
    const act = b.dataset.act;
    const panel = b.closest('.panel');
    const roleEl = b.closest('.role');
    const r = panel ? byUid(panel.dataset.uid) : roleEl ? byUid(roleEl.dataset.uid) : null;
    const l = panel && panel.dataset.l;

    switch (act) {
      case 'toggle': if (roleEl) pressToggle(roleEl.dataset.uid); break;
      case 'filter': pressFilter(b.dataset.f); break;
      case 'new': pressNew(); break;
      case 'pick-place': if (r) pressPickPlace(r, l, b.dataset.x); break;
      case 'save': if (r) await doSave(r, l); break;
      case 'pmode': if (panel) pressPmode(panel, b.dataset.v); break;
      case 'dup': if (r) pressDup(r, l); break;
      case 'copy': if (r) pressCopy(r, l); break;
      case 'del': if (r) pressDel(r, l); break;
      case 'del-no': if (r) pressDelNo(r, l); break;
      case 'del-yes': if (r) await doDelete(r, l); break;
      case 'discard': if (r) pressDiscard(r, l); break;
      case 'goto': pressGoto(b.dataset.t, b.dataset.m, b.dataset.i); break;
      case 'role-hi': pressRoleHi(b.dataset.n); break;
      // The tabs stand outside the screen, but a button in the screen may name
      // one (the 一覧へ of the empty YAML tab), so it is answered here too.
      case 'tab': pressTab(b.dataset.t); break;
      case 't-save': await doAssignSave(b.closest('.team').dataset.t); break;
      case 'yfile': pressYfile(b.dataset.key); break;
      case 'yaml-jump': window.hergyYaml.jump(b.closest('.yed'), Number(b.dataset.line)); break;
      case 'yaml-revert': {
        const el = b.closest('.yed');
        pressYamlRevert(el);
        window.hergyYaml.revert(el);
        break;
      }
      case 'yaml-save': window.hergyYaml.save(b.closest('.yed')); break;
      default: break;
    }
  });

  // boot reads the address and the saved documents; the band's promise (what the
  // screen before this one left unsaved) is part of it.
  boot().catch(() => {
    // A screen that could not read its documents draws what it has: the walk
    // says what is missing where it looks for it.
  }).finally(() => { page.settled = true; });
}
