package internal

import (
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type cacheEntry struct {
	body      []byte
	expiresAt time.Time
}

type httpCache struct {
	mu         sync.Mutex
	entries    map[string]cacheEntry
	max        int
	ttlConfig  time.Duration
	ttlDetails time.Duration
	ttlSearch  time.Duration
	ttlList    time.Duration
	ttlDefault time.Duration
}

func newHTTPCache() *httpCache {
	c := &httpCache{
		entries:    make(map[string]cacheEntry),
		max:        1024,
		ttlConfig:  24 * time.Hour,
		ttlDetails: 6 * time.Hour,
		ttlSearch:  15 * time.Minute,
		ttlList:    time.Hour,
		ttlDefault: 30 * time.Minute,
	}
	if v := os.Getenv("TMDB_CACHE_MAX"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			c.max = n
		}
	}
	if v := os.Getenv("TMDB_CACHE_TTL_DETAILS"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			c.ttlDetails = d
		}
	}
	if v := os.Getenv("TMDB_CACHE_TTL_SEARCH"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			c.ttlSearch = d
		}
	}
	if v := os.Getenv("TMDB_CACHE_TTL_LIST"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			c.ttlList = d
		}
	}
	return c
}

func cacheKey(endpoint string, params url.Values) string {
	if len(params) == 0 {
		return endpoint
	}
	filtered := make(url.Values, len(params))
	for k, vals := range params {
		if k == "api_key" {
			continue
		}
		filtered[k] = append([]string(nil), vals...)
	}
	enc := filtered.Encode()
	if enc == "" {
		return endpoint
	}
	return endpoint + "?" + enc
}

func (c *httpCache) ttlFor(endpoint string) time.Duration {
	switch {
	case endpoint == "/3/configuration":
		return c.ttlConfig
	case strings.HasPrefix(endpoint, "/3/search/") || strings.HasPrefix(endpoint, "/3/find/"):
		return c.ttlSearch
	case strings.HasPrefix(endpoint, "/3/trending/") ||
		endpoint == "/3/movie/popular" ||
		endpoint == "/3/tv/popular":
		return c.ttlList
	case strings.HasPrefix(endpoint, "/3/movie/") ||
		strings.HasPrefix(endpoint, "/3/tv/") ||
		strings.HasPrefix(endpoint, "/3/collection/"):
		return c.ttlDetails
	default:
		return c.ttlDefault
	}
}

func (c *httpCache) get(key string) ([]byte, bool) {
	if c == nil || c.max == 0 {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(e.expiresAt) {
		delete(c.entries, key)
		return nil, false
	}
	out := make([]byte, len(e.body))
	copy(out, e.body)
	return out, true
}

func (c *httpCache) set(key string, body []byte, ttl time.Duration) {
	if c == nil || c.max == 0 || ttl <= 0 || len(body) == 0 {
		return
	}
	stored := make([]byte, len(body))
	copy(stored, body)

	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= c.max {
		if _, ok := c.entries[key]; !ok {
			c.evictOldestLocked()
		}
	}
	c.entries[key] = cacheEntry{
		body:      stored,
		expiresAt: time.Now().Add(ttl),
	}
}

func (c *httpCache) evictOldestLocked() {
	var oldestKey string
	var oldestTime time.Time
	first := true
	for k, e := range c.entries {
		if first || e.expiresAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = e.expiresAt
			first = false
		}
	}
	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}
