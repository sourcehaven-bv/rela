// Package reconcile defines the element that rela-to-rela sync reconciles,
// and the additive fingerprints that let two instances compare whole ranges
// of elements at once (FEAT-CMVZQFI, RES-G8CYB1R).
//
// # Element
//
// An [Element] is one row as one reader sees it: a [Kind], an address and a
// view hash. The view hash is the [canonical.Digest] of the reader's view of
// the row, built from a visibility-redacted row.
//
// Three rules bind the code that lists elements or sums them. This package
// cannot check them.
//
//   - Gate before folding. A [Range] covers only rows the reader may read; in
//     SQL the row-gate predicate applies before the sum. A count or sum over
//     hidden rows discloses their existence or their values.
//   - The stored content hash of a row stands in for the view hash only when
//     this reader has no hidden field on that row. `visible:` grants carry
//     per-row `when:` conditions, so this is decided per row, not per type.
//     Otherwise a fingerprint over the stored hash becomes an offline oracle
//     for the hidden value (BUG-J01V0Q).
//   - A relation is an element only when both of its endpoints are readable
//     and in scope.
//
// # Order
//
// Elements are ordered by (Kind, Address), with addresses compared bytewise
// as Go compares strings. A range never spans kinds. A SQL implementation
// must compare addresses with a byte collation (COLLATE "C" in PostgreSQL,
// BINARY in SQLite), or its ranges will not line up with the other side's.
//
// # Fingerprints
//
// An element's [Fingerprint] is a 128-bit hash of its kind, address and view
// hash. A [Range] is the count of its elements and the sum of their
// fingerprints modulo 2^128, so the fingerprint of a range is one aggregate
// over its rows and adding or removing a row is one addition. [Fingerprint.Limbs]
// is the form a database stores and sums.
//
// The fingerprint detects accidental difference: two different sets match
// with probability about 2^-128 per comparison. It is not a commitment. A
// writer inside the scope can craft rows whose fingerprints cancel a change
// (Wagner's generalized birthday attack works at any width), and so hide that
// change from sync. A matching fingerprint is never evidence of integrity or
// of authorization.
//
// For a reader with hidden fields the view hash is computed from the
// redacted row, which needs the row's body. That cost is the redacted path's
// (step 4 of FEAT-CMVZQFI), not this package's.
package reconcile

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/bits"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// Kind says whether an element is an entity or a relation. The zero Kind is
// invalid.
type Kind byte

// The two kinds. The values are part of the fingerprint encoding.
const (
	KindEntity   Kind = 'E'
	KindRelation Kind = 'R'
)

// ParseKind returns the Kind encoded as b, or an error for any other byte.
// Code that decodes a kind from a peer must use it: [Element.Fingerprint]
// panics on an invalid kind.
func ParseKind(b byte) (Kind, error) {
	switch k := Kind(b); k {
	case KindEntity, KindRelation:
		return k, nil
	default:
		return 0, fmt.Errorf("reconcile: invalid element kind %q", b)
	}
}

// Element is one row as one reader sees it. Build one with [EntityElement] or
// [RelationElement]. A client that receives (kind, address, hash) from a
// peer may fill the fields directly, with the kind from [ParseKind].
type Element struct {
	Kind Kind
	// Address is "type/id" or "type/id@face" for an entity, and the
	// relation key "from--type--to" or "from@face--type--to" for a relation.
	Address string
	// Hash is the view hash: the canonical digest of the reader's view.
	Hash canonical.Digest
}

// EntityElement returns the element for view, the entity as the reader sees
// it. Hidden values must already be gone from view.
func EntityElement(view entity.Entity) Element {
	return Element{
		Kind:    KindEntity,
		Address: EntityAddress(view.Type, view.Ref()),
		Hash:    canonical.SumEntity(view),
	}
}

// RelationElement returns the element for view, the relation as the reader
// sees it. Hidden values must already be gone from view, and both endpoints
// must be readable and in scope.
func RelationElement(view entity.Relation) Element {
	return Element{
		Kind:    KindRelation,
		Address: view.Identity().String(),
		Hash:    canonical.SumRelation(view),
	}
}

// EntityAddress returns the address of the entity face row ref of type
// typeName: "type/id", or "type/id@face" for a non-implicit face.
func EntityAddress(typeName string, ref entity.Ref) string {
	return typeName + "/" + ref.String()
}

// fingerprintDomain separates element fingerprints from every other SHA-256
// use. Bump the version if the encoding below ever changes.
const fingerprintDomain = "rela/reconcile/fingerprint/v1"

// Fingerprint returns e's fingerprint: the first 128 bits, big endian, of
// SHA-256 over a domain tag, the kind byte, the length-prefixed address and
// the view hash. It panics on a kind other than [KindEntity] or
// [KindRelation]; decode a peer's kind with [ParseKind] so that only a
// programming error can reach the panic.
func (e Element) Fingerprint() Fingerprint {
	if e.Kind != KindEntity && e.Kind != KindRelation {
		panic(fmt.Sprintf("reconcile: invalid element kind %q", byte(e.Kind)))
	}
	h := sha256.New()
	h.Write([]byte(fingerprintDomain))
	h.Write([]byte{byte(e.Kind)})
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], uint64(len(e.Address)))
	h.Write(n[:])
	h.Write([]byte(e.Address))
	h.Write(e.Hash[:])
	var sum [sha256.Size]byte
	h.Sum(sum[:0])
	return Fingerprint{
		Hi: binary.BigEndian.Uint64(sum[0:8]),
		Lo: binary.BigEndian.Uint64(sum[8:16]),
	}
}

// Fingerprint is a 128-bit value in an additive group modulo 2^128. Hi holds
// the high 64 bits. The zero value is the fingerprint of the empty set.
type Fingerprint struct {
	Hi, Lo uint64
}

// Add returns f + g modulo 2^128.
func (f Fingerprint) Add(g Fingerprint) Fingerprint {
	lo, carry := bits.Add64(f.Lo, g.Lo, 0)
	hi, _ := bits.Add64(f.Hi, g.Hi, carry)
	return Fingerprint{Hi: hi, Lo: lo}
}

// Sub returns f - g modulo 2^128.
func (f Fingerprint) Sub(g Fingerprint) Fingerprint {
	lo, borrow := bits.Sub64(f.Lo, g.Lo, 0)
	hi, _ := bits.Sub64(f.Hi, g.Hi, borrow)
	return Fingerprint{Hi: hi, Lo: lo}
}

// limbBits is the width of one limb of [Fingerprint.Limbs].
const limbBits = 32

// Limbs splits f into four 32-bit limbs, most significant first. A database
// stores them as four non-negative 64-bit integer columns (bigint in
// PostgreSQL) and sums each column on its own. A sum of up to 2^31 rows of
// 32-bit values fits a signed 64-bit integer, so SQLite's SUM, which fails on
// overflow, is safe, and PostgreSQL's SUM(bigint), which returns numeric,
// casts back to bigint. [FingerprintFromLimbSums] folds the sums back.
func (f Fingerprint) Limbs() [4]uint32 {
	return [4]uint32{
		uint32(f.Hi >> limbBits), uint32(f.Hi), //nolint:gosec // truncation is the split
		uint32(f.Lo >> limbBits), uint32(f.Lo), //nolint:gosec // truncation is the split
	}
}

// FingerprintFromLimbSums returns the sum, modulo 2^128, of the fingerprints
// whose limbs were summed column by column into sums (most significant
// first). Each sum may exceed 32 bits; the carries are folded here.
func FingerprintFromLimbSums(sums [4]uint64) Fingerprint {
	// Limb i has weight 2^(32*(3-i)). Shift each sum into its 128-bit
	// position and add; anything above bit 127 falls off.
	return Fingerprint{Hi: sums[0] << limbBits}.
		Add(Fingerprint{Hi: sums[1]}).
		Add(Fingerprint{Hi: sums[2] >> limbBits, Lo: sums[2] << limbBits}).
		Add(Fingerprint{Lo: sums[3]})
}

// Range is the additive fingerprint of a set of elements: how many there are
// and the sum of their fingerprints. Count wraps modulo 2^64 like Sum wraps
// modulo 2^128, so [Range.Remove] always inverts [Range.Add]. The zero value
// is the empty range. Two ranges are compared with ==.
type Range struct {
	Count uint64
	Sum   Fingerprint
}

// Add returns r with one more element, e. It does not check that e is new to
// r; adding an element twice counts it twice. It panics on an invalid kind,
// like [Element.Fingerprint].
func (r Range) Add(e Element) Range {
	return Range{Count: r.Count + 1, Sum: r.Sum.Add(e.Fingerprint())}
}

// Remove returns r without e. It does not check that e is in r; removing an
// absent element yields a range no real set has.
func (r Range) Remove(e Element) Range {
	return Range{Count: r.Count - 1, Sum: r.Sum.Sub(e.Fingerprint())}
}

// Merge returns the range of the union of two disjoint ranges.
func (r Range) Merge(o Range) Range {
	return Range{Count: r.Count + o.Count, Sum: r.Sum.Add(o.Sum)}
}
