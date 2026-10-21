// Package newteam writes a team definition's folder: the definition's own files,
// the layer's role documents a starter team reads, and the starter profile in
// the layer's profiles.yaml. hergy new (internal/app) and the team screen
// (internal/web) both call it, so the two cannot drift about what making a team
// definition writes, or where (spec-teams C8, K2, K5, I7).
package newteam

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

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
	// Starter says this definition is made from the scaffold, so the layer's
	// role documents and the starter profile go with it. A copy of another
	// definition writes only Files (spec-teams B3).
	Starter bool
}

// Written is what one write makes, as paths, in the order they are written.
type Written struct {
	// Files are the definition's own files.
	Files []string
	// Roles are the layer's role documents hergy wrote.
	Roles []string
	// Kept are the layer's role documents already there, left as they are
	// (K5).
	Kept []string
	// Profile is the layer's profiles.yaml, when the starter profile was added
	// to it; "" when the layer already resolved the name.
	Profile string
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
	// The profiles.yaml the starter profile goes into is put back as it was:
	// the write may have made the file.
	var before []byte
	hadProfile := false
	if written.Profile != "" {
		if text, readErr := os.ReadFile(written.Profile); readErr == nil {
			before, hadProfile = text, true
		}
	}
	fail := func(err error) (Written, error) {
		for i := len(made) - 1; i >= 0; i-- {
			_ = os.Remove(made[i])
		}
		if written.Profile != "" {
			if hadProfile {
				_ = source.WriteBeside(written.Profile, string(before))
			} else {
				_ = os.Remove(written.Profile)
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

// plan is what a write would make: the paths, and, for a starter write, the role
// documents to write and to leave, and the starter profile when the layer cannot
// resolve it.
func plan(in In) (Written, error) {
	base := in.Catalog.LayerDir(in.Layer)
	dir := filepath.Join(base, "teams", in.Name)
	out := Written{}
	for _, file := range in.Files {
		out.Files = append(out.Files, filepath.Join(dir, file.Path))
	}
	if !in.Starter {
		return out, nil
	}
	roles, err := scaffold.Roles(in.Lang)
	if err != nil {
		return Written{}, err
	}
	for _, role := range roles {
		path := filepath.Join(base, "roles", role.Path)
		switch _, err := os.Stat(path); {
		case err == nil:
			out.Kept = append(out.Kept, path)
		case errors.Is(err, os.ErrNotExist):
			out.Roles = append(out.Roles, path)
		default:
			return Written{}, err
		}
	}
	profiles, _, err := in.Catalog.Profiles()
	if err != nil {
		return Written{}, err
	}
	if _, ok := profiles[scaffold.ProfileName]; !ok {
		out.Profile = filepath.Join(base, "profiles.yaml")
	}
	return out, nil
}

// writeFile writes one file, making the directories it needs.
func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// addStarterProfile adds the starter profile to the profiles.yaml at path,
// making the file when the layer has none. An existing file the layer cannot
// read is an error rather than something to write over (I7).
func addStarterProfile(path string) error {
	all := map[string]profile.Profile{}
	switch text, err := os.ReadFile(path); {
	case err == nil:
		if all, err = profile.ParseProfiles(string(text), path); err != nil {
			return err
		}
	case errors.Is(err, os.ErrNotExist):
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
	default:
		return err
	}
	all[scaffold.ProfileName] = profile.Profile{
		Name: scaffold.ProfileName,
		Kind: scaffold.ProfileKind,
		Path: path,
	}
	text, err := profile.EmitProfiles(all)
	if err != nil {
		return err
	}
	return source.WriteBeside(path, text)
}
