package fdaclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// This mirrors the shape openFDA sends back.
// Notice every field is a []string — a LIST — because that's what openFDA gives.
type OpenFDABlock struct {
	BrandName     []string `json:"brand_name"`
	GenericName   []string `json:"generic_name"`
	Manufacturer  []string `json:"manufacturer_name"`
	ProductNDC    []string `json:"product_ndc"`
	ProductType   []string `json:"product_type"`
	Route         []string `json:"route"`
	SubstanceName []string `json:"substance_name"`
}

type Result struct {
	ID      string       `json:"id"`
	OpenFDA OpenFDABlock `json:"openfda"`
}

type Response struct {
	Results []Result `json:"results"`
}

// Fetch asks openFDA for `count` drug records.
func Fetch(count int) ([]Result, error) {
	url := fmt.Sprintf("https://api.fda.gov/drug/label.json?limit=%d", count)

	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("calling openFDA: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openFDA returned status %d", resp.StatusCode)
	}

	var parsed Response
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("reading openFDA reply: %w", err)
	}

	return parsed.Results, nil
}