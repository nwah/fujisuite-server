package nav

// TransportMode represents the mode of transportation
type TransportMode string

const (
	ModeWalking TransportMode = "walking"
	ModeBiking  TransportMode = "biking"
	ModeAuto    TransportMode = "auto"
	ModeTransit TransportMode = "transit"
)

// DefaultMode is the default transport mode if none is specified
const DefaultMode = ModeAuto

// DistanceUnit represents the unit of measurement for distances
type DistanceUnit string

const (
	UnitKilometers DistanceUnit = "km"
	UnitMiles      DistanceUnit = "mi"
)

// DefaultUnit is the default distance unit if none is specified
const DefaultUnit = UnitKilometers

// CountryCode represents a two-letter ISO country code
type CountryCode string

// NormalizedGridSize is the size of the normalized grid for path points
const NormalizedGridSize = 100

// IsValid checks if the transport mode is valid
func (m TransportMode) IsValid() bool {
	switch m {
	case ModeWalking, ModeBiking, ModeAuto, ModeTransit:
		return true
	default:
		return false
	}
}

// IsValid checks if the distance unit is valid
func (u DistanceUnit) IsValid() bool {
	switch u {
	case UnitKilometers, UnitMiles:
		return true
	default:
		return false
	}
}

// IsValid checks if the country code is valid
func (c CountryCode) IsValid() bool {
	// For now, just check if it's exactly 2 characters
	// Could be enhanced to check against a list of valid ISO codes
	return len(c) == 2
}
