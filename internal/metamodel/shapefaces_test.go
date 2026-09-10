package metamodel

import "testing"

// Face shape is part of the migration identity (TKT-O0A8FO). Before this, a
// face rename moved stored rows without moving the hash, so the data-migration
// gate adopted the new schema silently and the rows were left orphaned.

func shapeWithFaces(t *testing.T, doc string) ShapeProjection {
	t.Helper()
	m, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return m.ShapeProjection()
}

func TestShapeProjection_FaceRenameMovesTheHash(t *testing.T) {
	before := shapeWithFaces(t, facesDoc("      en: {}\n      nl: {}\n"))
	after := shapeWithFaces(t, facesDoc("      en: {}\n      nl-BE: {}\n"))

	if before.Hash() == after.Hash() {
		t.Fatal("renaming a face moves every stored row of that face — the hash " +
			"must move so the gate demands a migration instead of adopting silently")
	}
}

func TestShapeProjection_AddingAFaceMovesTheHash(t *testing.T) {
	before := shapeWithFaces(t, facesDoc("      en: {}\n"))
	after := shapeWithFaces(t, facesDoc("      en: {}\n      nl: {}\n"))

	if before.Hash() == after.Hash() {
		t.Fatal("adding a face changes which coordinates rows may occupy; the hash must move")
	}
}

// Declaration order is not observable in stored data — faces are addressed by
// name — so reordering them must NOT demand a migration. This is the mirror of
// the tests above: it proves the hash tracks the shape rather than the file.
func TestShapeProjection_FaceOrderDoesNotMoveTheHash(t *testing.T) {
	a := shapeWithFaces(t, facesDoc("      en: {}\n      nl: {}\n"))
	b := shapeWithFaces(t, facesDoc("      nl: {}\n      en: {}\n"))

	if a.Hash() != b.Hash() {
		t.Fatal("reordering faces changes no stored row; it must not demand a migration")
	}
}

func TestShapeProjection_FacesAppearInTheProjection(t *testing.T) {
	proj := shapeWithFaces(t, facesDoc("      en: {}\n      nl: {}\n"))
	es := proj.Entities["guide"]
	if len(es.Faces) != 2 || es.Faces[0] != "en" || es.Faces[1] != "nl" {
		t.Fatalf("faces = %v, want [en nl] sorted", es.Faces)
	}
}

func facesDoc(faces string) string {
	return "version: \"1\"\nentities:\n  guide:\n    label: Guide\n    id_prefix: GUIDE\n" +
		"    faces:\n" + faces +
		"    properties:\n      title: {type: string}\n"
}

// flatDoc is the same type declaring NO faces: one state, stored at the zero
// coordinate, with no name for it.
func flatDoc() string {
	return "version: \"1\"\nentities:\n  guide:\n    label: Guide\n    id_prefix: GUIDE\n" +
		"    properties:\n      title: {type: string}\n"
}

// The classifier decides what the GATE does: additive adopts silently, drift
// adopts with a notice, needs-migration refuses. These pin the tier for each
// face delta by its consequence for rows that already exist.

func faceReport(t *testing.T, fromFaces, toFaces string) ShapeReport {
	t.Helper()
	return CompareShapes(
		shapeWithFaces(t, facesDoc(fromFaces)),
		shapeWithFaces(t, facesDoc(toFaces)),
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
	r := faceReport(t, "      en: {}\n", "      en: {}\n      nl: {}\n")
	if !hasKind(r, "face_added") {
		t.Fatalf("expected face_added, got %+v", r.Deltas)
	}
	if r.Tier() != TierAdditive {
		t.Errorf("tier = %v, want additive — nothing is stored at a new face yet", r.Tier())
	}
}

func TestCompareFaces_RemovedIsDrift(t *testing.T) {
	r := faceReport(t, "      en: {}\n      nl: {}\n", "      en: {}\n")
	if !hasKind(r, "face_removed") {
		t.Fatalf("expected face_removed, got %+v", r.Deltas)
	}
	if r.Tier() != TierDrift {
		t.Errorf("tier = %v, want drift — the rows are orphaned but intact", r.Tier())
	}
}

func TestCompareFaces_RenameIsHintedAsDeleteAddPair(t *testing.T) {
	r := faceReport(t, "      en: {}\n      nl: {}\n", "      en: {}\n      nl-BE: {}\n")
	if !hasKind(r, "possible_face_rename") {
		t.Fatalf("a one-for-one swap should be hinted as a rename, got %+v", r.Deltas)
	}
}

// The trap BUG-HC6I2T left behind. Gaining faces strands every existing row at
// the zero coordinate, which now names no declared face; losing them strands
// every named row. Neither moves a row or changes a value, so the store must
// NOT adopt either silently.
func TestCompareFaces_GainingFacesNeedsMigration(t *testing.T) {
	r := CompareShapes(
		shapeWithFaces(t, flatDoc()),
		shapeWithFaces(t, facesDoc("      en: {}\n      nl: {}\n")),
	)
	if !hasKind(r, "faces_introduced") {
		t.Fatalf("expected faces_introduced, got %+v", r.Deltas)
	}
	if r.Tier() != TierMigration {
		t.Fatalf("tier = %v, want needs-migration — existing rows name no face", r.Tier())
	}
}

func TestCompareFaces_LosingFacesNeedsMigration(t *testing.T) {
	r := CompareShapes(
		shapeWithFaces(t, facesDoc("      en: {}\n      nl: {}\n")),
		shapeWithFaces(t, flatDoc()),
	)
	if !hasKind(r, "faces_removed") {
		t.Fatalf("expected faces_removed, got %+v", r.Deltas)
	}
	if r.Tier() != TierMigration {
		t.Fatalf("tier = %v, want needs-migration — every named row is stranded", r.Tier())
	}
}

func TestCompareFaces_IdenticalShapesReportNothing(t *testing.T) {
	r := faceReport(t, "      en: {}\n      nl: {}\n", "      en: {}\n      nl: {}\n")
	for _, d := range r.Deltas {
		t.Errorf("unchanged faces must produce no delta, got %+v", d)
	}
}
