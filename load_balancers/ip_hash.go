package lb

import (
	"fmt"
	"hash/fnv"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/distroaryan/golb/logger"
	"github.com/distroaryan/golb/observability"
	pool "github.com/distroaryan/golb/server_pool"
)

type IPHash struct {
	serverPool *pool.ServerPool
}

func NewIPHash(serverPool *pool.ServerPool) *IPHash {
	return &IPHash{
		serverPool: serverPool,
	}
}

func calculateHash(r *http.Request) (uint32) {
	srcIPPort := r.RemoteAddr
	srcIP, _, err := net.SplitHostPort(srcIPPort)
	if err != nil {
		srcIP = srcIPPort
	}

	var destIPPort string
	if localAddr := r.Context().Value(http.LocalAddrContextKey); localAddr != nil {
		destIPPort = localAddr.(net.Addr).String()
	} else {
		destIPPort = r.Host
	}
	hashKey := fmt.Sprintf("%s-%s", srcIP, destIPPort)

	h := fnv.New32a()
	h.Write([]byte(hashKey))
	hashValue := h.Sum32()
	return hashValue
}

func (lb *IPHash) NextServer(r *http.Request) (*url.URL, error) {
	servers := lb.serverPool.GetServers()
	if len(servers) == 0 {
		return nil, fmt.Errorf("no Servers found")
	}

	healthyServersURL := []string{}
	for _, srv := range servers {
		if lb.serverPool.IsAlive(srv) {
			healthyServersURL = append(healthyServersURL, srv)
		}
	}
	if len(healthyServersURL) == 0 {
		if logger.Log != nil {
			logger.Log.Warn("All servers are down")
		}
		return nil, fmt.Errorf("all servers are down")
	}

	hashValue := calculateHash(r)
	serverIdx := int(hashValue) % len(healthyServersURL)
	selectedServerURL := healthyServersURL[serverIdx]
	if logger.Log != nil {
		logger.Log.Debug("Selected server via IP Hash", "server", selectedServerURL, "hashValue", hashValue)
	}
	return url.Parse(selectedServerURL)
}

func (lb *IPHash) Handler(w http.ResponseWriter, r *http.Request) {
	if logger.Log != nil {
		logger.Log.Info("Received request", "method", r.Method, "url", r.URL.String(), "remoteAddr", r.RemoteAddr)
	}
	target, err := lb.NextServer(r)
	if err != nil {
		http.Error(w, "No servers available", http.StatusServiceUnavailable)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxyFailed := false

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		if logger.Log != nil {
			logger.Log.Warn("Server request failed", "target", target.String(), "error", err)
		}
		lb.serverPool.UpdateHealthMap(target.String(), false)
		proxyFailed = true
	}

	rw := &observability.ResponseWriterRecorder{ResponseWriter: w, StatusCode: http.StatusOK}
	start := time.Now()

	proxy.ServeHTTP(rw, r)

	observability.RecordMetrics(target.String(), r.Method, start, rw.StatusCode)

	if proxyFailed {
		http.Error(w, "Assigned server is currently unavailable", http.StatusBadGateway)
	}
}
