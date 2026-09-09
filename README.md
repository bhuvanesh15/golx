# golx

A production-oriented OLX India car scraper written in Go.

`golx` does not parse listing HTML. It replays the public OLX search API (`api.olx.in/relevance/v4/search`), validates the JSON payload, and writes a clean UTF-8 CSV. The first target is a pinned Chennai filter: Maruti Suzuki S-Cross, diesel, first owner.

## Features

- **API-first** — extract from the JSON search endpoint, not the page DOM
- **Pinned filter** — Chennai (`4059162`) × Cars (`84`) × S-Cross × diesel × first owner
- **Resilient fetch** — 5s timeout, 2–3 retries with jitter on 429 / 5xx / timeouts
- **Soft-block detection** — HTTP 200 is not treated as success unless the body is valid JSON
- **Fingerprint fallback** — retries once without `x-panamera-fingerprint` on 403 / empty / block pages
- **Pagination** — walks `from` / `size` until `metadata.total_ads` is collected
- **Raw + parsed storage** — keep the API body and the CSV separately so parsers can change without a recrawl
- **Excel-safe CSV** — UTF-8 BOM, fixed column order, dated default filename
- **Progress logs** — fetch, retry, extract, and write steps on stderr
- **Offline mode** — parse a saved search JSON for tests and reruns
- **Stdlib only** — no Colly, no browser, no proxy dependency

## Requirements

- [Go](https://go.dev/dl/) 1.22 or later

## Install

```bash
git clone https://github.com/bhuvanesh15/golx.git
cd golx
go build -o golx .
```

Or install the binary:

```bash
go install github.com/bhuvanesh15/golx@latest
```

## Usage

Live scrape (writes CSV + raw JSON):

```bash
go run .
```

Parse a saved API response:

```bash
go run . -offline internal/olx/testdata/search.json
```

Flags:

| Flag | Default | Description |
|---|---|---|
| `-offline` | | Read a search JSON file instead of calling the API |
| `-out` | `data/out/olx_cars_chennai_s-cross_YYYYMMDD.csv` | CSV output path |
| `-raw` | `data/raw/search.json` | Raw API body path (live mode) |
| `-timeout` | `5s` | Per-request HTTP timeout |
| `-retries` | `3` | Retries for transient failures |

Example:

```bash
go run . -out data/out/cars.csv -timeout 8s -retries 3
```

## Output

Default CSV: `data/out/olx_cars_chennai_s-cross_YYYYMMDD.csv`

| Column | Source |
|---|---|
| `ad_id` | listing id |
| `title` | ad title |
| `price` | `price.value.raw` (INR) |
| `year` | parameter `year` |
| `km` | parameter `mileage` |
| `fuel` | parameter `petrol` |
| `transmission` | parameter `transmission` |
| `owners` | parameter `first_owner` |
| `variant` | parameter `variant` |
| `seller` | `user_name` |
| `location` | city / neighbourhood |
| `created_at` | listing created timestamp |
| `listing_url` | `https://www.olx.in/item/iid-{ad_id}` |

Success is `extracted rows == metadata.total_ads`. A `200` status alone is not enough.

## Architecture

```
capture (browser XHR)
        │
        ▼
   HTTP client  ──►  data/raw/search.json
        │
        ▼
    extractor   ──►  data/out/*.csv
```

```
golx/
├── main.go                 # CLI: live / offline, logging, write paths
├── internal/olx/           # client, types, extract, tests
│   └── testdata/search.json
├── internal/csvout/        # UTF-8 BOM CSV writer
└── README.md
```

## Tests

```bash
go test ./...
```

The golden fixture asserts 11 unique rows and the first listing (`1852925548`).

## Disclaimer

This tool is for personal research on publicly listed classifieds. Respect [OLX](https://www.olx.in) terms of use, robots rules, and local law. Do not use it to overload the service or collect data you are not allowed to store.

## License

[MIT](LICENSE)
