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

import { writeOpen, rowAddress } from './address.js';
import { PAGE_POSITION, movePage } from './page-position.js';
// @ts-expect-error callers cannot construct a write address
writeOpen(new URLSearchParams(), [{ name: 'core', group: 'global' }]);
const address = rowAddress('core', 'global');
// @ts-expect-error addresses are readonly
address.group = 'project';
// @ts-expect-error the page-position table is readonly
PAGE_POSITION.openRow.at = 'band';
// @ts-expect-error only named navigation operations are accepted
movePage('reread-and-scroll', null);

import { confirmAsk, askConfirm, confirmView } from './confirm.js';
import { undo, offerUndo } from './undo.js';
import { DANGER, dangerAttrs } from './danger.js';
// @ts-expect-error confirmation state has no public setter
confirmAsk.value = null;
// @ts-expect-error undo state has no public setter
undo.value = null;
// @ts-expect-error routing policy is readonly
DANGER.deleteTeam = 'none';
// @ts-expect-error only danger can authorize confirmation
askConfirm({}, {title: '', body: '', label: '', run: () => {}});
// @ts-expect-error only danger can authorize an undo notice
offerUndo({}, {said: '', restore: () => {}});
// @ts-expect-error only dialogs can mount the confirmation view
confirmView({});
// @ts-expect-error every marked danger action is known
dangerAttrs('erase-everything');

import { readPromptComparison, call, type Schemas } from './api.js';
declare const promptBody: Schemas['PromptComparisonRequest'];
// @ts-expect-error only prompt-source constructs prompt requests
readPromptComparison({body: promptBody});
// @ts-expect-error the raw prompt route is not a direct call
call('POST', '/api/team/prompt-comparison', {member: 'lead'});
// @ts-expect-error prompt runs have no exported arbitrary-key writer
import { setRun } from '../teams/prompt-source.js';
void setRun;

import { bandCounts, bandActivity } from './band-counts.js';
// @ts-expect-error a screen cannot supply its own resource count
bandCounts({teams: 3, roles: 2, profiles: 1});
// @ts-expect-error a screen cannot supply its own activity count
bandActivity(4, 5, true);
// @ts-expect-error the DOM count setter is private to band-counts
import { setCount } from './band-counts.js';
void setCount;
// @ts-expect-error nav no longer exports count writers
import { setActivity } from './nav.js';
void setActivity;
