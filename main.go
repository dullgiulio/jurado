package main

import (
	"flag"
	"log"
	"net/http"
	"os"
)

func main() {
	listen := flag.String("listen", ":8911", "What address/port to listen to")
	nTestWorkers := flag.Int("tworkers", 4, "Number of parallel test runners")
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
	agent, err := conf.forHostname(hname)
	if err != nil {
		log.Fatalf("cannot use configuration: %s", err)
	}

	agent.addRemotesPath(apiPutResultPath)
	pr := newPersister(agent)

	if err := conf.Products.init(); err != nil {
		log.Fatalf("cannot start: %s", err)
	}

	ts := make([]*task, 0)
	for _, prod := range agent.Checks {
		checks, ok := conf.Products[prod]
		if !ok {
			log.Fatalf("non-existing product %s to be checked for this host", prod)
		}
		for i := range checks {
			t, err := newTask(&checks[i])
			if err != nil {
				log.Fatalf("cannot make task for check %s: %s\n", checks[i], err)
			}
			ts = append(ts, t)
		}
	}

	s := newScheduler(hname, ts, pr.results, *nTestWorkers)
	go s.schedule()

	api := &api{pr}
	http.HandleFunc(apiPutResultPath, api.handlePutResult)
	// TODO: make API to reload configuration (and restart checkers etc)
	log.Fatal(http.ListenAndServe(*listen, nil))
}
