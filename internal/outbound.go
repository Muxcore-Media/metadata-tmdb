package internal

import (
	"net/http"
	"strings"
	"time"

	"github.com/Muxcore-Media/core/sdk/go/module/netguard"
)

// tmdbGuardOptions is the Integration profile for the TMDB API. The default
// host is public. A household may point TMDB_BASE_URL at a LAN fixture, so
// private and loopback are allowed. Link-local, cloud metadata, and non-HTTP
// schemes stay refused.
func tmdbGuardOptions(timeout time.Duration) netguard.Options {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return netguard.Options{
		AllowPrivate:  true,
		AllowLoopback: true,
		Timeout:       timeout,
	}
}

func newGuardedClient(timeout time.Duration) *http.Client {
	return netguard.NewClient(netguard.Integration, tmdbGuardOptions(timeout))
}

// guardOutboundURL checks raw before a request. An empty URL is left to the
// caller (unset configuration).
func guardOutboundURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return netguard.ValidateURL(raw, netguard.Integration, tmdbGuardOptions(15*time.Second))
}
