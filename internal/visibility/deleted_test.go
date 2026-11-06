package visibility_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// TestGateDeleted: a deleted face is row-gated and redacted as a live read
// would be, the ungated reader passes it through, and the refusing reader
// refuses.
func TestGateDeleted(t *testing.T) {
	st := memstore.New()
	gated, err := visibility.NewPolicyReader(typeGate{allow: "ticket"}, hideProps{"secret"}, st)
	if err != nil {
		t.Fatal(err)
	}
	sr, err := visibility.NewScriptReader(gated, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	type gater interface {
		GateDeleted(context.Context, *entity.Entity) (*entity.Entity, bool)
	}
	last := func(typ string) *entity.Entity {
		return &entity.Entity{ID: "X-1", Type: typ, Properties: map[string]any{"title": "t", "secret": "s"}}
	}
	tests := []struct {
		name       string
		reader     gater
		in         *entity.Entity
		wantOK     bool
		wantSecret bool
	}{
		{"readable type is redacted", sr, last("ticket"), true, false},
		{"hidden type is refused", sr, last("secret"), false, false},
		{"nil is refused", sr, nil, false, false},
		{"unrestricted passes through", visibility.Unrestricted(st), last("secret"), true, true},
		{"unrestricted refuses nil", visibility.Unrestricted(st), nil, false, false},
		{"deny reader refuses", visibility.DenyReader{}, last("ticket"), false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := tc.reader.GateDeleted(context.Background(), tc.in)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if got.GetString("title") != "t" {
				t.Errorf("title = %q, want t", got.GetString("title"))
			}
			if _, has := got.Properties["secret"]; has != tc.wantSecret {
				t.Errorf("secret present = %v, want %v", has, tc.wantSecret)
			}
		})
	}
}
