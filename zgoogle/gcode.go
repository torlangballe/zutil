package zgoogle

type AddressComponent struct {
	LongName  string   `json:"long_name"`
	ShortName string   `json:"short_name"`
	Types     []string `json:"types"`
}

type Location struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type Viewport struct {
	Northeast Location `json:"northeast"`
	Southwest Location `json:"southwest"`
}

type Bound struct {
	Northeast Location `json:"northeast"`
	Southwest Location `json:"southwest"`
}

type Geometry struct {
	Location     Location `json:"location"`
	LocationType string   `json:"location_type"`
	Viewport     Viewport `json:"viewport"`
	Bound        Bound    `json:"bounds"`
}

type Result struct {
	Components []AddressComponent `json:"address_components"`
	Formatted  string             `json:"formatted_address"`
	Geo        Geometry           `json:"geometry"`
	Types      []string           `json:"types"`
}

type Geocode struct {
	Results []Result `json:"results"`
	Status  string   `json:"status"`
}
