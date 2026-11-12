// Resource identifiers enter from the API here. Draft names remain strings:
// typing an incomplete name does not change the identity of the file being edited.
import type { Schemas } from './api.js';

declare const nameBrand: unique symbol;
declare const projectBrand: unique symbol;
declare const placeBrand: unique symbol;
export type ResourceKind = 'profile' | 'role' | 'team';
export type ResourceName<K extends ResourceKind> = string & { readonly [nameBrand]: K };
export type Project = string & { readonly [projectBrand]: true };
export type ResourcePlace = ('project' | 'global') & { readonly [placeBrand]: true };
export type ResourceIdentity<K extends ResourceKind> = Readonly<{
  kind: K; name: ResourceName<K> | null; place: ResourcePlace; project?: Project;
}>;

// API and embedded-page data use the same DTO scalars. An empty name denotes a
// new draft, never an invented saved identifier. The server owns path normalization.
export function resourceIdentity<K extends ResourceKind>(kind: K, dto: {
  name?: Schemas['ProfileDoc']['name']; place: string; project?: Schemas['HQProject']['path'];
}): ResourceIdentity<K> {
  const place = dto.place === 'global' ? 'global' : 'project';
  return Object.freeze({ kind,
    name: dto.name ? dto.name as ResourceName<K> : null,
    place: place as ResourcePlace,
    project: place === 'global' || !dto.project || dto.project === 'global' ? undefined : dto.project as Project,
  });
}
