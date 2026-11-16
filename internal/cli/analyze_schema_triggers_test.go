package cli

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestAnalyzeSchema_TriggersDoNotLoad pins what `rela analyze schema` does
// when flows.yaml has a trigger that does not load. The report goes on
// without the trigger references and warns. Cleanup refuses, because a type
// only a trigger names would look unused and be removed.
func TestAnalyzeSchema_TriggersDoNotLoad(t *testing.T) {
	const broken = "triggers:\n  bad:\n    on: {entity: item, created: true}\n" +
		"    outputs:\n      - set: {properties: {nope: x}}\n      - nope: {}\n"
	tests := []struct {
		name    string
		cmd     AnalyzeSchemaCmd
		wantErr string
		wantOut string
	}{
		{name: "report", cmd: AnalyzeSchemaCmd{}, wantOut: "Trigger references are left out"},
		{name: "cleanup", cmd: AnalyzeSchemaCmd{Cleanup: true, DryRun: true}, wantErr: "cannot clean up"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buf := withOutput(t, output.FormatTable)
			svc := flowReadServices(t, broken)
			svc.Families = store.TrivialScope()
			err := tc.cmd.Run(svc)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Contains(t, buf.String(), tc.wantOut)
		})
	}
}
