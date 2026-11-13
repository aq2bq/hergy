import { term } from './terms.generated.js';
import { confirmLeave } from './nav';
import type { components } from '../../../apicontract/api-types';
type Projects = components['schemas']['HQProjects'];
export function ProjectView({ projects }: { projects?: Projects | null }) {
  if (!projects) return null;
  const current = projects.current;
  const choices = projects.projects.filter((p) => p.listed || p.path === current?.path);
  if (current && !choices.some((p) => p.path === current.path)) choices.unshift(current);
  return <div class="project-view"><label>{`表示する${term('project')} `}<select aria-label={`表示する${term('project')}`} value={current?.path || ''} onChange={(e) => {
    const next = e.currentTarget.value;
    const go = () => {
      const url = new URL(location.href);
      url.search = new URLSearchParams({ project: next }).toString();
      location.assign(url.href);
    };
    // The whole page goes with the project, so a draft would go too: the select
    // goes back to the project it was on while the answer stands (d-021e).
    if (confirmLeave(go)) e.currentTarget.value = current?.path || '';
    else go();
  }}>{choices.map((p) => <option value={p.path}>{p.name} — {p.display}</option>)}</select></label></div>;
}
