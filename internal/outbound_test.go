package internal

import (
	"errors"
	"testing"

	"github.com/Muxcore-Media/core/sdk/go/module/netguard"
)

func TestGuardOutboundURL(t *testing.T) {
	if err := guardOutboundURL(""); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		"https://api.themoviedb.org/3/configuration",
		"http://127.0.0.1:9/3/search/movie",
		"http://tmdb:8080/3/movie/550",
	} {
		if err := guardOutboundURL(raw); err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
	}
	for _, raw := range []string{
		"http://169.254.169.254/latest/meta-data/",
		"http://metadata.google.internal/",
		"file:///etc/passwd",
	} {
		if err := guardOutboundURL(raw); !errors.Is(err, netguard.ErrBlocked) {
			t.Fatalf("%s: err=%v", raw, err)
		}
	}
}

func TestUpdateSettingRejectsMetadataBaseURL(t *testing.T) {
	m := NewModule(Config{})
	err := m.updateSetting("base_url", "http://169.254.169.254")
	if !errors.Is(err, netguard.ErrBlocked) {
		t.Fatalf("err=%v", err)
	}
	if m.baseURL == "http://169.254.169.254" {
		t.Fatal("stored metadata base")
	}
	if err := m.updateSetting("base_url", "http://127.0.0.1:8080"); err != nil {
		t.Fatal(err)
	}
}
