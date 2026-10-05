package entity

import "testing"

// TestRelation_Identity pins that the identity carries the tail face, so two
// edges on one triple with different tails are two keys (TKT-C1XUA8), and
// that the log form matches the historical [Relation.Key] string.
func TestRelation_Identity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		rel      Relation
		want     RelationKey
		wantTail Ref
		wantText string
	}{
		{
			name:     "identity-scoped edge",
			rel:      Relation{From: "A-1", Type: "uses", To: "B-1"},
			want:     RelationKey{From: "A-1", Type: "uses", To: "B-1"},
			wantTail: Ref{ID: "A-1"},
			wantText: "A-1--uses--B-1",
		},
		{
			name:     "content-scoped edge",
			rel:      Relation{From: "A-1", FromFace: "draft", Type: "uses", To: "B-1"},
			want:     RelationKey{From: "A-1", FromFace: "draft", Type: "uses", To: "B-1"},
			wantTail: Ref{ID: "A-1", Face: "draft"},
			wantText: "A-1@draft--uses--B-1",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.rel.Identity()
			if got != tc.want {
				t.Fatalf("Identity() = %#v, want %#v", got, tc.want)
			}
			if tail := got.Tail(); tail != tc.wantTail {
				t.Errorf("Tail() = %#v, want %#v", tail, tc.wantTail)
			}
			if s := got.String(); s != tc.wantText {
				t.Errorf("String() = %q, want %q", s, tc.wantText)
			}
			if k := tc.rel.Key(); k != got.String() {
				t.Errorf("Key() = %q, want it equal to Identity().String() %q", k, got.String())
			}
		})
	}

	t.Run("tails distinguish keys", func(t *testing.T) {
		t.Parallel()
		a := Relation{From: "A-1", Type: "uses", To: "B-1"}
		b := Relation{From: "A-1", FromFace: "draft", Type: "uses", To: "B-1"}
		if a.Identity() == b.Identity() {
			t.Fatal("edges with different tails share an identity")
		}
	})
}
