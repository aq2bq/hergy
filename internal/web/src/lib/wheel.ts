// r-ddb7 (d-656c): the wheel over an input nobody is typing in sends the page,
// not the input. Only an unfocused textarea's vertical wheel is taken; a focused
// input keeps its own scrolling, and Ctrl/⌘ (zoom, pinch) and horizontal wheels
// are left to the browser.
const LINE = 16;

function scrollHostOf(target: HTMLElement): HTMLElement | null {
  // Inside a modal, the body of the modal takes the place of the page.
  const dialog = target.closest('dialog[open]');
  for (let node: HTMLElement | null = target.parentElement; node; node = node.parentElement) {
    const style = getComputedStyle(node);
    if ((style.overflowY === 'auto' || style.overflowY === 'scroll') && node.scrollHeight > node.clientHeight) return node;
    if (node === dialog) return node;
  }
  return null;
}

function pixels(event: WheelEvent, host: HTMLElement): number {
  if (event.deltaMode === 1) return event.deltaY * LINE;
  if (event.deltaMode === 2) return event.deltaY * (host.clientHeight || LINE);
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
    const amount = pixels(event, target);
    const host = scrollHostOf(target);
    if (host) host.scrollTop += amount;
    else window.scrollBy(0, amount);
  }, { passive: false, capture: true });
}
