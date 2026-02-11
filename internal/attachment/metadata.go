package attachment

import (
	"time"

	"gopkg.in/yaml.v3"
)

// Metadata contains information about an attachment file.
type Metadata struct {
	OriginalName string    `yaml:"original-name"`
	ContentType  string    `yaml:"content-type"`
	Size         int64     `yaml:"size"`
	Added        time.Time `yaml:"added"`
	AddedBy      string    `yaml:"added-by,omitempty"`
}

// MarshalMetadata serializes metadata to YAML bytes.
func MarshalMetadata(m *Metadata) ([]byte, error) {
	return yaml.Marshal(m)
}

// UnmarshalMetadata deserializes metadata from YAML bytes.
func UnmarshalMetadata(data []byte) (*Metadata, error) {
	var m Metadata
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
