package lb

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	pool "github.com/distroaryan/golb/server_pool"
	"github.com/stretchr/testify/assert"
)

type MockServer struct {
	server *httptest.Server
}

func NewMockServer(id int) *MockServer {
	ms := &MockServer{}
	mux := http.NewServeMux()
	ms.server = httptest.NewServer(mux)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return ms
}

func (ms *MockServer) Close() {
	ms.server.Close()
}

func (ms *MockServer) URL() string {
	return ms.server.URL
}

func TestConcurrentAndSerialRequests(t *testing.T) {
	tests := []struct {
		algorithm     string
		totalRequests int
		totalServers  int
		checkFun      func(t testing.TB, urls []string, serverHit map[string]int)
	}{
		{
			algorithm:     "roundRobin",
			totalRequests: 1000,
			totalServers:  10,
			checkFun: func(t testing.TB, urls []string, serverHit map[string]int) {
				t.Helper()
				requestPerServer := 100
				for _, hit := range serverHit {
					assert.Equal(t, requestPerServer, hit)
				}
			},
		},
		{
			algorithm:     "weightedRoundRobin",
			totalRequests: 1000,
			totalServers:  10,
			checkFun: func(t testing.TB, urls []string, serverHit map[string]int) {
				t.Helper()
				// weight assigned -> [2, 1, 2, 1, 2, 1, 2, 1, 2, 1]
				// Total weight per cycle = 15
				// 1000 requests / 15 = 66 cycles (990 requests)
				// Remaining 10 requests: assigned to first 7 servers until exhausted
				expected := []int{134, 67, 134, 67, 134, 67, 133, 66, 132, 66}
				for i, url := range urls {
					assert.Equal(t, expected[i], serverHit[url], "mismatch for server index %d", i)
				}
			},
		},
		{
			algorithm:     "leastConnection",
			totalRequests: 1000,
			totalServers:  10,
			checkFun: func(t testing.TB, urls []string, serverHit map[string]int) {
				t.Helper()
				// Since test never releases connections, least connection behaves like perfectly-balanced round robin
				requestPerServer := 100
				for _, url := range urls {
					assert.Equal(t, requestPerServer, serverHit[url])
				}
			},
		},
		{
			algorithm:     "urlHash",
			totalRequests: 1000,
			totalServers:  10,
			checkFun: func(t testing.TB, urls []string, serverHit map[string]int) {
				t.Helper()
				// IP Hash is expected to route all requests with the same IP to the same server.
				assert.Equal(t, 1, len(serverHit))
				for _, hit := range serverHit {
					assert.Equal(t, 1000, hit)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.algorithm+"-concurrent", func(t *testing.T) {
			totalServers := tt.totalServers
			servers := make([]*MockServer, totalServers)
			urls := make([]string, totalServers)

			for i := range totalServers {
				servers[i] = NewMockServer(i)
				urls[i] = servers[i].URL()
				defer servers[i].Close()
			}

			serverPool := pool.NewServerPool(urls)

			var lb LB
			switch tt.algorithm {
			case "roundRobin":
				lb = NewRoundRobin(serverPool)
			case "weightedRoundRobin":
				lb = NewWeightedRoundRobin(serverPool)
			case "leastConnection":
				lb = NewLeastConnection(serverPool)
			case "urlHash":
				lb = NewIPHash(serverPool)
			}

			serverHits := make(map[string]int)

			var wg sync.WaitGroup
			var mu sync.Mutex

			for range tt.totalRequests {
				wg.Add(1)
				go func() {
					defer wg.Done()
					req := httptest.NewRequest(http.MethodGet, "/", nil)
					server, err := lb.NextServer(req)
					if err == nil && server != nil {
						mu.Lock()
						serverHits[server.String()]++
						mu.Unlock()
					}
				}()
			}
			wg.Wait()

			tt.checkFun(t, urls, serverHits)
		})
	}

	for _, tt := range tests {
		t.Run(tt.algorithm+"-serial", func(t *testing.T) {
			totalServers := tt.totalServers
			totalRequests := tt.totalRequests
			servers := make([]*MockServer, totalServers)
			urls := make([]string, totalServers)

			for i := range totalServers {
				servers[i] = NewMockServer(i)
				urls[i] = servers[i].URL()
				defer servers[i].Close()
			}

			serverPool := pool.NewServerPool(urls)

			var lb LB
			switch tt.algorithm {
			case "roundRobin":
				lb = NewRoundRobin(serverPool)
			case "weightedRoundRobin":
				lb = NewWeightedRoundRobin(serverPool)
			case "leastConnection":
				lb = NewLeastConnection(serverPool)
			case "urlHash":
				lb = NewIPHash(serverPool)
			}

			serverHits := make(map[string]int)

			for range totalRequests {
				server, err := lb.NextServer(httptest.NewRequest(http.MethodGet, "/", nil))
				if err == nil && server != nil {
					serverHits[server.String()]++
				}
			}

			tt.checkFun(t, urls, serverHits)
		})
	}
}

func TestEdgeCases(t *testing.T) {
	// 0 servers
	urls := []string{}

	serverPool := pool.NewServerPool(urls)
	lb := NewRoundRobin(serverPool)

	acceptedReq := 0

	for range 50 {
		server, err := lb.NextServer(httptest.NewRequest(http.MethodGet, "/", nil))
		if err == nil && server != nil {
			acceptedReq++
		}
	}
	assert.Equal(t, 0, acceptedReq)
}
