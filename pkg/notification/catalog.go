package notification

import (
	_ "embed"
	"encoding/json"
)

//go:embed catalog.json
var catalogJSON []byte

type Preset struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Template    string `json:"template"`
}

type Variable struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Group       string `json:"group"`
}

type TemplateCatalog struct {
	Presets   []Preset   `json:"presets"`
	Variables []Variable `json:"variables"`
}

var Catalog = func() TemplateCatalog {
	var data TemplateCatalog
	if err := json.Unmarshal(catalogJSON, &data); err != nil {
		panic(err)
	}
	if len(data.Presets) == 0 {
		panic("notification template catalog is empty")
	}
	return data
}()

var DefaultTemplate = Catalog.Presets[0].Template
