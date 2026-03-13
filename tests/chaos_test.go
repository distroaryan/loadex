package test

import (
	"net/http"
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