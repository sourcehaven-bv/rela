package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// HistoryPurgeCmd HARD-DELETES entity version snapshot rows for compliance
// redaction (TKT-BW6UUL) — the deliberate, audited, IRREVERSIBLE exception to
// append-only history. Operator-only: the trust boundary is shell +
// RELA_DATABASE_URL access (same as `rela db migrate` / `rela restore`), NOT an
// ACL permission — the CLI applies no per-invocation ACL. pgstore-build only.
//
// Safety: **dry-run is the DEFAULT**; nothing is deleted without --commit. A
// --commit requires typing the entity id to confirm (or --yes for scripts).
// Purge REFUSES when the live row still holds the content (the sweep would
// re-capture it) unless --force-live, and REFUSES a rename row (purging it would
// orphan lineage). Every purge is audited (op=purge-version) with the required
// --reason; the purged content is never logged.
type HistoryPurgeCmd struct {
	ID          string `arg:"" help:"Entity ID whose version history to purge."`
	Vseq        int64  `help:"Purge the single version row with this vseq (see 'rela history <id>')." default:"0"`
	ContentHash string `help:"Purge every version row in the lineage with this content hash (erase a value everywhere)." default:""`
	All         bool   `help:"Purge the entity's entire (fenced) version history."`
	Reason      string `help:"Required: operator justification, recorded in the audit trail. Do NOT put the secret being purged here (it is logged in cleartext)." default:""`
	Commit      bool   `help:"Actually delete. Without this, the command is a dry-run that only shows what WOULD be purged."`
	Yes         bool   `help:"Skip the type-the-id confirmation (for scripts). Requires --commit."`
	ForceLive   bool   `help:"Purge even though a live row still holds the content; writes a tombstone so the sweep won't re-capture it. Redact the live value first when possible."`
}

// Run dispatches `rela history-purge <id> ...`.
func (c *HistoryPurgeCmd) Run(ctx context.Context, svc *writeServices) error {
	if svc.Versions == nil {
		out.WriteMessage("The active storage backend does not support version purge " +
			"(a PostgreSQL-build compliance feature).")
		return nil
	}
	var purger store.VersionPurger = svc.Versions
	if err := validatePurgeFlags(c.Reason, c.Vseq, c.ContentHash, c.All); err != nil {
		return err
	}
	p := principal.From(ctx)
	req := store.VersionPurgeRequest{
		EntityID:      c.ID,
		Selector:      store.PurgeSelector{Vseq: c.Vseq, ContentHash: c.ContentHash, All: c.All},
		Reason:        c.Reason,
		ForceLive:     c.ForceLive,
		PrincipalUser: p.User,
		PrincipalTool: p.Tool,
	}

	// Always preview first (DryRun), so the operator sees exactly what will die
	// and any refusal BEFORE anything is deleted — even with --commit.
	preview := req
	preview.DryRun = true
	res, err := purger.PurgeVersions(ctx, preview)
	if err != nil {
		return fmt.Errorf("purge history for %q: %w", c.ID, err)
	}
	if refused := reportPurgePreview(res, c.ID, !c.Commit); refused || !c.Commit {
		return nil
	}
	if !c.Yes && !confirmPurge(c.ID) {
		out.WriteMessage("Cancelled")
		return nil
	}

	// Confirmed: do the real (destructive) purge.
	final, err := purger.PurgeVersions(ctx, req)
	if err != nil {
		return fmt.Errorf("purge history for %q: %w", c.ID, err)
	}
	auditPurge(svc.Audit, p, audit.Subject{Kind: "entity", ID: c.ID}, final, c.Reason)
	out.WriteSuccess("Purged %d version row(s) for %s. This is irreversible.", final.Purged, c.ID)
	if final.TombstoneWritten {
		out.WriteInfo("Wrote a purge tombstone (a live row still held the content); the sweep will not re-capture it.")
	}
	return nil
}

// RelationHistoryPurgeCmd is the relation analog.
type RelationHistoryPurgeCmd struct {
	From        string `arg:"" help:"Source entity ID (the relation's 'from')."`
	Type        string `arg:"" help:"Relation type."`
	To          string `arg:"" help:"Target entity ID (the relation's 'to')."`
	Vseq        int64  `help:"Purge the single version row with this vseq." default:"0"`
	ContentHash string `help:"Purge every version row in the lineage with this content hash." default:""`
	All         bool   `help:"Purge the relation's entire (fenced) version history."`
	Reason      string `help:"Required: operator justification (logged cleartext — not the secret)." default:""`
	Commit      bool   `help:"Actually delete. Without this, a dry-run showing what WOULD be purged."`
	Yes         bool   `help:"Skip the confirmation (scripts). Requires --commit."`
	ForceLive   bool   `help:"Purge even though the live relation still holds the content (writes a tombstone)."`
}

// Run dispatches `rela relation-history-purge <from> <type> <to> ...`.
func (c *RelationHistoryPurgeCmd) Run(ctx context.Context, svc *writeServices) error {
	if svc.Versions == nil {
		out.WriteMessage("The active storage backend does not support relation version purge " +
			"(a PostgreSQL-build compliance feature).")
		return nil
	}
	var purger store.RelationVersionPurger = svc.Versions
	if err := validatePurgeFlags(c.Reason, c.Vseq, c.ContentHash, c.All); err != nil {
		return err
	}
	p := principal.From(ctx)
	key := fmt.Sprintf("%s--%s--%s", c.From, c.Type, c.To)
	req := store.RelationVersionPurgeRequest{
		From: c.From, Type: c.Type, To: c.To,
		Selector:      store.PurgeSelector{Vseq: c.Vseq, ContentHash: c.ContentHash, All: c.All},
		Reason:        c.Reason,
		ForceLive:     c.ForceLive,
		PrincipalUser: p.User,
		PrincipalTool: p.Tool,
	}

	preview := req
	preview.DryRun = true
	res, err := purger.PurgeRelationVersions(ctx, preview)
	if err != nil {
		return fmt.Errorf("purge relation history for %s: %w", key, err)
	}
	if refused := reportPurgePreview(res, key, !c.Commit); refused || !c.Commit {
		return nil
	}
	if !c.Yes && !confirmPurge(key) {
		out.WriteMessage("Cancelled")
		return nil
	}

	final, err := purger.PurgeRelationVersions(ctx, req)
	if err != nil {
		return fmt.Errorf("purge relation history for %s: %w", key, err)
	}
	auditPurge(svc.Audit, p, audit.Subject{
		Kind: "relation", RelationType: c.Type, FromID: c.From, ToID: c.To,
	}, final, c.Reason)
	out.WriteSuccess("Purged %d version row(s) for %s. This is irreversible.", final.Purged, key)
	if final.TombstoneWritten {
		out.WriteInfo("Wrote a purge tombstone (a live relation still held the content); the sweep will not re-capture it.")
	}
	return nil
}

// --- shared helpers ---

func validatePurgeFlags(reason string, vseq int64, hash string, all bool) error {
	if strings.TrimSpace(reason) == "" {
		return errors.New("--reason is required (recorded in the audit trail; do not include the secret)")
	}
	n := 0
	if vseq != 0 {
		n++
	}
	if hash != "" {
		n++
	}
	if all {
		n++
	}
	if n != 1 {
		return errors.New("specify exactly one of --vseq, --content-hash, or --all")
	}
	return nil
}

// reportPurgePreview prints the resolved targets and any refusal. Returns true
// if the purge was REFUSED (rename row present, or live row without --force-live)
// so the caller stops. In dry-run mode it always prints the preview and the
// caller returns without deleting.
func reportPurgePreview(res *store.PurgeResult, target string, dryRun bool) (refused bool) {
	if len(res.Targets) == 0 {
		out.WriteMessage("Nothing to purge for %s (no matching version rows).", target)
		return true
	}
	if dryRun {
		out.WriteInfo("DRY RUN — would purge %d version row(s) for %s:", len(res.Targets), target)
	}
	for _, t := range res.Targets {
		line := fmt.Sprintf("  vseq %d  %s  %s", t.Vseq, t.Op, t.CreatedAt.Format("2006-01-02 15:04:05"))
		if t.IsRename {
			line += "  [RENAME — refused: purging it would orphan lineage]"
		}
		out.WriteInfo("%s", line)
	}
	if res.RenameInTargets {
		out.WriteMessage("Refused: the target set contains a rename row. " +
			"Purging a rename row orphans lineage; select non-rename rows (by --vseq or --content-hash).")
		return true
	}
	if res.LiveRowExists && !dryRun {
		// Only reached when --commit was set but --force-live was not (the store
		// refused and deleted nothing).
		out.WriteMessage("Refused: the live entity/relation still holds this content — the sweep would " +
			"re-capture it. Redact the live value (or delete it) first, or pass --force-live.")
		return true
	}
	if res.LiveRowExists && dryRun {
		out.WriteInfo("NOTE: a live row still holds this content. A --commit will REFUSE " +
			"unless you also pass --force-live (which writes a sweep-suppressing tombstone).")
	}
	return false
}

func confirmPurge(target string) bool {
	fmt.Printf("This IRREVERSIBLY deletes version history for %s.\nType the id/key to confirm: ", target)
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	return strings.TrimSpace(response) == target
}

// auditPurge records the forensic purge event through the audit sink. It records
// identity + count + reason + the vseqs/hash targeted — NEVER the purged content.
func auditPurge(sink audit.Audit, p principal.Principal, subj audit.Subject, res *store.PurgeResult, reason string) {
	// Summarize the targeted vseqs compactly (count + range), not an enumeration.
	summary := fmt.Sprintf("purged=%d reason=%q", res.Purged, reason)
	if len(res.Targets) > 0 {
		summary += fmt.Sprintf(" vseq_range=[%d,%d]",
			res.Targets[0].Vseq, res.Targets[len(res.Targets)-1].Vseq)
	}
	if res.TombstoneWritten {
		summary += " tombstone=true"
	}
	sink.Record(audit.Record{
		Time:      time.Now().UTC(),
		Op:        audit.OpPurgeVersion,
		Subject:   &subj,
		Principal: p,
		Summary:   summary,
	})
}
