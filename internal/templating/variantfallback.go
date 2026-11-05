package templating

import (
	"context"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// entityTemplateGetter is the one Templater method EntityTemplateFor needs.
type entityTemplateGetter interface {
	EntityTemplate(ctx context.Context, entityType, variant string) (*Template, error)
}

// entityTemplateLister is the one Templater method EntityTemplatesFor needs.
type entityTemplateLister interface {
	EntityTemplates(ctx context.Context, entityType string) ([]*Template, error)
}

// EntityTemplateFor returns the template for entityType, falling back to
// the template of the abstract type it extends. A variant such as
// `isms.task` then starts from `templates/entities/task.md` until the
// operator writes `isms.task.md`. Returns nil when neither exists.
//
// Nil: meta is accepted — no fallback is tried.
func EntityTemplateFor(
	ctx context.Context, t entityTemplateGetter, meta *metamodel.Metamodel, entityType, variant string,
) (*Template, error) {
	tmpl, err := t.EntityTemplate(ctx, entityType, variant)
	if err != nil || tmpl != nil {
		return tmpl, err
	}
	parent := parentType(meta, entityType)
	if parent == "" {
		return nil, nil //nolint:nilnil // miss is not an error at this layer
	}
	tmpl, err = t.EntityTemplate(ctx, parent, variant)
	if err != nil || tmpl == nil {
		return tmpl, err
	}
	return asType(tmpl, entityType), nil
}

// EntityTemplatesFor returns the templates for entityType plus each
// template of the abstract type it extends whose name the variant does not
// define itself. Order follows Templater.EntityTemplates: default first.
//
// Nil: meta is accepted — no fallback is tried.
func EntityTemplatesFor(
	ctx context.Context, t entityTemplateLister, meta *metamodel.Metamodel, entityType string,
) ([]*Template, error) {
	own, err := t.EntityTemplates(ctx, entityType)
	if err != nil {
		return nil, err
	}
	parent := parentType(meta, entityType)
	if parent == "" {
		return own, nil
	}
	inherited, err := t.EntityTemplates(ctx, parent)
	if err != nil {
		return nil, err
	}
	out := slices.Clone(own)
	for _, tmpl := range inherited {
		if !slices.ContainsFunc(own, func(o *Template) bool { return o.Name == tmpl.Name }) {
			out = append(out, asType(tmpl, entityType))
		}
	}
	slices.SortStableFunc(out, func(a, b *Template) int {
		switch {
		case a.Name == b.Name:
			return 0
		case a.Name == "":
			return -1
		case b.Name == "":
			return 1
		}
		return 0
	})
	return out, nil
}

func parentType(meta *metamodel.Metamodel, entityType string) string {
	if meta == nil {
		return ""
	}
	def, ok := meta.GetEntityDef(entityType)
	if !ok {
		return ""
	}
	return def.Extends
}

func asType(tmpl *Template, entityType string) *Template {
	c := *tmpl
	c.EntityType = entityType
	return &c
}
