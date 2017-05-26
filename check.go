package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
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

type testFn func(ch *Check, args TestArgs) (*CheckResult, error)

type testMap map[string]testFn

type tester interface {
	test(*Check) ([]CheckResult, error)
}

func newTester(srv string, ch *Check) (tester, error) {
	switch srv {
	case "http":
		return &httpTester{}, nil
	default:
		return nil, fmt.Errorf("unknown service %s", srv)
	}
}

type httpTester struct {
	resp *http.Response
	body []byte
}

func (h *httpTester) makeTestMap() map[string]testFn {
	tm := testMap(make(map[string]testFn))
	tm["http-check-status"] = h.testHttpCheckStatus
	tm["http-body-contains"] = h.testHttpBodyContains
	return tm
}

func (h *httpTester) test(ch *Check) ([]CheckResult, error) {
	if err := h.makeRequest(ch.Options); err != nil {
		return nil, fmt.Errorf("cannot make HTTP request: %s", err)
	}
	res := make([]CheckResult, len(ch.Tests))
	tm := h.makeTestMap() // TODO: Init should be done once only
	i := 0
	for tname := range ch.Tests {
		fn, ok := tm[tname]
		if !ok {
			log.Printf("Error: unknown test %s", tname)
			continue
		}
		ti := ch.Tests[tname]
		r, err := fn(ch, ti.Arguments)
		// Handle non-recoverable errors; for example, invalid test setup
		if err != nil {
			log.Printf("Error: in test %s: %s\n", tname, err)
			continue
		}
		if r.Status != 200 {
			r.Problem = ti.Problem
			r.Suggestion = ti.Suggestion
		}
		res[i] = *r
		i++
	}
	return res, nil
}

// TODO: Use different error interfaces to differentiate conf errors from real problems
func (h *httpTester) makeRequest(opts map[string]string) error {
	rurl, ok := opts["Url"]
	if !ok {
		return fmt.Errorf("need Url field in check options")
	}
	// If have a host, use swap it with the host in URL.
	host, ok := opts["Host"]
	if ok {
		u, err := url.Parse(rurl)
		if err != nil {
			return fmt.Errorf("cannot change URL host: %s", err)
		}
		hs := u.Host
		u.Host = host
		host = hs
		rurl = u.String()
	}
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	method := "GET"
	if m, ok := opts["Method"]; ok {
		method = m
	}
	req, err := http.NewRequest(method, rurl, nil)
	if err != nil {
		return fmt.Errorf("cannot create check request: %s", err)
	}
	if host != "" {
		req.Host = host
		req.Header.Set("Host", host)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("cannot fire check request: %s", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("cannot read check response body: %s", err)
	}
	h.body = body
	h.resp = resp
	return nil
}

func (h *httpTester) testHttpCheckStatus(ch *Check, args TestArgs) (*CheckResult, error) {
	val, ok := args["status"]
	if !ok {
		return nil, errors.New("need to specify a 'status' keyword")
	}
	st, ok := val.(float64)
	if !ok {
		return nil, errors.New("need to speficy a number as 'status' value")
	}
	if h.resp.StatusCode == int(st) {
		return NewCheckResult(200), nil
	}
	return NewCheckResultError(500, fmt.Sprintf("HTTP status is '%s' not %d", h.resp.Status, int(st))), nil
}

func (h *httpTester) testHttpBodyContains(ch *Check, args TestArgs) (*CheckResult, error) {
	val, ok := args["value"]
	if !ok {
		return nil, errors.New("need to specify a 'value' keyword")
	}
	str, ok := val.(string)
	if !ok {
		return nil, errors.New("need to speficy a string as 'value' value")
	}
	if bytes.Contains(h.body, []byte(str)) {
		return NewCheckResult(200), nil
	}
	return NewCheckResultError(500, fmt.Sprintf("Body does not contain '%s'", str)), nil
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
