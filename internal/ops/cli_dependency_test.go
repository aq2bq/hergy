package ops_test

// The internal tests build and execute the CLI as a subprocess. Keep that
// subprocess's source dependencies in Go's test cache key as well: changing a
// CLI adapter or one of its embedded inputs must invalidate these comparisons.
// This is an external test package; production ops never imports cli.
import _ "github.com/aq2bq/hergy/internal/cli"
