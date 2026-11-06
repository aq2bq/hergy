import type { ThemeMode } from './theme-init.js';

const names: Record<ThemeMode, string> = { light: 'ライト', dark: 'ダーク', system: 'OS に従う' };
function mount(): void {
  const theme = window.hergyTheme;
  const control = document.querySelector<HTMLElement>('.theme-control');
  if (!theme || !control) return;
  const trigger = control.querySelector<HTMLButtonElement>('.theme-trigger')!;
  const panel = control.querySelector<HTMLElement>('.theme-panel')!;
  const menu = control.querySelector<HTMLElement>('[role="menu"]')!;
  const notice = control.querySelector<HTMLElement>('[role="status"]')!;
  const items = [...menu.querySelectorAll<HTMLButtonElement>('[role="menuitemradio"]')];
  let opened = false;
  function update(): void {
    const { mode, failed } = theme!.state;
    trigger.disabled = false;
    trigger.dataset.mode = mode;
    trigger.title = `明暗を選ぶ（現在: ${names[mode]}）`;
    trigger.setAttribute('aria-label', trigger.title);
    trigger.setAttribute('aria-expanded', String(opened));
    for (const item of items) item.setAttribute('aria-checked', String(item.dataset.mode === mode));
    menu.hidden = !opened;
    notice.hidden = !failed;
    panel.hidden = !opened && !failed;
  }
  function close(returnFocus = false): void {
    opened = false; update();
    if (returnFocus) trigger.focus({ preventScroll: true });
  }
  function open(): void {
    opened = true; update();
    items.find(item => item.dataset.mode === theme!.state.mode)?.focus({ preventScroll: true });
  }
  trigger.addEventListener('click', () => { if (opened) close(); else open(); });
  trigger.addEventListener('keydown', (event) => {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') { event.preventDefault(); event.stopPropagation(); open(); }
  });
  for (const item of items) item.addEventListener('click', () => {
    const mode = item.dataset.mode;
    if (mode !== 'light' && mode !== 'dark' && mode !== 'system') return;
    theme.choose(mode); close(true);
  });
  control.addEventListener('keydown', (event) => {
    if (!opened || event.altKey || event.ctrlKey || event.metaKey || event.isComposing) return;
    const at = items.indexOf(document.activeElement as HTMLButtonElement);
    if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); close(true); }
    else if (event.key === 'Tab') close();
    else if (['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) {
      event.preventDefault();
      const index = event.key === 'Home' ? 0 : event.key === 'End' ? items.length - 1
        : (at + (event.key === 'ArrowDown' ? 1 : -1) + items.length) % items.length;
      items[index].focus({ preventScroll: true });
    }
  });
  document.addEventListener('click', (event) => { if (!control.contains(event.target as Node)) close(); });
  control.addEventListener('focusout', (event) => { if (!control.contains(event.relatedTarget as Node | null)) close(); });
  window.addEventListener('hergy:theme', update);
  update();
}
mount();
