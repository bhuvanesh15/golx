package olx

// SearchResponse is the relevance/v4/search payload.
type SearchResponse struct {
	Version  string   `json:"version"`
	Data     []Ad     `json:"data"`
	Metadata Metadata `json:"metadata"`
	Empty    bool     `json:"empty"`
}

type Metadata struct {
	TotalAds   int `json:"total_ads"`
	AdsOnPage  int `json:"ads_on_page"`
	TotalPages int `json:"total_pages"`
}

type Ad struct {
	ID                string            `json:"id"`
	AdID              string            `json:"ad_id"`
	Title             string            `json:"title"`
	UserName          string            `json:"user_name"`
	CreatedAt         string            `json:"created_at"`
	Price             Price             `json:"price"`
	Parameters        []Parameter       `json:"parameters"`
	LocationsResolved LocationsResolved `json:"locations_resolved"`
}

type Price struct {
	Value PriceValue `json:"value"`
}

type PriceValue struct {
	Raw     float64 `json:"raw"`
	Display string  `json:"display"`
}

type Parameter struct {
	Key            string `json:"key"`
	FormattedValue string `json:"formatted_value"`
	ValueName      string `json:"value_name"`
}

type LocationsResolved struct {
	AdminLevel3Name       string `json:"ADMIN_LEVEL_3_name"`
	SublocalityLevel1Name string `json:"SUBLOCALITY_LEVEL_1_name"`
}

// Row is one CSV record for the pinned Chennai S-Cross filter.
type Row struct {
	AdID         string
	Title        string
	Price        string
	Year         string
	KM           string
	Fuel         string
	Transmission string
	Owners       string
	Variant      string
	Seller       string
	Location     string
	CreatedAt    string
	ListingURL   string
}

func (r Row) CSV() []string {
	return []string{
		r.AdID,
		r.Title,
		r.Price,
		r.Year,
		r.KM,
		r.Fuel,
		r.Transmission,
		r.Owners,
		r.Variant,
		r.Seller,
		r.Location,
		r.CreatedAt,
		r.ListingURL,
	}
}
