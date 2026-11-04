package web

import (
	"net/http"
	"strings"
	"testing"
)

func TestRoleProseCheckReturnsSizeReadersAndNoTurns(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	text := strings.Repeat("あ", 1201)
	resp, decoded := f.apiJSON(t, server, http.MethodPost, "/api/role/check", map[string]any{"place": "project", "original": "lead", "doc": map[string]any{"name": "lead", "title": "", "text": text}})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%d: %v", resp.StatusCode, decoded)
	}
	problems, _ := decoded["problems"].([]any)
	if len(problems) != 1 {
		t.Fatalf("problems: %v", problems)
	}
	if decoded["size"] != float64(1201) || decoded["limit"] != float64(1200) {
		t.Fatalf("budget: %v", decoded)
	}
	effect, _ := decoded["effect"].(map[string]any)
	if _, ok := effect["diff"]; !ok {
		t.Fatal("missing diff")
	}
	if _, ok := effect["turns"]; ok {
		t.Fatal("prose turns were returned")
	}
	readers, _ := decoded["readers"].([]any)
	if len(readers) != 1 {
		t.Fatalf("readers: %v", readers)
	}
}
