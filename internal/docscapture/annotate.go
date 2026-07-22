package docscapture

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/chromedp/chromedp"

	"github.com/Sourcehaven-BV/rela/internal/docs"
)

// annotateAction injects an overlay drawing each annotation (arrow + text, or a
// box) anchored to its target element, then verifies every anchor resolved. It
// fails loud if any anchor matched no element (DR-S4 for annotations).
func annotateAction(arrows []docs.Annotation) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		script, err := annotateScript(arrows)
		if err != nil {
			return err
		}
		var missing []string
		if err := chromedp.Evaluate(script, &missing).Do(ctx); err != nil {
			return err
		}
		if len(missing) > 0 {
			return fmt.Errorf("annotation anchors not found on the page: %s", strings.Join(missing, ", "))
		}
		return nil
	})
}

// annotateSpec is the per-annotation data handed to the injected JS. Selectors
// and text are carried as DATA (a JSON literal), never spliced into JS source,
// so operator-authored text cannot break out of the script (DR-C2).
type annotateSpec struct {
	Selector string `json:"selector"`
	Text     string `json:"text"`
	Side     string `json:"side"`
	Box      bool   `json:"box"`
}

// annotateScript builds the overlay script. The annotation list is JSON-encoded
// and embedded as a literal; json.Marshal of a Go string produces a valid,
// fully-escaped JS string literal (handles ", </script>, backslashes, and the
// U+2028/U+2029 line separators that would otherwise break a JS string).
func annotateScript(arrows []docs.Annotation) (string, error) {
	specs := make([]annotateSpec, 0, len(arrows))
	for _, a := range arrows {
		sel, err := anchorSelector(a.At)
		if err != nil {
			return "", err
		}
		specs = append(specs, annotateSpec{
			Selector: sel,
			Text:     a.Text,
			Side:     a.Side,
			Box:      a.Box,
		})
	}
	data, err := json.Marshal(specs)
	if err != nil {
		return "", err
	}
	// Guard against the one sequence that closes a <script> context in HTML,
	// belt-and-braces even though this runs via Runtime.evaluate not a <script>.
	safe := strings.ReplaceAll(string(data), "</", `<\/`)
	return strings.ReplaceAll(overlayJS, "__SPECS__", safe), nil
}

//go:embed overlay.js
var overlayJS string

// anchorSelector maps a screenshot annotation target to a CSS selector:
//   - a bare property name → #field-<prop> (the widget id FieldRenderer stamps)
//   - "@button:<label>"    → a button by accessible label (best-effort)
//   - "@role:<sel>"        → a raw ARIA/CSS selector passthrough
func anchorSelector(at string) (string, error) {
	switch {
	case at == "":
		return "", errors.New("annotation `at` is required")
	case strings.HasPrefix(at, "@role:"):
		return strings.TrimPrefix(at, "@role:"), nil
	case strings.HasPrefix(at, "@button:"):
		// Buttons are matched in-JS by text; encode as a marker the overlay reads.
		return "@button:" + strings.TrimPrefix(at, "@button:"), nil
	default:
		return "#field-" + at, nil
	}
}

// fieldOf returns the property name an annotation targets a field of, or "" for
// non-field (@button/@role) targets — used to pick the renderability anchor.
func fieldOf(at string) string {
	if at == "" || strings.HasPrefix(at, "@") {
		return ""
	}
	return at
}
