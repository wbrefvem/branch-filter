package main

import (
	"fmt"
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/trysterodev/branch-filter/pkg/branchfilter"
)

func main() {
	log.SetFormatter(&log.JSONFormatter{})

	mux, err := branchfilter.NewBranchFilterServeMux()
	if err != nil {
		log.Fatalf("error initializing branch filter serve mux: %s", err)
	}

	if mux == nil {
		log.Fatal("unknown error initializing serve mux")
	}

	log.Infof("now listenting for webhooks on port %s", mux.Port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", mux.Port), mux); err != nil {
		log.Fatalf("error starting http server: %s", err)
	}

	os.Exit(0)
}
