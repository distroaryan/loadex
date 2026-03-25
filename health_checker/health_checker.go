package health_checker

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/distroaryan/golb/logger"
	pool "github.com/distroaryan/golb/server_pool"
)

type HealthChecker struct {
	serverPool *pool.ServerPool
}

func NewHealthChecker(serverPool *pool.ServerPool) *HealthChecker {
	return &HealthChecker{
		serverPool: serverPool,
	}
}

func (hc *HealthChecker) checkHealth(serverURL string) bool {
	u, err := url.Parse(serverURL)
	if err != nil {
		return false
	}

	healthEndpoint := u.Scheme + "://" + u.Host + "/health"
	client := http.Client{
		Timeout: 2 * time.Second,
	}

	resp, err := client.Get(healthEndpoint)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func (hc *HealthChecker) Start(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if logger.Log != nil {
				logger.Log.Info("Health Checker stopped")
			}
			return
		case <-ticker.C:
			servers := hc.serverPool.GetServers()
			for _, serverURL := range servers {
				alive := hc.checkHealth(serverURL)
				hc.serverPool.UpdateHealthMap(serverURL, alive)
				if logger.Log != nil {
					status := "DOWN"
					if alive {
						status = "UP"
					}
					logger.Log.Debug("Health Check", "server", serverURL, "status", status)
				}
			}
		}
	}
}
