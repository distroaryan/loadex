package golb

import (
	"net/http"
	"net/url"

	roundrobin "github.com/distroaryan/golb/load_balancers/round_robin"
)

type LoadBalancer interface {
	NextServer() *url.URL
	Handler(w http.ResponseWriter, r *http.Request)
	UpdateHealth(serverURL string, status bool)
}

func NewLoadBalancer(strategy string, servers []*url.URL) LoadBalancer {
	switch strategy {
	case "rr":
		return roundrobin.NewRoundRobin(servers)
	}
	return nil 
}