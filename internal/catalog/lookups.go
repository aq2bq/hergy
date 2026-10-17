package catalog

import (
	"github.com/aq2bq/hergy/internal/definition"
)

// MemberRef is one member of one team definition (定義 × メンバー), as a reverse
// lookup names it (C8): which definition, which member, and, for a worker, which
// purpose. A member's own name is the short name team.yaml writes; a worker is
// named by the member that hires it and the purpose it is hired for.
type MemberRef struct {
	Definition string
	Member     string
	Purpose    string // a worker's purpose; "" when the ref is a member
}

// Readers is the members of every readable team definition that read one rule
// document (C8): a role document, the agreement every team shares, or a team's
// own agreement. The document is told by its path, the way a screen holds it. A
// definition that does not read is left out: what cannot be read has no members
// to name.
//
// Every member of a definition reads the shared agreement, so when the path is
// that one, every member of every readable definition is named. A member reads
// its own role document, so the path is matched against the document its role
// resolves to (K1).
func (c *Catalog) Readers(path string) []MemberRef {
	shared, hasShared := c.SharedAgreement()
	var out []MemberRef
	for _, def := range c.readable() {
		// A definition's members share roles, so a role's document is looked up
		// once for each role, not once for each member.
		roles := map[string]string{}
		for _, member := range def.Graph.Members {
			doc, asked := roles[member.Role]
			if !asked {
				if found, ok := c.Role(member.Role); ok {
					doc = found.Path
				}
				roles[member.Role] = doc
			}
			switch {
			case hasShared && shared.Path == path:
				out = append(out, MemberRef{Definition: def.Name, Member: member.Name})
			case def.HasAgreement && def.Agreement.Path == path:
				out = append(out, MemberRef{Definition: def.Name, Member: member.Name})
			case doc != "" && doc == path:
				out = append(out, MemberRef{Definition: def.Name, Member: member.Name})
			}
		}
	}
	return out
}

// RoleUsers is the members that play one role, across every readable team
// definition (C8). A role document belongs to a layer, not to a team (K5), so a
// role is played in more than one definition; the refs say where.
func (c *Catalog) RoleUsers(role string) []MemberRef {
	var out []MemberRef
	for _, def := range c.readable() {
		for _, member := range def.Graph.Members {
			if member.Role == role {
				out = append(out, MemberRef{Definition: def.Name, Member: member.Name})
			}
		}
	}
	return out
}

// ProfileUsers is who runs one profile (C8): the members that name it, and the
// workers that run it, told apart because a member is started with it and a
// worker is hired with it (I7, I12c). The worker refs name the member that hires
// the worker and the purpose.
func (c *Catalog) ProfileUsers(name string) (members, workers []MemberRef) {
	for _, def := range c.readable() {
		for _, member := range def.Graph.Members {
			if member.Profile == name {
				members = append(members, MemberRef{Definition: def.Name, Member: member.Name})
			}
		}
		for _, worker := range def.Graph.Workers {
			if worker.Profile == name {
				workers = append(workers, MemberRef{Definition: def.Name, Member: worker.From, Purpose: worker.Purpose})
			}
		}
	}
	return members, workers
}

// readable is every team definition a name resolves to that reads, in the order
// the list holds them (by name). A definition that does not read is left out: a
// reverse lookup names members of teams hergy can read.
func (c *Catalog) readable() []*definition.Definition {
	names := c.candidateNames()
	out := make([]*definition.Definition, 0, len(names))
	for _, name := range names {
		def, err := c.Read(name)
		if err != nil {
			continue
		}
		out = append(out, def)
	}
	return out
}
