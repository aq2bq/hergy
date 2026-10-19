// The band's behaviour (spec-nav.md 1.4, 1.5). It remembers where a person last
// stood in each screen and carries the changes of the screen they are leaving
// that were not saved yet. The page says what it is through window.hergyPage;
// nothing here talks to the server, and a page whose own script did not run
// still walks the band's links.
(() => {
  'use strict';

  const band = document.querySelector('.screens');
  if (!band) return;
  const key = band.dataset.key || '';

  // The page's own script runs after this one (both are deferred, and the page
  // is read second), so what the page is is read when it is needed, not here.
  const page = () => window.hergyPage || null;
  const mine = () => { const p = page(); return p && p.screen ? p.screen : ''; };

  // sessionStorage holds one place and one draft per screen, under this
  // project's own key, so two projects on one machine do not mix. A browser
  // that refuses to hold them still walks the links.
  const at = (what, screen) => 'hergy.' + key + '.' + what + '.' + screen;
  const read = (k) => { try { return window.sessionStorage.getItem(k); } catch (e) { return null; } };
  const write = (k, v) => { try { window.sessionStorage.setItem(k, v); } catch (e) { /* the band still links */ } };
  const drop = (k) => { try { window.sessionStorage.removeItem(k); } catch (e) { /* as above */ } };

  const scr = (name) => band.querySelector('.scr[data-screen="' + name + '"]');
  const setDot = (el, on) => {
    if (!el) return;
    const dot = el.querySelector('.dot');
    if (on && !dot) {
      const i = document.createElement('i');
      i.className = 'dot';
      el.appendChild(i);
    }
    if (!on && dot) dot.remove();
  };

  // ---- 居た所 (1.4) ----------------------------------------------------
  function remember() {
    const screen = mine();
    if (!screen) return;
    write(at('at', screen), location.pathname + location.search);
  }

  // The page moves the address bar (a filter, an opened row, a chosen file)
  // through these two calls: wrapping them is what keeps the place right
  // without the pages doing anything for it.
  const pushState = history.pushState.bind(history);
  const replaceState = history.replaceState.bind(history);
  history.pushState = function (...args) { pushState(...args); remember(); };
  history.replaceState = function (...args) { replaceState(...args); remember(); };

  // decorate points the band's links at the place each screen was left, and
  // shows the dot of a screen whose changes were carried over. The screen the
  // person is on is a span, not a link, so it is not among these.
  function decorate() {
    band.querySelectorAll('a.scr[data-screen]').forEach((a) => {
      const name = a.dataset.screen;
      const was = read(at('at', name));
      if (was && was.startsWith('/' + name)) a.setAttribute('href', was);
      setDot(a, !!read(at('draft', name)));
    });
  }

  // ---- 保存前の変更 (1.5) ----------------------------------------------
  // Only the band's own walk carries the changes: pressing a link here writes
  // the draft of the screen being left and marks the screen being entered, and
  // only that arrival reads it back. A reload, the back button, a typed URL and
  // hergy go without them — and a draft they left behind is dropped, so it
  // cannot come back later (R2). The mark is written even from a page that is
  // no screen (a 404), so a draft waiting for the screen it points at comes
  // back the same way from there.
  band.querySelectorAll('a.scr').forEach((a) => {
    a.addEventListener('click', () => {
      write(at('carry', a.dataset.screen), '1');
      const screen = mine();
      if (!screen) return;
      const p = page();
      if (p && p.dirty && p.dirty()) write(at('draft', screen), JSON.stringify(p.take()));
      else drop(at('draft', screen));
    });
  });

  document.addEventListener('hergy:dirty', (e) => setDot(scr(mine()), !!e.detail));

  window.hergyNav = {
    // takeDraft hands one screen's unsaved changes over when the band walked
    // here, once, and forgets them. It answers null when the band did not walk
    // here, or when the screen held nothing. The draft is dropped either way:
    // one this page did not take is one nobody may put back (R2).
    takeDraft(screen) {
      const raw = read(at('draft', screen));
      const carried = !!read(at('carry', screen));
      drop(at('carry', screen));
      drop(at('draft', screen));
      if (!raw || !carried) return null;
      try { return JSON.parse(raw); } catch (e) { return null; }
    },
    // setCount is how a page keeps the band's number the same as its own when a
    // profile or a role is added or removed (1.1).
    setCount(screen, n) {
      const el = scr(screen);
      const num = el && el.querySelector('.n');
      if (num) num.textContent = String(n);
    },
  };

  // Both scripts have run by the time the document is parsed, so the page
  // knows what it is: remember the place, and point the links.
  document.addEventListener('DOMContentLoaded', () => { remember(); decorate(); });
})();