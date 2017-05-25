package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
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

type hostname string

type config map[hostname]struct {
	Options  ConfOptions
	Products ConfProducts
}

type ConfProducts map[string][]Check

func (p ConfProducts) runCheckers(host string, in chan<- *TestResult) {
	for prodName, prods := range p {
		for i := range prods {
			check := &prods[i]
			tr, err := check.run(prodName, host)
			if err != nil {
				tr.Error = err.Error()
			}
			in <- tr
		}
	}
}

type ConfOptions struct {
	File    string
	Remotes []string
}

func (o *ConfOptions) addRemotesPath(path string) {
	for i := range o.Remotes {
		o.Remotes[i] = o.Remotes[i] + path
	}
}

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

func loadConf(url string) (*config, error) {
	body, err := readFile(url)
	if err != nil {
		return nil, err
	}
	var cf config
	err = json.Unmarshal(body, &cf)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal checks: %s", err)
	}
	return &cf, nil
}

func (c config) forHostname(h hostname) (ConfProducts, *ConfOptions, error) {
	cf, ok := c[h]
	if !ok {
		return nil, nil, fmt.Errorf("no configuration for host %s", h)
	}
	return cf.Products, &cf.Options, nil
}

type status map[string]ProdStatus

func makeStatus() status {
	return make(map[string]ProdStatus)
}

// Append and keep latest N for each test (TODO)
func (s status) appendTestResult(host, product, group string, tr *TestResult) {
	if _, ok := s[host]; !ok {
		s[host] = makeProdStatus()
	}
	if _, ok := s[host][product]; !ok {
		s[host][product] = makeGroupStatus()
	}
	if _, ok := s[host][product][group]; !ok {
		s[host][product][group] = make([]*TestResult, 1)
	}
	s[host][product][group][0] = tr
}

type ProdStatus map[string]GroupStatus

func makeProdStatus() ProdStatus {
	return make(map[string]GroupStatus)
}

type GroupStatus map[string][]*TestResult

func makeGroupStatus() GroupStatus {
	return make(map[string][]*TestResult)
}

type persister struct {
	conf     *ConfOptions
	incoming chan *TestResult
	results  chan *TestResult
	status   status
}

func newPersister(cf *ConfOptions) *persister {
	p := &persister{
		conf:     cf,
		status:   makeStatus(),
		incoming: make(chan *TestResult),
		results:  make(chan *TestResult),
	}
	go p.processIncoming(p.incoming)
	go p.processResults(p.results)
	return p
}

// Events coming from other agents
func (p *persister) processIncoming(in <-chan *TestResult) {
	for tr := range in {
		p.status.appendTestResult(tr.Host, tr.Product, tr.Group, tr)
		data, err := json.Marshal(p.status)
		if err != nil {
			log.Fatalf("cannot marshal tests status: %s", err)
		}
		if err := writeFileAtomic(p.conf.File, data, os.FileMode(0644)); err != nil {
			log.Fatalf("cannot write results JSON file: %s", err)
		}
	}
}

// Events coming from our tests
func (p *persister) processResults(in <-chan *TestResult) {
	for r := range in {
		errs, err := p.putRemotes(p.conf.Remotes, r)
		if err != nil {
			log.Fatalf("cannot put result to remote: %s", err)
		}
		// TODO: errs should have the url etc
		for e := range errs {
			log.Printf("put failed: %s", errs[e])
		}
		p.incoming <- r
	}
}

func (p *persister) putRemotes(remotes []string, result *TestResult) ([]error, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal result: %s", err)
	}
	var errs []error
	for i := range remotes {
		r := bytes.NewBuffer(data) // TODO: optimize
		err = p.putRemote(remotes[i], r)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errs, nil
}

func (p *persister) putRemote(url string, r io.Reader) error {
	req, err := http.NewRequest("PUT", url, r)
	if err != nil {
		return fmt.Errorf("cannot PUT result: %s", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("cannot PUT result: %s", err)
	}
	defer resp.Body.Close()
	// TODO: don't discard body if status != 200
	io.Copy(ioutil.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("PUT of result to %s: server returned status %d", url, resp.StatusCode)
	}
	return nil
}

func (p *persister) receiveTestResult(req *http.Request) error {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return fmt.Errorf("cannot read request body: %s", err)
	}
	tr := &TestResult{}
	if err := json.Unmarshal(body, &tr); err != nil {
		return fmt.Errorf("cannot unmarshal into result object: %s", err)
	}
	p.incoming <- tr
	return err
}

type api struct {
	persister *persister
}

func (a *api) handlePutResult(rw http.ResponseWriter, req *http.Request) {
	if req.Method != "PUT" {
		rw.Header().Set("Allow", "PUT")
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := a.persister.receiveTestResult(req); err != nil {
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	io.WriteString(rw, "Success\n") // TODO: check error
}

func main() {
	flag.Parse()
	arg := flag.Arg(0)
	if arg == "" {
		log.Fatal("specify a URL to load the configuration from")
	}
	conf, err := loadConf(flag.Arg(0))
	if err != nil {
		log.Fatalf("cannot load checks profile: %s", err)
	}
	hname, err := os.Hostname()
	if err != nil {
		log.Fatalf("cannot determine local hostname: %s", err)
	}
	prods, opts, err := conf.forHostname(hostname(hname))
	if err != nil {
		log.Printf("cannot use configuration: %s", err)
	}

	apiPutResultPath := "/api/v0/results"

	opts.addRemotesPath(apiPutResultPath)
	pr := newPersister(opts)

	go func() {
		for {
			prods.runCheckers(hname, pr.results)
			// TODO: must come from tests, this is more complicated
			time.Sleep(1 * time.Second)
		}
	}()

	api := &api{pr}
	http.HandleFunc(apiPutResultPath, api.handlePutResult)
	// TODO: make API to reload configuration (and restart checkers etc)
	log.Fatal(http.ListenAndServe(":8911", nil))
}
