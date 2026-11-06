// The routing table (r-ccf8): confirm asks, undoable runs and offers undo,
// none just runs.
import { describe, expect, test } from 'bun:test';
import { DANGER, runDanger } from '../src/lib/danger';
import { confirmAsk, closeConfirm } from '../src/lib/confirm';
import { undo } from '../src/lib/undo';

describe('runDanger', () => {
  test('a confirm action asks and does not run', () => {
    let ran = false;
    runDanger('deleteTeam', { run: () => { ran = true; }, confirm: { title: 't', body: 'b', label: 'x', run: () => {} } });
    expect(ran).toBe(false);
    expect(confirmAsk.value?.title).toBe('t');
    closeConfirm();
  });
  test('an undoable action runs and offers its undo', () => {
    let ran = false, back = false;
    runDanger('unlistProject', { run: () => { ran = true; }, said: '外しました', restore: () => { back = true; } });
    expect(ran).toBe(true);
    expect(undo.value?.said).toBe('外しました');
    undo.value?.restore();
    expect(back).toBe(true);
  });
  test('a none action just runs', () => {
    let ran = false;
    runDanger('abortStart', { run: () => { ran = true; } });
    expect(ran).toBe(true);
    expect(confirmAsk.value).toBeNull();
  });
  test('the table has every action', () => {
    expect(DANGER.abortStart).toBe('none');
    expect(DANGER.deleteProfile).toBe('confirm');
    expect(DANGER.conflictRevert).toBe('undoable');
  });
});

describe('lost', () => {
  test('a discard names what it loses and says it cannot be taken back', async () => {
    const { render, h } = await import('../src/lib/preact.js');
    runDanger('discardRow', { run: () => {}, lost: ['本文'], confirm: { title: '捨てますか', label: '破棄' } });
    const host = document.createElement('div');
    render(h('div', {}, confirmAsk.value.body), host);
    expect(host.textContent).toContain('本文');
    expect(host.textContent).toContain('取り消せません');
    closeConfirm();
  });
});
