// Compile-time regression tests. This file is checked by tsc, never bundled.
import { save, check } from './api.js';
import type { ProfileEditing } from './editing.js';
import type { Project, ResourcePlace, ResourceName } from './resource-identity.js';
declare const editing: ProfileEditing;
if (false) {
  // @ts-expect-error An object with the HTTP body's shape has no editing brand.
  save('profile', { place: 'project', doc: { name: 'fast' }, hash: 'h1' });
  // @ts-expect-error Checks also require the private editing brand.
  check('profile', { place: 'project', doc: { name: 'fast' } });
  // @ts-expect-error The saved value is not writable by a screen.
  editing.read.value.saved = null;
  // @ts-expect-error Draft fields are deeply readonly.
  editing.read.value.draft.args.push('outside');
  // @ts-expect-error A screen cannot replace the read projection.
  editing.read.value = editing.read.value;
  // @ts-expect-error A string cannot stand for a project.
  const project: Project = '/work/alpha';
  // @ts-expect-error A string cannot stand for a place.
  const place: ResourcePlace = 'global';
  // @ts-expect-error A string cannot stand for a saved name.
  const name: ResourceName<'profile'> = 'fast';
  void [project, place, name];
}
