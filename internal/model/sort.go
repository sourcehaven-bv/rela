package model

// SortSpec describes a single sort criterion used in queries, metamodel default_sort,
// and data entry config. Property can be a real entity property name or a virtual
// property: "id" (entity ID) or "modified" (file modification time).
type SortSpec struct {
	Property  string `yaml:"property"  json:"property"`
	Direction string `yaml:"direction,omitempty" json:"direction,omitempty"` // "asc" (default) or "desc"
}

// IsDescending returns true if direction is "desc".
func (s SortSpec) IsDescending() bool {
	return s.Direction == "desc"
}
