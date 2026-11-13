package transitions

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func TestHeadless_Accessors(t *testing.T) {
	meta := snapshotMeta()
	// A second entity type whose property follows the same machine: one
	// headless flow, two uses.
	meta.Entities["draft"] = metamodel.EntityDef{Properties: map[string]metamodel.PropertyDef{
		"phase": {Type: "snapshot-status"},
	}}
	set := mustCompile(t, meta)

	all := set.Headless()
	require.Len(t, all, 1)
	hf, ok := set.HeadlessFlow("snapshot-status")
	require.True(t, ok)
	require.Same(t, all[0], hf)
	require.Equal(t, "snapshot-status", hf.Name())
	require.Equal(t, "in-review", hf.Initial())
	require.Equal(t, []string{"in-review", "approved", "established", "obsolete"}, hf.States())
	require.Equal(t, []string{"draft.phase", "snapshot.status"}, set.Uses("snapshot-status"))

	_, ok = set.HeadlessFlow("missing")
	require.False(t, ok)
	require.Empty(t, set.Uses("missing"))
}

func TestHeadless_NilTransitions(t *testing.T) {
	var set *Transitions
	require.Nil(t, set.Headless())
	_, ok := set.HeadlessFlow("x")
	require.False(t, ok)
	require.Nil(t, set.Uses("x"))
	require.Empty(t, NoTransitions().Headless())
}

func TestHeadless_Mermaid(t *testing.T) {
	meta := snapshotMeta()
	ct := meta.Types["snapshot-status"]
	ct.Transitions[0].Label = "Approve"
	meta.Types["snapshot-status"] = ct
	hf, ok := mustCompile(t, meta).HeadlessFlow("snapshot-status")
	require.True(t, ok)

	got := hf.Mermaid()
	require.True(t, strings.HasPrefix(got, "stateDiagram-v2"), got)
	for _, want := range []string{
		"Approve needs approve",
		"needs establish when count_relations(entity, #quot;signed-by#quot;) #gt; 0",
	} {
		require.Contains(t, got, want)
	}
	// Every transition is an edge; the entry arrow is the fifth.
	require.Equal(t, 5, strings.Count(got, "-->"), got)
}

// The prefix keeps a headless name apart from every flows.yaml name.
func TestHeadlessPrefix_NeverAFlowName(t *testing.T) {
	require.False(t, flow.ValidName(HeadlessPrefix+"status"))
}
