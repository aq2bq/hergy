// Compile-time regression tests. This file is checked by tsc, never bundled.
// @ts-expect-error The transport without an editing capability is private.
import { raw } from './api.js';
// @ts-expect-error The compatibility transport no longer exists.
import { sendLegacy } from './api.js';
import { save, check, call } from './api.js';
import { editProfile, editRole } from './editing.js';
import type { ProfileEditing, RoleEditing, TeamEditing, EditedRequest } from './editing.js';
import type { Project, ResourcePlace, ResourceName, ResourceIdentity } from './resource-identity.js';
declare const editing: ProfileEditing;
declare const role: RoleEditing;
declare const team: TeamEditing;
declare const identity: ResourceIdentity<'profile'>;
declare const roleIdentity: ResourceIdentity<'role'>;
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
  void sendLegacy;
  // @ts-expect-error A team save requires the private brand.
  save('team', { name: 'sample', place: 'project', doc: { ops: [] } });
  // @ts-expect-error A team check requires the private brand.
  check('team', { name: 'sample', place: 'project', doc: { ops: [] } });
  // @ts-expect-error A role save requires the private brand.
  save('role', { name: 'builder', place: 'project', doc: {} });
  // @ts-expect-error A role check requires the private brand.
  check('role', { place: 'project', doc: {} });
  // @ts-expect-error Team state is deeply readonly.
  team.team.read.value.draft.members.push({ name: 'outside', role: '', profile: '' });
  // @ts-expect-error Agreement state is deeply readonly.
  team.agreement.read.value.draft.text = 'outside';
  // @ts-expect-error Folder/copy options cannot be assigned by a screen.
  team.read.value.base = null;
  // @ts-expect-error A screen cannot inject a team save body.
  team.save('team', { hash: 'invented' });
  // @ts-expect-error Figure actions can only be constructed by editing's named operations.
  team.act([{ kind: 'memberRole', member: 'lead', role: 'builder' }]);
  // @ts-expect-error Operation provenance cannot be overwritten.
  team.operations.value = [];
  // @ts-expect-error A team request cannot be sent to the profile endpoint.
  save('profile', teamSave);
  // @ts-expect-error Saved names must come from the API identity boundary.
  editProfile(identity, { saved: { name: 'fast', kind: 'claude', args: [], env: [] }, draft: { name: 'fast', kind: 'claude', args: [], env: [] }, hash: 'h1' });
  // @ts-expect-error Saved role names must come from the identity boundary.
  editRole(roleIdentity, { saved: { name: 'builder', title: '', text: '' }, draft: { name: 'builder', title: '', text: '' }, hash: 'h1' });
  // @ts-expect-error Role drafts are readonly, including raw text.
  role.read.value.draft.text = 'outside';
  // @ts-expect-error A role editor cannot receive a caller-built save body.
  role.save({ place: 'project', doc: {} });
  // @ts-expect-error Cross-representation synchronization is private.
  role.accept({}, 'hash', '', 0);
  // @ts-expect-error The raw editor cannot expose its mutable saved value.
  role.yaml.read.value.saved = 'outside';
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
