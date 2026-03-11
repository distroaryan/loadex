package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/distroaryan/golb"
	healthchecker "github.com/distroaryan/golb/health_checker"
)

const (
	NUMBER_OF_SERVERS = 5
	HEALTH_CHECK_PERIOD = 5 * time.Second
)

func main() {
	var rawServersURL = make([]string, NUMBER_OF_SERVERS)
	for i := range NUMBER_OF_SERVERS {
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
	for i := range NUMBER_OF_SERVERS {
		golb.StartServer(8001 + i)
	}

	// Start the healthchecker
	hc := healthchecker.NewHealthChecker(HEALTH_CHECK_PERIOD, servers, lb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hc.Start(ctx)

	http.HandleFunc("/", lb.Handler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
