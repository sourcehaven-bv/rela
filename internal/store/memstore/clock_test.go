package memstore_test

import (
	"context"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// TestWithClock pins that writes stamp UpdatedAt from the injected clock,
// and that a nil clock keeps the wall clock.
func TestWithClock(t *testing.T) {
	pinned := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		clock  func() time.Time
		pinned bool
	}{
		{"pinned", func() time.Time { return pinned }, true},
		{"nil keeps the wall clock", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := memstore.New(memstore.WithClock(tc.clock))
			ctx := context.Background()
			e := &entity.Entity{ID: "X-1", Type: "x", Properties: map[string]any{"a": "1"}}
			if err := s.CreateEntity(ctx, e); err != nil {
				t.Fatal(err)
			}
			got, err := s.GetEntity(ctx, e.Ref())
			if err != nil {
				t.Fatal(err)
			}
			if got.UpdatedAt.Equal(pinned) != tc.pinned {
				t.Errorf("UpdatedAt = %v, pinned = %v", got.UpdatedAt, tc.pinned)
			}
		})
	}
}
