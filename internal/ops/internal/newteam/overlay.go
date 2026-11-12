package newteam

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/aq2bq/hergy/internal/ops/internal/catalog"
	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/scaffold"
	"github.com/aq2bq/hergy/internal/source"
)

// Overlay prepares exactly the files Write plans, without writing any of them.
func Overlay(in In) (*catalog.Catalog, error) {
	planned, err := plan(in)
	if err != nil {
		return nil, err
	}
	cat := in.Catalog
	dir := filepath.Join(cat.LayerDir(in.Layer), "teams", in.Name)
	for _, file := range in.Files {
		cat = cat.With(filepath.Join(dir, file.Path), string(file.Data))
	}
	starter, err := scaffold.Roles(in.Lang)
	if err != nil {
		return nil, err
	}
	for _, path := range planned.Roles {
		for _, file := range starter {
			if filepath.Base(path) == file.Path {
				cat = cat.With(path, string(file.Data))
			}
		}
	}
	for _, path := range planned.CarryRoles {
		doc, ok := in.Carry.Catalog.LayerRole(in.Carry.Layer, strings.TrimSuffix(filepath.Base(path), ".md"))
		if !ok {
			return nil, errors.New("role document disappeared")
		}
		text, err := in.Carry.Catalog.Reader().Read(doc)
		if err != nil {
			return nil, err
		}
		cat = cat.With(path, text)
	}
	path := planned.Profile
	if path == "" {
		path = planned.CarryProfileFile
	}
	if path == "" {
		return cat, nil
	}
	text, err := cat.Reader().Read(source.Doc{Path: path})
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		text = "version: 1\n"
	}
	var add []profile.Op
	if planned.Profile != "" {
		add = append(add, profile.Add(scaffold.ProfileName, scaffold.ProfileKind))
	}
	carried, err := overlayProfileOps(in, planned)
	if err != nil {
		return nil, err
	}
	add = append(add, carried...)
	for _, op := range add {
		out, err := profile.EditYAML([]byte(text), path, op)
		if err != nil {
			return nil, err
		}
		text = string(out)
	}
	return cat.With(path, text), nil
}

func overlayProfileOps(in In, planned Written) ([]profile.Op, error) {
	var add []profile.Op
	if len(planned.CarryProfiles) > 0 {
		profiles, err := in.Carry.Catalog.LayerProfiles(in.Carry.Layer)
		if err != nil {
			return nil, err
		}
		for _, name := range planned.CarryProfiles {
			p := profiles[name]
			add = append(add, profile.Add(name, p.Kind))
			if len(p.Args) > 0 {
				add = append(add, profile.SetArgs(name, p.Args))
			}
			if len(p.Env) > 0 {
				var env []profile.EnvPair
				for _, key := range slices.Sorted(maps.Keys(p.Env)) {
					env = append(env, profile.EnvPair{Key: key, Value: p.Env[key]})
				}
				add = append(add, profile.SetEnvInOrder(name, env))
			}
		}
	}
	return add, nil
}
