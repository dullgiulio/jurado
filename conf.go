package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

type config struct {
	Products ConfProducts
	Agents   map[string]ConfAgent
}

func (c config) forHostname(h string) (*ConfAgent, error) {
	agent, ok := c.Agents[h]
	if !ok {
		return nil, fmt.Errorf("no configuration for host %s", h)
	}
	return &agent, nil
}

type ConfProducts map[string][]Check

// TODO: this is in the wrong file/on the wrong type
func (p ConfProducts) init() error {
	for name, prods := range p {
		for i := range prods {
			prods[i].Product = name
			if err := prods[i].init(); err != nil {
				return err
			}
		}
	}
	return nil
}

type ConfAgent struct {
	Checks  []string
	Options ConfOptions
}

type ConfOptions struct {
	File    string
	Remotes []string
}

func (a *ConfAgent) addRemotesPath(path string) {
	for i := range a.Options.Remotes {
		a.Options.Remotes[i] = a.Options.Remotes[i] + path
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
