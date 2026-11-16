package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// otherProject makes a second directory to list, and a config.yaml that lists it
// with a comment a person wrote.
func (f *fixture) listOther(t *testing.T) (other, config string) {
	t.Helper()
	other = filepath.Join(filepath.Dir(f.proj), "other")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	config = filepath.Join(f.cfg, "hergy", "config.yaml")
	writeFile(t, config, "version: 1\n# the ones serve shows\nprojects:\n  - "+other+"\n")
	return other, config
}

// TestProjectsAddListsTheProject is ac-db73: hergy projects add <directory>
// writes that directory's project at the end of config.yaml's projects:, once,
// and leaves every other line as it was.
func TestProjectsAddListsTheProject(t *testing.T) {
	f := newFixture(t)
	other, config := f.listOther(t)
	before := readFile(t, config)
	sub := filepath.Join(f.proj, "deep", "er")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)

	out := mustRun(t, "projects", "add", ".")
	want := before + "  - " + f.proj + "\n"
	if got := readFile(t, config); got != want {
		t.Fatalf("config.yaml reads:\n%s\nwant:\n%s", got, want)
	}
	if !strings.HasPrefix(out, "added to projects: "+f.proj+" ") {
		t.Errorf("the first add answers: %s", out)
	}

	// The second add says the project was there, not that it was added.
	if out := mustRun(t, "projects", "add", f.proj); !strings.HasPrefix(out, "already in projects: "+f.proj+" ") {
		t.Errorf("the second add answers: %s", out)
	}
	if got := readFile(t, config); got != want {
		t.Errorf("a second add rewrote config.yaml:\n%s", got)
	}

	file := filepath.Join(other, "file")
	writeFile(t, file, "x")
	for _, argv := range [][]string{
		{"projects", "add", file},
		{"projects", "add"},
		{"projects", "remove", f.proj},
		{"projects", "add", f.proj, other},
	} {
		if _, stderr, code := run(argv...); code == 0 || stderr == "" {
			t.Errorf("hergy %v: exit %d, stderr %q", argv, code, stderr)
		}
	}
	if got := readFile(t, config); got != want {
		t.Errorf("a refused add rewrote config.yaml:\n%s", got)
	}
}

// TestProjectsAddStartsTheListWhenThereIsNone: a config.yaml that lists no
// project gets the key with this project under it.
func TestProjectsAddStartsTheListWhenThereIsNone(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "projects", "add", ".")
	got := readFile(t, filepath.Join(f.cfg, "hergy", "config.yaml"))
	if !strings.Contains(got, "projects:\n  - "+f.proj+"\n") {
		t.Errorf("config.yaml reads:\n%s", got)
	}
}

// TestUpSaysWhenServeWillNotShowTheProject is ac-0e64: a team started in a
// project config.yaml's projects: leaves out is one hergy serve does not show,
// and up says so in one line with the command that lists it.
func TestUpSaysWhenServeWillNotShowTheProject(t *testing.T) {
	const command = "hergy projects add ."
	for _, c := range []struct {
		name   string
		config func(f *fixture)
		dry    bool
		want   int
	}{
		{"listed elsewhere only", func(f *fixture) { f.listOther(t) }, false, 1},
		{"listed", func(f *fixture) {
			_, config := f.listOther(t)
			writeFile(t, config, readFile(t, config)+"  - "+f.proj+"\n")
		}, false, 0},
		{"no projects key", func(f *fixture) {
			writeFile(t, filepath.Join(f.cfg, "hergy", "config.yaml"), "version: 1\n")
		}, false, 0},
		{"dry", func(f *fixture) { f.listOther(t) }, true, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			mustRun(t, "new", "demo")
			c.config(f)
			f.statefulHerdr(t)
			t.Setenv("HERDR_ENV", "1")
			t.Setenv("HERDR_PANE_ID", "p9")
			argv := []string{"up", "demo"}
			if c.dry {
				argv = append(argv, "--dry")
			}
			stdout, stderr, code := run(argv...)
			if code != 0 {
				t.Fatalf("up: exit %d\n%s", code, stderr)
			}
			if got := strings.Count(stdout+stderr, command); got != c.want {
				t.Errorf("%q is printed %d times, want %d:\n%s%s", command, got, c.want, stdout, stderr)
			}
		})
	}
}

// TestUpNamesTheProjectWhenRunFromElsewhere: with -C the command is run outside
// the project, so the line names the directory rather than ".".
func TestUpNamesTheProjectWhenRunFromElsewhere(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	other, _ := f.listOther(t)
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	t.Chdir(other)
	stdout, stderr, code := run("up", "demo", "-C", f.proj)
	if code != 0 {
		t.Fatalf("up: exit %d\n%s", code, stderr)
	}
	if want := "hergy projects add " + f.proj; !strings.Contains(stdout+stderr, want) {
		t.Errorf("up does not print %q:\n%s%s", want, stdout, stderr)
	}
}
