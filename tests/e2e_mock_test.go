package test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/distroaryan/golb/health_checker"
	lb "github.com/distroaryan/golb/load_balancers"
	pool "github.com/distroaryan/golb/server_pool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	NUMBER_OF_REQUESTS  = 1000
	NUMBER_OF_SERVERS   = 10
	HEALTH_CHECK_PERIOD = 100 * time.Millisecond
)

const (
	ROUND_ROBIN          = "roundRobin"
	WEIGHTED_ROUND_ROBIN = "weightedRoundRobin"
	LEAST_CONNECTION     = "leastConnection"
	IP_HASH              = "ipHash"
)

type MockServer struct {
	Server *httptest.Server
	URL    *url.URL
	Alive  atomic.Bool
}

func NewMockServer() *MockServer {
	ms := &MockServer{}
	mux := http.NewServeMux()
	ms.Server = httptest.NewServer(mux)
	ms.Alive = atomic.Bool{}

	ms.Alive.Store(true)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if !ms.Alive.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, ms.Server.URL)
	})

	serverURL, err := url.Parse(ms.Server.URL)
	if err != nil {
		panic(fmt.Sprintf("Failed to parse the serverURL: %s\n", err))
	}

	ms.URL = serverURL
	return ms
}

func (ms *MockServer) Close() {
	ms.Server.Close()
}

type MockLoadBalancer struct {
	LoadBalancerSrv        *httptest.Server
	MockServers            []*MockServer
	ServerURLs             []string
	HealthcCheckCancelFunc context.CancelFunc
}

func NewMockLoadBalancer(algorithm string) *MockLoadBalancer {
	servers := make([]*MockServer, NUMBER_OF_SERVERS)
	urls := make([]string, NUMBER_OF_SERVERS)

	for i := range NUMBER_OF_SERVERS {
		servers[i] = NewMockServer()
		urls[i] = servers[i].Server.URL
	}

	serverPool := pool.NewServerPool(urls)
	lbInst := lb.NewLoadBalancer(serverPool)

	var handlerFunc http.HandlerFunc

	switch algorithm {
	case ROUND_ROBIN:
		handlerFunc = lbInst.RounRobin.Handler
	case WEIGHTED_ROUND_ROBIN:
		handlerFunc = lbInst.WeightedRoundRobin.Handler
	case LEAST_CONNECTION:
		handlerFunc = lbInst.LeastConnection.Handler
	case IP_HASH:
		handlerFunc = lbInst.IPHash.Handler
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handlerFunc)

	srv := httptest.NewServer(mux)

	hc := health_checker.NewHealthChecker(serverPool)
	ctx, cancel := context.WithCancel(context.Background())

	go hc.Start(ctx, HEALTH_CHECK_PERIOD)

	return &MockLoadBalancer{
		LoadBalancerSrv:        srv,
		MockServers:            servers,
		ServerURLs:             urls,
		HealthcCheckCancelFunc: cancel,
	}
}

func (mlb *MockLoadBalancer) Close() {
	mlb.HealthcCheckCancelFunc()
	mlb.LoadBalancerSrv.Close()
	for _, s := range mlb.MockServers {
		s.Close()
	}
}

var client = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:        10000,
		MaxIdleConnsPerHost: 10000,
		IdleConnTimeout:     30 * time.Second,
	}, // Keep-Alive reused
}

func assertRequestToLoadBalancer(t testing.TB, lb *MockLoadBalancer) string {
	t.Helper()

	lbURL := lb.LoadBalancerSrv.URL
	resp, err := client.Get(lbURL)
	require.NoError(t, err, "Request to Load Balancer failed")
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "Expected HTTP 200 OK from Load Balancer")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read response body")

	redirectedServerURL := string(body)
	assert.Greater(t, len(redirectedServerURL), 0, "Received empty body from LB — expected a server URL")
	return redirectedServerURL
}

func TestE2E_RoundRobin(t *testing.T) {
	t.Run("Distribution", func(t *testing.T) {
		lb := NewMockLoadBalancer(ROUND_ROBIN)
		defer lb.Close()

		serverHit := make(map[string]int)

		for range NUMBER_OF_REQUESTS {
			urlHit := assertRequestToLoadBalancer(t, lb)
			serverHit[urlHit]++
		}

		requestPerServer := NUMBER_OF_REQUESTS / NUMBER_OF_SERVERS
		for _, mockSrv := range lb.MockServers {
			assert.Equal(t, requestPerServer, serverHit[mockSrv.URL.String()])
		}
	})

	t.Run("Server crashes during journey and comes back up", func(t *testing.T) {
		lb := NewMockLoadBalancer(ROUND_ROBIN)
		defer lb.Close()

		lb.MockServers[2].Alive.Store(false)
		time.Sleep(HEALTH_CHECK_PERIOD * 2)

		serverHit := make(map[string]int)
		for range 900 {
			urlHit := assertRequestToLoadBalancer(t, lb)
			serverHit[urlHit]++
		}

		for url, hit := range serverHit {
			if url == lb.MockServers[2].Server.URL {
				assert.Equal(t, 0, hit)
			} else {
				assert.Equal(t, 100, hit)
			}
		}

		lb.MockServers[2].Alive.Store(true)
		time.Sleep(HEALTH_CHECK_PERIOD * 2)

		for range 100 {
			urlHit := assertRequestToLoadBalancer(t, lb)
			serverHit[urlHit]++
		}

		for url, hit := range serverHit {
			if url == lb.MockServers[2].Server.URL {
				assert.Equal(t, 10, hit)
			} else {
				assert.Equal(t, 110, hit)
			}
		}

	})

	t.Run("Concurrent Requests", func(t *testing.T) {
		lb := NewMockLoadBalancer(ROUND_ROBIN)
		defer lb.Close()

		var wg sync.WaitGroup
		var mu sync.Mutex 
		serverHit := map[string]int{}

		for range 100 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				urlHit := assertRequestToLoadBalancer(t, lb)
				mu.Lock()
				serverHit[urlHit]++
				mu.Unlock()
			}()
		}
		wg.Wait()

		for _, hit := range serverHit {
			assert.Equal(t, 10, hit)
		}
	})

	t.Run("All Servers Crash", func(t *testing.T) {
		lb := NewMockLoadBalancer(ROUND_ROBIN)
		defer lb.Close()

		for _, s := range lb.MockServers {
			s.Alive.Store(false)
		}
		time.Sleep(HEALTH_CHECK_PERIOD * 2)

		urlHit := assertRequestToLoadBalancer(t, lb)
		assert.Equal(t, "FAILED", urlHit)
	})
}

func TestE2E_WeightedRoundRobin(t *testing.T) {
	t.Run("Distribution", func(t *testing.T) {
		lb := NewMockLoadBalancer(WEIGHTED_ROUND_ROBIN)
		defer lb.Close()

		serverHit := make(map[string]int)
		for range NUMBER_OF_REQUESTS {
			urlHit := assertRequestToLoadBalancer(t, lb)
			serverHit[urlHit]++
		}

		expected := []int{134, 67, 134, 67, 134, 67, 133, 66, 132, 66}
		for i, s := range lb.MockServers {
			assert.Equal(t, expected[i], serverHit[s.URL.String()])
		}
	})

	t.Run("Server crashes during journey and comes back up", func(t *testing.T) {
		lb := NewMockLoadBalancer(WEIGHTED_ROUND_ROBIN)
		defer lb.Close()

		lb.MockServers[0].Alive.Store(false)
		time.Sleep(HEALTH_CHECK_PERIOD * 2)

		serverHit := make(map[string]int)
		for range 900 {
			urlHit := assertRequestToLoadBalancer(t, lb)
			serverHit[urlHit]++
		}

		expectedWithCrash := []int{0, 70, 140, 69, 138, 69, 138, 69, 138, 69}
		for i, s := range lb.MockServers {
			assert.Equal(t, expectedWithCrash[i], serverHit[s.URL.String()])
		}

		lb.MockServers[0].Alive.Store(true)
		time.Sleep(HEALTH_CHECK_PERIOD * 2)

		for range 100 {
			urlHit := assertRequestToLoadBalancer(t, lb)
			serverHit[urlHit]++
		}

		expectedRecover := []int{12, 76, 152, 76, 152, 76, 152, 76, 152, 76}
		for i, s := range lb.MockServers {
			assert.Equal(t, expectedRecover[i], serverHit[s.URL.String()])
		}
	})

	t.Run("Concurrent Requests", func(t *testing.T) {
		lb := NewMockLoadBalancer(WEIGHTED_ROUND_ROBIN)
		defer lb.Close()

		var wg sync.WaitGroup
		var mu sync.Mutex 
		serverHit := map[string]int{}

		for range 100 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				urlHit := assertRequestToLoadBalancer(t, lb)
				mu.Lock()
				serverHit[urlHit]++
				mu.Unlock()
			}()
		}
		wg.Wait()

		expected := []int{14, 7, 14, 7, 14, 7, 13, 6, 12, 6}
		for i, s := range lb.MockServers {
			assert.Equal(t, expected[i], serverHit[s.URL.String()])
		}
	})

	t.Run("All Servers Crash", func(t *testing.T) {
		lb := NewMockLoadBalancer(WEIGHTED_ROUND_ROBIN)
		defer lb.Close()

		for _, s := range lb.MockServers {
			s.Alive.Store(false)
		}
		time.Sleep(HEALTH_CHECK_PERIOD * 2)

		urlHit := assertRequestToLoadBalancer(t, lb)
		assert.Equal(t, "FAILED", urlHit)
	})
}

func TestE2E_LeastConnection(t *testing.T) {
	t.Run("Distribution", func(t *testing.T) {
		lb := NewMockLoadBalancer(LEAST_CONNECTION)
		defer lb.Close()

		serverHit := make(map[string]int)
		for range NUMBER_OF_REQUESTS {
			urlHit := assertRequestToLoadBalancer(t, lb)
			serverHit[urlHit]++
		}

		requestPerServer := NUMBER_OF_REQUESTS / NUMBER_OF_SERVERS
		for _, s := range lb.MockServers {
			assert.Equal(t, requestPerServer, serverHit[s.URL.String()])
		}
	})

	t.Run("Server crashes during journey and comes back up", func(t *testing.T) {
		lb := NewMockLoadBalancer(LEAST_CONNECTION)
		defer lb.Close()

		lb.MockServers[2].Alive.Store(false)
		time.Sleep(HEALTH_CHECK_PERIOD * 2)

		serverHit := make(map[string]int)
		for range 900 {
			urlHit := assertRequestToLoadBalancer(t, lb)
			serverHit[urlHit]++
		}

		for url, hit := range serverHit {
			if url == lb.MockServers[2].Server.URL {
				assert.Equal(t, 0, hit)
			} else {
				assert.Equal(t, 100, hit)
			}
		}

		lb.MockServers[2].Alive.Store(true)
		time.Sleep(HEALTH_CHECK_PERIOD * 2)

		for range 100 {
			urlHit := assertRequestToLoadBalancer(t, lb)
			serverHit[urlHit]++
		}

		// Because minConns for server 2 is at 0, while others are at 100, 
		// server 2 will receive the next 100 continuous requests.
		for url, hit := range serverHit {
			if url == lb.MockServers[2].Server.URL {
				assert.Equal(t, 100, hit)
			} else {
				assert.Equal(t, 100, hit)
			}
		}
	})

	t.Run("Concurrent Requests", func(t *testing.T) {
		lb := NewMockLoadBalancer(LEAST_CONNECTION)
		defer lb.Close()

		var wg sync.WaitGroup
		var mu sync.Mutex 
		serverHit := map[string]int{}

		for range 100 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				urlHit := assertRequestToLoadBalancer(t, lb)
				mu.Lock()
				serverHit[urlHit]++
				mu.Unlock()
			}()
		}
		wg.Wait()

		for _, hit := range serverHit {
			assert.Equal(t, 10, hit)
		}
	})

	t.Run("All Servers Crash", func(t *testing.T) {
		lb := NewMockLoadBalancer(LEAST_CONNECTION)
		defer lb.Close()

		for _, s := range lb.MockServers {
			s.Alive.Store(false)
		}
		time.Sleep(HEALTH_CHECK_PERIOD * 2)

		urlHit := assertRequestToLoadBalancer(t, lb)
		assert.Equal(t, "FAILED", urlHit)
	})
}

func TestE2E_IPHash(t *testing.T) {
	t.Run("Distribution Stickiness", func(t *testing.T) {
		lb := NewMockLoadBalancer(IP_HASH)
		defer lb.Close()

		serverHit := make(map[string]int)
		for range NUMBER_OF_REQUESTS {
			urlHit := assertRequestToLoadBalancer(t, lb)
			serverHit[urlHit]++
		}

		assert.Equal(t, 1, len(serverHit), "IPHash should direct all Keep-Alive identical IPs to the same server")
		for _, hit := range serverHit {
			assert.Equal(t, NUMBER_OF_REQUESTS, hit)
		}
	})

	t.Run("Server crashes during journey and comes back up", func(t *testing.T) {
		lb := NewMockLoadBalancer(IP_HASH)
		defer lb.Close()

		urlHitStr := assertRequestToLoadBalancer(t, lb)

		var killedServer *MockServer
		for _, s := range lb.MockServers {
			if s.URL.String() == urlHitStr {
				s.Alive.Store(false)
				killedServer = s
			}
		}

		time.Sleep(HEALTH_CHECK_PERIOD * 2)

		serverHit := make(map[string]int)
		for range 10 {
			newURL := assertRequestToLoadBalancer(t, lb)
			serverHit[newURL]++
		}

		assert.Equal(t, 1, len(serverHit), "Should failover to a new single server")
		assert.Equal(t, 0, serverHit[killedServer.URL.String()])
        
        var backupServerURL string 
        for k := range serverHit {
            backupServerURL = k 
        }

		// Now bring it back up
		killedServer.Alive.Store(true)
		time.Sleep(HEALTH_CHECK_PERIOD * 2)

		for range 10 {
			newURL := assertRequestToLoadBalancer(t, lb)
			serverHit[newURL]++
		}

		// When killed server comes back, IP Hash restores mapping cleanly.
		assert.Equal(t, 2, len(serverHit), "Should revert to the original server")
		assert.Equal(t, 10, serverHit[killedServer.URL.String()])
		assert.Equal(t, 10, serverHit[backupServerURL])
	})

	t.Run("Concurrent Requests", func(t *testing.T) {
		lb := NewMockLoadBalancer(IP_HASH)
		defer lb.Close()

		var wg sync.WaitGroup
		var mu sync.Mutex 
		serverHit := map[string]int{}

		for range 100 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				urlHit := assertRequestToLoadBalancer(t, lb)
				mu.Lock()
				serverHit[urlHit]++
				mu.Unlock()
			}()
		}
		wg.Wait()

		assert.Equal(t, 1, len(serverHit), "All requests from same IP run concurrently should hit same server")
		for _, hit := range serverHit {
			assert.Equal(t, 100, hit)
		}
	})

	t.Run("All Servers Crash", func(t *testing.T) {
		lb := NewMockLoadBalancer(IP_HASH)
		defer lb.Close()

		for _, s := range lb.MockServers {
			s.Alive.Store(false)
		}
		time.Sleep(HEALTH_CHECK_PERIOD * 2)

		urlHit := assertRequestToLoadBalancer(t, lb)
		assert.Equal(t, "FAILED", urlHit)
	})
}
