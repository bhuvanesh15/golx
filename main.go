package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bhuvanesh15/golx/internal/csvout"
	"github.com/bhuvanesh15/golx/internal/olx"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("olx-cars", flag.ContinueOnError)
	offline := fs.String("offline", "", "parse a golden search JSON file instead of calling the API")
	out := fs.String("out", "", "CSV output path (default: dated data/out/olx_cars_chennai_s-cross_YYYYMMDD.csv)")
	raw := fs.String("raw", filepath.Join("data", "raw", "search.json"), "raw JSON output path (live mode)")
	timeout := fs.Duration("timeout", olx.DefaultTimeout, "HTTP timeout per request")
	retries := fs.Int("retries", olx.DefaultMaxRetries, "retries for 429/5xx/timeouts")
	if err := fs.Parse(args); err != nil {
		return err
	}

	csvPath := *out
	if csvPath == "" {
		csvPath = csvout.DefaultPath(time.Now())
	}

	logger := log.New(os.Stderr, "", log.LstdFlags)
	started := time.Now()

	var (
		resp  olx.SearchResponse
		pages []olx.FetchResult
	)

	if *offline != "" {
		logger.Printf("start  mode=offline  file=%s", *offline)
		body, err := os.ReadFile(*offline)
		if err != nil {
			return fmt.Errorf("read offline file: %w", err)
		}
		logger.Printf("read   %s  (%s)", *offline, formatBytes(len(body)))
		resp, err = olx.Parse(body)
		if err != nil {
			return err
		}
		logger.Printf("parse  ads=%d  total_ads=%d  total_pages=%d",
			len(resp.Data), resp.Metadata.TotalAds, resp.Metadata.TotalPages)
	} else {
		logger.Printf("start  mode=live  filter=Chennai S-Cross diesel 1st-owner")
		logger.Printf("out    csv=%s  raw=%s  timeout=%s  retries=%d", csvPath, *raw, *timeout, *retries)
		ctx := context.Background()
		client := olx.NewClient(*timeout)
		client.MaxRetries = *retries
		client.Log = logger.Printf
		var err error
		pages, err = client.SearchAll(ctx)
		if err != nil {
			if len(pages) > 0 {
				logger.Printf("raw    saving %d page(s) before exit", len(pages))
				_ = writeRaw(*raw, pages)
			}
			return err
		}
		if err := writeRaw(*raw, pages); err != nil {
			return err
		}
		if len(pages) > 0 {
			logger.Printf("raw    wrote %s  (%s)", *raw, formatBytes(len(pages[0].Body)))
			for _, p := range pages[1:] {
				logger.Printf("raw    extra page from=%d  (%s)", p.From, formatBytes(len(p.Body)))
			}
		}
		resp = olx.Merge(pages)
		logger.Printf("merge  pages=%d  ads=%d  total_ads=%d", len(pages), len(resp.Data), resp.Metadata.TotalAds)
	}

	rows := olx.Extract(resp)
	logger.Printf("extract  unique_rows=%d", len(rows))
	for i, row := range rows {
		logger.Printf("ad     %2d/%d  %s  ₹%s  %s  %s  %s",
			i+1, len(rows), row.AdID, emptyDash(row.Price), emptyDash(row.Year), emptyDash(row.Fuel), emptyDash(row.Location))
	}

	if err := csvout.Write(csvPath, rows); err != nil {
		return err
	}
	logger.Printf("csv    wrote %s  rows=%d", csvPath, len(rows))

	total := resp.Metadata.TotalAds
	if total > 0 && len(rows) != total {
		return fmt.Errorf("count mismatch: extracted %d rows, metadata.total_ads=%d", len(rows), total)
	}
	logger.Printf("done   parsed=%d  total_ads=%d  elapsed=%s  ok", len(rows), total, time.Since(started).Round(time.Millisecond))
	return nil
}

func formatBytes(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1fMB", float64(n)/(1<<20))
	case n >= 1024:
		return fmt.Sprintf("%.1fKB", float64(n)/1024.0)
	default:
		return fmt.Sprintf("%dB", n)
	}
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func writeRaw(path string, pages []olx.FetchResult) error {
	if len(pages) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("raw: mkdir: %w", err)
	}
	if err := os.WriteFile(path, pages[0].Body, 0o644); err != nil {
		return fmt.Errorf("raw: write: %w", err)
	}
	dir := filepath.Dir(path)
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	ext := filepath.Ext(path)
	for _, p := range pages[1:] {
		extra := filepath.Join(dir, fmt.Sprintf("%s_from_%d%s", base, p.From, ext))
		if err := os.WriteFile(extra, p.Body, 0o644); err != nil {
			return fmt.Errorf("raw: write page from=%d: %w", p.From, err)
		}
	}
	return nil
}
