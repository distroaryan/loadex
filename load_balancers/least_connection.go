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

type LeastConnection struct {
	serverPool *pool.ServerPool 
	activeConns map[string]int
	mu sync.Mutex	
}

func NewLeastConnection(serverPool *pool.ServerPool) *LeastConnection {
	return &LeastConnection{
		serverPool: serverPool,
		activeConns: make(map[string]int),
	}
}

func (lb *LeastConnection) NextServer(r *http.Request) (*url.URL, error) {
	servers := lb.serverPool.GetServers()
	if len(servers) == 0 {
		return nil, fmt.Errorf("No servers are available")
	}

	lb.mu.Lock()
	defer lb.mu.Unlock()

	var selectedServerURL string
	minConns := 10000

	for _, srv := range servers {
		if !lb.serverPool.IsAlive(srv) {
			continue
		}

		conns := lb.activeConns[srv]
		if conns < minConns {
			minConns = conns
			selectedServerURL = srv 
		}
	}

	if selectedServerURL == "" {
		if logger.Log != nil {
			logger.Log.Warn("All servers are down")
		}
		return nil, fmt.Errorf("All servers are down")
	}

	lb.activeConns[selectedServerURL]++
	if logger.Log != nil {
		logger.Log.Debug("Selected server via Least Connection", "server", selectedServerURL, "activeConns", lb.activeConns[selectedServerURL])
	}
	return url.Parse(selectedServerURL)
}

func (lb *LeastConnection) Handler(w http.ResponseWriter, r *http.Request) {
	if logger.Log != nil {
		logger.Log.Info("Received request", "method", r.Method, "url", r.URL.String(), "remoteAddr", r.RemoteAddr)
	}
	servers := lb.serverPool.GetServers()
	maxRetries := len(servers)

	for range maxRetries {
		target, err := lb.NextServer(r)
		if err != nil {
			http.Error(w, "No Servers Available", http.StatusServiceUnavailable)
			return 
		}
		proxy := httputil.NewSingleHostReverseProxy(target)
		proxyFailed := false

		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
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