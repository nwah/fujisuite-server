package nav

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Maps for address abbreviations
var (
	directionAbbrev = map[string]string{
		"north":     "N",
		"south":     "S",
		"east":      "E",
		"west":      "W",
		"northeast": "NE",
		"northwest": "NW",
		"southeast": "SE",
		"southwest": "SW",
	}

	streetTypeAbbrev = map[string]string{
		"avenue":     "Ave",
		"boulevard":  "Blvd",
		"circle":     "Cir",
		"court":      "Ct",
		"drive":      "Dr",
		"expressway": "Expy",
		"heights":    "Hts",
		"highway":    "Hwy",
		"junction":   "Jct",
		"lane":       "Ln",
		"parkway":    "Pkwy",
		"place":      "Pl",
		"plaza":      "Plz",
		"road":       "Rd",
		"square":     "Sq",
		"street":     "St",
		"terrace":    "Ter",
		"trail":      "Trl",
		"turnpike":   "Tpke",
		"way":        "Way",
	}

	stateAbbrev = map[string]string{
		"alabama":        "AL",
		"alaska":         "AK",
		"arizona":        "AZ",
		"arkansas":       "AR",
		"california":     "CA",
		"colorado":       "CO",
		"connecticut":    "CT",
		"delaware":       "DE",
		"florida":        "FL",
		"georgia":        "GA",
		"hawaii":         "HI",
		"idaho":          "ID",
		"illinois":       "IL",
		"indiana":        "IN",
		"iowa":           "IA",
		"kansas":         "KS",
		"kentucky":       "KY",
		"louisiana":      "LA",
		"maine":          "ME",
		"maryland":       "MD",
		"massachusetts":  "MA",
		"michigan":       "MI",
		"minnesota":      "MN",
		"mississippi":    "MS",
		"missouri":       "MO",
		"montana":        "MT",
		"nebraska":       "NE",
		"nevada":         "NV",
		"new hampshire":  "NH",
		"new jersey":     "NJ",
		"new mexico":     "NM",
		"new york":       "NY",
		"north carolina": "NC",
		"north dakota":   "ND",
		"ohio":           "OH",
		"oklahoma":       "OK",
		"oregon":         "OR",
		"pennsylvania":   "PA",
		"rhode island":   "RI",
		"south carolina": "SC",
		"south dakota":   "SD",
		"tennessee":      "TN",
		"texas":          "TX",
		"utah":           "UT",
		"vermont":        "VT",
		"virginia":       "VA",
		"washington":     "WA",
		"west virginia":  "WV",
		"wisconsin":      "WI",
		"wyoming":        "WY",
	}
)

// ErrNoResults is returned when no geocoding results are found
type ErrNoResults struct {
	Query string
}

func (e *ErrNoResults) Error() string {
	return fmt.Sprintf("no results found for query: %s", e.Query)
}

type nominatimAddress struct {
	HouseNumber string `json:"house_number"`
	Road        string `json:"road"`
	Suburb      string `json:"suburb"`
	City        string `json:"city"`
	Town        string `json:"town"`
	Village     string `json:"village"`
	County      string `json:"county"`
	State       string `json:"state"`
	PostCode    string `json:"postcode"`
	Name        string `json:"name"`
	Country     string `json:"country_code"` // Two-letter ISO country code
}

type nominatimResponse struct {
	DisplayName string `json:"display_name"`
	NameDetails struct {
		Name     string `json:"name"`
		Official string `json:"official_name"`
		Alt      string `json:"alt_name"`
	} `json:"namedetails"`
	Lat        string           `json:"lat"`
	Lon        string           `json:"lon"`
	Address    nominatimAddress `json:"address"`
	Importance float64          `json:"importance"`
}

// Helper functions for address abbreviations
func abbreviateDirection(word string) string {
	if abbrev, ok := directionAbbrev[strings.ToLower(word)]; ok {
		return abbrev
	}
	return word
}

func abbreviateStreetType(word string) string {
	if abbrev, ok := streetTypeAbbrev[strings.ToLower(word)]; ok {
		return abbrev
	}
	return word
}

func abbreviateState(state string) string {
	if abbrev, ok := stateAbbrev[strings.ToLower(state)]; ok {
		return abbrev
	}
	return state
}

func abbreviateStreetName(street string) string {
	words := strings.Fields(street)
	if len(words) == 0 {
		return street
	}

	// Check if the first word is a direction
	if len(words) > 1 {
		words[0] = abbreviateDirection(words[0])
	}

	// Check if the last word is a street type
	if len(words) > 1 {
		words[len(words)-1] = abbreviateStreetType(words[len(words)-1])
	}

	return strings.Join(words, " ")
}

func formatAddress(addr nominatimAddress, nameDetails struct {
	Name     string `json:"name"`
	Official string `json:"official_name"`
	Alt      string `json:"alt_name"`
}) (name string, formattedAddr string, countryCode string) {
	// Try to get the best name from namedetails
	name = nameDetails.Official
	if name == "" {
		name = nameDetails.Name
	}
	if name == "" {
		name = nameDetails.Alt
	}

	// If no name from namedetails, try address components
	if name == "" {
		name = addr.Name
	}

	// Try to get the city name from various fields
	city := addr.City
	if city == "" {
		city = addr.Town
	}
	if city == "" {
		city = addr.Village
	}
	if city == "" && addr.Suburb != "" {
		city = addr.Suburb
	}
	if city == "" && addr.County != "" {
		city = addr.County
	}

	// Build the street address with abbreviations
	var streetParts []string
	if addr.HouseNumber != "" {
		streetParts = append(streetParts, addr.HouseNumber)
	}
	if addr.Road != "" {
		streetParts = append(streetParts, abbreviateStreetName(addr.Road))
	}
	streetAddress := strings.Join(streetParts, " ")

	// If still no name, use abbreviated street address
	if name == "" {
		name = streetAddress
	}

	// Build the formatted address in US format
	var addrParts []string
	if streetAddress != "" {
		addrParts = append(addrParts, streetAddress)
	}

	// Add city
	var cityStateParts []string
	if city != "" {
		cityStateParts = append(cityStateParts, city)
	}

	// Add abbreviated state and zip in standard US format
	if addr.State != "" && addr.PostCode != "" {
		cityStateParts = append(cityStateParts, fmt.Sprintf("%s %s", abbreviateState(addr.State), addr.PostCode))
	} else if addr.State != "" {
		cityStateParts = append(cityStateParts, abbreviateState(addr.State))
	} else if addr.PostCode != "" {
		cityStateParts = append(cityStateParts, addr.PostCode)
	}

	if len(cityStateParts) > 0 {
		addrParts = append(addrParts, strings.Join(cityStateParts, ", "))
	}

	return name, strings.Join(addrParts, ", "), strings.ToLower(addr.Country)
}

// geocode performs geocoding using Nominatim
func geocode(query string) ([]GeocodeResponse, error) {
	// Build query parameters
	params := url.Values{
		"q":              {query},
		"format":         {"json"},
		"limit":          {"5"},
		"addressdetails": {"1"},
		"namedetails":    {"1"},
	}

	// Create request URL with query parameters
	apiURL := fmt.Sprintf("%s/search?%s", navConfig.NominatimURL, params.Encode())

	// Make GET request
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("error making request to Nominatim: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nominatim API returned status: %d", resp.StatusCode)
	}

	// Decode response
	var nominatimResults []nominatimResponse
	if err := json.NewDecoder(resp.Body).Decode(&nominatimResults); err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}

	if len(nominatimResults) == 0 {
		return nil, &ErrNoResults{Query: query}
	}

	// Convert nominatim results to our format
	results := make([]GeocodeResponse, len(nominatimResults))
	for i, result := range nominatimResults {
		// Parse lat/lon strings to float64
		lat, err := parseFloat(result.Lat)
		if err != nil {
			return nil, fmt.Errorf("error parsing latitude: %v", err)
		}
		lng, err := parseFloat(result.Lon)
		if err != nil {
			return nil, fmt.Errorf("error parsing longitude: %v", err)
		}

		// Format the address components
		name, addr, country := formatAddress(result.Address, result.NameDetails)

		results[i] = GeocodeResponse{
			Name:       name,
			Address:    addr,
			Lat:        lat,
			Lng:        lng,
			Importance: result.Importance,
			Country:    country,
		}
	}

	return results, nil
}

func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}
