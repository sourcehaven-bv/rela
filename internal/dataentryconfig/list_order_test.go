package dataentryconfig

import (
	"strings"
	"testing"
)

// TestValidateConfig_ListOrder pins the load check of `order:` (TKT-AVNEX5T).
func TestValidateConfig_ListOrder(t *testing.T) {
	cols := []ListColumn{{Property: "title"}}
	tests := []struct {
		name string
		list List
		want string
	}{
		{"manual", List{Order: "manual"}, ""},
		{"manual grouped on a property", List{Order: "manual", GroupBy: &ListGroupBy{Property: "status"}}, ""},
		{"unknown order", List{Order: "random"}, `invalid order "random"`},
		{"manual with buckets", List{Order: "manual",
			GroupBy: &ListGroupBy{Property: "created", Buckets: &ListBuckets{By: "month"}}},
			"order: manual needs group_by on a property without buckets"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := tc.list
			l.EntityType, l.Columns = "ticket", cols
			cfg := &Config{Lists: map[string]List{"all": l}}
			err := ValidateConfig([]byte(`version: "1.0"`), cfg, testMetamodel())
			if tc.want == "" {
				if err != nil {
					t.Fatalf("want load, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
}
