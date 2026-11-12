import type { Rec } from '../teams/state.js';
import type { Selection } from '../teams/selection.js';
import { select } from '../teams/selection.js';
import { SIDE_RULES } from '../teams/selection-table.js';
declare const row: Rec;
if (false) {
 // @ts-expect-error Selection is private state, not a screen's signal.
 row.sel.value = null;
 // @ts-expect-error The right column is readonly.
 row.side.value = 'prompt';
 // @ts-expect-error The table cannot be rewritten by a reader.
 SIDE_RULES[0].side = 'yaml';
 // @ts-expect-error A column event must name an existing column.
 select(row, { how: 'side', side: 'invented' });
 // @ts-expect-error The component cannot be constructed from writable fields.
 const fake: Selection = { pick: { value: null }, side: { value: 'attr' }, send: () => {} };
 void fake;
}
