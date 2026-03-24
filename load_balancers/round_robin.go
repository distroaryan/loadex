package lb

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"

	"github.com/distroaryan/golb/logger"
	pool "github.com/distroaryan/golb/server_pool"
)

type RoundRobin struct {
	serverPool            *pool.ServerPool
	currentServerURLIndex uint64
}

func NewRoundRobin(serverPool *pool.ServerPool) *RoundRobin {
	return &RoundRobin{
		serverPool:            serverPool,
		currentServerURLIndex: 0,
	}
}

func (lb *RoundRobin) NextServer(r *http.Request) (*url.URL, error) {
	servers := lb.serverPool.GetServers()
	if len(servers) == 0 {
		return nil, fmt.Errorf("no servers found")
	}

	var serverURL *url.URL
	var err error

	for range len(servers) {
		idx := atomic.AddUint64(&lb.currentServerURLIndex, 1)
		nextURLString := servers[(idx-1)%uint64(len(servers))]

		if lb.serverPool.IsAlive(nextURLString) {
			serverURL, err = url.Parse(nextURLString)
			if err != nil {
				return nil, err
			}
			if logger.Log != nil {
				logger.Log.Debug("Selected server via Round Robin", "server", serverURL.String())
			}
			return serverURL, nil
		}
	}

	if logger.Log != nil {
		logger.Log.Warn("All servers are down")
	}
	return nil, fmt.Errorf("all servers are down")
}

func (lb *RoundRobin) Handler(w http.ResponseWriter, r *http.Request) {
	if logger.Log != nil {
		logger.Log.Info("Received request", "method", r.Method, "url", r.URL.String(), "remoteAddr", r.RemoteAddr)
	}
	servers := lb.serverPool.GetServers()
	maxRetries := len(servers)

	for attempt := range maxRetries {
		target, err := lb.NextServer(r)
		if target == nil || err != nil {
			http.Error(w, "No Servers Available", http.StatusServiceUnavailable)
			return
		}
		proxy := httputil.NewSingleHostReverseProxy(target)
		proxyFailed := false

		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			if logger.Log != nil {
				logger.Log.Warn("Server request failed", "attempt", attempt+1, "maxRetries", maxRetries, "target", target.String(), "error", err)
			}
			lb.serverPool.UpdateHealthMap(target.String(), false)
			proxyFailed = true
		}

		proxy.ServeHTTP(w, r)

		if !proxyFailed {
			if logger.Log != nil {
				logger.Log.Info("Successfully proxied request", "target", target.String())
			}
			return
		}
	}

	if logger.Log != nil {
		logger.Log.Error("Retries Exhausted, All servers failed", "maxRetries", maxRetries)
	}
	http.Error(w, "Retries Exhausted, All servers failed", http.StatusBadGateway)
}
