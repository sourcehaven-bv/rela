package views

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Load reads and parses a views file from a YAML file
func Load(path string) (*File, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Views file is optional, return empty views
		return &File{Views: make(map[string]ViewDef)}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read views file: %w", err)
	}

	return Parse(data)
}

// Parse parses views YAML content
func Parse(data []byte) (*File, error) {
	var vf File
	if err := yaml.Unmarshal(data, &vf); err != nil {
		return nil, fmt.Errorf("failed to parse views YAML: %w", err)
	}

	if vf.Views == nil {
		vf.Views = make(map[string]ViewDef)
	}

	return &vf, nil
}

// GetView returns a view definition by name
func (vf *File) GetView(name string) (ViewDef, bool) {
	view, ok := vf.Views[name]
	return view, ok
}

// ViewNames returns all view names
func (vf *File) ViewNames() []string {
	names := make([]string, 0, len(vf.Views))
	for name := range vf.Views {
		names = append(names, name)
	}
	return names
}
