package realtime

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// TestDescribe_ReusesMessageTypes pins that Describe serves the timing and
// reply rule a channel read serves, sorts properties required first, and
// carries labels and promote targets.
func TestDescribe_ReusesMessageTypes(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	types := map[string]ChannelTypeView{}
	for _, ct := range h.svc.Describe() {
		types[ct.Name] = ct
	}
	roomType := types["room"]
	require.Equal(t, "Room", roomType.Label)
	require.Equal(t, string(realtimecfg.CardinalityOne), roomType.Cardinality)
	require.True(t, roomType.RemoveReasonRequired)
	require.Empty(t, roomType.Lifecycle)

	read, err := h.svc.Read(as(user("bob")), room, ReadRequest{Schemas: true})
	require.NoError(t, err)
	byName := map[string]MessageTypeDescription{}
	for _, mt := range roomType.MessageTypes {
		byName[mt.Name] = mt
	}
	require.Len(t, byName, len(read.MessageTypes))
	for _, want := range read.MessageTypes {
		got := byName[want.Name]
		require.Equal(t, want.TTLSeconds, got.TTLSeconds, want.Name)
		require.Equal(t, want.Reply, got.Reply, want.Name)
		require.Equal(t, want.Label, got.Label, want.Name)
		require.Equal(t, string(want.Persist), got.Persist, want.Name)
	}
	var names []string
	for _, p := range byName["note"].Properties {
		names = append(names, p.Name)
	}
	require.Equal(t, []string{"level", "about", "also", "at", "ok", "on", "pick"}, names)
	require.Equal(t, []string{"ticket"}, byName["note"].Properties[1].To)
	for _, p := range byName["note"].Properties {
		require.NotEmpty(t, p.Label, "an unauthored label is the name: %s", p.Name)
	}

	threads := types["threads"]
	require.Equal(t, string(realtimecfg.CardinalityMany), threads.Cardinality)

	a := newActionHarness(t, &fakePromoter{})
	for _, ct := range a.svc.Describe() {
		if ct.Name != "room" {
			continue
		}
		for _, mt := range ct.MessageTypes {
			if mt.Name == "proposal" {
				require.Equal(t, []ActionDescription{
					{Name: "accept", Label: "accept", PromoteTo: "doc"},
					{Name: "ack", Label: "ack"},
					{Name: "triage", Label: "triage"},
				}, mt.Actions)
			}
		}
	}

	l := newHarness(t, withLifecycleYAML())
	for _, ct := range l.svc.Describe() {
		if ct.Name == "room" {
			require.Equal(t, []LabeledName{
				{Name: "close", Label: "close"}, {Name: "reopen", Label: "reopen"},
				{Name: "archive", Label: "archive"}, {Name: "delete", Label: "delete"},
			}, ct.Lifecycle)
		}
	}
}

// TestDescribe_Off pins that realtime off describes no channel type.
func TestDescribe_Off(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.cfg.set(Snapshot{})
	require.NotNil(t, h.svc.Describe())
	require.Empty(t, h.svc.Describe())
}

// TestDescribe_ListAndChoices pins AC 3 and AC 6 of TKT-03FQPD: a list
// property carries list and max_items, a question type carries its
// choices, and a type without choices has none.
func TestDescribe_ListAndChoices(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(listRealtime))
	views := h.svc.Describe()
	require.Len(t, views, 1)
	byName := map[string]MessageTypeDescription{}
	for _, mt := range views[0].MessageTypes {
		byName[mt.Name] = mt
	}
	ask := byName["ask"]
	require.Equal(t, &ChoicesDescription{From: "options", Answer: "answer", Into: "body", Other: true}, ask.Choices)
	var options PropertyDescription
	for _, p := range ask.Properties {
		if p.Name == "options" {
			options = p
		}
	}
	require.True(t, options.List)
	require.Equal(t, 4, options.MaxItems)
	require.Equal(t, 80, options.Max)
	require.Equal(t, "string", options.Type)
	require.Nil(t, byName["answer"].Choices)
}

// TestDescribe_DecisionButton pins TKT-TXZZZ2 AC 5: a message type with
// decision_button carries it in Describe and in a read's message types;
// a type without it carries nil.
func TestDescribe_DecisionButton(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(optionsRealtime()), withOutputs(&fakeOutputs{}))
	yes := true
	want := &ButtonOptionsView{Confirm: &yes, ConfirmText: "Sure?", Style: "danger"}
	views := h.svc.Describe()
	require.Len(t, views, 1)
	described := map[string]MessageTypeDescription{}
	for _, mt := range views[0].MessageTypes {
		described[mt.Name] = mt
	}
	require.Equal(t, want, described["decide"].DecisionButton)
	require.Nil(t, described["text"].DecisionButton)

	read, err := h.svc.Read(as(user("mo", "plan.member")), planCh, ReadRequest{Schemas: true})
	require.NoError(t, err)
	viewed := map[string]MessageTypeView{}
	for _, mt := range read.MessageTypes {
		viewed[mt.Name] = mt
	}
	require.Equal(t, want, viewed["decide"].DecisionButton)
	require.Nil(t, viewed["text"].DecisionButton)
}

// TestDescribe_In pins TKT-CPZKPB AC 2: over the PoC with
// session.default_in: keyed, Describe and a read's message types serve the
// same In and InDerived for a derived type (in session.states order), a
// type with a written in: and a type no state keys. Both serve a copy: a
// caller that changes the returned In does not change the config.
func TestDescribe_In(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(pocRealtimeDefaultIn(t, "")))
	h.pocDriveTo(t, "new")
	type in struct {
		in      []string
		derived bool
	}
	want := map[string]in{
		"answer":   {[]string{"plan-asking", "asking"}, true},
		"approve":  {[]string{"plan-review"}, true},
		"progress": {[]string{"planning", "working"}, false},
		"thanks":   {[]string{"done"}, false},
		"text":     {nil, false},
	}
	describe := func() map[string]MessageTypeDescription {
		views := h.svc.Describe()
		require.Len(t, views, 1)
		out := map[string]MessageTypeDescription{}
		for _, mt := range views[0].MessageTypes {
			out[mt.Name] = mt
		}
		return out
	}
	read := func() map[string]MessageTypeView {
		v, err := h.svc.Read(as(pocOwner()), pocRun, ReadRequest{Schemas: true})
		require.NoError(t, err)
		out := map[string]MessageTypeView{}
		for _, mt := range v.MessageTypes {
			out[mt.Name] = mt
		}
		return out
	}
	described, viewed := describe(), read()
	for name, w := range want {
		require.Equal(t, w.in, described[name].In, "Describe %s.In", name)
		require.Equal(t, w.derived, described[name].InDerived, "Describe %s.InDerived", name)
		require.Equal(t, w.in, viewed[name].In, "Read %s.In", name)
		require.Equal(t, w.derived, viewed[name].InDerived, "Read %s.InDerived", name)
	}
	for name, mt := range described {
		require.Equal(t, mt.In, viewed[name].In, "%s: Describe and Read disagree on In", name)
		require.Equal(t, mt.InDerived, viewed[name].InDerived, "%s: Describe and Read disagree on InDerived", name)
	}

	described["answer"].In[0] = "x"
	described["progress"].In[0] = "x"
	viewed["answer"].In[0] = "x"
	viewed["progress"].In[0] = "x"
	require.Equal(t, want["answer"].in, describe()["answer"].In, "Describe aliases a derived In")
	require.Equal(t, want["progress"].in, describe()["progress"].In, "Describe aliases a written In")
	require.Equal(t, want["answer"].in, read()["answer"].In, "Read aliases a derived In")
	require.Equal(t, want["progress"].in, read()["progress"].In, "Read aliases a written In")
}

// TestDescribe_ObjectList pins TKT-GR1MXL5: an object list carries its
// fields, required first and then by name, and its choices name the label.
func TestDescribe_ObjectList(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(objectRealtime))
	views := h.svc.Describe()
	require.Len(t, views, 1)
	var ask MessageTypeDescription
	for _, mt := range views[0].MessageTypes {
		if mt.Name == "ask" {
			ask = mt
		}
	}
	require.Equal(t, &ChoicesDescription{From: "options", Label: "label", Answer: "answer", Into: "body"}, ask.Choices)
	var options PropertyDescription
	for _, p := range ask.Properties {
		if p.Name == "options" {
			options = p
		}
	}
	require.Equal(t, "object", options.Type)
	require.True(t, options.List)
	names := make([]string, len(options.Fields))
	for i, f := range options.Fields {
		names[i] = f.Name
	}
	require.Equal(t, []string{"label", "pros", "recommended"}, names)
	require.True(t, options.Fields[0].Required)
	require.True(t, options.Fields[1].List)
}
