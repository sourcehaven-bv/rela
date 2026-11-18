// Package sqlaudittest is the conformance suite every sqlaudit adapter
// passes: pgaudit and sqliteaudit run [RunAll] over their own database.
package sqlaudittest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/audit/sqlaudit"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/sqlexec"
)

// Harness is one fresh, empty audit log.
type Harness struct {
	// Log is the log under test.
	Log *sqlaudit.Log
	// Raw runs statements on the same database, to inspect and tamper.
	Raw sqlexec.Querier
	// Second opens another log on the same database through an independent
	// handle, standing in for a second process. Nil when the backend is
	// single-process.
	Second func(t *testing.T) *sqlaudit.Log
}

// RunAll runs every conformance test. newHarness returns a fresh log per
// test.
func RunAll(t *testing.T, newHarness func(t *testing.T) Harness) {
	t.Helper()
	tests := []struct {
		name string
		fn   func(t *testing.T, h Harness)
	}{
		{"AppendThenVerify", testAppendThenVerify},
		{"StoresPreparedRecord", testStoresPreparedRecord},
		{"IndexesSubjects", testIndexesSubjects},
		{"NewestPages", testNewestPages},
		{"ForEntityPages", testForEntityPages},
		{"ForEntityOpFilter", testForEntityOpFilter},
		{"ForEntityLifecycle", testForEntityLifecycle},
		{"ForEntityFaces", testForEntityFaces},
		{"ForEntityFacelessStart", testForEntityFacelessStart},
		{"ForEntityCopyCreatesFace", testForEntityCopyCreatesFace},
		{"ForEntityConversionKeepsID", testForEntityConversionKeepsID},
		{"EditedRowBreaks", testEditedRowBreaks},
		{"ColumnEditsBreak", testColumnEditsBreak},
		{"DeletedRowBreaks", testDeletedRowBreaks},
		{"RelinkedRowBreaks", testRelinkedRowBreaks},
		{"PruneKeepsChainVerifiable", testPruneKeepsChainVerifiable},
		{"PruneNothing", testPruneNothing},
		{"PruneEverythingThenAppend", testPruneEverythingThenAppend},
		{"ForgedAnchorBreaks", testForgedAnchorBreaks},
		{"PruneRefusesBrokenChain", testPruneRefusesBrokenChain},
		{"PruneStopsAtFirstYoungRow", testPruneStopsAtFirstYoungRow},
		{"ImportRefusesMaintenanceRecords", testImportRefusesMaintenanceRecords},
		{"Import", testImport},
		{"ImportStopsAtError", testImportStopsAtError},
		{"SinkDrainsOnClose", testSinkDrainsOnClose},
		{"VerifyPagesLongLog", testVerifyPagesLongLog},
		{"ConcurrentAppends", testConcurrentAppends},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.fn(t, newHarness(t))
		})
	}
}

var t0 = time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)

func rec(i int) audit.Record {
	return audit.Record{
		Time:       t0.Add(time.Duration(i) * time.Second),
		Op:         audit.OpUpdateEntity,
		Subject:    &audit.Subject{Kind: "entity", Type: "control", ID: fmt.Sprintf("CTL-%d", i)},
		Principal:  principal.Principal{User: "alice", Tool: principal.ToolCLI},
		AssertedBy: "ci-runner",
		Summary:    "updated: status",
	}
}

func appendN(t *testing.T, l *sqlaudit.Log, from, n int) {
	t.Helper()
	for i := from; i < from+n; i++ {
		require.NoError(t, l.Append(context.Background(), rec(i)))
	}
}

func verifyOK(t *testing.T, l *sqlaudit.Log) sqlaudit.Verification {
	t.Helper()
	v, err := l.Verify(context.Background())
	require.NoError(t, err)
	require.Nil(t, v.Break, "chain must verify")
	return v
}

func verifyBreak(t *testing.T, l *sqlaudit.Log) *sqlaudit.Break {
	t.Helper()
	v, err := l.Verify(context.Background())
	require.NoError(t, err)
	require.NotNil(t, v.Break, "chain must not verify")
	return v.Break
}

func exec(t *testing.T, q sqlexec.Querier, stmt string, args ...any) {
	t.Helper()
	_, err := q.Exec(context.Background(), stmt, args...)
	require.NoError(t, err, stmt)
}

func count(t *testing.T, q sqlexec.Querier, stmt string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, q.QueryRow(context.Background(), stmt, args...).Scan(&n), stmt)
	return n
}

func testAppendThenVerify(t *testing.T, h Harness) {
	v := verifyOK(t, h.Log)
	require.Equal(t, sqlaudit.Verification{}, v, "an empty log verifies with no head")

	appendN(t, h.Log, 1, 3)
	v = verifyOK(t, h.Log)
	require.Equal(t, int64(3), v.Rows)
	require.Equal(t, int64(3), v.HeadSeq)

	// The head hash is reproducible from the stored rows alone.
	prev := ""
	for seq := int64(1); seq <= 3; seq++ {
		r := readRow(t, h, seq)
		prev = sqlaudit.ChainHash(prev, r)
	}
	require.Equal(t, prev, v.HeadHash)

	got, ok, err := h.Log.HashAt(context.Background(), 3)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, v.HeadHash, got)
	_, ok, err = h.Log.HashAt(context.Background(), 4)
	require.NoError(t, err)
	require.False(t, ok)
}

func testStoresPreparedRecord(t *testing.T, h Harness) {
	r := rec(1)
	r.Summary = "updated:\x00 status"
	require.NoError(t, h.Log.Append(context.Background(), r))

	var doc, op, user string
	require.NoError(t, h.Raw.QueryRow(context.Background(),
		`SELECT doc, op, principal_user FROM audit_log WHERE seq = 1`).Scan(&doc, &op, &user))
	want, err := json.Marshal(audit.Prepare(r))
	require.NoError(t, err)
	require.JSONEq(t, string(want), doc, "the stored doc is the prepared record, as the JSONL sink writes it")
	require.Equal(t, audit.OpUpdateEntity, op)
	require.Equal(t, "alice", user)

	var got audit.Record
	require.NoError(t, json.Unmarshal([]byte(doc), &got))
	require.NotContains(t, got.Summary, "\x00", "control characters are sanitized")
	require.Equal(t, principal.ToolCLI, got.Source)

	entries, err := h.Log.Records(context.Background(), 0, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, int64(1), entries[0].Seq)
	require.Equal(t, got, entries[0].Record)
}

func testIndexesSubjects(t *testing.T, h Harness) {
	ctx := context.Background()
	require.NoError(t, h.Log.Append(ctx, audit.Record{
		Op: audit.OpCreateRelation, Time: t0,
		Subject: &audit.Subject{Kind: "relation", RelationType: "mitigates", FromID: "CTL-1", ToID: "RSK-1"},
	}))
	require.NoError(t, h.Log.Append(ctx, audit.Record{
		Op: audit.OpRenameEntity, Time: t0,
		Before: &audit.Subject{Kind: "entity", Type: "control", ID: "CTL-1"},
		After:  &audit.Subject{Kind: "entity", Type: "control", ID: "CTL-9"},
	}))
	require.NoError(t, h.Log.Append(ctx, audit.Record{Op: audit.OpACLBypassRead, Time: t0}))

	require.Equal(t, int64(2), count(t, h.Raw, `SELECT COUNT(*) FROM audit_subjects WHERE entity_id = ?`, "CTL-1"))
	require.Equal(t, int64(1), count(t, h.Raw, `SELECT COUNT(*) FROM audit_subjects WHERE entity_id = ?`, "RSK-1"))
	require.Equal(t, int64(1), count(t, h.Raw, `SELECT COUNT(*) FROM audit_subjects WHERE entity_id = ?`, "CTL-9"))
	require.Equal(t, int64(0), count(t, h.Raw, `SELECT COUNT(*) FROM audit_subjects WHERE seq = 3`))
	require.Equal(t, int64(1), count(t, h.Raw,
		`SELECT COUNT(*) FROM audit_subjects WHERE starts AND entity_id = ?`, "CTL-9"), "a rename starts its After")
	require.Equal(t, int64(0), count(t, h.Raw,
		`SELECT COUNT(*) FROM audit_subjects WHERE starts AND entity_id = ?`, "CTL-1"))
}

// testNewestPages: Newest walks the log newest first, and a page's last
// seq is the cursor of the next.
func testNewestPages(t *testing.T, h Harness) {
	ctx := context.Background()
	appendN(t, h.Log, 1, 5)
	var seqs []int64
	for before := int64(0); ; {
		page, err := h.Log.Newest(ctx, before, 2)
		require.NoError(t, err)
		if len(page) == 0 {
			break
		}
		for _, e := range page {
			seqs = append(seqs, e.Seq)
		}
		before = page[len(page)-1].Seq
	}
	require.Equal(t, []int64{5, 4, 3, 2, 1}, seqs)
	page, err := h.Log.Newest(ctx, 3, 10)
	require.NoError(t, err)
	require.Len(t, page, 2)
	require.Equal(t, "CTL-2", page[0].Record.Subject.ID)
}

// changeOps are the ops the per-entity tests read.
var changeOps = []string{
	audit.OpCreateEntity, audit.OpUpdateEntity, audit.OpDeleteEntity, audit.OpRenameEntity,
	audit.OpCreateRelation, audit.OpCopyState, audit.OpConvertEntity,
}

func entityQuery(id, face string) sqlaudit.SubjectQuery {
	return sqlaudit.SubjectQuery{ID: id, Face: face, Ops: changeOps}
}

func seqsOf(entries []sqlaudit.Entry) []int64 {
	seqs := make([]int64, len(entries))
	for i, e := range entries {
		seqs[i] = e.Seq
	}
	return seqs
}

func appendAll(t *testing.T, l *sqlaudit.Log, recs ...audit.Record) {
	t.Helper()
	for _, r := range recs {
		if r.Time.IsZero() {
			r.Time = t0
		}
		require.NoError(t, l.Append(context.Background(), r))
	}
}

func entityRec(op, id, face string) audit.Record {
	return audit.Record{Op: op, Subject: &audit.Subject{Kind: "entity", Type: "control", ID: id, Face: face}}
}

// testForEntityPages: ForEntity returns the records that name an id in any
// position, newest first, and nothing for another id.
func testForEntityPages(t *testing.T, h Harness) {
	ctx := context.Background()
	appendN(t, h.Log, 1, 3) // CTL-1, CTL-2, CTL-3
	appendAll(t, h.Log, audit.Record{
		Op:      audit.OpCreateRelation,
		Subject: &audit.Subject{Kind: "relation", RelationType: "mitigates", FromID: "CTL-2", ToID: "RSK-1"},
	}, audit.Record{
		Op:     audit.OpRenameEntity,
		Before: &audit.Subject{Kind: "entity", Type: "control", ID: "CTL-2"},
		After:  &audit.Subject{Kind: "entity", Type: "control", ID: "CTL-7"},
	})

	got, err := h.Log.ForEntity(ctx, entityQuery("CTL-2", ""), 0, 10)
	require.NoError(t, err)
	require.Equal(t, []int64{5, 4, 2}, seqsOf(got))

	got, err = h.Log.ForEntity(ctx, entityQuery("CTL-2", ""), 5, 1)
	require.NoError(t, err)
	require.Equal(t, []int64{4}, seqsOf(got))

	got, err = h.Log.ForEntity(ctx, entityQuery("CTL-404", ""), 0, 10)
	require.NoError(t, err)
	require.Empty(t, got)

	got, err = h.Log.ForEntity(ctx, sqlaudit.SubjectQuery{ID: "CTL-2"}, 0, 10)
	require.NoError(t, err)
	require.Empty(t, got, "no ops reads nothing")
}

// testForEntityOpFilter: only the listed ops are read, and the page cut
// comes after the filter, so other ops cannot crowd a page out.
func testForEntityOpFilter(t *testing.T, h Harness) {
	ctx := context.Background()
	appendAll(t, h.Log, entityRec(audit.OpCreateEntity, "CTL-1", ""))
	for range 5 {
		appendAll(t, h.Log, entityRec(audit.OpDeniedWrite, "CTL-1", ""))
	}
	got, err := h.Log.ForEntity(ctx, entityQuery("CTL-1", ""), 0, 1)
	require.NoError(t, err)
	require.Equal(t, []int64{1}, seqsOf(got))
}

// testForEntityLifecycle: an id deleted and created again, or renamed away
// and reused, shows only the records of its latest lifecycle.
func testForEntityLifecycle(t *testing.T, h Harness) {
	ctx := context.Background()
	appendAll(t, h.Log,
		entityRec(audit.OpCreateEntity, "CTL-1", ""), // 1
		entityRec(audit.OpUpdateEntity, "CTL-1", ""), // 2
		entityRec(audit.OpDeleteEntity, "CTL-1", ""), // 3
		entityRec(audit.OpCreateEntity, "CTL-1", ""), // 4
		entityRec(audit.OpUpdateEntity, "CTL-1", ""), // 5
		entityRec(audit.OpCreateEntity, "CTL-2", ""), // 6
		audit.Record{ // 7
			Op:     audit.OpRenameEntity,
			Before: &audit.Subject{Kind: "entity", Type: "control", ID: "CTL-2"},
			After:  &audit.Subject{Kind: "entity", Type: "control", ID: "CTL-8"},
		},
		entityRec(audit.OpCreateEntity, "CTL-2", ""), // 8
		audit.Record{ // 9
			Op:     audit.OpRenameEntity,
			Before: &audit.Subject{Kind: "entity", Type: "control", ID: "CTL-3"},
			After:  &audit.Subject{Kind: "entity", Type: "control", ID: "CTL-1"},
		},
	)
	got, err := h.Log.ForEntity(ctx, entityQuery("CTL-2", ""), 0, 10)
	require.NoError(t, err)
	require.Equal(t, []int64{8}, seqsOf(got), "a reused id starts at its create")

	got, err = h.Log.ForEntity(ctx, entityQuery("CTL-8", ""), 0, 10)
	require.NoError(t, err)
	require.Equal(t, []int64{7}, seqsOf(got), "a renamed id starts at the rename")

	got, err = h.Log.ForEntity(ctx, entityQuery("CTL-1", ""), 0, 10)
	require.NoError(t, err)
	require.Equal(t, []int64{9}, seqsOf(got), "renaming into an id starts a new lifecycle")

	got, err = h.Log.ForEntity(ctx, entityQuery("CTL-1", ""), 9, 10)
	require.NoError(t, err)
	require.Empty(t, got, "a page before the fence is empty, not the predecessor's")
}

// testForEntityFaces: a face reads its own records and those stored without
// a face, never another face's.
func testForEntityFaces(t *testing.T, h Harness) {
	ctx := context.Background()
	appendAll(t, h.Log,
		entityRec(audit.OpCreateEntity, "DOC-1", "draft"),     // 1
		entityRec(audit.OpCreateEntity, "DOC-1", "published"), // 2
		entityRec(audit.OpUpdateEntity, "DOC-1", "published"), // 3
		audit.Record{ // 4: an edge from the draft face
			Op: audit.OpCreateRelation,
			Subject: &audit.Subject{
				Kind: "relation", RelationType: "refs", FromID: "DOC-1", FromFace: "draft", ToID: "DOC-2",
			},
		},
		audit.Record{ // 5: an edge into DOC-1, which has no head face
			Op:      audit.OpCreateRelation,
			Subject: &audit.Subject{Kind: "relation", RelationType: "refs", FromID: "DOC-2", ToID: "DOC-1"},
		},
		entityRec(audit.OpDeleteEntity, "DOC-1", "published"), // 6
		entityRec(audit.OpCreateEntity, "DOC-1", "published"), // 7
	)
	got, err := h.Log.ForEntity(ctx, entityQuery("DOC-1", "draft"), 0, 10)
	require.NoError(t, err)
	require.Equal(t, []int64{5, 4, 1}, seqsOf(got), "another face's records and fence do not apply")

	got, err = h.Log.ForEntity(ctx, entityQuery("DOC-1", "published"), 0, 10)
	require.NoError(t, err)
	require.Equal(t, []int64{7}, seqsOf(got))

	got, err = h.Log.ForEntity(ctx, entityQuery("DOC-2", ""), 0, 10)
	require.NoError(t, err)
	require.Equal(t, []int64{5, 4}, seqsOf(got))
}

// testForEntityFacelessStart: a start stored without a face (a record from
// before faces were recorded) is shown on every face but fences none, so it
// cannot hide a face's earlier records.
func testForEntityFacelessStart(t *testing.T, h Harness) {
	ctx := context.Background()
	appendAll(t, h.Log,
		entityRec(audit.OpCreateEntity, "DOC-1", "draft"), // 1
		entityRec(audit.OpUpdateEntity, "DOC-1", "draft"), // 2
		entityRec(audit.OpCreateEntity, "DOC-1", ""),      // 3
	)
	got, err := h.Log.ForEntity(ctx, entityQuery("DOC-1", "draft"), 0, 10)
	require.NoError(t, err)
	require.Equal(t, []int64{3, 2, 1}, seqsOf(got))
}

// testForEntityCopyCreatesFace: a copy that created a face starts that
// face's lifecycle, and LifecycleStart returns it; a copy onto an existing
// face does not.
func testForEntityCopyCreatesFace(t *testing.T, h Harness) {
	ctx := context.Background()
	copyRec := func(created bool) audit.Record {
		return audit.Record{
			Op:      audit.OpCopyState,
			Subject: &audit.Subject{Kind: "entity", Type: "doc", ID: "DOC-1", Face: "published"},
			Summary: audit.CopyStateSummary("promote", "DOC-1@draft", "DOC-1@published", created),
		}
	}
	appendAll(t, h.Log,
		entityRec(audit.OpCreateEntity, "DOC-1", "published"), // 1: an earlier family
		entityRec(audit.OpDeleteEntity, "DOC-1", "published"), // 2
		entityRec(audit.OpCreateEntity, "DOC-1", "draft"),     // 3
		copyRec(true),  // 4
		copyRec(false), // 5
	)
	got, err := h.Log.ForEntity(ctx, entityQuery("DOC-1", "published"), 0, 10)
	require.NoError(t, err)
	require.Equal(t, []int64{5, 4}, seqsOf(got))

	start, ok, err := h.Log.LifecycleStart(ctx, entityQuery("DOC-1", "published"))
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, int64(4), start.Seq)

	_, ok, err = h.Log.LifecycleStart(ctx, entityQuery("DOC-9", ""))
	require.NoError(t, err)
	require.False(t, ok)
}

// testForEntityConversionKeepsID: a conversion that keeps the id and maps
// one face onto another starts the face it lands on, so a deleted earlier
// entity at that face stays behind the fence.
func testForEntityConversionKeepsID(t *testing.T, h Harness) {
	ctx := context.Background()
	appendAll(t, h.Log,
		entityRec(audit.OpCreateEntity, "DOC-1", "done"),  // 1: an earlier family
		entityRec(audit.OpUpdateEntity, "DOC-1", "done"),  // 2
		entityRec(audit.OpDeleteEntity, "DOC-1", "done"),  // 3
		entityRec(audit.OpCreateEntity, "DOC-1", "draft"), // 4
		audit.Record{ // 5
			Op:     audit.OpConvertEntity,
			Before: &audit.Subject{Kind: "entity", Type: "doc", ID: "DOC-1", Face: "draft"},
			After:  &audit.Subject{Kind: "entity", Type: "task", ID: "DOC-1", Face: "done"},
		},
		entityRec(audit.OpUpdateEntity, "DOC-1", "done"), // 6
	)
	got, err := h.Log.ForEntity(ctx, entityQuery("DOC-1", "done"), 0, 10)
	require.NoError(t, err)
	require.Equal(t, []int64{6, 5}, seqsOf(got))

	start, ok, err := h.Log.LifecycleStart(ctx, entityQuery("DOC-1", "done"))
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, int64(5), start.Seq)
}

func testEditedRowBreaks(t *testing.T, h Harness) {
	appendN(t, h.Log, 1, 3)
	exec(t, h.Raw, `UPDATE audit_log SET doc = REPLACE(doc, 'alice', 'mallory') WHERE seq = 2`)
	b := verifyBreak(t, h.Log)
	require.Equal(t, int64(2), b.Seq)
	require.Contains(t, b.Reason, "content")
}

// testColumnEditsBreak: every stored column is covered by the hash, not
// only doc.
func testColumnEditsBreak(t *testing.T, h Harness) {
	for _, stmt := range []string{
		`UPDATE audit_log SET at = at - 1000000 WHERE seq = 2`,
		`UPDATE audit_log SET op = 'create-entity' WHERE seq = 2`,
		`UPDATE audit_log SET principal_user = 'mallory' WHERE seq = 2`,
	} {
		t.Run(stmt, func(t *testing.T) {
			h := h
			exec(t, h.Raw, `DELETE FROM audit_subjects`)
			exec(t, h.Raw, `DELETE FROM audit_log`)
			appendN(t, h.Log, 1, 3)
			exec(t, h.Raw, stmt)
			b := verifyBreak(t, h.Log)
			require.Equal(t, int64(2), b.Seq)
		})
	}
}

// readRow reads the hashed columns of the row at seq.
func readRow(t *testing.T, h Harness, seq int64) sqlaudit.Row {
	t.Helper()
	r := sqlaudit.Row{Seq: seq}
	var doc string
	require.NoError(t, h.Raw.QueryRow(context.Background(),
		`SELECT at, op, principal_user, doc FROM audit_log WHERE seq = ?`, seq).Scan(&r.At, &r.Op, &r.User, &doc))
	r.Doc = []byte(doc)
	return r
}

func testDeletedRowBreaks(t *testing.T, h Harness) {
	appendN(t, h.Log, 1, 3)
	exec(t, h.Raw, `DELETE FROM audit_log WHERE seq = 2`)
	b := verifyBreak(t, h.Log)
	require.Equal(t, int64(2), b.Seq)
	require.Contains(t, b.Reason, "missing")
}

// testRelinkedRowBreaks: a row whose content and hash were both rewritten
// to match still breaks the link from its successor.
func testRelinkedRowBreaks(t *testing.T, h Harness) {
	appendN(t, h.Log, 1, 3)
	var prev string
	require.NoError(t, h.Raw.QueryRow(context.Background(),
		`SELECT prev_hash FROM audit_log WHERE seq = 2`).Scan(&prev))
	r := readRow(t, h, 2)
	r.Doc = append(r.Doc, ' ')
	exec(t, h.Raw, `UPDATE audit_log SET doc = ?, hash = ? WHERE seq = 2`,
		string(r.Doc), sqlaudit.ChainHash(prev, r))
	b := verifyBreak(t, h.Log)
	require.Equal(t, int64(3), b.Seq)
	require.Contains(t, b.Reason, "link")
}

// clockAt returns h.Log stamping appends at at.
func clockAt(h Harness, at time.Time) *sqlaudit.Log {
	return h.Log.WithClock(func() time.Time { return at })
}

func testPruneKeepsChainVerifiable(t *testing.T, h Harness) {
	ctx := context.Background()
	appendN(t, clockAt(h, t0), 1, 2)
	appendN(t, clockAt(h, t0.Add(48*time.Hour)), 3, 2)

	res, err := clockAt(h, t0.Add(72*time.Hour)).Prune(ctx, t0.Add(24*time.Hour), rec(0))
	require.NoError(t, err)
	require.Equal(t, sqlaudit.Pruned{Rows: 2, Anchor: 2}, res)

	v := verifyOK(t, h.Log)
	require.Equal(t, int64(2), v.Anchor)
	require.Equal(t, int64(3), v.Rows, "two kept rows and the prune record")
	require.Equal(t, int64(5), v.HeadSeq)
	require.Equal(t, int64(0), count(t, h.Raw, `SELECT COUNT(*) FROM audit_subjects WHERE seq <= 2`))

	var doc string
	require.NoError(t, h.Raw.QueryRow(ctx, `SELECT doc FROM audit_log WHERE seq = 5`).Scan(&doc))
	var got audit.Record
	require.NoError(t, json.Unmarshal([]byte(doc), &got))
	require.Equal(t, audit.OpAuditPrune, got.Op)
	require.Contains(t, got.Summary, "rows=2")

	// Deleting the first kept row is still detected after the prune.
	exec(t, h.Raw, `DELETE FROM audit_log WHERE seq = 3`)
	require.Equal(t, int64(3), verifyBreak(t, h.Log).Seq)
}

func testPruneNothing(t *testing.T, h Harness) {
	appendN(t, clockAt(h, t0), 1, 2)
	res, err := h.Log.Prune(context.Background(), t0.Add(-time.Hour), rec(0))
	require.NoError(t, err)
	require.Equal(t, sqlaudit.Pruned{}, res)
	require.Equal(t, int64(2), verifyOK(t, h.Log).Rows, "no prune record when nothing was pruned")
}

func testPruneEverythingThenAppend(t *testing.T, h Harness) {
	ctx := context.Background()
	appendN(t, clockAt(h, t0), 1, 2)
	_, err := clockAt(h, t0.Add(time.Hour)).Prune(ctx, t0.Add(time.Minute), rec(0))
	require.NoError(t, err)
	// A second prune takes the first prune row with it and records its own.
	res, err := clockAt(h, t0.Add(3*time.Hour)).Prune(ctx, t0.Add(2*time.Hour), rec(0))
	require.NoError(t, err)
	require.Equal(t, sqlaudit.Pruned{Rows: 1, Anchor: 3}, res)

	appendN(t, h.Log, 9, 1)
	v := verifyOK(t, h.Log)
	require.Equal(t, int64(3), v.Anchor)
	require.Equal(t, int64(5), v.HeadSeq, "seq continues after the anchor")
}

// testForgedAnchorBreaks: deleting the oldest rows and pointing the anchor
// at the last deleted one, without a prune, does not verify.
func testForgedAnchorBreaks(t *testing.T, h Harness) {
	appendN(t, h.Log, 1, 4)
	var hash string
	require.NoError(t, h.Raw.QueryRow(context.Background(),
		`SELECT hash FROM audit_log WHERE seq = 2`).Scan(&hash))
	exec(t, h.Raw, `DELETE FROM audit_subjects WHERE seq <= 2`)
	exec(t, h.Raw, `DELETE FROM audit_log WHERE seq <= 2`)
	exec(t, h.Raw, `INSERT INTO audit_anchor (id, seq, hash, pruned_at) VALUES (1, 2, ?, 0)`, hash)
	b := verifyBreak(t, h.Log)
	require.Equal(t, int64(2), b.Seq)
	require.Contains(t, b.Reason, "audit-prune")
}

// testPruneRefusesBrokenChain: a prune does not delete a row that fails
// verification, which would erase the evidence.
func testPruneRefusesBrokenChain(t *testing.T, h Harness) {
	appendN(t, clockAt(h, t0), 1, 3)
	exec(t, h.Raw, `UPDATE audit_log SET principal_user = 'mallory' WHERE seq = 2`)
	_, err := h.Log.Prune(context.Background(), t0.Add(time.Hour), rec(0))
	require.ErrorIs(t, err, sqlaudit.ErrChainBroken)
	require.ErrorContains(t, err, "seq 2")
	require.Equal(t, int64(3), count(t, h.Raw, `SELECT COUNT(*) FROM audit_log`), "nothing deleted")
}

// testPruneStopsAtFirstYoungRow: a row stamped with a clock far behind
// does not pull the newer rows before it into the prune.
func testPruneStopsAtFirstYoungRow(t *testing.T, h Harness) {
	appendN(t, clockAt(h, t0), 1, 1)
	appendN(t, clockAt(h, t0.Add(48*time.Hour)), 2, 2)
	appendN(t, clockAt(h, time.Unix(0, 0)), 4, 1)
	res, err := h.Log.Prune(context.Background(), t0.Add(24*time.Hour), rec(0))
	require.NoError(t, err)
	require.Equal(t, sqlaudit.Pruned{Rows: 1, Anchor: 1}, res)
	verifyOK(t, h.Log)
}

func records(n, failAt int) iter.Seq2[audit.Record, error] {
	return func(yield func(audit.Record, error) bool) {
		for i := 1; i <= n; i++ {
			if i == failAt {
				yield(audit.Record{}, errors.New("bad line"))
				return
			}
			if !yield(rec(i), nil) {
				return
			}
		}
	}
}

func testImport(t *testing.T, h Harness) {
	n, err := h.Log.Import(context.Background(), records(1203, 0), rec(0), "files=2")
	require.NoError(t, err)
	require.Equal(t, int64(1203), n)
	v := verifyOK(t, h.Log)
	require.Equal(t, int64(1204), v.Rows)

	var doc string
	require.NoError(t, h.Raw.QueryRow(context.Background(), `SELECT doc FROM audit_log WHERE seq = 1`).Scan(&doc))
	var first audit.Record
	require.NoError(t, json.Unmarshal([]byte(doc), &first))
	require.True(t, first.Time.Equal(rec(1).Time), "an imported record keeps its own time")
	require.Equal(t, rec(1).Time.UnixMicro(), readRow(t, h, 1).At, "and is appended at it, for retention")
	require.NoError(t, h.Raw.QueryRow(context.Background(), `SELECT doc FROM audit_log WHERE seq = 1204`).Scan(&doc))
	var last audit.Record
	require.NoError(t, json.Unmarshal([]byte(doc), &last))
	require.Equal(t, audit.OpAuditImport, last.Op)
	require.Equal(t, "imported records=1203 files=2", last.Summary)
}

func testImportRefusesMaintenanceRecords(t *testing.T, h Harness) {
	bad := func(yield func(audit.Record, error) bool) {
		if !yield(rec(1), nil) {
			return
		}
		yield(audit.Record{Op: audit.OpAuditPrune, Summary: "pruned through seq=9 hash=x"}, nil)
	}
	_, err := h.Log.Import(context.Background(), bad, rec(0), "")
	require.ErrorContains(t, err, "audit-prune")
	require.Equal(t, int64(0), count(t, h.Raw, `SELECT COUNT(*) FROM audit_log`), "the import rolled back")
}

func testImportStopsAtError(t *testing.T, h Harness) {
	n, err := h.Log.Import(context.Background(), records(10, 4), rec(0), "")
	require.Error(t, err)
	require.Equal(t, int64(0), n, "the open batch is not written")
	require.Equal(t, int64(0), verifyOK(t, h.Log).Rows, "no import record after a failure")
}

func testSinkDrainsOnClose(t *testing.T, h Harness) {
	sink, err := sqlaudit.NewSink(h.Log)
	require.NoError(t, err)
	for i := 1; i <= 50; i++ {
		sink.Record(rec(i))
	}
	require.NoError(t, sink.Close())
	require.NoError(t, sink.Close(), "Close is idempotent")
	sink.Record(rec(51)) // dropped and logged

	v := verifyOK(t, h.Log)
	require.Equal(t, int64(50), v.Rows)
	var doc string
	require.NoError(t, h.Raw.QueryRow(context.Background(), `SELECT doc FROM audit_log WHERE seq = 50`).Scan(&doc))
	var last audit.Record
	require.NoError(t, json.Unmarshal([]byte(doc), &last))
	require.Equal(t, "CTL-50", last.Subject.ID, "the sink appends in Record order")
}

func testVerifyPagesLongLog(t *testing.T, h Harness) {
	_, err := h.Log.Import(context.Background(), records(2500, 0), rec(0), "")
	require.NoError(t, err)
	require.Equal(t, int64(2501), verifyOK(t, h.Log).Rows)
	exec(t, h.Raw, `DELETE FROM audit_log WHERE seq = 2001`)
	require.Equal(t, int64(2001), verifyBreak(t, h.Log).Seq, "a break past the first page is found")
}

// testConcurrentAppends: appends from several goroutines, and from a second
// process where the backend has one, form one gapless chain.
func testConcurrentAppends(t *testing.T, h Harness) {
	logs := []*sqlaudit.Log{h.Log}
	if h.Second != nil {
		logs = append(logs, h.Second(t))
	}
	const perWorker = 10
	workers := 4
	var wg sync.WaitGroup
	errs := make(chan error, workers*perWorker)
	for w := range workers {
		wg.Add(1)
		go func(l *sqlaudit.Log) {
			defer wg.Done()
			for i := range perWorker {
				if err := l.Append(context.Background(), rec(w*perWorker+i)); err != nil {
					errs <- err
				}
			}
		}(logs[w%len(logs)])
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, int64(workers*perWorker), verifyOK(t, h.Log).Rows)
}
