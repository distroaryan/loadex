package test

import (
	"io"
	"math/rand/v2"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAllServersDown(t *testing.T) {
	lbURL, mockServers, cancel := NewMockLoadBalancer()
	defer cancel()

	// SHUTDOWN ALL SERVERS
	for _, s := range mockServers {
		s.Server.Close()
	}

	// UPDATE THE HEALTH MAP
	time.Sleep(HEALTH_CHECK_PERIOD + 5)

	_, err := http.Get(lbURL.String())
	assert.Error(t, err)
}

func TestServerGoDownDuringTraffic(t *testing.T) {
	lbURL, mockServers, cancel := NewMockLoadBalancer()
	defer cancel()

	urlHitRate := map[string]int{}
	for _, s := range mockServers {
		serverURL := s.Server.URL
		urlHitRate[serverURL] = 0
	}

	// Randomly kill any 2 servers mid traffic
	// Verify all requests reached
	count := 0
	for range 50 {
		if count < 2 {
			serverIdx := rand.IntN(NUMBER_OF_SERVERS)
			mockServers[serverIdx].Server.Close()
			count++
		}
		resp, err := http.Get(lbURL.String())
		assert.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		urlHitRate[string(body)]++
		resp.Body.Close()
	}

	totalCount := 0
	for url, hitRate := range urlHitRate {
		if url == "" || len(url) == 0 {
			continue
		}
		totalCount += hitRate
	}
	assert.Equal(t, 50, totalCount)
}

func TestConcurrentHealthUpdatesAndRequest(t *testing.T) {
	lbURL, mockServers, cancel := NewMockLoadBalancer()
	defer cancel()

	urlHitRate := map[string]int{}
	for _, s := range mockServers {
		serverURL := s.Server.URL
		urlHitRate[serverURL] = 0
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	// make 100 requests in parallel, 20 request each server
	for range 100 {
		wg.Add(1)
		go func(){
			defer wg.Done()
			resp, err := http.Get(lbURL.String())
			assert.NoError(t, err)
			body, err := io.ReadAll(resp.Body)
			mu.Lock()
			urlHitRate[string(body)]++
			mu.Unlock()
			resp.Body.Close()
		}()
	}

	wg.Wait()

	for _, hitRate := range urlHitRate {
		assert.Equal(t, 20, hitRate)
	}
}
