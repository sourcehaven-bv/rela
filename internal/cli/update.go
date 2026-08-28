package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
)

// UpdateCmd updates an existing entity.
type UpdateCmd struct {
	ID string `arg:"" help:"Entity ID."`
	// Title writes the literal "title" property. Unlike the removed
	// `create -t` flag (which wrote GetPrimaryProperty()'s resolved
	// property), update never targeted the display property — so it stays.
	// `create` deliberately has no -t: the display property may be a derived
	// multi-property template (TKT-2SVA3L); use `-P <prop>=` there instead.
	Title       string   `short:"t" help:"New title."`
	Status      string   `short:"s" help:"New status."`
	Priority    string   `short:"p" help:"New priority."`
	Description string   `short:"d" help:"New description."`
	Property    []string `short:"P" help:"Set a property (format: key=value, can be repeated)."`
	// Unset removes a property outright. Distinct from `-P key=`, which
	// sets the EMPTY STRING and always has — changing that silently would
	// break existing scripts, so removal gets its own flag.
	Unset     []string `short:"U" help:"Remove a property entirely (can be repeated). Differs from -P key=, which sets an empty value."`
	Body      string   `short:"b" help:"Markdown body content for the entity."`
	BodyFile  string   `name:"body-file" short:"B" help:"Read body content from file (use - for stdin)."`
	ClearBody bool     `name:"clear-body" help:"Remove the entity's markdown body."`
	Strict    bool     `help:"Exit with status 1 if soft validation warnings are surfaced."`
}

// Run dispatches `rela update <id>`.
func (c *UpdateCmd) Run(ctx context.Context, svc *writeServices) error {
	// A targeted patch: name only the flags the operator passed. Properties
	// that go unmentioned are preserved by the manager, so `rela update` can
	// no longer clobber state it never read (TKT-80EWGM).
	patch := entity.Patch{Properties: map[string]any{}}

	for _, prop := range c.Property {
		key, value, parseErr := parsePropertyFlag(prop)
		if parseErr != nil {
			return parseErr
		}
		patch.Properties[key] = value
	}
	patch.MetaUnset = append(patch.MetaUnset, c.Unset...)

	for key, value := range map[string]string{
		"title":       c.Title,
		"status":      c.Status,
		"priority":    c.Priority,
		"description": c.Description,
	} {
		if value != "" {
			patch.Properties[key] = value
		}
	}

	// Check the FLAGS, not the resolved content: `--clear-body -B empty.md`
	// is just as contradictory as `--clear-body -b text`, and testing the
	// content would let the empty-file case through silently.
	if c.ClearBody && (c.Body != "" || c.BodyFile != "") {
		return errors.New("--clear-body cannot be combined with -b/--body or -B/--body-file")
	}

	bodyContent, err := c.getBodyContent()
	if err != nil {
		return err
	}
	switch {
	case c.ClearBody:
		patch.Content = new("")
	case c.BodyFile != "":
		// An explicitly-supplied file wins even when it is empty or all
		// whitespace: the operator named a source, so honor it rather than
		// silently degrading to "no updates specified". Use --clear-body to
		// clear deliberately.
		patch.Content = &bodyContent
	case bodyContent != "":
		patch.Content = &bodyContent
	}

	if patch.IsEmpty() {
		return errors.New("no updates specified")
	}

	result, err := svc.EntityManager.PatchEntity(ctx, c.ID, patch)
	if err != nil {
		if errors.Is(err, entitymanager.ErrEntityNotFound) {
			return &entityNotFoundError{ID: c.ID}
		}
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
