package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/source"
)

// changedFile is the file a 409 answers with, as these tests read it.
type changedFile struct {
	Path    string         `json:"path"`
	Exists  bool           `json:"exists"`
	Hash    string         `json:"hash"`
	Text    string         `json:"text"`
	Changed string         `json:"changed"`
	Doc     map[string]any `json:"doc"`
}

// conflictOf reads the file out of a 409's answer.
func conflictOf(t *testing.T, decoded map[string]any) changedFile {
	t.Helper()
	raw, err := json.Marshal(decoded["file"])
	if err != nil {
		t.Fatal(err)
	}
	var out changedFile
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestROLES_3C77_AChangedFileAnswersTheFileAndItsReading: a save whose file moved
// under the screen is refused with the file as it is now — its path, whether it
// is there, its hash, its text, the hour it was written, and the file as the
// kind reads it — so the screen keeps the draft and can read the file in without
// knowing its format (r-3c77, d-a34a).
func TestROLES_3C77_AChangedFileAnswersTheFileAndItsReading(t *testing.T) {
	f := ruFixtureRoles(t)
	server := f.server(t)
	path := ruRolePath(f, catalog.PlaceProject, "builder")
	read := readFile(t, path)
	draft, _ := ruLoadDraft(t, path)
	changed := read + "\n# ほかの所で変わった\n"
	if err := os.WriteFile(path, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	resp, decoded := f.apiJSON(t, server, http.MethodPut, "/api/role/builder", map[string]any{
		"place": catalog.PlaceProject, "original": "builder", "hash": source.Hash(read), "doc": draft,
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("the save answered %d, want 409", resp.StatusCode)
	}
	file := conflictOf(t, decoded)
	if file.Path != path || !file.Exists {
		t.Errorf("the answer names %q (there: %v), want the file that moved", file.Path, file.Exists)
	}
	if file.Text != changed {
		t.Errorf("the answer carries a text the file does not hold")
	}
	if file.Hash != source.Hash(changed) {
		t.Errorf("the answer's hash is not the text's: %q", file.Hash)
	}
	if file.Changed == "" {
		t.Errorf("the answer says no hour for a file that was just written")
	}
	// The file's own reading travels with it, so 元に戻す puts the file in
	// place without the screen knowing its format.
	if file.Doc == nil {
		t.Errorf("the answer carries no reading of the file")
	}
	if want := i18n.Tf("error.file_changed", path, file.Changed); decoded["text"] != want {
		t.Errorf("the refusal is %q, want %q", decoded["text"], want)
	}
	if got := readFile(t, path); got != changed {
		t.Errorf("the refused save wrote the file:\n%s", got)
	}
}

// TestROLES_3C77_TheOverwriteWritesAndAStaleOneIsRefusedAgain: the save with the
// file's own hash writes what the person wrote, and a save with a hash that has
// moved again is refused the same way (r-3c77).
func TestROLES_3C77_TheOverwriteWritesAndAStaleOneIsRefusedAgain(t *testing.T) {
	f := ruFixtureRoles(t)
	server := f.server(t)
	path := ruRolePath(f, catalog.PlaceProject, "builder")
	read := readFile(t, path)
	draft, _ := ruLoadDraft(t, path)
	// The person is in the middle of a change, and another place wrote one too.
	draft.Sections[0].Rules[0].Text = "書き換えている途中の本文。"
	changed := read + "\n# ほかの所で変わった\n"
	if err := os.WriteFile(path, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	_, decoded := f.apiJSON(t, server, http.MethodPut, "/api/role/builder", map[string]any{
		"place": catalog.PlaceProject, "original": "builder", "hash": source.Hash(read), "doc": draft,
	})
	seen := conflictOf(t, decoded)
	resp, decoded := f.apiJSON(t, server, http.MethodPut, "/api/role/builder", map[string]any{
		"place": catalog.PlaceProject, "original": "builder", "hash": seen.Hash, "doc": draft,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the overwrite answered %d, want 200: %v", resp.StatusCode, decoded["text"])
	}
	if got := readFile(t, path); !strings.Contains(got, "書き換えている途中の本文。") {
		t.Errorf("the overwrite did not write the words the person wrote:\n%s", got)
	}
	// The seen hash is old now: the same overwrite is refused the same way.
	resp, decoded = f.apiJSON(t, server, http.MethodPut, "/api/role/builder", map[string]any{
		"place": catalog.PlaceProject, "original": "builder", "hash": seen.Hash, "doc": draft,
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("the second overwrite answered %d, want 409", resp.StatusCode)
	}
	if file := conflictOf(t, decoded); !file.Exists || file.Hash == seen.Hash {
		t.Errorf("the second refusal does not carry the file as it is now: %+v", file)
	}
}

// TestROLES_3C77_AFileThatIsGoneIsToldSoAndCanBeWrittenAgain: a file deleted in
// another place answers Exists false and its own sentence, and the same save
// with the empty hash writes the draft as a new file (r-3c77).
func TestROLES_3C77_AFileThatIsGoneIsToldSoAndCanBeWrittenAgain(t *testing.T) {
	f := ruFixtureRoles(t)
	server := f.server(t)
	path := ruRolePath(f, catalog.PlaceProject, "builder")
	read := readFile(t, path)
	draft, _ := ruLoadDraft(t, path)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	resp, decoded := f.apiJSON(t, server, http.MethodPut, "/api/role/builder", map[string]any{
		"place": catalog.PlaceProject, "original": "builder", "hash": source.Hash(read), "doc": draft,
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("the save answered %d, want 409", resp.StatusCode)
	}
	file := conflictOf(t, decoded)
	if file.Exists || file.Text != "" || file.Hash != "" {
		t.Errorf("the answer says the file is still there: %+v", file)
	}
	if want := i18n.Tf("error.file_gone", path); decoded["text"] != want {
		t.Errorf("the refusal is %q, want %q", decoded["text"], want)
	}
	resp, decoded = f.apiJSON(t, server, http.MethodPut, "/api/role/builder", map[string]any{
		"place": catalog.PlaceProject, "original": "builder", "hash": "", "doc": draft,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("writing the draft again answered %d, want 200: %v", resp.StatusCode, decoded["text"])
	}
	if got := readFile(t, path); !strings.Contains(got, "version: 1") {
		t.Errorf("the file written again is not the draft the panel held:\n%s", got)
	}
}

// TestROLES_3C77_AStaleDeleteSaysItDidNotDelete: a delete whose file moved is
// refused with the words for a delete, and the file stays (r-3c77).
func TestROLES_3C77_AStaleDeleteSaysItDidNotDelete(t *testing.T) {
	f := ruFixtureRoles(t)
	server := f.server(t)
	path := ruRolePath(f, catalog.PlaceProject, "builder")
	read := readFile(t, path)
	changed := read + "\n# ほかの所で変わった\n"
	if err := os.WriteFile(path, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	resp, decoded := f.apiJSON(t, server, http.MethodDelete, "/api/role/builder?place=project&hash="+source.Hash(read), nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("the delete answered %d, want 409", resp.StatusCode)
	}
	if want := i18n.Tf("error.file_changed_delete", path); decoded["text"] != want {
		t.Errorf("the refusal is %q, want %q", decoded["text"], want)
	}
	if got := readFile(t, path); got != changed {
		t.Errorf("the refused delete took the file away")
	}
}

// TestTEAMS_3C77_TheAgreementAndItsTextAnswerWhatChanged: the team's own rules
// and the YAML tab answer the same way (r-3c77).
func TestTEAMS_3C77_TheAgreementAndItsTextAnswerWhatChanged(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml")
	before := readFile(t, path)
	doc := map[string]any{"name": "", "title": "demo チームの約束", "revision": 1,
		"sections": []map[string]any{{"heading": "使命", "rules": []map[string]any{
			{"id": "", "text": "足したルール。", "in_prompt": true},
		}}}}
	changed := before + "\n# ほかの所で変わった\n"
	if err := os.WriteFile(path, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	resp, decoded := f.apiJSON(t, server, http.MethodPut, "/api/team/demo", map[string]any{
		"place": "project", "part": "agreement", "hash": source.Hash(before),
		"doc": map[string]any{"ops": []map[string]any{}, "doc": doc},
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("the agreement save answered %d, want 409", resp.StatusCode)
	}
	file := conflictOf(t, decoded)
	if file.Text != changed || file.Doc == nil {
		t.Errorf("the answer does not carry the file and its reading: %+v", file)
	}
	// The YAML tab: the text the person typed is the draft.
	resp, decoded = f.apiJSON(t, server, http.MethodPut, "/api/team/demo", map[string]any{
		"place": "project", "part": "agreement", "name": "demo", "hash": source.Hash(before),
		"text": "version: 1\nsections: []\n",
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("the YAML save answered %d, want 409", resp.StatusCode)
	}
	if file := conflictOf(t, decoded); file.Text != changed {
		t.Errorf("the YAML answer's text is not the file as it is: %q", file.Text)
	}
}

// TestPROFILES_3C77_ThePanelAnswersWhatChanged: the profile panel's save answers
// the file and the panel's own text (r-3c77).
func TestPROFILES_3C77_ThePanelAnswersWhatChanged(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	before := readFile(t, path)
	changed := before + "# ほかの所で変わった\n"
	if err := os.WriteFile(path, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	resp, decoded := f.apiJSON(t, server, http.MethodPut, "/api/profile/coder", map[string]any{
		"place": "project", "original": "coder", "hash": source.Hash(before),
		"doc": map[string]any{"name": "coder", "kind": "gemini", "args": []string{}, "env": []map[string]string{}},
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("the profile save answered %d, want 409", resp.StatusCode)
	}
	file := conflictOf(t, decoded)
	if file.Text != changed || file.Doc == nil {
		t.Errorf("the answer does not carry the file and its reading: %+v", file)
	}
}

// TestCHANGED_3C77_TheHourIsTheDayForAnotherDay: the hour a person reads is the
// time of day for today and the day with it for another day (r-3c77).
func TestCHANGED_3C77_TheHourIsTheDayForAnotherDay(t *testing.T) {
	now := time.Now()
	if got, want := changedClock(now), now.Format("15:04"); got != want {
		t.Errorf("today's hour is %q, want %q", got, want)
	}
	other := now.AddDate(0, 0, -3)
	if got := changedClock(other); !strings.Contains(got, other.Format("15:04")) || strings.Contains(got, " ") == false {
		t.Errorf("another day's hour is %q, want the day and the hour", got)
	}
	if got := changedClock(time.Time{}); got != "" {
		t.Errorf("a file with no time answers %q, want nothing", got)
	}
}
