package fsimport

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

func TestNormalizeProps(t *testing.T) {
	at := time.Date(2026, 3, 4, 10, 0, 0, 0, time.FixedZone("x", 3600))
	day := time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC) // how YAML decodes 2026-03-04
	defs := map[string]metamodel.PropertyDef{
		"due":   {Type: metamodel.PropertyTypeDate},
		"dates": {Type: metamodel.PropertyTypeDate},
		"seen":  {Type: metamodel.PropertyTypeDatetime},
	}
	cases := []struct {
		name    string
		in      map[string]any
		want    map[string]any
		changed int
		err     string
	}{
		{name: "empty", in: nil, want: nil},
		{name: "untouched", in: map[string]any{"title": "x", "n": 2.0}, want: map[string]any{"title": "x", "n": 2.0}},
		{
			name:    "date and datetime",
			in:      map[string]any{"due": day, "seen": at},
			want:    map[string]any{"due": "2026-03-04", "seen": "2026-03-04T10:00:00+01:00"},
			changed: 2,
		},
		{
			name:    "bare date on a datetime property stays a timestamp",
			in:      map[string]any{"seen": day},
			want:    map[string]any{"seen": "2026-03-04T00:00:00Z"},
			changed: 1,
		},
		{
			name:    "bare date on a string property keeps its text",
			in:      map[string]any{"version": day, "stamp": at},
			want:    map[string]any{"version": "2026-03-04", "stamp": "2026-03-04T10:00:00+01:00"},
			changed: 2,
		},
		{
			name:    "list and map",
			in:      map[string]any{"dates": []any{day, "b"}, "meta": map[string]any{"at": at}},
			want:    map[string]any{"dates": []any{"2026-03-04", "b"}, "meta": map[string]any{"at": "2026-03-04T10:00:00+01:00"}},
			changed: 2,
		},
		{name: "time of day on a date property", in: map[string]any{"due": at}, err: "time of day"},
		{name: "time of day inside a date list", in: map[string]any{"dates": []any{day, at}}, err: "time of day"},
		{name: "infinity", in: map[string]any{"n": math.Inf(1)}, err: `property "n"`},
		{name: "non-string keys", in: map[string]any{"m": map[any]any{1: "a"}}, err: `property "m"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, changed, err := normalizeProps(tc.in, defs)
			if tc.err != "" {
				require.ErrorContains(t, err, tc.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.changed, changed)
		})
	}
}

func TestDescribeWriteErr(t *testing.T) {
	require.ErrorContains(t, describeWriteErr(store.ErrConflict), "regardless of case")
	require.EqualError(t, describeWriteErr(errEncrypted), "is encrypted")
	plain := errors.New("plain")
	assert.Equal(t, plain, describeWriteErr(plain))
}

func TestSplitRelationStem(t *testing.T) {
	for stem, want := range map[string][3]string{
		"A--links--B":       {"A", "links", "B"},
		"A@draft--links--B": {"A@draft", "links", "B"},
		"A--has--many--B":   {"A", "has--many", "B"},
		"A--links":          {},
		"--links--B":        {},
		"A--links--":        {},
	} {
		from, typ, to := splitRelationStem(stem)
		assert.Equal(t, want, [3]string{from, typ, to}, stem)
	}
}
