package realtimecfg

import (
	"os"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
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
	rest := doc[at:]
	for {
		start := strings.Index(rest, "```yaml\n")
		if start < 0 {
			t.Fatalf("no yaml block starting with %q after %q", prefix, heading)
		}
		rest = rest[start+len("```yaml\n"):]
		end := strings.Index(rest, "```")
		if end < 0 {
			t.Fatalf("unterminated yaml block after %q", heading)
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
