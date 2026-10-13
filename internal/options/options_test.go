package options

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		argv    []string
		kind    Kind
		dry     bool
		wantErr bool
	}{
		{name: "no arguments prints usage", kind: Help},
		{name: "unknown command", argv: []string{"frobnicate"}, wantErr: true},
		{name: "unknown flag", argv: []string{"up", "--frobnicate"}, wantErr: true},
		{name: "unknown flag with a value", argv: []string{"up", "--frobnicate=1"}, wantErr: true},
		{name: "a flag that belongs to another command", argv: []string{"down", "--profile", "p"}, wantErr: true},
		{name: "version word", argv: []string{"version"}, kind: Version},
		{name: "version flag", argv: []string{"--version"}, kind: Version},
		{name: "up with dry", argv: []string{"up", "--dry"}, kind: Up, dry: true},
		{name: "ls", argv: []string{"ls"}, kind: Ls},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.argv)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Parse(%v) = %+v, want an error", tt.argv, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%v): %v", tt.argv, err)
			}
			if got.Kind != tt.kind {
				t.Fatalf("Parse(%v).Kind = %d, want %d", tt.argv, got.Kind, tt.kind)
			}
			if got.Dry != tt.dry {
				t.Fatalf("Parse(%v).Dry = %v, want %v", tt.argv, got.Dry, tt.dry)
			}
		})
	}
}
