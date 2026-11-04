// A transfer owns exact character ranges and saves the destination first.
export type TransferDocument = { id: string; label: string; text: () => string; set: (text: string) => void; save: () => Promise<boolean> };
export type TransferTarget = { id: string; label: string; kind: 'team' | 'role'; group: string; otherProject: boolean; load: () => Promise<TransferDocument> };
export type TransferSource = TransferDocument & { targets: () => Promise<TransferTarget[]> };
export const documents = new Map<string, TransferDocument>();
export const documentID = (kind: string, name: string, place: string, project: string) => JSON.stringify([kind, name, place, place === 'global' ? '' : project]);
export type Block = { heading: string; depth: number; start: number; end: number };
export function blockRange(parts: Block[], index: number, children: boolean): [number, number] {
  const part = parts[index];
  if (!part) throw new Error('見出しが変わりました。選び直してください');
  let end = part.end;
  if (children && part.depth > 0) {
    for (let i = index + 1; i < parts.length && parts[i].depth > part.depth; i++) end = parts[i].end;
  }
  return [part.start, end];
}
export function transferDraft(source: string, destination: string, start: number, end: number, at: number, move: boolean) {
  if (start < 0 || end > source.length || start >= end || at < 0 || at > destination.length) throw new Error('挿入位置を選び直してください');
  const before = destination.slice(0, at), copied = source.slice(start, end);
  const newline = destination.includes('\r\n') ? '\r\n' : '\n';
  const separator = !before || /\r?\n[ \t]*\r?\n$/.test(before) ? '' : /\n$/.test(before) ? newline : newline + newline;
  return { source: move ? source.slice(0, start) + source.slice(end) : source, destination: before + separator + copied + destination.slice(at) };
}
export async function saveTransfer(destination: TransferDocument, source: TransferDocument, move: boolean, failure?: (error: unknown) => void): Promise<'done' | 'destination-failed' | 'source-failed'> {
  const attempt = async (doc: TransferDocument) => { try { return await doc.save(); } catch (err) { failure?.(err); return false; } };
  if (!await attempt(destination)) return 'destination-failed';
  if (move && !await attempt(source)) return 'source-failed';
  return 'done';
}
