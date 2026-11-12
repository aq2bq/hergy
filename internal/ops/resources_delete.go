package ops

import (
	"errors"
	"os"
	"path/filepath"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/ops/internal/resourcewrite"
	"github.com/aq2bq/hergy/internal/profile"
)

// ResourceDelete versions both definition documents independently.
type ResourceDelete struct{ Hash, AgreementHash string }

// DeleteResource removes only the resource's owned files. Profiles remove a key
// from the shared file; definition folders keep files not owned by hergy.
func (s ResourceScope) DeleteResource(target ResourceTarget, in ResourceDelete) *ResourceFailure {
	kind, _, name := target.values()
	if kind == domainplace.Profile {
		before, bad := s.ReadResource(target, ResourceMain)
		if bad != nil {
			return bad
		}
		if before.Hash != in.Hash {
			return &ResourceFailure{Text: "changed", Path: before.Path, Conflict: true}
		}
		edited, err := profile.EditYAML([]byte(before.Text), s.Catalog.Display(before.Path), profile.Remove(name))
		if err != nil {
			return &ResourceFailure{Text: err.Error()}
		}
		if err := resourcewrite.WriteIfUnchanged(before.Path, before.Hash, string(edited)); err != nil {
			return resourceWriteFailure(err, before.Path, nil)
		}
		return nil
	}
	parts := []ResourcePart{ResourceMain}
	hashes := []string{in.Hash}
	if kind == domainplace.Definition {
		parts = append(parts, ResourceAgreement)
		hashes = append(hashes, in.AgreementHash)
	}
	files := make([]ResourceFile, 0, len(parts))
	for i, part := range parts {
		before, bad := s.ReadResource(target, part)
		if bad != nil {
			return bad
		}
		// Definition deletion historically accepts an omitted hash. Missing files
		// have no slot in the removal list, but retain their own version identity.
		if (kind != domainplace.Definition || hashes[i] != "") && before.Hash != hashes[i] {
			return &ResourceFailure{Text: "changed", Path: before.Path, Conflict: true}
		}
		files = append(files, before)
	}
	for _, file := range files {
		if !file.Exists {
			continue
		}
		if err := os.Remove(file.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return &ResourceFailure{Text: err.Error(), Internal: true}
		}
	}
	if kind == domainplace.Definition && len(files) > 0 {
		_ = os.Remove(filepath.Dir(files[0].Path))
	}
	return nil
}

// ResourceLoss is a resource's deletion footprint; rendering chooses words.
type ResourceLoss struct {
	Files, Leftovers       []string
	RemoveDirectory, Found bool
	Users                  int
}

// ResourceLoss reads or changes the selected resource.
func (s ResourceScope) ResourceLoss(target ResourceTarget) (ResourceLoss, *ResourceFailure) {
	kind, _, _ := target.values()
	var out ResourceLoss
	file, bad := s.ReadResource(target, ResourceMain)
	if bad != nil {
		return out, bad
	}
	if kind == domainplace.Profile {
		_, _, n := target.values()
		entries, _ := profile.ReadAll(file.Text, s.Catalog.Display(file.Path))
		for _, e := range entries {
			if e.Name == n {
				out.Found = true
			}
		}
		out.Files = []string{file.Path}
	}
	if kind == domainplace.Role {
		out.Found = file.Exists
		out.Files = []string{file.Path}
	}
	if kind == domainplace.Definition {
		agree, bad := s.ReadResource(target, ResourceAgreement)
		if bad != nil {
			return out, bad
		}
		for _, one := range []ResourceFile{file, agree} {
			if one.Exists {
				out.Files = append(out.Files, one.Path)
			}
		}
		entries, _ := os.ReadDir(filepath.Dir(file.Path))
		for _, entry := range entries {
			if entry.Name() != "team.yaml" && entry.Name() != "agreement.md" {
				out.Leftovers = append(out.Leftovers, entry.Name())
			}
		}
		out.Found = len(out.Files) > 0
		out.RemoveDirectory = len(out.Leftovers) == 0
		return out, nil
	}
	if out.Found {
		out.Users, bad = s.ResourceUsers(target)
	}
	return out, bad
}
