// lib/copy.ts — where a copy into the other place stands (r-f781).
//
// A document stands in the project's place or in the global layer. Copying it
// into the other place writes a new file there and leaves the original alone;
// the screens draw the copy on the side it was copied to. On a headquarters
// with projects listed, that side is a group's key: "global" for the global
// layer, the project's own path otherwise. Without projects listed the group is
// empty and the screen keeps its one list.
export type CopyPlace = 'project' | 'global';

// otherPlace is the place a copy into the other side goes to.
export const otherPlace = (place: CopyPlace): CopyPlace => (place === 'project' ? 'global' : 'project');

// copyGroupOf is the group a copy into `to` stands in: the global layer's key,
// the source project's key, or the project this serve was started in when the
// copy comes out of the global layer.
export const copyGroupOf = (to: CopyPlace, from: string, current: string): string => {
  if (to === 'global') return 'global';
  return from && from !== 'global' ? from : current;
};

// groupPlace is the place a group's rows stand in: the global group reads the
// global layer, and every project's group reads its own project.
export const groupPlace = (group: string): CopyPlace => (group === 'global' ? 'global' : 'project');
