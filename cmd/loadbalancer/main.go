package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/distroaryan/golb/health_checker"
	lb "github.com/distroaryan/golb/load_balancers"
	"github.com/distroaryan/golb/logger"
	pool "github.com/distroaryan/golb/server_pool"
)

var (
	port        int
	backendsStr string
	algorithm   string

	serverPool *pool.ServerPool
)

func handleHealth(w http.ResponseWriter, r *http.Request) {
	if serverPool == nil {
		http.Error(w, "Server pool not initialized", http.StatusInternalServerError)
		return
	}
	servers := serverPool.GetServers()
	healthMap := make(map[string]bool)
	for _, srv := range servers {
		healthMap[srv] = serverPool.IsAlive(srv)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(healthMap)
}

func handleKill(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	target := r.URL.Query().Get("url")
	if target == "" {
		http.Error(w, "Missing url parameter", http.StatusBadRequest)
		return
	}
	serverPool.UpdateHealthMap(target, false)
	fmt.Fprintf(w, "Marked %s as dead\n", target)
	logger.Log.Info("Manual kill-server executed", "url", target)
}

func handleAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	target := r.URL.Query().Get("url")
	if target == "" {
		http.Error(w, "Missing url parameter", http.StatusBadRequest)
		return
	}
	serverPool.AddServer(target)
	fmt.Fprintf(w, "Added %s to the pool\n", target)
	logger.Log.Info("Manual add-server executed", "url", target)
}

func main() {
	flag.IntVar(&port, "port", 8080, "Port to run the load balancer on")
	flag.StringVar(&backendsStr, "backends", "", "Comma-separated list of backend URLs")
	flag.StringVar(&algorithm, "algo", "roundrobin", "Load balancing algorithm (roundrobin, weightedroundrobin, leastconnection, iphash)")
	flag.Parse()

	logger.InitLogger()
	logger.Log.Info("Starting loadbalancer server...", "version", "1.0.0")

	if backendsStr == "" {
		logger.Log.Error("No backend servers provided. Use --backends flag.")
		os.Exit(1)
	}

	backends := strings.Split(backendsStr, ",")
	for i := range backends {
		backends[i] = strings.TrimSpace(backends[i])
	}
	logger.Log.Info("Parsed backend servers", "count", len(backends), "servers", backends)

	serverPool = pool.NewServerPool(backends)

	var baseHandlerFunc http.HandlerFunc
	golbRouter := lb.NewLoadBalancer(serverPool)

	switch strings.ToLower(algorithm) {
	case "roundrobin", "rr":
		baseHandlerFunc = golbRouter.RounRobin.Handler
	case "weightedroundrobin", "wrr":
		baseHandlerFunc = golbRouter.WeightedRoundRobin.Handler
	case "leastconnection", "lc":
		baseHandlerFunc = golbRouter.LeastConnection.Handler
	case "iphash", "ip", "urlhash":
		baseHandlerFunc = golbRouter.IPHash.Handler
	default:
		logger.Log.Error("Unknown algorithm selected", "algorithm", algorithm)
		os.Exit(1)
	}
	logger.Log.Info("Configured load balancer", "algorithm", algorithm)

	proxyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		baseHandlerFunc(w, r)
	})

	hc := health_checker.NewHealthChecker(serverPool)
	ctx, stopHC := context.WithCancel(context.Background())
	go hc.Start(ctx, 5*time.Second)

	mux := http.NewServeMux()

	// System Endpoints
	mux.HandleFunc("/api/health", handleHealth)
	mux.HandleFunc("/api/kill", handleKill)
	mux.HandleFunc("/api/add", handleAdd)
	
	// Fallback proxy
	mux.HandleFunc("/", proxyHandler)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		logger.Log.Info("Loadbalancer server listening", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Error("Server crashed", "error", err)
		}
	}()

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)
	<-stopChan
	logger.Log.Info("Shutting down loadbalancer server gracefully...")

	stopHC()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	srv.Shutdown(shutdownCtx)
	logger.Log.Info("Shutdown complete")
}
