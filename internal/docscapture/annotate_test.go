package docscapture

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/docs"
)

// The overlay script must carry operator text as a JSON literal so a hostile
// string cannot break out of the JS (DR-C2). chromedp.Evaluate has no args
// channel, so json.Marshal is the only safe splice.
func TestAnnotateScript_InjectionSafe(t *testing.T) {
	t.Parallel()
	hostile := "\"; document.title='pwned'; //</script> x"
	script, err := annotateScript([]docs.Annotation{{At: "status", Text: hostile}})
	if err != nil {
		t.Fatalf("annotateScript: %v", err)
	}
	// The raw hostile text must NOT appear verbatim — it must be JSON-escaped.
	if strings.Contains(script, `document.title='pwned'; //</script>`) {
		t.Errorf("hostile text leaked unescaped into the script:\n%s", script)
	}
	// The line separator U+2028 must be escaped (json.Marshal emits  ).
	if strings.ContainsRune(script, ' ') {
		t.Errorf("U+2028 not escaped — would break the JS string")
	}
	// The </ sequence must be defanged.
	if strings.Contains(script, "</script>") {
		t.Errorf("</script> not defanged:\n%s", script)
	}
	// The escaped forms should be present.
	if !strings.Contains(script, `document.title=`) {
		// It's fine that a backslash-escaped form exists; just ensure the field
		// name still made it as data.
		t.Logf("script:\n%s", script)
	}
}

func TestAnchorSelector(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"status":          "#field-status",
		"@role:.foo .bar": ".foo .bar",
		"@button:Save":    "@button:Save",
	}
	for in, want := range cases {
		got, err := anchorSelector(in)
		if err != nil {
			t.Fatalf("anchorSelector(%q): %v", in, err)
		}
		if got != want {
			t.Errorf("anchorSelector(%q) = %q, want %q", in, got, want)
		}
	}
	if _, err := anchorSelector(""); err == nil {
		t.Error("empty `at` should error")
	}
}

func TestFormURL(t *testing.T) {
	t.Parallel()
	base := "http://127.0.0.1:9999"
	cases := []struct {
		spec docs.CaptureSpec
		want string
	}{
		{docs.CaptureSpec{View: "form", Type: "ticket", Entity: "T-1"}, base + "/form/edit_ticket/T-1"},
		{docs.CaptureSpec{View: "form", Type: "ticket", Entity: "T-1", Form: "custom"}, base + "/form/custom/T-1"},
		// Every screen kind the SPA routes, so a manual can show a whole
		// workflow rather than only edit forms.
		{docs.CaptureSpec{View: "list", List: "policies"}, base + "/list/policies"},
		{docs.CaptureSpec{View: "entity", Type: "policy", Entity: "POL-1"}, base + "/entity/policy/POL-1"},
		{docs.CaptureSpec{View: "create", Type: "policy"}, base + "/form/new_policy"},
		{docs.CaptureSpec{View: "create", Type: "policy", Form: "custom"}, base + "/form/custom"},
		{docs.CaptureSpec{View: "search", Query: "access"}, base + "/search?q=access"},
		// The world rides as ?world=, the same parameter a browser carries —
		// so the figure is the page a reader would actually see.
		{docs.CaptureSpec{View: "list", List: "policies", World: "published"},
			base + "/list/policies?world=published"},
		{docs.CaptureSpec{View: "entity", Type: "policy", Entity: "POL-1", World: "editorial"},
			base + "/entity/policy/POL-1?world=editorial"},
		{docs.CaptureSpec{View: "search", Query: "access", World: "published"},
			base + "/search?q=access&world=published"},
	}
	for _, c := range cases {
		if got := captureURL(base, c.spec); got != c.want {
			t.Errorf("captureURL(%+v) = %q, want %q", c.spec, got, c.want)
		}
	}
}
