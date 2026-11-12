package words

import (
	"strings"
	"testing"
)

func TestTermsHaveUniqueIDsAndNames(t *testing.T) {
	ids, english, japanese := map[TermID]bool{}, map[string]bool{}, map[string]bool{}
	for _, term := range Terms() {
		if term.ID == "" || term.English == "" || term.Japanese == "" || term.Meaning == "" {
			t.Fatalf("incomplete term: %+v", term)
		}
		if ids[term.ID] || english[term.English] || japanese[term.Japanese] {
			t.Fatalf("duplicate term: %+v", term)
		}
		ids[term.ID], english[term.English], japanese[term.Japanese] = true, true, true
		if term.ID.Text("en") != term.English || term.ID.Text("ja") != term.Japanese {
			t.Fatal(term.ID)
		}
	}
}

func TestGeneratedTermsCoverEveryID(t *testing.T) {
	outputs, err := Generated()
	if err != nil {
		t.Fatal(err)
	}
	for _, term := range Terms() {
		for _, path := range []string{"internal/words/terms_generated.go", "internal/web/src/lib/terms.generated.ts"} {
			if !strings.Contains(string(outputs[path]), string(term.ID)) {
				t.Errorf("%s omits %s", path, term.ID)
			}
		}
	}
}
