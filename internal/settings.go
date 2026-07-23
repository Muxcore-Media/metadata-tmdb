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
		return nil
	case "base_url", "TMDB_BASE_URL":
		value = strings.TrimRight(strings.TrimSpace(value), "/")
		if value == "" {
			value = "https://api.themoviedb.org"
		}
		m.mu.Lock()
		m.baseURL = value
		m.mu.Unlock()
		return nil
	default:
		return fmt.Errorf("unknown setting %q", key)
	}
}

func (m *Module) registerSettingsMesh(srv *grpc.Server) {
	modulesdk.RegisterMeshHandler(srv, m.id, modulesdk.SettingsHandler{
		List:   m.settingsDefs,
		Update: m.updateSetting,
	})
}
