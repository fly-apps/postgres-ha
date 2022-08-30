package check

import (
	"fmt"
	"time"
)

type CheckFunction func() (string, error)
type Check struct {
	Name      string
	CheckFunc CheckFunction
	startTime time.Time
	endTime   time.Time
	message   string
	err       error
}

func (c *Check) Process() {
	c.startTime = time.Now()
	c.message, c.err = c.CheckFunc()
	c.endTime = time.Now()
}

func (c *Check) Error() string {
	return c.err.Error()
}

func (c *Check) Passed() bool {
	if c.startTime.IsZero() || c.endTime.IsZero() {
		return false
	}
	return c.err == nil
}

func (c *Check) ExecutionTime() time.Duration {
	if !c.endTime.IsZero() {
		return RoundDuration(c.endTime.Sub(c.startTime), 2)
	}
	return RoundDuration(time.Now().Sub(c.startTime), 2)
}

func (c *Check) Result() string {
	if c.startTime.IsZero() {
		return fmt.Sprintf("[-] %s: %s", c.Name, "Not processed")
	}
	if !c.startTime.IsZero() && c.endTime.IsZero() {
		return fmt.Sprintf("[✗] %s: %s (%s)", c.Name, "Timed out", c.ExecutionTime())
	}
	if c.Passed() {
		return fmt.Sprintf("[✓] %s: %s (%s)", c.Name, c.message, c.ExecutionTime())
	} else {
		return fmt.Sprintf("[✗] %s: %s (%s)", c.Name, c.err.Error(), c.ExecutionTime())
	}
}

func (c *Check) RawResult() string {
	if c.Passed() {
		return c.message
	}
	return c.err.Error()
}
