// Package importer provides functionality to import entities and relations
// from JSON, YAML, and CSV files into rela projects.
package importer

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// Format represents an import file format
type Format string

const (
	FormatJSON Format = "json"
	FormatYAML Format = "yaml"
	FormatCSV  Format = "csv"
)

// Options configures the import behavior
type Options struct {
	// Format specifies the input format. If empty, auto-detected from file extension.
	Format Format

	// DryRun validates without creating files
	DryRun bool

	// Update allows updating existing entities instead of failing on duplicates
	Update bool

	// SkipErrors continues importing on validation errors
	SkipErrors bool

	// RelationsFile is the path to a separate relations CSV file (for CSV imports)
	RelationsFile string
}

// Result contains the outcome of an import operation
type Result struct {
	EntitiesCreated  int
	EntitiesUpdated  int
	EntitiesSkipped  int
	RelationsCreated int
	RelationsSkipped int
	Errors           []ImportError
}

// ImportError represents an error during import with context
type ImportError struct {
	Type    string // "entity" or "relation"
	ID      string // entity ID or relation key
	Message string
}

func (e ImportError) Error() string {
	return fmt.Sprintf("%s %s: %s", e.Type, e.ID, e.Message)
}

// ImportData represents the parsed import data
type ImportData struct {
	Entities  []EntityData
	Relations []RelationData
}

// EntityData represents an entity to import
type EntityData struct {
	ID         string                 `json:"id" yaml:"id"`
	Type       string                 `json:"type" yaml:"type"`
	Properties map[string]interface{} `json:"properties,omitempty" yaml:"properties,omitempty"`
}

// RelationData represents a relation to import
type RelationData struct {
	From       string                 `json:"from" yaml:"from"`
	Relation   string                 `json:"relation" yaml:"relation"`
	To         string                 `json:"to" yaml:"to"`
	Properties map[string]interface{} `json:"properties,omitempty" yaml:"properties,omitempty"`
}

// ImportSource provides filesystem access for reading import input files.
// It wraps a storage.FS to make the intent explicit: the FS is for reading
// source data (CSV, JSON, YAML), which may be on a different filesystem
// than the project's repository.
type ImportSource struct {
	fs storage.FS
}

// NewImportSource creates an ImportSource from a filesystem.
func NewImportSource(fs storage.FS) *ImportSource {
	return &ImportSource{fs: fs}
}

// Open opens a file for reading from the import source.
func (s *ImportSource) Open(path string) (io.ReadCloser, error) {
	return s.fs.Open(path)
}

// Importer handles importing data into a rela project
type Importer struct {
	store  store.Store
	meta   *metamodel.Metamodel
	opts   Options
	source *ImportSource
}

// New creates a new Importer that reads input files from the given source.
func New(
	s store.Store, meta *metamodel.Metamodel, opts Options, source *ImportSource,
) *Importer {
	return &Importer{
		store:  s,
		meta:   meta,
		opts:   opts,
		source: source,
	}
}

// ImportFile imports data from a file
func (imp *Importer) ImportFile(path string) (*Result, error) {
	format := imp.opts.Format
	if format == "" {
		format = detectFormat(path)
		if format == "" {
			return nil, fmt.Errorf("cannot determine format for file: %s (use --format to specify)", path)
		}
	}

	file, err := imp.source.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var data *ImportData
	switch format {
	case FormatJSON:
		data, err = parseJSON(file)
	case FormatYAML:
		data, err = parseYAML(file)
	case FormatCSV:
		data, err = parseCSV(file)
		// Handle separate relations file for CSV
		if err == nil && imp.opts.RelationsFile != "" {
			relData, relErr := imp.parseRelationsCSV(imp.opts.RelationsFile)
			if relErr != nil {
				return nil, fmt.Errorf("failed to parse relations file: %w", relErr)
			}
			data.Relations = relData
		}
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", format, err)
	}

	return imp.Import(data)
}

// Import imports the parsed data
func (imp *Importer) Import(data *ImportData) (*Result, error) {
	result := &Result{}

	// Phase 1: Validate all entities
	validEntities, err := imp.validateEntities(data.Entities, result)
	if err != nil {
		return result, err
	}

	// Phase 2: Validate all relations
	validRelations, err := imp.validateRelations(data.Relations, validEntities, result)
	if err != nil {
		return result, err
	}

	// If dry run, stop here
	if imp.opts.DryRun {
		result.EntitiesCreated = len(validEntities)
		result.RelationsCreated = len(validRelations)
		return result, nil
	}

	// Phase 3: Create/update entities
	if err := imp.createEntities(validEntities, result); err != nil {
		return result, err
	}

	// Phase 4: Create relations
	if err := imp.createRelations(validRelations, result); err != nil {
		return result, err
	}

	return result, nil
}

// validateEntities validates all entities and returns valid ones
func (imp *Importer) validateEntities(entities []EntityData, result *Result) ([]EntityData, error) {
	valid := make([]EntityData, 0, len(entities))
	for _, ed := range entities {
		if err := imp.validateEntityData(&ed); err != nil {
			impErr := ImportError{Type: "entity", ID: ed.ID, Message: err.Error()}
			if imp.opts.SkipErrors {
				result.Errors = append(result.Errors, impErr)
				result.EntitiesSkipped++
				continue
			}
			return valid, &impErr
		}
		valid = append(valid, ed)
	}
	return valid, nil
}

// validateRelations validates all relations and returns valid ones
func (imp *Importer) validateRelations(
	relations []RelationData, validEntities []EntityData, result *Result,
) ([]RelationData, error) {
	// Build set of known entity IDs
	ctx := context.Background()
	entityIDs := make(map[string]bool)
	for _, ed := range validEntities {
		entityIDs[ed.ID] = true
	}
	for e, err := range imp.store.ListEntities(ctx, store.EntityQuery{}) {
		if err != nil {
			break
		}
		entityIDs[e.ID] = true
	}

	valid := make([]RelationData, 0, len(relations))
	for _, rd := range relations {
		if err := imp.validateRelationData(&rd, entityIDs); err != nil {
			impErr := ImportError{Type: "relation", ID: rd.From + "--" + rd.Relation + "--" + rd.To, Message: err.Error()}
			if imp.opts.SkipErrors {
				result.Errors = append(result.Errors, impErr)
				result.RelationsSkipped++
				continue
			}
			return valid, &impErr
		}
		valid = append(valid, rd)
	}
	return valid, nil
}

// createEntities creates or updates validated entities
func (imp *Importer) createEntities(entities []EntityData, result *Result) error {
	for _, ed := range entities {
		created, err := imp.importEntity(&ed)
		if err != nil {
			impErr := ImportError{Type: "entity", ID: ed.ID, Message: err.Error()}
			if imp.opts.SkipErrors {
				result.Errors = append(result.Errors, impErr)
				result.EntitiesSkipped++
				continue
			}
			return &impErr
		}
		if created {
			result.EntitiesCreated++
		} else {
			result.EntitiesUpdated++
		}
	}
	return nil
}

// createRelations creates validated relations
func (imp *Importer) createRelations(relations []RelationData, result *Result) error {
	for _, rd := range relations {
		created, err := imp.importRelation(&rd)
		if err != nil {
			impErr := ImportError{Type: "relation", ID: rd.From + "--" + rd.Relation + "--" + rd.To, Message: err.Error()}
			if imp.opts.SkipErrors {
				result.Errors = append(result.Errors, impErr)
				result.RelationsSkipped++
				continue
			}
			return &impErr
		}
		if created {
			result.RelationsCreated++
		} else {
			result.RelationsSkipped++
		}
	}
	return nil
}

// validateEntityData validates entity data before import
func (imp *Importer) validateEntityData(ed *EntityData) error {
	// ID is required
	if ed.ID == "" {
		return errors.New("missing required field: id")
	}
	if err := storeutil.ValidateID(ed.ID); err != nil {
		return err
	}

	// Type is required
	if ed.Type == "" {
		return errors.New("missing required field: type")
	}

	// Resolve type alias
	ed.Type = imp.meta.ResolveAlias(ed.Type)

	// Check type exists
	entityDef, ok := imp.meta.GetEntityDef(ed.Type)
	if !ok {
		return fmt.Errorf("unknown entity type: %s", ed.Type)
	}

	// Check if entity already exists
	if _, err := imp.store.GetEntity(context.Background(), ed.ID); err == nil {
		if !imp.opts.Update {
			return errors.New("entity already exists (use --update to overwrite)")
		}
	}

	// Build entity for validation
	e := entity.New(ed.ID, ed.Type)
	for k, v := range ed.Properties {
		e.Properties[k] = v
	}

	// Apply default status if not provided
	if _, hasStatus := e.Properties["status"]; !hasStatus {
		defaultStatus := entityDef.GetDefaultStatus(imp.meta)
		if defaultStatus != "" {
			e.Properties["status"] = defaultStatus
		}
	}

	// Validate against metamodel
	errs := imp.meta.ValidateEntity(e.ID, e.Type, e.Properties)
	if len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		return fmt.Errorf("validation failed: %s", strings.Join(msgs, "; "))
	}

	return nil
}

// validateRelationData validates relation data before import
func (imp *Importer) validateRelationData(rd *RelationData, knownIDs map[string]bool) error {
	if rd.From == "" {
		return errors.New("missing required field: from")
	}
	if rd.Relation == "" {
		return errors.New("missing required field: relation")
	}
	if rd.To == "" {
		return errors.New("missing required field: to")
	}

	// Check entities exist (either in graph or in import batch)
	if !knownIDs[rd.From] {
		return fmt.Errorf("source entity not found: %s", rd.From)
	}
	if !knownIDs[rd.To] {
		return fmt.Errorf("target entity not found: %s", rd.To)
	}

	// Get entity types for relation validation
	ctx := context.Background()
	var fromType, toType string
	if e, err := imp.store.GetEntity(ctx, rd.From); err == nil {
		fromType = e.Type
	} else {
		// Must be in the import batch - we'll validate after entities are created
		// For now, skip metamodel validation
		return nil
	}
	if e, err := imp.store.GetEntity(ctx, rd.To); err == nil {
		toType = e.Type
	} else {
		return nil
	}

	// Validate relation against metamodel
	return imp.meta.ValidateRelation(rd.Relation, fromType, toType)
}

// importEntity creates or updates an entity
func (imp *Importer) importEntity(ed *EntityData) (created bool, err error) {
	entityDef, _ := imp.meta.GetEntityDef(ed.Type)
	ctx := context.Background()

	e := entity.New(ed.ID, ed.Type)
	for k, v := range ed.Properties {
		e.Properties[k] = v
	}

	// Apply default status if not provided
	if _, hasStatus := e.Properties["status"]; !hasStatus {
		defaultStatus := entityDef.GetDefaultStatus(imp.meta)
		if defaultStatus != "" {
			e.Properties["status"] = defaultStatus
		}
	}

	// Check if updating
	_, getErr := imp.store.GetEntity(ctx, ed.ID)
	exists := getErr == nil

	if exists {
		if err := imp.store.UpdateEntity(ctx, e); err != nil {
			return false, fmt.Errorf("failed to update entity: %w", err)
		}
	} else {
		if err := imp.store.CreateEntity(ctx, e); err != nil {
			return false, fmt.Errorf("failed to create entity: %w", err)
		}
	}

	return !exists, nil
}

// importRelation creates a relation
func (imp *Importer) importRelation(rd *RelationData) (created bool, err error) {
	ctx := context.Background()

	// Check if relation already exists
	if _, err := imp.store.GetRelation(ctx, rd.From, rd.Relation, rd.To); err == nil {
		return false, nil
	}

	var data *store.RelationData
	if len(rd.Properties) > 0 {
		data = &store.RelationData{Properties: rd.Properties}
	}

	if _, err := imp.store.CreateRelation(ctx, rd.From, rd.Relation, rd.To, data); err != nil {
		return false, fmt.Errorf("failed to create relation: %w", err)
	}

	return true, nil
}

// detectFormat detects the file format from extension
func detectFormat(path string) Format {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return FormatJSON
	case ".yaml", ".yml":
		return FormatYAML
	case ".csv":
		return FormatCSV
	default:
		return ""
	}
}

// parseJSON parses JSON import data
func parseJSON(r io.Reader) (*ImportData, error) {
	var raw json.RawMessage
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, err
	}

	// Try full format first (with entities and relations keys)
	var full struct {
		Entities  []EntityData   `json:"entities"`
		Relations []RelationData `json:"relations"`
	}
	if err := json.Unmarshal(raw, &full); err == nil {
		// Accept the full format even if empty (valid structure)
		if full.Entities != nil || full.Relations != nil {
			if len(full.Entities) == 0 && len(full.Relations) == 0 {
				return nil, errors.New("no entities or relations to import")
			}
			return &ImportData{
				Entities:  full.Entities,
				Relations: full.Relations,
			}, nil
		}
	}

	// Try array of entities
	var entities []EntityData
	if err := json.Unmarshal(raw, &entities); err == nil {
		if len(entities) == 0 {
			return nil, errors.New("no entities to import")
		}
		return &ImportData{Entities: entities}, nil
	}

	return nil, errors.New("invalid JSON format: expected object with 'entities' key or array of entities")
}

// parseYAML parses YAML import data
func parseYAML(r io.Reader) (*ImportData, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// Try full format first
	var full struct {
		Entities  []EntityData   `yaml:"entities"`
		Relations []RelationData `yaml:"relations"`
	}
	if err := yaml.Unmarshal(content, &full); err == nil {
		// Check if this looks like the full format structure
		if full.Entities != nil || full.Relations != nil {
			if len(full.Entities) == 0 && len(full.Relations) == 0 {
				return nil, errors.New("no entities or relations to import")
			}
			return &ImportData{
				Entities:  full.Entities,
				Relations: full.Relations,
			}, nil
		}
	}

	// Try array of entities
	var entities []EntityData
	if err := yaml.Unmarshal(content, &entities); err == nil {
		if len(entities) == 0 {
			return nil, errors.New("no entities to import")
		}
		return &ImportData{Entities: entities}, nil
	}

	return nil, errors.New("invalid YAML format: expected object with 'entities' key or array of entities")
}

// parseCSV parses CSV import data (entities only)
func parseCSV(r io.Reader) (*ImportData, error) {
	reader := csv.NewReader(r)

	// Read header
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Build column index
	colIndex := make(map[string]int)
	for i, col := range header {
		colIndex[strings.ToLower(strings.TrimSpace(col))] = i
	}

	// Require id and type columns
	idCol, hasID := colIndex["id"]
	typeCol, hasType := colIndex["type"]
	if !hasID {
		return nil, errors.New("CSV must have 'id' column")
	}
	if !hasType {
		return nil, errors.New("CSV must have 'type' column")
	}

	// Read rows - estimate capacity from file size
	entities := make([]EntityData, 0, 100)
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV row: %w", err)
		}

		ed := EntityData{
			ID:         strings.TrimSpace(row[idCol]),
			Type:       strings.TrimSpace(row[typeCol]),
			Properties: make(map[string]interface{}),
		}

		// Add other columns as properties
		for col, idx := range colIndex {
			if col == "id" || col == "type" || idx >= len(row) {
				continue
			}
			value := strings.TrimSpace(row[idx])
			if value != "" {
				ed.Properties[col] = value
			}
		}

		entities = append(entities, ed)
	}

	return &ImportData{Entities: entities}, nil
}

// parseRelationsCSV parses a relations CSV file
func (imp *Importer) parseRelationsCSV(path string) ([]RelationData, error) {
	file, err := imp.source.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)

	// Read header
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Build column index
	colIndex := make(map[string]int)
	for i, col := range header {
		colIndex[strings.ToLower(strings.TrimSpace(col))] = i
	}

	// Require from, relation/type, to columns
	fromCol, hasFrom := colIndex["from"]
	toCol, hasTo := colIndex["to"]
	relCol, hasRel := colIndex["relation"]
	if !hasRel {
		relCol, hasRel = colIndex["type"]
	}

	if !hasFrom || !hasTo || !hasRel {
		return nil, errors.New("relations CSV must have 'from', 'relation' (or 'type'), and 'to' columns")
	}

	// Read rows
	relations := make([]RelationData, 0, 100)
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV row: %w", err)
		}

		rd := RelationData{
			From:     strings.TrimSpace(row[fromCol]),
			Relation: strings.TrimSpace(row[relCol]),
			To:       strings.TrimSpace(row[toCol]),
		}

		relations = append(relations, rd)
	}

	return relations, nil
}
