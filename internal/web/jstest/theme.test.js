import { expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { runInNewContext } from 'node:vm';

const code = readFileSync(new URL('../static/lib/theme-init.js', import.meta.url), 'utf8');
function boot(value = null, failure = '') {
  const attrs = new Map(), events = {};
  const storage = {
    getItem: () => { if (failure === 'read') throw new Error('blocked'); return value; },
    setItem: (_key, next) => { if (failure === 'write') throw new Error('full'); value = next; },
    removeItem: () => { value = null; },
  };
  const window = { localStorage: storage, dispatchEvent() {}, addEventListener: (name, fn) => { events[name] = fn; } };
  runInNewContext(code, { window, localStorage: storage, Event,
    document: { documentElement: { setAttribute: (key, val) => attrs.set(key, val), removeAttribute: key => attrs.delete(key) } } });
  return { theme: window.hergyTheme, attrs, value: () => value,
    fail: next => { failure = next; }, storage: next => { value = next; events.storage({ key: 'hergy.theme', storageArea: storage }); } };
}

test('saved preferences restore, invalid values and OS mode remove overrides', () => {
  for (const mode of ['light', 'dark']) expect(boot(mode).attrs.get('data-theme')).toBe(mode);
  for (const mode of [null, 'system', 'garbage']) expect(boot(mode).attrs.has('data-theme')).toBe(false);
  const app = boot('dark'); app.theme.choose('system');
  expect(app.value()).toBe('system'); expect(app.attrs.has('data-theme')).toBe(false);
});
test('read failure follows OS and a successful later choice recovers', () => {
  const app = boot('dark', 'read');
  expect(app.theme.state).toEqual({ mode: 'system', failed: true });
  app.fail(''); app.theme.choose('light');
  expect(app.value()).toBe('light'); expect(app.theme.state).toEqual({ mode: 'light', failed: false });
});
test('failed writes discard the old override; other tabs and deletion restore current storage', () => {
  const app = boot('dark', 'write'); app.theme.choose('light');
  expect(app.theme.state).toEqual({ mode: 'system', failed: true });
  expect(app.attrs.has('data-theme')).toBe(false); expect(app.value()).toBe(null);
  app.fail(''); app.storage('dark'); expect(app.attrs.get('data-theme')).toBe('dark');
  app.storage(null); expect(app.theme.state).toEqual({ mode: 'system', failed: false });
});
