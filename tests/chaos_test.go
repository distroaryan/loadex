package test

import (
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestChaos_ServerUpAndDown(t *testing.T) {
	tests := []struct {
		lb *MockLoadBalancer
	}{
		{
			lb: NewMockLoadBalancer(ROUND_ROBIN),
		},
		{
			lb: NewMockLoadBalancer(WEIGHTED_ROUND_ROBIN),
		},
		{
			lb: NewMockLoadBalancer(LEAST_CONNECTION),
		},
		{
			lb: NewMockLoadBalancer(IP_HASH),
		},
	}

	for _,tt := range tests {
		t.Run("", func(t *testing.T) {
			lb := tt.lb
			defer lb.Close()
		
			serverHit := map[string]int{}
			var wg sync.WaitGroup
			var start sync.WaitGroup
			var mu sync.Mutex 
		
			start.Add(1)
		
			// Start 50 concurrent requests
			for range 50 {
				wg.Add(1)
				go func(){
					defer wg.Done()
					start.Wait()
					serverURL := assertRequestToLoadBalancer(t, lb)
					mu.Lock()
					serverHit[serverURL]++
					mu.Unlock()
				}()
			}
		
		
			// Start a chaos monkey which flips server up and down
			wg.Add(1)
			go func(){
				defer wg.Done()
				start.Wait()
				// flip the server for 2 seconds with 10 milliseond internal 
				end := time.Now().Add(2 * time.Second)
		
				for time.Now().Before(end) {
					servers := lb.MockServers 
					serverIdx := rand.IntN(len(servers))
					servers[serverIdx].Alive.Store(false)
		
					time.Sleep(5 * time.Millisecond)
		
					servers[serverIdx].Alive.Store(true)
				}
			}()
		
			// release the requests
			start.Done()
			wg.Wait()
		
			totalRequests := 0
			for _, hit := range serverHit {
				totalRequests += hit 
			}
			assert.Equal(t, 50, totalRequests)
		})
	} 
}
