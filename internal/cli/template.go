package cli

import (
	"context"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/templating"
)

// TemplateCmd is the parent of template subcommands.
type TemplateCmd struct {
	Init TemplateInitCmd `cmd:"" help:"Generate template files from metamodel."`
}

// TemplateInitCmd generates entity/relation templates.
type TemplateInitCmd struct {
	Force     bool     `help:"Overwrite existing templates."`
	Entities  bool     `help:"Only generate entity templates."`
	Relations bool     `help:"Only generate relation templates."`
	Variant   string   `help:"Create a named variant template (e.g., --variant epic)."`
	Types     []string `arg:"" optional:"" help:"Specific types to generate (default: all)."`
}

// Run dispatches `rela template init [type...]`.
func (c *TemplateInitCmd) Run(ctx context.Context, svc *cliServices) error {
	meta := svc.Meta()
	entityTypes, relationTypes, err := c.resolveTemplateTypes(meta)
	if err != nil {
		return err
	}

	createdCount, skippedCount := 0, 0
	tmpl := svc.Templater()

	for _, entityType := range entityTypes {
		created, err := c.generateEntityTemplate(ctx, tmpl, meta, entityType)
		if err != nil {
			return err
		}
		if created {
			createdCount++
		} else {
			skippedCount++
		}
	}

	for _, relationType := range relationTypes {
		created, err := c.generateRelationTemplate(ctx, tmpl, meta, relationType)
		if err != nil {
			return err
		}
		if created {
			createdCount++
		} else {
			skippedCount++
		}
	}

	if createdCount > 0 || skippedCount > 0 {
		if !quiet {
			out.WriteInfo("Generated %d template(s), skipped %d existing", createdCount, skippedCount)
		}
	} else {
		out.WriteInfo("No templates to generate")
	}
	return nil
}

func (c *TemplateInitCmd) resolveTemplateTypes(
	meta *metamodel.Metamodel,
) (entityTypes, relationTypes []string, err error) {
	if len(c.Types) > 0 {
		entityTypes, relationTypes, err = c.partitionExplicitTypes(meta)
		if err != nil {
			return nil, nil, err
		}
	} else {
		if !c.Relations {
			entityTypes = meta.EntityTypes()
			natsort.Strings(entityTypes)
		}
		if !c.Entities {
			relationTypes = meta.RelationTypes()
			natsort.Strings(relationTypes)
		}
	}
	if c.Entities && !c.Relations {
		relationTypes = nil
	}
	if c.Relations && !c.Entities {
		entityTypes = nil
	}
	return entityTypes, relationTypes, nil
}

func (c *TemplateInitCmd) partitionExplicitTypes(
	meta *metamodel.Metamodel,
) (entityTypes, relationTypes []string, err error) {
	for _, typeName := range c.Types {
		if _, ok := meta.GetEntityDef(typeName); ok {
			entityTypes = append(entityTypes, meta.ResolveAlias(typeName))
			continue
		}
		if _, ok := meta.GetRelationDef(typeName); ok {
			relationTypes = append(relationTypes, typeName)
			continue
		}
		return nil, nil, fmt.Errorf("unknown type: %s (not an entity or relation type)", typeName)
	}
	return entityTypes, relationTypes, nil
}

func (c *TemplateInitCmd) generateEntityTemplate(
	ctx context.Context,
	tmpl templating.Templater,
	meta *metamodel.Metamodel,
	entityType string,
) (bool, error) {
	created, err := tmpl.GenerateEntity(ctx, meta, entityType, c.Variant, c.Force)
	if err != nil {
		return false, fmt.Errorf("failed to generate template for %s: %w", entityType, err)
	}
	filename := entityType + ".md"
	if c.Variant != "" {
		filename = entityType + "--" + c.Variant + ".md"
	}
	if created {
		out.WriteSuccess("Created template: templates/entities/%s", filename)
	} else if !quiet {
		out.WriteInfo("Skipped (exists): templates/entities/%s", filename)
	}
	return created, nil
}

func (c *TemplateInitCmd) generateRelationTemplate(
	ctx context.Context,
	tmpl templating.Templater,
	meta *metamodel.Metamodel,
	relationType string,
) (bool, error) {
	created, err := tmpl.GenerateRelation(ctx, meta, relationType, c.Force)
	if err != nil {
		return false, fmt.Errorf("failed to generate template for %s: %w", relationType, err)
	}
	if created {
		out.WriteSuccess("Created template: templates/relations/%s.md", relationType)
	} else if !quiet {
		out.WriteInfo("Skipped (exists): templates/relations/%s.md", relationType)
	}
	return created, nil
}
