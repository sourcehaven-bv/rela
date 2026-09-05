package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// HistoryCmd shows the version timeline of an entity, or prints one past
// version's snapshot for piping into an external diff tool (Unix-philosophy:
// rela emits the bytes, `diff`/`delta`/`git diff --no-index` render them).
//
// Content versioning is a pgstore-only capability; on other backends this
// command reports that the active backend does not support history (fsstore
// deployments use git for the same purpose).
type HistoryCmd struct {
	ID      string `arg:"" help:"Entity ID (e.g. REQ-001). May name a live or deleted entity."`
	Version int    `help:"Print the full snapshot for this 1-based version ordinal (for piping to a diff tool) instead of the timeline." default:"0"`
}

// Run dispatches `rela history <id> [--version N]`.
func (c *HistoryCmd) Run(ctx context.Context, svc *readServices) error {
	if svc.Versions == nil {
		out.WriteMessage("The active storage backend does not support version history " +
			"(content versioning is a PostgreSQL-build feature; filesystem deployments use git).")
		return nil
	}
	var reader store.HistoryReader = svc.Versions

	if c.Version > 0 {
		return c.printSnapshot(ctx, reader)
	}
	return c.printTimeline(ctx, reader)
}

// printTimeline lists the version metadata rows oldest-first.
func (c *HistoryCmd) printTimeline(ctx context.Context, reader store.HistoryReader) error {
	metas, err := reader.ListVersions(ctx, c.ID)
	if err != nil {
		return fmt.Errorf("read history for %q: %w", c.ID, err)
	}
	if len(metas) == 0 {
		out.WriteMessage("No version history for %s.", c.ID)
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
		line := fmt.Sprintf("v%d  %s  %s  %s", m.Version, m.CreatedAt.Format("2006-01-02 15:04:05"), m.Op, who)
		if m.Op == store.VersionOpRename && m.PrevID != "" {
			line += "  (was " + m.PrevID + ")"
		}
		if m.TriggeredBy != "" {
			line += "  [" + m.TriggeredBy + "]"
		}
		// A version with NO origin is a direct edit, and prints nothing extra:
		// the principal already on the line says who typed it. Only a
		// mechanism-produced write earns a marker.
		if m.Origin.Kind != "" {
			line += "  " + string(m.Origin.Kind)
			if src := m.Origin.SourceLabel(); src != "" {
				line += " from " + src
			}
			if m.Origin.Definition != "" {
				line += " (" + m.Origin.Definition + ")"
			}
		}
		out.WriteInfo("%s", line)
	}
	return nil
}

// printSnapshot writes one version's content + properties as JSON to stdout, so
// two invocations can be diffed by an external tool.
func (c *HistoryCmd) printSnapshot(ctx context.Context, reader store.HistoryReader) error {
	snap, err := reader.GetVersion(ctx, c.ID, c.Version)
	if errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("no version %d for %q", c.Version, c.ID)
	}
	if err != nil {
		return fmt.Errorf("read version %d for %q: %w", c.Version, c.ID, err)
	}
	payload := map[string]any{
		"id":         c.ID,
		"version":    snap.Version,
		"op":         snap.Op,
		"type":       snap.Type,
		"created_at": snap.CreatedAt,
		"principal":  map[string]string{"user": snap.PrincipalUser, "tool": snap.PrincipalTool},
		"content":    snap.Content,
		"properties": snap.Properties,
	}
	if o := originPayload(snap.Origin); o != nil {
		payload["origin"] = o
	}
	enc := json.NewEncoder(out.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

// originPayload renders a version's provenance for the JSON snapshot, or nil
// for a direct edit — an OMITTED key, not a null or an "origin": "manual"
// placeholder, so the absence carries the meaning (see store.Origin).
func originPayload(o store.Origin) map[string]string {
	if o.IsZero() {
		return nil
	}
	m := map[string]string{"kind": string(o.Kind)}
	if src := o.SourceLabel(); src != "" {
		m["source"] = src
	}
	if o.Definition != "" {
		m["definition"] = o.Definition
	}
	return m
}
