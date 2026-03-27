package views

// QueryNode represents a node in the query-as-output-structure view tree.
// Each node defines how to traverse the graph and what to include in output.
// The structure of QueryNodes mirrors the desired output structure.
type QueryNode struct {
	// Entry point fields (only valid at root level)
	EntryType string `yaml:"entry_type,omitempty"` // Entity type for entry point
	Param     string `yaml:"param,omitempty"`      // Parameter name for entry ID

	// Traversal configuration
	Via         string   `yaml:"via,omitempty"`          // Follow outgoing relation
	ViaIncoming string   `yaml:"via_incoming,omitempty"` // Follow incoming relation
	Types       []string `yaml:"types,omitempty"`        // Filter by entity types
	Recursive   int      `yaml:"recursive,omitempty"`    // Max recursion depth (0 = not recursive)

	// Filtering
	Where   string            `yaml:"where,omitempty"`   // Property filter expression
	Require map[string]string `yaml:"require,omitempty"` // Scope filter: relation -> JSONPath

	// Output control
	Only    []string `yaml:"only,omitempty"`    // Properties to include (nil = all)
	Content *bool    `yaml:"content,omitempty"` // Include content (nil = true)
	Props   *bool    `yaml:"props,omitempty"`   // Include props block in output (nil = true)

	// Children - nested relation traversals
	Relations map[string]*QueryNode `yaml:"relations,omitempty"`
}

// ViewDefV2 defines a view using the query-as-output-structure format.
// The view definition itself is the root QueryNode with additional metadata.
type ViewDefV2 struct {
	QueryNode   `yaml:",inline"`
	Description string `yaml:"description,omitempty"`
}

// FileV2 represents a views.yaml file using the v2 format.
type FileV2 struct {
	Views map[string]*ViewDefV2 `yaml:"views"`
}

// IncludeContent returns whether content should be included (default: true).
func (q *QueryNode) IncludeContent() bool {
	if q.Content == nil {
		return true
	}
	return *q.Content
}

// IncludeProps returns whether props block should be included (default: true).
func (q *QueryNode) IncludeProps() bool {
	if q.Props == nil {
		return true
	}
	return *q.Props
}

// IsRecursive returns whether this node has recursive traversal enabled.
func (q *QueryNode) IsRecursive() bool {
	return q.Recursive > 0
}

// HasChildren returns whether this node has child relations defined.
func (q *QueryNode) HasChildren() bool {
	return len(q.Relations) > 0
}

// IsRoot returns whether this node is a root node (has entry_type defined).
func (q *QueryNode) IsRoot() bool {
	return q.EntryType != ""
}

// GetView returns a v2 view definition by name.
func (f *FileV2) GetView(name string) (*ViewDefV2, bool) {
	if f.Views == nil {
		return nil, false
	}
	view, ok := f.Views[name]
	return view, ok
}

// ViewNames returns all view names in the file in sorted order.
func (f *FileV2) ViewNames() []string {
	return f.SortedViewNames()
}
