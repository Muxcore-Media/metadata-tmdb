package internal

import (
	"context"
	"os"
	"strconv"
	"sync"
	"time"
)

type tokenBucket struct {
	mu     sync.Mutex
	rate   float64
	burst  float64
	tokens float64
	last   time.Time
}

func newTokenBucket() *tokenBucket {
	rate := 35.0
	burst := 40.0
	if v := os.Getenv("TMDB_RATE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			rate = f
		}
	}
	if v := os.Getenv("TMDB_BURST"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			burst = f
		}
	}
	return &tokenBucket{
		rate:   rate,
		burst:  burst,
		tokens: burst,
		last:   time.Now(),
	}
}

func (b *tokenBucket) wait(ctx context.Context) error {
	if b == nil || b.rate <= 0 {
		return nil
	}
	for {
		b.mu.Lock()
		now := time.Now()
		elapsed := now.Sub(b.last).Seconds()
		b.tokens += elapsed * b.rate
		if b.tokens > b.burst {
			b.tokens = b.burst
		}
		b.last = now
		if b.tokens >= 1 {
			b.tokens--
			b.mu.Unlock()
			return nil
		}
		need := 1 - b.tokens
		wait := time.Duration(need / b.rate * float64(time.Second))
		if wait < time.Millisecond {
			wait = time.Millisecond
		}
		b.mu.Unlock()

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
