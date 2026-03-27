package views

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// Load reads and parses a views file from a YAML file using the given filesystem.
func Load(path string, fs storage.FS) (*File, error) {
	// Check if file exists
	if _, err := fs.Stat(path); os.IsNotExist(err) {
		// Views file is optional, return empty views
		return &File{Views: make(map[string]ViewDef)}, nil
	}

	data, err := fs.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read views file: %w", err)
	}

	return Parse(data)
}

// Parse parses views YAML content (v1 format)
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

// LoadV2 reads and parses a views file in v2 format using the given filesystem.
func LoadV2(path string, fs storage.FS) (*FileV2, error) {
	// Check if file exists
	if _, err := fs.Stat(path); os.IsNotExist(err) {
		// Views file is optional, return empty views
		return &FileV2{Views: make(map[string]*ViewDefV2)}, nil
	}

	data, err := fs.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read views file: %w", err)
	}

	return ParseV2(data)
}

// ParseV2 parses views YAML content in v2 format
func ParseV2(data []byte) (*FileV2, error) {
	var vf FileV2
	if err := yaml.Unmarshal(data, &vf); err != nil {
		return nil, fmt.Errorf("failed to parse views YAML: %w", err)
	}

	if vf.Views == nil {
		vf.Views = make(map[string]*ViewDefV2)
	}

	return &vf, nil
}

// IsV2Format checks if the views data is in v2 format.
// V2 format is detected by the presence of entry_type at the view level
// (v1 uses entry.type instead).
func IsV2Format(data []byte) bool {
	// Parse into a generic structure to detect format
	var generic struct {
		Views map[string]struct {
			EntryType string `yaml:"entry_type"`
			Entry     struct {
				Type string `yaml:"type"`
			} `yaml:"entry"`
		} `yaml:"views"`
	}

	if err := yaml.Unmarshal(data, &generic); err != nil {
		return false
	}

	// Check each view - if any has entry_type, it's v2
	for _, view := range generic.Views {
		if view.EntryType != "" {
			return true
		}
		// If it has entry.type, it's v1
		if view.Entry.Type != "" {
			return false
		}
	}

	// Empty or ambiguous - default to v1
	return false
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
