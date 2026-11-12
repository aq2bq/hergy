// Compile-time regression tests. This file is checked by tsc, never bundled.
// @ts-expect-error The transport without an editing capability is private.
import { raw } from './api.js';
import { save, check, call, sendLegacy } from './api.js';
import { editProfile } from './editing.js';
import type { ProfileEditing, EditedRequest } from './editing.js';
import type { Project, ResourcePlace, ResourceName, ResourceIdentity } from './resource-identity.js';
declare const editing: ProfileEditing;
declare const identity: ResourceIdentity<'profile'>;
declare const teamSave: EditedRequest<'team', 'save'>;
if (false) {
  void raw;
  // @ts-expect-error An object with the HTTP body's shape has no editing brand.
  save('profile', { place: 'project', doc: { name: 'fast' }, hash: 'h1' });
  // @ts-expect-error Checks also require the private editing brand.
  check('profile', { place: 'project', doc: { name: 'fast' } });
  // @ts-expect-error The generic call cannot send a resource save body.
  call('PUT', '/api/profile/fast', { doc: { name: 'fast' } });
  // @ts-expect-error The generic call cannot send a resource check body.
  call('POST', '/api/profile/check', { doc: { name: 'fast' } });
  // @ts-expect-error The compatibility transport also requires its private brand.
  sendLegacy({ kind: 'profile', action: 'save', name: 'fast', body: {} });
  // @ts-expect-error A team request cannot be sent to the profile endpoint.
  save('profile', teamSave);
  // @ts-expect-error Saved names must come from the API identity boundary.
  editProfile(identity, { saved: { name: 'fast', kind: 'claude', args: [], env: [] }, draft: { name: 'fast', kind: 'claude', args: [], env: [] }, hash: 'h1' });
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
