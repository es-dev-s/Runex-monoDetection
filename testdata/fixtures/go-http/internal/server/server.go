package server

import (
	"net/http"
)

func Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return http.ListenAndServe(":18780", mux)
}
