import { expect, test } from 'bun:test';
import { observePerformance } from '../../../e2e/tests/performance-observation';
import { mark } from '../src/lib/diagnostics';
test('failed-page observations keep bounded marks, long tasks and frame gaps on one clock', () => {
 const prior = {observer: globalThis.PerformanceObserver, frame: globalThis.requestAnimationFrame, diagnostics: Object.getOwnPropertyDescriptor(window, 'hergyDiagnostics')};
 let notify, frame;
 globalThis.PerformanceObserver = class {
  static supportedEntryTypes = ['longtask'];
  constructor(callback) { notify = callback; }
  observe() {}
 };
 globalThis.requestAnimationFrame = (callback) => { frame = callback; return 1; };
 try {
  observePerformance();
  mark('prose:measure:start', {width: 500});
  notify({getEntries: () => [{toJSON: () => ({entryType: 'longtask', startTime: 10, duration: 9000})}]});
  frame(performance.now() + 9000);
  const before = window.hergyDiagnostics.snapshot();
  expect(before.events.map((e) => e.type)).toEqual(['mark', 'longtask', 'frame-gap']);
  expect(before.events[1].lastMark.name).toBe('prose:measure:start');
  expect(before.events[2].detail.duration).toBeGreaterThan(8000);
  for (let i = 0; i < 2100; i++) mark('mark', {i});
  const after = window.hergyDiagnostics.snapshot();
  expect(after.events).toHaveLength(2048);
  expect(after.events[0].detail.detail.i).toBe(52);
  expect(after.events.at(-1).detail.detail.i).toBe(2099);
 } finally {
  globalThis.PerformanceObserver = prior.observer; globalThis.requestAnimationFrame = prior.frame;
  if (prior.diagnostics) Object.defineProperty(window, 'hergyDiagnostics', prior.diagnostics); else delete window.hergyDiagnostics;
 }
});
