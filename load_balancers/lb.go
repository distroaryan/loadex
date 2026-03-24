package lb

import (
	"net/http"
	"net/url"

	pool "github.com/distroaryan/golb/server_pool"
)

type LB interface {
	NextServer(r *http.Request) (*url.URL, error)
}

type LoadBalancer struct {
	RounRobin          *RoundRobin
	LeastConnection    *LeastConnection
	WeightedRoundRobin *WeightedRoundRobin
	IPHash             *IPHash
}

func NewLoadBalancer(serverPool *pool.ServerPool) *LoadBalancer {
	return &LoadBalancer{
		RounRobin:          NewRoundRobin(serverPool),
		LeastConnection:    NewLeastConnection(serverPool),
		WeightedRoundRobin: NewWeightedRoundRobin(serverPool),
		IPHash:             NewIPHash(serverPool),
	}
}
