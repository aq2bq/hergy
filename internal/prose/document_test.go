package prose

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/aq2bq/hergy/internal/source"
)

func TestTextRoundTripAndConflict(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_STATE_HOME", dir)
	path := filepath.Join(dir, "role.md")
	text := "# 題\r\n\r\n  本文 → %{role_doc}\r\n\t末尾  "
	d := Document{Text: text}
	if err := d.Save(path, source.Hash("")); err != nil {
		t.Fatal(err)
	}
	got, err := Read(path)
	if err != nil || got.Text != text {
		t.Fatalf("read: %#v, %v", got, err)
	}
	if err := os.WriteFile(path, []byte("別の編集"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := d.Save(path, source.Hash(text)); !errors.Is(err, source.ErrChanged) {
		t.Fatalf("conflict: %v", err)
	}
	if _, err := Read(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("missing file accepted")
	}
}

func TestTitleAndCountLeaveTextIntact(t *testing.T) {
	for _, tc := range []struct {
		text, title string
		count       int
	}{
		{"# 題\r\n本文", "題", 6},
		{"\n# 次の行", "fallback", 6},
		{"##\t題", "題", 4},
		{"#hashtag", "#hashtag", 8},
		{"", "fallback", 0},
		{"😀 e\u0301", "😀 e\u0301", 4},
	} {
		d := Document{Text: tc.text}
		if d.Title("fallback") != tc.title || d.Chars() != tc.count {
			t.Errorf("%q: title=%q count=%d", tc.text, d.Title("fallback"), d.Chars())
		}
		if d.Text != tc.text {
			t.Fatal("text changed")
		}
	}
}
