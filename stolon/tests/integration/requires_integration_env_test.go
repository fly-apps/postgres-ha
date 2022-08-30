package integration

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if os.Getenv("INTEGRATION") == "" {
		return
	}

	os.Exit(m.Run())
}
