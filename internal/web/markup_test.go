package web

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This file holds the WebUI to one way of writing the page: Preact's render.
// A screen that assembles a string of markup and puts it into an element
// replaces the element, so the input a person was in is a different input
// afterwards, and an answer that comes later is written into an element that is
// no longer on the screen (webui-preact-design 1 章). The rewrite exists so that
// this cannot happen, and this test is what keeps it from coming back: the ban
// is a test, not a convention.

// markupWrites are the ways a script writes the DOM itself. Any of them in a
// file that is not one of the exceptions below fails.
var markupWrites = []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "dangerouslySetInnerHTML"}

// exception is one file the ban lets through, and why. It is a debt with a
// name: a screen that is moved takes its line out of this table, and the test
// fails if a line names a file that is no longer there.
type exception struct {
	// only, when set, is the single write this file may still make; the other
	// markup writes are banned in it either way.
	only string
	why  string
}

// markupExceptions are the files that still write markup, and the reason each
// one does. Everything else under static/ (vendor/ aside) is held to the ban,
// so a file made from now on is covered without being added here — the list
// only ever shrinks.
var markupExceptions = map[string]exception{
	"teams-v2.js": {
		why: "チーム定義の画面。パネルを文字列で組み立てて領域ごと差し替えている（段階 1 以降で移す）",
	},
	"roles-v2.js": {
		why: "ロールの画面。行とパネルを文字列で組み立てている（段階 1 以降で移す）",
	},
	"hq.js": {
		why: "本部の読みと、各画面が借りているシート。図（teamSVG）も文字列で返す（hq の段で移す）",
	},
	"ruledoc.js": {
		why: "ルール文書の面。roles と teams の両方が使う（共通部品の段で移す）",
	},
	"deliver.js": {
		why: "伝えていない変更のシート。各画面が借りている（共通部品の段で移す）",
	},
	"topocanvas.js": {
		why: "チーム定義の図を描く canvas（最後の段で移す）",
	},
	// The one place a string of markup may be put into a page after the
	// rewrite: the figure the activity screen borrows from hq.js.
	//
	// 理由: teamSVG は SVG を文字列で返し、活動の画面とチーム定義の画面が同じ
	// 1 つの図を描いている。図を Preact の部品に書き直すのは hq.js を module に
	// する段なので、それまでは「操作しない図」に限って流し込む。図の中身は
	// svg(role="img") / g / circle / text / path / line だけで、押せるものも
	// フォーカスの入るものも無い（activity/figure.js にも同じことを書いてある）。
	//
	// hq.js を module にして teamSVG が Preact の部品になったら、この行と
	// activity/figure.js の dangerouslySetInnerHTML を一緒に消す。
	"activity/figure.js": {
		only: "dangerouslySetInnerHTML",
		why:  "hq.js の teamSVG が返す「操作しない図」を流し込む 1 か所。hq.js を module にする段で消す",
	},
}

// TestTheDomIsWrittenOnlyByRender walks the WebUI's scripts and fails on a
// markup write outside the exceptions. vendor/ is left alone: it is what the
// three packages make (tools/webvendor), not a screen.
func TestTheDomIsWrittenOnlyByRender(t *testing.T) {
	root := "static"
	seen := make(map[string]bool)
	err := fs.WalkDir(os.DirFS(root), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		path = filepath.ToSlash(path)
		if d.IsDir() {
			if path == "vendor" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".js") {
			return nil
		}
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return err
		}
		seen[path] = true
		body := string(raw)

		ex, excepted := markupExceptions[path]
		if excepted && ex.only == "" {
			// A screen not moved yet: its writes are the rewrite's work, not
			// this test's.
			return nil
		}
		for _, w := range markupWrites {
			if countCode(body, w) == 0 {
				continue
			}
			if excepted && w == ex.only {
				continue
			}
			t.Errorf("%s writes the DOM itself (%s): draw it with Preact's render instead, or name it in markupExceptions with the reason (webui-preact-design 5 章 1)", path, w)
		}
		if excepted && ex.only != "" {
			if n := countCode(body, ex.only); n != 1 {
				t.Errorf("%s: the exception is for one %s and the code has %d: the figure is one place, not a habit", path, ex.only, n)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}

	// A line that names a file nobody has any more is a debt that was paid:
	// take it out, so the table is the list of what is left.
	for path, ex := range markupExceptions {
		if !seen[path] {
			t.Errorf("markupExceptions names %s (%s), which is not there: take the line out", path, ex.why)
		}
	}
}

// countCode counts one word in a file's code, leaving its comment lines out: a
// comment that explains this rule must not fail it. A block comment's body
// lines start with *, and an inline comment stays counted, so a write hidden
// behind one is still a finding.
func countCode(body, word string) int {
	n := 0
	for line := range strings.SplitSeq(body, "\n") {
		if t := strings.TrimSpace(line); strings.HasPrefix(t, "//") || strings.HasPrefix(t, "/*") || strings.HasPrefix(t, "*") {
			continue
		}
		n += strings.Count(line, word)
	}
	return n
}
