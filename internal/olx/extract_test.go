package olx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractGoldenResponse(t *testing.T) {
	body, err := os.ReadFile(goldenPath(t))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	resp, err := Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Metadata.TotalAds != 11 {
		t.Fatalf("metadata.total_ads=%d want 11", resp.Metadata.TotalAds)
	}

	rows := Extract(resp)
	if len(rows) != 11 {
		t.Fatalf("rows=%d want 11", len(rows))
	}

	seen := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		if r.AdID == "" {
			t.Fatal("empty ad_id")
		}
		if _, ok := seen[r.AdID]; ok {
			t.Fatalf("duplicate ad_id %s", r.AdID)
		}
		seen[r.AdID] = struct{}{}
	}

	first := rows[0]
	if first.AdID != "1852925548" {
		t.Fatalf("first ad_id=%q want 1852925548", first.AdID)
	}
	if first.Title != "Maruti Suzuki S-Cross Zeta 1.6, 2017, Diesel" {
		t.Fatalf("title=%q", first.Title)
	}
	if first.Price != "765000" {
		t.Fatalf("price=%q want 765000", first.Price)
	}
	if first.Year != "2017" {
		t.Fatalf("year=%q", first.Year)
	}
	if first.KM != "85,000 km" {
		t.Fatalf("km=%q", first.KM)
	}
	if first.Fuel != "Diesel" {
		t.Fatalf("fuel=%q", first.Fuel)
	}
	if first.Transmission != "Manual" {
		t.Fatalf("transmission=%q", first.Transmission)
	}
	if first.Owners != "1st" {
		t.Fatalf("owners=%q", first.Owners)
	}
	if first.Variant != "Zeta 1.6" {
		t.Fatalf("variant=%q", first.Variant)
	}
	if first.Seller != "Mahadev cars" {
		t.Fatalf("seller=%q", first.Seller)
	}
	if first.Location != "Chennai / Nungambakkam Mahalinga Puram" {
		t.Fatalf("location=%q", first.Location)
	}
	if first.CreatedAt != "2026-08-18T07:41:06+05:30" {
		t.Fatalf("created_at=%q", first.CreatedAt)
	}
	if first.ListingURL != "https://www.olx.in/item/iid-1852925548" {
		t.Fatalf("listing_url=%q", first.ListingURL)
	}

	if len(rows) != resp.Metadata.TotalAds {
		t.Fatalf("count mismatch rows=%d total_ads=%d", len(rows), resp.Metadata.TotalAds)
	}
}

func TestExtractDedupesAndSkipsMissingParams(t *testing.T) {
	resp := SearchResponse{
		Data: []Ad{
			{AdID: "1", Title: "a", Parameters: nil},
			{AdID: "1", Title: "duplicate"},
			{ID: "2", Title: "b"},
			{Title: "no-id"},
		},
		Metadata: Metadata{TotalAds: 2},
	}
	rows := Extract(resp)
	if len(rows) != 2 {
		t.Fatalf("rows=%d want 2", len(rows))
	}
	if rows[0].Year != "" || rows[0].Price != "" {
		t.Fatalf("expected empty optional cells, got %+v", rows[0])
	}
	if rows[1].AdID != "2" {
		t.Fatalf("second id=%q", rows[1].AdID)
	}
}

func goldenPath(t *testing.T) string {
	t.Helper()
	candidates := []string{
		filepath.Join("testdata", "search.json"),
		filepath.Join("internal", "olx", "testdata", "search.json"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Fatal("testdata/search.json not found")
	return ""
}
