package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

type TestResult struct {
	From    string // from which host
	Host    string // observed host
	Product string
	Group   string
	Error   string
	Date    time.Time
	Results []CheckResult
}

func newTestResult(from, product string, date time.Time, res []CheckResult, c *Check) *TestResult {
	return &TestResult{
		From:    from,
		Product: product,
		Host:    c.Host,
		Group:   c.Group,
		Date:    date,
		Results: res,
	}
}

type CheckResult struct {
	Status int // Use HTTP statuses
	Time   time.Time
	Error  string
}

func NewCheckResult(status int) *CheckResult {
	return &CheckResult{
		Time:   time.Now(),
		Status: status,
	}
}

type testFn func(ch *Check, args TestArgs) (*CheckResult, error)

type testMap map[string]testFn

type tester interface {
	test(*Check) ([]CheckResult, error)
}

func newTester(srv string, ch *Check) (tester, error) {
	switch srv {
	case "http":
		return &httpTester{host: ch.Host}, nil
	default:
		return nil, fmt.Errorf("unknown service %s", srv)
	}
}

type httpTester struct {
	host   string
	client *http.Client
}

func (h *httpTester) test(ch *Check) ([]CheckResult, error) {
	tm := testMap(make(map[string]testFn))
	tm["http-check-status"] = h.testHttpCheckStatus
	tm["http-body-contains"] = h.testHttpBodyContains

	res := make([]CheckResult, len(ch.Tests))
	i := 0
	for tname := range ch.Tests {
		fn, ok := tm[tname]
		if !ok {
			return nil, fmt.Errorf("unknown test %s", tname)
		}
		r, err := fn(ch, ch.Tests[tname])
		if err != nil {
			// TODO: turn errors into results
			log.Printf("Error: in test %s: %s\n", tname, err)
		}
		res[i] = *r
		i++
	}
	return res, nil
}

func (h *httpTester) testHttpCheckStatus(ch *Check, args TestArgs) (*CheckResult, error) {
	log.Print("running test testHttpCheckStatus")
	return NewCheckResult(200), nil
}

func (h *httpTester) testHttpBodyContains(ch *Check, args TestArgs) (*CheckResult, error) {
	log.Print("running test testHttpBodyContains")
	return NewCheckResult(200), nil
}

type TestArgs map[string]string

// Test name : map of arguments
type TestInfo map[string]TestArgs

type Check struct {
	Name    string
	Host    string
	Group   string
	Service string
	Tests   TestInfo
	// TODO: Checked from etc
}

func (c *Check) run(product, host string) (*TestResult, error) {
	tester, err := newTester(c.Service, c)
	if err != nil {
		return nil, err
	}
	res, err := tester.test(c)
	if err != nil {
		return nil, err
	}
	return newTestResult(host, product, time.Now(), res, c), nil
}
