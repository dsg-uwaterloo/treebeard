package client

import (
	"context"

	"github.com/rs/zerolog/log"
	"golang.org/x/sync/semaphore"
)

// RateLimit is a simple rate limiter that allows a maximum of tokensLimit requests at a time.
type RateLimit struct {
	sem *semaphore.Weighted
}

func NewRateLimit(tokensLimit int) *RateLimit {
	return &RateLimit{
		sem: semaphore.NewWeighted(int64(tokensLimit)),
	}
}

// Aquire blocks until a token is available.
func (r *RateLimit) Acquire() {
	r.sem.Acquire(context.Background(), 1)
	log.Debug().Msgf("Aquired token from rate limiter")
}

// Release releases a token.
func (r *RateLimit) Release() {
	r.sem.Release(1)
	log.Debug().Msgf("Released token from rate limiter")
}
