package csvout

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bhuvanesh15/golx/internal/olx"
)

func TestWriteBOMAndHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cars.csv")
	rows := []olx.Row{{
		AdID:         "1852925548",
		Title:        "Maruti Suzuki S-Cross Zeta 1.6, 2017, Diesel",
		Price:        "765000",
		Year:         "2017",
		KM:           "85,000 km",
		Fuel:         "Diesel",
		Transmission: "Manual",
		Owners:       "1st",
		Variant:      "Zeta 1.6",
		Seller:       "Mahadev cars",
		Location:     "Chennai / Nungambakkam Mahalinga Puram",
		CreatedAt:    "2026-08-18T07:41:06+05:30",
		ListingURL:   "https://www.olx.in/item/iid-1852925548",
	}}
	if err := Write(path, rows); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF}) {
		t.Fatal("missing utf-8 bom")
	}
	text := string(raw[3:])
	if !strings.HasPrefix(text, "ad_id,title,price,") {
		t.Fatalf("header: %q", text[:min(80, len(text))])
	}
	if !strings.Contains(text, "1852925548") {
		t.Fatal("missing ad id")
	}
}

func TestDefaultPath(t *testing.T) {
	got := DefaultPath(time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC))
	want := filepath.Join("data", "out", "olx_cars_chennai_s-cross_20260909.csv")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
