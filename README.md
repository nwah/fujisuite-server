# FujiNav API

A simple HTTP API that wraps OpenStreetMap and Valhalla for geocoding and routing functionality for use with the FujiNav program on 8-bit computers like the Atari.

## Endpoints

### 1. Geocoding

```
GET /nav/geocode?q={query}
```

Search for an address, landmark, or point of interest by name.

```
POST /nav/geocode
Content-Type: text/plain

123 Main St, Springfield, IL
```

For POST requests, the entire request body is used as the search query, with whitespace trimmed.

**Response:**
```json
{
    "name": "place or street name",
    "address": "normalized address string",
    "lat": 123.456,
    "lng": 789.012
}
```

### 2. Routing

```
GET /route?from={lat,lng}&to={lat,lng}&mode={mode}&units={units}
```

```
POST /route
Content-Type: text/plain

40.7128,-74.0060
34.0522,-118.2437
```

Get navigation directions between two points.

**GET Parameters:**
- `from`: Starting coordinates (lat,lng)
- `to`: Destination coordinates (lat,lng)
- `mode`: One of: walking, biking, driving, transit (default: driving)
- `units`: One of: km, mi (default: km)

**POST Format:**
- Plain text body with exactly 2 lines
- Each line contains coordinates in "lat,lng" format
- First line is the starting point
- Second line is the destination
- Uses default mode (driving) and units (km)

**Response:**
```json
{
    "duration": "estimated duration in seconds",
    "distance": "distance in specified units",
    "units": "km or mi",
    "mode": "the routing mode used",
    "steps": [
        {
            "number": 1,
            "description": "Turn left onto Main St",
            "distance": "distance in specified units"
        }
    ],
    "path": [
        {
            "x": 123, // normalized to 0-100
            "y": 456  // normalized to 0-100
        }
    ]
}
```

## Setup

1. Install Go 1.21 or later
2. Clone this repository
3. Run `go build`
4. Start the server: `./fujisuite-server`

The server will start on port 8080 by default. 
