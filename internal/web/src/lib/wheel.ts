// r-ddb7 (d-656c): the wheel over an input nobody is typing in sends the page,
// not the input. Only an unfocused textarea's vertical wheel is taken; a focused
// input keeps its own scrolling, and Ctrl/⌘ (zoom, pinch) and horizontal wheels
// are left to the browser.
const LINE = 16;

// Inside a modal the body of the modal takes the place of the page; outside it,
// the page itself is what the wheel sends, whatever scrollers stand around the
// input (r-ddb7, ac-6dec).
function scrollHostOf(target: HTMLElement): HTMLElement | null {
  const dialog = target.closest<HTMLElement>('dialog[open]');
  if (!dialog) return null;
  for (let node: HTMLElement | null = target.parentElement; node; node = node.parentElement) {
    const style = getComputedStyle(node);
    if ((style.overflowY === 'auto' || style.overflowY === 'scroll') && node.scrollHeight > node.clientHeight) return node;
    if (node === dialog) return dialog;
  }
  return dialog;
}

// One wheel's amount in pixels: a line wheel is a line, and a page wheel is the
// height of what it moves, the way the browser's own scroll is (lead's condition 2).
function pixels(event: WheelEvent, height: number): number {
  if (event.deltaMode === 1) return event.deltaY * LINE;
  if (event.deltaMode === 2) return event.deltaY * (height || LINE);
  return event.deltaY;
}

export function installWheel(): void {
  document.addEventListener('wheel', (event) => {
    if (event.ctrlKey || event.metaKey) return;
    if (Math.abs(event.deltaY) < Math.abs(event.deltaX)) return;
    const target = event.target;
    if (!(target instanceof HTMLTextAreaElement)) return;
    if (document.activeElement === target) return;
    event.preventDefault();
    const host = scrollHostOf(target);
    const amount = pixels(event, host ? host.clientHeight : window.innerHeight);
    if (host) host.scrollTop += amount;
    else window.scrollBy(0, amount);
  }, { passive: false, capture: true });
}
