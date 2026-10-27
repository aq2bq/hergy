// teams/main.js — チーム定義の画面の入口（webui-preact-design 4 章）。
//
// mount と、押し込み・住所・帯への約束だけを持つ。描くのは view.js（一覧・パネル・
// YAML）と canvas.js（トポロジー）、状態と書き込みは state.js。ここも DOM を書か
// ない（5 章 1）。
import { html, render as draw } from '../vendor/preact.js';
import { insertTab } from '../lib/yamltab.js';
import { rulesOf, findRule, clone, mount as mountRules, state as ruleState } from '../lib/ruledoc.js';
import {
  view, tabsEl, state, render, onDraw, boot, take, tellDirty, dkey, byKey, byName,
  addDraft, dropDraft, checkTeam, checkAgree, syncTeam, saveTeam, saveAgree, doDelete,
  syncURL, readURL, jumpWantRule, jumpToRule, setStick, other, orderRows, firstIndexOf,
  forgetUndo, canvasUndoFromToast, recs, applyOps, pushOp, debounce, cloneTeam, theName,
  narrow, GROUPED, CURRENT, host,
} from './state.js';
import { Tabs, App } from './view.js';
import { mountCanvas, selectPick, canvasReset, fitYboxes } from './canvas.js';

if (view) {
  // The page says what it is through window.hergyPage (nav.js reads it, 5 章 7).
  // settled turns true once the reads that follow the first frame have answered
  // (the roles, the HQ's roles and profiles, the first checks).
  window.hergyPage = { screen: 'teams', dirty: false, settled: false, take, tellDirty };

  onDraw(() => {
    draw(html`<${Tabs} />`, tabsEl);
    draw(html`<${App} />`, view);
  });

  // The shared part listens on the view first: its presses are its own (the
  // rules, the sections, the preview), and what it does not answer comes on to
  // this screen's list below.
  mountRules(view, host);
  mountCanvas(view);
  render();

  // The presses on the page land here, once. The canvas answers its own
  // presses first (it stands inside #view), and what it does not answer comes
  // on to this screen's list.
  const wrapEl = document.querySelector('.wrap');
  wrapEl.addEventListener('click', async (e) => {
    // A button of the headquarters' 動き column (start, stop) acts on the
    // definition, not on the row: it must not open or close the row (r-5b96
    // audit 1).
    if (e.target.closest('[data-hq]')) return;
    if (e.target.closest('.in-mission, .in-name')) return;
    const b = e.target.closest('[data-act]');
    if (!b) return;
    const act = b.dataset.act;
    const row = b.closest('.tdef'), n = row && row.dataset.name;
    if (act === 'cv-undo') { canvasUndoFromToast(); return; }
    const tp = b.closest('.panel.team-p'), td = tp && byKey(tp.dataset.key);
    const ap = b.closest('.panel.agree-p'), ar = ap && byKey(ap.dataset.key);
    const r = row ? byName(n) : null;
    switch (act) {
      case 'tab': state.tab = b.dataset.t; render(); break;
      case 'toggle': if (state.open.has(n)) state.open.delete(n); else state.open.add(n); render(); break;
      // The list's own toolbar adds a definition from the scaffold, and each row
      // duplicates itself, copies itself into the other place, or goes away
      // (spec-teams B1-B5).
      case 'new': await addDraft('new', 'project', '', null, GROUPED ? CURRENT : ''); break;
      case 'dup': if (r) await addDraft('dup', r.place, theName(r.name), { place: r.place, name: r.name }, r.project); break;
      case 'copy': if (r) await addDraft('copy', other(r.place), r.name, { place: r.place, name: r.name }, r.project); break;
      case 'pick-place': {
        if (!r || !r.draftRow || r.place === b.dataset.x) break;
        // The row keeps its place in the list while its place changes
        // (spec-teams B1, N4).
        const at = firstIndexOf((x) => x === r);
        recs.delete(dkey(r));
        r.place = b.dataset.x;
        r.key = dkey(r);
        recs.set(r.key, r);
        orderRows(r, at);
        canvasReset(r.key);
        // The base is read again: the other place's role documents and profile
        // are not the ones the row stood on (spec-teams B2).
        r.seeded = false;
        r.team.saved = cloneTeam(null); r.team.draft = cloneTeam(null); r.savedTurns = [];
        r.agree.saved = null;
        await checkTeam(r);
        state.focusAsk = { dom: r.dom, sel: `[data-act=pick-place][data-x="${b.dataset.x}"]` };
        render();
        break;
      }
      case 'discard': if (r) dropDraft(r); break;
      case 'del': if (r) { state.confirm['del' + dkey(r)] = 1; render(); } break;
      case 'del-no': if (r) { delete state.confirm['del' + dkey(r)]; render(); } break;
      case 'del-yes': if (r) await doDelete(r); break;
      // A press on a line of the preview picks what the line stands for, the way
      // a press on the canvas's own point does (spec-teams B7). A mission line
      // puts the cursor in the mission field of the attribute column.
      case 'tyline': if (td) {
        if (b.dataset.f === 'mission') {
          selectPick(td, null, 'attr');
          state.focusAsk = { dom: td.dom, sel: '[data-cv-f="mission"]' };
          render();
          break;
        }
        if (b.dataset.node) selectPick(td, { type: 'node', id: b.dataset.node }, 'attr');
        else if (b.dataset.worker != null) selectPick(td, { type: 'worker', i: +b.dataset.worker }, 'attr');
        else if (b.dataset.a) selectPick(td, { type: 'edge', k: [b.dataset.a, b.dataset.b].sort().join('|') }, 'attr');
        else selectPick(td, null, 'attr');
      } break;
      case 'vpick':
        if (b.dataset.nameField) { state.focusAsk = { dom: n, sel: '.in-name' }; render(); break; }
        if (td) {
          if (b.dataset.node) selectPick(td, { type: 'node', id: b.dataset.node }, 'attr');
          else if (b.dataset.worker != null) selectPick(td, { type: 'worker', i: +b.dataset.worker }, 'attr');
        }
        break;
      case 'vrule': {
        // A warning about the team's own agreement opens that rule (spec-teams
        // 1.4.5, 監査 R9) in the ルール tab, where a person can see it, with the
        // cursor in its body (監査 R12).
        const r2 = byName(b.dataset.name);
        if (!r2) break;
        state.open.add(r2.dom);
        state.dtab[r2.dom] = 'agree';
        const hit = rulesOf(r2.agree.draft).find((x) => String(x.id) === b.dataset.id);
        render();
        checkAgree(r2);
        syncURL();
        if (hit) jumpToRule(r2, hit._k);
        break;
      }
      case 't-save': if (td) saveTeam(td); break;
      case 't-revert': if (td) { td.ops = []; canvasReset(dkey(td)); forgetUndo(td); state.confirm = {}; syncTeam(td); } break;
      case 'to-yaml': state.tab = 'yaml'; state.yfile = `${b.dataset.key}.team`; render(); window.scrollTo(0, 0); break;
      case 'dtab': if (r) { state.dtab[n] = b.dataset.t; render(); if (b.dataset.t === 'agree') checkAgree(r); } break;
      case 'yfile': state.yfile = b.dataset.file; render(); break;
      case 'yaml-jump': window.hergyYaml.jump(b.closest('.yed'), Number(b.dataset.line)); break;
      case 'yaml-revert': { const sec = b.closest('.yed'); delete state.ytext[sec.dataset.key]; window.hergyYaml.revert(sec); render(); break; }
      case 'yaml-save': window.hergyYaml.save(b.closest('.yed')); break;
      case 'save': if (ar) saveAgree(ar); break;
      case 'revert': if (ar) { ar.agree.draft = clone(ar.agree.saved); checkAgree(ar); render(); } break;
      case 'pmode': if (ar) { state.pmode[dkey(ar)] = b.dataset.v; render(); } break;
      default: break;
    }
    // The list's tab, the open rows, the open rule and the chosen file ride in
    // the address (spec-teams 2 章): a reload comes back to the same place.
    if (act === 'tab' || act === 'toggle' || act === 'to-yaml' || act === 'yfile' || act === 'dtab') syncURL();
  });

  view.addEventListener('input', (e) => {
    const t = e.target;
    if (t.matches('.in-mission')) {
      const r = byName(t.closest('.tdef').dataset.name);
      if (!r) return;
      const text = t.value.replace(/\n/g, ' ');
      r.ops = r.ops.filter((o) => o.kind !== 'mission');
      if (text !== r.team.saved.mission) pushOp(r, { kind: 'mission', text });
      r.team.draft = applyOps(r.team.saved, r.ops);
      render();
      checkTeam(r);
      return;
    }
    if (t.matches('.in-name')) {
      // The name a definition that is not there yet takes is checked, and the
      // files a save would write are read again: the server owns both (spec-teams
      // B2). The field keeps its place and its cursor while the person types.
      const r = byName(t.closest('.tdef').dataset.name);
      if (!r) return;
      r.name = t.value;
      debounce('n' + r.dom, () => checkTeam(r));
    }
  });

  // 入力の途中でも、ルールのまとめと Markdown を行番号に合わせて引き直す
  view.addEventListener('focusin', (e) => {
    const t = e.target;
    if (!t.closest('.pedit')) return;
    const panel = t.closest('.panel.agree-p');
    const r = panel && byKey(panel.dataset.key);
    if (!r) return;
    const ruleEl = t.closest('.rule');
    const x = ruleEl ? findRule(r.agree.draft, +ruleEl.dataset.k) : null;
    ruleState.pfocus[r.uid + r.place] = { k: x ? x._k : 0 };
  });
  view.addEventListener('scroll', (e) => {
    if (e.target.matches?.('.ytext')) {
      const g = e.target.closest('.code');
      const gutter = g && g.querySelector('.gutter');
      if (gutter) gutter.scrollTop = e.target.scrollTop;
    }
  }, true);
  view.addEventListener('keydown', (e) => {
    // A point or a line of the canvas answers its own keys (canvas.js); this
    // screen answers the row, the list and the validate column.
    if ((e.key === 'Enter' || e.key === ' ') && !e.target.closest('.cv-canvas') && e.target.matches('.sum, .vitem, [role=button]')) { e.preventDefault(); e.target.dispatchEvent(new MouseEvent('click', { bubbles: true })); }
    if (e.key === 'Tab' && e.target.matches('.ytext') && !e.shiftKey) { e.preventDefault(); insertTab(e.target); }
    if (e.key === 'Enter' && e.target.matches('.in-mission, .in-name')) e.preventDefault();
  });

  let wasNarrow = narrow();
  let rt = null;
  window.addEventListener('resize', () => {
    clearTimeout(rt);
    rt = setTimeout(() => {
      setStick();
      if (narrow() !== wasNarrow) { wasNarrow = narrow(); render(); }
      else { fitYboxes(); }
    }, 160);
  });
  // Where the save's band stands is a matter of where the page is, so the box
  // is capped again as the page moves (監査 R2). The work is left to one frame.
  let ft = false;
  window.addEventListener('scroll', () => {
    if (ft || state.tab !== 'list') return;
    ft = true;
    requestAnimationFrame(() => { ft = false; fitYboxes(); });
  }, { passive: true });
  window.addEventListener('popstate', () => { state.open = new Set(); readURL(); render(); jumpWantRule(); });

  boot();
}
