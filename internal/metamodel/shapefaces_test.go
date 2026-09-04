package metamodel

import "testing"

// Face shape is part of the migration identity (TKT-O0A8FO). Before this, a
// face rename or a bare_face repoint moved stored rows without moving the
// hash, so the data-migration gate adopted the new schema silently and the
// rows were left orphaned.

func shapeWithFaces(t *testing.T, doc string) ShapeProjection {
	t.Helper()
	m, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return m.ShapeProjection()
}

func TestShapeProjection_FaceRenameMovesTheHash(t *testing.T) {
	before := shapeWithFaces(t, sprintfDoc("en", "      en: {}\n      nl: {}\n"))
	after := shapeWithFaces(t, sprintfDoc("en", "      en: {}\n      nl-BE: {}\n"))

	if before.Hash() == after.Hash() {
		t.Fatal("renaming a face moves every stored row of that face — the hash " +
			"must move so the gate demands a migration instead of adopting silently")
	}
}

func TestShapeProjection_RepointingBareFaceMovesTheHash(t *testing.T) {
	before := shapeWithFaces(t, sprintfDoc("en", "      en: {}\n      nl: {}\n"))
	after := shapeWithFaces(t, sprintfDoc("nl", "      en: {}\n      nl: {}\n"))

	if before.Hash() == after.Hash() {
		t.Fatal("repointing bare_face relabels every existing bare row without " +
			"touching the face list — the hash must move")
	}
}

func TestShapeProjection_AddingAFaceMovesTheHash(t *testing.T) {
	before := shapeWithFaces(t, sprintfDoc("en", "      en: {}\n"))
	after := shapeWithFaces(t, sprintfDoc("en", "      en: {}\n      nl: {}\n"))

	if before.Hash() == after.Hash() {
		t.Fatal("flat → faces changes what the bare row MEANS; the hash must move")
	}
}

// Declaration order is not observable in stored data — faces are addressed by
// name — so reordering them must NOT demand a migration. This is the mirror of
// the tests above: it proves the hash tracks the shape rather than the file.
func TestShapeProjection_FaceOrderDoesNotMoveTheHash(t *testing.T) {
	a := shapeWithFaces(t, sprintfDoc("en", "      en: {}\n      nl: {}\n"))
	b := shapeWithFaces(t, sprintfDoc("en", "      nl: {}\n      en: {}\n"))

	if a.Hash() != b.Hash() {
		t.Fatal("reordering faces changes no stored row; it must not demand a migration")
	}
}

func TestShapeProjection_FacesAppearInTheProjection(t *testing.T) {
	proj := shapeWithFaces(t, sprintfDoc("en", "      en: {}\n      nl: {}\n"))
	es := proj.Entities["guide"]
	if len(es.Faces) != 2 || es.Faces[0] != "en" || es.Faces[1] != "nl" {
		t.Fatalf("faces = %v, want [en nl] sorted", es.Faces)
	}
	if es.BareFace != "en" {
		t.Fatalf("bare_face = %q, want en", es.BareFace)
	}
}

func sprintfDoc(bare, faces string) string {
	return "version: \"1\"\nentities:\n  guide:\n    label: Guide\n    id_prefix: GUIDE\n" +
		"    bare_face: " + bare + "\n    faces:\n" + faces +
		"    properties:\n      title: {type: string}\n"
}

// The classifier decides what the GATE does: additive adopts silently, drift
// adopts with a notice, needs-migration refuses. These pin the tier for each
// face delta by its consequence for rows that already exist.

// The BEFORE bare face is always `en`: each case varies what changes, not
// where it started from.
const faceReportFromBare = "en"

func faceReport(t *testing.T, fromFaces, toBare, toFaces string) ShapeReport {
	t.Helper()
	return CompareShapes(
		shapeWithFaces(t, sprintfDoc(faceReportFromBare, fromFaces)),
		shapeWithFaces(t, sprintfDoc(toBare, toFaces)),
	)
}

func hasKind(r ShapeReport, kind string) bool {
	for _, d := range r.Deltas {
		if d.Kind == kind {
			return true
		}
	}
	return false
}

func TestCompareFaces_AddedIsAdditive(t *testing.T) {
	r := faceReport(t, "      en: {}\n", "en", "      en: {}\n      nl: {}\n")
	if !hasKind(r, "face_added") {
		t.Fatalf("expected face_added, got %+v", r.Deltas)
	}
	if r.Tier() != TierAdditive {
		t.Errorf("tier = %v, want additive — nothing is stored at a new face yet", r.Tier())
	}
}

func TestCompareFaces_RemovedIsDrift(t *testing.T) {
	r := faceReport(t, "      en: {}\n      nl: {}\n", "en", "      en: {}\n")
	if !hasKind(r, "face_removed") {
		t.Fatalf("expected face_removed, got %+v", r.Deltas)
	}
	if r.Tier() != TierDrift {
		t.Errorf("tier = %v, want drift — the rows are orphaned but intact", r.Tier())
	}
}

func TestCompareFaces_RenameIsHintedAsDeleteAddPair(t *testing.T) {
	r := faceReport(t, "      en: {}\n      nl: {}\n", "en", "      en: {}\n      nl-BE: {}\n")
	if !hasKind(r, "possible_face_rename") {
		t.Fatalf("a one-for-one swap should be hinted as a rename, got %+v", r.Deltas)
	}
}

// The trap: repointing bare_face relabels every existing bare row in place.
// Nothing moves and no value changes, so the store must NOT adopt it silently.
func TestCompareFaces_BareFaceRepointNeedsMigration(t *testing.T) {
	r := faceReport(t, "      en: {}\n      nl: {}\n", "nl", "      en: {}\n      nl: {}\n")
	if !hasKind(r, "bare_face_changed") {
		t.Fatalf("expected bare_face_changed, got %+v", r.Deltas)
	}
	if r.Tier() != TierMigration {
		t.Fatalf("tier = %v, want needs-migration — this silently relabels every bare row", r.Tier())
	}
}

func TestCompareFaces_IdenticalShapesReportNothing(t *testing.T) {
	r := faceReport(t, "      en: {}\n      nl: {}\n", "en", "      en: {}\n      nl: {}\n")
	for _, d := range r.Deltas {
		t.Errorf("unchanged faces must produce no delta, got %+v", d)
	}
}
