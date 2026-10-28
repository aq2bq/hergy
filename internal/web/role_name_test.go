package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// TestRoleNameThatIsNotAnIdentifierIsANameProblem: a role's name is an
// identifier (d-841e, r-7ea6). The check answers a problem on the name field —
// at: name — for the saved text and for a panel's draft, so the screen can stand
// its sentence under 名前 before a save, and a save still refuses it at the same
// field.
func TestRoleNameThatIsNotAnIdentifierIsANameProblem(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	text := "version: 1\nsections:\n  - heading: 進め方\n    rules:\n      - id: PM-1\n        text: 今日決めることを 1 つ書く。\n"
	f.write(t, "roles/進行役.yaml", text)

	type problem struct {
		Sev   string `json:"sev"`
		At    string `json:"at"`
		Field string `json:"field"`
		Text  string `json:"text"`
	}
	readProblems := func(body string) []problem {
		t.Helper()
		var answer struct {
			Problems []problem `json:"problems"`
		}
		if err := json.Unmarshal([]byte(body), &answer); err != nil {
			t.Fatalf("the answer does not read: %v\n%s", err, body)
		}
		return answer.Problems
	}
	wantName := func(t *testing.T, problems []problem) {
		t.Helper()
		for _, one := range problems {
			if one.At != "name" {
				continue
			}
			if one.Field != "name" || one.Sev != "err" {
				t.Errorf("the name problem reads %+v, want an error on the name field", one)
			}
			if !strings.Contains(one.Text, "進行役") || !strings.Contains(one.Text, "タイトル") {
				t.Errorf("the name problem does not name the name and the title: %q", one.Text)
			}
			return
		}
		t.Fatalf("no problem stands on the name field: %+v", problems)
	}

	// The YAML tab's text.
	resp, body := f.api(t, server, http.MethodPost, "/api/role/check", map[string]any{
		"place": "project", "name": "進行役", "text": text,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the text check answers %d: %s", resp.StatusCode, body)
	}
	wantName(t, readProblems(body))

	// The panel's draft a copy makes: the name comes from the document.
	resp, body = f.api(t, server, http.MethodPost, "/api/role/check", map[string]any{
		"place": "global",
		"doc": map[string]any{
			"name": "進行役", "title": "", "revision": 1,
			"sections": []any{map[string]any{
				"heading": "進め方", "level": 2, "intro": "", "rules": []any{
					map[string]any{"id": "PM-1", "text": "今日決めることを 1 つ書く。", "in_prompt": true},
				},
			}},
		},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the draft check answers %d: %s", resp.StatusCode, body)
	}
	wantName(t, readProblems(body))

	// The save refuses it on the same field.
	resp, body = f.api(t, server, http.MethodPut, "/api/role/"+url.PathEscape("進行役"), map[string]any{
		"place": "global",
		"doc": map[string]any{
			"name": "進行役", "title": "", "revision": 1,
			"sections": []any{map[string]any{
				"heading": "進め方", "level": 2, "intro": "", "rules": []any{
					map[string]any{"id": "PM-1", "text": "今日決めることを 1 つ書く。", "in_prompt": true},
				},
			}},
		},
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("the save of a role named 進行役 answers %d: %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, `"at":"name"`) {
		t.Errorf("the refusal does not stand on the name field: %s", body)
	}
}
