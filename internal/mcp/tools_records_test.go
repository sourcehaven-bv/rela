package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/records/filerecords"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

const recordsTestSchema = `
version: "1"
entities:
  site:
    label: Site
    id_prefix: "SITE-"
    properties:
      name: {type: string}
records:
  reading:
    owner: site
    fields:
      kwh: {type: decimal, scale: 2}
      ok: {type: boolean}
`

// recordOwners is a fake owner gate: hidden ids are unreadable, readOnly
// ids are readable but not writable.
type recordOwners struct {
	hidden, readOnly map[string]bool
}

func (o recordOwners) Readable(_ context.Context, ids []string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, id := range ids {
		if strings.HasPrefix(id, "SITE-") && !o.hidden[id] {
			out[id] = true
		}
	}
	return out, nil
}

func (o recordOwners) Writable(_ context.Context, id string) (records.Owner, error) {
	if !strings.HasPrefix(id, "SITE-") || o.hidden[id] {
		return records.Owner{}, records.ErrNotFound
	}
	if o.readOnly[id] {
		return records.Owner{}, records.ErrForbidden
	}
	return records.Owner{ID: id, Type: "site", Faces: []map[string]any{{}}}, nil
}

func newRecordsSession(t *testing.T, owners recordOwners) *Server {
	t.Helper()
	meta, err := metamodel.Parse([]byte(recordsTestSchema))
	require.NoError(t, err)
	st, err := filerecords.New(storage.NewMemFS(), "/records", filelock.ProcessPrivate{})
	require.NoError(t, err)
	svc, err := records.NewService(records.Deps{Store: st, Owners: owners, Meta: meta, Audit: audit.NewMemory()})
	require.NoError(t, err)
	m, fixture := makeTestFixture(t)
	deps := newTestDeps(t, m, fixture)
	deps.Records = svc
	srv, err := NewServer(deps, "test", WithPrincipal(principal.Principal{User: "tester", Tool: principal.ToolMCP}))
	require.NoError(t, err)
	return srv
}

func TestRecordTools(t *testing.T) {
	owners := recordOwners{hidden: map[string]bool{"SITE-9": true}, readOnly: map[string]bool{"SITE-3": true}}
	cs := connect(t, newRecordsSession(t, owners))

	text, isErr := callToolOn(t, cs, "list_record_types", `{}`)
	require.False(t, isErr, text)
	require.Contains(t, text, `"key": "reading"`)

	text, isErr = callToolOn(t, cs, "append_records", `{"type":"reading","rows":[
		{"owner":"SITE-1","ts":"2026-11-01T00:00:00Z","fields":{"kwh":"1.25","ok":true}},
		{"owner":"SITE-1","ts":"2026-11-02T00:00:00Z","fields":{"kwh":2.5}},
		{"owner":"SITE-2","ts":"2026-11-02T00:00:00Z","fields":{"kwh":"10"}}]}`)
	require.False(t, isErr, text)
	var stored struct {
		Records []records.Record `json:"records"`
	}
	require.NoError(t, json.Unmarshal([]byte(text), &stored))
	require.Len(t, stored.Records, 3)
	require.Equal(t, "1.25", stored.Records[0].Fields["kwh"], "a decimal string stays exact")

	text, isErr = callToolOn(t, cs, "list_records", `{"type":"reading","owner":"SITE-1","limit":1}`)
	require.False(t, isErr, text)
	var page records.Page
	require.NoError(t, json.Unmarshal([]byte(text), &page))
	require.Len(t, page.Records, 1)
	require.NotEmpty(t, page.Next)

	text, isErr = callToolOn(t, cs, "list_records",
		`{"type":"reading","owner":"SITE-1","after":"`+page.Next+`","from":"2026-11-01T12:00:00Z"}`)
	require.False(t, isErr, text)
	require.NoError(t, json.Unmarshal([]byte(text), &page))
	require.Len(t, page.Records, 1)

	text, isErr = callToolOn(t, cs, "aggregate_records", `{"type":"reading","field":"kwh"}`)
	require.False(t, isErr, text)
	var agg struct {
		Points []records.Point `json:"points"`
	}
	require.NoError(t, json.Unmarshal([]byte(text), &agg))
	require.Len(t, agg.Points, 1)
	require.Equal(t, int64(3), agg.Points[0].Count)
	require.Equal(t, "13.75", agg.Points[0].Sum)

	text, isErr = callToolOn(t, cs, "aggregate_records", `{"type":"reading","per_owner":true,"bucket":"day"}`)
	require.False(t, isErr, text)
	require.NoError(t, json.Unmarshal([]byte(text), &agg))
	require.Len(t, agg.Points, 3)
}

func TestRecordTools_Refusals(t *testing.T) {
	owners := recordOwners{hidden: map[string]bool{"SITE-9": true}, readOnly: map[string]bool{"SITE-3": true}}
	cs := connect(t, newRecordsSession(t, owners))
	tests := []struct {
		name, tool, args, want string
	}{
		{"append read-only owner", "append_records",
			`{"type":"reading","rows":[{"owner":"SITE-3","fields":{"kwh":"1"}}]}`, "you may not change"},
		{"append hidden owner", "append_records",
			`{"type":"reading","rows":[{"owner":"SITE-9","fields":{"kwh":"1"}}]}`, "not found"},
		{"append unknown type", "append_records", `{"type":"nope","rows":[{"owner":"SITE-1"}]}`, "not found"},
		{"append no rows", "append_records", `{"type":"reading","rows":[]}`, "at least one row"},
		{"append bad field", "append_records",
			`{"type":"reading","rows":[{"owner":"SITE-1","fields":{"kwh":"abc"}}]}`, "kwh"},
		{"list hidden owner", "list_records", `{"type":"reading","owner":"SITE-9"}`, "not found"},
		{"list bad time", "list_records", `{"type":"reading","owner":"SITE-1","from":"yesterday"}`, "RFC 3339"},
		{"aggregate bad bucket", "aggregate_records", `{"type":"reading","bucket":"year"}`, "bucket"},
		{"aggregate hidden owner", "aggregate_records", `{"type":"reading","owners":["SITE-9"]}`, "not found"},
		{"aggregate non-numeric field", "aggregate_records", `{"type":"reading","field":"ok"}`, "not a numeric"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			text, isErr := callToolOn(t, cs, tc.tool, tc.args)
			require.True(t, isErr, text)
			require.Contains(t, text, tc.want)
		})
	}
}

func TestRecordTools_AbsentWithoutService(t *testing.T) {
	cs := connect(t, newDispatchServer(t))
	res, err := cs.ListTools(context.Background(), nil)
	require.NoError(t, err)
	for _, tool := range res.Tools {
		require.NotContains(t, tool.Name, "record")
	}
}

func TestRecordTools_AreWatchedForNameDrift(t *testing.T) {
	for _, name := range []string{"list_record_types", "append_records", "list_records", "aggregate_records"} {
		require.True(t, knownToolNames[name], name)
	}
}
