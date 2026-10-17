/* hergy の画面の小さな振る舞い。CSP (default-src 'self') なので、この 1 ファイルだけ。
 *
 * 書きかけ (C5, W10): [data-draft] の欄は sessionStorage に残し、欄が戻ったときに書き戻す。
 * 変えたら [data-change] を現す (C1)。[data-revert] は元の値 (data-origin) に戻す。
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

  document.addEventListener("input", function (event) {
    var el = event.target.closest("[data-draft]");
    if (!el) return;
    sessionStorage.setItem(draftKey(el), el.value);
    mark(el.closest("form"));
  });

  document.addEventListener("click", function (event) {
    var revert = event.target.closest("[data-revert]");
    if (!revert) return;
    var form = revert.closest("form");
    form.querySelectorAll("[data-draft]").forEach(function (el) {
      sessionStorage.removeItem(draftKey(el));
      el.value = el.getAttribute("data-origin") || "";
    });
    mark(form);
  });

  document.addEventListener("keydown", function (event) {
    if (event.key !== "Escape") return;
    var panel = document.getElementById("panel");
    if (panel && panel.getAttribute("data-parent")) {
      window.location.href = panel.getAttribute("data-parent");
    }
  });

  document.addEventListener("DOMContentLoaded", function () {
    restore(document);
  });
  document.addEventListener("htmx:afterSwap", function (event) {
    restore(event.target);
  });
})();
