package main

import (
	"fmt"

	"github.com/BurntSushi/toml"
	"github.com/nwah/fujisuite-server/nav"
)

// Config holds the application configuration
type Config struct {
	Port string        `toml:"port"`
	Nav  nav.NavConfig `toml:"nav"`
}

var config Config

// LoadConfig loads the configuration from a TOML file
func LoadConfig(filename string) error {
	if _, err := toml.DecodeFile(filename, &config); err != nil {
		return fmt.Errorf("error decoding config file: %v", err)
	}

	// Validate required fields
	if config.Port == "" {
		config.Port = ":8080" // Default port
	}
	if config.Nav.NominatimURL == "" {
		return fmt.Errorf("nav.nominatim_url is required in config file")
	}
	if config.Nav.ValhallaURL == "" {
		return fmt.Errorf("nav.valhalla_url is required in config file")
	}

	return nil
}

// GetConfig returns the current configuration
func GetConfig() Config {
	return config
}

// GetNavConfig returns the navigation-specific configuration
func GetNavConfig() nav.NavConfig {
	return config.Nav
}
