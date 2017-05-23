package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

type CheckResult struct {
	Check  *Check
	Status int // Use HTTP statuses
	Time   time.Time
	Error  string
}

func NewCheckResult(ch *Check, status int) *CheckResult {
	return &CheckResult{
		Check:  ch,
		Time:   time.Now(),
		Status: status,
	}
}

type testFn func(ch *Check, args TestArgs) (*CheckResult, error)

type testMap map[string]testFn

type tester interface {
	test(*Check) ([]*CheckResult, error)
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
	host string
	client *http.Client
}

// TODO implement tester interface
func (h *httpTester) test(ch *Check) ([]*CheckResult, error) {
	tm := testMap(make(map[string]testFn))
	tm["http-check-status"] = h.testHttpCheckStatus
	tm["http-body-contains"] = h.testHttpBodyContains

	res := make([]*CheckResult, len(ch.Tests))
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
		res[i] = r
		i++
	}
	return res, nil
}

func (h *httpTester) testHttpCheckStatus(ch *Check, args TestArgs) (*CheckResult, error) {
	log.Print("running test testHttpCheckStatus")
	return NewCheckResult(ch, 200), nil
}

func (h *httpTester) testHttpBodyContains(ch *Check, args TestArgs) (*CheckResult, error) {
	log.Print("running test testHttpBodyContains")
	return NewCheckResult(ch, 200), nil
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

func (c *Check) run() ([]*CheckResult, error) {
	tester, err := newTester(c.Service, c)
	if err != nil {
		return nil, err
	}
	return tester.test(c)
}

func postCheckResult(rw http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	log.Println(req.Form)
	cr := &CheckResult{}
	for key, _ := range req.Form {
		err := json.Unmarshal([]byte(key), &cr)
		if err != nil {
			log.Println(err.Error())
		}
	}
	fmt.Printf("%+v\n", cr)
}

func readHttpFile(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("cannot GET from HTTP: %s", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read from HTTP: %s", err)
	}
	return body, nil
}

func readLocalFile(url string) ([]byte, error) {
	fh, err := os.Open(url)
	if err != nil {
		return nil, fmt.Errorf("cannot open file %s: %s", url, err)
	}
	defer fh.Close()
	body, err := ioutil.ReadAll(fh)
	if err != nil {
		return nil, fmt.Errorf("cannot read file %s: %s", url, err)
	}
	return body, nil
}

func readFile(url string) ([]byte, error) {
	if url[0:7] == "file://" {
		url = url[7:]
		return readLocalFile(url)
	}
	return readHttpFile(url)
}

func loadChecks(url string) ([]Check, error) {
	body, err := readFile(url)
	if err != nil {
		return nil, err
	}
	var checks []Check
	err = json.Unmarshal(body, &checks)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal checks: %s", err)
	}
	return checks, nil
}

func main() {
	flag.Parse()
	checks, err := loadChecks(flag.Arg(0))
	if err != nil {
		log.Fatalf("cannot load checks profile: %s", err)
	}

	for i := range checks {
		res, err := checks[i].run()
		if err != nil {
			log.Printf("Error: can't run check %s: %s", checks[i].Name, err)
			continue
		}
		for c := range res {
			fmt.Printf("%+v\n", res[c])
		}
	}


	//http.HandleFunc("/check/result/add", postCheckResult)
	//log.Fatal(http.ListenAndServe(":8082", nil))
}
