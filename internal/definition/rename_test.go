package definition

import (
	"strings"
	"testing"
)

func TestRenameYAMLPreservesOtherBytes(t *testing.T) {
	for _, text := range []string{
		"---\r\nversion: 1\r\nteam: 'demo' # keep\r\ndescription: sample\r\n",
		"version: 1\nteam: \"demo\"\ndescription: sample\n",
		"version: 1\nteam: demo\ndescription: sample\n",
		"version: 1\ndescription: sample\n",
	} {
		got, err := RenameYAML([]byte(text), "team.yaml", "next")
		if err != nil {
			t.Fatal(err)
		}
		if want := strings.ReplaceAll(text, "demo", "next"); string(got) != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
	for _, text := range []string{"[", "", "team: &name demo\n"} {
		if _, err := RenameYAML([]byte(text), "team.yaml", "next"); err == nil {
			t.Errorf("accepted %q", text)
		}
	}
}
