package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/audit/sqlaudit"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// AuditCmd checks and maintains the audit log kept in the database
// (TKT-USQBMG6). The fs build keeps JSONL files instead, which these
// commands do not read.
type AuditCmd struct {
	Verify AuditVerifyCmd `cmd:"" help:"Check the audit log's hash chain and print its head."`
	Prune  AuditPruneCmd  `cmd:"" help:"Delete audit rows older than the retention period."`
	Import AuditImportCmd `cmd:"" help:"Append the records of JSONL audit files to the database audit log."`
}

// minAuditRetentionDays is the shortest retention `rela audit prune`
// accepts without --allow-short-retention: the three years the retention
// policy requires for audit logs (security logs need 12 months).
const minAuditRetentionDays = 1095

// errNoSQLAudit is an audit command run on a backend whose audit log is not
// in a database.
var errNoSQLAudit = errors.New(
	"this backend keeps the audit log as JSONL files under .rela/audit/; " +
		"rela audit needs the postgres or sqlite build")

// auditLog is what `rela audit` needs of the database audit log.
type auditLog interface {
	Verify(ctx context.Context) (sqlaudit.Verification, error)
	HashAt(ctx context.Context, seq int64) (string, bool, error)
	Prune(ctx context.Context, cutoff time.Time, rec audit.Record) (sqlaudit.Pruned, error)
	Import(ctx context.Context, recs iter.Seq2[audit.Record, error], rec audit.Record, summary string) (int64, error)
}

// auditLogOf returns svc's database audit log, or nil (not a nil
// *sqlaudit.Log in an interface) on a tier without one.
func auditLogOf(svc interface{ AuditLog() *sqlaudit.Log }) auditLog {
	l := svc.AuditLog()
	if l == nil {
		return nil
	}
	return l
}

// sqlAuditLog returns svc's database audit log, or errNoSQLAudit.
func sqlAuditLog(svc *writeServices) (auditLog, error) {
	if svc.AuditLog == nil {
		return nil, errNoSQLAudit
	}
	return svc.AuditLog, nil
}

// AuditVerifyCmd walks the chain.
type AuditVerifyCmd struct {
	ExpectHead string `name:"expect-head" placeholder:"SEQ:HASH" help:"Fail unless the chain has this head."`
}

// auditVerifyJSON is the `-o json` shape of `rela audit verify`.
type auditVerifyJSON struct {
	OK       bool   `json:"ok"`
	Rows     int64  `json:"rows"`
	Anchor   int64  `json:"anchor_seq"`
	HeadSeq  int64  `json:"head_seq"`
	HeadHash string `json:"head_hash"`
	BreakSeq int64  `json:"break_seq,omitempty"`
	Reason   string `json:"break_reason,omitempty"`
}

// Run verifies the chain and fails when it is broken.
func (c *AuditVerifyCmd) Run(ctx context.Context, svc *writeServices) error {
	expectSeq, expectHash, err := parseExpectHead(c.ExpectHead)
	if err != nil {
		return err
	}
	log, err := sqlAuditLog(svc)
	if err != nil {
		return err
	}
	v, err := log.Verify(ctx)
	if err != nil {
		return err
	}
	if v.Break == nil && expectSeq > 0 {
		reason, err := expectedHeadBreak(ctx, log, v, expectSeq, expectHash)
		if err != nil {
			return err
		}
		if reason != "" {
			v.Break = &sqlaudit.Break{Seq: expectSeq, Reason: reason}
		}
	}
	res := auditVerifyJSON{OK: v.Break == nil, Rows: v.Rows, Anchor: v.Anchor, HeadSeq: v.HeadSeq, HeadHash: v.HeadHash}
	if v.Break != nil {
		res.BreakSeq, res.Reason = v.Break.Seq, v.Break.Reason
	}
	if out.Format == output.FormatJSON {
		enc := json.NewEncoder(out.Out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			return err
		}
	} else {
		out.WriteMessage("Checked %d rows after seq %d.", v.Rows, v.Anchor)
		out.WriteMessage("Head: seq %d, hash %s", v.HeadSeq, v.HeadHash)
	}
	if v.Break != nil {
		return fmt.Errorf("audit chain broken at seq %d: %s", v.Break.Seq, v.Break.Reason)
	}
	if out.Format != output.FormatJSON {
		out.WriteSuccess("The audit chain verifies. Keep the head outside the database to detect a rewrite.")
	}
	return nil
}

// parseExpectHead parses --expect-head SEQ:HASH; seq is 0 when it is empty.
func parseExpectHead(s string) (seq int64, hash string, err error) {
	if s == "" {
		return 0, "", nil
	}
	seqStr, hash, ok := strings.Cut(s, ":")
	seq, err = strconv.ParseInt(seqStr, 10, 64)
	if !ok || err != nil || seq < 1 || hash == "" {
		return 0, "", fmt.Errorf("--expect-head %q: want SEQ:HASH, as printed by rela audit verify", s)
	}
	return seq, hash, nil
}

// expectedHeadBreak returns why the verified chain v does not contain the
// row seq with hash, or "" when it does.
func expectedHeadBreak(
	ctx context.Context, log auditLog, v sqlaudit.Verification, seq int64, hash string,
) (string, error) {
	switch {
	case seq <= v.Anchor:
		return fmt.Sprintf(
			"the expected head was pruned (the chain now starts after seq %d); record a new head", v.Anchor), nil
	case seq > v.HeadSeq:
		return fmt.Sprintf(
			"the expected head is past the current head seq %d: newer rows were deleted", v.HeadSeq), nil
	}
	got, ok, err := log.HashAt(ctx, seq)
	if err != nil {
		return "", err
	}
	if !ok || got != hash {
		return "the row's hash differs from the expected head: the chain was rewritten", nil
	}
	return "", nil
}

// AuditPruneCmd deletes old rows.
type AuditPruneCmd struct {
	KeepDays            int  `name:"keep-days" required:"" help:"Keep rows appended in the last N days."`
	AllowShortRetention bool `name:"allow-short-retention" help:"Accept --keep-days below 1095."`
}

// Run prunes the rows appended before the retention window.
func (c *AuditPruneCmd) Run(ctx context.Context, svc *writeServices) error {
	if c.KeepDays < 1 {
		return errors.New("--keep-days must be at least 1")
	}
	if c.KeepDays < minAuditRetentionDays && !c.AllowShortRetention {
		return fmt.Errorf("--keep-days %d is below the %d-day retention minimum; "+
			"pass --allow-short-retention if your policy allows it", c.KeepDays, minAuditRetentionDays)
	}
	log, err := sqlAuditLog(svc)
	if err != nil {
		return err
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -c.KeepDays)
	res, err := log.Prune(ctx, cutoff, operatorRecord(ctx))
	if err != nil {
		return err
	}
	if res.Rows == 0 {
		out.WriteSuccess("No audit rows were appended before %s.", cutoff.Format(time.RFC3339))
		return nil
	}
	out.WriteSuccess("Deleted %d audit rows through seq %d; the chain now starts after seq %d.",
		res.Rows, res.Anchor, res.Anchor)
	return nil
}

// AuditImportCmd appends JSONL files.
type AuditImportCmd struct {
	Paths []string `arg:"" help:"JSONL files, or directories whose *.jsonl files are imported in name order."`
}

// Run appends every record of the files, in order.
func (c *AuditImportCmd) Run(ctx context.Context, svc *writeServices) error {
	log, err := sqlAuditLog(svc)
	if err != nil {
		return err
	}
	files, err := jsonlFiles(c.Paths)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("no .jsonl files found")
	}
	n, err := log.Import(ctx, jsonlRecords(files), operatorRecord(ctx), fmt.Sprintf("files=%d", len(files)))
	if err != nil {
		return fmt.Errorf("imported %d records, then: %w", n, err)
	}
	out.WriteSuccess("Imported %d records from %d files.", n, len(files))
	return nil
}

// operatorRecord is the record a maintenance command writes about itself,
// attributed to the CLI's principal.
func operatorRecord(ctx context.Context) audit.Record {
	return audit.Stamp(ctx, audit.Record{
		Time:       time.Now().UTC(),
		Principal:  principal.From(ctx),
		AssertedBy: audit.AssertedByFrom(ctx),
	})
}

// jsonlFiles expands paths: a file stands for itself, a directory for its
// *.jsonl files in name order, which is date order for the daily files.
func jsonlFiles(paths []string) ([]string, error) {
	var files []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			files = append(files, p)
			continue
		}
		matches, err := filepath.Glob(filepath.Join(p, "*.jsonl"))
		if err != nil { // coverage-ignore: the pattern is fixed and valid
			return nil, err
		}
		slices.Sort(matches)
		files = append(files, matches...)
	}
	return files, nil
}

// jsonlRecords yields the records of files in order. An error names the
// file.
func jsonlRecords(files []string) iter.Seq2[audit.Record, error] {
	return func(yield func(audit.Record, error) bool) {
		for _, name := range files {
			f, err := os.Open(name)
			if err != nil {
				yield(audit.Record{}, err)
				return
			}
			for rec, err := range audit.ReadJSONL(f) {
				if err != nil {
					_ = f.Close()
					yield(audit.Record{}, fmt.Errorf("%s: %w", name, err))
					return
				}
				if !yield(rec, nil) {
					_ = f.Close()
					return
				}
			}
			_ = f.Close()
		}
	}
}
