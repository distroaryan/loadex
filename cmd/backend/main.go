package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
)

func main() {
	var port int
	flag.IntVar(&port, "port", 8081, "Port for dummy backend server")
	flag.Parse()

	hostname, _ := os.Hostname()

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Response from backend: %s:%d\n", hostname, port)
	})

	fmt.Printf("Starting dummy backend on port %d\n", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		fmt.Println("Server crashed:", err)
	}
}
