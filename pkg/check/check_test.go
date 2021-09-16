package check

import (
	"fmt"
	"testing"
)

func TestPassingCheck(t *testing.T) {
	chk := &Check{
		Name: "testCheck",
		CheckFunc: func() (string, error) {
			return "this worked", nil
		},
	}

	chk.Process()

	if !chk.Passed() {
		t.Fatalf("expected %s to pass, but the test failed with %s", chk.Name, chk.Error())
	}

	if chk.RawResult() != "this worked" {
		t.Fatalf("expected %q to be %q", chk.RawResult(), "this worked")
	}
}

func TestFailingCheck(t *testing.T) {
	chk := &Check{
		Name: "failCheck",
		CheckFunc: func() (string, error) {
			return "", fmt.Errorf("This check failed")
		},
	}
	chk.Process()
	if chk.Passed() {
		t.Fatalf("expected %s to pass, but the test failed with %s", chk.Name, chk.Error())
	}
}
