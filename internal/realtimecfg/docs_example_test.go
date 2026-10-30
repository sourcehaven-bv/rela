package realtimecfg

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

const realtimeGuide = "../../docs-project/entities/guides/GUIDE-realtime.md"

// yamlBlock returns the first fenced yaml block after heading in doc whose
// body starts with prefix.
func yamlBlock(t *testing.T, doc, heading, prefix string) string {
	t.Helper()
	at := strings.Index(doc, "\n"+heading+"\n")
	if at < 0 {
		t.Fatalf("heading %q not found in the guide", heading)
	}
	return yamlBlockIn(t, doc[at:], heading, prefix)
}

// yamlBlockIn returns the first fenced yaml block in rest whose body starts
// with prefix. after names where rest starts, for the failure message.
func yamlBlockIn(t *testing.T, rest, after, prefix string) string {
	t.Helper()
	for {
		start := strings.Index(rest, "```yaml\n")
		if start < 0 {
			t.Fatalf("no yaml block starting with %q after %q", prefix, after)
		}
		rest = rest[start+len("```yaml\n"):]
		end := strings.Index(rest, "```")
		if end < 0 {
			t.Fatalf("unterminated yaml block after %q", after)
		}
		if body := rest[:end]; strings.HasPrefix(body, prefix) {
			return body
		}
		rest = rest[end:]
	}
}

// TestDocsExample_Realtime pins that the session example of the realtime
// guide is the breach-response block of testdata/valid.yaml, and that it,
// with its run_as, timer and handoff outputs, loads with no problems and no
// warnings: against the testdata policy, and against the guide's own
// acl.yaml example for the run_as identity.
func TestDocsExample_Realtime(t *testing.T) {
	data, err := os.ReadFile(realtimeGuide)
	if err != nil {
		t.Fatal(err)
	}
	doc := string(data)
	example := "version: 1\n" + yamlBlock(t, doc, "## Sessions", "channel_types:")
	if !strings.Contains(example, "run_as: system:breach-bot") {
		t.Fatal("the session example lost its run_as")
	}
	valid, err := os.ReadFile("testdata/valid.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if body := strings.TrimPrefix(example, "version: 1\nchannel_types:\n"); !strings.Contains(string(valid), body) {
		t.Error("the guide's session example is not the breach-response block of testdata/valid.yaml")
	}

	t.Run("testdata policy", func(t *testing.T) {
		c, err := Parse([]byte(example), testMeta(t), testPolicy(t))
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if w := c.Warnings(); len(w) != 0 {
			t.Errorf("warnings = %v", w)
		}
	})

	t.Run("guide policy", func(t *testing.T) {
		guideACL := strings.TrimPrefix(yamlBlock(t, doc, "### Timers and triggers", "# acl.yaml"), "# acl.yaml\n")
		// The example's dpo role needs some role to grant privacy.officer,
		// and its answers need some role that may update datalek.
		guideACL = strings.Replace(guideACL, "roles:\n",
			"roles:\n  officer:\n    read: [\"*\"]\n    permissions: [privacy.officer]\n"+
				"  handler: { read: [datalek], update: [datalek] }\n", 1)
		p, err := acl.ParsePolicy([]byte(guideACL), "acl.yaml")
		if err != nil {
			t.Fatalf("parse the guide's acl.yaml: %v", err)
		}
		c, err := Parse([]byte(example), testMeta(t), p)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if w := c.Warnings(); len(w) != 0 {
			t.Errorf("warnings = %v", w)
		}
		lessACL := strings.Replace(guideACL, "    update: [datalek]\n", "", 1)
		p, err = acl.ParsePolicy([]byte(lessACL), "acl.yaml")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Parse([]byte(example), testMeta(t), p); err == nil ||
			!strings.Contains(err.Error(), "system:breach-bot cannot update datalek") {

			t.Errorf("Parse without the update grant = %v; want the run_as grant problem", err)
		}
	})
}

// TestDocsExample_Branching pins TKT-FPXTQJ AC 12: the guide's example of
// a branching message type loads with no problems and no warnings, and its
// decide type has two entries in file order.
func TestDocsExample_Branching(t *testing.T) {
	data, err := os.ReadFile(realtimeGuide)
	if err != nil {
		t.Fatal(err)
	}
	example := "version: 1\n" + yamlBlock(t, string(data), "## Sessions", "channel_types:\n  plan-review:")
	c, err := Parse([]byte(example), testMeta(t), testPolicy(t))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if w := c.Warnings(); len(w) != 0 {
		t.Errorf("warnings = %v", w)
	}
	ct, _ := c.ChannelType("plan-review")
	var tos []string
	for _, tr := range ct.Session().Branches("review", "decide") {
		tos = append(tos, tr.To)
	}
	if strings.Join(tos, ",") != "approved,drafting" {
		t.Errorf("decide entries lead to %v; want approved, drafting", tos)
	}
	var labels []string
	for _, tr := range ct.Session().Branches("review", "decide") {
		b, ok := tr.Button()
		if !ok {
			t.Fatalf("entry %d has no button; the guide gives both entries one", tr.Index())
		}
		labels = append(labels, b.Label+"="+fmt.Sprint(b.Props()["decision"]))
	}
	if strings.Join(labels, ",") != "Approve=approve,Revise=revise" {
		t.Errorf("buttons = %v; want Approve and Revise with their decisions", labels)
	}
	// TKT-TXZZZ2 AC 10: Approve is the primary decision; Revise keeps the
	// derived look.
	var styles []string
	for _, tr := range ct.Session().Branches("review", "decide") {
		b, _ := tr.Button()
		styles = append(styles, string(b.Options.Style))
	}
	if strings.Join(styles, ",") != "primary," {
		t.Errorf("button styles = %q; want Approve primary and Revise empty", styles)
	}
}

// TestDocsExample_DecisionButton pins TKT-TXZZZ2 AC 10: the guide's Allow
// and Abandon example loads with no problems and no warnings, and its
// decision_button keys compile as the guide describes.
func TestDocsExample_DecisionButton(t *testing.T) {
	data, err := os.ReadFile(realtimeGuide)
	if err != nil {
		t.Fatal(err)
	}
	example := "version: 1\n" + yamlBlock(t, string(data), "## Sessions", "channel_types:\n  tool-run:")
	c, err := Parse([]byte(example), testMeta(t), testPolicy(t))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if w := c.Warnings(); len(w) != 0 {
		t.Errorf("warnings = %v", w)
	}
	ct, _ := c.ChannelType("tool-run")
	allow, abandon := ct.MessageTypes["allow"], ct.MessageTypes["abandon"]
	if allow == nil || abandon == nil {
		t.Fatal("the example lost its allow or abandon type")
	}
	want := ButtonOptions{Confirm: ConfirmAsk, ConfirmText: "This runs the tool on your machine."}
	if allow.DecisionButton == nil || *allow.DecisionButton != want {
		t.Errorf("allow decision_button = %+v; want %+v", allow.DecisionButton, want)
	}
	if abandon.DecisionButton == nil || *abandon.DecisionButton != (ButtonOptions{Style: ButtonStyleDanger}) {
		t.Errorf("abandon decision_button = %+v; want style danger", abandon.DecisionButton)
	}
}

// TestDocsExample_Choices pins AC 14 of TKT-03FQPD: the Choices example of
// the realtime guide loads with no warnings, and its question type holds
// the choices the guide describes.
func TestDocsExample_Choices(t *testing.T) {
	data, err := os.ReadFile(realtimeGuide)
	if err != nil {
		t.Fatal(err)
	}
	example := "version: 1\n" + yamlBlock(t, string(data), "## Choices", "channel_types:\n  agent-run:")
	c, err := Parse([]byte(example), testMeta(t), testPolicy(t))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if w := c.Warnings(); len(w) != 0 {
		t.Errorf("warnings = %v", w)
	}
	ct, _ := c.ChannelType("agent-run")
	want := Choices{From: "options", Answer: "answer", Into: "body", Other: true}
	if got := ct.MessageTypes["ask"].Choices; got == nil || *got != want {
		t.Errorf("ask.Choices = %+v; want %+v", got, want)
	}
	if p := ct.MessageTypes["ask"].Properties["options"]; !p.List || p.Max != 80 || p.MaxItems != 4 {
		t.Errorf("options = %+v; want a list of at most 4 items of 80 characters", p)
	}
}

// TestDocsExample_Busy pins AC 12 of TKT-21G6JS: the agent-run example under
// "Busy indicator" in the realtime guide loads with no warnings, only
// planning counts as working, and the initial state is not working.
func TestDocsExample_Busy(t *testing.T) {
	data, err := os.ReadFile(realtimeGuide)
	if err != nil {
		t.Fatal(err)
	}
	example := yamlBlock(t, string(data), "## Busy indicator", "version: 1\nchannel_types:\n  run:")
	meta, err := metamodel.Parse([]byte(`
entities:
  ticket:
    label: Ticket
    id_prefix: TKT
    properties:
      title:  { type: string }
      owner:  { type: string }
      runner: { type: string }
`))
	if err != nil {
		t.Fatal(err)
	}
	c, err := Parse([]byte(example), meta, testPolicy(t))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if w := c.Warnings(); len(w) != 0 {
		t.Errorf("warnings = %v", w)
	}
	ct, _ := c.ChannelType("run")
	s := ct.Session()
	if got := strings.Join(s.WorkingStates(), ","); got != "planning" {
		t.Errorf("working states = %q; want planning", got)
	}
	if st, _ := s.State(s.Initial()); st.Status == StatusWorking {
		t.Errorf("initial state %q reads working; the guide says it waits", st.Name)
	}
}

// TestDocsExample_Unread pins TKT-O6RC8Z AC 11: the example under "Messages
// that do not count as unread" in the realtime guide loads with no
// warnings, and only ask counts as unread.
func TestDocsExample_Unread(t *testing.T) {
	data, err := os.ReadFile(realtimeGuide)
	if err != nil {
		t.Fatal(err)
	}
	example := yamlBlock(t, string(data), "### Messages that do not count as unread", "version: 1\nchannel_types:\n  run:")
	meta, err := metamodel.Parse([]byte("entities:\n  ticket:\n    label: Ticket\n    id_prefix: TKT\n" +
		"    properties:\n      title: { type: string }\n"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := Parse([]byte(example), meta, testPolicy(t))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if w := c.Warnings(); len(w) != 0 {
		t.Errorf("warnings = %v", w)
	}
	ct, _ := c.ChannelType("run")
	for name, want := range map[string]bool{"ask": false, "claim": true, "resume": true, "progress": true} {
		if got := ct.MessageTypes[name].NoUnread; got != want {
			t.Errorf("%s.NoUnread = %v; want %v", name, got, want)
		}
	}
}
