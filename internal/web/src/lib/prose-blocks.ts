// Offsets refer to the original string, including its line endings.
export function blocks(text: string): { heading: string; depth: number; start: number; end: number }[] {
  const out: { heading: string; depth: number; start: number; end: number }[] = [];
  let offset = 0;
  let fence = '';
  for (const line of text.split('\n')) {
    const parsed = line.replace(/\r$/, '');
    const marker = parsed.match(/^ {0,3}(`{3,}|~{3,})(.*)$/);
    if (marker) {
      if (!fence && !(marker[1][0] === '`' && marker[2].includes('`'))) fence = marker[1];
      else if (marker[1][0] === fence[0] && marker[1].length >= fence.length && !marker[2].trim()) fence = '';
    } else if (!fence) {
      const heading = parsed.match(/^\s{0,3}(#{1,6})(?:\s+|$)(.*)/);
      if (heading) out.push({ heading: heading[2], depth: heading[1].length, start: offset, end: text.length });
    }
    offset += line.length + 1;
  }
  if (!out.length || out[0].start !== 0) out.unshift({ heading: '本文', depth: 0, start: 0, end: text.length });
  out.forEach((block, i) => { block.end = out[i + 1]?.start ?? text.length; });
  return out;
}
