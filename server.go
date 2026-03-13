package golb

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

func StartServer(port int) {
	addr := fmt.Sprintf(":%d", port)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	go func(){
		log.Printf("Server started on PORT: %d", port)
		log.Fatal(http.ListenAndServe(addr, mux))
	}()
}

func StopServer(server *http.Server) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}