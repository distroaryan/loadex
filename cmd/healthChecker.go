package main

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/distroaryan/golb"
)

type HealthChecker struct {
	interval   time.Duration
	serversURL []*url.URL
	lb         golb.LoadBalancer
}

func NewHealthChecker(interval time.Duration, servers []*url.URL, lb golb.LoadBalancer) *HealthChecker {
	return &HealthChecker{
		interval:   interval,
		serversURL: servers,
		lb:  lb,
	}
}

func (hc *HealthChecker) Start(ctx context.Context) {
	ticker := time.NewTicker(hc.interval)
	go func() {
		defer func() {
			ticker.Stop()
			log.Println("Health Checker stopped")
		}()
		for {
			select {
			case <-ticker.C:
				if err := hc.updateHealthMap(); err != nil {
					log.Printf("Health check error: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (hc *HealthChecker) updateHealthMap() error {
	for _, url := range hc.serversURL {
		go func() {
			healthCheck := pingServer(url)
			hc.lb.UpdateHealth(url.String(), healthCheck)
		}()
	}
	return nil
}

func pingServer(url *url.URL) bool {
	resp, err := http.Get(url.String() + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
