package nav

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

var navConfig NavConfig

// SetConfig sets the navigation configuration
func SetConfig(cfg NavConfig) {
	navConfig = cfg
}

// Helper functions for formatting
func formatDuration(seconds float64) string {
	hours := int(seconds / 3600)
	minutes := int((seconds - float64(hours*3600)) / 60)

	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%dhr %dmin", hours, minutes)
		}
		return fmt.Sprintf("%dhr", hours)
	}
	return fmt.Sprintf("%dmin", minutes)
}

func formatDistance(distance float64, units DistanceUnit) string {
	if units == UnitMiles {
		if distance < 0.1 {
			feet := distance * 5280
			return fmt.Sprintf("%.0fft", feet)
		}
		return fmt.Sprintf("%.1fmi", distance)
	}
	// For kilometers
	if distance < 1.0 {
		return fmt.Sprintf("%.0fm", distance*1000)
	}
	return fmt.Sprintf("%.1fkm", distance)
}

func writePlainTextRoute(w http.ResponseWriter, result *RouteResponse) {
	w.Header().Set("Content-Type", "text/plain")

	// Write duration and distance
	fmt.Fprintf(w, "%s\n", formatDuration(result.Duration))
	fmt.Fprintf(w, "%s\n", formatDistance(result.Distance, result.Units))
	fmt.Fprintf(w, "%d\n", len(result.Steps))

	// Write steps
	for i, step := range result.Steps {
		// Write icon on its own line
		fmt.Fprintf(w, "%s\n", step.Icon)

		// For non-transit modes, append the distance in parentheses
		if result.Mode != ModeTransit && i < len(result.Steps)-1 {
			fmt.Fprintf(w, "%s (%s)\n", step.Description, formatDistance(step.Distance, result.Units))
		} else {
			fmt.Fprintf(w, "%s\n", step.Description)
		}
	}
}

func writeError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func parseLatLng(s string) (float64, float64, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid lat,lng format")
	}

	lat, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid latitude: %v", err)
	}

	lng, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid longitude: %v", err)
	}

	return lat, lng, nil
}

// HandleGeocode handles the /nav/geocode endpoint
func HandleGeocode(w http.ResponseWriter, r *http.Request) {
	// Log request URL and method
	log.Printf("Debug: Geocode %s request to %s", r.Method, r.URL.String())

	switch r.Method {
	case http.MethodGet:
		query := r.URL.Query().Get("q")
		if query == "" {
			writeError(w, http.StatusBadRequest, "query parameter 'q' is required")
			return
		}

		// Log query parameter
		log.Printf("Debug: Geocode query: %q", query)

		results, err := geocode(query)
		if err != nil {
			if _, ok := err.(*ErrNoResults); ok {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Log number of results
		log.Printf("Debug: Geocode found %d results", len(results))

		writeJSON(w, results)

	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to read request body")
			return
		}
		defer r.Body.Close()

		query := strings.TrimSpace(string(body))
		log.Printf(query)
		if query == "" {
			writeError(w, http.StatusBadRequest, "request body cannot be empty")
			return
		}

		results, err := geocode(query)
		if err != nil {
			if _, ok := err.(*ErrNoResults); ok {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Log number of results
		log.Printf("Debug: Geocode found %d results", len(results))

		// Return plain text format for POST requests
		w.Header().Set("Content-Type", "text/plain")
		// First line is the number of results
		fmt.Fprintf(w, "%d\n", len(results))
		// Output each result as 4 consecutive lines
		for _, result := range results {
			fmt.Fprintf(w, "%.4f,%.4f\n%s\n%s\n%s\n", result.Lat, result.Lng, result.Name, result.Address, result.Country)
		}

	default:
		writeError(w, http.StatusMethodNotAllowed, "only GET and POST methods are allowed")
	}
}

// HandleRoute handles the /nav/route endpoint
func HandleRoute(w http.ResponseWriter, r *http.Request) {
	// Log request URL and method
	log.Printf("Debug: Route %s request to %s", r.Method, r.URL.String())

	switch r.Method {
	case http.MethodGet:
		// Parse parameters
		from := r.URL.Query().Get("from")
		to := r.URL.Query().Get("to")
		mode := r.URL.Query().Get("mode")
		units := r.URL.Query().Get("units")
		country := strings.ToLower(r.URL.Query().Get("country"))
		fromDesc := r.URL.Query().Get("fromDesc")
		toDesc := r.URL.Query().Get("toDesc")

		// Log query parameters
		log.Printf("Debug: Route parameters - from=%q, to=%q, mode=%q, units=%q, country=%q, fromDesc=%q, toDesc=%q",
			from, to, mode, units, country, fromDesc, toDesc)

		if from == "" || to == "" {
			writeError(w, http.StatusBadRequest, "both 'from' and 'to' parameters are required")
			return
		}

		// Validate country code if provided
		var countryCode CountryCode
		if country != "" {
			countryCode = CountryCode(country)
			if !countryCode.IsValid() {
				writeError(w, http.StatusBadRequest, "country must be a valid 2-letter ISO code in lowercase")
				return
			}
		}

		// Validate mode
		var transportMode TransportMode
		if mode == "" {
			transportMode = DefaultMode
		} else {
			transportMode = TransportMode(strings.ToLower(mode))
			if !transportMode.IsValid() {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid mode. Must be one of: %s, %s, %s, %s",
					ModeWalking, ModeBiking, ModeAuto, ModeTransit))
				return
			}
		}

		// Validate units
		var distanceUnit DistanceUnit
		if units == "" {
			distanceUnit = DefaultUnit
		} else {
			distanceUnit = DistanceUnit(strings.ToLower(units))
			if !distanceUnit.IsValid() {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid units. Must be one of: %s, %s",
					UnitKilometers, UnitMiles))
				return
			}
		}

		// Parse coordinates
		fromLat, fromLng, err := parseLatLng(from)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid 'from' parameter: %v", err))
			return
		}

		toLat, toLng, err := parseLatLng(to)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid 'to' parameter: %v", err))
			return
		}

		handleRouteRequest(w, r.Method, fromLat, fromLng, toLat, toLng, transportMode, distanceUnit, countryCode, fromDesc, toDesc)

	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintf(w, "\n\n0\nfailed to read request body\n")
			return
		}
		defer r.Body.Close()

		// Log request body
		log.Printf("Debug: Route POST body: %s", string(body))

		// Split the body into lines
		lines := strings.Split(strings.TrimSpace(string(body)), "\n")
		if len(lines) < 5 {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintf(w, "\n\n0\nrequest must contain at least 5 lines\n")
			return
		}

		// Clean up any \r from \r\n line endings
		mode := strings.TrimSpace(strings.TrimRight(lines[0], "\r"))
		country := strings.TrimSpace(strings.TrimRight(lines[1], "\r"))
		units := strings.TrimSpace(strings.TrimRight(lines[2], "\r"))
		from := strings.TrimSpace(strings.TrimRight(lines[3], "\r"))
		to := strings.TrimSpace(strings.TrimRight(lines[4], "\r"))

		// Validate and convert mode and units
		transportMode := TransportMode(strings.ToLower(mode))
		if !transportMode.IsValid() {
			transportMode = DefaultMode
		}
		distanceUnit := DistanceUnit(strings.ToLower(units))
		if !distanceUnit.IsValid() {
			distanceUnit = DefaultUnit
		}
		countryCode := CountryCode(strings.ToLower(country))
		if !countryCode.IsValid() {
			countryCode = CountryCode("us")
		}

		// Get optional descriptions if provided
		var fromDesc, toDesc string
		if len(lines) > 5 {
			fromDesc = strings.TrimSpace(strings.TrimRight(lines[5], "\r"))
		}
		if len(lines) > 6 {
			toDesc = strings.TrimSpace(strings.TrimRight(lines[6], "\r"))
		}

		// Parse coordinates
		fromLat, fromLng, err := parseLatLng(from)
		if err != nil {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintf(w, "\n\n0\ninvalid 'from' coordinates\n")
			return
		}

		toLat, toLng, err := parseLatLng(to)
		if err != nil {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintf(w, "\n\n0\ninvalid 'to' coordinates\n")
			return
		}

		// Handle the route request
		result, err := route(RouteRequest{
			FromLat:  fromLat,
			FromLng:  fromLng,
			ToLat:    toLat,
			ToLng:    toLng,
			FromDesc: fromDesc,
			ToDesc:   toDesc,
			Mode:     transportMode,
			Units:    distanceUnit,
			Country:  countryCode,
		})
		if err != nil {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintf(w, "\n\n0\n%s\n", err.Error())
			return
		}

		// Write plain text response
		writePlainTextRoute(w, result)

	default:
		writeError(w, http.StatusMethodNotAllowed, "only GET and POST methods are allowed")
	}
}

// handleRouteRequest handles the common routing logic for both GET and POST requests
func handleRouteRequest(w http.ResponseWriter, method string, fromLat, fromLng, toLat, toLng float64, mode TransportMode, units DistanceUnit, country CountryCode, fromDesc, toDesc string) {
	// Create route request
	req := RouteRequest{
		FromLat:  fromLat,
		FromLng:  fromLng,
		ToLat:    toLat,
		ToLng:    toLng,
		FromDesc: fromDesc,
		ToDesc:   toDesc,
		Mode:     mode,
		Units:    units,
		Country:  country,
	}

	// Get route
	result, err := route(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// For POST requests, return plain text format
	if method == http.MethodPost {
		writePlainTextRoute(w, result)
		return
	}

	// For GET requests, return JSON format
	writeJSON(w, result)
}
