package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/projectsetup"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
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
			require.NoError(t, (&FlowShowCmd{Name: tc.flow}).write(&buf, flows, nil))
			tc.check(t, buf.String())
		})
	}
	t.Run("unknown flow", func(t *testing.T) {
		withOutput(t, output.FormatTable)
		err := (&FlowShowCmd{Name: "nope"}).write(&bytes.Buffer{}, flows, nil)
		require.ErrorContains(t, err, `no flow "nope"`)
	})
	t.Run("no flows", func(t *testing.T) {
		withOutput(t, output.FormatTable)
		var buf bytes.Buffer
		require.NoError(t, (&FlowShowCmd{}).write(&buf, nil, nil))
		require.Equal(t, "No flows declared.\n", buf.String())
	})
}

func compileCLIHeadless(t *testing.T) *flowcfg.Transitions {
	t.Helper()
	meta, err := metamodel.Parse([]byte(`types:
  item-status:
    base: enum
    values: [open, closed]
    initial: open
    transitions:
      - { from: open, to: closed, guard: item.close }
entities:
  item:
    label: Item
    id_type: sequential
    id_prefix: ITEM-
    properties:
      status: { type: item-status }
relations: {}
`))
	require.NoError(t, err)
	tr, err := flowcfg.CompileTransitions(meta)
	require.NoError(t, err)
	return tr
}

// TestFlowShow_Headless pins the headless flows beside the flows.yaml ones:
// listed with their kind and uses, rendered by their schema. name, and
// listed when there is no flows.yaml.
func TestFlowShow_Headless(t *testing.T) {
	flows, headless := compileCLIFlows(t), compileCLIHeadless(t)
	run := func(t *testing.T, format output.Format, name string, flows *flowcfg.Compiled) (string, error) {
		t.Helper()
		withOutput(t, format)
		var buf bytes.Buffer
		err := (&FlowShowCmd{Name: name}).write(&buf, flows, headless)
		return buf.String(), err
	}
	t.Run("list json kinds", func(t *testing.T) {
		got, err := run(t, output.FormatJSON, "", flows)
		require.NoError(t, err)
		var list []flowSummary
		require.NoError(t, json.Unmarshal([]byte(got), &list))
		require.Len(t, list, 2)
		require.Equal(t, flowKindFlow, list[0].Kind)
		require.Equal(t, "item", list[0].Subject)
		require.Equal(t, flowKindHeadless, list[1].Kind)
		require.Equal(t, "schema.item-status", list[1].Name)
		require.Equal(t, []string{"item.status"}, list[1].Uses)
		require.Empty(t, list[1].Subject)
		require.Equal(t, []string{"open", "closed"}, list[1].States)
	})
	t.Run("list text without flows.yaml", func(t *testing.T) {
		got, err := run(t, output.FormatTable, "", nil)
		require.NoError(t, err)
		require.Equal(t, "schema.item-status\theadless\tuses: item.status\tinitial: open\tstates: open, closed\n", got)
	})
	t.Run("diagram", func(t *testing.T) {
		got, err := run(t, output.FormatTable, "schema.item-status", nil)
		require.NoError(t, err)
		require.Contains(t, got, "s0 --> s1: needs item.close")
	})
	t.Run("unknown headless", func(t *testing.T) {
		_, err := run(t, output.FormatTable, "schema.nope", flows)
		require.ErrorContains(t, err, `no flow "schema.nope"`)
	})
	t.Run("bare type name names the headless flow", func(t *testing.T) {
		_, err := run(t, output.FormatTable, "item-status", flows)
		require.ErrorContains(t, err, `no flow "item-status"`)
		require.ErrorContains(t, err, `"schema.item-status"`)
	})
}

// cliFlowSchema has a flows.yaml subject (item.title) and a headless flow
// (item-status).
const cliFlowSchema = `types:
  item-status:
    base: enum
    values: [open, closed]
    initial: open
    transitions:
      - { from: open, to: closed }
entities:
  item:
    label: Item
    id_type: sequential
    id_prefix: ITEM-
    properties:
      title: { type: string }
      status: { type: item-status }
relations: {}
`

// flowReadServices returns the services `rela flow` reads: cliFlowSchema, and
// flowsYAML as the project's flows.yaml unless it is empty.
func flowReadServices(t *testing.T, flowsYAML string) *readServices {
	t.Helper()
	meta, err := metamodel.Parse([]byte(cliFlowSchema))
	require.NoError(t, err)
	root := t.TempDir()
	if flowsYAML != "" {
		writeProjectFile(t, root, flowcfg.FileName, flowsYAML)
	}
	return &readServices{
		Meta: meta, Store: memstore.New(), World: store.TrivialScope(),
		Config: config.NewFSLoader(storage.NewOsFS(), root),
	}
}

const brokenFlowsYAML = "flows:\n  review:\n    subject: nope\n"

// TestFlowShow_Run pins how show combines the two sources: a headless name
// never reads flows.yaml, a broken flows.yaml still lists the headless flows
// as text but writes no partial JSON list, and a bare name fails on it.
func TestFlowShow_Run(t *testing.T) {
	ctx := context.Background()
	show := func(t *testing.T, format output.Format, name, flowsYAML string) (string, error) {
		t.Helper()
		withOutput(t, format)
		var buf bytes.Buffer
		err := (&FlowShowCmd{Name: name}).run(ctx, flowReadServices(t, flowsYAML), &buf)
		return buf.String(), err
	}
	t.Run("list both", func(t *testing.T) {
		got, err := show(t, output.FormatTable, "", cliFlowsYAML)
		require.NoError(t, err)
		require.Contains(t, got, "review\tReview")
		require.Contains(t, got, flowcfg.HeadlessPrefix+"item-status\theadless")
	})
	t.Run("headless name skips a broken flows.yaml", func(t *testing.T) {
		got, err := show(t, output.FormatTable, flowcfg.HeadlessPrefix+"item-status", brokenFlowsYAML)
		require.NoError(t, err)
		require.Contains(t, got, "stateDiagram")
	})
	t.Run("text list with a broken flows.yaml", func(t *testing.T) {
		got, err := show(t, output.FormatTable, "", brokenFlowsYAML)
		require.Error(t, err)
		require.Contains(t, got, flowcfg.HeadlessPrefix+"item-status")
	})
	t.Run("json list with a broken flows.yaml", func(t *testing.T) {
		got, err := show(t, output.FormatJSON, "", brokenFlowsYAML)
		require.Error(t, err)
		require.Empty(t, got)
	})
	t.Run("bare name with a broken flows.yaml", func(t *testing.T) {
		got, err := show(t, output.FormatTable, "review", brokenFlowsYAML)
		require.Error(t, err)
		require.Empty(t, got)
	})
}

// TestFlowValidate_Run pins the validate report over both sources.
func TestFlowValidate_Run(t *testing.T) {
	ctx := context.Background()
	t.Run("valid", func(t *testing.T) {
		buf := withOutput(t, output.FormatTable)
		require.NoError(t, (&FlowValidateCmd{}).Run(ctx, flowReadServices(t, cliFlowsYAML)))
		require.Contains(t, buf.String(), "1 flow(s)")
		require.Contains(t, buf.String(), "1 headless flow(s)")
	})
	t.Run("no flows.yaml", func(t *testing.T) {
		buf := withOutput(t, output.FormatTable)
		require.NoError(t, (&FlowValidateCmd{}).Run(ctx, flowReadServices(t, "")))
		require.Contains(t, buf.String(), "No flows.yaml found.")
		require.Contains(t, buf.String(), "1 headless flow(s)")
	})
	t.Run("broken flows.yaml", func(t *testing.T) {
		buf := withOutput(t, output.FormatTable)
		err := (&FlowValidateCmd{}).Run(ctx, flowReadServices(t, brokenFlowsYAML))
		require.ErrorContains(t, err, "problem(s)")
		require.NotContains(t, buf.String(), "headless flow(s)")
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
