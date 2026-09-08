package entity

import (
	"strings"
	"testing"
)

// TestParseFace pins the face grammar (design doc §3.2, amended
// 2026-08-20): lowercase alphanumeric runs joined by SINGLE hyphens. The
// no-consecutive-hyphens rule is load-bearing for the storage format —
// the face serializes into the FROM slot of relation keys
// ("FROM--TYPE--TO"), mirroring ValidateID's identical rule for base ids.
func TestParseFace(t *testing.T) {
	t.Parallel()

	valid := []string{"draft", "published", "review-2", "a", "x9", "nl-be-draft"}
	for _, s := range valid {
		if p, err := ParseFace(s); err != nil || string(p) != s {
			t.Errorf("ParseFace(%q) = (%q, %v), want (%q, nil)", s, p, err, s)
		}
	}

	invalid := map[string]string{
		"":            "empty",
		"Draft":       "uppercase",
		"9draft":      "leading digit",
		"-draft":      "leading hyphen",
		"draft-":      "trailing hyphen",
		"a--b":        "consecutive hyphens (relation-key separator)",
		"nl+draft":    "multi-axis reserved",
		"dr aft":      "space",
		"draft@x":     "separator char",
		"dráft":       "non-ASCII",
		"draft/x":     "path separator",
		"draft\x00":   "NUL",
		"a-b--c-d":    "interior consecutive hyphens",
		"PAGE-1@d":    "not a bare face",
		"draft\ttab":  "control char",
		"draft\nline": "newline",
	}
	for s, why := range invalid {
		if _, err := ParseFace(s); err == nil {
			t.Errorf("ParseFace(%q) succeeded, want error (%s)", s, why)
		}
	}

	// The multi-axis rejection is a distinct, forward-looking error so a
	// newer project's data fails loudly on an older binary.
	if _, err := ParseFace("nl+draft"); err == nil || !strings.Contains(err.Error(), "multi-axis") {
		t.Errorf("ParseFace(nl+draft) error = %v, want a multi-axis message", err)
	}
}

func TestParseStateRef(t *testing.T) {
	t.Parallel()

	t.Run("bare id is the default state", func(t *testing.T) {
		t.Parallel()
		id, p, err := ParseStateRef("PAGE-1")
		if err != nil || id != "PAGE-1" || !p.IsDefault() {
			t.Errorf("got (%q, %q, %v), want (PAGE-1, default, nil)", id, p, err)
		}
	})

	t.Run("qualified form", func(t *testing.T) {
		t.Parallel()
		id, p, err := ParseStateRef("PAGE-1@draft")
		if err != nil || id != "PAGE-1" || p != Face("draft") {
			t.Errorf("got (%q, %q, %v), want (PAGE-1, draft, nil)", id, p, err)
		}
	})

	invalid := map[string]string{
		"PAGE-1@":       "empty face",
		"@draft":        "empty id",
		"PAGE-1@a@b":    "two separators",
		"PAGE-1@Draft":  "bad face grammar",
		"pa/ge@draft":   "bad base id",
		"PAGE-1@nl+da":  "multi-axis reserved",
		"":              "empty",
		"PAGE--1@draft": "base id with consecutive hyphens",
		"PAGE-1@a--b":   "face with consecutive hyphens",
	}
	for s, why := range invalid {
		if _, _, err := ParseStateRef(s); err == nil {
			t.Errorf("ParseStateRef(%q) succeeded, want error (%s)", s, why)
		}
	}
}

// TestFormatStateRef pins that Format never emits an empty face: the
// default state's serialization IS the bare id.
func TestFormatStateRef(t *testing.T) {
	t.Parallel()

	if got := FormatStateRef("PAGE-1", ""); got != "PAGE-1" {
		t.Errorf("FormatStateRef(PAGE-1, default) = %q, want PAGE-1", got)
	}
	if got := FormatStateRef("PAGE-1", Face("draft")); got != "PAGE-1@draft" {
		t.Errorf("FormatStateRef(PAGE-1, draft) = %q, want PAGE-1@draft", got)
	}

	// Round trip: Format ∘ Parse = identity on both forms.
	for _, s := range []string{"PAGE-1", "PAGE-1@draft", "X_2@rev-3"} {
		id, p, err := ParseStateRef(s)
		if err != nil {
			t.Fatalf("ParseStateRef(%q): %v", s, err)
		}
		if got := FormatStateRef(id, p); got != s {
			t.Errorf("round trip %q -> %q", s, got)
		}
	}
}
