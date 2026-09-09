package csvout

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bhuvanesh15/golx/internal/olx"
)

var header = []string{
	"ad_id",
	"title",
	"price",
	"year",
	"km",
	"fuel",
	"transmission",
	"owners",
	"variant",
	"seller",
	"location",
	"created_at",
	"listing_url",
}

// DefaultPath is Goshare2-style: data/out/olx_cars_chennai_s-cross_YYYYMMDD.csv
func DefaultPath(now time.Time) string {
	return filepath.Join("data", "out", fmt.Sprintf("olx_cars_chennai_s-cross_%s.csv", now.Format("20060102")))
}

// Write writes a UTF-8 BOM CSV so Excel opens it cleanly.
func Write(path string, rows []olx.Row) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("csv: mkdir: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("csv: create: %w", err)
	}
	defer f.Close()

	if _, err := f.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return fmt.Errorf("csv: bom: %w", err)
	}
	w := csv.NewWriter(f)
	if err := w.Write(header); err != nil {
		return fmt.Errorf("csv: header: %w", err)
	}
	for _, row := range rows {
		if err := w.Write(row.CSV()); err != nil {
			return fmt.Errorf("csv: row %s: %w", row.AdID, err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return fmt.Errorf("csv: flush: %w", err)
	}
	return nil
}
