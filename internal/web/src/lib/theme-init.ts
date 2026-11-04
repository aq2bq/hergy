// Runs synchronously before stylesheets. Keep this entry independent of modules.
export type ThemeMode = 'light' | 'dark' | 'system';
export type ThemeState = { mode: ThemeMode; failed: boolean };
export type ThemeController = { readonly state: ThemeState; choose: (mode: ThemeMode) => void };
declare global { interface Window { hergyTheme?: ThemeController } }

const key = 'hergy.theme';
let state: ThemeState = { mode: 'system', failed: false };
const modeOf = (value: string | null): ThemeMode => value === 'light' || value === 'dark' ? value : 'system';
function apply(mode: ThemeMode, failed = false): void {
  state = { mode, failed };
  if (mode === 'system') document.documentElement.removeAttribute('data-theme');
  else document.documentElement.setAttribute('data-theme', mode);
  window.dispatchEvent(new Event('hergy:theme'));
}
function restore(): void {
  try { apply(modeOf(localStorage.getItem(key))); }
  catch { apply('system', true); }
}
window.hergyTheme = {
  get state() { return { ...state }; },
  choose(mode) {
    try {
      localStorage.setItem(key, mode);
      apply(mode);
    } catch {
      // Remove a stale preference when possible; blocked storage may refuse both.
      try { localStorage.removeItem(key); } catch { /* The page still follows the OS. */ }
      apply('system', true);
    }
  },
};
window.addEventListener('storage', (event) => {
  if (event.key !== null && event.key !== key) return;
  try { if (event.storageArea !== localStorage) return; }
  catch { apply('system', true); return; }
  restore();
});
restore();
