package internal

import (
	"fmt"
	"strings"

	"github.com/Muxcore-Media/core/pkg/contracts"
	modulesdk "github.com/Muxcore-Media/core/sdk/go/module"
	"google.golang.org/grpc"
)

func (m *Module) settingsDefs() []contracts.SettingDef {
	m.mu.RLock()
	key := m.apiKey
	base := m.baseURL
	country := m.certCountry
	m.mu.RUnlock()
	return []contracts.SettingDef{
		{
			Key:         "api_key",
			Label:       "TMDB API Key",
			Type:        contracts.SettingTypeSecret,
			Default:     "",
			Value:       modulesdk.MaskSecret(key),
			Description: "API key from themoviedb.org",
			Required:    true,
			Group:       "Connection",
		},
		{
			Key:         "base_url",
			Label:       "TMDB Base URL",
			Type:        contracts.SettingTypeString,
			Default:     "https://api.themoviedb.org",
			Value:       base,
			Description: "TMDB API base URL",
			Required:    false,
			Group:       "Connection",
		},
		{
			Key:         "certification_country",
			Label:       "Certification country",
			Type:        contracts.SettingTypeString,
			Default:     defaultCertificationCountry,
			Value:       country,
			Description: "ISO 3166-1 alpha-2 country whose raw TMDB certification is returned on movie/TV details (e.g. US, GB, DE). Invalid values are rejected; an invalid TMDB_CERTIFICATION_COUNTRY at startup disables certification (shown empty).",
			Required:    false,
			Group:       "Content ratings",
		},
	}
}

func (m *Module) updateSetting(key, value string) error {
	switch key {
	case "api_key", "TMDB_API_KEY", "MUXCORE_CFG_TMDB_API_KEY":
		if value == "********" {
			return nil
		}
		m.mu.Lock()
		m.apiKey = strings.TrimSpace(value)
		m.mu.Unlock()
		m.cache.clear()
		return nil
	case "base_url", "TMDB_BASE_URL":
		value = strings.TrimRight(strings.TrimSpace(value), "/")
		if value == "" {
			value = "https://api.themoviedb.org"
		}
		m.mu.Lock()
		m.baseURL = value
		m.mu.Unlock()
		m.cache.clear()
		return nil
	case "certification_country", "TMDB_CERTIFICATION_COUNTRY":
		// Invalid values are rejected and the current country is kept. Empty
		// resets to the default. No cache clear: the cache holds raw TMDB bodies
		// with every country's certifications, and the country is applied per
		// request after the cache, so entries can never mix countries.
		country, err := parseCertificationCountry(value)
		if err != nil {
			return err
		}
		m.mu.Lock()
		m.certCountry = country
		m.mu.Unlock()
		return nil
	default:
		return fmt.Errorf("unknown setting %q", key)
	}
}

func (m *Module) Settings() []contracts.SettingDef {
	return m.settingsDefs()
}

func (m *Module) UpdateSetting(key, value string) error {
	return m.updateSetting(key, value)
}

func (m *Module) registerSettingsMesh(srv *grpc.Server) {
	modulesdk.RegisterSettings(srv, m.id, m)
}
