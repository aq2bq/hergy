package ops

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aq2bq/hergy/internal/definition"
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/ops/internal/catalog"
	"github.com/aq2bq/hergy/internal/ops/internal/resourcewrite"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/words"
	"go.yaml.in/yaml/v3"
)

type resourceReference struct {
	cat   *catalog.Catalog
	path  string
	ops   []definition.Op
	users int
}
type resourceRewrite struct{ path, before, after string }

// referenceDefinitions covers every physical definition that can read the
// resource, including hidden definitions. Project overrides do not read a global
// role/profile of the same name. Global definitions are visited once.
func (s ResourceScope) referenceDefinitions(old SeenResource) ([]resourceReference, []string) {
	cats := []*catalog.Catalog{s.Catalog}
	if old.at.IsGlobal() && s.Projects != nil {
		all, err := s.Projects()
		if err != nil {
			return nil, []string{err.Error()}
		}
		cats = all
	}
	seen := map[string]bool{}
	var out []resourceReference
	var failures []string
	for _, cat := range cats {
		refs, bad := resourceReferencesAt(cat, source.LayerLocal, old, seen)
		out = append(out, refs...)
		failures = append(failures, bad...)
	}
	if old.at.IsGlobal() {
		refs, bad := resourceReferencesAt(s.Catalog, source.LayerGlobal, old, seen)
		out = append(out, refs...)
		failures = append(failures, bad...)
	}
	return out, failures
}
func resourceReferencesAt(cat *catalog.Catalog, layer string, old SeenResource, seen map[string]bool) ([]resourceReference, []string) {
	if old.at.IsGlobal() && layer == source.LayerLocal {
		shadowed, err := resourceShadowed(cat, old)
		if err != nil {
			return nil, []string{err.Error()}
		}
		if shadowed {
			return nil, nil
		}
	}
	var out []resourceReference
	var failures []string
	for _, row := range cat.Definitions() {
		path := row.LocalPath
		if layer == source.LayerGlobal {
			path = row.GlobalPath
		}
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		ref, err := resourceReferenceIn(cat, path, old)
		if err != nil {
			failures = append(failures, path+": "+err.Error())
			continue
		}
		if ref.users > 0 {
			out = append(out, ref)
		}
	}
	return out, failures
}
func resourceShadowed(cat *catalog.Catalog, old SeenResource) (bool, error) {
	if old.kind == domainplace.Role {
		_, held := cat.LayerRole(source.LayerLocal, old.name)
		return held, nil
	}
	profiles, err := cat.LayerProfiles(source.LayerLocal)
	_, held := profiles[old.name]
	return held, err
}
func resourceReferenceIn(cat *catalog.Catalog, path string, old SeenResource) (resourceReference, error) {
	ref := resourceReference{cat: cat, path: path}
	if _, ok, why := cat.FileState(path); !ok {
		return ref, errors.New(why)
	}
	text, err := cat.Reader().Read(source.Doc{Path: path})
	if err != nil {
		return ref, err
	}
	var raw struct {
		Members []struct{ Name, Role, Profile string }
		Workers []struct{ From, Purpose, Profile string }
	}
	if err := yaml.Unmarshal([]byte(text), &raw); err != nil {
		return ref, err
	}
	for _, m := range raw.Members {
		if old.kind == domainplace.Role && m.Role == old.name {
			ref.users++
			ref.ops = append(ref.ops, definition.MemberRole(m.Name, ""))
		}
		if old.kind == domainplace.Profile && m.Profile == old.name {
			ref.users++
			ref.ops = append(ref.ops, definition.MemberProfile(m.Name, ""))
		}
	}
	if old.kind == domainplace.Profile {
		for _, w := range raw.Workers {
			if w.Profile == old.name {
				ref.users++
				ref.ops = append(ref.ops, definition.WorkerProfile(w.From, w.Purpose, ""))
			}
		}
	}
	return ref, nil
}

// ResourceUsers counts actual readers throughout the registered project scope.
func (s ResourceScope) ResourceUsers(target ResourceTarget) (int, *ResourceFailure) {
	kind, at, n := target.values()
	refs, _ := s.referenceDefinitions(SeenResource{kind: kind, at: at, name: n})
	count := 0
	for _, ref := range refs {
		count += ref.users
	}
	return count, nil
}
func (s ResourceScope) prepareResourceReferences(old SeenResource, next domainplace.Ref) ([]resourceRewrite, *ResourceFailure) {
	refs, failures := s.referenceDefinitions(old)
	var writes []resourceRewrite
	for _, ref := range refs {
		text, err := ref.cat.Reader().Read(source.Doc{Path: ref.path})
		if err != nil {
			failures = append(failures, ref.path+": "+err.Error())
			continue
		}
		edited := text
		for _, op := range ref.ops {
			// The parser records who refers to the name; only the destination changes.
			op = resourceReferenceDestination(op, next.Name())
			data, err := definition.EditYAML([]byte(edited), ref.cat.Display(ref.path), op)
			if err != nil {
				failures = append(failures, ref.path+": "+err.Error())
				break
			}
			edited = string(data)
		}
		if edited != text {
			writes = append(writes, resourceRewrite{path: ref.path, before: text, after: edited})
		}
	}
	if len(failures) > 0 {
		return nil, &ResourceFailure{Text: words.Errorf("resources.references-unreadable", i18n.Language(), strings.Join(failures, "\n")).Error()}
	}
	return writes, nil
}
func writeResourceReferences(refs []resourceRewrite) *ResourceFailure {
	for i, ref := range refs {
		if err := resourcewrite.WriteIfUnchanged(ref.path, source.Hash(ref.before), ref.after); err != nil {
			return referenceWriteFailure(err, refs, i, ref.path)
		}
	}
	return nil
}
func referenceWriteFailure(err error, refs []resourceRewrite, done int, target string) *ResourceFailure {
	completed := make([]string, 0, done)
	pending := make([]string, 0, len(refs)-done+1)
	for i, ref := range refs {
		if i < done {
			completed = append(completed, ref.path)
		} else {
			pending = append(pending, ref.path)
		}
	}
	if done == len(refs) {
		pending = append(pending, target)
	}
	return &ResourceFailure{Text: words.Errorf("resources.references-write", i18n.Language(), strings.Join(completed, "\n"), strings.Join(pending, "\n"), fmt.Sprint(err)).Error(), Path: target, Internal: true, Conflict: errors.Is(err, resourcewrite.ErrChanged)}
}

func resourceReferenceDestination(op definition.Op, n string) definition.Op {
	if op.Kind == definition.OpSetMemberRole {
		op.Role = n
	} else {
		op.Profile = n
	}
	return op
}
