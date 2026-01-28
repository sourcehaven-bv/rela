package views

// File represents the complete views.yaml file structure
type File struct {
	Views map[string]ViewDef `yaml:"views"`
}

// ViewDef defines a single view for context generation
type ViewDef struct {
	Description     string             `yaml:"description,omitempty"`
	Entry           EntryDef           `yaml:"entry"`
	Output          OutputDef          `yaml:"output,omitempty"`
	Traverse        []TraverseRule     `yaml:"traverse,omitempty"`
	Filters         map[string]Filter  `yaml:"filters,omitempty"`
	Derived         map[string]Derived `yaml:"derived,omitempty"`
	RelationExports []RelationExport   `yaml:"relation_exports,omitempty"`
}

// EntryDef defines the entry point for a view
type EntryDef struct {
	Type      string `yaml:"type"`
	Parameter string `yaml:"parameter"`
}

// OutputDef defines output options for a view
type OutputDef struct {
	IncludeContent        bool `yaml:"include_content,omitempty"`
	ResolveRelationTitles bool `yaml:"resolve_relation_titles,omitempty"`
	IncludeEntry          bool `yaml:"include_entry,omitempty"`
}

// TraverseRule defines how to traverse the graph from a collection
type TraverseRule struct {
	From           interface{} `yaml:"from"` // string or []string
	Follow         string      `yaml:"follow,omitempty"`
	FollowIncoming string      `yaml:"follow_incoming,omitempty"`
	CollectAs      interface{} `yaml:"collect_as"` // string or []string
	Recursive      bool        `yaml:"recursive,omitempty"`
	MaxDepth       int         `yaml:"max_depth,omitempty"`
	Where          string      `yaml:"where,omitempty"`
}

// GetFromCollections returns the from field as a slice of strings
func (t *TraverseRule) GetFromCollections() []string {
	switch v := t.From.(type) {
	case string:
		return []string{v}
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case []string:
		return v
	default:
		return []string{}
	}
}

// GetCollectAsNames returns the collect_as field as a slice of strings
func (t *TraverseRule) GetCollectAsNames() []string {
	switch v := t.CollectAs.(type) {
	case string:
		return []string{v}
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case []string:
		return v
	default:
		return []string{}
	}
}

// Filter defines filtering criteria for a collection
type Filter struct {
	ViaTraversal bool     `yaml:"via_traversal,omitempty"`
	IDPrefix     []string `yaml:"id_prefix,omitempty"`
	Where        string   `yaml:"where,omitempty"`
	MatchAny     []Filter `yaml:"match_any,omitempty"`
}

// Derived defines a derived collection (group_by, where, embed)
type Derived struct {
	Source  string      `yaml:"source"`
	GroupBy string      `yaml:"group_by,omitempty"`
	Where   string      `yaml:"where,omitempty"`
	Embed   []EmbedRule `yaml:"embed,omitempty"`
}

// EmbedRule defines how to embed related entities into a collection
type EmbedRule struct {
	Relation string   `yaml:"relation"`
	Target   string   `yaml:"target"`
	As       string   `yaml:"as"`
	Include  []string `yaml:"include,omitempty"`
}

// RelationExport defines which relations to export as a separate collection
type RelationExport struct {
	Types     []string `yaml:"types"`
	Between   []string `yaml:"between,omitempty"`
	CollectAs string   `yaml:"collect_as"`
}
