package dataentry

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// The operator's chrome text rides the schema and the copy offers verbatim,
// and an undeclared block is OMITTED rather than sent empty (TKT-5SZG2L). A
// client renders nothing for absence and emptiness alike, so the wire carries
// only what an operator wrote.
func TestSchemaWorlds_CarriesOperatorMessages(t *testing.T) {
	meta := worldsMeta()
	site := meta.Worlds["site-nl"]
	site.Messages = metamodel.WorldMessages{Absent: "Nog niet vertaald", StandIn: "{face}"}
	site.OnAbsent = metamodel.WorldOnAbsent{Redirect: "default"}
	meta.Worlds["site-nl"] = site

	got := schemaWorlds(context.Background(), meta)

	nl := got["site-nl"]
	if nl.Messages == nil || nl.Messages.Absent != "Nog niet vertaald" || nl.Messages.StandIn != "{face}" {
		t.Errorf("site-nl messages = %+v, want the declared text verbatim", nl.Messages)
	}
	if nl.Messages != nil && nl.Messages.Projection != "" {
		t.Errorf("an undeclared entry stays empty; got %q", nl.Messages.Projection)
	}
	if nl.OnAbsent == nil || nl.OnAbsent.Redirect != "default" {
		t.Errorf("site-nl on_absent = %+v, want redirect: default", nl.OnAbsent)
	}
	if pub := got["published"]; pub.Messages != nil || pub.OnAbsent != nil {
		t.Errorf("a world with nothing declared omits both blocks; got %+v / %+v", pub.Messages, pub.OnAbsent)
	}
}

func TestSchemaFaces_CarriesOperatorMessages(t *testing.T) {
	meta := worldsMeta()
	def := meta.Entities["policy"]
	def.Faces["published"] = metamodel.FaceDef{Messages: metamodel.FaceMessages{ReadOnly: "Alleen lezen"}}

	got := schemaFaceDefs(def)
	if got["published"].Messages == nil || got["published"].Messages.ReadOnly != "Alleen lezen" {
		t.Errorf("published face messages = %+v, want read_only verbatim", got["published"].Messages)
	}
	if got["draft"].Messages != nil {
		t.Errorf("a face with nothing declared omits the block; got %+v", got["draft"].Messages)
	}
}

// TestSchemaFaces_CarriesNoticeIndependentOfReadOnly pins the second face
// message key (TKT-NLWZLX). The two are independent: `notice` is about the
// document and `read_only` about the reader, so a face may declare either
// alone or both, and each must reach the wire without the other.
func TestSchemaFaces_CarriesNoticeIndependentOfReadOnly(t *testing.T) {
	meta := worldsMeta()
	def := meta.Entities["policy"]
	// The case the key exists for: a WRITABLE draft carrying a notice and no
	// read_only. Before this key there was no config that put text here.
	def.Faces["draft"] = metamodel.FaceDef{Messages: metamodel.FaceMessages{Notice: "Nog niet vastgesteld"}}
	def.Faces["published"] = metamodel.FaceDef{Messages: metamodel.FaceMessages{
		ReadOnly: "Alleen lezen",
		Notice:   "Vastgesteld op {title}",
	}}

	got := schemaFaceDefs(def)

	draft := got["draft"].Messages
	if draft == nil || draft.Notice != "Nog niet vastgesteld" {
		t.Errorf("draft face messages = %+v, want notice verbatim", draft)
	}
	if draft != nil && draft.ReadOnly != "" {
		t.Errorf("notice alone must not synthesize a read_only; got %q", draft.ReadOnly)
	}

	pub := got["published"].Messages
	if pub == nil || pub.ReadOnly != "Alleen lezen" || pub.Notice != "Vastgesteld op {title}" {
		t.Errorf("published face messages = %+v, want both keys verbatim", pub)
	}
}

func TestCopyOnSuccessWire(t *testing.T) {
	t.Parallel()
	if got := copyOnSuccessWire(metamodel.CopyOnSuccess{}); got != nil {
		t.Errorf("nothing declared must be omitted; got %+v", got)
	}
	for _, tc := range []struct {
		name string
		in   metamodel.CopyLanding
		mode string
	}{
		{"message only lands on the face written", metamodel.CopyLanding{}, "written"},
		{"stay", metamodel.CopyLanding{Mode: "stay"}, "stay"},
		{"world", metamodel.CopyLanding{World: "actueel"}, "world"},
		{"face", metamodel.CopyLanding{Face: "concept"}, "face"},
	} {
		got := copyOnSuccessWire(metamodel.CopyOnSuccess{Message: "Vastgesteld.", Landing: tc.in})
		if got == nil || got.Message != "Vastgesteld." || got.Landing.Mode != tc.mode ||
			got.Landing.World != tc.in.World || got.Landing.Face != tc.in.Face {

			t.Errorf("%s: got %+v", tc.name, got)
		}
	}
}
