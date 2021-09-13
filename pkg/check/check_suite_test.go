package check

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestPassingCheckSuite(t *testing.T) {
	suite := NewCheckSuite("passingChecks")
	suite.AddCheck("test_one", func() (string, error) {
		return "pass", nil
	})
	suite.AddCheck("test_two", func() (string, error) {
		return "pass", nil
	})
	suite.AddCheck("test_three", func() (string, error) {
		return "pass", nil
	})

	if len(suite.Checks) != 3 {
		t.Fatalf("expected suite to contain %d checks, but had %d instead.", 3, len(suite.Checks))
	}

	suite.Process(context.TODO())

	if !suite.Passed() {
		t.Fatalf("expected %s to pass, but didn't", suite.Name)
	}
}

func TestFailingCheckSuite(t *testing.T) {
	suite := NewCheckSuite("passingChecks")

	suite.AddCheck("test_one", func() (string, error) {
		return "pass", nil
	})
	suite.AddCheck("test_two", func() (string, error) {
		return "pass", nil
	})
	suite.AddCheck("test_three", func() (string, error) {
		return "", fmt.Errorf("I failed")
	})

	if len(suite.Checks) != 3 {
		t.Fatalf("expected suite to contain %d checks, but had %d instead.", 3, len(suite.Checks))
	}

	suite.Process(context.TODO())

	if suite.Passed() {
		t.Fatalf("expected %s to fail, but it didn't", suite.Name)
	}
}

func TestBuildingChecksFromLoop(t *testing.T) {
	suite := NewCheckSuite("passingChecks")
	units := []string{"memory", "io", "disk"}
	for _, u := range units {
		// The closure function isn't evaluated until suite.Processed() is called.
		// Iterators reuse the same pointer, so if you're passing a value into the anon function
		// re-assign it to a new variable before hand.
		unit := u
		suite.AddCheck(unit, func() (string, error) {
			return unit, nil
		})
	}

	suite.Process(context.TODO())

	resultArr := strings.Split(suite.RawResult(), "\n")
	if len(resultArr) != len(units) {
		t.Fatalf("expected resultArr to eq %d, got %d", len(units), len(resultArr))
	}

	r := strings.Join(sort.StringSlice(resultArr), "")
	u := strings.Join(sort.StringSlice(units), "")
	if r != u {
		t.Fatalf("Expected slices to match. expected %q, received %q", u, r)
	}

	if !suite.Passed() {
		t.Fatalf("Expected check suite to pass")
	}

}

func TestFailureDueToTimeout(t *testing.T) {
	timeout := 200 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	suite := NewCheckSuite("timeoutChecks")

	suite.AddCheck("timeoutCheck-one", func() (string, error) {
		return "passing", nil
	})
	suite.AddCheck("timeoutCheck-two", func() (string, error) {
		time.Sleep(300 * time.Millisecond)
		return "passing", nil
	})

	go func() {
		suite.Process(ctx)
		cancel()
	}()

	select {
	case <-ctx.Done():
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Fatalf("expected context to fail with context deadline exceeded. received: %v", ctx.Err())
		}
		if suite.Passed() {
			t.Fatalf("expected suite to fail, but it passed instead.")
		}
	}
}

func TestPartialSuccessWithTimeout(t *testing.T) {
	timeout := 200 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	suite := NewCheckSuite("successWithTimeoutChecks")
	suite.AddCheck("successWithTimeoutCheck-one", func() (string, error) {
		return "passing", nil
	})
	// Times out here.
	suite.AddCheck("successWithTimeoutCheck-two", func() (string, error) {
		time.Sleep(250 * time.Millisecond)
		return "passing", nil
	})
	// This check should not be processed.
	suite.AddCheck("successWithTimeoutCheck-three", func() (string, error) {
		time.Sleep(100 * time.Millisecond)
		return "passing", nil
	})

	go func() {
		suite.Process(ctx)
		cancel()
	}()
	select {
	case <-ctx.Done():

		if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Fatalf("expected context to deadline, but instead received: %v", ctx.Err())
		}
		if suite.Passed() {
			t.Fatalf("check suite should not have passed...")
		}
		if suite.Checks[0].message != "passing" {
			t.Fatalf("first check should have completed.")
		}
		if suite.Checks[1].startTime.IsZero() || !suite.Checks[1].endTime.IsZero() {
			t.Fatalf("%s should have timed out.", suite.Checks[1].Name)
		}
		if !suite.Checks[2].startTime.IsZero() && !suite.Checks[2].endTime.IsZero() {
			t.Fatalf("%s should not have been processed", suite.Checks[2].Name)
		}
	}
}

func timeoutBeforeChecksHelper(ctx context.Context, suite *CheckSuite) *CheckSuite {
	time.Sleep(200 * time.Millisecond)

	suite.AddCheck("setupTimeout-one", func() (string, error) {
		return "passing", nil
	})

	suite.AddCheck("setupTimeout-two", func() (string, error) {
		return "passing", nil
	})

	return suite
}

func TestSetupTimeout(t *testing.T) {
	timeout := 100 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	suite := NewCheckSuite("setupTimeout")

	go func(ctx context.Context) {
		timeoutBeforeChecksHelper(ctx, suite)
		suite.Process(ctx)
		cancel()
	}(ctx)

	select {
	case <-ctx.Done():

		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			if len(suite.Checks) > 0 {
				t.Fatalf("No checks should have been added")
			}
		}
	}
}

func TestOnCompletionHook(t *testing.T) {
	target := "onCompletion"
	myVar := "original"

	suite := NewCheckSuite("onCompletionTest")

	suite.OnCompletion = func() {
		myVar = target
	}

	suite.AddCheck("onCompletionTest-one", func() (string, error) {
		myVar = "one"
		return "passing", nil
	})

	suite.AddCheck("onCompletionTest-two", func() (string, error) {
		myVar = "two"
		return "passing", nil
	})

	suite.Process(context.TODO())

	if myVar != target {
		t.Fatalf("expected value to eq %s, instead got %s", target, myVar)
	}
}
