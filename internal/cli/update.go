package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// UpdateCmd updates an existing entity.
type UpdateCmd struct {
	ID          string   `arg:"" help:"Entity ID."`
	Title       string   `short:"t" help:"New title."`
	Status      string   `short:"s" help:"New status."`
	Priority    string   `short:"p" help:"New priority."`
	Description string   `short:"d" help:"New description."`
	Property    []string `short:"P" help:"Set a property (format: key=value, can be repeated)."`
	Body        string   `short:"b" help:"Markdown body content for the entity."`
	BodyFile    string   `name:"body-file" short:"B" help:"Read body content from file (use - for stdin)."`
	Strict      bool     `help:"Exit with status 1 if soft validation warnings are surfaced."`
}

// Run dispatches `rela update <id>`.
func (c *UpdateCmd) Run(ctx context.Context, svc *cliServices) error {
	entity, err := svc.Store().GetEntity(ctx, c.ID)
	if err != nil {
		return &entityNotFoundError{ID: c.ID}
	}

	changed := false
	for _, prop := range c.Property {
		key, value, parseErr := parsePropertyFlag(prop)
		if parseErr != nil {
			return parseErr
		}
		entity.SetString(key, value)
		changed = true
	}

	if c.Title != "" {
		entity.SetString("title", c.Title)
		changed = true
	}
	if c.Status != "" {
		entity.SetString("status", c.Status)
		changed = true
	}
	if c.Priority != "" {
		entity.SetString("priority", c.Priority)
		changed = true
	}
	if c.Description != "" {
		entity.SetString("description", c.Description)
		changed = true
	}

	bodyContent, err := c.getBodyContent()
	if err != nil {
		return err
	}
	if bodyContent != "" {
		entity.Content = bodyContent
		changed = true
	}

	if !changed {
		return errors.New("no updates specified")
	}

	result, err := svc.EntityManager().UpdateEntity(ctx, entity)
	if err != nil {
		return err
	}

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

	out.WriteSuccess("Updated %s", c.ID)

	if c.Strict && len(result.Warnings) > 0 {
		return errStrictWarnings
	}
	return nil
}

func (c *UpdateCmd) getBodyContent() (string, error) {
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
