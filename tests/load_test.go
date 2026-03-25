package test

// import (
// 	"io"
// 	"math/rand"
// 	"net/http"
// 	"sync"
// 	"sync/atomic"
// 	"testing"
// 	"time"

// 	"github.com/stretchr/testify/assert"
// )

// func TestSustainedTraffic(t *testing.T) {
// 	// 1000 concurrent go routines
// 	// Each go routines make 100 request to the load balancer
// 	lbURL, mockServers, cancel := NewMockLoadBalancer()
// 	defer cancel()

// 	urlHitRate := map[string]int{}

// 	for _, s := range mockServers {
// 		serverURL := s.Server.URL
// 		urlHitRate[serverURL] = 0
// 	}

// 	concurrentRequests := 200

// 	var wg sync.WaitGroup
// 	var mu sync.Mutex

// 	for range concurrentRequests {
// 		wg.Add(1)
// 		go func() {
// 			defer wg.Done()
// 			for range 100 {
// 				serverURL := assertRequestToLoadBalancer(t, lbURL)
// 				mu.Lock()
// 				urlHitRate[serverURL]++
// 				mu.Unlock()
// 			}
// 		}()
// 	}

// 	wg.Wait()

// 	// TOTAL SERVERS = 5
// 	// TOTAL REQUESTS = 1000 * 100
// 	REQUESTS_PER_SERVER := (concurrentRequests * 100) / 5
// 	totalRequests := 0
// 	for _, hitRate := range urlHitRate {
// 		assert.Equal(t, REQUESTS_PER_SERVER, hitRate)
// 		totalRequests += hitRate
// 	}
// 	assert.Equal(t, concurrentRequests*100, totalRequests)
// }

// func TestBurstTraffic(t *testing.T) {
// 	// Tests how the load balancer handles a sudden spike in traffic
// 	// where all requests are released at the exact same time

// 	lbURL, mockServers, cancel := NewMockLoadBalancer()
// 	defer cancel()

// 	urlHitRate := map[string]int{}
// 	for _, s := range mockServers {
// 		urlHitRate[s.Server.URL] = 0
// 	}

// 	concurrentRequests := 500
// 	var wg sync.WaitGroup
// 	var startWg sync.WaitGroup
// 	var mu sync.Mutex 

// 	startWg.Add(1) // Block all go routines untill ready

// 	for range concurrentRequests {
// 		wg.Add(1)
// 		go func(){
// 			defer wg.Done()
// 			startWg.Wait() // Wait 

// 			resp, err := client.Get(lbURL.String())
// 			if err == nil && resp.StatusCode == http.StatusOK {
// 				body, err := io.ReadAll(resp.Body)
// 				resp.Body.Close()
// 				mu.Lock()
// 				if err == nil && len(string(body)) > 0 {
// 					urlHitRate[string(body)]++
// 				}
// 				mu.Unlock()
// 			}
// 		}()
// 	}

// 	startWg.Done() // Release the lock to simulate a spike
// 	wg.Wait()

// 	for _, hits := range urlHitRate {
// 		assert.Equal(t, concurrentRequests / 5, hits)
// 	}
// }

// func TestChaosUnderLoad(t *testing.T) {
// 	// Tests the stability of the load balancer when backend servers
// 	// are going up and down while under the heavy load

// 	lbURL, mockServers, cancel := NewMockLoadBalancer()
// 	defer cancel()

// 	var wg sync.WaitGroup
// 	var activeRequests atomic.Int32
// 	var totalRequestCount atomic.Int32 
// 	duration := 10 * time.Second // Run load for 10 seconds


// 	// 1.  Start background load generator
// 	wg.Add(1)
// 	go func() {
// 		defer wg.Done()
// 		endTime := time.Now().Add(duration)
// 		for time.Now().Before(endTime) {
// 			activeRequests.Add(1)
// 			go func ()  {
// 				defer activeRequests.Add(-1)
// 				resp, err := client.Get(lbURL.String())
// 				if resp.StatusCode == http.StatusOK && err == nil {
// 					body, err := io.ReadAll(resp.Body)
// 					resp.Body.Close()
// 					if err == nil && len(body) > 0 {
// 						// okay request
// 						totalRequestCount.Add(1)
// 					}
// 				}
// 			}()
// 			time.Sleep(5 * time.Millisecond) // Slight delay
// 		}
// 	}()

// 	// 2. Start chaos monkey to randomly bring servers up and down
// 	wg.Add(1)
// 	go func() {
// 		defer wg.Done()
// 		endTime := time.Now().Add(duration)
// 		for time.Now().Before(endTime) {
// 			// Simulate taking a random server down
// 			serverIdx := rand.Intn(NUMBER_OF_SERVERS)
// 			mockServers[serverIdx].Alive.Store(false)
// 			time.Sleep(HEALTH_CHECK_PERIOD + 1 * time.Second)

// 			// Bring it back up
// 			mockServers[serverIdx].Alive.Store(true)
// 			time.Sleep(HEALTH_CHECK_PERIOD + 1 * time.Second)
// 		}
// 	}()

// 	wg.Wait()
// 	// If reached here, then hurrrayyyy
// }