package olx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultBaseURL     = "https://api.olx.in/relevance/v4/search"
	DefaultFingerprint = "c9119c164cb19f6047141ce6179ed91d#1694198426336"
	DefaultPageSize    = 40
	DefaultTimeout     = 5 * time.Second
	DefaultMaxRetries  = 3
)

var capturedHeaders = map[string]string{
	"accept":                 "*/*",
	"accept-language":        "en-US,en;q=0.9",
	"origin":                 "https://www.olx.in",
	"referer":                "https://www.olx.in/chennai_g4059162/cars_c84?isSearchCall=true&filter=first_owner_eq_1%2Cmake_eq_maruti-suzuki%2Cmodel_eq_maruti-suzuki-s-cross%2Cpetrol_eq_diesel",
	"sec-ch-ua":              `"Chromium";v="152", "Not?A_Brand";v="24", "Google Chrome";v="152"`,
	"sec-ch-ua-mobile":       "?0",
	"sec-ch-ua-platform":     `"Windows"`,
	"sec-fetch-dest":         "empty",
	"sec-fetch-mode":         "cors",
	"sec-fetch-site":         "same-site",
	"user-agent":             "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.0.0 Safari/537.36",
	"x-panamera-fingerprint": DefaultFingerprint,
}

var softBlockNeedles = []string{
	"captcha",
	"verify you are human",
	"access denied",
	"please enable javascript",
	"automated access",
	"rate limit",
	"sign in to continue",
	"unusual traffic",
}

// PermanentError is a non-retryable HTTP failure (4xx except 429).
type PermanentError struct {
	Status int
	Body   string
}

func (e *PermanentError) Error() string {
	return fmt.Sprintf("olx: permanent http %d: %s", e.Status, trimBody(e.Body, 200))
}

// SoftBlockError is HTTP 200 with a block page or non-JSON body.
type SoftBlockError struct {
	Reason string
}

func (e *SoftBlockError) Error() string {
	return "olx: soft block: " + e.Reason
}

// FetchResult is one search page plus the raw body (for raw-vs-parsed storage).
type FetchResult struct {
	Body     []byte
	Response SearchResponse
	From     int
}

// Client replays the captured OLX search request.
type Client struct {
	HTTP        *http.Client
	BaseURL     string
	Fingerprint string
	MaxRetries  int
	PageSize    int
	Sleep       func(time.Duration)
	// Log is optional. When set, the client prints fetch/retry progress.
	Log func(format string, args ...any)
}

func NewClient(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Client{
		HTTP:        &http.Client{Timeout: timeout},
		BaseURL:     DefaultBaseURL,
		Fingerprint: DefaultFingerprint,
		MaxRetries:  DefaultMaxRetries,
		PageSize:    DefaultPageSize,
		Sleep:       time.Sleep,
	}
}

// SearchQuery is the pinned Chennai / S-Cross / diesel / first-owner filter.
func SearchQuery(from, size int) url.Values {
	q := url.Values{}
	q.Set("category", "84")
	q.Set("facet_limit", "1000")
	q.Set("first_owner", "1")
	q.Set("isSearchCall", "true")
	q.Set("location", "4059162")
	q.Set("location_facet_limit", "40")
	q.Set("make", "maruti-suzuki")
	q.Set("model", "maruti-suzuki-s-cross")
	q.Set("nested-filters", `{"make":[{"maruti-suzuki":{"model":["maruti-suzuki-s-cross"]}}]}`)
	q.Set("petrol", "diesel")
	q.Set("platform", "web-desktop")
	q.Set("pttEnabled", "true")
	q.Set("relaxedFilters", "true")
	q.Set("size", strconv.Itoa(size))
	q.Set("user", "anonymous")
	q.Set("lang", "en-IN")
	if from > 0 {
		q.Set("from", strconv.Itoa(from))
	}
	return q
}

// SearchAll walks pages until ads run out. On 403/empty/soft-block it retries once without the fingerprint.
func (c *Client) SearchAll(ctx context.Context) ([]FetchResult, error) {
	var pages []FetchResult
	from := 0
	droppedFP := false
	c.logf("search start  url=%s  size=%d  timeout=%s  retries=%d", c.BaseURL, c.pageSize(), c.httpTimeout(), c.retryLimit())
	for {
		c.logf("page fetch  from=%d  fingerprint=%s", from, onOff(!droppedFP))
		page, err := c.fetchPage(ctx, from, !droppedFP)
		if reason := fingerprintDropReason(err, page); reason != "" && !droppedFP {
			droppedFP = true
			c.logf("fingerprint drop  reason=%s  retrying without x-panamera-fingerprint", reason)
			page, err = c.fetchPage(ctx, from, false)
		}
		if err != nil {
			c.logf("page fail  from=%d  err=%v", from, err)
			return pages, err
		}
		pages = append(pages, page)
		meta := page.Response.Metadata
		c.logf("page ok  from=%d  ads=%d  total_ads=%d  total_pages=%d  bytes=%d",
			from, len(page.Response.Data), meta.TotalAds, meta.TotalPages, len(page.Body))
		from += c.pageSize()
		if len(page.Response.Data) == 0 {
			break
		}
		if page.Response.Metadata.TotalAds > 0 && from >= page.Response.Metadata.TotalAds {
			c.logf("pagination done  collected_pages=%d  next_from=%d  total_ads=%d", len(pages), from, meta.TotalAds)
			break
		}
	}
	return pages, nil
}

func (c *Client) fetchPage(ctx context.Context, from int, withFingerprint bool) (FetchResult, error) {
	var last error
	retries := c.MaxRetries
	if retries <= 0 {
		retries = 1
	}
	for attempt := 0; attempt < retries; attempt++ {
		if attempt > 0 {
			d := c.backoff(attempt)
			c.logf("retry  from=%d  attempt=%d/%d  after=%v  backoff=%s", from, attempt+1, retries, last, d)
		}
		c.logf("http get  from=%d  attempt=%d/%d  fingerprint=%s", from, attempt+1, retries, onOff(withFingerprint))
		started := time.Now()
		body, status, err := c.doGET(ctx, from, withFingerprint)
		elapsed := time.Since(started).Round(time.Millisecond)
		if err != nil {
			c.logf("http err  from=%d  elapsed=%s  err=%v", from, elapsed, err)
			if isTransientNet(err) && attempt+1 < retries {
				last = err
				continue
			}
			return FetchResult{}, err
		}
		c.logf("http %d  from=%d  elapsed=%s  bytes=%d", status, from, elapsed, len(body))
		if isTransientStatus(status) {
			last = fmt.Errorf("olx: http %d", status)
			if attempt+1 < retries {
				continue
			}
			return FetchResult{}, last
		}
		if status >= 400 && status != 429 {
			return FetchResult{}, &PermanentError{Status: status, Body: string(body)}
		}
		if reason, blocked := DetectSoftBlock(status, body); blocked {
			return FetchResult{}, &SoftBlockError{Reason: reason}
		}
		parsed, err := Parse(body)
		if err != nil {
			return FetchResult{}, &SoftBlockError{Reason: err.Error()}
		}
		return FetchResult{Body: body, Response: parsed, From: from}, nil
	}
	if last == nil {
		last = errors.New("olx: exhausted retries")
	}
	return FetchResult{}, last
}

func (c *Client) doGET(ctx context.Context, from int, withFingerprint bool) ([]byte, int, error) {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, 0, err
	}
	u.RawQuery = SearchQuery(from, c.pageSize()).Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, 0, err
	}
	for k, v := range capturedHeaders {
		if k == "x-panamera-fingerprint" && !withFingerprint {
			continue
		}
		if k == "x-panamera-fingerprint" && c.Fingerprint != "" {
			req.Header.Set(k, c.Fingerprint)
			continue
		}
		req.Header.Set(k, v)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func (c *Client) pageSize() int {
	if c.PageSize <= 0 {
		return DefaultPageSize
	}
	return c.PageSize
}

func (c *Client) backoff(attempt int) time.Duration {
	sleep := c.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	// 200ms * 2^attempt + 0–150ms jitter, capped at 2s (filter is tiny).
	d := 200*time.Millisecond*(1<<uint(attempt)) + time.Duration(rand.Intn(150))*time.Millisecond
	if d > 2*time.Second {
		d = 2 * time.Second
	}
	sleep(d)
	return d
}

func (c *Client) logf(format string, args ...any) {
	if c.Log == nil {
		return
	}
	c.Log(format, args...)
}

func (c *Client) httpTimeout() time.Duration {
	if c.HTTP == nil {
		return 0
	}
	return c.HTTP.Timeout
}

func (c *Client) retryLimit() int {
	if c.MaxRetries <= 0 {
		return 1
	}
	return c.MaxRetries
}

func onOff(v bool) string {
	if v {
		return "on"
	}
	return "off"
}

func fingerprintDropReason(err error, page FetchResult) string {
	if err == nil {
		if len(page.Response.Data) == 0 {
			return "empty data"
		}
		return ""
	}
	var perm *PermanentError
	if errors.As(err, &perm) && perm.Status == 403 {
		return "http 403"
	}
	var soft *SoftBlockError
	if errors.As(err, &soft) {
		return soft.Reason
	}
	return ""
}

// DetectSoftBlock flags 200s that are block pages or not JSON.
func DetectSoftBlock(status int, body []byte) (string, bool) {
	if status != http.StatusOK {
		return "", false
	}
	trimmed := strings.TrimSpace(string(body))
	if len(trimmed) < 500 && !strings.HasPrefix(trimmed, "{") {
		return "suspiciously small non-json body", true
	}
	lower := strings.ToLower(trimmed)
	for _, n := range softBlockNeedles {
		if strings.Contains(lower, n) && !strings.HasPrefix(trimmed, "{") {
			return n, true
		}
	}
	if !jsonLooksLikeObject(trimmed) {
		return "response is not json object", true
	}
	return "", false
}

func jsonLooksLikeObject(s string) bool {
	return strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")
}

func isTransientStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

func isTransientNet(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "timeout") || strings.Contains(s, "connection reset") || strings.Contains(s, "eof")
}

func trimBody(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// Merge combines paginated ads; first page metadata wins for totals.
func Merge(pages []FetchResult) SearchResponse {
	out := SearchResponse{}
	if len(pages) == 0 {
		return out
	}
	out.Version = pages[0].Response.Version
	out.Metadata = pages[0].Response.Metadata
	for _, p := range pages {
		out.Data = append(out.Data, p.Response.Data...)
	}
	out.Empty = len(out.Data) == 0
	if out.Metadata.TotalAds == 0 {
		out.Metadata.TotalAds = len(out.Data)
	}
	return out
}
