package docs

import (
	"context"
	"errors"
	"fmt"
)

// PageInspector reads back the rendered text of one region of a page.
//
// It is a SEPARATE, optional capability from [Capturer] rather than a method
// on it, following the store's `HistoryReader`/`Formatter` pattern: a capturer
// that cannot inspect is still a perfectly good screenshot capturer, and
// widening the required interface would break every implementation for a
// feature most of them do not need. page{} type-asserts for it and fails loud
// when it is absent — never silently skips, because a skipped assertion is
// indistinguishable from a passing one.
//
// Like [Capturer] it is consumer-side (defined here, implemented in
// internal/docscapture, injected by the CLI), so the core docs package never
// imports a browser.
type PageInspector interface {
	// Inspect navigates to the page described by spec — the same standing-up
	// and readiness gate a capture uses — and returns the trimmed,
	// whitespace-collapsed text of every element matching selector, in document
	// order. An empty result means the selector matched nothing; that is a
	// normal return, and the CALLER decides it is a failure (see
	// checkRegionText), because only the caller knows whether emptiness was the
	// claim.
	//
	// selector is supplied from the docs package's own closed region table
	// ([regions]) and never from a manual.
	Inspect(ctx context.Context, spec CaptureSpec, selector string) ([]string, error)
}

// requireViewArgs enforces the per-view argument rules shared by screenshot{}
// and page{}.
//
// Both verbs address a screen the same way, so the rules live in one place: two
// copies would drift, and the failure when they do is a verb that silently
// renders the wrong screen — a capture of the app's empty state under a caption
// about a populated one. Asking for the wrong argument must be a clear refusal.
func requireViewArgs(spec CaptureSpec) error {
	switch spec.View {
	case "list", "kanban", "calendar":
		if spec.List == "" {
			return fmt.Errorf("view=%q: `list` is required (the view id from data-entry.yaml)", spec.View)
		}
	case "dashboard", "analyze":
		// Whole-app screens: nothing to name.
	case "search":
		if spec.Query == "" {
			return errors.New("view=\"search\": `q` is required — a search screen with no " +
				"query renders the idle state, which shows nothing for a reason unrelated " +
				"to what you are illustrating")
		}
	case "create":
		if spec.Type == "" && spec.Form == "" {
			return errors.New("view=\"create\": `type` or `form` is required")
		}
	case "history":
		if spec.Type == "" || spec.Entity == "" {
			return errors.New("view=\"history\": `type` and `entity` are required")
		}
	default: // "form", "entity"
		if spec.Type == "" {
			return fmt.Errorf("view=%q: `type` is required", spec.View)
		}
		if spec.Entity == "" {
			return fmt.Errorf("view=%q: `entity` is required (the id of a seeded entity to render)", spec.View)
		}
	}
	return nil
}
