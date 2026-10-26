package core

import (
	"net/http"
	"strconv"
	"time"
)

// newSystemServer — the system port: closed from outside the cluster (no
// ingress), for k8s probes and pulse. /healthcheck only says the process is up
// and never touches the db, so a db outage does not restart core.
func newSystemServer(port int, registers ...func(mux *http.ServeMux)) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthcheck", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	for _, register := range registers {
		register(mux)
	}

	return &http.Server{
		Addr:              ":" + strconv.Itoa(port),
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       10 * time.Second,
		MaxHeaderBytes:    32 * 1024,
	}
}
