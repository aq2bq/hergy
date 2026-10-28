package app

import (
	"testing"

	"github.com/aq2bq/hergy/internal/apicontract"
	"github.com/aq2bq/hergy/internal/web"
	"go.uber.org/goleak"
)

// TestMain holds every API call the tests make through hergy serve to
// openapi.yaml (n-e3de), and fails when a test leaves a goroutine behind.
// hergy serve starts a team in the background (the headquarters' start), and a
// test that goes away must not leave that goroutine running (d-b28a).
func TestMain(m *testing.M) {
	checker, err := apicontract.NewChecker()
	if err != nil {
		panic(err)
	}
	web.WrapForTest = checker.Wrap
	goleak.VerifyTestMain(m, goleak.Cleanup(checker.Report))
}
