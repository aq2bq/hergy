import { expect, test } from 'bun:test';
import { act } from 'preact/test-utils';
import { h, render } from '../src/lib/preact';
import { ProseEditor } from '../src/lib/prose-editor';

test('prose measures after a hidden parent opens, new text arrives and fonts finish', async () => {
 const priorObserver = globalThis.ResizeObserver;
 const sizes = ['clientWidth', 'scrollHeight'].map((key) => Object.getOwnPropertyDescriptor(HTMLElement.prototype, key));
 const priorFonts = Object.getOwnPropertyDescriptor(document, 'fonts');
 const fonts = new EventTarget(), observers = new Set();
 let width = 0, height = 0, reads = 0;
 Object.defineProperty(HTMLElement.prototype, 'clientWidth', { configurable: true, get: () => width });
 Object.defineProperty(HTMLElement.prototype, 'scrollHeight', { configurable: true, get: () => { reads++; return height; } });
 Object.defineProperty(document, 'fonts', { configurable: true, value: fonts });
 globalThis.ResizeObserver = class {
  constructor(callback) { this.callback = callback; }
  observe() { observers.add(this); }
  disconnect() { observers.delete(this); }
 };
 const host = document.createElement('div'); document.body.appendChild(host);
 const draw = (text) => render(h(ProseEditor, { text, heading: { kind: 'ロール', name: 'builder' }, onChange: () => {} }), host);
 try {
  await act(() => draw('saved'));
  const prose = host.querySelector('textarea');
  expect(prose.style.height).toBe(''); expect(reads).toBe(0);
  width = 500;
  await act(() => { for (const observer of observers) observer.callback(); });
  expect(prose.style.height).toBe('320px');
  height = 600;
  await act(() => draw('carried draft'));
  expect(prose.style.height).toBe('602px');
  height = 700;
  await act(() => fonts.dispatchEvent(new Event('loadingdone')));
  expect(prose.style.height).toBe('702px');
  const after = reads;
  await act(() => { for (const observer of observers) observer.callback(); });
  expect(reads).toBe(after);
  expect(prose.disabled).toBe(false); expect(prose.readOnly).toBe(false);
 } finally {
  render(null, host); host.remove(); globalThis.ResizeObserver = priorObserver;
  ['clientWidth', 'scrollHeight'].forEach((key, index) => { if (sizes[index]) Object.defineProperty(HTMLElement.prototype, key, sizes[index]); else delete HTMLElement.prototype[key]; });
  if (priorFonts) Object.defineProperty(document, 'fonts', priorFonts); else delete document.fonts;
 }
});
