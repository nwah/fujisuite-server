package main

import (
	"log"
	"net/http"

	"github.com/nwah/fujisuite-server/nav"
)

func main() {
	// Load configuration
	if err := LoadConfig("config.toml"); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Set nav config for the nav package
	nav.SetConfig(GetNavConfig())

	// Register handlers under /nav path
	http.HandleFunc("/nav/geocode", nav.HandleGeocode)
	http.HandleFunc("/nav/route", nav.HandleRoute)

	// Start server
	config := GetConfig()
	log.Printf("Starting server on port %s", config.Port)
	if err := http.ListenAndServe(config.Port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
