package pingpresets

import (
	_ "embed"
	"encoding/json"
)

type Preset struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Group     string `json:"group"`
	Region    string `json:"region"`
	Carrier   string `json:"carrier"`
	Family    string `json:"family"`
	Type      string `json:"type"`
	Target    string `json:"target"`
	SourceURL string `json:"source_url"`
	Note      string `json:"note"`
}

type CatalogData struct {
	UpdatedAt   string   `json:"updated_at"`
	Description string   `json:"description"`
	Entries     []Preset `json:"entries"`
}

//go:embed catalog.json
var catalogJSON []byte

var Catalog = func() CatalogData {
	var data CatalogData
	if err := json.Unmarshal(catalogJSON, &data); err != nil {
		panic(err)
	}
	seen := map[string]bool{}
	for _, p := range data.Entries {
		if p.ID == "" || seen[p.ID] || p.Target == "" || (p.Type != "tcp" && p.Type != "icmp") {
			panic("invalid ping preset catalog")
		}
		seen[p.ID] = true
	}
	return data
}()

func Find(id string) (Preset, bool) {
	for _, preset := range Catalog.Entries {
		if preset.ID == id {
			return preset, true
		}
	}
	return Preset{}, false
}
