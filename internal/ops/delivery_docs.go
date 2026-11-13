package ops

import "github.com/aq2bq/hergy/internal/ops/internal/runningstate"

// oldDocs is exactly the rule text this member received, including empty files.
func oldDocs(receipt runningstate.Receipt) map[string]string {
	out := make(map[string]string, len(receipt.Docs))
	for _, doc := range receipt.Docs {
		out[doc.Path] = doc.Text
	}
	return out
}
