package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"
)

type hostname string

func main() {
	listen := flag.String("listen", ":8911", "What address/port to listen to")
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
		log.Fatalf("cannot use configuration: %s", err)
	}

	opts.addRemotesPath(apiPutResultPath)
	pr := newPersister(opts)

	if err := prods.init(); err != nil {
		log.Fatalf("cannot start: %s", err)
	}

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
	log.Fatal(http.ListenAndServe(*listen, nil))
}
