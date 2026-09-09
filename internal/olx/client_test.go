package olx

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDetectSoftBlock(t *testing.T) {
	if reason, ok := DetectSoftBlock(200, []byte(`<html>please enable javascript</html>`)); !ok {
		t.Fatalf("expected soft block, got %q", reason)
	}
	if _, ok := DetectSoftBlock(200, []byte(`{"data":[]}`)); ok {
		t.Fatal("valid json should not be a soft block")
	}
	if _, ok := DetectSoftBlock(403, []byte("no")); ok {
		t.Fatal("non-200 is not classified as soft block")
	}
}

func TestSearchAllSuccessAndPagination(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if r.Header.Get("User-Agent") == "" || r.Header.Get("Origin") == "" {
			t.Error("missing replay headers")
		}
		from := r.URL.Query().Get("from")
		w.Header().Set("Content-Type", "application/json")
		if from == "" || from == "0" {
			io.WriteString(w, `{"data":[{"ad_id":"1","title":"one"}],"metadata":{"total_ads":2,"ads_on_page":1,"total_pages":2}}`)
			return
		}
		io.WriteString(w, `{"data":[{"ad_id":"2","title":"two"}],"metadata":{"total_ads":2,"ads_on_page":1,"total_pages":2}}`)
	}))
	defer srv.Close()

	c := NewClient(2 * time.Second)
	c.HTTP = srv.Client()
	c.BaseURL = srv.URL
	c.PageSize = 1
	c.Sleep = func(time.Duration) {}

	pages, err := c.SearchAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	merged := Merge(pages)
	rows := Extract(merged)
	if len(rows) != 2 {
		t.Fatalf("rows=%d pages=%d hits=%d", len(rows), len(pages), hits)
	}
}

func TestSearchAllDropsFingerprintOn403(t *testing.T) {
	var withFP, withoutFP int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Panamera-Fingerprint") != "" {
			atomic.AddInt32(&withFP, 1)
			http.Error(w, "nope", http.StatusForbidden)
			return
		}
		atomic.AddInt32(&withoutFP, 1)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"data":[{"ad_id":"9","title":"ok"}],"metadata":{"total_ads":1,"ads_on_page":1,"total_pages":1}}`)
	}))
	defer srv.Close()

	c := NewClient(2 * time.Second)
	c.HTTP = srv.Client()
	c.BaseURL = srv.URL
	c.Sleep = func(time.Duration) {}

	pages, err := c.SearchAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if withFP == 0 || withoutFP == 0 {
		t.Fatalf("withFP=%d withoutFP=%d", withFP, withoutFP)
	}
	if len(Extract(Merge(pages))) != 1 {
		t.Fatal("expected one row after fingerprint drop")
	}
}

func TestSearchAllRetries429(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		io.WriteString(w, `{"data":[{"ad_id":"3"}],"metadata":{"total_ads":1}}`)
	}))
	defer srv.Close()

	c := NewClient(2 * time.Second)
	c.HTTP = srv.Client()
	c.BaseURL = srv.URL
	c.Sleep = func(time.Duration) {}

	pages, err := c.SearchAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if hits < 2 {
		t.Fatalf("hits=%d want retry", hits)
	}
	if len(pages) != 1 || pages[0].Response.Data[0].AdID != "3" {
		t.Fatalf("unexpected pages: %+v", pages)
	}
}

func TestPermanent400NotRetried(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		http.Error(w, "bad", http.StatusBadRequest)
	}))
	defer srv.Close()

	c := NewClient(2 * time.Second)
	c.HTTP = srv.Client()
	c.BaseURL = srv.URL
	c.MaxRetries = 3
	c.Sleep = func(time.Duration) {}

	_, err := c.SearchAll(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if hits != 1 {
		t.Fatalf("hits=%d want 1 (no retry on 400)", hits)
	}
	if !strings.Contains(err.Error(), "permanent") {
		t.Fatalf("err=%v", err)
	}
}
