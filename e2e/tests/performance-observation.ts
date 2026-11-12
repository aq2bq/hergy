// Installed before scripts, only by tests. Keep a bounded in-page timeline so
// recording does not send a CDP message on every render or animation frame.
export function observePerformance() {
  type Event = { at: number; type: string; detail: unknown; lastMark?: unknown };
  const capacity = 2048, events: Event[] = [];
  let cursor = 0, lastMark: unknown = null;
  const record = (type: string, detail: unknown) => {
    events[cursor++ % capacity] = {at: performance.now(), type, detail, lastMark};
  };
  const mark = (name: string, detail?: unknown) => {
    lastMark = {at: performance.now(), name, detail};
    record('mark', lastMark);
  };
  const timeline = {
    mark,
    snapshot: () => ({timeOrigin: performance.timeOrigin, now: performance.now(), url: location.href,
      supported: PerformanceObserver.supportedEntryTypes,
      events: cursor <= capacity ? events : [...events.slice(cursor % capacity), ...events.slice(0, cursor % capacity)]}),
  };
  Object.defineProperty(window, 'hergyDiagnostics', {value: timeline, configurable: true});
  if (PerformanceObserver.supportedEntryTypes.includes('longtask')) {
    new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) record('longtask', entry.toJSON());
    }).observe({type: 'longtask', buffered: true});
  }
  let last = performance.now();
  const frame = (now: number) => {
    if (now - last > 250) record('frame-gap', {from: last, to: now, duration: now - last, visibility: document.visibilityState});
    last = now;
    requestAnimationFrame(frame);
  };
  requestAnimationFrame(frame);
}
