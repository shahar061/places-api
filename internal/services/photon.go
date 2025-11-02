package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type PhotonResponse struct {
	Type     string           `json:"type"`
	Features []PhotonLocation `json:"features"`
}

type PhotonLocation struct {
	Type       string `json:"type"`
	Properties struct {
		OsmType     string `json:"osm_type"`
		OsmID       int    `json:"osm_id"`
		OsmKey      string `json:"osm_key"`
		OsmValue    string `json:"osm_value"`
		Type        string `json:"type"`
		Postcode    string `json:"postcode"`
		Countrycode string `json:"countrycode"`
		Name        string `json:"name"`
		Country     string `json:"country"`
		City        string `json:"city"`
		Street      string `json:"street"`
		State       string `json:"state"`
		County      string `json:"county"`
		HouseNumber string `json:"housenumber"`
	} `json:"properties"`
	Geometry struct {
		Type        string    `json:"type"`
		Coordinates []float64 `json:"coordinates"`
	} `json:"geometry"`
}

type PhotonService struct {
}

func NewPhotonService() *PhotonService {
	return &PhotonService{}
}

func (s *PhotonService) GetLocationData(locationName string, latitude float64, longitude float64) (*PhotonLocation, error) {
	photonBaseURL := "https://photon.komoot.io/api/"
	// Build the query parameters
	queryParams := url.Values{}
	queryParams.Set("q", locationName)
	queryParams.Set("lat", fmt.Sprintf("%f", latitude))
	queryParams.Set("lon", fmt.Sprintf("%f", longitude))
	queryParams.Set("limit", "1")
	queryParams.Set("lang", "en")

	// Build the Photon URL
	photonURL := fmt.Sprintf("%s?%s", photonBaseURL, queryParams.Encode())

	resp, err := http.Get(photonURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get location data: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get location data: %v", resp.StatusCode)
	}

	// Parse the response
	var photonResponse PhotonResponse
	if err := json.NewDecoder(resp.Body).Decode(&photonResponse); err != nil {
		return nil, fmt.Errorf("failed to parse photon response: %v", err)
	}

	if len(photonResponse.Features) == 0 {
		return nil, fmt.Errorf("no results found for query")
	}

	return &photonResponse.Features[0], nil
}
