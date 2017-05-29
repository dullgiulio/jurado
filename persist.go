package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

type persister struct {
	agent    *ConfAgent
	incoming chan *CheckResult
	results  chan *CheckResult
	status   status
}

func newPersister(a *ConfAgent) *persister {
	p := &persister{
		agent:    a,
		status:   makeStatus(),
		incoming: make(chan *CheckResult),
		results:  make(chan *CheckResult),
	}
	go p.processIncoming(p.incoming)
	go p.processResults(p.results)
	return p
}

// Events coming from other agents
func (p *persister) processIncoming(in <-chan *CheckResult) {
	for cr := range in {
		if p.agent.Options.File == "" {
			continue
		}
		p.status.add(cr)
		data, err := json.Marshal(p.status)
		if err != nil {
			log.Fatalf("cannot marshal tests status: %s", err)
		}
		if err := writeFileAtomic(p.agent.Options.File, data, os.FileMode(0644)); err != nil {
			log.Fatalf("cannot write results JSON file: %s", err)
		}
	}
}

// Events coming from our tests
func (p *persister) processResults(in <-chan *CheckResult) {
	for r := range in {
		if len(p.agent.Options.Remotes) == 0 {
			p.incoming <- r
			continue
		}
		errs, err := p.putRemotes(p.agent.Options.Remotes, r)
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

func (p *persister) putRemotes(remotes []string, result *CheckResult) ([]error, error) {
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

func (p *persister) receiveCheckResult(req *http.Request) error {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return fmt.Errorf("cannot read request body: %s", err)
	}
	cr := &CheckResult{}
	if err := json.Unmarshal(body, &cr); err != nil {
		return fmt.Errorf("cannot unmarshal into result object: %s", err)
	}
	p.incoming <- cr
	return err
}
