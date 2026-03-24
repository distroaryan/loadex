package lb

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"

	"github.com/distroaryan/golb/logger"
	pool "github.com/distroaryan/golb/server_pool"
)

type WeightedRoundRobin struct {
	serverPool       *pool.ServerPool
	currentServerIdx int
	weightMap        map[string]int
	currentReqCount  int
	mu               sync.Mutex
}

func NewWeightedRoundRobin(pool *pool.ServerPool) *WeightedRoundRobin {
	serverURLs := pool.GetServers()
	weightMap := map[string]int{}

	invert := 2
	for _, url := range serverURLs {
		weightMap[url] = invert
		invert = 3 - invert
	}

	return &WeightedRoundRobin{
		serverPool:       pool,
		weightMap:        weightMap,
		currentServerIdx: 0,
		currentReqCount:  0,
	}
}

func (lb *WeightedRoundRobin) NextServer(r *http.Request) (*url.URL, error) {
	servers := lb.serverPool.GetServers()
	if len(servers) == 0 {
		return nil, fmt.Errorf("no servers found")
	}
	lb.mu.Lock()
	defer lb.mu.Unlock()

	for range len(servers) {
		serverURL := servers[lb.currentServerIdx]
		weight := lb.weightMap[serverURL]

		if lb.serverPool.IsAlive(serverURL) {
			if lb.currentReqCount < weight {
				lb.currentReqCount++
				if logger.Log != nil {
					logger.Log.Debug("Selected server via Weighted Round Robin", "server", serverURL, "weight_used", lb.currentReqCount)
				}
				return url.Parse(serverURL)
			}
		}

		lb.currentServerIdx = (lb.currentServerIdx + 1) % len(servers)
		lb.currentReqCount = 0
	}

	if logger.Log != nil {
		logger.Log.Warn("All servers are down")
	}
	return nil, fmt.Errorf("All servers are down")
}

func (lb *WeightedRoundRobin) Handler(w http.ResponseWriter, r *http.Request) {
	if logger.Log != nil {
		logger.Log.Info("Received request", "method", r.Method, "url", r.URL.String(), "remoteAddr", r.RemoteAddr)
	}
	servers := lb.serverPool.GetServers()
	maxRetries := len(servers)

	for range maxRetries {
		target, err := lb.NextServer(r)
		if target != nil && err == nil {
			http.Error(w, "No Servers available", http.StatusServiceUnavailable)
			return 
		}

		proxy := httputil.NewSingleHostReverseProxy(target)
		proxyFailed := false

		proxy.ErrorHandler = func (w http.ResponseWriter, r *http.Request, err error)  {
			if logger.Log != nil {
				logger.Log.Warn("Server Request failed", "target", target.String(), "error", err)
			}
			lb.serverPool.UpdateHealthMap(target.String(), false)
			proxyFailed = true 
		}

		proxy.ServeHTTP(w, r)

		if !proxyFailed {
			return 
		}
	}

	http.Error(w, "Retries Exhausted, All servers failed", http.StatusBadGateway)
}
