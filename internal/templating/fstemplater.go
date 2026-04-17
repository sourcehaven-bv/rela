package templating

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// FSTemplater loads and writes entity/relation templates as markdown files
// under the project's templates/ tree.
type FSTemplater struct {
	fs    storage.FS
	paths *project.Context
}

var _ Templater = (*FSTemplater)(nil)

// NewFSTemplater constructs a filesystem-backed Templater.
func NewFSTemplater(fs storage.FS, paths *project.Context) *FSTemplater {
	return &FSTemplater{fs: fs, paths: paths}
}

func (t *FSTemplater) EntityTemplate(_ context.Context, entityType, variant string) (*Template, error) {
	path := t.paths.EntityTemplateVariantPath(entityType, variant)
	doc, err := loadEntityTemplateDoc(t.fs, path)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, nil //nolint:nilnil // miss is not an error at this layer
	}
	return entityDocToTemplate(doc, entityType, variant), nil
}

func (t *FSTemplater) EntityTemplates(_ context.Context, entityType string) ([]*Template, error) {
	return discoverEntityTemplates(t.fs, t.paths.EntityTemplatesDir, entityType)
}

func (t *FSTemplater) RelationTemplate(_ context.Context, relationType string) (*Template, error) {
	path := t.paths.RelationTemplatePath(relationType)
	doc, err := loadRelationTemplateDoc(t.fs, path)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, nil //nolint:nilnil // miss is not an error at this layer
	}
	return relationDocToTemplate(doc), nil
}

func (t *FSTemplater) GenerateEntity(
	_ context.Context, meta *metamodel.Metamodel, entityType, variant string, force bool,
) (bool, error) {
	path := t.paths.EntityTemplateVariantPath(entityType, variant)
	return generateEntityTemplate(t.fs, path, meta, entityType, force)
}

func (t *FSTemplater) GenerateRelation(
	_ context.Context, meta *metamodel.Metamodel, relationType string, force bool,
) (bool, error) {
	path := t.paths.RelationTemplatePath(relationType)
	return generateRelationTemplate(t.fs, path, meta, relationType, force)
}

// entityDocToTemplate converts a parsed markdown document into an entity
// Template.
func entityDocToTemplate(doc *markdown.Document, entityType, variant string) *Template {
	properties := make(map[string]interface{}, len(doc.Frontmatter))
	for k, v := range doc.Frontmatter {
		if k == "id" || k == "type" || k == "_template_relations" {
			continue
		}
		properties[k] = v
	}
	return &Template{
		Name:       variant,
		EntityType: entityType,
		Properties: properties,
		Content:    doc.Content,
		Relations:  docTemplateRelations(doc.Frontmatter),
	}
}

// relationDocToTemplate converts a relation template document.
// from/relation/to are dropped because they're structural, not defaults.
func relationDocToTemplate(doc *markdown.Document) *Template {
	properties := make(map[string]interface{}, len(doc.Frontmatter))
	for k, v := range doc.Frontmatter {
		if k == "from" || k == "relation" || k == "to" {
			continue
		}
		properties[k] = v
	}
	return &Template{
		Properties: properties,
		Content:    doc.Content,
	}
}

// docTemplateRelations reads _template_relations from the frontmatter
// and returns them in the top-level Relation shape used by Template.
func docTemplateRelations(frontmatter map[string]interface{}) []Relation {
	raw, ok := frontmatter["_template_relations"]
	if !ok || raw == nil {
		return nil
	}
	list, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	out := make([]Relation, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		rel, _ := m["relation"].(string)
		target, _ := m["target"].(string)
		if rel != "" {
			out = append(out, Relation{Type: rel, Target: target})
		}
	}
	return out
}
