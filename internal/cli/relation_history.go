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
	From          string `arg:"" help:"Source entity ID (the relation's 'from')."`
	Type          string `arg:"" help:"Relation type (e.g. addresses)."`
	To            string `arg:"" help:"Target entity ID (the relation's 'to')."`
	Version       int    `help:"Print the full snapshot for this 1-based version ordinal (for piping to a diff tool) instead of the timeline." default:"0"`
	Lifetime      int    `help:"Select a past lifetime of a deleted-and-recreated key (1 = newest; see --list-lifetimes). Default: newest." default:"0"`
	ListLifetimes bool   `help:"List every past lifetime of this key (a reused (from,type,to) has several) instead of a timeline."`
}

// Run dispatches `rela relation-history <from> <type> <to> [--version N] [--lifetime K] [--list-lifetimes]`.
func (c *RelationHistoryCmd) Run(ctx context.Context, svc *writeServices) error {
	if svc.Versions == nil {
		out.WriteMessage("The active storage backend does not support relation version history " +
			"(content versioning is a PostgreSQL-build feature; filesystem deployments use git).")
		return nil
	}
	var reader store.RelationHistoryReader = svc.Versions
	if c.ListLifetimes {
		return c.printLifetimes(ctx, reader)
	}
	recordID, err := resolveLifetimeRecordID(ctx, reader, c.From, c.Type, c.To, c.Lifetime)
	if err != nil {
		return err
	}
	q := store.RelationHistoryQuery{From: c.From, Type: c.Type, To: c.To, RecordID: recordID}
	if c.Version > 0 {
		return c.printSnapshot(ctx, reader, q)
	}
	return c.printTimeline(ctx, reader, q)
}

// resolveLifetimeRecordID maps a 1-based lifetime ordinal to its durable
// rel_record_id via a single lifetimes enumeration (atomic w.r.t. that read).
// Returns 0 (newest lifetime) when lifetime <= 0. Shared by the relation-history,
// relation-restore, and relation-history-purge commands.
func resolveLifetimeRecordID(
	ctx context.Context, reader store.RelationHistoryReader, from, relType, to string, lifetime int,
) (int64, error) {
	if lifetime <= 0 {
		return 0, nil
	}
	lifetimes, err := reader.ListRelationLifetimes(ctx, from, relType, to)
	if err != nil {
		return 0, fmt.Errorf("list lifetimes for %s--%s--%s: %w", from, relType, to, err)
	}
	if lifetime > len(lifetimes) {
		return 0, fmt.Errorf("no lifetime %d for %s--%s--%s (%d exist)", lifetime, from, relType, to, len(lifetimes))
	}
	return lifetimes[lifetime-1].RecordID, nil
}

func (c *RelationHistoryCmd) printLifetimes(ctx context.Context, reader store.RelationHistoryReader) error {
	lifetimes, err := reader.ListRelationLifetimes(ctx, c.From, c.Type, c.To)
	if err != nil {
		return fmt.Errorf("list lifetimes for %s--%s--%s: %w", c.From, c.Type, c.To, err)
	}
	if len(lifetimes) == 0 {
		out.WriteMessage("No version history for %s--%s--%s.", c.From, c.Type, c.To)
		return nil
	}
	for _, lt := range lifetimes {
		flag := "deleted"
		if lt.Live {
			flag = "live"
		}
		out.WriteInfo("lifetime %d  [%s]  %d version(s)  %s → %s  final=%s  (record-id %d)",
			lt.Lifetime, flag, lt.VersionCount,
			lt.FirstSeen.Format("2006-01-02 15:04:05"), lt.LastSeen.Format("2006-01-02 15:04:05"),
			lt.FinalOp, lt.RecordID)
	}
	return nil
}

func (c *RelationHistoryCmd) printTimeline(
	ctx context.Context, reader store.RelationHistoryReader, q store.RelationHistoryQuery,
) error {
	metas, err := reader.ListRelationVersions(ctx, q)
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
	// Footer: signal that older deleted lifetimes exist (only for the newest view).
	if q.RecordID == 0 {
		if lifetimes, err := reader.ListRelationLifetimes(ctx, c.From, c.Type, c.To); err == nil && len(lifetimes) > 1 {
			out.WriteMessage("note: %d earlier deleted lifetime(s) of this key exist — "+
				"use --list-lifetimes, or --lifetime K to view one.", len(lifetimes)-1)
		}
	}
	return nil
}

func (c *RelationHistoryCmd) printSnapshot(
	ctx context.Context, reader store.RelationHistoryReader, q store.RelationHistoryQuery,
) error {
	snap, err := reader.GetRelationVersion(ctx, q, c.Version)
	if errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("no version %d for %s--%s--%s", c.Version, c.From, c.Type, c.To)
	}
	if err != nil {
		return fmt.Errorf("read version %d for %s--%s--%s: %w", c.Version, c.From, c.Type, c.To, err)
	}
	payload := map[string]any{
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
//
// Restoring from an OLD lifetime (--lifetime K on a deleted-and-recreated key)
// re-creates the triple as a NORMAL write, which mints a FRESH rel_record_id — a
// new lifetime 1 appears afterward; it does NOT revive the old lineage in place.
// That is the only sensible semantics (restore = an authorized re-create).
type RelationRestoreCmd struct {
	From     string `arg:"" help:"Source entity ID (the relation's 'from')."`
	Type     string `arg:"" help:"Relation type."`
	To       string `arg:"" help:"Target entity ID (the relation's 'to')."`
	Version  int    `arg:"" help:"The 1-based version ordinal to restore to (see 'rela relation-history')."`
	Lifetime int    `help:"Restore from a past lifetime of a deleted-and-recreated key (1 = newest; see --list-lifetimes). Default: newest." default:"0"`
}

// Run dispatches `rela relation-restore <from> <type> <to> <version> [--lifetime K]`.
func (c *RelationRestoreCmd) Run(ctx context.Context, svc *writeServices) error {
	if svc.Versions == nil {
		out.WriteMessage("The active storage backend does not support relation version history " +
			"(restore is a PostgreSQL-build feature).")
		return nil
	}
	var reader store.RelationHistoryReader = svc.Versions

	recordID, err := resolveLifetimeRecordID(ctx, reader, c.From, c.Type, c.To, c.Lifetime)
	if err != nil {
		return err
	}
	q := store.RelationHistoryQuery{From: c.From, Type: c.Type, To: c.To, RecordID: recordID}
	snap, err := reader.GetRelationVersion(ctx, q, c.Version)
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
