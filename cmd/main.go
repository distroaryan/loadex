package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/distroaryan/golb"
)

func main() {
	var rawServersURL = make([]string, 10)
	for i := range 10 {
		rawServersURL[i] = fmt.Sprintf("http://localhost:%d", 8001+i)
	}

	var servers []*url.URL
	for _, raw := range rawServersURL {
		parsedURL, err := url.Parse(raw)
		if err != nil {
			continue
		}
		servers = append(servers, parsedURL)
	}

	lb := golb.NewLoadBalancer("rr", servers)

	// Start the Mock Server
	for i := range 10 {
		golb.StartServer(8001 + i)
	}

	http.HandleFunc("/", lb.Handler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
