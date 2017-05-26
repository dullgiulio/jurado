package main

import (
	"io"
	"net/http"
)

const (
	apiPutResultPath = "/api/v0/results"
)

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
