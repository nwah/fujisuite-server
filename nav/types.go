package nav

// NavConfig holds navigation-specific configuration
type NavConfig struct {
	NominatimURL      string `toml:"nominatim_url"`
	ValhallaURL       string `toml:"valhalla_url"`
	TransitlandURL    string `toml:"transitland_url"`
	TransitlandAPIKey string `toml:"transitland_api_key"`
}

// GeocodeResponse represents the response from the geocoding endpoint
type GeocodeResponse struct {
	Name       string  `json:"name"`    // Place name or street address
	Address    string  `json:"address"` // Simplified address (street, postal code, city)
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
	Importance float64 `json:"importance"` // Relevance score from 0 to 1
	Country    string  `json:"country"`    // Two-letter ISO country code
}

// RouteRequest represents the parameters for a routing request
type RouteRequest struct {
	FromLat  float64       `json:"fromLat"`
	FromLng  float64       `json:"fromLng"`
	ToLat    float64       `json:"toLat"`
	ToLng    float64       `json:"toLng"`
	FromDesc string        `json:"fromDesc,omitempty"`
	ToDesc   string        `json:"toDesc,omitempty"`
	Mode     TransportMode `json:"mode"`
	Units    DistanceUnit  `json:"units"`
	Country  CountryCode   `json:"country,omitempty"`
}

// RouteStep represents a single navigation step
type RouteStep struct {
	Number      int     `json:"number"`
	Description string  `json:"description"`
	Distance    float64 `json:"distance"` // in specified units
	Icon        string  `json:"icon"`     // Icon representing the step type
}

// PathPoint represents a normalized point on the route path
type PathPoint [2]int // [x, y] normalized to 0-NormalizedGridSize

// Path represents the complete path with metadata
type Path struct {
	Points []PathPoint `json:"points"` // Array of [x, y] points
	Length int         `json:"length"` // Number of points in the path
	Width  int         `json:"width"`  // Width of the normalized grid (NormalizedGridSize)
	Height int         `json:"height"` // Height of the normalized grid (NormalizedGridSize)
}

// Location represents a point with description and coordinates
type Location struct {
	Desc string  `json:"desc"`
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
}

// RouteResponse represents the response from the routing endpoint
type RouteResponse struct {
	Duration float64       `json:"duration"` // in seconds
	Distance float64       `json:"distance"` // in specified units
	Units    DistanceUnit  `json:"units"`    // km or mi
	Steps    []RouteStep   `json:"steps"`
	Path     Path          `json:"path"` // Complete path with metadata
	Mode     TransportMode `json:"mode"` // The mode used for routing
	From     Location      `json:"from"` // Starting location
	To       Location      `json:"to"`   // Destination location
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}
