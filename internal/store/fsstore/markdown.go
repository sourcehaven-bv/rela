package fsstore

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/frontmatter"
	"github.com/Sourcehaven-BV/rela/internal/markdown"
)

// errConflictedFile is returned when a file has unresolved git conflict markers.
var errConflictedFile = errors.New("file has unresolved git conflicts")

const frontmatterDelimiter = frontmatter.Delimiter

// conflictMarkerStart is git's opening conflict marker. Detection
// MUST be line-anchored — see [hasLineAnchoredConflict] and BUG-WN6D
// for why a substring match false-positives on legitimate content
// (markdown codespans or quoted prose mentioning the marker).
var conflictMarkerStart = []byte("<<<<<<<")

// hasLineAnchoredConflict reports whether `raw` contains the
// conflict marker at column 0 of any line. Returns false for
// substring matches inside inline text — those are not real
// conflicts.
//
// NOTE: a near-duplicate of [markdown.HasConflictMarkers] exists in
// the markdown package. Both live behind the same line-anchored
// semantics; deduplicating into one shared helper is tracked
// separately (the dependency widening of fsstore→markdown is the
// reason for keeping it local for now).
func hasLineAnchoredConflict(raw string) bool {
	idx := strings.Index(raw, string(conflictMarkerStart))
	for idx >= 0 {
		if idx == 0 || raw[idx-1] == '\n' {
			return true
		}
		offset := idx + len(conflictMarkerStart)
		rest := strings.Index(raw[offset:], string(conflictMarkerStart))
		if rest < 0 {
			return false
		}
		idx = offset + rest
	}
	return false
}

// --- document parsing ---

// document represents a parsed markdown file with YAML frontmatter.
type document struct {
	frontmatter map[string]any
	content     string
}

func (d *document) getString(key string) string {
	if d.frontmatter == nil {
		return ""
	}
	if v, ok := d.frontmatter[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// parseDocument parses a markdown file with YAML frontmatter.
func parseDocument(raw string) (*document, error) {
	if hasLineAnchoredConflict(raw) {
		return nil, errConflictedFile
	}

	fm, body := frontmatter.Split(raw)

	var parsed map[string]any
	if fm != "" {
		if err := yaml.Unmarshal([]byte(fm), &parsed); err != nil {
			return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
		}
	}

	return &document{frontmatter: parsed, content: body}, nil
}

// --- document formatting ---

func formatDocumentOrdered(fm map[string]any, content string, keyOrder []string) (string, error) {
	var sb strings.Builder

	if len(fm) > 0 {
		sb.WriteString(frontmatterDelimiter)
		sb.WriteString("\n")

		var yamlBytes []byte
		var err error

		if len(keyOrder) > 0 {
			yamlBytes, err = marshalOrdered(fm, keyOrder)
		} else {
			yamlBytes, err = yaml.Marshal(fm)
		}
		if err != nil {
			return "", err
		}
		sb.Write(yamlBytes)
		sb.WriteString(frontmatterDelimiter)
		sb.WriteString("\n")
	}

	if content != "" {
		sb.WriteString("\n")
		sb.WriteString(content)
		if !strings.HasSuffix(content, "\n") {
			sb.WriteString("\n")
		}
	}

	return sb.String(), nil
}

func marshalOrdered(data map[string]any, keyOrder []string) ([]byte, error) {
	node := &yaml.Node{Kind: yaml.MappingNode}
	added := make(map[string]bool)

	for _, key := range keyOrder {
		val, ok := data[key]
		if !ok {
			continue
		}
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key},
		)
		valNode, err := valueToNode(val)
		if err != nil {
			return nil, err
		}
		node.Content = append(node.Content, valNode)
		added[key] = true
	}

	var remaining []string
	for key := range data {
		if !added[key] {
			remaining = append(remaining, key)
		}
	}
	sort.Strings(remaining)

	for _, key := range remaining {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key},
		)
		valNode, err := valueToNode(data[key])
		if err != nil {
			return nil, err
		}
		node.Content = append(node.Content, valNode)
	}

	return yaml.Marshal(node)
}

func valueToNode(val any) (*yaml.Node, error) {
	var node yaml.Node
	if err := node.Encode(val); err != nil {
		return nil, err
	}
	return &node, nil
}

// --- markdown content formatting ---

// formatMarkdown normalizes an entity/relation body before it is written to
// disk. It delegates to [markdown.FormatMarkdown] so there is a single
// formatting implementation: the canonical content hash (internal/canonical)
// re-runs the same normalization to converge fsstore's reflowed body with
// pgstore's raw body, and that convergence breaks if fsstore reflows
// differently. Do not reintroduce a private copy.
func formatMarkdown(content string) string {
	return markdown.FormatMarkdown(content)
}

// --- entity I/O ---

// readEntityFile reads and parses an entity from a markdown file key.
//
// id and entityType are caller-supplied (the caller already knows them
// from the index); they are used to populate the resulting entity if the
// file is git-crypt encrypted and its frontmatter cannot be read.
func (s *FSStore) readEntityFile(key, id, entityType string) (*entity.Entity, error) {
	data, err := s.readDataFile(key)
	if err != nil {
		return nil, err
	}

	if isGitCryptEncrypted(data) {
		return s.buildInaccessibleEntity(key, id, entityType, entity.InaccessibleReasonGitCrypt), nil
	}

	doc, err := parseDocument(string(data))
	if err != nil {
		return nil, err
	}

	docID := doc.getString("id")
	docType := doc.getString("type")

	e := entity.New(docID, docType)
	e.Content = doc.content

	if info, err := s.rooted.Stat(key); err == nil {
		e.UpdatedAt = info.ModTime()
	}

	for key, value := range doc.frontmatter {
		if entity.IsEntityPropertyKey(key) {
			e.Properties[key] = entity.CloneValue(value)
		}
	}

	return e, nil
}

// buildInaccessibleEntity constructs a stand-in entity for a file whose
// content cannot be read. The ID and type are caller-supplied; every
// property declared by the entity type's schema is listed in
// [entity.Entity.Inaccessible] along with the magic "content" field
// that names the markdown body, so consumers know exactly which fields
// exist but are unreadable.
//
// Invariant: entityType is always present in s.schemas. [New] rejects
// stores constructed without a populated Schemas map, and unknown-type
// directories are skipped at scan time and in the watcher path. The
// resulting Inaccessible slice always has at least one entry (the
// content marker), so every IsLocked() guard fires reliably.
func (s *FSStore) buildInaccessibleEntity(key, id, entityType string, reason entity.InaccessibleReason) *entity.Entity {
	e := entity.New(id, entityType)
	if info, err := s.rooted.Stat(key); err == nil {
		e.UpdatedAt = info.ModTime()
	}
	props := s.propertyOrder(entityType)
	e.Inaccessible = make([]entity.InaccessibleField, 0, len(props)+1)
	for _, name := range props {
		e.Inaccessible = append(e.Inaccessible, entity.InaccessibleField{Name: name, Reason: reason})
	}
	e.Inaccessible = append(e.Inaccessible, entity.InaccessibleField{
		Name:   entity.InaccessibleFieldContent,
		Reason: reason,
	})
	return e
}

// formatEntity formats an entity as markdown with YAML frontmatter.
func formatEntity(e *entity.Entity, propertyOrder []string) (string, error) {
	fm := make(map[string]any)
	fm["id"] = e.ID
	fm["type"] = e.Type
	maps.Copy(fm, e.Properties)

	keyOrder := []string{"id", "type"}
	if len(propertyOrder) > 0 {
		keyOrder = append(keyOrder, propertyOrder...)
	}

	content := e.Content
	if content != "" {
		content = formatMarkdown(content)
	}

	return formatDocumentOrdered(fm, content, keyOrder)
}

// writeEntityFile writes an entity to a markdown file using temp-file +
// rename. The filename stem is the state key ("id" or "id@pointer");
// the pointer lives ONLY in the filename — entity frontmatter carries
// no pointer key, so there is a single source of truth (TKT-DOFYR1).
func (s *FSStore) writeEntityFile(e *entity.Entity) error {
	key := s.entityFileKey(e.Type, stateKey(e.ID, e.Pointer))
	order := s.propertyOrder(e.Type)
	content, err := formatEntity(e, order)
	if err != nil {
		return err
	}
	return s.writeDataFile(key, []byte(content), 0o644)
}

// --- relation I/O ---

// readRelationFile reads and parses a relation from a markdown file key.
//
// from, relType, to are caller-supplied (derived from the filename); they
// are used to populate the resulting relation if the file is git-crypt
// encrypted and its frontmatter cannot be read.
func (s *FSStore) readRelationFile(key, from, relType, to string) (*entity.Relation, error) {
	data, err := s.readDataFile(key)
	if err != nil {
		return nil, err
	}

	if isGitCryptEncrypted(data) {
		return s.buildInaccessibleRelation(key, from, relType, to, entity.InaccessibleReasonGitCrypt), nil
	}

	doc, err := parseDocument(string(data))
	if err != nil {
		return nil, err
	}

	r := entity.NewRelation(
		doc.getString("from"),
		doc.getString("relation"),
		doc.getString("to"),
	)
	// from_pointer frontmatter is WRITE-ONLY human documentation: the
	// filename (and hence the index meta, stamped by loadRelationMeta)
	// is the single authority for the tail pointer, mirroring entities
	// where the pointer lives only in the filename. Parsing it here
	// would create a second source of truth that a hand-edit could
	// desynchronize (TKT-DOFYR1 review).
	r.Content = doc.content

	if info, err := s.rooted.Stat(key); err == nil {
		r.UpdatedAt = info.ModTime()
	}

	for key, value := range doc.frontmatter {
		if entity.IsRelationPropertyKey(key) {
			if r.Properties == nil {
				r.Properties = make(map[string]any)
			}
			r.Properties[key] = entity.CloneValue(value)
		}
	}

	return r, nil
}

// buildInaccessibleRelation constructs a stand-in relation for a file
// whose content cannot be read. The endpoints are caller-supplied. The
// relation has no metamodel-declared properties (unlike entities), so
// the sole inaccessible entry names the markdown body.
func (s *FSStore) buildInaccessibleRelation(
	key, from, relType, to string,
	reason entity.InaccessibleReason,
) *entity.Relation {
	r := entity.NewRelation(from, relType, to)
	if info, err := s.rooted.Stat(key); err == nil {
		r.UpdatedAt = info.ModTime()
	}
	r.Inaccessible = []entity.InaccessibleField{
		{Name: entity.InaccessibleFieldContent, Reason: reason},
	}
	return r
}

// formatRelation formats a relation as markdown with YAML frontmatter.
// from_pointer is written only for a non-default tail (TKT-DOFYR1): the
// key never appears for pointerless projects, and the codec never emits
// an empty pointer.
func formatRelation(r *entity.Relation) (string, error) {
	fm := map[string]any{
		"from":     r.From,
		"relation": r.Type,
		"to":       r.To,
	}
	if !r.FromPointer.IsDefault() {
		fm["from_pointer"] = string(r.FromPointer)
	}
	maps.Copy(fm, r.Properties)

	keyOrder := []string{"from", "from_pointer", "relation", "to"}

	content := r.Content
	if content != "" {
		content = formatMarkdown(content)
	}

	return formatDocumentOrdered(fm, content, keyOrder)
}

// writeRelationFile writes a relation to a markdown file using temp-file +
// rename. The FROM slot of the filename carries the tail pointer.
func (s *FSStore) writeRelationFile(r *entity.Relation) error {
	key := s.relationFileKeyMeta(relationMeta{
		From: r.From, Type: r.Type, To: r.To, FromPointer: r.FromPointer,
	})
	content, err := formatRelation(r)
	if err != nil {
		return err
	}
	return s.writeDataFile(key, []byte(content), 0o644)
}

// writeDataFile writes content to the given key through RootedFS.
// RootedFS handles path validation, parent-directory creation, and
// delegates the actual write to the underlying FS (SafeFS in
// production, which then handles atomic temp+rename+fsync).
//
// Self-echo hash recording is handled entirely by the post-write
// observer installed on the bottom-most FS (SafeFS.OnPostWrite or
// MemFS.OnPostWrite for tests). That's where the actual on-disk
// bytes are visible — fsstore sees only plaintext here and must not
// record a plaintext hash the watcher can't match.
func (s *FSStore) writeDataFile(key string, content []byte, perm os.FileMode) error {
	return s.rooted.WriteFile(key, content, perm)
}

// readDataFile reads the given key through RootedFS. RootedFS
// validates the key and delegates to the underlying FS (which applies
// any decoding decorators — none currently in production).
func (s *FSStore) readDataFile(key string) ([]byte, error) {
	return s.rooted.ReadFile(key)
}
