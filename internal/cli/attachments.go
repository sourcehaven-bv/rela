package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/output"
)

// AttachmentsCmd lists all file attachments for an entity.
type AttachmentsCmd struct {
	EntityID string `arg:"" name:"entity-id" help:"Target entity ID."`
}

// Run dispatches `rela attachments <entity-id>`.
func (c *AttachmentsCmd) Run(ctx context.Context, svc *cliServices) error {
	infos, err := svc.ListAttachments(ctx, c.EntityID)
	if err != nil {
		return err
	}
	if len(infos) == 0 {
		out.WriteMessage("No attachments found for %s", c.EntityID)
		return nil
	}

	out.WriteMessage("Attachments for %s:\n", c.EntityID)

	propWidth := len("PROPERTY")
	pathWidth := len("PATH")
	for _, info := range infos {
		if len(info.Property) > propWidth {
			propWidth = len(info.Property)
		}
		if len(info.Path) > pathWidth {
			pathWidth = len(info.Path)
		}
	}

	format := fmt.Sprintf("  %%-%ds  %%-%ds  %%s\n", propWidth, pathWidth)
	out.WriteMessage(format, "PROPERTY", "PATH", "SIZE")
	out.WriteMessage(format,
		strings.Repeat("-", propWidth),
		strings.Repeat("-", pathWidth),
		"----")

	for _, info := range infos {
		size := "-"
		if info.Size > 0 {
			size = output.FormatSize(info.Size)
		}
		out.WriteMessage(format, info.Property, info.Path, size)
	}
	return nil
}
