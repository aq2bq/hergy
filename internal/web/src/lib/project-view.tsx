import { confirmLeave } from './nav';
import type { components } from '../../../apicontract/api-types';
type Projects = components['schemas']['HQProjects'];
export function ProjectView({ projects }: { projects?: Projects | null }) {
  if (!projects) return null;
  const current = projects.current;
  const choices = projects.projects.filter((p) => p.listed || p.path === current?.path);
  if (current && !choices.some((p) => p.path === current.path)) choices.unshift(current);
  return <div class="project-view"><label>表示するプロジェクト <select aria-label="表示するプロジェクト" value={current?.path || ''} onChange={(e) => {
    if (!confirmLeave()) { e.currentTarget.value = current?.path || ''; return; }
    const url = new URL(location.href);
    url.search = new URLSearchParams({ project: e.currentTarget.value }).toString();
    location.assign(url.href);
  }}>{choices.map((p) => <option value={p.path}>{p.name} — {p.display}</option>)}</select></label><span>このプロジェクトとグローバルの設定を表示しています。</span></div>;
}
