/* hergy の画面の小さな振る舞い。CSP (default-src 'self') なので、この 1 ファイルだけ。
 *
 * 書きかけ (C5, W10): [data-draft] の欄は sessionStorage に残し、欄が戻ったときに書き戻す。
 * 変えたら [data-change] を現す (C1)。[data-revert] は元の値 (data-origin) に戻す。
 * 選んだプロファイルの kind は、選び直すたびに横に出す (I7)。選んだ値が何をするかは、
 * 書く前に欄のそばに出す (T1): 選びが落ち着いてから欄の URL を読み直すので、下見は
 * URL に出る (消すの ?remove= と同じ)。
 * Esc は開いている欄 (#panel[data-parent]) を閉じる (B3)。保存が通った頁 (data-saved) では
 * その欄の書きかけを捨てる。script が無くても、欄はそのまま書けて送れる。
 */
(function () {
  function draftKey(el) {
    return "hergy:draft:" + (el.getAttribute("data-draft") || "");
  }

  function mark(form) {
    if (!form) return;
    var actions = form.querySelector("[data-change]");
    if (!actions) return;
    var changed = false;
    form.querySelectorAll("[data-draft]").forEach(function (el) {
      if (el.value !== (el.getAttribute("data-origin") || "")) changed = true;
    });
    actions.hidden = !changed;
  }

  function restore(root) {
    var scope = root && root.querySelectorAll ? root : document;
    scope.querySelectorAll("[data-draft]").forEach(function (el) {
      var form = el.closest("form");
      if (form && form.hasAttribute("data-saved")) {
        sessionStorage.removeItem(draftKey(el));
        mark(form);
        return;
      }
      var stored = sessionStorage.getItem(draftKey(el));
      if (stored !== null && stored !== el.value) el.value = stored;
      mark(form);
    });
  }

  // The kind of the profile a person picked stands beside the pick, as an
  // attribute of the profile and never inside the choice (I7).
  function kindBeside(select) {
    var out = select.parentElement && select.parentElement.querySelector("[data-kind-show]");
    if (!out) return;
    var opt = select.options[select.selectedIndex];
    out.textContent = (opt && opt.getAttribute("data-kind")) || "";
  }

  function showKinds(root) {
    var scope = root && root.querySelectorAll ? root : document;
    scope.querySelectorAll("select[data-kind-of]").forEach(kindBeside);
  }

  // The page's own title travels on #page (the tab is not inside the region the
  // server swaps, so htmx cannot carry it).
  function setTitle() {
    var page = document.getElementById("page");
    if (page && page.getAttribute("data-title")) {
      document.title = "hergy · " + page.getAttribute("data-title");
    }
  }

  document.addEventListener("input", function (event) {
    var el = event.target.closest("[data-draft]");
    if (!el) return;
    sessionStorage.setItem(draftKey(el), el.value);
    mark(el.closest("form"));
  });

  // A select fires change rather than input for the pick itself: keep the same
  // draft, show the picked profile's kind, and look at what the pick would do
  // before it is written (T1).
  document.addEventListener("change", function (event) {
    var el = event.target;
    if (!el.matches || !el.matches("select[data-draft]")) return;
    sessionStorage.setItem(draftKey(el), el.value);
    mark(el.closest("form"));
    kindBeside(el);
    if (el.matches("select[data-preview]")) lookAtPick(el);
  });

  // A person browsing the list of choices settles on one first, so the read
  // waits for the picking to stop. A swap or a write takes the pick's place, so
  // a line about a pick never stands over a value that is not there any more.
  var pickTimer = null;

  function lookAtPick(el) {
    forgetPick();
    pickTimer = window.setTimeout(function () {
      pickTimer = null;
      if (!el.isConnected) return;
      var target = new URL(el.getAttribute("data-preview"), window.location.href);
      target.searchParams.set(el.name, el.value);
      window.location.href = target.toString();
    }, 350);
  }

  function forgetPick() {
    if (pickTimer) window.clearTimeout(pickTimer);
    pickTimer = null;
  }

  document.addEventListener("click", function (event) {
    var revert = event.target.closest("[data-revert]");
    if (!revert) return;
    var form = revert.closest("form");
    form.querySelectorAll("[data-draft]").forEach(function (el) {
      sessionStorage.removeItem(draftKey(el));
      el.value = el.getAttribute("data-origin") || "";
    });
    mark(form);
    showKinds(form);
    forgetPick();
    leavePendings();
  });

  // A write that has been chosen but not written lives in the field's URL (the
  // removal of a worker, or a pick being looked at). 取り消す leaves it: the field
  // is read again without the pending state, so the line about it does not stand
  // over a value that is no longer chosen.
  function leavePendings() {
    var panel = document.getElementById("panel");
    var url = panel && panel.getAttribute("data-url");
    if (!url || !window.location.search) return;
    if (new URL(url, window.location.href).pathname !== window.location.pathname) return;
    window.location.href = url;
  }

  document.addEventListener("keydown", function (event) {
    if (event.key !== "Escape") return;
    var panel = document.getElementById("panel");
    if (panel && panel.getAttribute("data-parent")) {
      window.location.href = panel.getAttribute("data-parent");
    }
  });

  document.addEventListener("DOMContentLoaded", function () {
    restore(document);
    showKinds(document);
    setTitle();
  });
  document.addEventListener("htmx:afterSwap", function (event) {
    forgetPick();
    restore(event.target);
    showKinds(event.target);
    setTitle();
  });
  document.addEventListener("submit", forgetPick);

  // The activity redraws itself every few seconds, but only while its tab is
  // visible (設計 5.2): the server cannot know that, so the tick comes from the
  // one place that can. The element is looked up each time, so a swap does not
  // stop it.
  window.setInterval(function () {
    if (document.visibilityState !== "visible") return;
    var list = document.getElementById("activity-list");
    if (list && window.htmx) window.htmx.trigger(list, "hergy-tick");
  }, 5000);
})();
