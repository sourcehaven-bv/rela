package dataentry

import (
	"slices"
	"testing"
)

// A relation write must reach the views of both its end types (BUG-022MB1).
func TestRelationEndTypes(t *testing.T) {
	app := newRelationsTestApp(t)
	if got := app.relationEndTypes("belongs-to"); !slices.Equal(got, []string{"ticket", "category"}) {
		t.Errorf("belongs-to end types = %v, want [ticket category]", got)
	}
	if got := app.relationEndTypes("nonsuch"); got != nil {
		t.Errorf("unknown relation end types = %v, want nil", got)
	}
}
