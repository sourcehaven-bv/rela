package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	entitypkg "github.com/Sourcehaven-BV/rela/internal/entity"
)

// CreateCmd creates a new entity.
type CreateCmd struct {
	Type     string   `arg:"" help:"Entity type (alias allowed)."`
	Title    string   `short:"t" help:"Primary property value (title, name, etc. depending on entity type)."`
	Status   string   `short:"s" help:"Entity status (defaults to entity type's default)."`
	Priority string   `short:"p" help:"Entity priority."`
	ID       string   `name:"id" help:"Custom entity ID (auto-generated if not provided)."`
	Property []string `short:"P" help:"Set a property (format: key=value, can be repeated)."`
	Body     string   `short:"b" help:"Markdown body content for the entity."`
	BodyFile string   `name:"body-file" short:"B" help:"Read body content from file (use - for stdin)."`
	Strict   bool     `help:"Exit with status 1 if soft validation warnings are surfaced."`
}

// Run dispatches `rela create <type>`.
func (c *CreateCmd) Run(ctx context.Context, svc *cliServices) error {
	resolvedType, entityDef, err := resolveEntityType(svc.Meta(), c.Type)
	if err != nil {
		return err
	}

	props := make(map[string]interface{})
	for _, prop := range c.Property {
		key, value, parseErr := parsePropertyFlag(prop)
		if parseErr != nil {
			return parseErr
		}
		props[key] = value
	}

	if strings.TrimSpace(c.Title) != "" {
		primaryProp := entityDef.GetPrimaryProperty()
		if primaryProp == "" {
			primaryProp = "title"
		}
		props[primaryProp] = c.Title
	}

	if c.Status != "" {
		props["status"] = c.Status
	}
	if c.Priority != "" {
		props["priority"] = c.Priority
	}

	bodyContent, err := c.getBodyContent()
	if err != nil {
		return err
	}

	result, err := svc.EntityManager().CreateEntity(ctx,
		&entitypkg.Entity{
			Type:       resolvedType,
			Properties: props,
			Content:    bodyContent,
		},
		entitypkg.CreateOptions{ID: c.ID},
	)
	if err != nil {
		return err
	}
	entity := result.Entity

	printValidationWarnings(result.Warnings)
	for _, warning := range result.AutomationWarnings {
		out.WriteWarning("Automation: %s", warning)
	}
	for _, errMsg := range result.AutomationErrors {
		out.WriteWarning("Automation error: %s", errMsg)
	}
	for _, rel := range result.RelationsCreated {
		out.WriteInfo("Automation created relation: %s --%s--> %s", rel.From, rel.Type, rel.To)
	}

	out.WriteSuccess("Created %s %s", resolvedType, entity.ID)
	if outputFormat == "json" {
		if e, err := svc.Store().GetEntity(ctx, entity.ID); err == nil {
			_ = out.WriteEntities([]*entitypkg.Entity{e})
		}
	}

	if c.Strict && len(result.Warnings) > 0 {
		return errStrictWarnings
	}
	return nil
}

// getBodyContent returns the body content from --body or --body-file flags.
func (c *CreateCmd) getBodyContent() (string, error) {
	if c.Body != "" && c.BodyFile != "" {
		return "", errors.New("cannot specify both --body and --body-file")
	}
	if c.Body != "" {
		return c.Body, nil
	}
	if c.BodyFile != "" {
		var content []byte
		var err error
		if c.BodyFile == "-" {
			content, err = io.ReadAll(os.Stdin)
		} else {
			content, err = os.ReadFile(c.BodyFile)
		}
		if err != nil {
			return "", fmt.Errorf("failed to read body file: %w", err)
		}
		return strings.TrimSpace(string(content)), nil
	}
	return "", nil
}

// parsePropertyFlag parses a "key=value" property flag.
func parsePropertyFlag(prop string) (key, value string, err error) {
	idx := strings.Index(prop, "=")
	if idx == -1 {
		return "", "", fmt.Errorf("invalid property format %q: expected key=value", prop)
	}
	key = strings.TrimSpace(prop[:idx])
	value = strings.TrimSpace(prop[idx+1:])
	if key == "" {
		return "", "", fmt.Errorf("invalid property format %q: key cannot be empty", prop)
	}
	return key, value, nil
}
