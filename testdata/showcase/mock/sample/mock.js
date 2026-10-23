// The e2e walk's mock (tools/showcase --mock): it adds one element to the real
// screen, so the walk sees that the prototype's file stands in the page. It
// writes no style attribute, because the serve's CSP refuses one.
(() => {
  const mark = document.createElement('p');
  mark.className = 'mock-mark';
  mark.textContent = 'mock が差し込んだ行';
  const view = document.querySelector('#view');
  if (view) view.appendChild(mark);
})();
