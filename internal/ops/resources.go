package ops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aq2bq/hergy/internal/definition"
	domainname "github.com/aq2bq/hergy/internal/domain/name"
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/ops/internal/catalog"
	"github.com/aq2bq/hergy/internal/ops/internal/check"
	"github.com/aq2bq/hergy/internal/ops/internal/resourcewrite"
	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/source"
)

// SeenResource is an existing resource observed on disk, including old names
// which cannot be used for a new resource. It never converts to a writable Ref.
type SeenResource struct {
	kind domainplace.Kind
	at   domainplace.Place
	name string
}

// ObserveResource checks only the safety of an existing path component.
func ObserveResource(kind domainplace.Kind, at domainplace.Place, name string) (SeenResource, error) {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00") {
		return SeenResource{}, fmt.Errorf("invalid resource name: %q", name)
	}
	switch kind {
	case domainplace.Definition, domainplace.Role, domainplace.Profile:
	default:
		return SeenResource{}, errors.New("unknown resource kind")
	}
	return SeenResource{kind: kind, at: at, name: name}, nil
}

// Kind reads or changes the selected resource.
func (r SeenResource) Kind() domainplace.Kind { return r.kind }

// Place reads or changes the selected resource.
func (r SeenResource) Place() domainplace.Place { return r.at }

// Name reads or changes the selected resource.
func (r SeenResource) Name() string { return r.name }

// ResourceTarget is either a checked model reference or an existing observation.
// Only SaveResource accepts the model reference alone.
type ResourceTarget struct {
	model *domainplace.Ref
	seen  *SeenResource
}

// ModelResource reads or changes the selected resource.
func ModelResource(ref domainplace.Ref) ResourceTarget { return ResourceTarget{model: &ref} }

// ExistingResource reads or changes the selected resource.
func ExistingResource(ref SeenResource) ResourceTarget { return ResourceTarget{seen: &ref} }
func (r ResourceTarget) values() (domainplace.Kind, domainplace.Place, string) {
	if r.model != nil {
		return r.model.Kind(), r.model.Place(), r.model.Name()
	}
	if r.seen != nil {
		return r.seen.kind, r.seen.at, r.seen.name
	}
	return domainplace.Kind(255), domainplace.Place{}, ""
}

// ResourcePart selects the independently versioned document of a definition.
type ResourcePart uint8

// Resource parts have independent hashes.
const (
	ResourceMain ResourcePart = iota
	ResourceAgreement
)

// ResourceFailure describes a domain refusal. HTTP chooses its representation.
type ResourceFailure struct {
	Text     string
	Field    string
	Path     string
	Conflict bool
	Internal bool
	Missing  bool
	Status   int
}

// Error reads or changes the selected resource.
func (e *ResourceFailure) Error() string { return e.Text }

// ResourceFile is the exact current text and the fingerprint of its whole file.
type ResourceFile struct {
	Path, Text, Hash string
	Exists           bool
}

// ResourceScope supplies catalogs of the registered projects. Global references
// are read in all of them; no HTTP type or transport enters the operation.
type ResourceScope struct {
	Catalog       *catalog.Catalog
	Global        *catalog.Catalog
	Registered    bool
	RenameChecker func(domainplace.Project) error
	Projects      func() ([]*catalog.Catalog, error)
}

func (s ResourceScope) path(target ResourceTarget, part ResourcePart) (string, error) {
	kind, at, name := target.values()
	if name == "" && kind != domainplace.Profile {
		return "", errors.New("resource name is empty")
	}
	if !at.IsGlobal() && at.Project().Compare(s.Catalog.Base()) != domainplace.Same {
		return "", errors.New("resource belongs to another project")
	}
	if part != ResourceMain && (part != ResourceAgreement || kind != domainplace.Definition) {
		return "", errors.New("unknown resource document")
	}
	layer := at.Layer()
	switch kind {
	case domainplace.Definition:
		path := s.Catalog.TeamYAML(name, layer)
		if part == ResourceAgreement {
			path = filepath.Join(filepath.Dir(path), "agreement.md")
		}
		return path, nil
	case domainplace.Role:
		return s.Catalog.RolePath(at.API(), name), nil
	case domainplace.Profile:
		return filepath.Join(s.Catalog.LayerDir(layer), "profiles.yaml"), nil
	}
	return "", errors.New("unknown resource kind")
}

// ReadResource preserves the distinction between missing and unreadable files.
func (s ResourceScope) ReadResource(target ResourceTarget, part ResourcePart) (ResourceFile, *ResourceFailure) {
	path, err := s.path(target, part)
	if err != nil {
		return ResourceFile{}, &ResourceFailure{Text: err.Error()}
	}
	there, readable, why := s.Catalog.FileState(path)
	if !readable {
		if why == "" {
			_, err := s.Catalog.Reader().Read(source.Doc{Path: path})
			if err != nil {
				why = err.Error()
			} else {
				why = "file cannot be read: " + path
			}
		}
		return ResourceFile{}, &ResourceFailure{Text: why, Path: path}
	}
	out := ResourceFile{Path: path, Hash: source.Hash(""), Exists: there}
	if !there {
		return out, nil
	}
	text, err := s.Catalog.Reader().Read(source.Doc{Path: path})
	if err != nil {
		return out, &ResourceFailure{Text: err.Error(), Path: path}
	}
	out.Text, out.Hash = text, source.Hash(text)
	return out, nil
}

// ResourceEdit expresses the kind-specific change. Profile edits apply to the
// complete containing file; definition operations preserve untouched lines.
type ResourceEdit struct {
	Part     ResourcePart
	Text     string
	Ops      []definition.Op
	Profile  *profile.Draft
	Original *SeenResource
	Hash     string
	// MissingHash preserves the role panel's historical empty missing-file token.
	MissingHash bool
	RenameUsers bool
	Create      bool
}

// ResourceSaved is the shared resource model.
type ResourceSaved struct {
	ResourceFile
	PreviousPath string
	References   []string
}

type resourceDefinition struct {
	read  func() (ResourceFile, *ResourceFailure)
	edit  func(ResourceFile) (string, *ResourceFailure)
	check func(ResourceFile, string) *ResourceFailure
	write func(ResourceFile, string) *ResourceFailure
	after func() *ResourceFailure
}

// saveResource is the only saving sequence. Kind definitions cannot reorder
// reading, unreadable refusal, conflict, validation, conditional write or cleanup.
func saveResource(expected string, missingHash bool, d resourceDefinition) (ResourceSaved, *ResourceFailure) {
	before, bad := d.read()
	if bad != nil {
		return ResourceSaved{}, bad
	}
	hash := before.Hash
	if !before.Exists && missingHash {
		hash = ""
	}
	if expected != hash {
		return ResourceSaved{}, &ResourceFailure{Text: "changed", Path: before.Path, Conflict: true}
	}
	after, bad := d.edit(before)
	if bad != nil {
		return ResourceSaved{}, bad
	}
	if bad = d.check(before, after); bad != nil {
		return ResourceSaved{}, bad
	}
	if bad = d.write(before, after); bad != nil {
		return ResourceSaved{}, bad
	}
	if d.after != nil {
		if bad = d.after(); bad != nil {
			return ResourceSaved{}, bad
		}
	}
	return ResourceSaved{ResourceFile: ResourceFile{Path: before.Path, Text: after, Hash: source.Hash(after), Exists: true}}, nil
}

// SaveResource writes a checked destination through the common resource sequence.
// A rename preflights every reference before the first write and writes the
// references before changing the resource's own name.
func (s ResourceScope) SaveResource(ref domainplace.Ref, in ResourceEdit) (ResourceSaved, *ResourceFailure) {
	if ref.Name() == "" {
		return ResourceSaved{}, &ResourceFailure{Field: "name", Text: "resource name is empty"}
	}
	return s.saveResourceRef(ref, in)
}
func (s ResourceScope) saveResourceRef(ref domainplace.Ref, in ResourceEdit) (ResourceSaved, *ResourceFailure) {
	if in.Original != nil && (in.Original.Kind() != ref.Kind() || in.Original.Place() != ref.Place()) {
		return ResourceSaved{}, &ResourceFailure{Text: "rename source and destination have different kinds or places"}
	}
	target := ModelResource(ref)
	old := target
	if in.Original != nil {
		old = ExistingResource(*in.Original)
	}
	path, err := s.path(target, in.Part)
	if err != nil {
		return ResourceSaved{}, &ResourceFailure{Text: err.Error()}
	}
	oldPath, err := s.path(old, in.Part)
	if err != nil {
		return ResourceSaved{}, &ResourceFailure{Text: err.Error()}
	}
	if bad := s.layerReady(ref); bad != nil {
		return ResourceSaved{}, bad
	}
	var refs []resourceRewrite
	d := resourceDefinition{
		read: func() (ResourceFile, *ResourceFailure) {
			return s.readResourceReplacement(ref, old, target, in, path, oldPath)
		},
		edit: func(before ResourceFile) (string, *ResourceFailure) { return s.editResourceText(path, before, in) },
		check: func(before ResourceFile, text string) *ResourceFailure {
			if bad := s.checkResourceText(ref, in.Part, before, text); bad != nil {
				// A complete YAML text historically omits its own file prefix;
				// operation-based edits keep the checker's complete sentence.
				if ref.Kind() == domainplace.Definition && in.Part == ResourceMain && in.Ops == nil {
					bad.Text = strings.TrimPrefix(bad.Text, s.Catalog.Display(path)+": ")
				}
				return bad
			}
			if in.RenameUsers && in.Original != nil && in.Original.Name() != ref.Name() {
				var bad *ResourceFailure
				refs, bad = s.prepareResourceReferences(*in.Original, ref)
				return bad
			}
			return nil
		},
		write: func(before ResourceFile, text string) *ResourceFailure {
			if bad := writeResourceReferences(refs); bad != nil {
				return bad
			}
			hash := before.Hash
			if oldPath != path {
				hash = source.Hash("")
			}
			if err := resourcewrite.WriteIfUnchanged(path, hash, text); err != nil {
				return resourceWriteFailure(err, path, refs)
			}
			return nil
		},
		after: func() *ResourceFailure {
			if oldPath != path {
				if err := os.Remove(oldPath); err != nil {
					return resourceWriteFailure(err, oldPath, refs)
				}
			}
			return nil
		},
	}
	out, bad := saveResource(in.Hash, in.MissingHash, d)
	if bad == nil {
		out.PreviousPath = oldPath
		out.Path = path
		for _, one := range refs {
			out.References = append(out.References, one.path)
		}
	}
	return out, bad
}

func (s ResourceScope) layerReady(ref domainplace.Ref) *ResourceFailure {
	dir := s.Catalog.LayerDir(ref.Place().Layer())
	key := "error.teams_no_layer"
	switch ref.Kind() {
	case domainplace.Role:
		key = "error.roles_no_layer"
	case domainplace.Profile:
		key = "error.profiles_no_layer"
	default:
	}
	if _, err := os.Stat(dir); err != nil {
		return &ResourceFailure{Text: i18n.Tf(key, s.Catalog.Display(dir))}
	}
	if ref.Kind() == domainplace.Role {
		if err := os.MkdirAll(filepath.Join(dir, "roles"), 0755); err != nil {
			return &ResourceFailure{Text: err.Error()}
		}
	}
	return nil
}
func resourceWriteFailure(err error, path string, refs []resourceRewrite) *ResourceFailure {
	if len(refs) > 0 {
		return referenceWriteFailure(err, refs, len(refs), path)
	}
	return &ResourceFailure{Text: err.Error(), Path: path, Conflict: errors.Is(err, resourcewrite.ErrChanged), Internal: !errors.Is(err, resourcewrite.ErrChanged)}
}

func (s ResourceScope) checkResourceText(ref domainplace.Ref, part ResourcePart, before ResourceFile, text string) *ResourceFailure {
	known := map[string]bool{}
	// Profiles have always refused every error in the resulting whole file.
	if before.Exists && ref.Kind() != domainplace.Profile {
		for _, p := range s.resourceProblems(ref, part, before.Path, before.Text) {
			known[p] = true
		}
	}
	for _, p := range s.resourceProblems(ref, part, before.Path, text) {
		if !known[p] {
			return &ResourceFailure{Text: p}
		}
	}
	return nil
}
func (s ResourceScope) resourceProblems(ref domainplace.Ref, part ResourcePart, path, text string) []string {
	switch ref.Kind() {
	case domainplace.Definition:
		if part == ResourceAgreement {
			return documentStops(check.Agreement(check.AgreementInput{Catalog: s.Catalog, Name: ref.Name(), Place: ref.Place().Layer(), Text: text}))
		}
		var out []string
		for _, p := range check.Team(check.TeamInput{Catalog: s.Catalog.With(path, text), Name: ref.Name(), Layer: ref.Place().Layer()}).Problems {
			if p.Stops && (p.Kind != check.KindGraph || !strings.HasPrefix(p.Field, "node:")) {
				out = append(out, p.Text)
			}
		}
		return out
	case domainplace.Role:
		return documentStops(check.Role(check.RoleInput{Catalog: s.Catalog, Name: ref.Name(), Place: ref.Place().Layer(), Text: text}))
	case domainplace.Profile:
		_, ps := profile.ReadAll(text, s.Catalog.Display(path))
		var out []string
		for _, p := range ps {
			if p.Sev == profile.Err {
				out = append(out, p.Text)
			}
		}
		return out
	}
	return nil
}
func documentStops(report check.DocReport) []string {
	var out []string
	for _, p := range report.Problems {
		if p.Sev == "err" {
			out = append(out, p.Text)
		}
	}
	return out
}

// NewResourceRef is the boundary parser for a new, checked destination.
func NewResourceRef(kind domainplace.Kind, at domainplace.Place, n string) (domainplace.Ref, error) {
	switch kind {
	case domainplace.Definition:
		v, e := domainname.ParseDefinitionName(n)
		return domainplace.DefinitionRef(at, v), e
	case domainplace.Role:
		v, e := domainname.ParseRoleName(n)
		if e != nil {
			return domainplace.Ref{}, i18n.Errorf("error.role_name_pattern", n)
		}
		return domainplace.RoleRef(at, v), nil
	case domainplace.Profile:
		v, e := domainname.ParseProfileName(n)
		return domainplace.ProfileRef(at, v), e
	}
	return domainplace.Ref{}, errors.New("unknown resource kind")
}

// SaveProfilesFile saves the containing file, whose conflict unit is all profiles.
func (s ResourceScope) SaveProfilesFile(at domainplace.Place, hash, text string) (ResourceSaved, *ResourceFailure) {
	return s.saveResourceRef(domainplace.ProfileRef(at, domainname.ProfileName{}), ResourceEdit{Text: text, Hash: hash})
}

func (s ResourceScope) editResourceText(path string, before ResourceFile, in ResourceEdit) (string, *ResourceFailure) {
	text := in.Text
	if in.Profile != nil {
		entries, _ := profile.ReadAll(before.Text, s.Catalog.Display(path))
		original := ""
		if in.Original != nil {
			original = in.Original.Name()
		}
		out, err := editResourceProfile(before.Text, s.Catalog.Display(path), entries, *in.Profile, original)
		if err != nil {
			return "", &ResourceFailure{Text: err.Error()}
		}
		text = out
	}
	if in.Ops != nil {
		edited, err := editDefinitionText(before.Text, s.Catalog.Display(path), in.Ops)
		if err != nil {
			return "", &ResourceFailure{Text: err.Error()}
		}
		text = edited
	}
	return text, nil
}

// CopyResource saves the role/profile snapshot supplied by the caller. A
// definition copy uses CreateDefinition and reads its source again.
func (s ResourceScope) CopyResource(ref domainplace.Ref, in ResourceEdit) (ResourceSaved, *ResourceFailure) {
	in.Original = nil
	in.Create = true
	return s.SaveResource(ref, in)
}

// RenameResource uses a checked new destination and an observed source.
func (s ResourceScope) RenameResource(old SeenResource, next domainplace.Ref, in ResourceEdit) (ResourceSaved, *ResourceFailure) {
	in.Original = &old
	return s.SaveResource(next, in)
}

func (s ResourceScope) readResourceReplacement(ref domainplace.Ref, old, target ResourceTarget, in ResourceEdit, path, oldPath string) (ResourceFile, *ResourceFailure) {
	current, bad := s.ReadResource(old, in.Part)
	if bad != nil {
		return current, bad
	}
	if in.Create && ref.Kind() == domainplace.Role && current.Exists {
		return current, &ResourceFailure{Field: "name", Text: i18n.Tf("error.role_name_taken", ref.Name(), s.Catalog.Display(path))}
	}
	if oldPath != path {
		if _, bad := s.ReadResource(target, in.Part); bad != nil {
			return current, bad
		}
		if _, err := os.Stat(path); err == nil {
			return current, &ResourceFailure{Field: "name", Text: i18n.Tf("error.role_name_taken", ref.Name(), s.Catalog.Display(path))}
		}
	}
	return current, nil
}
