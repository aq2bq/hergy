package definition

import (
	"github.com/aq2bq/hergy/internal/yamledit"
	"go.yaml.in/yaml/v3"
)

// RenameYAML changes only an existing team: value, preserving its quoting,
// comments, line endings and all other bytes. An omitted team: stays omitted.
func RenameYAML(text []byte, path, name string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(text, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 {
		return nil, yamledit.New(text, path).Shape("team")
	}
	value := yamledit.MappingValue(doc.Content[0], "team")
	if value == nil {
		return text, nil
	}
	src := yamledit.New(text, path)
	if err := yamledit.ReplaceScalar(src, value, name, "team", false); err != nil {
		return nil, err
	}
	value.Value = name
	out := src.Bytes()
	if err := yamledit.SameValues(&doc, out, path, yamledit.Bug{Operation: "チーム定義名の変更", Workaround: "team.yaml の team: を確認してください"}); err != nil {
		return nil, err
	}
	return out, nil
}
