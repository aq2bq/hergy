package ops

import (
	"context"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/options"
)

func TestTellRefusalsNameMembersAndWorkers(t *testing.T) {
	for _, lang := range []string{"en", "ja"} {
		t.Run(lang, func(t *testing.T) {
			previous := i18n.Language()
			t.Cleanup(func() { _ = i18n.Use(previous) })
			f := newFixture(t)
			mustRun(t, "new", "demo")
			_, prompts := f.controlHerdr(t, "demo-lead idle p1 screen\ndemo-builder idle p2 screen\n")
			if err := i18n.Use(lang); err != nil {
				t.Fatal(err)
			}
			for _, tc := range []struct {
				member, from, body, key string
				args                    []any
			}{
				{member: "stranger", body: "hi", key: "error.tell_target_unknown", args: []any{"stranger"}},
				{member: "builder", from: "stranger", body: "hi", key: "error.tell_from_unknown", args: []any{"stranger"}},
				{member: "human", key: "error.tell_human_from_person"},
			} {
				opts, err := options.Parse([]string{"status", "demo"})
				if err != nil {
					t.Fatal(err)
				}
				_, err = Tell(context.Background(), TellInput{MemberInput: MemberInput{Env: serveEnv(t), Team: opts.Team, Member: tc.member}, From: tc.from, Body: tc.body})
				if err == nil || err.Error() != i18n.Textf(lang, tc.key, tc.args...) {
					t.Fatalf("%s: %v", tc.key, err)
				}
			}
			if sent := promptsRead(t, prompts); sent != nil {
				t.Fatalf("a refusal sent messages: %q", sent)
			}
		})
	}
}
