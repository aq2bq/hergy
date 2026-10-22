package main

import (
	"io"
	"strings"
	"testing"
)

// The gate's two shapes: an expect( that awaits a read itself, and an expect(
// that takes a name one await of read filled. Both are one reading compared
// once, so both fail (r-291a).
func TestSpecFindingsTakesTheReadingComparedOnce(t *testing.T) {
	text := "import { read, readTo } from './helpers';\n" +
		"\n" +
		"test('X-1 one walk', async ({ page }) => {\n" +
		"  expect(await read<string>(page, 'a')).toBe('a');\n" +
		"  expect(\n" +
		"    await read<number>(page, `b`),\n" +
		"    'the number reads',\n" +
		"  ).toBe(2);\n" +
		"  const rows = (id: string) => read<string>(page, `row ${id}`);\n" +
		"  expect(await rows('x'), 'the row reads').not.toBe('');\n" +
		"  const count = await read<number>(page, 'c');\n" +
		"  expect(count, 'the count reads').toBe(3);\n" +
		"});\n"
	findings := specFindings("e2e/tests/x.spec.ts", text)
	if len(findings) != 4 {
		t.Fatalf("read %d findings, want 4:\n%s", len(findings), strings.Join(findings, "\n"))
	}
	for i, want := range []string{":4:", ":5:", ":10:", ":12:"} {
		if !strings.Contains(findings[i], want) {
			t.Errorf("finding %d is %q, want the expect( at %s", i, findings[i], want)
		}
	}
	if !strings.Contains(findings[0], "readTo") {
		t.Errorf("the finding does not say what to do instead: %q", findings[0])
	}
}

// The shapes the gate asks for pass: a reading polled until it matches, an
// expect.poll of the walk's own, and every comparison that is not a reading at
// all — the files a save wrote, a value computed from one, a reading a walk
// holds to compare with what the page says later.
func TestSpecFindingsLetsThePollThrough(t *testing.T) {
	text := "import { read, readTo } from './helpers';\n" +
		"\n" +
		"test('X-1 one walk', async ({ page, app }) => {\n" +
		"  await readTo<string>(page, `a`, 'the mark reads').toBe('a');\n" +
		"  await expect.poll(() => read<string>(page, 'b'), { timeout: 8000 }).toBe('b');\n" +
		"  const before = await read<string>(page, 'c');\n" +
		"  await readTo<string>(page, 'c', 'the person did not move').toBe(before);\n" +
		"  expect(readFileSync(app.proj, 'utf8'), 'the file reads').toContain('a');\n" +
		"  expect(before.startsWith('a'), `the person stands at ${before}`).toBe(true);\n" +
		"});\n"
	if findings := specFindings("e2e/tests/x.spec.ts", text); len(findings) != 0 {
		t.Fatalf("read findings from the shapes the gate asks for:\n%s", strings.Join(findings, "\n"))
	}
}

// A shape written in prose is not the shape: a comment and a string that say
// expect(await read(...)) are words the gate does not read.
func TestSpecFindingsReadsCodeAndNotProse(t *testing.T) {
	text := "// expect(await read<string>(page, 'a')).toBe('a');\n" +
		"/* const n = await read<number>(page, 'b');\n" +
		"   expect(n).toBe(1); */\n" +
		"const said = 'expect(await read<string>(page, `a`)).toBe(`a`)';\n" +
		"const script = `expect(await read(page, 'a'))`;\n" +
		"test('X-1 one walk', async () => { expect(said).toContain(script); });\n"
	if findings := specFindings("e2e/tests/x.spec.ts", text); len(findings) != 0 {
		t.Fatalf("read findings from the prose:\n%s", strings.Join(findings, "\n"))
	}
}

// The gate fails an expect( that awaits, whichever matcher or not stands under
// it, and the line it names is the expect's own.
func TestSpecFindingsNamesTheLine(t *testing.T) {
	text := "test('X-1 one walk', async ({ page }) => {\n" +
		"\n" +
		"  expect(\n" +
		"    await read<string>(page, 'a').then((v) => v.length),\n" +
		"    'the length reads',\n" +
		"  ).toBeGreaterThan(0);\n" +
		"});\n"
	findings := specFindings("e2e/tests/x.spec.ts", text)
	if len(findings) != 1 || !strings.HasPrefix(findings[0], "e2e/tests/x.spec.ts:3:") {
		t.Fatalf("read %v, want one finding at e2e/tests/x.spec.ts:3", findings)
	}
}

// The gate over a tree: a walk that compares one reading fails it, and the same
// tree with the comparison polled passes, with no spec file at all is a failure
// too (a green run over no walk says nothing).
func TestReadingGate(t *testing.T) {
	const walked = "test('X-1 one walk', async ({ page }) => {\n" +
		"  await readTo<string>(page, 'a', 'the mark reads').toBe('a');\n" +
		"});\n"
	const once = "test('X-1 one walk', async ({ page }) => {\n" +
		"  expect(await read<string>(page, 'a'), 'the mark reads').toBe('a');\n" +
		"});\n"
	for _, tc := range []struct {
		name string
		text string
		want string
	}{
		{"the poll", walked, ""},
		{"one reading", once, "1 回だけ読んだ値を比べる expect が 1 件"},
	} {
		root := t.TempDir()
		writeTree(t, root, "e2e/tests/x.spec.ts", tc.text)
		writeTree(t, root, "e2e/tests/helpers.ts", "export const readTo = () => {};\n")
		r := &runner{root: root, stdout: io.Discard}
		err := r.readingGate()
		switch {
		case tc.want == "" && err != nil:
			t.Errorf("%s: the gate failed: %v", tc.name, err)
		case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
			t.Errorf("%s: the gate answered %v, want %q", tc.name, err, tc.want)
		}
	}

	root := t.TempDir()
	if err := (&runner{root: root, stdout: io.Discard}).readingGate(); err == nil {
		t.Error("a tree with no walk passed the gate")
	}
}

// The walks of this repository pass the gate: the rule is read over the files a
// person runs, not only over the ones a test writes.
func TestReadingGateOverTheWalks(t *testing.T) {
	root, err := moduleRoot()
	if err != nil {
		t.Skip(err)
	}
	if err := (&runner{root: root, stdout: io.Discard}).readingGate(); err != nil {
		t.Fatalf("the walks compare one reading: %v", err)
	}
}
