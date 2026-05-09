package openapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Config holds configuration for spec generation.
type Config struct {
	Title       string // API title (defaults to "Rela API")
	Description string // API description
	Version     string // API version (defaults to "1.0.0")
	ServerURL   string // Server URL (optional)
}

// Generator builds OpenAPI specs from metamodels.
type Generator struct {
	meta *metamodel.Metamodel
	cfg  Config

	// Caching
	mu         sync.RWMutex
	cachedSpec *Spec
	cachedJSON []byte
	cachedHash string
}

// New creates a new OpenAPI generator for the given metamodel.
func New(meta *metamodel.Metamodel, cfg Config) *Generator {
	if cfg.Title == "" {
		cfg.Title = "Rela API"
	}
	if cfg.Version == "" {
		cfg.Version = "1.0.0"
	}
	return &Generator{
		meta: meta,
		cfg:  cfg,
	}
}

// Generate returns the OpenAPI spec, using cached version if metamodel unchanged.
func (g *Generator) Generate() *Spec {
	hash := g.computeMetamodelHash()

	g.mu.RLock()
	if g.cachedSpec != nil && g.cachedHash == hash {
		spec := g.cachedSpec
		g.mu.RUnlock()
		return spec
	}
	g.mu.RUnlock()

	// Generate new spec
	spec := g.generate()

	g.mu.Lock()
	g.cachedSpec = spec
	g.cachedHash = hash
	g.cachedJSON = nil // Invalidate JSON cache
	g.mu.Unlock()

	return spec
}

// GenerateJSON returns the OpenAPI spec as JSON bytes, cached.
func (g *Generator) GenerateJSON() ([]byte, error) {
	hash := g.computeMetamodelHash()

	g.mu.RLock()
	if g.cachedJSON != nil && g.cachedHash == hash {
		data := g.cachedJSON
		g.mu.RUnlock()
		return data, nil
	}
	g.mu.RUnlock()

	// Generate spec and marshal to JSON
	spec := g.Generate()
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return nil, err
	}

	g.mu.Lock()
	g.cachedJSON = data
	g.mu.Unlock()

	return data, nil
}

// Invalidate clears the cached spec (call when metamodel changes).
func (g *Generator) Invalidate() {
	g.mu.Lock()
	g.cachedSpec = nil
	g.cachedJSON = nil
	g.cachedHash = ""
	g.mu.Unlock()
}

// UpdateMetamodel updates the metamodel reference and invalidates cache.
func (g *Generator) UpdateMetamodel(meta *metamodel.Metamodel) {
	g.mu.Lock()
	g.meta = meta
	g.cachedSpec = nil
	g.cachedJSON = nil
	g.cachedHash = ""
	g.mu.Unlock()
}

// generate builds a fresh OpenAPI spec from the metamodel.
func (g *Generator) generate() *Spec {
	spec := &Spec{
		OpenAPI: "3.1.0",
		Info: Info{
			Title:       g.cfg.Title,
			Description: g.cfg.Description,
			Version:     g.cfg.Version,
		},
		Paths: make(map[string]PathItem),
		Components: &Components{
			Schemas: make(map[string]*Schema),
		},
	}

	// Add server if configured
	if g.cfg.ServerURL != "" {
		spec.Servers = []Server{{URL: g.cfg.ServerURL}}
	}

	// Add system paths (static endpoints)
	g.addSystemPaths(spec)

	// Add entity paths (dynamic from metamodel)
	entityTypes := g.meta.EntityTypes()
	sort.Strings(entityTypes) // Deterministic order
	for _, typeName := range entityTypes {
		def := g.meta.Entities[typeName]
		g.addEntityPaths(spec, typeName, def)
	}

	// Add common schemas
	g.addCommonSchemas(spec)

	return spec
}

// computeMetamodelHash computes a hash of the metamodel for cache invalidation.
// This includes entity types, their properties, relations, and custom types.
func (g *Generator) computeMetamodelHash() string {
	h := sha256.New()

	// Hash entity types (sorted for determinism)
	entityTypes := g.meta.EntityTypes()
	sort.Strings(entityTypes)
	for _, typeName := range entityTypes {
		h.Write([]byte("entity:" + typeName))
		def := g.meta.Entities[typeName]

		// Hash entity metadata
		h.Write([]byte(def.Label))
		h.Write([]byte(def.Description))
		h.Write([]byte(def.GetPlural(typeName)))

		// Hash properties (sorted)
		propNames := make([]string, 0, len(def.Properties))
		for name := range def.Properties {
			propNames = append(propNames, name)
		}
		sort.Strings(propNames)
		for _, name := range propNames {
			prop := def.Properties[name]
			h.Write([]byte("prop:" + name))
			h.Write([]byte(prop.Type))
			if prop.Required {
				h.Write([]byte("required"))
			}
			if prop.List {
				h.Write([]byte("list"))
			}
			for _, v := range prop.Values {
				h.Write([]byte("value:" + v))
			}
		}
	}

	// Hash relation types (sorted)
	relNames := make([]string, 0, len(g.meta.Relations))
	for name := range g.meta.Relations {
		relNames = append(relNames, name)
	}
	sort.Strings(relNames)
	for _, name := range relNames {
		rel := g.meta.Relations[name]
		h.Write([]byte("relation:" + name))
		h.Write([]byte(rel.Label))
		for _, f := range rel.From {
			h.Write([]byte("from:" + f))
		}
		for _, t := range rel.To {
			h.Write([]byte("to:" + t))
		}
	}

	// Hash custom types (sorted)
	typeNames := make([]string, 0, len(g.meta.Types))
	for name := range g.meta.Types {
		typeNames = append(typeNames, name)
	}
	sort.Strings(typeNames)
	for _, name := range typeNames {
		ct := g.meta.Types[name]
		h.Write([]byte("type:" + name))
		for _, v := range ct.Values {
			h.Write([]byte("value:" + v))
		}
	}

	return hex.EncodeToString(h.Sum(nil))
}
