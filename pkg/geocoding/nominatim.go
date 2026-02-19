package geocoding

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const nominatimBaseURL = "https://nominatim.openstreetmap.org/reverse"

// ReverseResult holds country, province, district, neighborhood from reverse geocoding.
type ReverseResult struct {
	Country      string
	Province     string
	District     string
	Neighborhood string
}

// nominatimResponse top-level; address is parsed as map for flexibility.
type nominatimResponse struct {
	Address map[string]interface{} `json:"address"`
}

func strFromMap(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return s
			}
		}
	}
	return ""
}

// ReverseGeocode calls Nominatim to get address components for lat, lng.
// Requires outbound HTTPS from the server to nominatim.openstreetmap.org.
// Set env DISABLE_REVERSE_GEOCODE=1 to skip the external call (e.g. when server has no internet).
// Returns nil if the request fails or address details are missing (no error).
func ReverseGeocode(ctx context.Context, lat, lng float64) (*ReverseResult, error) {
	if os.Getenv("DISABLE_REVERSE_GEOCODE") == "1" {
		return nil, nil
	}
	u, err := url.Parse(nominatimBaseURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("lat", fmt.Sprintf("%f", lat))
	q.Set("lon", fmt.Sprintf("%f", lng))
	q.Set("format", "json")
	q.Set("addressdetails", "1")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	// Nominatim usage policy requires a valid User-Agent identifying the application
	req.Header.Set("User-Agent", "Hamsaya/1.0 (https://github.com/hamsaya; support@hamsaya.com)")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nominatim returned %d", resp.StatusCode)
	}

	var out nominatimResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.Address == nil {
		return nil, nil
	}

	r := &ReverseResult{
		Country:      strFromMap(out.Address, "country"),
		Province:     strFromMap(out.Address, "state"),
		District:     strFromMap(out.Address, "state_district", "county"),
		Neighborhood: strFromMap(out.Address, "neighbourhood", "suburb"),
	}

	if r.Country == "" && r.Province == "" && r.District == "" && r.Neighborhood == "" {
		return nil, nil
	}
	return r, nil
}
