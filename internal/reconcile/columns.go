package reconcile

import "github.com/Sourcehaven-BV/rela/internal/entity"

// Columns is an element in the form a database store keeps it on the row
// (TKT-1G2XR1R): the hex content hash and the four fingerprint limbs, most
// significant first. The limbs are int64 because SQL has no unsigned
// integer; each is below 2^32. The address is not stored: it is a function
// of the row's key columns.
//
// The stored hash covers the whole row, hidden fields included. The package
// rules on stored hashes apply to any reader of these columns.
type Columns struct {
	Hash  string
	Limbs [4]int64
}

// Columns returns e in stored form.
func (e Element) Columns() Columns {
	c := Columns{Hash: e.Hash.String()}
	for i, l := range e.Fingerprint().Limbs() {
		c.Limbs[i] = int64(l)
	}
	return c
}

// Args returns c as five statement arguments in column order: element_hash,
// element_fp0, element_fp1, element_fp2, element_fp3.
func (c Columns) Args() []any {
	return []any{c.Hash, c.Limbs[0], c.Limbs[1], c.Limbs[2], c.Limbs[3]}
}

// EntityRowColumns returns the stored columns of the entity row ref of type
// typ with these properties and body. props must be decoded the way the
// store's read decodes them, so the stored hash equals the hash of the row a
// later read returns.
func EntityRowColumns(ref entity.Ref, typ string, props map[string]any, content string) Columns {
	e := entity.New(ref.ID, typ)
	e.Face = ref.Face
	e.Properties = props
	e.Content = content
	return EntityElement(*e).Columns()
}

// RelationRowColumns is [EntityRowColumns] for the relation row k.
func RelationRowColumns(k entity.RelationKey, props map[string]any, content string) Columns {
	r := entity.NewRelation(k.From, k.Type, k.To)
	r.FromFace = k.FromFace
	r.Properties = props
	r.Content = content
	return RelationElement(*r).Columns()
}
