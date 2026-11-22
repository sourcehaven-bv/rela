package records

import (
	"errors"
	"fmt"
	"testing"
)

func TestUserText(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		want   string
		caller bool
	}{
		{"not found", fmt.Errorf("%w: record type %q", ErrNotFound, "x"), `not found: record type "x"`, true},
		{"forbidden", fmt.Errorf("%w: update MET-1", ErrForbidden), "you may not change the records of this entity", true},
		{"stale", ErrStale, "the record changed since you read it; read it again and retry", true},
		{"group", ErrGroupExists, "this transaction id is already used", true},
		{"invalid", fmt.Errorf("%w: owner is required", ErrInvalid), "owner is required", true},
		{"backend", errors.New("pq: connection refused"), "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := UserText(tc.err)
			if got != tc.want || ok != tc.caller {
				t.Errorf("UserText = %q, %v; want %q, %v", got, ok, tc.want, tc.caller)
			}
		})
	}
}
