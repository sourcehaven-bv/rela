package v1

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestSectionFieldWireKeys pins the JSON keys the SPA reads off a section
// field.
//
// The struct's doc comment says the compiler keeps this in step with
// dataentry.SectionFieldData, and for the Go-side SHAPE that is true: the
// direct struct conversion requires identical field names in identical order,
// so a deleted or reordered field fails the build.
//
// It does NOT cover the json tags. Renaming `json:"span"` to `json:"columnSpan"`
// compiles clean and passes every other test in the repo, while shipping a UI
// where no field is ever laid out — the frontend reads `span` and would find
// nothing. A one-character typo in a struct tag is a silent, total feature
// failure, so the wire contract gets its own assertion.
func TestSectionFieldWireKeys(t *testing.T) {
	b, err := json.Marshal(SectionField{
		Property:     "status",
		Label:        "Status",
		Values:       []string{"open"},
		PropType:     "ticket_status",
		Inaccessible: true,
		Span:         4,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)

	for _, want := range []string{
		`"property":"status"`,
		`"label":"Status"`,
		`"values":["open"]`,
		`"propType":"ticket_status"`,
		`"inaccessible":true`,
		`"span":4`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("wire payload missing %s\ngot: %s", want, got)
		}
	}
}

// TestSectionFieldOmitsEmptySpan pins that an unauthored span stays OFF the
// wire rather than serializing as `"span":0`.
//
// The full-width default lives in one place — the CSS `var(--field-span, 12)`
// fallback — so the backend must not express it. Emitting `"span":0` would put
// a second, conflicting default on the wire and add a key to every field of
// every response for the overwhelmingly common case of no span authored.
func TestSectionFieldOmitsEmptySpan(t *testing.T) {
	b, err := json.Marshal(SectionField{Property: "title", Label: "Title"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "span") {
		t.Errorf("unauthored span must be omitted from the wire, got: %s", b)
	}
}
