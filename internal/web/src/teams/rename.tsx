import { useEffect, useMemo, useState } from '../lib/preact.js';
import { resource } from '../lib/resource.js';
import { call, check, problemText } from '../lib/api.js';
import type { Schemas } from '../lib/api.js';
import { projectOfGroup } from '../lib/copy.js';
import { renamed, renameDirty, groupOfRecord } from './state.js';
import type { Rec } from './state.js';

export function Rename({ r }: { r: Rec }) {
  const [open, setOpen] = useState(false), [name, setName] = useState(r.name.value);
  const [saving, setSaving] = useState(false), [error, setError] = useState('');
  const reading = useMemo(() => resource(async (signal: AbortSignal, next: string) => {
    const res = await check('team', { name: r.name.value, place: r.place.value, project: projectOfGroup(groupOfRecord(r)), rename_to: next }, { signal });
    return res.json?.rename || null;
  }), [r]);
  useEffect(() => { if (open) void reading.run(name); return () => reading.abort(); }, [open, name, reading]);
  const dirty = renameDirty(r);
  async function save() {
    if (dirty || saving || reading.loading.value || reading.error.value || !reading.value.value) return;
    setSaving(true); setError('');
    try {
      const payload: Schemas['TeamRenameRequest'] = {
        project: projectOfGroup(groupOfRecord(r)), place: r.place.value, rename_to: name,
        hash: r.hash.value, agreement_hash: r.agreeHash.value || 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855', agreement_exists: !!r.agreeHash.value,
      };
      const res = await call<Schemas['TeamSaved']>('POST', `/api/team/${encodeURIComponent(r.name.value)}/rename`, payload);
      if (!res.json) throw new Error('保存の結果を読めませんでした。画面を読み直してください');
      await renamed(r, name, res.json);
      setOpen(false);
    } catch (err) { setError(problemText(err, '名前を変更できませんでした')); }
    finally { setSaving(false); }
  }
  return <section class="team-rename" aria-label="チーム定義名の変更">
    <button class="btn" type="button" disabled={dirty || saving} onClick={() => setOpen(!open)}>名前を変更</button>
    {dirty ? <span class="none">保存していない変更があります。保存するか、元に戻してから名前を変更してください</span> : null}
    {open ? <div>
      <label>新しいチーム定義名 <input value={name} disabled={saving} onInput={(e) => setName(e.currentTarget.value)} /></label>
      {reading.value.value ? <><p><code>{reading.value.value.from}</code> → <code>{reading.value.value.to}</code></p><ul>{reading.value.value.notes.map((note) => <li>{note}</li>)}</ul></> : null}
      {reading.loading.value ? <p>変更先を確認しています。</p> : null}
      {reading.error.value ? <p role="alert">{problemText(reading.error.value, '変更先を確認できませんでした')}</p> : null}
      {error ? <p role="alert">{error}</p> : null}
      <button class="btn primary" disabled={dirty || saving || reading.loading.value || !!reading.error.value || !reading.value.value} onClick={() => void save()}>名前を保存</button>
      <button class="btn" disabled={saving} onClick={() => setOpen(false)}>取り消す</button>
    </div> : null}
    {r.renameNotes.value.length ? <ul>{r.renameNotes.value.map((note) => <li>{note}</li>)}</ul> : null}
  </section>;
}
