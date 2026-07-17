package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RelationHistoryCmd shows the version timeline of a relation, or prints one
// past version's snapshot for piping into an external diff tool. A relation is
// addressed by its three-part key. Relation content versioning is a pgstore-only
// capability (filesystem deployments use git for the same purpose).
type RelationHistoryCmd struct {
	From    string `arg:"" help:"Source entity ID (the relation's 'from')."`
	Type    string `arg:"" help:"Relation type (e.g. addresses)."`
	To      string `arg:"" help:"Target entity ID (the relation's 'to')."`
	Version int    `help:"Print the full snapshot for this 1-based version ordinal (for piping to a diff tool) instead of the timeline." default:"0"`
}

// Run dispatches `rela relation-history <from> <type> <to> [--version N]`.
func (c *RelationHistoryCmd) Run(ctx context.Context, svc *writeServices) error {
	reader, ok := svc.Store.(store.RelationHistoryReader)
	if !ok {
		out.WriteMessage("The active storage backend does not support relation version history " +
			"(content versioning is a PostgreSQL-build feature; filesystem deployments use git).")
		return nil
	}
	if c.Version > 0 {
		return c.printSnapshot(ctx, reader)
	}
	return c.printTimeline(ctx, reader)
}

func (c *RelationHistoryCmd) printTimeline(ctx context.Context, reader store.RelationHistoryReader) error {
	metas, err := reader.ListRelationVersions(ctx, c.From, c.Type, c.To)
	if err != nil {
		return fmt.Errorf("read relation history for %s--%s--%s: %w", c.From, c.Type, c.To, err)
	}
	if len(metas) == 0 {
		out.WriteMessage("No version history for %s--%s--%s.", c.From, c.Type, c.To)
		return nil
	}
	for _, m := range metas {
		who := m.PrincipalUser
		if who == "" {
			who = "unknown"
		}
		if m.PrincipalTool != "" {
			who += " (" + m.PrincipalTool + ")"
		}
		line := fmt.Sprintf("v%d  %s  %s  %s  (%s--%s--%s)",
			m.Version, m.CreatedAt.Format("2006-01-02 15:04:05"), m.Op, who, m.From, m.Type, m.To)
		if m.Op == store.VersionOpRename && (m.PrevFrom != "" || m.PrevTo != "") {
			line += fmt.Sprintf("  (was %s--%s--%s)", m.PrevFrom, m.Type, m.PrevTo)
		}
		if m.TriggeredBy != "" {
			line += "  [" + m.TriggeredBy + "]"
		}
		out.WriteInfo("%s", line)
	}
	return nil
}

func (c *RelationHistoryCmd) printSnapshot(ctx context.Context, reader store.RelationHistoryReader) error {
	snap, err := reader.GetRelationVersion(ctx, c.From, c.Type, c.To, c.Version)
	if errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("no version %d for %s--%s--%s", c.Version, c.From, c.Type, c.To)
	}
	if err != nil {
		return fmt.Errorf("read version %d for %s--%s--%s: %w", c.Version, c.From, c.Type, c.To, err)
	}
	payload := map[string]interface{}{
		"from":       snap.From,
		"type":       snap.Type,
		"to":         snap.To,
		"version":    snap.Version,
		"op":         snap.Op,
		"created_at": snap.CreatedAt,
		"principal":  map[string]string{"user": snap.PrincipalUser, "tool": snap.PrincipalTool},
		"content":    snap.Content,
		"properties": snap.Properties,
	}
	enc := json.NewEncoder(out.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

// RelationRestoreCmd restores a relation's content and properties to a past
// version, applying the historical snapshot as a normal write through the
// entitymanager (authorized, validated, audited, re-versioned). If the relation
// currently exists it is updated; if it was deleted, it is re-created — which
// fails if an endpoint entity no longer exists. pgstore-only.
type RelationRestoreCmd struct {
	From    string `arg:"" help:"Source entity ID (the relation's 'from')."`
	Type    string `arg:"" help:"Relation type."`
	To      string `arg:"" help:"Target entity ID (the relation's 'to')."`
	Version int    `arg:"" help:"The 1-based version ordinal to restore to (see 'rela relation-history')."`
}

// Run dispatches `rela relation-restore <from> <type> <to> <version>`.
func (c *RelationRestoreCmd) Run(ctx context.Context, svc *writeServices) error {
	reader, ok := svc.Store.(store.RelationHistoryReader)
	if !ok {
		out.WriteMessage("The active storage backend does not support relation version history " +
			"(restore is a PostgreSQL-build feature).")
		return nil
	}

	snap, err := reader.GetRelationVersion(ctx, c.From, c.Type, c.To, c.Version)
	if errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("no version %d for %s--%s--%s", c.Version, c.From, c.Type, c.To)
	}
	if err != nil {
		return fmt.Errorf("read version %d for %s--%s--%s: %w", c.Version, c.From, c.Type, c.To, err)
	}

	content := snap.Content
	opts := entity.RelationOptions{Properties: snap.Properties, Content: &content}

	_, getErr := svc.Store.GetRelation(ctx, c.From, c.Type, c.To)
	switch {
	case getErr == nil:
		if _, err := svc.EntityManager.UpdateRelation(ctx, c.From, c.Type, c.To, opts); err != nil {
			return fmt.Errorf("restore (update) %s--%s--%s to v%d: %w", c.From, c.Type, c.To, c.Version, err)
		}
	case errors.Is(getErr, store.ErrNotFound):
		if _, err := svc.EntityManager.CreateRelation(ctx, c.From, c.Type, c.To, opts); err != nil {
			return fmt.Errorf("restore (re-create) %s--%s--%s to v%d: %w", c.From, c.Type, c.To, c.Version, err)
		}
	default:
		return fmt.Errorf("restore %s--%s--%s: check current state: %w", c.From, c.Type, c.To, getErr)
	}

	out.WriteSuccess("Restored %s--%s--%s to version %d.", c.From, c.Type, c.To, c.Version)
	return nil
}
