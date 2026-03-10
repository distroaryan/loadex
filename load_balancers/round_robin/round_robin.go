package roundrobin

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
)

type RoundRobin struct {
	Servers []*url.URL // [localhost:8001, localhost:8002, localhost:8003]
	counter int64 
}

func NewRoundRobin(servers []*url.URL) *RoundRobin {
	return &RoundRobin{
		Servers: servers,
		counter: 0,
	}
}

func (lb *RoundRobin) NextServer() *url.URL {
	if len(lb.Servers) == 0 {
		return nil 
	}

	totalServers := len(lb.Servers)
	idx := atomic.AddInt64(&lb.counter, 1)
	// -1 bcoz initially the idx = 1, but we redirect to 0th index url
	nextServerURL := lb.Servers[(idx-1) % int64(totalServers)] 
	return nextServerURL
}

func (lb *RoundRobin) Handler(w http.ResponseWriter, r *http.Request) {
	target := lb.NextServer()
	if target == nil {
		http.Error(w, "No servers available", http.StatusServiceUnavailable)
	}
	log.Printf("Routing Request to the server with URL: %s", target.String())
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ServeHTTP(w, r)
}