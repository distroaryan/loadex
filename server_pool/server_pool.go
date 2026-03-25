package pool

import (
	"slices"
	"sync"
)

type ServerPool struct {
	ServerURLs []string
	HealthMap  map[string]bool
	mu         sync.RWMutex
}

func NewServerPool(serverURLs []string) *ServerPool {
	healthMap := make(map[string]bool)

	for _, url := range serverURLs {
		healthMap[url] = true
	}

	return &ServerPool{
		ServerURLs: serverURLs,
		HealthMap:  healthMap,
	}
}

func (pool *ServerPool) UpdateHealthMap(serverURL string, status bool) {
	pool.mu.Lock()
	pool.HealthMap[serverURL] = status
	pool.mu.Unlock()
}

func (pool *ServerPool) AddServer(serverURL string) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	if slices.Contains(pool.ServerURLs, serverURL) {
		return
	}

	pool.HealthMap[serverURL] = true
	pool.ServerURLs = append(pool.ServerURLs, serverURL)
}

func (pool *ServerPool) GetServers() []string {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	serversCopy := make([]string, len(pool.ServerURLs))
	copy(serversCopy, pool.ServerURLs)
	return serversCopy
}

func (pool *ServerPool) IsAlive(serverURL string) bool {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	return pool.HealthMap[serverURL]
}
