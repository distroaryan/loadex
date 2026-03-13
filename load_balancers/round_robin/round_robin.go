package roundrobin

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
)

type RoundRobin struct {
	Servers        []*url.URL // [localhost:8001, localhost:8002, localhost:8003]
	counter        int64
	healthCheckMap map[string]bool // [localhost:8001] : true
	mu             sync.Mutex
}

func NewRoundRobin(servers []*url.URL) *RoundRobin {
	healthMap := map[string]bool{}
	for _, url := range servers {
		healthMap[url.String()] = true
	}
	return &RoundRobin{
		Servers:        servers,
		counter:        0,
		healthCheckMap: healthMap,
	}
}

func (lb *RoundRobin) NextServer() *url.URL {
	if len(lb.Servers) == 0 {
		return nil
	}

	// Check if all servers are down
	allServersDown := true
	for _, url := range lb.Servers {
		if lb.healthCheckMap[url.String()] {
			allServersDown = false
			break
		}
	}

	if allServersDown {
		log.Println("All the servers are down:")
		return nil
	}

	var nextServerURL *url.URL

	for {
		totalServers := len(lb.Servers)
		idx := atomic.AddInt64(&lb.counter, 1)
		// -1 bcoz initially the idx = 1, but we redirect to 0th index url
		nextServerURL = lb.Servers[(idx-1)%int64(totalServers)]
		if lb.healthCheckMap[nextServerURL.String()] {
			break
		}
	}

	return nextServerURL
}

func (lb *RoundRobin) Handler(w http.ResponseWriter, r *http.Request) {
	maxRetries := len(lb.Servers)

	for attempt := range 1 {
		target := lb.NextServer()
		if target == nil {
			http.Error(w, "No Servers Available", http.StatusServiceUnavailable)
			return 
		}
		proxy := httputil.NewSingleHostReverseProxy(target)
		proxyFailed := false

		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("[Attempt %d/%d] Server %s failed: %v", attempt+1, maxRetries, target.String(), err)
			lb.UpdateHealth(target.String(), false)
			proxyFailed = true 
		}

		proxy.ServeHTTP(w, r)

		if !proxyFailed {
			return 
		}
	}

	http.Error(w, "Retries Exhausted, All servers failed", http.StatusBadGateway)
}

func (lb *RoundRobin) UpdateHealth(serverURL string, healthy bool) {
	lb.mu.Lock()
	lb.healthCheckMap[serverURL] = healthy
	lb.mu.Unlock()
}

func (lb *RoundRobin) AddServerURL(serverURL *url.URL) {
	lb.mu.Lock()
	lb.healthCheckMap[serverURL.String()] = true 
	lb.Servers = append(lb.Servers, serverURL)
	lb.mu.Unlock()
}
