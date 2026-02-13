package transclusion

import (
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/graph"
)

// DefaultMaxDepth is the default maximum transclusion depth.
const DefaultMaxDepth = 10

// Resolver handles transclusion resolution.
type Resolver struct {
	graph    *graph.Graph
	maxDepth int
}

// NewResolver creates a new transclusion resolver.
func NewResolver(g *graph.Graph) *Resolver {
	return &Resolver{
		graph:    g,
		maxDepth: DefaultMaxDepth,
	}
}

// WithMaxDepth sets the maximum transclusion depth.
func (r *Resolver) WithMaxDepth(depth int) *Resolver {
	r.maxDepth = depth
	return r
}

// resolveState tracks resolution state for cycle detection.
type resolveState struct {
	inStack map[string]bool   // Entities currently being resolved (cycle detection)
	cache   map[string]string // Cache of resolved content
	chain   []string          // Current resolution chain for error messages
}

// Resolve resolves all transclusions in content.
// Returns the resolved content with all transclusions expanded.
func (r *Resolver) Resolve(content string) (string, error) {
	if !HasTransclusions(content) {
		return content, nil
	}

	state := &resolveState{
		inStack: make(map[string]bool),
		cache:   make(map[string]string),
		chain:   nil,
	}

	return r.resolveContent(content, state, 0)
}

// ResolveEntity resolves all transclusions in an entity's content.
// The entity's own ID is added to the resolution stack to detect self-references.
func (r *Resolver) ResolveEntity(entityID string) (string, error) {
	entity, ok := r.graph.GetNode(entityID)
	if !ok {
		return "", &EntityNotFoundError{ID: entityID}
	}

	state := &resolveState{
		inStack: map[string]bool{entityID: true},
		cache:   make(map[string]string),
		chain:   []string{entityID},
	}

	return r.resolveContent(entity.Content, state, 0)
}

// resolveContent resolves transclusions in content at the given depth.
func (r *Resolver) resolveContent(content string, state *resolveState, depth int) (string, error) {
	if depth >= r.maxDepth {
		return content, nil // Stop at max depth without error
	}

	transclusions := Parse(content)
	if len(transclusions) == 0 {
		return content, nil
	}

	// Resolve each transclusion
	replacements := make(map[int]string)
	for _, t := range transclusions {
		resolved, err := r.resolveTransclusion(t, state, depth)
		if err != nil {
			return "", err
		}
		replacements[t.Start] = resolved
	}

	// Apply replacements from end to start
	result := content
	for i := len(transclusions) - 1; i >= 0; i-- {
		t := transclusions[i]
		result = result[:t.Start] + replacements[t.Start] + result[t.End:]
	}

	return result, nil
}

// resolveTransclusion resolves a single transclusion reference.
func (r *Resolver) resolveTransclusion(t Transclusion, state *resolveState, depth int) (string, error) {
	// Check for circular reference
	if state.inStack[t.EntityID] {
		chain := make([]string, len(state.chain)+1)
		copy(chain, state.chain)
		chain[len(state.chain)] = t.EntityID
		return "", &CircularTransclusionError{Chain: chain}
	}

	// Check depth limit
	if depth >= r.maxDepth {
		return "", &MaxDepthExceededError{MaxDepth: r.maxDepth, EntityID: t.EntityID}
	}

	// Look up the entity
	entity, ok := r.graph.GetNode(t.EntityID)
	if !ok {
		refFrom := ""
		if len(state.chain) > 0 {
			refFrom = state.chain[len(state.chain)-1]
		}
		return "", &EntityNotFoundError{ID: t.EntityID, ReferencedFrom: refFrom}
	}

	// Get the content to include
	var content string
	if t.Section != "" {
		// Extract specific section
		sectionContent, found := ExtractSection(entity.Content, t.Section)
		if !found {
			return "", &SectionNotFoundError{EntityID: t.EntityID, Section: t.Section}
		}
		content = sectionContent
	} else {
		// Include full content
		content = entity.Content
	}

	// Check cache for already-resolved content
	cacheKey := t.EntityID
	if t.Section != "" {
		cacheKey = t.EntityID + "#" + t.Section
	}
	if cached, ok := state.cache[cacheKey]; ok {
		return cached, nil
	}

	// Add to stack and chain for cycle detection
	state.inStack[t.EntityID] = true
	state.chain = append(state.chain, t.EntityID)

	// Recursively resolve nested transclusions
	resolved, err := r.resolveContent(content, state, depth+1)
	if err != nil {
		return "", err
	}

	// Remove from stack (but keep in chain for error messages)
	delete(state.inStack, t.EntityID)
	state.chain = state.chain[:len(state.chain)-1]

	// Cache the resolved content
	state.cache[cacheKey] = resolved

	return resolved, nil
}

// RenderEntity renders an entity with resolved transclusions and optional formatting.
type RenderOptions struct {
	IncludeFrontmatter bool   // Include YAML frontmatter
	MaxDepth           int    // Maximum transclusion depth (0 = use default)
	StripComments      bool   // Remove HTML comments
	LinkStyle          string // How to render [[links]]: "plain", "emph", "at_file_ref"
}

// DefaultRenderOptions returns sensible defaults for rendering.
func DefaultRenderOptions() RenderOptions {
	return RenderOptions{
		IncludeFrontmatter: false,
		MaxDepth:           DefaultMaxDepth,
		StripComments:      true,
		LinkStyle:          "plain",
	}
}

// RenderEntity renders an entity with resolved transclusions.
func (r *Resolver) RenderEntity(entityID string, opts RenderOptions) (string, error) {
	entity, ok := r.graph.GetNode(entityID)
	if !ok {
		return "", &EntityNotFoundError{ID: entityID}
	}

	// Set max depth if specified
	if opts.MaxDepth > 0 {
		r.maxDepth = opts.MaxDepth
	}

	// Resolve transclusions
	content, err := r.ResolveEntity(entityID)
	if err != nil {
		return "", err
	}

	// Strip HTML comments if requested
	if opts.StripComments {
		content = stripHTMLComments(content)
	}

	// Build result
	var result strings.Builder

	if opts.IncludeFrontmatter {
		result.WriteString("---\n")
		result.WriteString("id: ")
		result.WriteString(entity.ID)
		result.WriteString("\n")
		result.WriteString("type: ")
		result.WriteString(entity.Type)
		result.WriteString("\n")
		for key, value := range entity.Properties {
			result.WriteString(key)
			result.WriteString(": ")
			if s, ok := value.(string); ok {
				// Quote strings with special characters
				if strings.ContainsAny(s, ":\n") {
					result.WriteString("\"")
					result.WriteString(strings.ReplaceAll(s, "\"", "\\\""))
					result.WriteString("\"")
				} else {
					result.WriteString(s)
				}
			} else {
				result.WriteString(formatValue(value))
			}
			result.WriteString("\n")
		}
		result.WriteString("---\n\n")
	}

	result.WriteString(content)

	return result.String(), nil
}

// stripHTMLComments removes HTML comments from content.
func stripHTMLComments(content string) string {
	var result strings.Builder
	i := 0
	for i < len(content) {
		if strings.HasPrefix(content[i:], "<!--") {
			// Find closing -->
			end := strings.Index(content[i:], "-->")
			if end != -1 {
				i += end + 3
				continue
			}
		}
		result.WriteByte(content[i])
		i++
	}
	return result.String()
}

// formatValue formats a value for YAML output.
func formatValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	case int, int64, float64:
		return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(
			strings.ReplaceAll("%v", "%", ""), "v", ""), " ", ""))
	default:
		return ""
	}
}
