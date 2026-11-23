package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/records"
)

const recordsCLIMetamodel = `version: "1.0"
entities:
  meter:
    label: Meter
    plural: meters
    id_prefix: "MTR-"
    id_type: manual
    properties:
      title: {type: string}
records:
  reading:
    owner: meter
    retention: 24h
    fields:
      kwh: {type: decimal, scale: 2}
`

func TestRecordsRequiresProject(t *testing.T) {
	require.True(t, requiresProject("records list reading MTR-1"))
}

func TestRecordsCLI(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".rela"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "metamodel.yaml"), []byte(recordsCLIMetamodel), 0o600))
	svc := openLeaseServices(t, root)
	e := entity.New("", "meter")
	e.SetString("title", "m")
	_, err := svc.EntityManager().CreateEntity(leaseCtx(), e, entity.CreateOptions{ID: "MTR-1"})
	require.NoError(t, err)

	old := time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339)
	_, err = runCLI(t, svc, "", "records", "append", "reading", "--owner", "MTR-1", "--ts", old, "-F", "kwh=9.5")
	require.NoError(t, err)
	recent := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	got, err := runCLI(t, svc, `{"rows":[{"owner":"MTR-1","ts":"`+recent+`","fields":{"kwh":1.25}}]}`,
		"-o", "json", "records", "append", "reading", "--file", "-")
	require.NoError(t, err)
	var appended struct {
		Records []records.Record `json:"records"`
	}
	require.NoError(t, json.Unmarshal([]byte(got), &appended), got)
	require.Equal(t, "1.25", appended.Records[0].Fields["kwh"])

	got, err = runCLI(t, svc, "", "-o", "json", "records", "list", "reading", "MTR-1", "--limit", "1")
	require.NoError(t, err)
	var page records.Page
	require.NoError(t, json.Unmarshal([]byte(got), &page), got)
	require.Len(t, page.Records, 1)
	require.NotEmpty(t, page.Next)

	table, err := runCLI(t, svc, "", "records", "list", "reading", "MTR-1")
	require.NoError(t, err)
	require.Contains(t, table, "kwh=1.25")

	got, err = runCLI(t, svc, "", "-o", "json", "records", "aggregate", "reading", "--field", "kwh",
		"--owner", "MTR-1", "--bucket", "day")
	require.NoError(t, err)
	var agg struct {
		Points []records.Point `json:"points"`
	}
	require.NoError(t, json.Unmarshal([]byte(got), &agg), got)
	require.Len(t, agg.Points, 2)
	require.Equal(t, int64(1), agg.Points[0].Count)

	got, err = runCLI(t, svc, "", "-o", "json", "records", "expire")
	require.NoError(t, err)
	require.JSONEq(t, `{"expired":1}`, got)
	got, err = runCLI(t, svc, "", "-o", "json", "records", "list", "reading", "MTR-1")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal([]byte(got), &page), got)
	require.Len(t, page.Records, 1)
}

func TestRecordsCLI_Refusals(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".rela"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "metamodel.yaml"), []byte(recordsCLIMetamodel), 0o600))
	svc := openLeaseServices(t, root)
	tests := []struct {
		name  string
		stdin string
		args  []string
		want  string
	}{
		{"no owner", "", []string{"append", "reading"}, "--owner is required"},
		{"owner and file", "", []string{"append", "reading", "--owner", "MTR-1", "--file", "-"}, "not both"},
		{"bad ts", "", []string{"append", "reading", "--owner", "MTR-1", "--ts", "x"}, "--ts"},
		{"bad file", "nope", []string{"append", "reading", "--file", "-"}, "--file"},
		{"missing owner entity", "", []string{"append", "reading", "--owner", "MTR-404", "-F", "kwh=1"}, "not found"},
		{"unknown type", "", []string{"list", "nope", "MTR-1"}, "not found"},
		{"bad range", "", []string{"list", "reading", "MTR-1", "--from", "x"}, "RFC 3339"},
		{"bad bucket", "", []string{"aggregate", "reading", "--bucket", "year"}, "bucket"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := runCLI(t, svc, tc.stdin, append([]string{"records"}, tc.args...)...)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestRecordsCLI_NoRecordTypes(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".rela"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "metamodel.yaml"), []byte(leaseCLIMetamodel), 0o600))
	svc := openLeaseServices(t, root)
	_, err := runCLI(t, svc, "", "records", "expire")
	require.ErrorIs(t, err, errNoRecordTypes)
}
