package fsstore_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TestFSStore_Encrypted_FormatEntity_NoFalseDiff is a regression test
// for the formatter bug: FormatEntity compares the *raw* on-disk bytes
// (which, on an encrypted repo, are the age-sealed ciphertext) against
// the plaintext formatted output. They never match, so FormatEntity
// always reports diff=true and rewrites every file on every run.
//
// The fix is part of TKT-8S1SA: once fsstore reads through the
// plaintext-returning StoreFS decorator, FormatEntity will compare
// plaintext-to-plaintext and the bug vanishes.
//
// This test is committed ahead of the fix (as Skip) so the bug cannot
// be forgotten if the refactor scope is reduced mid-flight.
func TestFSStore_Encrypted_FormatEntity_NoFalseDiff(t *testing.T) {
	s, _ := buildEncryptedStore(t)
	ctx := context.Background()

	// Create an entity whose formatted output equals its persisted
	// plaintext exactly (no reformatting changes expected).
	e := entity.New("TKT-FMT-1", "ticket")
	e.Properties["title"] = "already canonical"
	e.Content = "body text\n"
	if err := s.CreateEntity(ctx, e); err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}

	// First FormatEntity call: the just-written file should already be
	// in canonical form, so diff must be false.
	diff, err := s.FormatEntity(ctx, "TKT-FMT-1", true /*dryRun*/)
	if err != nil {
		t.Fatalf("FormatEntity: %v", err)
	}
	if diff {
		t.Errorf("FormatEntity reported diff=true on an unchanged encrypted file " +
			"(formatter compared sealed ciphertext to plaintext)")
	}
}

// TestFSStore_Encrypted_FormatRelation_NoFalseDiff mirrors the entity
// regression for the relation path.
func TestFSStore_Encrypted_FormatRelation_NoFalseDiff(t *testing.T) {
	s, _ := buildEncryptedStore(t)
	ctx := context.Background()

	from := entity.New("TKT-FMT-2", "ticket")
	from.Properties["title"] = "source"
	to := entity.New("TKT-FMT-3", "ticket")
	to.Properties["title"] = "target"
	if err := s.CreateEntity(ctx, from); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateEntity(ctx, to); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateRelation(ctx, from.ID, "blocks", to.ID, nil); err != nil {
		t.Fatalf("CreateRelation: %v", err)
	}

	diff, err := s.FormatRelation(ctx, from.ID, "blocks", to.ID, true /*dryRun*/)
	if err != nil {
		t.Fatalf("FormatRelation: %v", err)
	}
	if diff {
		t.Errorf("FormatRelation reported diff=true on an unchanged encrypted file " +
			"(formatter compared sealed ciphertext to plaintext)")
	}
}
