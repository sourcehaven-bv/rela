package dataentry

import (
	"sync/atomic"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/openapi"
)

// Schema is the co-derived reload core of the data-entry app: the config and
// metamodel, plus everything that is a pure function of the two (the style map
// and the OpenAPI generator). These MUST move together — the style map is
// derived from (Cfg, Meta), so publishing them independently could let a reader
// observe a new metamodel with a stale style map.
//
// It is the residue of the former AppState after the independently-owned state
// (logo, palette, user defaults) moved into their own self-synchronized
// services. Readers Load a coherent Schema once per request via
// [schemaProvider.Current] (exposed on App as State()).
type Schema struct {
	Cfg         *Config
	Meta        *metamodel.Metamodel
	StyleMap    map[string]map[string]string
	StyledTypes map[string]bool
	OpenAPIGen  *openapi.Generator
}

// schemaProvider publishes the current [Schema] via an atomic.Pointer: readers
// Load lock-free, the watcher's reload path swaps a freshly-derived Schema in
// atomically. Owning the face here (rather than as a bare field on App)
// keeps the state-publish mechanics and the reload derivation in one place.
type schemaProvider struct {
	ptr atomic.Pointer[Schema]
}

// Current returns the published Schema snapshot. Nil only before the initial
// Publish (never in a running App).
func (p *schemaProvider) Current() *Schema { return p.ptr.Load() }

// Publish installs the initial snapshot. Called once during App construction.
func (p *schemaProvider) Publish(s *Schema) { p.ptr.Store(s) }

// Reload derives a fresh Schema from the (possibly changed) config and
// metamodel and publishes it atomically. The style map and OpenAPI generator
// are recomputed only when config or metamodel changed; otherwise the previous
// derivations are carried forward unchanged. Returns the new Schema so the
// caller can drive dependent recomputation (e.g. the palette) off the same
// values.
//
// The OpenAPI generator is internally synchronized and reused in place
// (UpdateMetamodel), matching the previous behavior.
func (p *schemaProvider) Reload(newCfg *Config, newMeta *metamodel.Metamodel, derive bool) *Schema {
	current := p.ptr.Load()
	next := &Schema{
		Cfg:         newCfg,
		Meta:        newMeta,
		StyleMap:    current.StyleMap,
		StyledTypes: current.StyledTypes,
		OpenAPIGen:  current.OpenAPIGen,
	}
	if derive {
		next.StyleMap, next.StyledTypes = buildStyleMap(newCfg, newMeta)
		if next.OpenAPIGen != nil {
			next.OpenAPIGen.UpdateMetamodel(newMeta)
		}
	}
	p.ptr.Store(next)
	return next
}
