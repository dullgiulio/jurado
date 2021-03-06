package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

type httpTester struct {
	resp  *http.Response
	body  []byte
	tests map[string]testFn
}

// TODO: Check structure of Options.
func (h *httpTester) init(ch *Check) error {
	if _, ok := ch.Options["Url"]; !ok {
		return fmt.Errorf("need Url field in check options")
	}
	h.tests = testMap(make(map[string]testFn))
	for k := range ch.Tests {
		var fn testFn
		args := ch.Tests[k].Arguments
		switch k {
		case "http-check-status":
			val, ok := args["status"]
			if !ok {
				return fmt.Errorf("%s: need to specify a 'status' keyword", k)
			}
			st, ok := val.(float64)
			if !ok {
				return fmt.Errorf("%s: need to speficy a number as 'status' value", k)
			}
			fn = h.testHttpStatus(int(st))
		case "http-body-contains":
			val, ok := args["value"]
			if !ok {
				return fmt.Errorf("%s: need to specify a 'value' keyword", k)
			}
			str, ok := val.(string)
			if !ok {
				return fmt.Errorf("%s: need to speficy a string as 'value' value", k)
			}
			fn = h.testHttpBodyContains([]byte(str))
		default:
			return fmt.Errorf("unknown test %s", k)
		}
		h.tests[k] = fn
	}
	return nil
}

func (h *httpTester) setUp(ch *Check) error {
	var err error
	h.body, h.resp, err = httpRequest(ch.Options)
	return err
}

func (h *httpTester) tearDown() error {
	return nil
}

func (h *httpTester) get(name string) testFn {
	return h.tests[name]
}

func httpRequest(opts map[string]interface{}) ([]byte, *http.Response, error) {
	val := opts["Url"]
	rurl := val.(string)
	// If have a host, use swap it with the host in URL.
	var host string
	val, ok := opts["Host"]
	if ok {
		host = val.(string)
		u, err := url.Parse(rurl)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot change URL host: %s", err)
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
		method = m.(string)
	}
	req, err := http.NewRequest(method, rurl, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot create check request: %s", err)
	}
	if host != "" {
		req.Host = host
		req.Header.Set("Host", host)
	}
	val, ok = opts["Headers"]
	if ok {
		hrs := val.(map[string]interface{})
		for k, v := range hrs {
			req.Header.Set(k, v.(string))
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot fire check request: %s", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read check response body: %s", err)
	}
	return body, resp, nil
}

func (h *httpTester) testHttpStatus(status int) testFn {
	return func(ch *Check) *TestResult {
		if h.resp.StatusCode == status {
			return NewTestResult(200)
		}
		return NewTestResultError(500, fmt.Sprintf("HTTP status is '%s' not %d", h.resp.Status, status))
	}
}

func (h *httpTester) testHttpBodyContains(str []byte) testFn {
	return func(ch *Check) *TestResult {
		if bytes.Contains(h.body, str) {
			return NewTestResult(200)
		}
		return NewTestResultError(500, fmt.Sprintf("Body does not contain '%s'", str))
	}
}
