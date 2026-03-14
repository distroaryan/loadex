package test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSustainedTraffic(t *testing.T) {
	// 1000 concurrent go routines
	// Each go routines make 100 request to the load balancer
	lbURL, mockServers, cancel := NewMockLoadBalancer()
	defer cancel()

	urlHitRate := map[string]int{}

	for _, s := range mockServers {
		serverURL := s.Server.URL 
		urlHitRate[serverURL] = 0 
	}

	concurrentRequests := 1000

	var wg sync.WaitGroup
	var mu sync.Mutex 

	for range concurrentRequests {
		wg.Add(1)
		go func(){
			defer wg.Done()
			for range 100 {
				serverURL := assertRequestToLoadBalancer(t, lbURL)
				mu.Lock()
				urlHitRate[serverURL]++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// TOTAL SERVERS = 5
	// TOTAL REQUESTS = 1000 * 100
	REQUESTS_PER_SERVER := (concurrentRequests * 100) / 5
	for _, hitRate := range urlHitRate {
		assert.Equal(t, REQUESTS_PER_SERVER, hitRate)
	}
}

