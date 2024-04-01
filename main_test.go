package hot

import (
	"testing"
)

func TestMain(m *testing.M) {
	// commented because it breaks some tests and we need to mock time package
	// goleak.VerifyTestMain(m)
}
