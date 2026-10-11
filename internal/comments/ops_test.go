package comments_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/comments"
)

func TestOpError_Error(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  comments.OpError
		want string
	}{
		{"title only", comments.OpError{Code: "not_found", Title: "Entity not found"},
			"Entity not found [not_found]"},
		{"with detail and faces", comments.OpError{
			Code: "face_required", Title: "Address one face", Detail: "two faces",
			Faces: []string{"P-1@draft", "P-1@published"},
		}, "Address one face: two faces (faces: P-1@draft, P-1@published) [face_required]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.err.Error(); got != tc.want {
				t.Errorf("Error() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDetached(t *testing.T) {
	const content = "The retention period for audit records is five years.\n"
	body := comments.NewBody(content)
	properties := map[string]bool{"title": true}
	located, err := comments.NewTextAnchor(content, 4, 20)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name   string
		anchor comments.Anchor
		want   bool
	}{
		{"present property", comments.Anchor{Kind: comments.AnchorProperty, Ref: "title"}, false},
		{"absent property", comments.Anchor{Kind: comments.AnchorProperty, Ref: "gone"}, true},
		{"section", comments.Anchor{Kind: comments.AnchorSection, Ref: "anything"}, false},
		{"located text", comments.Anchor{Kind: comments.AnchorText, Text: located}, false},
		{"text without descriptor", comments.Anchor{Kind: comments.AnchorText}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := comments.Detached(tc.anchor, properties, body); got != tc.want {
				t.Errorf("Detached = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNewView_ShowsOnlyTheReaderFacingAnchor(t *testing.T) {
	const content = "The retention period for audit records is five years.\n"
	text, err := comments.NewTextAnchor(content, 4, 20)
	if err != nil {
		t.Fatal(err)
	}
	replacement := "storage period"
	c := comments.Comment{
		ID: "c1", Author: "alice", Body: "shorter", Resolved: true,
		CreatedAt: time.Date(2026, 10, 11, 14, 0, 5, 999, time.FixedZone("CEST", 2*60*60)),
		Anchor:    comments.Anchor{Kind: comments.AnchorText, Text: text, Replacement: &replacement},
	}

	v := comments.NewView(c)

	want := comments.AnchorView{Kind: comments.AnchorText, Quote: text.Quote, Replacement: &replacement}
	if !reflect.DeepEqual(v.Anchor, want) {
		t.Errorf("Anchor = %+v, want %+v", v.Anchor, want)
	}
	if !v.CreatedAt.Equal(c.CreatedAt.Truncate(time.Second)) || v.CreatedAt.Location() != time.UTC {
		t.Errorf("CreatedAt = %v, want %v in UTC", v.CreatedAt, c.CreatedAt.Truncate(time.Second))
	}
	if v.ID != c.ID || v.Author != c.Author || v.Body != c.Body || v.Resolved != c.Resolved {
		t.Errorf("view %+v does not carry the comment %+v", v, c)
	}
	encoded, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	for _, stored := range []string{"prefix", "suffix", "containing_sentence", "heading_context", "paragraph_index"} {
		if strings.Contains(string(encoded), stored) {
			t.Errorf("view JSON carries the stored %q: %s", stored, encoded)
		}
	}
}

func TestLiveProperties(t *testing.T) {
	got := comments.LiveProperties(map[string]int{"title": 0, "owner": 0}, map[string]any{"title": "x", "legacy": 1})
	want := map[string]bool{"title": true, "owner": true, "legacy": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LiveProperties = %v, want %v", got, want)
	}
}
