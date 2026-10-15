/* hergy の「コピー」ボタン。この 1 つを frame が読み込む (CSP: default-src 'self')。
 *
 * 画面は <button type="button" data-copy="#yaml-src" data-copied="コピーしました"> を書く。
 * data-copy は、中身をクリップボードに入れる要素を指す CSS の選択子。data-copied は、
 * 入れたあとにボタンへ出す言葉 (locale から画面が書くので、ここに言葉は持たない)。
 * script が無くても、指す要素の中身は画面に出ている: 人が選んで写せる。
 */
document.addEventListener("click", function (event) {
  const button = event.target.closest("[data-copy]");
  if (!button) return;
  const source = document.querySelector(button.getAttribute("data-copy"));
  if (!source) return;
  const text = source.textContent;
  const show = function () {
    const was = button.textContent;
    button.textContent = button.getAttribute("data-copied") || was;
    button.disabled = true;
    setTimeout(function () {
      button.textContent = was;
      button.disabled = false;
    }, 1500);
  };
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(text).then(show, function () {});
  }
});