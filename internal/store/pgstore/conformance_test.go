package pgstore_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// Conformance: pgstore must satisfy the full store.Store contract. The factory
// and searchFactory live in testdb_test.go (they provision an isolated schema
// per call). The whole suite is skipped when RELA_TEST_DATABASE_URL is unset.
func TestConformance(t *testing.T) {
	storetest.RunAll(t, factory, searchFactory, storetest.Capabilities{Attachments: true})
}

func FuzzRelationKeyCollision(f *testing.F) {
	storetest.FuzzRelationKeyCollision(f, fuzzFactory(f))
}

func FuzzAttachmentKeyCollision(f *testing.F) {
	storetest.FuzzAttachmentKeyCollision(f, fuzzFactory(f))
}

func FuzzRenameKeyCollapse(f *testing.F) {
	storetest.FuzzRenameKeyCollapse(f, fuzzFactory(f))
}

func FuzzConcurrentOps(f *testing.F) {
	storetest.FuzzConcurrentOps(f, fuzzFactory(f))
}

func FuzzCloneNestedValues(f *testing.F) {
	storetest.FuzzCloneNestedValues(f, fuzzFactory(f))
}

func FuzzPropertyValuesTypeZoo(f *testing.F) {
	storetest.FuzzPropertyValuesTypeZoo(f, fuzzFactory(f))
}
