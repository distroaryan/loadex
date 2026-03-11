package test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/distroaryan/golb"
	healthchecker "github.com/distroaryan/golb/health_checker"
)

const (
	NUMBER_OF_SERVERS   = 5
	HEALTH_CHECK_PERIOD = 5 * time.Second
)

func StartMockServers() ([]*httptest.Server, []*url.URL, error) {
	servers := make([]*httptest.Server, NUMBER_OF_SERVERS)
	urls := make([]*url.URL, NUMBER_OF_SERVERS)

	for range NUMBER_OF_SERVERS {
		mux := http.NewServeMux()

		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			fmt.Println("Health check success success from server")
		})

		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Printf("Hello from server")
		})

		server := httptest.NewServer(mux)
		servers = append(servers, server)
		url, err := url.Parse(server.URL)
		if err != nil {
			panic("Error parsing the mock server URL")
		}
		urls = append(urls, url)
	}

	return servers, urls, nil 
}

func NewMockLoadBalancer() *url.URL {
	_, urls, err := StartMockServers()
	if err != nil {
		panic("Error starting mock servers")
	}
	lb := golb.NewLoadBalancer("rr", urls)

	// Start the healthchecker
	hc := healthchecker.NewHealthChecker(HEALTH_CHECK_PERIOD, urls, lb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hc.Start(ctx)

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	lbURL, err := url.Parse(server.URL)
	if err != nil {
		panic("Error starting load balancer")
	}
	return lbURL
}

func TestRequestToBackend(t *testing.T) {
	
}
