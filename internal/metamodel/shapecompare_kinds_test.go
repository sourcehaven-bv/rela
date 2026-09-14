package metamodel

import (
	"os"
	"regexp"
	"slices"
	"testing"
)

// The TierMigration kind list is consumed by internal/datamigration to check
// that every change the classifier can DEMAND has a step that can DELIVER it.
// That check is only as good as the list, so this scans the classifier's own
// source for the kinds it raises and fails when the two disagree.
//
// Scanning cannot replace the list — the kinds are string literals spread
// across several comparison functions, some reached through helpers — but it
// can prove the list complete, which is the property consumers rely on.
func TestMigrationDeltaKinds_MatchesTheClassifier(t *testing.T) {
	src, err := os.ReadFile("shapecompare.go")
	if err != nil {
		t.Fatalf("read classifier source: %v", err)
	}
	// Both call shapes that raise a delta: r.add(tier, kind, ...) and
	// addValueDelta(tier, kind, ...).
	re := regexp.MustCompile(`(?:r\.add|addValueDelta)\(\s*TierMigration,\s*"([a-z_]+)"`)
	var found []string
	for _, m := range re.FindAllSubmatch(src, -1) {
		kind := string(m[1])
		if !slices.Contains(found, kind) {
			found = append(found, kind)
		}
	}
	if len(found) == 0 {
		t.Fatal("scanned no TierMigration kinds — the scan pattern has gone stale, " +
			"which would silently make this test vacuous")
	}
	slices.Sort(found)

	declared := MigrationDeltaKinds()
	slices.Sort(declared)
	if !slices.Equal(found, declared) {
		t.Errorf("migrationDeltaKinds is out of step with the classifier:\n"+
			"  raised in shapecompare.go: %v\n"+
			"  declared in the list:      %v\n"+
			"Add the new kind to migrationDeltaKinds, and give internal/datamigration's "+
			"resolvingSteps an entry for it (a resolving step, or an empty value with a reason).",
			found, declared)
	}
}
