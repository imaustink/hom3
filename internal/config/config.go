// Package config loads HOM3 configuration from CLI flags, environment
// variables, a .env file, or ~/.config/hom3/config.json.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/imaustink/hatui/internal/model"
	"github.com/joho/godotenv"
)

// Load resolves configuration using the precedence order:
//  1. Explicit url/token arguments (CLI flags, passed by caller)
//  2. Environment variables (HASS_URL / HA_URL, HASS_TOKEN / HA_TOKEN)
//  3. .env file in the current working directory
//  4. ~/.config/hom3/config.json
//
// Returns the list of configured homes and the index of the active one.
// url/token override the entire multi-home list to a single home.
func Load(url, token, name string) ([]model.HassConfig, int, error) {
	// Try loading .env (ignore error if file absent)
	_ = godotenv.Load()

	// 1. CLI flags take priority
	if url != "" && token != "" {
		return []model.HassConfig{{Name: displayName(name, url), URL: normalizeURL(url), Token: token}}, 0, nil
	}

	// 2. Environment variables
	envURL := firstEnv("HASS_URL", "HA_URL")
	envToken := firstEnv("HASS_TOKEN", "HA_TOKEN")
	if envURL != "" && envToken != "" {
		return []model.HassConfig{{Name: displayName("", envURL), URL: normalizeURL(envURL), Token: envToken}}, 0, nil
	}

	// 3 / 4. JSON config file
	homes, err := loadConfigFile()
	if err == nil && len(homes) > 0 {
		return homes, 0, nil
	}

	return nil, 0, os.ErrNotExist
}

// loadConfigFile reads ~/.config/hom3/config.json and returns the home list.
func loadConfigFile() ([]model.HassConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, ".config", "hom3", "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var file model.HassConfigFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}

	// Multi-home format
	if len(file.Homes) > 0 {
		for i := range file.Homes {
			file.Homes[i].URL = normalizeURL(file.Homes[i].URL)
			if file.Homes[i].Name == "" {
				file.Homes[i].Name = displayName("", file.Homes[i].URL)
			}
		}
		return file.Homes, nil
	}

	// Legacy single-home format
	if file.URL != "" && file.Token != "" {
		return []model.HassConfig{{
			Name:  displayName("", file.URL),
			URL:   normalizeURL(file.URL),
			Token: file.Token,
		}}, nil
	}

	return nil, os.ErrNotExist
}

// normalizeURL ensures the URL has http:// prefix and no trailing slash.
func normalizeURL(u string) string {
	if !strings.Contains(u, "://") {
		u = "http://" + u
	}
	return strings.TrimRight(u, "/")
}

// displayName returns the name to display for a home: name arg if set,
// otherwise the URL hostname.
func displayName(name, rawURL string) string {
	if name != "" {
		return name
	}
	// Extract hostname for display
	u := normalizeURL(rawURL)
	u = strings.TrimPrefix(u, "http://")
	u = strings.TrimPrefix(u, "https://")
	if idx := strings.IndexByte(u, '/'); idx >= 0 {
		u = u[:idx]
	}
	return u
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}
