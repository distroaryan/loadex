package test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/distroaryan/golb"
	healthchecker "github.com/distroaryan/golb/health_checker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	NUMBER_OF_SERVERS   = 5
	HEALTH_CHECK_PERIOD = 5 * time.Second
)

func StartMockServers() ([]*httptest.Server, []*url.URL, error) {
	servers := make([]*httptest.Server, NUMBER_OF_SERVERS)
	urls := make([]*url.URL, NUMBER_OF_SERVERS)

	for i := range NUMBER_OF_SERVERS {
		mux := http.NewServeMux()
		server := httptest.NewServer(mux)

		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			fmt.Println("Health check success success from server")
		})

		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, server.URL)
		})

		servers[i] = server
		url, err := url.Parse(server.URL)
		if err != nil {
			panic("Error parsing the mock server URL")
		}
		urls[i] = url
	}

	return servers, urls, nil
}

func NewMockLoadBalancer() (*url.URL, []*url.URL, []*httptest.Server, context.CancelFunc) {
	servers, urls, err := StartMockServers()
	if err != nil {
		panic("Error starting mock servers")
	}
	lb := golb.NewLoadBalancer("rr", urls)

	// Start the healthchecker
	hc := healthchecker.NewHealthChecker(HEALTH_CHECK_PERIOD, urls, lb)
	ctx, cancel := context.WithCancel(context.Background())
	hc.Start(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/", lb.Handler)
	server := httptest.NewServer(mux)
	lbURL, err := url.Parse(server.URL)
	if err != nil {
		panic("Error starting load balancer")
	}
	return lbURL, urls, servers, cancel
}

func TestRoundRobinDistribution(t *testing.T) {
	lbURL, urls, _, cancel := NewMockLoadBalancer()
	defer cancel()
	// Make 50 requests to the load balancer, each server should get 10 requests
	urlHitRate := map[string]int{}

	for _, url := range urls {
		urlHitRate[url.String()] = 0
	}

	for range 50 {
		resp, err := http.Get(lbURL.String())
		assert.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		urlHitRate[string(body)]++
		resp.Body.Close()
	}

	// Every url should be hit 10 times
	for _, hitRate := range urlHitRate {
		assert.Equal(t, 10, hitRate)
	}
}

func TestHealthCheckerMarksUnHealthyServer(t *testing.T) {
	lbURL, urls, servers, cancel := NewMockLoadBalancer()
	defer cancel()
	// FIRST 40 requests -> 5 servers -> 8 request per each
	// LAST 10 request -> 2 servers -> 5 request per each
	// 3 SERVERS -> 8 REQUESTS
	// 2 SERVERS -> 8 + 5 = 13 REQUESTS

	urlHitRate := map[string]int{}

	for _, url := range urls {
		urlHitRate[url.String()] = 0 
	}

	for range 40 {
		resp, err := http.Get(lbURL.String())
		assert.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		urlHitRate[string(body)]++
		resp.Body.Close()
	}

	// Every url should be hit 8 times
	for _, hitRate := range urlHitRate {
		assert.Equal(t, 8, hitRate)
	}

	// Close any random 3 servers
	closedServerUrls := map[string]bool{}
	for i:= range 3 {
		serverURL, err := url.Parse(servers[i].URL)
		require.NoError(t, err)
		closedServerUrls[serverURL.String()] = true 
		servers[i].Close()
	}

	// UPDATE THE HEALTH MAP
	time.Sleep(HEALTH_CHECK_PERIOD)

	// Now only 2 servers are up and running
	for range 10 {
		resp, err := http.Get(lbURL.String())
		assert.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		urlHitRate[string(body)]++
		resp.Body.Close()
	}

	for serverURL, hitRate := range urlHitRate {
		if closedServerUrls[serverURL] {
			assert.Equal(t, 8, hitRate)
		} else{
			assert.Equal(t, 13, hitRate)
		}
		fmt.Printf("URL %s. Hits %d\n", serverURL, hitRate)
	}
}

func TestServerRecovery(t *testing.T) {
	lbURL, urls, server, cancel := NewMockLoadBalancer()
	defer cancel()

	urlHitRate := map[string]int{}

	for _, url := range urls {
		urlHitRate[url.String()] = 0
	}

	// CLOSE FIRST 3 SERVERS
	// 2 SERVERS -> 10 REQUESTS -> 5 REQUEST EACH
	// 3 SERVERS -> 0 REQUESTS

	closedServerUrls := map[string]bool{}
	for i := range 3 {
		serverURL, err := url.Parse(server[i].URL)
		require.NoError(t, err)
		closedServerUrls[serverURL.String()] = true
		server[i].Close()
	}

	// UPDATE THE HEALTH MAP
	time.Sleep(HEALTH_CHECK_PERIOD)

	for range 10 {
		resp, err := http.Get(lbURL.String())
		assert.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		urlHitRate[string(body)]++
		resp.Body.Close()
	}

	for serverURL, hitRate := range urlHitRate {
		if closedServerUrls[serverURL] {
			assert.Equal(t, 0, hitRate)
		} else{
			assert.Equal(t, 5, hitRate)
		}
	}

	
}