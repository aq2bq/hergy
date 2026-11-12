package words

import (
	"fmt"
	"strings"
)

// Region replaces exactly one marked projection and preserves all bytes outside
// it. Missing or duplicated markers fail rather than swallowing handwritten text.
func Region(document, name, content string) (string, error) {
	start, end := "<!-- words:"+name+":start -->", "<!-- words:"+name+":end -->"
	if strings.Count(document, start) != 1 || strings.Count(document, end) != 1 {
		return "", fmt.Errorf("expected one pair of words:%s markers", name)
	}
	a, b := strings.Index(document, start)+len(start), strings.Index(document, end)
	if a > b {
		return "", fmt.Errorf("reversed words:%s markers", name)
	}
	return document[:a] + "\n" + strings.TrimSpace(content) + "\n" + document[b:], nil
}

// Handwritten removes marked generated regions before checking free prose.
// Marker validity is checked by the generator's Region call.
func Handwritten(document string) string {
	var out []string
	generated := false
	for _, line := range strings.Split(document, "\n") {
		if strings.HasPrefix(line, "<!-- words:") && strings.HasSuffix(line, ":start -->") {
			generated = true
		}
		if !generated {
			out = append(out, line)
		} else {
			out = append(out, "")
		}
		if strings.HasPrefix(line, "<!-- words:") && strings.HasSuffix(line, ":end -->") {
			generated = false
		}
	}
	return strings.Join(out, "\n")
}
