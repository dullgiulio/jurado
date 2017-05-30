package main

import (
	"fmt"
	"time"
)

type TestResult struct {
	Status     int // Use HTTP statuses
	Problem    string
	Suggestion string
	Error      string
}

func NewTestResult(status int) *TestResult {
	return &TestResult{
		Status: status,
	}
}

type CheckResult struct {
	From    string // from which host
	Host    string // observed host
	Product string
	Group   string
	Error   string
	Date    time.Time
	Results []TestResult
}

func newCheckResult(from string, date time.Time, c *Check) *CheckResult {
	return &CheckResult{
		From:    from,
		Product: c.Product,
		Host:    c.Host,
		Group:   c.Group,
		Date:    date,
	}
}

func NewTestResultError(status int, err string) *TestResult {
	tr := NewTestResult(status)
	tr.Error = err
	return tr
}

type testFn func(ch *Check) *TestResult

type testMap map[string]testFn

type tester interface {
	// Static initialization
	init(*Check) error
	// Initialization for each test run
	setUp(*Check) error
	tearDown() error
	get(string) testFn
}

func newTester(srv string, ch *Check) (tester, error) {
	switch srv {
	case "http":
		return &httpTester{}, nil
	case "json":
		return &jsonTester{}, nil
	default:
		return nil, fmt.Errorf("unknown service %s", srv)
	}
}

type TestArgs map[string]interface{}

type TestInfo struct {
	Problem    string
	Suggestion string
	Arguments  TestArgs
}

type Tests map[string]TestInfo

type Check struct {
	Product  string
	Name     string
	Host     string
	Group    string
	Service  string
	Interval string
	Tests    Tests
	Options  map[string]interface{}
	tester   tester
}

func (c *Check) String() string {
	return fmt.Sprintf("%s:%s:%s(%s)", c.Product, c.Group, c.Name, c.Host)
}

// TODO: should return an array of errors
func (c *Check) init() error {
	var err error
	c.tester, err = newTester(c.Service, c)
	if err != nil {
		return fmt.Errorf("cannot create checking service: %s", err)
	}
	if err = c.tester.init(c); err != nil {
		return fmt.Errorf("cannot init check %s: %s", c, err)
	}
	return nil
}

func (c *Check) run(host string) *CheckResult {
	tr := newCheckResult(host, time.Now(), c)
	if err := c.tester.setUp(c); err != nil {
		tr.Error = err.Error()
		return tr
	}
	tr.Results = c.runTests(c.tester)
	if err := c.tester.tearDown(); err != nil {
		tr.Error = err.Error()
	}
	return tr
}

func (c *Check) runTests(t tester) []TestResult {
	var i int
	res := make([]TestResult, len(c.Tests))
	for tname := range c.Tests {
		fn := t.get(tname)
		r := fn(c)
		if r.Status != 200 {
			ti := c.Tests[tname]
			r.Problem = ti.Problem
			r.Suggestion = ti.Suggestion
		}
		res[i] = *r
		i++
	}
	return res
}
