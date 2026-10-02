package datamigration

// RenameKind says what a [Rename] renamed.
type RenameKind int

// The renames a migration file can record.
const (
	RenameProperty RenameKind = iota
	RenameEntityType
	RenameRelationType
)

// Rename is one schema rename a migration step performs. Owner is the entity
// type of a property rename and empty otherwise.
type Rename struct {
	Kind            RenameKind
	Owner, From, To string
}

// Renames returns the renames among the file's steps, in step order. Tools
// that key their own config by schema names (the classification overlay) use
// it to follow a name across the migration history.
func (f *File) Renames() []Rename {
	var out []Rename
	for _, step := range f.Steps {
		switch s := step.(type) {
		case *renamePropertyStep:
			out = append(out, Rename{Kind: RenameProperty, Owner: s.Entity, From: s.From, To: s.To})
		case *renameEntityTypeStep:
			out = append(out, Rename{Kind: RenameEntityType, From: s.From, To: s.To})
		case *renameRelationTypeStep:
			out = append(out, Rename{Kind: RenameRelationType, From: s.From, To: s.To})
		}
	}
	return out
}
