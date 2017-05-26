package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

type config map[hostname]struct {
	Options  ConfOptions
	Products ConfProducts
}

func (c config) forHostname(h hostname) (ConfProducts, *ConfOptions, error) {
	cf, ok := c[h]
	if !ok {
		return nil, nil, fmt.Errorf("no configuration for host %s", h)
	}
	return cf.Products, &cf.Options, nil
}

type ConfProducts map[string][]Check

// TODO: this is in the wrong file/on the wrong type
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
	if url[0:7] == "http://" {
		return readHttpFile(url)
	}
	if url[0:7] == "file://" {
		url = url[7:]
	}
	return readLocalFile(url)
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
