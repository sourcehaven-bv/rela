package audit_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/audit"
)

func TestReadJSONL(t *testing.T) {
	in := `{"op":"create-entity"}` + "\n\n" + `{"op":"update-entity"}` + "\n" + `{bad` + "\n"
	var ops []string
	var gotErr error
	for rec, err := range audit.ReadJSONL(strings.NewReader(in)) {
		if err != nil {
			gotErr = err
			continue
		}
		ops = append(ops, rec.Op)
	}
	require.Equal(t, []string{"create-entity", "update-entity"}, ops)
	require.ErrorContains(t, gotErr, "line 4")

	n := 0
	for range audit.ReadJSONL(strings.NewReader(in)) {
		n++
		break
	}
	require.Equal(t, 1, n, "the sequence stops when the consumer does")

	long := `{"summary":"` + strings.Repeat("x", 2<<20) + `"}`
	for _, err := range audit.ReadJSONL(strings.NewReader(long)) {
		require.Error(t, err, "a line over the limit is an error")
	}
}

func TestPrepareSanitizes(t *testing.T) {
	got := audit.Prepare(audit.Record{Summary: "a\x00b"})
	require.NotContains(t, got.Summary, "\x00")
}
