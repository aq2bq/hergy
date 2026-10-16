package definition

import (
	"reflect"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/cheat"
	"go.yaml.in/yaml/v3"
)

// TestCheatMachineNamesEveryConfigKey collects every yaml key team.yaml's
// settings structs read, by reflection over their yaml tags, and checks the
// agent cheatsheet names each one, so a key added later cannot be left out.
func TestCheatMachineNamesEveryConfigKey(t *testing.T) {
	keys := map[string]bool{}
	roots := []any{
		rawTeam{}, rawMember{}, rawWorker{},
	}
	for _, root := range roots {
		collectYAMLKeys(keys, reflect.TypeOf(root))
	}
	if len(keys) == 0 {
		t.Fatal("no yaml keys were collected; the reflection walk is broken")
	}
	for key := range keys {
		if !strings.Contains(cheat.Machine, key) {
			t.Errorf("cheat --ai does not name the settings key %q", key)
		}
	}
}

// collectYAMLKeys adds every yaml tag under a type to keys, descending into the
// settings structs of this package only, so the yaml library's own structs are
// left alone. Pointers, slices, arrays and maps are unwrapped.
func collectYAMLKeys(keys map[string]bool, typ reflect.Type) {
	if typ == nil {
		return
	}
	switch typ.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
		collectYAMLKeys(keys, typ.Elem())
	case reflect.Struct:
		if typ.PkgPath() != reflect.TypeFor[rawTeam]().PkgPath() {
			return
		}
		for field := range typ.Fields() {
			if field.Type == reflect.TypeFor[yaml.Node]() {
				// A tombstone: the key is gone, and the raw node is kept only
				// so a file that still writes it is stopped with the sentence
				// that says where it went (段 5). It is not a key a person
				// writes, so the cheatsheet does not name it.
				continue
			}
			if tag, _, _ := strings.Cut(field.Tag.Get("yaml"), ","); tag != "" && tag != "-" {
				keys[tag] = true
			}
			collectYAMLKeys(keys, field.Type)
		}
	default:
		// scalars carry no yaml keys.
	}
}
