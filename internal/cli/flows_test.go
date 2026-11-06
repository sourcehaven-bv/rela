package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/projectsetup"
)

const cliFlowsYAML = `flows:
  review:
    label: Review
    subject: item
    initial: open
    states:
      open:
        on:
          change: { to: closed, when: "entity.title == 'done'" }
      closed:
        final: true
`

func compileCLIFlows(t *testing.T) *flowcfg.Compiled {
	t.Helper()
	meta, err := metamodel.Parse([]byte("entities:\n  item:\n    label: Item\n    id_type: sequential\n    id_prefix: ITEM-\n" +
		"    properties:\n      title: { type: string }\nrelations: {}\n"))
	require.NoError(t, err)
	c, err := flowcfg.Parse([]byte(cliFlowsYAML), meta, nil)
	require.NoError(t, err)
	return c
}

// TestFlowShow_Write pins the list and the diagram, in text and JSON.
func TestFlowShow_Write(t *testing.T) {
	flows := compileCLIFlows(t)
	cases := []struct {
		name, flow string
		format     output.Format
		check      func(t *testing.T, got string)
	}{
		{"list text", "", output.FormatTable, func(t *testing.T, got string) {
			t.Helper()
			require.Contains(t, got, "review\tReview\tsubject: item")
			require.Contains(t, got, "states: open, closed")
		}},
		{"list json", "", output.FormatJSON, func(t *testing.T, got string) {
			t.Helper()
			var list []flowSummary
			require.NoError(t, json.Unmarshal([]byte(got), &list))
			require.Len(t, list, 1)
			require.Equal(t, "open", list[0].Initial)
			require.Empty(t, list[0].Mermaid)
		}},
		{"diagram", "review", output.FormatTable, func(t *testing.T, got string) {
			t.Helper()
			require.Contains(t, got, "stateDiagram-v2")
			require.Contains(t, got, "s0 --> s1: change when")
		}},
		{"diagram json", "review", output.FormatJSON, func(t *testing.T, got string) {
			t.Helper()
			var s flowSummary
			require.NoError(t, json.Unmarshal([]byte(got), &s))
			require.Contains(t, s.Mermaid, "stateDiagram-v2")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withOutput(t, tc.format)
			var buf bytes.Buffer
			require.NoError(t, (&FlowShowCmd{Name: tc.flow}).write(&buf, flows))
			tc.check(t, buf.String())
		})
	}
	t.Run("unknown flow", func(t *testing.T) {
		withOutput(t, output.FormatTable)
		err := (&FlowShowCmd{Name: "nope"}).write(&bytes.Buffer{}, flows)
		require.ErrorContains(t, err, `no flow "nope"`)
	})
	t.Run("no flows", func(t *testing.T) {
		withOutput(t, output.FormatTable)
		var buf bytes.Buffer
		require.NoError(t, (&FlowShowCmd{}).write(&buf, nil))
		require.Equal(t, "No flows declared.\n", buf.String())
	})
}

// TestValidate_Flows pins the `rela validate` check of flows.yaml: absent
// is no check, valid passes, a problem fails.
func TestValidate_Flows(t *testing.T) {
	root := seedProject(t)
	check := func() bool {
		t.Helper()
		result, err := projectsetup.Validate(context.Background(), root)
		require.NoError(t, err)
		require.NotNil(t, result.Metamodel)
		return reportFlowsValidation(context.Background(), result, false)
	}
	require.False(t, check(), "no flows.yaml")
	writeProjectFile(t, root, "flows.yaml", cliFlowsYAML)
	require.False(t, check())
	writeProjectFile(t, root, "flows.yaml", "flows:\n  review:\n    subject: nope\n")
	require.True(t, check())
}
