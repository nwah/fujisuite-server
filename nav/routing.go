package nav

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Valhalla URL is configured in config.json

type valhallaLocation struct {
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
	Type string  `json:"type"`
}

type valhallaRequest struct {
	Locations      []valhallaLocation     `json:"locations"`
	Costing        string                 `json:"costing"`
	Units          string                 `json:"units"`
	CostingOptions map[string]interface{} `json:"costing_options,omitempty"`
	DateTime       map[string]interface{} `json:"date_time,omitempty"`
}

type valhallaManeuver struct {
	Type        int     `json:"type"`
	Instruction string  `json:"instruction"`
	Distance    float64 `json:"length"`
}

type valhallaLeg struct {
	Maneuvers []valhallaManeuver `json:"maneuvers"`
	Shape     string             `json:"shape"`
}

type valhallaResponse struct {
	Trip struct {
		Legs    []valhallaLeg `json:"legs"`
		Summary struct {
			Time     float64 `json:"time"`
			Distance float64 `json:"length"`
		} `json:"summary"`
	} `json:"trip"`
}

type transitlandRequest struct {
	From     string `json:"from"`     // lat,lon format
	To       string `json:"to"`       // lat,lon format
	Time     string `json:"time"`     // ISO time (optional)
	Date     string `json:"date"`     // YYYY-MM-DD (optional)
	Mode     string `json:"mode"`     // TRANSIT,WALK
	MaxWalk  string `json:"maxWalk"`  // distance in meters
	NumTrips int    `json:"numTrips"` // max number of alternatives
}

type transitlandResponse struct {
	Plan struct {
		Itineraries []struct {
			Duration     float64 `json:"duration"`     // seconds
			WalkTime     float64 `json:"walkTime"`     // seconds
			TransitTime  float64 `json:"transitTime"`  // seconds
			WalkDistance float64 `json:"walkDistance"` // meters
			Legs         []struct {
				Mode     string  `json:"mode"`
				Distance float64 `json:"distance"` // meters
				Duration float64 `json:"duration"` // seconds
				From     struct {
					Name     string `json:"name"`     // station/stop name
					StopId   string `json:"stopId"`   // stop ID
					StopCode string `json:"stopCode"` // stop code
				} `json:"from"`
				To struct {
					Name     string `json:"name"`     // station/stop name
					StopId   string `json:"stopId"`   // stop ID
					StopCode string `json:"stopCode"` // stop code
				} `json:"to"`
				RouteId        string `json:"routeId"`        // route ID
				RouteShortName string `json:"routeShortName"` // route number
				RouteLongName  string `json:"routeLongName"`  // route name
				AgencyName     string `json:"agencyName"`     // transit agency
				LegGeometry    struct {
					Points string `json:"points"` // encoded polyline
				} `json:"legGeometry"`
				IntermediateStops []struct {
					Name      string  `json:"name"`
					StopId    string  `json:"stopId"`
					StopCode  string  `json:"stopCode"`
					Lat       float64 `json:"lat"`
					Lon       float64 `json:"lon"`
					Departure int64   `json:"departure"`
				} `json:"intermediateStops"`
				Steps []struct {
					Distance          float64 `json:"distance"`
					RelativeDirection string  `json:"relativeDirection"`
					StreetName        string  `json:"streetName"`
				} `json:"steps"`
			} `json:"legs"`
		} `json:"itineraries"`
	} `json:"plan"`
}

type transitlandRouteResponse struct {
	Routes []struct {
		ID          string `json:"id"`
		OnestopID   string `json:"onestop_id"`
		Name        string `json:"name"`
		VehicleType string `json:"vehicle_type"`
		ShortName   string `json:"short_name"`
		LongName    string `json:"long_name"`
		Color       string `json:"color"`
		Operator    struct {
			Name string `json:"name"`
		} `json:"operator"`
	} `json:"routes"`
}

const metersPerMile = 1609.344

func getTransportMode(mode TransportMode) string {
	switch mode {
	case ModeWalking:
		return "pedestrian"
	case ModeBiking:
		return "bicycle"
	case ModeTransit:
		return "transit"
	default:
		return "auto"
	}
}

func getValhallaUnits(units DistanceUnit) string {
	if units == UnitMiles {
		return "miles"
	}
	return "kilometers"
}

func convertDistance(meters float64, units DistanceUnit) float64 {
	if units == UnitMiles {
		return meters / metersPerMile
	}
	return meters / 1000 // convert to kilometers
}

func decodePolyline(encoded string) []PathPoint {
	if encoded == "" {
		return []PathPoint{}
	}

	// Use precision of 5 for Valhalla coordinates
	precision := 5
	factor := math.Pow10(precision)

	lat, lng := 0, 0
	var rawPoints [][2]float64
	index := 0

	// First pass: decode all points
	for index < len(encoded) {
		// Consume varint bits for lat until we run out
		var byte int = 0x20
		shift, result := 0, 0
		for byte >= 0x20 {
			byte = int(encoded[index]) - 63
			result |= (byte & 0x1f) << shift
			shift += 5
			index++
		}

		// check if we need to go negative or not
		if (result & 1) > 0 {
			lat += ^(result >> 1)
		} else {
			lat += result >> 1
		}

		// Consume varint bits for lng until we run out
		byte = 0x20
		shift, result = 0, 0
		for byte >= 0x20 {
			byte = int(encoded[index]) - 63
			result |= (byte & 0x1f) << shift
			shift += 5
			index++
		}

		// check if we need to go negative or not
		if (result & 1) > 0 {
			lng += ^(result >> 1)
		} else {
			lng += result >> 1
		}

		// Convert to actual coordinates
		actualLat := float64(lat) / factor
		actualLng := float64(lng) / factor
		rawPoints = append(rawPoints, [2]float64{actualLat, actualLng})
	}

	if len(rawPoints) == 0 {
		return []PathPoint{}
	}

	// Find bounds
	minLat := rawPoints[0][0]
	maxLat := rawPoints[0][0]
	minLng := rawPoints[0][1]
	maxLng := rawPoints[0][1]

	for _, p := range rawPoints[1:] {
		minLat = math.Min(minLat, p[0])
		maxLat = math.Max(maxLat, p[0])
		minLng = math.Min(minLng, p[1])
		maxLng = math.Max(maxLng, p[1])
	}

	// Handle cases where all points are the same
	latRange := maxLat - minLat
	if latRange == 0 {
		latRange = 1 // Avoid division by zero
	}
	lngRange := maxLng - minLng
	if lngRange == 0 {
		lngRange = 1 // Avoid division by zero
	}

	// Second pass: normalize points and remove duplicates and near-duplicates
	var normalizedPoints []PathPoint

	for _, p := range rawPoints {
		// Normalize to 100x100 grid
		x := int(math.Round((p[1] - minLng) / lngRange * float64(NormalizedGridSize)))
		y := int(math.Round((p[0] - minLat) / latRange * float64(NormalizedGridSize)))

		// Ensure points are within bounds
		x = max(0, min(NormalizedGridSize, x))
		y = max(0, min(NormalizedGridSize, y))

		// Check if this point is too close to any existing point
		isDuplicate := false
		for _, existing := range normalizedPoints {
			// Calculate Manhattan distance
			dist := abs(x-existing[0]) + abs(y-existing[1])
			if dist <= 2 { // Points within 2 units of each other
				isDuplicate = true
				break
			}
		}

		if !isDuplicate {
			normalizedPoints = append(normalizedPoints, PathPoint{x, y})
		}
	}

	return normalizedPoints
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// abs returns the absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// Add helper function for US distance formatting
func formatUSDistance(meters float64) string {
	feet := meters * 3.28084
	if feet < 1000 {
		return fmt.Sprintf("%.0f feet", feet)
	}
	miles := feet / 5280
	if miles < 0.1 {
		return fmt.Sprintf("%.0f feet", feet)
	}
	return fmt.Sprintf("%.1f miles", miles)
}

func routeTransitUS(req RouteRequest) (*RouteResponse, error) {
	if navConfig.TransitlandURL == "" || navConfig.TransitlandAPIKey == "" {
		return nil, fmt.Errorf("transitland configuration not complete")
	}

	// Build query parameters
	now := time.Now()
	params := url.Values{
		"api_key":   {navConfig.TransitlandAPIKey},
		"fromPlace": {fmt.Sprintf("%.6f,%.6f", req.FromLat, req.FromLng)},
		"toPlace":   {fmt.Sprintf("%.6f,%.6f", req.ToLat, req.ToLng)},
		"date":      {now.Format("2006-01-02")},
		"time":      {now.Format("15:04")},
	}

	// Create request URL with query parameters
	apiURL := fmt.Sprintf("%s/routing/otp/plan?%s", navConfig.TransitlandURL, params.Encode())
	fmt.Printf("Debug: Making request to %s\n", apiURL)

	// Make GET request
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("error making request to transitland: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("transitland API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Decode response
	var tResp transitlandResponse
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&tResp); err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}

	if len(tResp.Plan.Itineraries) == 0 {
		return nil, fmt.Errorf("no route found")
	}

	// Use the first itinerary
	itinerary := tResp.Plan.Itineraries[0]
	result := &RouteResponse{
		Duration: itinerary.Duration,
		Distance: convertDistance(itinerary.WalkDistance, req.Units), // Convert walk distance to requested units
		Units:    req.Units,
		Mode:     req.Mode,
		From: Location{
			Desc: req.FromDesc,
			Lat:  req.FromLat,
			Lng:  req.FromLng,
		},
		To: Location{
			Desc: req.ToDesc,
			Lat:  req.ToLat,
			Lng:  req.ToLng,
		},
	}

	// Process legs and build path
	var allPoints []PathPoint
	for i, leg := range itinerary.Legs {
		// Create step description based on mode
		var description string
		var icon string
		switch leg.Mode {
		case "WALK":
			if req.Country == "us" {
				description = fmt.Sprintf("Walk %s", formatUSDistance(leg.Distance))
			} else {
				description = fmt.Sprintf("Walk %.0f meters", leg.Distance)
			}
			if leg.To.Name != "" {
				description += fmt.Sprintf(" to %s", leg.To.Name)
			}
			icon = "Walk"
		case "BUS", "RAIL", "SUBWAY", "TRAM", "FERRY":
			description = fmt.Sprintf("Take")
			if leg.RouteShortName != "" {
				description += fmt.Sprintf(" the %s", leg.RouteShortName)
			}
			if leg.RouteLongName != "" {
				description += fmt.Sprintf(" %s", leg.RouteLongName)
			}
			if leg.AgencyName != "" {
				description += fmt.Sprintf(" operated by %s", leg.AgencyName)
			}
			if leg.From.Name != "" && leg.To.Name != "" {
				description += fmt.Sprintf(" from %s to %s", leg.From.Name, leg.To.Name)
			}
			if len(leg.IntermediateStops) > 0 {
				description += fmt.Sprintf(" (%d stops)", len(leg.IntermediateStops))
			}
			icon = getStepIcon(0, "", leg.Mode)
		default:
			if req.Country == "us" {
				description = fmt.Sprintf("%s for %s", leg.Mode, formatUSDistance(leg.Distance))
			} else {
				description = fmt.Sprintf("%s for %.0f meters", leg.Mode, leg.Distance)
			}
			icon = "Straight"
		}

		step := RouteStep{
			Number:      i + 1,
			Description: description,
			Distance:    convertDistance(leg.Distance, req.Units),
			Icon:        icon,
		}
		result.Steps = append(result.Steps, step)

		// Decode and add points from this leg's geometry
		if leg.LegGeometry.Points != "" {
			points := decodePolyline(leg.LegGeometry.Points)
			allPoints = append(allPoints, points...)
		}
	}

	// Set complete path
	result.Path = Path{
		Points: allPoints,
		Length: len(allPoints),
		Width:  NormalizedGridSize,
		Height: NormalizedGridSize,
	}

	return result, nil
}

func getRouteDetails(routeID string) (*transitlandRouteResponse, error) {
	if routeID == "" {
		return nil, fmt.Errorf("route ID is required")
	}

	params := url.Values{
		"api_key": {navConfig.TransitlandAPIKey},
		"ids":     {routeID},
	}

	apiURL := fmt.Sprintf("%s/routes?%s", navConfig.TransitlandURL, params.Encode())
	fmt.Printf("Debug: Fetching route details from %s\n", apiURL)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("error fetching route details: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("route API returned status %d: %s", resp.StatusCode, string(body))
	}

	var routeResp transitlandRouteResponse
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&routeResp); err != nil {
		return nil, fmt.Errorf("error decoding route response: %v", err)
	}

	return &routeResp, nil
}

func getTransportModeName(vehicleType string) string {
	switch strings.ToLower(vehicleType) {
	case "bus":
		return "Bus"
	case "tram":
		return "Tram"
	case "subway", "metro":
		return "Subway"
	case "rail":
		return "Train"
	case "ferry":
		return "Ferry"
	default:
		return strings.Title(strings.ToLower(vehicleType))
	}
}

// Helper function to abbreviate street names in instructions
func abbreviateInstruction(instruction string) string {
	// Replace "You have arrived at your destination." with "Arrive at destination"
	if strings.Contains(instruction, "You have arrived at your destination") {
		return "Arrive at destination"
	}

	// Remove trailing period
	instruction = strings.TrimSuffix(instruction, ".")

	// Abbreviate common words
	instruction = strings.ReplaceAll(instruction, " onto ", " on ")
	instruction = strings.ReplaceAll(instruction, " Avenue", " Ave")
	instruction = strings.ReplaceAll(instruction, " Street", " St")
	instruction = strings.ReplaceAll(instruction, " Road", " Rd")
	instruction = strings.ReplaceAll(instruction, " Boulevard", " Blvd")
	instruction = strings.ReplaceAll(instruction, " Drive", " Dr")
	instruction = strings.ReplaceAll(instruction, " Court", " Ct")
	instruction = strings.ReplaceAll(instruction, " Circle", " Cir")
	instruction = strings.ReplaceAll(instruction, " Highway", " Hwy")
	instruction = strings.ReplaceAll(instruction, " Parkway", " Pkwy")
	instruction = strings.ReplaceAll(instruction, " Place", " Pl")
	instruction = strings.ReplaceAll(instruction, " Square", " Sq")
	instruction = strings.ReplaceAll(instruction, " Terrace", " Ter")
	instruction = strings.ReplaceAll(instruction, " Trail", " Trl")
	instruction = strings.ReplaceAll(instruction, " Turnpike", " Tpke")
	instruction = strings.ReplaceAll(instruction, " Lane", " Ln")
	instruction = strings.ReplaceAll(instruction, " North ", " N ")
	instruction = strings.ReplaceAll(instruction, " South ", " S ")
	instruction = strings.ReplaceAll(instruction, " East ", " E ")
	instruction = strings.ReplaceAll(instruction, " West ", " W ")
	instruction = strings.ReplaceAll(instruction, " Northeast ", " NE ")
	instruction = strings.ReplaceAll(instruction, " Northwest ", " NW ")
	instruction = strings.ReplaceAll(instruction, " Southeast ", " SE ")
	instruction = strings.ReplaceAll(instruction, " Southwest ", " SW ")

	return instruction
}

// getStepIcon determines the appropriate icon based on the maneuver type and mode
func getStepIcon(maneuverType int, instruction string, mode string) string {
	// For transit modes
	switch strings.ToUpper(mode) {
	case "BUS":
		return "Bus"
	case "RAIL", "SUBWAY", "TRAM", "TRAIN":
		return "Train"
	case "FERRY", "BOAT":
		return "Ferry"
	case "WALK":
		return "Walk"
	}

	// For driving/walking/biking modes, check the maneuver type
	switch maneuverType {
	case 2, 10, 11, 12, 1: // Right/Sharp right turn
		return "Right"
	case 3, 13, 14, 15, 19: // Left/Sharp left turn
		return "Left"
	case 9, 23: // Slight right
		return "right"
	case 16, 24: // Slight left
		return "left"
	case 7, 8, 17, 22: // Continue/Bear straight
		return "Straight"
	case 25, 26, 37, 38: // Merge
		return "Merge"
	case 20, 21, 27: // Exit/Ramp
		return "Exit"
	case 28, 29: // Ferry
		return "Ferry"
	case 42, 43:
		return "building"
	default:
		return ""
	}

}

func route(req RouteRequest) (*RouteResponse, error) {
	// Check if this is a US transit request
	if req.Mode == ModeTransit && req.Country == CountryCode("us") && navConfig.TransitlandURL != "" {
		return routeTransitUS(req)
	}

	// Validate units
	if req.Units == "" {
		req.Units = DefaultUnit
	} else if !req.Units.IsValid() {
		return nil, fmt.Errorf("invalid units: must be one of: %s, %s", UnitKilometers, UnitMiles)
	}

	// Create Valhalla request
	vReq := valhallaRequest{
		Locations: []valhallaLocation{
			{
				Lat:  req.FromLat,
				Lon:  req.FromLng,
				Type: "break",
			},
			{
				Lat:  req.ToLat,
				Lon:  req.ToLng,
				Type: "break",
			},
		},
		Costing: getTransportMode(req.Mode),
		Units:   getValhallaUnits(req.Units),
		CostingOptions: map[string]interface{}{
			"auto": map[string]interface{}{
				"use_display_name": false,
			},
			"pedestrian": map[string]interface{}{
				"use_display_name": false,
			},
			"bicycle": map[string]interface{}{
				"use_display_name": false,
			},
		},
	}

	// Add transit-specific parameters if mode is transit
	if req.Mode == ModeTransit {
		// Add current date/time for transit routing
		vReq.DateTime = map[string]interface{}{
			"type":  1,                                     // Meaning depart at specified time
			"value": time.Now().Format("2006-01-02T15:04"), // Current time in ISO format
		}

		// Add transit costing options
		vReq.CostingOptions = map[string]interface{}{
			"transit": map[string]interface{}{
				"use_bus":                        1.0,
				"use_rail":                       1.0,
				"use_transfers":                  1.0,
				"transit_start_end_max_distance": 2000, // meters
				"transit_transfer_max_distance":  500,  // meters
			},
		}

		// For transit, we need to specify costing as "transit" not "multimodal"
		vReq.Costing = "transit"
	}

	// Convert request to JSON
	reqBody, err := json.Marshal(vReq)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %v", err)
	}

	// Make request to Valhalla
	resp, err := http.Post(navConfig.ValhallaURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("error making request to Valhalla: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		// Read error response body
		errorBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("valhalla API returned status %d, failed to read error message: %v", resp.StatusCode, err)
		}

		// Try to parse the error response
		var valhallaError struct {
			ErrorCode  int    `json:"error_code"`
			Error      string `json:"error"`
			StatusCode int    `json:"status_code"`
			Status     string `json:"status"`
		}

		if err := json.Unmarshal(errorBody, &valhallaError); err == nil {
			// Handle specific error codes
			switch valhallaError.ErrorCode {
			case 170:
				if req.Mode == ModeTransit {
					// Switch to auto routing
					req.Mode = ModeAuto
					return route(req)
				}
				return nil, fmt.Errorf("no route found: locations are not connected in the transportation network")
			default:
				return nil, fmt.Errorf("routing error: %s", valhallaError.Error)
			}
		}

		// If we couldn't parse the error, return the raw message
		return nil, fmt.Errorf("valhalla API returned status %d: %s", resp.StatusCode, string(errorBody))
	}

	// Decode response
	var vResp valhallaResponse
	if err := json.NewDecoder(resp.Body).Decode(&vResp); err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}

	// Convert response to our format
	result := &RouteResponse{
		Duration: vResp.Trip.Summary.Time,
		Distance: convertDistance(vResp.Trip.Summary.Distance*1000, req.Units), // convert to specified units
		Units:    req.Units,
		Mode:     req.Mode,
		From: Location{
			Desc: req.FromDesc,
			Lat:  req.FromLat,
			Lng:  req.FromLng,
		},
		To: Location{
			Desc: req.ToDesc,
			Lat:  req.ToLat,
			Lng:  req.ToLng,
		},
	}

	// Process steps
	if len(vResp.Trip.Legs) > 0 {
		for i, maneuver := range vResp.Trip.Legs[0].Maneuvers {
			step := RouteStep{
				Number:      i + 1,
				Description: abbreviateInstruction(maneuver.Instruction),
				Distance:    convertDistance(maneuver.Distance*1000, req.Units),
				Icon:        getStepIcon(maneuver.Type, maneuver.Instruction, ""),
			}

			// For the first step, override the icon based on the transport mode
			if i == 0 {
				switch req.Mode {
				case ModeBiking:
					step.Icon = "Cycle"
				case ModeWalking:
					step.Icon = "Walk"
				case ModeAuto:
					step.Icon = "Drive"
				}
			}

			result.Steps = append(result.Steps, step)
		}

		// Decode and normalize the path
		points := decodePolyline(vResp.Trip.Legs[0].Shape)
		result.Path = Path{
			Points: points,
			Length: len(points),
			Width:  NormalizedGridSize,
			Height: NormalizedGridSize,
		}
	}

	return result, nil
}
