// Package newteam writes a team definition's folder: the definition's own files,
// the layer's role documents a starter team reads, and the starter profile in
// the layer's profiles.yaml. hergy new (internal/app) and the team screen
// (internal/web) both call it, so the two cannot drift about what making a team
// definition writes, or where (spec-teams C8, K2, K5, I7).
package newteam

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/scaffold"
	"github.com/aq2bq/hergy/internal/source"
)

// In is one write of a team definition's folder.
type In struct {
	// Catalog reads the layers and says where each one is.
	Catalog *catalog.Catalog
	// Layer is the layer the definition goes into: source.LayerLocal or
	// source.LayerGlobal. The screen's place is turned into a layer by
	// catalog.Layer.
	Layer string
	// Name is the definition name: the folder under the layer's teams/.
	Name string
	// Lang is the language the layer's role documents are read in. Only a
	// starter write reads it.
	Lang string
	// Files are the definition's own files, in the order they are written.
	Files []scaffold.File
	// Starter says this definition is made from the scaffold, so the starter
	// roles and the starter profile may go with it. A copy of another definition
	// writes only Files (spec-teams B3).
	Starter bool
	// StarterRoles says this definition's members use the starter roles, so a
	// starter role document no place holds is written. hergy new asks for them
	// only when neither place holds a role document at all; the team screen asks
	// when the person pressed the starter-roles button (r-54b8, d-86b1).
	StarterRoles bool
	// StarterProfile says the starter profile goes with this definition: it is
	// added to the layer's profiles.yaml when neither place defines it. hergy
	// new and the team screen ask for it the way they ask for StarterRoles.
	StarterProfile bool
	// Carry is what a copy of another layer's definition brings with it: the
	// role documents and profiles the person chose, written into this layer as
	// the layer they stand in holds them (r-7e4a). A copy writes the definition
	// alone when nothing was chosen, and a name this layer already holds is left
	// as it is.
	Carry *Carry
}

// Carry is the layer a copy's documents are read from, and the names the person
// chose to bring with it (r-7e4a, d-dc81).
type Carry struct {
	// Catalog reads the layer the definition the row stands on reads, so the
	// carried documents are the ones that definition uses.
	Catalog *catalog.Catalog
	// Layer is that layer: source.LayerLocal or source.LayerGlobal.
	Layer string
	// Roles and Profiles are the names the person chose, in the order they are
	// written.
	Roles    []string
	Profiles []string
}

// Written is what one write makes, as paths, in the order they are written.
type Written struct {
	// Files are the definition's own files.
	Files []string
	// Roles are the layer's role documents hergy wrote.
	Roles []string
	// Kept are the role documents already there — in either place — left as
	// they are (K5).
	Kept []string
	// Profile is the layer's profiles.yaml, when the starter profile was added
	// to it; "" when a place already resolved the name or the person did not ask
	// for it.
	Profile string
	// Missing are the starter roles the definition's members name that no place
	// holds and this write does not write, in the scaffold's order. hergy new
	// and the team screen say what to choose instead (r-54b8).
	Missing []string
	// ProfileMissing says the starter profile the definition's members name is
	// held by neither place and this write does not add it.
	ProfileMissing bool
	// CarryRoles are the role documents this write copies into the layer from
	// the layer the definition it copies stands in (r-7e4a).
	CarryRoles []string
	// CarryProfiles are the profile names this write adds to the layer's
	// profiles.yaml from the same place, and CarryProfileFile is that
	// profiles.yaml when any was added.
	CarryProfiles    []string
	CarryProfileFile string
}

// Plan is what a write would make, without writing: the same paths Write
// reports, in the same order. The team screen shows these as the files the row
// would write (spec-teams B2).
func Plan(in In) (Written, error) {
	return plan(in)
}

// Write makes a team definition's folder: Files, and, for a starter write, the
// layer's role documents (leaving one already there as it is) and the starter
// profile in the layer's profiles.yaml (I7, K5). A write that cannot finish
// takes back what it wrote, so a refused save leaves the layer as it was
// (spec-teams B6).
func Write(in In) (Written, error) {
	written, err := plan(in)
	if err != nil {
		return Written{}, err
	}
	var made []string
	// The profiles.yaml this write adds to is put back as it was: the write may
	// have made the file. The starter profile and the carried ones are added to
	// the same file.
	profilesFile := written.Profile
	if profilesFile == "" {
		profilesFile = written.CarryProfileFile
	}
	var before []byte
	hadProfile := false
	if profilesFile != "" {
		if text, readErr := os.ReadFile(profilesFile); readErr == nil {
			before, hadProfile = text, true
		}
	}
	fail := func(err error) (Written, error) {
		for _, m := range slices.Backward(made) {
			_ = os.Remove(m)
		}
		if profilesFile != "" {
			if hadProfile {
				_ = source.WriteBeside(profilesFile, string(before))
			} else {
				_ = os.Remove(profilesFile)
			}
		}
		return Written{}, err
	}
	dir := filepath.Join(in.Catalog.LayerDir(in.Layer), "teams", in.Name)
	for _, file := range in.Files {
		path := filepath.Join(dir, file.Path)
		if err := writeFile(path, file.Data); err != nil {
			return fail(err)
		}
		made = append(made, path)
	}
	if err := writeCarry(in, written, &made); err != nil {
		return fail(err)
	}
	if !in.Starter {
		return written, nil
	}
	roles, err := scaffold.Roles(in.Lang)
	if err != nil {
		return fail(err)
	}
	byName := map[string][]byte{}
	for _, role := range roles {
		byName[role.Path] = role.Data
	}
	for _, path := range written.Roles {
		data, ok := byName[filepath.Base(path)]
		if !ok {
			return fail(fmt.Errorf("newteam: no starter role document for %s", filepath.Base(path)))
		}
		if err := writeFile(path, data); err != nil {
			return fail(err)
		}
		made = append(made, path)
	}
	if written.Profile != "" {
		if err := addStarterProfile(written.Profile); err != nil {
			return fail(err)
		}
	}
	return written, nil
}

// writeCarry copies the documents a person chose into this layer: a role
// document's own bytes, and a profile's kind, args and env as the layer it
// stands in writes them (r-7e4a). The names were settled by plan: a document
// this layer holds is not among them, so a copy never writes over the layer's
// own.
func writeCarry(in In, written Written, made *[]string) error {
	if in.Carry == nil {
		return nil
	}
	for _, path := range written.CarryRoles {
		name := strings.TrimSuffix(filepath.Base(path), ".md")
		doc, held := in.Carry.Catalog.LayerRole(in.Carry.Layer, name)
		if !held {
			return fmt.Errorf("newteam: no role document for %s in that layer", name)
		}
		text, err := in.Carry.Catalog.Reader().Read(doc)
		if err != nil {
			return err
		}
		if err := writeFile(path, []byte(text)); err != nil {
			return err
		}
		*made = append(*made, path)
	}
	if len(written.CarryProfiles) == 0 {
		return nil
	}
	from, err := in.Carry.Catalog.LayerProfiles(in.Carry.Layer)
	if err != nil {
		return err
	}
	add := make([]profile.Profile, 0, len(written.CarryProfiles))
	for _, name := range written.CarryProfiles {
		one, ok := from[name]
		if !ok {
			return fmt.Errorf("newteam: no profile for %s in that layer", name)
		}
		add = append(add, one)
	}
	if err := addProfiles(written.CarryProfileFile, add); err != nil {
		return err
	}
	return nil
}

// plan is what a write would make: the paths, and, for a starter write, the role
// documents to write and to leave, and the starter profile when no place
// resolves it and the person asked for it. A role document either place holds is
// looked up across both places, the way a member's role is (K1), so a starter
// team does not write over a global role into the project (r-54b8).
//
// A starter role document or the starter profile belongs to this write only when
// the definition's own members name it: the person may have asked for the
// starter roles and then named other roles, and a role no member names is one
// the person did not choose (r-54b8, d-86b1).
func plan(in In) (Written, error) {
	base := in.Catalog.LayerDir(in.Layer)
	dir := filepath.Join(base, "teams", in.Name)
	out := Written{}
	for _, file := range in.Files {
		out.Files = append(out.Files, filepath.Join(dir, file.Path))
	}
	if err := planCarry(in, base, &out); err != nil {
		return Written{}, err
	}
	if !in.Starter {
		return out, nil
	}
	namedRoles, namedProfiles, membersKnown := membersNamed(in)
	roles, err := scaffold.Roles(in.Lang)
	if err != nil {
		return Written{}, err
	}
	for _, role := range roles {
		name := strings.TrimSuffix(role.Path, ".md")
		if membersKnown && !namedRoles[name] {
			continue
		}
		if doc, ok := in.Catalog.Role(name); ok {
			out.Kept = append(out.Kept, doc.Path)
			continue
		}
		if in.StarterRoles {
			out.Roles = append(out.Roles, filepath.Join(base, "roles", role.Path))
			continue
		}
		out.Missing = append(out.Missing, name)
	}
	if membersKnown && !namedProfiles[scaffold.ProfileName] {
		return out, nil
	}
	profiles, _, err := in.Catalog.Profiles()
	if err != nil {
		return Written{}, err
	}
	if _, ok := profiles[scaffold.ProfileName]; ok {
		return out, nil
	}
	if in.StarterProfile {
		out.Profile = filepath.Join(base, "profiles.yaml")
	} else {
		out.ProfileMissing = true
	}
	return out, nil
}

// planCarry is what a copy brings with it: the role documents and profiles the
// person chose, less the ones this layer already holds. A document this layer
// holds is left as it is (r-7e4a): a copy never writes over the layer's own.
func planCarry(in In, base string, out *Written) error {
	if in.Carry == nil {
		return nil
	}
	for _, name := range in.Carry.Roles {
		// A name the layer this copy reads no longer holds is not carried: the
		// person chose it from an answer that read a file which has moved since,
		// and the row's next check says what the definition still misses.
		if _, there := in.Carry.Catalog.LayerRole(in.Carry.Layer, name); !there {
			continue
		}
		path := filepath.Join(base, "roles", name+".md")
		if _, held := in.Catalog.LayerRole(in.Layer, name); held {
			out.Kept = append(out.Kept, path)
			continue
		}
		out.CarryRoles = append(out.CarryRoles, path)
	}
	if len(in.Carry.Profiles) == 0 {
		return nil
	}
	from, err := in.Carry.Catalog.LayerProfiles(in.Carry.Layer)
	if err != nil {
		return err
	}
	held, err := in.Catalog.LayerProfiles(in.Layer)
	if err != nil {
		return err
	}
	for _, name := range in.Carry.Profiles {
		if _, there := from[name]; !there {
			continue
		}
		if _, ok := held[name]; ok {
			continue
		}
		out.CarryProfiles = append(out.CarryProfiles, name)
	}
	if len(out.CarryProfiles) > 0 {
		out.CarryProfileFile = filepath.Join(base, "profiles.yaml")
	}
	return nil
}

// membersNamed is the starter roles and profiles the definition's own team.yaml
// says its members use. membersKnown is false when the caller gave no team.yaml
// text or the text does not read: then the caller could not say which names the
// members use, and the starter documents are the ones the definition starts
// with, so plan keeps the older reading rather than writing nothing.
func membersNamed(in In) (roles, profiles map[string]bool, membersKnown bool) {
	var text string
	for _, file := range in.Files {
		if filepath.Base(file.Path) == "team.yaml" && len(file.Data) > 0 {
			text = string(file.Data)
			break
		}
	}
	if text == "" {
		return nil, nil, false
	}
	path := filepath.Join(in.Catalog.LayerDir(in.Layer), "teams", in.Name, "team.yaml")
	def, err := in.Catalog.With(path, text).Definition(in.Name, in.Layer)
	if err != nil || def == nil || def.Graph == nil {
		return nil, nil, false
	}
	roles, profiles = map[string]bool{}, map[string]bool{}
	for _, member := range def.Graph.Members {
		roles[member.Role] = true
		profiles[member.Profile] = true
	}
	return roles, profiles, true
}

// HasRoles reports whether either place holds a role document: a role a new
// definition's members can choose. A name only a definition writes, with no
// document behind it, is not one (r-54b8).
func HasRoles(cat *catalog.Catalog) bool {
	for _, role := range cat.Roles() {
		if role.HasDocument {
			return true
		}
	}
	return false
}

// HasProfiles reports whether either place defines a profile, so a new
// definition's members choose from them rather than the starter profile
// (r-54b8). A profiles.yaml that does not read is the error it is.
func HasProfiles(cat *catalog.Catalog) (bool, error) {
	profiles, _, err := cat.Profiles()
	if err != nil {
		return false, err
	}
	return len(profiles) > 0, nil
}

// writeFile writes one file, making the directories it needs.
func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// addProfiles adds the profiles a copy brings to the profiles.yaml at path,
// each with the kind, args and env the layer it stands in writes, in the order
// the person chose them (r-7e4a). The file is read and written by the edit the
// profiles screen makes, so every other line it holds comes back as it was, and
// a file that cannot be read is an error rather than something to write over
// (I7). The names were settled by plan: this layer holds none of them.
func addProfiles(path string, add []profile.Profile) error {
	for _, one := range add {
		text, err := os.ReadFile(path)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			text = []byte("version: 1\n")
		}
		edited, err := profile.EditYAML(text, path, profile.Add(one.Name, one.Kind))
		if err != nil {
			return err
		}
		if len(one.Args) > 0 {
			if edited, err = profile.EditYAML(edited, path, profile.SetArgs(one.Name, one.Args)); err != nil {
				return err
			}
		}
		if len(one.Env) > 0 {
			pairs := make([]profile.EnvPair, 0, len(one.Env))
			for _, key := range slices.Sorted(maps.Keys(one.Env)) {
				pairs = append(pairs, profile.EnvPair{Key: key, Value: one.Env[key]})
			}
			if edited, err = profile.EditYAML(edited, path, profile.SetEnvInOrder(one.Name, pairs)); err != nil {
				return err
			}
		}
		if err := source.WriteBeside(path, string(edited)); err != nil {
			return err
		}
	}
	return nil
}

// addStarterProfile adds the starter profile to the profiles.yaml at path,
// making the file when the layer has none. A file that is already there is
// added to with the one edit the profiles screen makes, so every other line it
// holds — the order of the profiles, a comment, the value form a person wrote —
// comes back as it was (spec-boundary 5, 監査 R3). A file the layer cannot read
// is an error rather than something to write over (I7).
func addStarterProfile(path string) error {
	switch text, err := os.ReadFile(path); {
	case err == nil:
		out, editErr := profile.EditYAML(text, path, profile.Add(scaffold.ProfileName, scaffold.ProfileKind))
		if editErr != nil {
			return editErr
		}
		return source.WriteBeside(path, string(out))
	case errors.Is(err, os.ErrNotExist):
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
	default:
		return err
	}
	text, err := profile.EmitProfiles(map[string]profile.Profile{
		scaffold.ProfileName: {
			Name: scaffold.ProfileName,
			Kind: scaffold.ProfileKind,
			Path: path,
		},
	})
	if err != nil {
		return err
	}
	return source.WriteBeside(path, text)
}
