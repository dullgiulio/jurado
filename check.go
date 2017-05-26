package main

import (
	"fmt"
	"time"
)

// TODO: this is mixed. This should be called CheckResult
type TestResult struct {
	From    string // from which host
	Host    string // observed host
	Product string
	Group   string
	Error   string
	Date    time.Time
	Results []CheckResult
}

func newTestResult(from, product string, date time.Time, c *Check) *TestResult {
	return &TestResult{
		From:    from,
		Product: product,
		Host:    c.Host,
		Group:   c.Group,
		Date:    date,
	}
}

type CheckResult struct {
	Status     int // Use HTTP statuses
	Problem    string
	Suggestion string
	Time       time.Time
	Error      string
}

func NewCheckResult(status int) *CheckResult {
	return &CheckResult{
		Time:   time.Now(),
		Status: status,
	}
}

func NewCheckResultError(status int, err string) *CheckResult {
	c := NewCheckResult(status)
	c.Error = err
	return c
}

type testFn func(ch *Check) *CheckResult

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
	Name    string
	Host    string
	Group   string
	Service string
	Tests   Tests
	Options map[string]string // TODO: should be string : interface{}
	tester  tester
}

func (c *Check) String() string {
	return fmt.Sprintf("%s:%s(%s)", c.Group, c.Name, c.Host)
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

func (c *Check) run(product, host string) *TestResult {
	tr := newTestResult(host, product, time.Now(), c)
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

func (c *Check) runTests(t tester) []CheckResult {
	var i int
	res := make([]CheckResult, len(c.Tests))
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
