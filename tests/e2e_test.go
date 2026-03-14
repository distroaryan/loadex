package test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
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

type MockServer struct {
	Server *httptest.Server
	URL    *url.URL
	Alive  *atomic.Bool
}

func StartMockServers() []*MockServer {
	servers := make([]*MockServer, NUMBER_OF_SERVERS)

	for i := range NUMBER_OF_SERVERS {
		servers[i] = &MockServer{
			Alive: &atomic.Bool{},
		}

		servers[i].Alive.Store(true)

		mux := http.NewServeMux()
		servers[i].Server = httptest.NewServer(mux)

		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			if !servers[i].Alive.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
		})

		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, servers[i].Server.URL)
		})

		url, err := url.Parse(servers[i].Server.URL)
		if err != nil {
			panic("Error parsing the mock server URL")
		}
		servers[i].URL = url
	}

	return servers
}

func NewMockLoadBalancer() (*url.URL, []*MockServer, context.CancelFunc) {
	mockServers := StartMockServers()
	serverURLs := []*url.URL{}
	for _, mockServer := range mockServers {
		serverURLs = append(serverURLs, mockServer.URL)
	}
	lb := golb.NewLoadBalancer("rr", serverURLs)

	// Start the healthchecker
	hc := healthchecker.NewHealthChecker(HEALTH_CHECK_PERIOD, serverURLs, lb)
	ctx, cancel := context.WithCancel(context.Background())
	hc.Start(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/", lb.Handler)
	server := httptest.NewServer(mux)
	lbURL, err := url.Parse(server.URL)
	if err != nil {
		panic("Error starting load balancer")
	}
	return lbURL, mockServers, cancel
}

func TestRoundRobinDistribution(t *testing.T) {
	lbURL, mockServers, cancel := NewMockLoadBalancer()
	defer cancel()
	// Make 50 requests to the load balancer, each server should get 10 requests
	urlHitRate := map[string]int{}

	for _, s := range mockServers {
		urlHitRate[s.URL.String()] = 0
	}

	for range 50 {
		serverURL := assertRequestToLoadBalancer(t, lbURL)
		urlHitRate[serverURL]++
	}

	// Every url should be hit 10 times
	for _, hitRate := range urlHitRate {
		assert.Equal(t, 10, hitRate, "Each server should get exactly 10 requests")
	}
}

func TestHealthCheckerMarksUnHealthyServer(t *testing.T) {
	lbURL, mockServers, cancel := NewMockLoadBalancer()
	defer cancel()
	// FIRST 40 requests -> 5 servers -> 8 request per each
	// LAST 10 request -> 2 servers -> 5 request per each
	// 3 SERVERS -> 8 REQUESTS
	// 2 SERVERS -> 8 + 5 = 13 REQUESTS

	urlHitRate := map[string]int{}

	for _, s := range mockServers {
		urlHitRate[s.Server.URL] = 0
	}

	for range 40 {
		serverURL := assertRequestToLoadBalancer(t, lbURL)
		urlHitRate[serverURL]++
	}

	// Every url should be hit 8 times
	for _, hitRate := range urlHitRate {
		assert.Equal(t, 8, hitRate, "Each server should get exactly 8 requests initially")
	}

	// Close any random 3 servers
	closedServerUrls := map[string]bool{}
	for i := range 3 {
		serverURL := mockServers[i].Server.URL
		closedServerUrls[serverURL] = true
		mockServers[i].Server.Close()
	}

	// UPDATE THE HEALTH MAP
	time.Sleep(HEALTH_CHECK_PERIOD)

	// Now only 2 servers are up and running
	for range 10 {
		serverURL := assertRequestToLoadBalancer(t, lbURL)
		urlHitRate[serverURL]++
	}

	for serverURL, hitRate := range urlHitRate {
		if closedServerUrls[serverURL] {
			assert.Equal(t, 8, hitRate, "Closed server %s should not receive new requests", serverURL)
		} else {
			assert.Equal(t, 13, hitRate, "Active server %s should have received exactly 13 requests", serverURL)
		}
		fmt.Printf("URL %s. Hits %d\n", serverURL, hitRate)
	}
}

func TestServerRecovery(t *testing.T) {
	lbURL, mockServers, cancel := NewMockLoadBalancer()
	defer cancel()

	urlHitRate := map[string]int{}

	for _, s := range mockServers {
		urlHitRate[s.Server.URL] = 0
	}

	// CLOSE FIRST 3 SERVERS
	// 2 SERVERS -> 10 REQUESTS -> 5 REQUEST EACH
	// 3 SERVERS -> 0 REQUESTS

	closedServerUrls := map[string]bool{}
	for i := range 3 {
		serverURL := mockServers[i].Server.URL
		mockServers[i].Alive.Store(false)
		closedServerUrls[serverURL] = true
	}

	// UPDATE THE HEALTH MAP
	time.Sleep(2 * HEALTH_CHECK_PERIOD)

	for range 10 {
		serverURL := assertRequestToLoadBalancer(t, lbURL)
		urlHitRate[serverURL]++
	}

	for serverURL, hitRate := range urlHitRate {
		// t.Logf("Server calls %d", hitRate)
		if closedServerUrls[serverURL] {
			assert.Equal(t, 0, hitRate, "Closed server %s should not receive any requests", serverURL)
		} else {
			assert.Equal(t, 5, hitRate, "Active server %s should have received exactly 5 requests", serverURL)
		}
	}

	// // START THE FIRST 3 SERVERS
	for i := range 3 {
		mockServers[i].Alive.Store(true)
	}

	// UPDATE THE HEALTH MAP
	time.Sleep(2 * HEALTH_CHECK_PERIOD)

	// // CURRENT STATE
	// // 3 SERVERS -> 0 REQUESTS (NOW-ACTIVE)
	// // 2 SERVERS -> 5 REQUESTS EACH

	for range 50 {
		serverURL := assertRequestToLoadBalancer(t, lbURL)
		urlHitRate[serverURL]++
	}

	// // CURRENT STATE
	// // 3 REQUESTS -> 10 REQUESTS
	// // 2 SERVERS -> 5 + 10 = 15

	for i, mockServer := range mockServers {
		serverURL := mockServer.Server.URL
		if i < 3 {
			assert.Equal(t, 10, urlHitRate[serverURL], "Recovered server %s should have 10 total requests", serverURL)
		} else {
			assert.Equal(t, 15, urlHitRate[serverURL], "Continuous active server %s should have 15 total requests", serverURL)
		}
	}
}

var client = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:        10000,
		MaxIdleConnsPerHost: 10000,
		IdleConnTimeout:     30 * time.Second,
	},
}

func assertRequestToLoadBalancer(t testing.TB, lbURL *url.URL) string {
	t.Helper()
	resp, err := client.Get(lbURL.String())
	require.NoError(t, err, "Request to Load Balancer failed")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "Expected HTTP 200 OK from Load Balancer")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read response body")

	redirectedServerURL := string(body)
	assert.Greater(t, len(redirectedServerURL), 0, "Received empty body from LB — expected a server URL")
	return redirectedServerURL
}
