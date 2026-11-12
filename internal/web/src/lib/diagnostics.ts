// Optional, test-installed markers. No text, request body or credential is
// recorded, and normal pages have no collector or performance observers.
type Detail = Record<string, string | number | boolean>;
declare global {
  interface Window { hergyDiagnostics?: { mark: (name: string, detail?: Detail) => void } }
}
export function mark(name: string, detail?: Detail | (() => Detail)): void {
  const collector = window.hergyDiagnostics;
  if (collector) collector.mark(name, typeof detail === 'function' ? detail() : detail);
}
