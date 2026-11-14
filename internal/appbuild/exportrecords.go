package appbuild

import "github.com/Sourcehaven-BV/rela/internal/records"

// RecordStore returns the raw record store of s, for an operator export
// that reads every row, as `rela export --all` reads the raw entity store.
// It applies no access rules: a request goes through [Records] instead.
//
// Nil: returned for a Services built by [NewFromCollaborators].
func RecordStore(s *Services) records.Store {
	if s == nil {
		return nil
	}
	return s.recordStore
}
