package olx

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const listingURLPrefix = "https://www.olx.in/item/iid-"

// Parse decodes a search API body.
func Parse(body []byte) (SearchResponse, error) {
	var resp SearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return SearchResponse{}, fmt.Errorf("olx: parse search json: %w", err)
	}
	return resp, nil
}

// Extract maps ads to unique core CSV rows. Missing params become empty cells.
func Extract(resp SearchResponse) []Row {
	seen := make(map[string]struct{}, len(resp.Data))
	rows := make([]Row, 0, len(resp.Data))
	for _, ad := range resp.Data {
		id := strings.TrimSpace(ad.AdID)
		if id == "" {
			id = strings.TrimSpace(ad.ID)
		}
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		rows = append(rows, Row{
			AdID:         id,
			Title:        ad.Title,
			Price:        formatPrice(ad.Price.Value.Raw),
			Year:         param(ad, "year"),
			KM:           param(ad, "mileage"),
			Fuel:         param(ad, "petrol"),
			Transmission: param(ad, "transmission"),
			Owners:       param(ad, "first_owner"),
			Variant:      param(ad, "variant"),
			Seller:       ad.UserName,
			Location:     formatLocation(ad.LocationsResolved),
			CreatedAt:    ad.CreatedAt,
			ListingURL:   listingURLPrefix + id,
		})
	}
	return rows
}

func param(ad Ad, key string) string {
	for _, p := range ad.Parameters {
		if p.Key == key {
			if p.FormattedValue != "" {
				return p.FormattedValue
			}
			return p.ValueName
		}
	}
	return ""
}

func formatLocation(loc LocationsResolved) string {
	city := strings.TrimSpace(loc.AdminLevel3Name)
	sub := strings.TrimSpace(loc.SublocalityLevel1Name)
	switch {
	case city != "" && sub != "":
		return city + " / " + sub
	case city != "":
		return city
	default:
		return sub
	}
}

func formatPrice(raw float64) string {
	if raw == 0 {
		return ""
	}
	if raw == float64(int64(raw)) {
		return strconv.FormatInt(int64(raw), 10)
	}
	return strconv.FormatFloat(raw, 'f', -1, 64)
}
